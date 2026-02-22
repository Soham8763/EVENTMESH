package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"eventmesh/worker/internal/consumer"
	"eventmesh/worker/internal/executor"
)

func main() {
	log.Println("worker starting...")

	brokers := []string{"localhost:19092"}

	// Initialize executors
	registry := executor.NewRegistry()
	registry.Register("send_email", &executor.SendEmailExecutor{})
	registry.Register("create_profile", &executor.CreateProfileExecutor{})

	// Initialize task consumer
	taskConsumer, err := consumer.NewTaskConsumer(
		brokers,
		"worker-group-2",
		"workflow_tasks",
		registry,
	)
	if err != nil {
		log.Fatalf("failed to create task consumer: %v", err)
	}

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go taskConsumer.Start(ctx)

	// Wait for terminate signal
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	<-sigterm

	log.Println("worker shutting down...")
}
