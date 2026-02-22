package main

import (
	"context"
	"log"

	"eventmesh/workflow-orchestrator/internal/consumer"
	"eventmesh/workflow-orchestrator/internal/engine"
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

	execEngine := engine.NewExecutionEngine(db)

	triggerConsumer, err := consumer.NewTriggerConsumer(
		[]string{"localhost:19092"},
		"workflow-orchestrator-group",
		"workflow_triggers",
		execEngine,
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	go triggerConsumer.Start(ctx)

	select {}
}
