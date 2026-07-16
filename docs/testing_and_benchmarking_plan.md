# EventMesh — Pre-Production Testing & Benchmarking Plan

This document details the strategy to benchmark, stress-test, and validate **EventMesh** at enterprise scale (10,000+ events/sec) before implementing Phase 5 (Enterprise Features) or making the platform publicly available.

---

## 1. Retaining the Core Developer Experience (DX)

A common mistake in scaling system infrastructure is making it harder for developers to use. **EventMesh preserves its core simplicity.**

None of our changes (async Kafka producers, offline JWT validation, atomic idempotency, or lease recovery) affect the developer-facing SDK:

```go
// The developer's workflow definition remains exactly the same:
workflow := sdk.NewWorkflow("order-processing").
    Step("reserve_inventory", ReserveInventory).
    Step("process_payment", ProcessPayment).
    Step("send_email", SendEmail)

// The worker registration is completely unchanged:
worker := sdk.NewWorker()
worker.Register("reserve_inventory", ReserveInventory)
```

### Why it is better now:
* **The "Redis-like" promise is now true**: Previously, a worker crash would result in a task being lost forever because the idempotency key was pre-locked. Now, with the `lease` + `done-key` pattern, a worker crash results in a safe, automatic retry by another worker.
* **Deterministic order**: The orchestrator now writes its initial state synchronously. The sleep hack is gone, meaning that workflows will start immediately and reliably, without flaky timing issues under load.

---

## 2. Load Testing & Benchmarking Strategy

To validate that EventMesh handles peak Big Tech loads (like Flipkart Big Billion Days or Amazon Prime Day), we will conduct benchmarks across three distinct bottlenecks:

```
[ Load Generators ] ──(HTTP Ingest)──> [ Ingestor ] ──(Async)──> [ Kafka ] ──> [ Orchestrator/Workers ]
```

### 2.1 Ingestion Throughput Benchmarking (k6 & Vegeta)
* **Goal**: Measure max request rate, p99 latency, and resource utilization (CPU/Memory/Network) of the ingestor node.
* **Tools**: **k6** or **Vegeta**.
* **Methodology**:
  1. Generate 50,000 requests/sec targeting `/events` using a distributed load test.
  2. Test **JWT Validation (Offline)** vs **API Key validation (Online/HTTP)**.
  3. Verify that the Async Producer doesn't run out of memory (heap pressure check) when Kafka latency spikes.

#### k6 Ingestion Test Script (`load_test.js`):
```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '1m', target: 1000 },  // Ramp up to 1000 users
    { duration: '3m', target: 10000 }, // Sustained stress load
    { duration: '1m', target: 0 },     // Cool down
  ],
};

export default function () {
  const url = 'http://localhost:8080/events';
  const payload = JSON.stringify({
    event_type: 'user_signed_up',
    payload: { user_id: 'user-' + __VU + '-' + __ITER, email: 'test@example.com' }
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer <JWT_TOKEN_HERE>',
      'Idempotency-Key': 'idemp-' + __VU + '-' + __ITER,
    },
  };

  let res = http.post(url, payload, params);
  check(res, {
    'status is 202': (r) => r.status === 202,
  });
}
```

### 2.2 End-to-End Execution Latency Benchmarking
* **Goal**: Measure the total time elapsed from event ingestion to the completion of the final workflow step.
* **Methodology**:
  1. Trigger 5,000 multi-step workflows simultaneously.
  2. Measure time-to-first-task-dispatch and time-to-workflow-completion.
  3. Monitor Prometheus metrics for consumer lag:
     - `kafka_consumergroup_lag` for topics `workflow_triggers`, `workflow_tasks`, and `workflow_task_results`.

---

## 3. Chaos & Resilience Testing (Validating the Safety Nets)

At Flipkart/Amazon scale, infrastructure failures are the norm, not the exception. We will validate our new resilience features using **Chaos Mesh** or **Toxiproxy** in a Kubernetes or Docker Compose environment.

### 3.1 Scenario A: Worker Crash Recovery (Lease Handover)
* **Hypothesis**: A worker crashes mid-execution; the task lease auto-expires, and the task is safely executed by another worker without duplication.
* **Action**:
  1. Run a 3-step workflow.
  2. Kill the worker pod exactly 2 seconds into executing `reserve_inventory`.
  3. Verify:
     - The Redis lease `lease:<task_id>` expires after 30 seconds.
     - The orchestrator's Stuck Checker detects the running state and resets the step to `PENDING`.
     - A second worker picks up the task, executes it, and the workflow finishes successfully.
     - The task is **never** skipped as a duplicate because the `done-key` was never written.

### 3.2 Scenario B: Redis Network Partitioning (Brain Split)
* **Hypothesis**: The worker loses connection to Redis during lease checks.
* **Action**:
  1. Inject packet drop (100% loss) between the worker and Redis using Toxiproxy.
  2. Verify:
     - The worker fails to acquire the lease, handles the error gracefully, and does **not** execute the task.
     - The Kafka message remains uncommitted and is safely consumed by a different worker.

### 3.3 Scenario C: Database Primary Failover
* **Hypothesis**: PostgreSQL primary crashes and triggers replica promotion.
* **Action**:
  1. Run a high volume of triggers (100 workflows/sec).
  2. Stop the PostgreSQL primary node.
  3. Verify:
     - The orchestrator transactions rollback cleanly.
     - No execution is left in a half-finished state.
     - Kafka offsets are not committed for failed state changes, allowing automatic retry once PostgreSQL is back online.

---

## 4. Production Release Checklist

Before marking the project public and moving to Phase 5, we must satisfy the following criteria:

| Category | Metric/Check | Target | Validation Method |
|:---|:---|:---|:---|
| **Performance** | Ingestion Latency | p99 < 5ms (under load) | k6 run |
| **Performance** | Ingestion Throughput | > 5,000 req/sec per node | k6 run |
| **Data Integrity** | Duplication Rate | Exactly 0% | Run 10x duplicate storm |
| **Data Integrity** | Event Loss Rate | Exactly 0% | Ingest 100k events; reconcile PG executions |
| **Resilience** | Worker Recovery | < 60s to resume task | Chaos script kill |
| **Resilience** | DB Failover | Zero orphaned records | Reconnection verification |
