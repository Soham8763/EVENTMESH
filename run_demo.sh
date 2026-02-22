#!/bin/bash

# EventMesh Demo Script
# This script starts the entire EventMesh system and runs a sample transaction.

set -e

# ANSI color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================================================${NC}"
echo -e "${BLUE}          EventMesh — Distributed Workflow Demo                 ${NC}"
echo -e "${BLUE}================================================================${NC}"

# 1. Check Prerequisites
echo -e "\n${YELLOW}[1/5] Checking prerequisites...${NC}"
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed."
    exit 1
fi
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed."
    exit 1
fi

# 2. Start Infrastructure
echo -e "\n${YELLOW}[2/5] Starting infrastructure (Postgres, Redis, Redpanda)...${NC}"
cd deployments
docker compose up -d
cd ..

echo -e "${GREEN}Infrastructure is up! Waiting for Redpanda to be ready...${NC}"
sleep 10

# 3. Initialize Database & Topics
echo -e "\n${YELLOW}[3/5] Initializing database and Kafka topics...${NC}"
# Note: Assuming psql and rpk are available or can be run via docker
docker exec -i eventmesh-postgres psql -U eventmesh -d eventmesh < setup_db.sql || true
docker exec eventmesh-redpanda rpk topic create events workflow_triggers workflow_tasks workflow_task_results system_failures || true

# 4. Start Services
echo -e "\n${YELLOW}[4/5] Starting EventMesh services in background...${NC}"
mkdir -p logs

# Start all 5 services
# Using 'nohup' to keep them running in background for the demo
nohup go run services/auth-service/cmd/main.go > logs/auth-service.log 2>&1 &
echo -e "  - Auth Service started [port :8081]"
nohup go run services/event-ingestor/cmd/main.go > logs/event-ingestor.log 2>&1 &
echo -e "  - Event Ingestor started [port :8080]"
nohup go run services/rule-engine/cmd/main.go > logs/rule-engine.log 2>&1 &
echo -e "  - Rule Engine started"
nohup go run services/workflow-orchestrator/cmd/main.go > logs/workflow-orchestrator.log 2>&1 &
echo -e "  - Workflow Orchestrator started"
nohup go run services/worker/cmd/main.go > logs/worker.log 2>&1 &
echo -e "  - Worker Node started"

sleep 5 # Wait for services to bind ports

# 5. Run Demo Transaction
echo -e "\n${YELLOW}[5/5] Sending test event (user_signed_up)...${NC}"
IDEMPOTENCY_KEY="demo-$(date +%s)"
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -H "X-API-Key: demo-api-key" \
  -H "Idempotency-Key: ${IDEMPOTENCY_KEY}" \
  -d '{
    "event_type": "user_signed_up",
    "payload": { "user_id": "u-demo-1", "email": "demo@eventmesh.io" }
  }' | jq .

echo -e "\n${GREEN}Success! Transaction processed through the mesh.${NC}"
echo -e "You can monitor the full execution trace in the logs:"
echo -e "  ${BLUE}tail -f logs/*.log${NC}"
echo -e "\nTo stop all services, run: ${YELLOW}pkill -f 'go run'${NC}"
echo -e "${BLUE}================================================================${NC}"
