package executor

import (
	"context"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/worker/internal/model"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type SendEmailExecutor struct{}

func (s *SendEmailExecutor) Execute(ctx context.Context, task model.WorkflowTask) error {
	tr := otel.Tracer("worker")
	_, span := tr.Start(ctx, "SendEmail")
	defer span.End()

	logger.Log.Info("sending email", zap.String("execution_id", task.WorkflowExecutionID))
	time.Sleep(1 * time.Second)
	logger.Log.Info("finished sending email", zap.String("execution_id", task.WorkflowExecutionID))
	return nil
}
