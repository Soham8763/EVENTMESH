package events

import "time"

type BaseEvent struct {
	EventID     string    `json:"event_id"`
	EventType   EventType `json:"event_type"`
	ExecutionID string    `json:"execution_id"`
	Timestamp   time.Time `json:"timestamp"`
}
