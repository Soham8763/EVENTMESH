package model

import "time"

type WorkflowDefinition struct {
	Name      string
	Steps     []byte
	CreatedAt time.Time
}

type WorkflowExecution struct {
	ID           string
	TenantID     string
	WorkflowName string
	TriggerID    string
	Status       string
	CurrentStep  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
