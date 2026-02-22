package main

import (
	"context"

	"eventmesh/pkg/logger"
	"eventmesh/workflow-orchestrator/internal/consumer"
	"eventmesh/workflow-orchestrator/internal/engine"
	"eventmesh/workflow-orchestrator/internal/producer"
	"eventmesh/workflow-orchestrator/internal/repository"

	"go.uber.org/zap"
)

func main() {
	logger.Init()
	defer logger.Log.Sync()

	logger.Log.Info("workflow-orchestrator starting...")

	db := repository.NewPostgres()
	workflowRepo := repository.NewWorkflowRepository(db)

	defs, err := workflowRepo.LoadDefinitions()
	if err != nil {
		logger.Log.Fatal("failed to load workflow definitions", zap.Error(err))
	}

	logger.Log.Info("loaded workflow definitions", zap.Int("count", len(defs)))

	taskProducer, err := producer.NewProducer(
		[]string{"localhost:19092"},
		"workflow_tasks",
	)
	if err != nil {
		logger.Log.Fatal("failed to create task producer", zap.Error(err))
	}

	execEngine := engine.NewExecutionEngine(db, taskProducer)

	triggerConsumer, err := consumer.NewTriggerConsumer(
		[]string{"localhost:19092"},
		"workflow-orchestrator-group",
		"workflow_triggers",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create trigger consumer", zap.Error(err))
	}

	resultConsumer, err := consumer.NewResultConsumer(
		[]string{"localhost:19092"},
		"workflow-results-group",
		"workflow_task_results",
		execEngine,
	)
	if err != nil {
		logger.Log.Fatal("failed to create result consumer", zap.Error(err))
	}

	ctx := context.Background()
	go triggerConsumer.Start(ctx)
	go resultConsumer.Start(ctx)

	logger.Log.Info("orchestrator ready and consuming")
	select {}
}
