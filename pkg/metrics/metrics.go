package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// Worker Metrics
	TasksProcessed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "tasks_processed_total",
			Help: "Total tasks executed",
		},
	)

	TaskFailures = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "task_failures_total",
			Help: "Total failed tasks",
		},
	)

	TaskDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "task_duration_seconds",
			Help:    "Task execution duration",
			Buckets: prometheus.DefBuckets,
		},
	)

	// Orchestrator Metrics
	WorkflowsStarted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "workflows_started_total",
			Help: "Total workflows started",
		},
	)

	WorkflowsCompleted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "workflows_completed_total",
			Help: "Total workflows completed",
		},
	)

	WorkflowsFailed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "workflows_failed_total",
			Help: "Total workflows failed",
		},
	)

	// Ingestor Metrics
	EventsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "events_received_total",
			Help: "Total events received",
		},
	)

	EventsRejected = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "events_rejected_total",
			Help: "Total events rejected",
		},
	)
)

func Init() {
	prometheus.MustRegister(
		TasksProcessed,
		TaskFailures,
		TaskDuration,
		WorkflowsStarted,
		WorkflowsCompleted,
		WorkflowsFailed,
		EventsReceived,
		EventsRejected,
	)
}
