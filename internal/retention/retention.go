// Package retention limpia periódicamente alertas antiguas y sus snapshots para que el
// almacenamiento (un PVC compartido con otros servicios del homelab) no crezca sin límite.
package retention

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bigbrother/api/internal/models"
	"gorm.io/gorm"
)

const (
	batchSize = 200
	// Ficheros sin alerta asociada más recientes que esto se respetan: pueden estar
	// escribiéndose justo antes de crear la fila en la BD.
	orphanGrace = time.Hour
)

// Policy define los límites. Un valor 0 desactiva ese límite.
type Policy struct {
	MaxAge   time.Duration
	MaxBytes int64
}

// Store abstrae la base de datos para poder testear la política sin Postgres.
type Store interface {
	// OldestAlerts devuelve hasta limit alertas ordenadas de más antigua a más nueva.
	// Si before no es cero, solo las detectadas antes de esa fecha.
	OldestAlerts(before time.Time, limit int) ([]models.MotionAlert, error)
	DeleteAlerts(ids []uint) error
	// SnapshotFiles devuelve el nombre de fichero de todos los snapshots y clips referenciados.
	SnapshotFiles() (map[string]bool, error)
}

type Service struct {
	store  Store
	dir    string
	policy Policy
	now    func() time.Time
}

func New(store Store, dir string, policy Policy) *Service {
	return &Service{store: store, dir: dir, policy: policy, now: time.Now}
}

// Result resume una ejecución (para logs y tests).
type Result struct {
	ExpiredAlerts int
	EvictedAlerts int
	OrphanFiles   int
	BytesFreed    int64
}

// Start ejecuta la limpieza al poco de arrancar y después cada interval, hasta cancelar ctx.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	if s.policy.MaxAge == 0 && s.policy.MaxBytes == 0 {
		log.Printf("[RETENTION] Desactivada (ALERT_RETENTION_DAYS=0 y ALERT_MAX_STORAGE_MB=0)")
		return
	}
	log.Printf("[RETENTION] Activa: antigüedad máx %s, espacio máx %d MB, cada %s",
		s.policy.MaxAge, s.policy.MaxBytes/(1024*1024), interval)
	go func() {
		timer := time.NewTimer(time.Minute)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				res, err := s.RunOnce()
				if err != nil {
					log.Printf("[RETENTION] Error: %v", err)
				} else if res.ExpiredAlerts+res.EvictedAlerts+res.OrphanFiles > 0 {
					log.Printf("[RETENTION] Eliminadas %d alertas caducadas, %d por espacio y %d ficheros huérfanos (%d MB liberados)",
						res.ExpiredAlerts, res.EvictedAlerts, res.OrphanFiles, res.BytesFreed/(1024*1024))
				}
				timer.Reset(interval)
			}
		}
	}()
}

// RunOnce aplica la política: caducidad por antigüedad, huérfanos y tope de espacio.
func (s *Service) RunOnce() (Result, error) {
	var res Result

	if s.policy.MaxAge > 0 {
		cutoff := s.now().Add(-s.policy.MaxAge)
		for {
			alerts, err := s.store.OldestAlerts(cutoff, batchSize)
			if err != nil {
				return res, err
			}
			if len(alerts) == 0 {
				break
			}
			freed, err := s.deleteAlerts(alerts)
			if err != nil {
				return res, err
			}
			res.ExpiredAlerts += len(alerts)
			res.BytesFreed += freed
			if len(alerts) < batchSize {
				break
			}
		}
	}

	orphans, freed, err := s.removeOrphans()
	if err != nil {
		return res, err
	}
	res.OrphanFiles = orphans
	res.BytesFreed += freed

	if s.policy.MaxBytes > 0 {
		used, err := dirSize(s.dir)
		if err != nil {
			return res, err
		}
		for used > s.policy.MaxBytes {
			alerts, err := s.store.OldestAlerts(time.Time{}, batchSize)
			if err != nil {
				return res, err
			}
			if len(alerts) == 0 {
				break
			}
			// Borrar solo lo necesario para bajar del tope, de más antigua a más nueva
			var chunk []models.MotionAlert
			var projected = used
			for _, a := range alerts {
				chunk = append(chunk, a)
				projected -= s.alertSize(a)
				if projected <= s.policy.MaxBytes {
					break
				}
			}
			freed, err := s.deleteAlerts(chunk)
			if err != nil {
				return res, err
			}
			res.EvictedAlerts += len(chunk)
			res.BytesFreed += freed
			used -= freed
			if freed == 0 && len(chunk) == len(alerts) && len(alerts) < batchSize {
				break // no quedan snapshots que liberar
			}
		}
	}

	return res, nil
}

// alertFiles devuelve los ficheros en disco de una alerta (snapshot y clip de vídeo).
func (s *Service) alertFiles(a models.MotionAlert) []string {
	var files []string
	for _, p := range []string{a.SnapshotPath, a.ClipPath} {
		if p != "" {
			files = append(files, filepath.Join(s.dir, filepath.Base(p)))
		}
	}
	return files
}

func (s *Service) alertSize(a models.MotionAlert) int64 {
	var total int64
	for _, f := range s.alertFiles(a) {
		total += fileSize(f)
	}
	return total
}

func (s *Service) deleteAlerts(alerts []models.MotionAlert) (int64, error) {
	ids := make([]uint, 0, len(alerts))
	for _, a := range alerts {
		ids = append(ids, a.ID)
	}
	if err := s.store.DeleteAlerts(ids); err != nil {
		return 0, err
	}
	var freed int64
	for _, a := range alerts {
		for _, p := range s.alertFiles(a) {
			size := fileSize(p)
			if err := os.Remove(p); err == nil {
				freed += size
			}
		}
	}
	return freed, nil
}

func (s *Service) removeOrphans() (int, int64, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	referenced, err := s.store.SnapshotFiles()
	if err != nil {
		return 0, 0, err
	}
	count := 0
	var freed int64
	for _, e := range entries {
		// Ocultos: ficheros temporales de escritura (p. ej. ".clip_1.mp4.tmp")
		if e.IsDir() || referenced[e.Name()] || strings.HasPrefix(e.Name(), ".") && s.now().Sub(modTime(e)) < orphanGrace {
			continue
		}
		info, err := e.Info()
		if err != nil || s.now().Sub(info.ModTime()) < orphanGrace {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err == nil {
			count++
			freed += info.Size()
		}
	}
	return count, freed, nil
}

func modTime(e os.DirEntry) time.Time {
	if info, err := e.Info(); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func fileSize(p string) int64 {
	if p == "" {
		return 0
	}
	info, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return info.Size()
}

func dirSize(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
	}
	return total, nil
}

// GormStore implementa Store sobre la tabla alerts.
type GormStore struct {
	DB *gorm.DB
}

func (g GormStore) OldestAlerts(before time.Time, limit int) ([]models.MotionAlert, error) {
	q := g.DB.Model(&models.MotionAlert{}).Order("detected_at ASC").Limit(limit)
	if !before.IsZero() {
		q = q.Where("detected_at < ?", before)
	}
	var alerts []models.MotionAlert
	return alerts, q.Find(&alerts).Error
}

func (g GormStore) DeleteAlerts(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return g.DB.Delete(&models.MotionAlert{}, ids).Error
}

func (g GormStore) SnapshotFiles() (map[string]bool, error) {
	files := map[string]bool{}
	for _, col := range []string{"snapshot_path", "clip_path"} {
		var paths []string
		if err := g.DB.Model(&models.MotionAlert{}).Where(col+" <> ''").Pluck(col, &paths).Error; err != nil {
			return nil, err
		}
		for _, p := range paths {
			files[filepath.Base(p)] = true
		}
	}
	return files, nil
}
