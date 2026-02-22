# Interview Cheat Sheet — EventMesh

This guide prepares you to answer deep technical questions about EventMesh during software engineering interviews (Backend / Distributed Systems).

## Core Concepts & FAQ

### 1. Why did you choose this architecture?

"I wanted to build a system that was truly decoupled and resilient. By using an event-driven architecture with a Kafka backbone, I ensured that no single service failure could bring down the entire pipeline. It mirrors the reliable ingestion and orchestration patterns used at companies like Uber and Stripe."

### 2. What was the biggest challenge?

"Handling state consistency in a distributed environment. I had to ensure that if the Orchestrator crashed, the workflow state wouldn't be lost or duplicated. I solved this by implementing a **transactional state machine** in PostgreSQL, ensuring atomicity between state updates and task emissions."

### 3. How do you handle duplicate events?

"I implemented **dual-layer idempotency**. Layer 1 is at the ingestion gateway using Redis (fast check/set with TTL). Layer 2 is at the worker level using Redis `SetNX` for distributed locking. This prevents duplicates from both network retries and internal consumer re-deliveries."

### 4. How would you scale this to 10x traffic?

"The system scales horizontally by design. I would increase the Kafka partition count and add more instances of the ingestors and workers. I'd also consider sharding the PostgreSQL database by `tenant_id` to prevent it from becoming a monolithic bottleneck."

### 5. What are the failure scenarios?

- **Worker Crash**: Handled by Redis leases (30s TTL) and a stuck workflow monitor.
- **Broker Downtime**: Handled by client-side reconnection and Kafka's durable persistence.
- **Dependency Loss**: The system degrades gracefully; if Redis is down, ingestion fails with a 500, preserving data integrity over availability.

## Design Tradeoffs to Highlight

- **Consistency vs. Latency**: We chose strong consistency (transactional updates) over sub-millisecond latency.
- **Statelessness**: Services are stateless, offloading complexity to Kafka, Redis, and Postgres.
- **JSON over Protobuf**: Chose readability and ease of debugging for v1, with a roadmap to move to Protobuf for performance.

## Key Stats to Reference

- **58 events/sec** sustained throughput.
- **p99 latency < 85ms**.
- **100% accuracy** in duplicate prevention during stress tests.
- **Zero data loss** throughout chaos testing.
