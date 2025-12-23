package model

import "time"

type Rule struct {
	ID           string
	TenantID     string
	EventType    string
	WorkflowName string
	IsActive     bool
	CreatedAt    time.Time
}
