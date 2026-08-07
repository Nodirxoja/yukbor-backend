package notifications

import (
	"sync"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Hub is an in-memory per-user event fan-out. The WS handler subscribes a
// connection to its userId channel; services publish via /internal/events.
//
// In-memory is correct here rather than merely expedient: every event is also
// persisted as a notification row, so a client that was disconnected catches up
// over REST. The hub is an accelerator, not the system of record. It would need
// a broker only to run more than one replica.
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
				// Dropping beats blocking the emitting service: the row is in
				// the database either way.
			}
		}
	}
}

// Count reports a user's live connections (logging and diagnostics).
func (h *Hub) Count(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[userID])
}
