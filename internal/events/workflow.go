package events

import "time"

type WorkflowStartedEvent struct {
	BaseEvent
	WorkflowID string                   `json:"workflow_id"`
	Steps      []map[string]interface{} `json:"steps"`
	TenantID   string                   `json:"tenant_id"`
	TriggerID  string                   `json:"trigger_id"`
}

type WorkflowCompletedEvent struct {
	BaseEvent
}

type WorkflowFailedEvent struct {
	BaseEvent
	Error string `json:"error"`
}

type WorkflowDefinedEvent struct {
	BaseEvent
	WorkflowName string                   `json:"workflow_name"`
	Steps        []map[string]interface{} `json:"steps"`
}

type WorkflowTriggerEvent struct {
	TriggerID     string    `json:"trigger_id"`
	EventID       string    `json:"event_id"`
	TenantID      string    `json:"tenant_id"`
	WorkflowName  string    `json:"workflow_name"`
	CorrelationID string    `json:"correlation_id"`
	TriggeredAt   time.Time `json:"triggered_at"`
}
