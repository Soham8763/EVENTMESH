package monitor

import (
	"context"
	"database/sql"
	"time"

	"eventmesh/pkg/logger"
	"eventmesh/pkg/metrics"

	"go.uber.org/zap"
)

type StuckChecker struct {
	db       *sql.DB
	interval time.Duration
	timeout  time.Duration
}

func NewStuckChecker(db *sql.DB) *StuckChecker {
	return &StuckChecker{
		db:       db,
		interval: 30 * time.Second,
		timeout:  5 * time.Minute,
	}
}

func (sc *StuckChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(sc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.check()
		}
	}
}

func (sc *StuckChecker) check() {
	var count float64

	err := sc.db.QueryRow(`
		SELECT COUNT(*)
		FROM workflow_executions
		WHERE status = 'RUNNING'
		  AND updated_at < now() - interval '5 minutes'
	`).Scan(&count)

	if err != nil {
		logger.Log.Error("stuck checker query failed", zap.Error(err))
		return
	}

	metrics.StuckWorkflows.Set(count)

	if count > 0 {
		logger.Log.Warn("stuck workflows detected",
			zap.Float64("count", count))
	}
}
