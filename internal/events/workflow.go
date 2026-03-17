package events

type WorkflowStartedEvent struct {
	BaseEvent
	WorkflowID string `json:"workflow_id"`
}

type WorkflowCompletedEvent struct {
	BaseEvent
}

type WorkflowFailedEvent struct {
	BaseEvent
	Error string `json:"error"`
}
