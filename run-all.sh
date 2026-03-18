#!/bin/bash

# EventMesh ALL Services Runner
# Kills existing processes and restarts all core services

set -e

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
LOGS_DIR="$SCRIPT_DIR/logs"

# Create logs directory
mkdir -p "$LOGS_DIR"

echo "🔄 Stopping existing services..."

# Kill ALL Go run processes related to eventmesh services
pkill -9 -f "go run services/" 2>/dev/null || true
pkill -9 -f "go-build.*services/" 2>/dev/null || true

# Kill processes on standard ports
lsof -ti :8080,8081,2112,2113,2114,2115 | xargs kill -9 2>/dev/null || true

sleep 2

echo "✅ Ports cleared"

# Clear old logs
for log in auth-service event-ingestor orchestrator worker projector; do
    > "$LOGS_DIR/$log.log"
done

# Track all child PIDs
PIDS=()

echo "🚀 Starting auth-service on :8081..."
go run services/auth-service/cmd/main.go > "$LOGS_DIR/auth-service.log" 2>&1 &
PIDS+=($!)

sleep 1

echo "🚀 Starting event-ingestor on :8080 (Metrics: 2112)..."
go run services/event-ingestor/cmd/main.go > "$LOGS_DIR/event-ingestor.log" 2>&1 &
PIDS+=($!)

echo "🚀 Starting workflow-orchestrator (Metrics: 2113)..."
go run services/workflow-orchestrator/cmd/main.go > "$LOGS_DIR/orchestrator.log" 2>&1 &
PIDS+=($!)

echo "🚀 Starting worker (Metrics: 2114)..."
go run services/worker/cmd/main.go > "$LOGS_DIR/worker.log" 2>&1 &
PIDS+=($!)

echo "🚀 Starting state-projector (Metrics: 2115)..."
go run services/state-projector/cmd/main.go > "$LOGS_DIR/projector.log" 2>&1 &
PIDS+=($!)

echo ""
echo "═══════════════════════════════════════════"
echo "  EventMesh Services Running"
echo "═══════════════════════════════════════════"
echo "  Auth Service:        :8081"
echo "  Event Ingestor:      :8080"
echo "  Orchestrator Metrics: :2113"
echo "  Worker Metrics:       :2114"
echo "  Projector Metrics:    :2115"
echo "═══════════════════════════════════════════"
echo ""
echo "📋 Logs available in ./logs/"
echo "Press Ctrl+C to stop all services"

# Cleanup function that kills ALL related processes
cleanup() {
    echo ''
    echo '🛑 Stopping services...'
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    # Also kill any Go-compiled child binaries
    pkill -9 -f "go run services/" 2>/dev/null || true
    pkill -9 -f "go-build.*services/" 2>/dev/null || true
    echo '✅ All services stopped'
    exit 0
}

trap cleanup SIGINT SIGTERM

wait

