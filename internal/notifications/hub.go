package notifications

import (
	"sync"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Hub is an in-memory per-user event fan-out. The WS handler subscribes a
// connection to its userId channel; services publish via /internal/events.
//
// TODO(day-3): pair with a real websocket upgrade (nhooyr.io/websocket or
// gorilla/websocket — stdlib has no WS server). Single-instance in-memory is
// fine for the hackathon; a broker only becomes necessary with >1 replica.
type Hub struct {
	mu   sync.RWMutex
	subs map[string][]chan models.WSEvent // userId → subscriber channels
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string][]chan models.WSEvent)}
}

// Subscribe registers a channel for a user; returns an unsubscribe func.
func (h *Hub) Subscribe(userID string) (<-chan models.WSEvent, func()) {
	ch := make(chan models.WSEvent, 16)
	h.mu.Lock()
	h.subs[userID] = append(h.subs[userID], ch)
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[userID]
		for i, c := range list {
			if c == ch {
				h.subs[userID] = append(list[:i], list[i+1:]...)
				close(ch)
				break
			}
		}
	}
}

// Publish delivers an event to every live connection of each user.
// Slow consumers are skipped (non-blocking send) — REST polling catches up.
func (h *Hub) Publish(userIDs []string, ev models.WSEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, id := range userIDs {
		for _, ch := range h.subs[id] {
			select {
			case ch <- ev:
			default:
			}
		}
	}
}
