package main

import (
	"log"

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

	select {}
}
