package alerts

import (
	"sync"

	"github.com/bigbrother/api/internal/models"
)

type Hub struct {
	clients    map[chan *models.MotionAlert]bool
	register   chan chan *models.MotionAlert
	unregister chan chan *models.MotionAlert
	broadcast  chan *models.MotionAlert
	mu         sync.RWMutex
}

func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[chan *models.MotionAlert]bool),
		register:   make(chan chan *models.MotionAlert),
		unregister: make(chan chan *models.MotionAlert),
		broadcast:  make(chan *models.MotionAlert, 100),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case ch := <-h.register:
			h.mu.Lock()
			h.clients[ch] = true
			h.mu.Unlock()

		case ch := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[ch]; ok {
				delete(h.clients, ch)
				close(ch)
			}
			h.mu.Unlock()

		case alert := <-h.broadcast:
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- alert:
				default:
					// Canal lleno, omitir para no bloquear el bucle de difusión
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register() chan *models.MotionAlert {
	ch := make(chan *models.MotionAlert, 10)
	h.register <- ch
	return ch
}

func (h *Hub) Unregister(ch chan *models.MotionAlert) {
	h.unregister <- ch
}

func (h *Hub) Broadcast(alert *models.MotionAlert) {
	h.broadcast <- alert
}
