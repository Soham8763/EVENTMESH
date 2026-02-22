package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"eventmesh/worker/internal/consumer"
	"eventmesh/worker/internal/executor"
	"eventmesh/worker/internal/idempotency"
	"eventmesh/worker/internal/producer"
)

func main() {
	log.Println("worker starting...")

	brokers := []string{"localhost:19092"}

	// Initialize idempotency store
	store := idempotency.NewStore("localhost:6379", 10*time.Minute)

	// Initialize producer
	resProducer, err := producer.NewProducer(brokers, "workflow_task_results")
	if err != nil {
		log.Fatalf("failed to create result producer: %v", err)
	}

	// Initialize executors
	registry := executor.NewRegistry()
	registry.Register("send_email", &executor.SendEmailExecutor{})
	registry.Register("create_profile", &executor.CreateProfileExecutor{})

	// Initialize task consumer
	taskConsumer, err := consumer.NewTaskConsumer(
		brokers,
		"worker-group-4", // New group ID for fresh rebalance
		"workflow_tasks",
		registry,
		resProducer,
		store,
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
