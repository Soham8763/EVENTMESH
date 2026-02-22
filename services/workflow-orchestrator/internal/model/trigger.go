package model

import "time"

type WorkflowTriggerEvent struct {
	TriggerID    string    `json:"trigger_id"`
	EventID      string    `json:"event_id"`
	TenantID     string    `json:"tenant_id"`
	WorkflowName string    `json:"workflow_name"`
	TriggeredAt  time.Time `json:"triggered_at"`
}
