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

	// Failure Visibility Metrics
	RetryCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "workflow_retries_total",
			Help: "Total retries executed",
		},
	)

	StuckWorkflows = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "stuck_workflows",
			Help: "Number of workflows stuck in RUNNING state",
		},
	)

	WorkerHeartbeat = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_heartbeat_timestamp",
			Help: "Last worker heartbeat Unix timestamp",
		},
	)

	// New Metrics (Phase 7)
	EventsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventmesh_events_processed_total",
			Help: "Total number of processed events",
		},
		[]string{"type"},
	)

	WorkflowExecutions = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "eventmesh_workflow_executions_total",
			Help: "Total workflow executions",
		},
	)

	StepExecutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventmesh_step_executions_total",
			Help: "Total step executions",
		},
		[]string{"step", "status"},
	)

	WorkerThroughput = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eventmesh_worker_throughput_total",
			Help: "Total execution throughput by worker",
		},
		[]string{"worker_id", "capability"},
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
		RetryCount,
		StuckWorkflows,
		WorkerHeartbeat,
		EventsProcessed,
		WorkflowExecutions,
		StepExecutions,
		WorkerThroughput,
	)
}
