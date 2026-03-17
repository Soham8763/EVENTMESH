package events

type StepScheduledEvent struct {
	BaseEvent
	StepName string `json:"step_name"`
}

type StepStartedEvent struct {
	BaseEvent
	StepName string `json:"step_name"`
	WorkerID string `json:"worker_id"`
}

type StepCompletedEvent struct {
	BaseEvent
	StepName string `json:"step_name"`
	Result   string `json:"result"`
}

type StepFailedEvent struct {
	BaseEvent
	StepName string `json:"step_name"`
	Error    string `json:"error"`
}
