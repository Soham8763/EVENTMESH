package executor

import (
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/worker/internal/model"

	"go.uber.org/zap"
)

type CreateProfileExecutor struct{}

func (c *CreateProfileExecutor) Execute(task model.WorkflowTask) error {
	logger.Log.Info("creating profile", zap.String("execution_id", task.WorkflowExecutionID))
	time.Sleep(40 * time.Second)
	logger.Log.Info("finished creating profile", zap.String("execution_id", task.WorkflowExecutionID))
	return nil
}
