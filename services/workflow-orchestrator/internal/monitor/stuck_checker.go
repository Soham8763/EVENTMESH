package monitor

import (
	"context"
	"database/sql"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"
	"eventmesh/workflow-orchestrator/internal/model"

	"go.uber.org/zap"
)

// AdvanceFunc is the function signature for re-advancing a stuck workflow.
// This is injected from the ExecutionEngine to avoid a circular dependency.
type AdvanceFunc func(ctx context.Context, execID string, correlationID string) error

type StuckChecker struct {
	db       *sql.DB
	interval time.Duration
	timeout  time.Duration
	advance  AdvanceFunc
}

func NewStuckChecker(db *sql.DB, advance AdvanceFunc) *StuckChecker {
	return &StuckChecker{
		db:       db,
		interval: 30 * time.Second,
		timeout:  5 * time.Minute,
		advance:  advance,
	}
}

func (sc *StuckChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.check(ctx)
		}
	}
}

func (sc *StuckChecker) check(ctx context.Context) {
	// Find stuck steps — steps that have been RUNNING for longer than the timeout
	rows, err := sc.db.QueryContext(ctx, `
		SELECT we.id, wse.step_name, wse.retry_count
		FROM workflow_executions we
		JOIN workflow_step_executions wse
		  ON wse.workflow_execution_id = we.id
		WHERE we.status = 'RUNNING'
		  AND wse.status = 'RUNNING'
		  AND wse.updated_at < now() - interval '5 minutes'
	`)

	if err != nil {
		logger.Log.Error("stuck checker query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	var stuckCount float64

	for rows.Next() {
		var execID, stepName string
		var retryCount int

		if err := rows.Scan(&execID, &stepName, &retryCount); err != nil {
			logger.Log.Error("stuck checker scan error", zap.Error(err))
			continue
		}

		stuckCount++

		if retryCount >= 3 {
			// Max retries exceeded — mark the workflow as FAILED
			_, err := sc.db.ExecContext(ctx, `
				UPDATE workflow_executions
				SET status = $1, updated_at = NOW()
				WHERE id = $2
			`, model.WorkflowFailed, execID)
			if err != nil {
				logger.Log.Error("stuck checker: failed to mark workflow as FAILED", zap.Error(err))
				continue
			}

			_, err = sc.db.ExecContext(ctx, `
				UPDATE workflow_step_executions
				SET status = $1, updated_at = NOW()
				WHERE workflow_execution_id = $2 AND step_name = $3
			`, model.StepFailed, execID, stepName)
			if err != nil {
				logger.Log.Error("stuck checker: failed to mark step as FAILED", zap.Error(err))
			}

			metrics.WorkflowsFailed.Inc()
			logger.Log.Error("stuck checker: workflow failed due to stuck step exceeding retry limit",
				zap.String("execution_id", execID),
				zap.String("step", stepName),
				zap.Int("retry_count", retryCount))
			continue
		}

		// Reset the stuck step to PENDING and increment retry count
		_, err := sc.db.ExecContext(ctx, `
			UPDATE workflow_step_executions
			SET status = $1, retry_count = retry_count + 1, updated_at = NOW()
			WHERE workflow_execution_id = $2 AND step_name = $3
		`, model.StepPending, execID, stepName)
		if err != nil {
			logger.Log.Error("stuck checker: failed to reset step", zap.Error(err))
			continue
		}

		logger.Log.Warn("stuck checker: re-dispatching stuck step",
			zap.String("execution_id", execID),
			zap.String("step", stepName),
			zap.Int("retry_count", retryCount+1))

		metrics.RetryCount.Inc()

		// Re-advance the workflow to dispatch the reset step
		if err := sc.advance(ctx, execID, ""); err != nil {
			logger.Log.Error("stuck checker: failed to re-advance execution",
				zap.String("execution_id", execID),
				zap.Error(err))
		}
	}

	metrics.StuckWorkflows.Set(stuckCount)

	if stuckCount > 0 {
		logger.Log.Warn("stuck workflows detected and recovered",
			zap.Float64("count", stuckCount))
	}
}
