package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"eventmesh/pkg/logger"

	"go.uber.org/zap"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

type WorkflowDefinition struct {
	Name      string        `json:"name"`
	Steps     []interface{} `json:"steps"`
	CreatedAt string        `json:"created_at"`
}

type ExecutionSummary struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	WorkflowName string `json:"workflow_name"`
	TriggerID    string `json:"trigger_id"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type StepExecutionSummary struct {
	StepName   string  `json:"step_name"`
	Status     string  `json:"status"`
	RetryCount int     `json:"retry_count"`
	LastError  *string `json:"last_error,omitempty"`
	UpdatedAt  string  `json:"updated_at"`
}

type ExecutionDetail struct {
	ExecutionSummary
	Steps []StepExecutionSummary `json:"steps"`
}

func (h *Handler) HandleWorkflows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Check if requesting single workflow /workflows/{name}
		path := strings.TrimPrefix(r.URL.Path, "/workflows/")
		if path != "" && path != "/workflows" {
			h.getWorkflow(w, r, path)
			return
		}
		h.listWorkflows(w, r)

	case http.MethodPost:
		h.registerWorkflow(w, r)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandleExecutions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract path parameter for single execution
	path := strings.TrimPrefix(r.URL.Path, "/executions/")
	if path != "" && path != "/executions" {
		// Check for cancel action /executions/{id}/cancel
		if strings.HasSuffix(path, "/cancel") {
			execID := strings.TrimSuffix(path, "/cancel")
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			h.cancelExecution(w, r, execID)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.getExecution(w, r, path)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.listExecutions(w, r)
}

func (h *Handler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT name, steps, created_at
		FROM workflow_definitions
		ORDER BY name ASC
	`)
	if err != nil {
		logger.Log.Error("failed to query workflow definitions", zap.Error(err))
		http.Error(w, "database query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []WorkflowDefinition
	for rows.Next() {
		var wf WorkflowDefinition
		var stepsBytes []byte
		var createdAt timeString
		err := rows.Scan(&wf.Name, &stepsBytes, &createdAt)
		if err != nil {
			logger.Log.Error("failed to scan workflow definition", zap.Error(err))
			continue
		}

		json.Unmarshal(stepsBytes, &wf.Steps)
		wf.CreatedAt = string(createdAt)
		list = append(list, wf)
	}

	json.NewEncoder(w).Encode(list)
}

func (h *Handler) getWorkflow(w http.ResponseWriter, r *http.Request, name string) {
	var wf WorkflowDefinition
	var stepsBytes []byte
	var createdAt timeString

	err := h.db.QueryRow(`
		SELECT name, steps, created_at
		FROM workflow_definitions
		WHERE name = $1
	`, name).Scan(&wf.Name, &stepsBytes, &createdAt)

	if err == sql.ErrNoRows {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Log.Error("failed to query workflow definition", zap.Error(err))
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	json.Unmarshal(stepsBytes, &wf.Steps)
	wf.CreatedAt = string(createdAt)

	json.NewEncoder(w).Encode(wf)
}

func (h *Handler) registerWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string        `json:"name"`
		Steps []interface{} `json:"steps"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || len(req.Steps) == 0 {
		http.Error(w, "name and steps are required", http.StatusBadRequest)
		return
	}

	stepsBytes, err := json.Marshal(req.Steps)
	if err != nil {
		http.Error(w, "failed to serialize steps", http.StatusBadRequest)
		return
	}

	_, err = h.db.Exec(`
		INSERT INTO workflow_definitions (name, steps, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (name) DO UPDATE SET steps = $2
	`, req.Name, stepsBytes)

	if err != nil {
		logger.Log.Error("failed to insert workflow definition", zap.Error(err))
		http.Error(w, "database insert error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "registered",
		"message": "workflow registered successfully",
	})
}

func (h *Handler) listExecutions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, tenant_id, workflow_name, trigger_id, status, created_at
		FROM workflow_executions
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		logger.Log.Error("failed to query executions", zap.Error(err))
		http.Error(w, "database query error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var list []ExecutionSummary
	for rows.Next() {
		var ex ExecutionSummary
		var createdAt timeString
		err := rows.Scan(&ex.ID, &ex.TenantID, &ex.WorkflowName, &ex.TriggerID, &ex.Status, &createdAt)
		if err != nil {
			logger.Log.Error("failed to scan execution summary", zap.Error(err))
			continue
		}
		ex.CreatedAt = string(createdAt)
		list = append(list, ex)
	}

	json.NewEncoder(w).Encode(list)
}

func (h *Handler) getExecution(w http.ResponseWriter, r *http.Request, execID string) {
	var detail ExecutionDetail
	var createdAt timeString

	err := h.db.QueryRow(`
		SELECT id, tenant_id, workflow_name, trigger_id, status, created_at
		FROM workflow_executions
		WHERE id = $1
	`, execID).Scan(&detail.ID, &detail.TenantID, &detail.WorkflowName, &detail.TriggerID, &detail.Status, &createdAt)

	if err == sql.ErrNoRows {
		http.Error(w, "execution not found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Log.Error("failed to query execution summary", zap.Error(err))
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	detail.CreatedAt = string(createdAt)

	rows, err := h.db.Query(`
		SELECT step_name, status, retry_count, last_error, updated_at
		FROM workflow_step_executions
		WHERE workflow_execution_id = $1
		ORDER BY step_index ASC
	`, execID)
	if err != nil {
		logger.Log.Error("failed to query step executions", zap.Error(err))
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var step StepExecutionSummary
		var updatedAt timeString
		err := rows.Scan(&step.StepName, &step.Status, &step.RetryCount, &step.LastError, &updatedAt)
		if err != nil {
			logger.Log.Error("failed to scan step summary", zap.Error(err))
			continue
		}
		step.UpdatedAt = string(updatedAt)
		detail.Steps = append(detail.Steps, step)
	}

	json.NewEncoder(w).Encode(detail)
}

func (h *Handler) cancelExecution(w http.ResponseWriter, r *http.Request, execID string) {
	// Start transaction
	tx, err := h.db.Begin()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var currentStatus string
	err = tx.QueryRow(`
		SELECT status
		FROM workflow_executions
		WHERE id = $1
	`, execID).Scan(&currentStatus)

	if err == sql.ErrNoRows {
		http.Error(w, "execution not found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Log.Error("failed to query status for cancel", zap.Error(err))
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if currentStatus == "COMPLETED" || currentStatus == "FAILED" {
		http.Error(w, "cannot cancel a completed or failed execution", http.StatusBadRequest)
		return
	}

	_, err = tx.Exec(`
		UPDATE workflow_executions
		SET status = 'FAILED', updated_at = NOW()
		WHERE id = $1
	`, execID)
	if err != nil {
		logger.Log.Error("failed to update status to FAILED on cancel", zap.Error(err))
		http.Error(w, "database update error", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(`
		UPDATE workflow_step_executions
		SET status = 'FAILED', last_error = 'cancelled by administrator', updated_at = NOW()
		WHERE workflow_execution_id = $1 AND status IN ('PENDING', 'RUNNING')
	`, execID)
	if err != nil {
		logger.Log.Error("failed to update steps to FAILED on cancel", zap.Error(err))
		http.Error(w, "database update error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "database commit error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":  "cancelled",
		"message": "workflow execution cancelled successfully",
	})
}

// timeString helps map raw scanner strings correctly
type timeString string

func (ts *timeString) Scan(value interface{}) error {
	switch v := value.(type) {
	case string:
		*ts = timeString(v)
	case []byte:
		*ts = timeString(v)
	case timeString:
		*ts = v
	default:
		*ts = ""
	}
	return nil
}
