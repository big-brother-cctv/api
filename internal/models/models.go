package models

import "time"

type Camera struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"uniqueIndex;not null" json:"name"`
	SourceType     string     `gorm:"default:'v4l2'" json:"sourceType"`         // 'v4l2', 'rtsp', 'http', 'test'
	ConnectionMode string     `gorm:"default:'pull'" json:"connectionMode"`     // 'pull', 'push', 'local', 'test'
	SourceURL      string     `json:"sourceUrl"`                                 // Para cámaras IP o PULL remota
	Device         string     `json:"device"`
	Resolution     string     `json:"resolution"`
	FPS            string     `gorm:"column:fps" json:"fps"`
	PostURL        string     `gorm:"column:post_url" json:"postUrl"`
	Codec          string     `json:"codec"`
	Preset         string     `json:"preset"`
	Tune           string     `json:"tune"`
	Buffer         string     `json:"buffer"`
	Rotation       string     `json:"rotation"`
	Status         string     `gorm:"default:'offline'" json:"status"`           // 'online', 'offline', 'standby'
	LastSeen       *time.Time `json:"lastSeen"`
	StreamURL      string     `json:"streamUrl"`                                 // HLS stream URL
	IngestURL      string     `json:"ingestUrl"`                                 // RTSP Ingest URL para modo PUSH
}

type HeartbeatRequest struct {
	Name           string `json:"name" binding:"required"`
	SourceType     string `json:"sourceType"`
	ConnectionMode string `json:"connectionMode"`
	Resolution     string `json:"resolution"`
	FPS            string `json:"fps"`
	Device         string `json:"device"`
	Status         string `json:"status"`
}

func (Camera) TableName() string {
	return "cameras"
}

type User struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	Username       string `gorm:"uniqueIndex;not null" json:"username"`
	Email          string `gorm:"uniqueIndex;not null" json:"email"`
	HashedPassword string `gorm:"not null" json:"-"`
	Name           string `json:"name"`
	Role           string `gorm:"default:'operator';not null" json:"role"` // "admin", "operator", "viewer"
	Permissions    string `gorm:"default:''" json:"permissions"`
	Disabled       bool   `gorm:"default:false" json:"disabled"`
	Photo          string `gorm:"default:''" json:"photo"`
}

func (User) TableName() string {
	return "users"
}

// UserDTO representa la vista pública de un usuario (sin password)
type UserDTO struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Permissions string `json:"permissions"`
	Disabled    bool   `json:"disabled"`
	Photo       string `json:"photo"`
}

func (u *User) ToDTO() UserDTO {
	role := u.Role
	if role == "" {
		if u.Username == "admin" {
			role = "admin"
		} else {
			role = "operator"
		}
	}

	return UserDTO{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Name:        u.Name,
		Role:        role,
		Permissions: u.Permissions,
		Disabled:    u.Disabled,
		Photo:       u.Photo,
	}
}

type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

type MotionAlert struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CameraID     *uint     `json:"cameraId"`
	CameraName   string    `gorm:"index;not null" json:"cameraName"`
	SnapshotPath string    `json:"snapshotPath"` // URL accesible para Telescreen: /api/alerts/snapshots/...
	DetectedAt   time.Time `gorm:"index;not null" json:"detectedAt"`
	Label        string    `gorm:"default:'motion'" json:"label"`
	Confidence   float64   `gorm:"default:1.0" json:"confidence"`
	Acknowledged bool      `gorm:"default:false;index" json:"acknowledged"`
	Severity     string    `gorm:"default:'warning'" json:"severity"` // 'info', 'warning', 'critical'
}

func (MotionAlert) TableName() string {
	return "alerts"
}
