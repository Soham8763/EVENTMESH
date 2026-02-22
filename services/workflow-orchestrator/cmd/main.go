package main

import (
	"context"
	"log"

	"eventmesh/workflow-orchestrator/internal/consumer"
	"eventmesh/workflow-orchestrator/internal/engine"
	"eventmesh/workflow-orchestrator/internal/producer"
	"eventmesh/workflow-orchestrator/internal/repository"
)

func main() {
	log.Println("workflow-orchestrator starting...")

	db := repository.NewPostgres()
	workflowRepo := repository.NewWorkflowRepository(db)

	defs, err := workflowRepo.LoadDefinitions()
	if err != nil {
		log.Fatalf("failed to load workflow definitions: %v", err)
	}

	log.Printf("loaded %d workflow definitions\n", len(defs))

	taskProducer, err := producer.NewProducer(
		[]string{"localhost:19092"},
		"workflow_tasks",
	)
	if err != nil {
		log.Fatalf("failed to create task producer: %v", err)
	}

	execEngine := engine.NewExecutionEngine(db, taskProducer)

	triggerConsumer, err := consumer.NewTriggerConsumer(
		[]string{"localhost:19092"},
		"workflow-orchestrator-group",
		"workflow_triggers",
		execEngine,
	)
	if err != nil {
		log.Fatal(err)
	}

	resultConsumer, err := consumer.NewResultConsumer(
		[]string{"localhost:19092"},
		"workflow-results-group",
		"workflow_task_results",
		execEngine,
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	go triggerConsumer.Start(ctx)
	go resultConsumer.Start(ctx)

	select {}
}
