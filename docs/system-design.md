# System Design Document — EventMesh

This document captures the high-level architectural decisions and the underlying rationale for the EventMesh design.

## Core Architectural Decisions

### 1. Why Kafka (Redpanda) instead of RabbitMQ/Redis Streams?

We chose a Kafka-compatible broker (Redpanda) for its **reproducible streaming** and **partition-based scaling**. Unlike RabbitMQ, Kafka allows us to re-read messages from a specific offset, which is critical for recovering workflow state after a crash. Redpanda was specifically chosen for its zero-dependency, single-binary architecture, making it faster to deploy and more performant in local environments.

### 2. Why Redis for Coordination?

While Postgres is our source of truth, the high-frequency "check-and-set" operations (idempotency, locking, leases) are offloaded to Redis. This prevents Postgres from becoming a bottleneck during high ingestion volumes and keeps the core DB transactions focused on long-lived workflow state.

### 3. Async Workflow Orchestration

EventMesh uses an **asynchronous dispatch** model. The Orchestrator does not wait for a worker to finish; it simply emits a task and moves on. This allows the system to handle thousands of concurrent workflows without blocking threads, scaling only indexed by Kafka partitions.

## Scaling Model

- **Ingestion**: Scaled horizontally via load-balanced stateless ingestors.
- **Workflow State**: Scaled by increasing Kafka partition count for the `workflow_triggers` and `workflow_tasks` topics.
- **Persistence**: Currently a single instance, but ready for read-replicas for metrics/monitoring queries.

## Bottlenecks & Failure Strategy

| Component | Potential Bottleneck | Failure Strategy |
|-----------|----------------------|------------------|
| **Ingestor** | Network I/O, Auth latency | Circuit breakers on Auth call, horizontal scaling |
| **Kafka** | Disk I/O, Partition imbalance | Increase partition count, disk monitoring |
| **Postgres** | Write throughput | Connection pooling, transaction optimization |
| **Worker** | Business logic (e.g., API calls) | Leases + retries with exponential backoff |

## Future Redesign Considerations

If we were to scale to millions of events per second, we would:

1. Move from JSON to **Protobuf** for more efficient serialization.
2. Implement **sharded Postgres** based on `tenant_id`.
3. Switch from synchronous Auth calls in the ingestor to an **async JWT-based auth** model.
