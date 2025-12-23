package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"eventmesh/rule-engine/internal/consumer"
	"eventmesh/rule-engine/internal/repository"
)

func main() {
	log.Println("rule-engine starting...")

	db := repository.NewPostgres()
	ruleRepo := repository.NewRuleRepository(db)

	rules, err := ruleRepo.LoadActiveRules()
	if err != nil {
		log.Fatalf("failed to load rules: %v", err)
	}
	log.Printf("loaded %d active rules\n", len(rules))

	eventConsumer, err := consumer.NewEventConsumer(
		[]string{"localhost:19092"},
		"rule-engine-group",
		"events",
	)
	if err != nil {
		log.Fatalf("failed to create consumer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go eventConsumer.Start(ctx)

	log.Println("rule-engine consuming from 'events' topic")

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig

	log.Println("rule-engine shutting down")
}
