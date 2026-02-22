package executor

import (
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/worker/internal/model"

	"go.uber.org/zap"
)

type SendEmailExecutor struct{}

func (s *SendEmailExecutor) Execute(task model.WorkflowTask) error {
	logger.Log.Info("sending email", zap.String("execution_id", task.WorkflowExecutionID))
	time.Sleep(40 * time.Second)
	logger.Log.Info("finished sending email", zap.String("execution_id", task.WorkflowExecutionID))
	return nil
}
