package models

// Realtime event names carried by WSEvent.Event and POST /internal/events
// (contract §7). The payload of each is the same JSON object the REST
// endpoints return, so iOS decodes events with its existing models.
const (
	EventOrderCreated        = "order.created"
	EventOrderUpdated        = "order.updated"
	EventTransactionUpdated  = "transaction.updated"
	EventNotificationCreated = "notification.created"
)

// EmitEventRequest is the body of POST /internal/events: any service can ask
// the notifications hub to fan an event out to a set of users, optionally
// persisting a notification row for the feed.
type EmitEventRequest struct {
	UserIDs []string `json:"userIds"`
	Event   string   `json:"event"`
	Data    any      `json:"data"`

	// Notify, when set, also writes a notifications row per user and pushes a
	// notification.created event alongside Event. Omit for pure state pushes.
	Notify *NotifySpec `json:"notify,omitempty"`
}

type NotifySpec struct {
	Type           NotificationType `json:"type"`
	Title          string           `json:"title"`
	Body           string           `json:"body"`
	RelatedOrderID *string          `json:"relatedOrderId,omitempty"`
}
