package main

import (
	"log"
	"net/http"

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

	// Simple health endpoint to keep service alive
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	log.Println("rule-engine running on :8083")
	log.Fatal(http.ListenAndServe(":8083", nil))
}
