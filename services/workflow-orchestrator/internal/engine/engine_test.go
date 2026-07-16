package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"eventmesh/internal/events"
	"eventmesh/workflow-orchestrator/internal/model"
	"eventmesh/workflow-orchestrator/internal/producer"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// SimpleMockSyncProducer implements sarama.SyncProducer for testing
type SimpleMockSyncProducer struct {
	sent []*sarama.ProducerMessage
}

func (m *SimpleMockSyncProducer) SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error) {
	m.sent = append(m.sent, msg)
	return 0, 0, nil
}

func (m *SimpleMockSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	m.sent = append(m.sent, msgs...)
	return nil
}

func (m *SimpleMockSyncProducer) Close() error {
	return nil
}

func (m *SimpleMockSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag {
	return 0
}

func (m *SimpleMockSyncProducer) IsTransactional() bool {
	return false
}

func (m *SimpleMockSyncProducer) BeginTxn() error {
	return nil
}

func (m *SimpleMockSyncProducer) CommitTxn() error {
	return nil
}

func (m *SimpleMockSyncProducer) AbortTxn() error {
	return nil
}

func (m *SimpleMockSyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupId string) error {
	return nil
}

func (m *SimpleMockSyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupId string, metadata *string) error {
	return nil
}

func TestExecutionEngine_Transitions(t *testing.T) {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		dsn = "postgres://eventmesh:eventmesh@localhost:5432/eventmesh?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skip("PostgreSQL not accessible, skipping integration test")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skip("PostgreSQL connection refused, skipping integration test")
	}

	// 1. Setup Definitions
	workflowName := "test_workflow_" + uuid.New().String()[:8]
	steps := []map[string]interface{}{
		{"step": "step_one"},
		{"step": "step_two"},
	}
	stepsJSON, _ := json.Marshal(steps)

	_, err = db.Exec(`
		INSERT INTO workflow_definitions (name, steps, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (name) DO UPDATE SET steps = $2
	`, workflowName, stepsJSON)
	if err != nil {
		t.Fatalf("Failed to seed workflow definition: %v", err)
	}
	defer db.Exec("DELETE FROM workflow_definitions WHERE name = $1", workflowName)

	// 2. Setup Mock Producers
	mockSaramaProducer := &SimpleMockSyncProducer{}
	taskProducer := producer.NewProducerWithSyncProducer(mockSaramaProducer, "workflow_tasks")
	failureProducer := producer.NewFailureProducerWithSyncProducer(mockSaramaProducer)
	eventPublisher := events.NewEventPublisher([]string{"localhost:9999"}) // Dummy broker (publisher errors are ignored in engine)

	engine := NewExecutionEngine(db, taskProducer, failureProducer, eventPublisher)

	// 3. Trigger Workflow (Start)
	trigger := events.WorkflowTriggerEvent{
		TriggerID:     "trig-" + uuid.New().String()[:8],
		EventID:       "evt-" + uuid.New().String()[:8],
		TenantID:      "tenant-test",
		WorkflowName:  workflowName,
		CorrelationID: "corr-123",
	}

	ctx := context.Background()
	err = engine.HandleTrigger(ctx, trigger)
	if err != nil {
		t.Fatalf("HandleTrigger failed: %v", err)
	}

	// 4. Verify workflow is created and running, and step 1 is pending/running
	// Since HandleTrigger calls AdvanceExecution directly, workflow status should be RUNNING
	var status string
	var currentStep int
	var execID string

	err = db.QueryRow(`
		SELECT id, status, current_step
		FROM workflow_executions
		WHERE trigger_id = $1
	`, trigger.TriggerID).Scan(&execID, &status, &currentStep)
	if err != nil {
		t.Fatalf("Failed to query workflow execution: %v", err)
	}

	if status != model.WorkflowRunning {
		t.Errorf("Expected status %s, got: %s", model.WorkflowRunning, status)
	}

	// Verify step_one status is RUNNING
	var stepOneStatus string
	err = db.QueryRow(`
		SELECT status
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'step_one'
	`, execID).Scan(&stepOneStatus)
	if err != nil {
		t.Fatalf("Failed to query step_one status: %v", err)
	}
	if stepOneStatus != model.StepRunning {
		t.Errorf("Expected step_one status %s, got: %s", model.StepRunning, stepOneStatus)
	}

	// Verify a task was emitted to mock Kafka for step_one
	if len(mockSaramaProducer.sent) == 0 {
		t.Fatalf("Expected at least 1 Kafka message emitted, got 0")
	}

	// Clear sent messages to track step 2
	mockSaramaProducer.sent = nil

	// 5. Complete step_one successfully
	resultOne := model.TaskResult{
		TaskID:              uuid.New().String(),
		WorkflowExecutionID: execID,
		StepName:            "step_one",
		Status:              model.StepSuccess,
		CorrelationID:       "corr-123",
	}

	err = engine.HandleResult(ctx, resultOne)
	if err != nil {
		t.Fatalf("HandleResult for step_one failed: %v", err)
	}

	// Verify step_one is now SUCCESS in DB
	err = db.QueryRow(`
		SELECT status
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'step_one'
	`, execID).Scan(&stepOneStatus)
	if stepOneStatus != model.StepSuccess {
		t.Errorf("Expected step_one to be SUCCESS, got: %s", stepOneStatus)
	}

	// Verify step_two is now RUNNING (since HandleResult advanced the workflow)
	var stepTwoStatus string
	err = db.QueryRow(`
		SELECT status
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'step_two'
	`, execID).Scan(&stepTwoStatus)
	if err != nil {
		t.Fatalf("Failed to query step_two status: %v", err)
	}
	if stepTwoStatus != model.StepRunning {
		t.Errorf("Expected step_two to be RUNNING, got: %s", stepTwoStatus)
	}

	// Verify step_two task was emitted to Kafka
	if len(mockSaramaProducer.sent) == 0 {
		t.Fatalf("Expected Kafka message for step_two, got 0")
	}
	mockSaramaProducer.sent = nil

	// 6. Complete step_two successfully
	resultTwo := model.TaskResult{
		TaskID:              uuid.New().String(),
		WorkflowExecutionID: execID,
		StepName:            "step_two",
		Status:              model.StepSuccess,
		CorrelationID:       "corr-123",
	}

	err = engine.HandleResult(ctx, resultTwo)
	if err != nil {
		t.Fatalf("HandleResult for step_two failed: %v", err)
	}

	// Verify workflow status is now COMPLETED in DB
	err = db.QueryRow(`
		SELECT status
		FROM workflow_executions
		WHERE id = $1
	`, execID).Scan(&status)
	if status != model.WorkflowCompleted {
		t.Errorf("Expected workflow to be COMPLETED, got: %s", status)
	}

	// Cleanup test execution records
	db.Exec("DELETE FROM workflow_step_executions WHERE workflow_execution_id = $1", execID)
	db.Exec("DELETE FROM workflow_executions WHERE id = $1", execID)
}

func TestExecutionEngine_RetryLogic(t *testing.T) {
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		dsn = "postgres://eventmesh:eventmesh@localhost:5432/eventmesh?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skip("PostgreSQL not accessible, skipping integration test")
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skip("PostgreSQL connection refused, skipping integration test")
	}

	// 1. Setup Definitions
	workflowName := "test_retry_workflow_" + uuid.New().String()[:8]
	steps := []map[string]interface{}{
		{"step": "unreliable_step"},
	}
	stepsJSON, _ := json.Marshal(steps)

	_, err = db.Exec(`
		INSERT INTO workflow_definitions (name, steps, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (name) DO UPDATE SET steps = $2
	`, workflowName, stepsJSON)
	if err != nil {
		t.Fatalf("Failed to seed workflow definition: %v", err)
	}
	defer db.Exec("DELETE FROM workflow_definitions WHERE name = $1", workflowName)

	// 2. Setup Mock Producers
	mockSaramaProducer := &SimpleMockSyncProducer{}
	taskProducer := producer.NewProducerWithSyncProducer(mockSaramaProducer, "workflow_tasks")
	failureProducer := producer.NewFailureProducerWithSyncProducer(mockSaramaProducer)
	eventPublisher := events.NewEventPublisher([]string{"localhost:9999"})

	engine := NewExecutionEngine(db, taskProducer, failureProducer, eventPublisher)

	// 3. Trigger Workflow
	trigger := events.WorkflowTriggerEvent{
		TriggerID:     "trig-" + uuid.New().String()[:8],
		EventID:       "evt-" + uuid.New().String()[:8],
		TenantID:      "tenant-test",
		WorkflowName:  workflowName,
		CorrelationID: "corr-123",
	}

	ctx := context.Background()
	err = engine.HandleTrigger(ctx, trigger)
	if err != nil {
		t.Fatalf("HandleTrigger failed: %v", err)
	}

	var execID string
	db.QueryRow("SELECT id FROM workflow_executions WHERE trigger_id = $1", trigger.TriggerID).Scan(&execID)

	// 4. Fail step 1 (retry 1)
	errMsg := "network timeout"
	resultFail1 := model.TaskResult{
		TaskID:              uuid.New().String(),
		WorkflowExecutionID: execID,
		StepName:            "unreliable_step",
		Status:              model.StepFailed,
		Error:               &errMsg,
		CorrelationID:       "corr-123",
	}

	err = engine.HandleResult(ctx, resultFail1)
	if err != nil {
		t.Fatalf("HandleResult retry 1 failed: %v", err)
	}

	// Verify retry count is now 1, status is RUNNING (since we reset it and re-advanced)
	// Note: in HandleResult, if step fails and retry count < 3, it sets status to PENDING
	// but then calls AdvanceExecution which marks it back to RUNNING and dispatches it.
	var retryCount int
	var stepStatus string
	err = db.QueryRow(`
		SELECT retry_count, status
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'unreliable_step'
	`, execID).Scan(&retryCount, &stepStatus)
	if err != nil {
		t.Fatalf("Failed to query step retry details: %v", err)
	}

	if retryCount != 1 {
		t.Errorf("Expected retry_count = 1, got: %d", retryCount)
	}
	if stepStatus != model.StepRunning {
		t.Errorf("Expected step to be RUNNING (re-dispatched), got: %s", stepStatus)
	}

	// 5. Fail step 1 again (retry 2)
	err = engine.HandleResult(ctx, resultFail1)
	if err != nil {
		t.Fatalf("HandleResult retry 2 failed: %v", err)
	}
	db.QueryRow(`
		SELECT retry_count
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'unreliable_step'
	`, execID).Scan(&retryCount)
	if retryCount != 2 {
		t.Errorf("Expected retry_count = 2, got: %d", retryCount)
	}

	// 6. Fail step 1 again (retry 3)
	err = engine.HandleResult(ctx, resultFail1)
	if err != nil {
		t.Fatalf("HandleResult retry 3 failed: %v", err)
	}
	db.QueryRow(`
		SELECT retry_count
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'unreliable_step'
	`, execID).Scan(&retryCount)
	if retryCount != 3 {
		t.Errorf("Expected retry_count = 3, got: %d", retryCount)
	}

	// 7. Fail step 1 for the 4th time (triggers FAILED state since retryCount >= 3)
	err = engine.HandleResult(ctx, resultFail1)
	if err != nil {
		t.Fatalf("HandleResult 4th fail failed: %v", err)
	}

	// Verify step status is FAILED
	err = db.QueryRow(`
		SELECT status
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1 AND step_name = 'unreliable_step'
	`, execID).Scan(&stepStatus)
	if stepStatus != model.StepFailed {
		t.Errorf("Expected step to be FAILED, got: %s", stepStatus)
	}

	// Verify workflow status is FAILED
	var wfStatus string
	err = db.QueryRow(`
		SELECT status
		FROM workflow_executions
		WHERE id = $1
	`, execID).Scan(&wfStatus)
	if wfStatus != model.WorkflowFailed {
		t.Errorf("Expected workflow to be FAILED, got: %s", wfStatus)
	}

	// Cleanup
	db.Exec("DELETE FROM workflow_step_executions WHERE workflow_execution_id = $1", execID)
	db.Exec("DELETE FROM workflow_executions WHERE id = $1", execID)
}
