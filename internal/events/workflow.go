package events

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
