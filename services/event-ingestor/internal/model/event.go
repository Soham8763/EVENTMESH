package model

type IngestEventRequest struct {
	EventType string                 `json:"event_type"`
	Payload   map[string]interface{} `json:"payload"`
}

type IngestEventResponse struct {
	Status   string `json:"status"`
	TenantID string `json:"tenant_id"`
}
