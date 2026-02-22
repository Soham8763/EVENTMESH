package executor

import (
	"context"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/worker/internal/model"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

type CreateProfileExecutor struct{}

func (c *CreateProfileExecutor) Execute(ctx context.Context, task model.WorkflowTask) error {
	tr := otel.Tracer("worker")
	_, span := tr.Start(ctx, "CreateProfile")
	defer span.End()

	logger.Log.Info("creating user profile", zap.String("execution_id", task.WorkflowExecutionID))
	time.Sleep(10 * time.Second)
	logger.Log.Info("finished creating user profile", zap.String("execution_id", task.WorkflowExecutionID))
	return nil
}
