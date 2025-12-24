package model

import "time"

type EventEnvelope struct {
	EventID        string                 `json:"event_id"`
	EventType      string                 `json:"event_type"`
	TenantID       string                 `json:"tenant_id"`
	OccurredAt     time.Time              `json:"occurred_at"`
	ReceivedAt     time.Time              `json:"received_at"`
	RequestID      string                 `json:"request_id"`
	IdempotencyKey string                 `json:"idempotency_key"`
	Payload        map[string]interface{} `json:"payload"`
}
