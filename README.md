# EventMesh

EventMesh is a distributed, event-driven backend platform designed to reliably
ingest, validate, and durably persist high volumes of events in a multi-tenant
environment.

The system is architected with scalability, fault tolerance, and observability
as first-class concerns, following real-world patterns used in large-scale
production systems.

---

## 🎯 Problem Statement

Modern backend systems generate and consume large volumes of events
(user actions, webhooks, integrations, async jobs).

Common challenges include:
- Tight coupling between services
- Duplicate event processing due to retries
- Loss of events during failures
- Poor observability of event flows
- Difficulty scaling ingestion pipelines

EventMesh addresses these problems by providing a **reliable event ingestion
and routing layer** that decouples producers from downstream processing.

---

## 🧠 High-Level Architecture

EventMesh follows an **event-driven, microservice-based architecture**:

Client
|
| HTTP Events
v
Event Ingestor
| (auth, validation, idempotency)
v
Event Bus (Redpanda / Kafka)
|
v
Downstream Consumers (Rule Engine, Orchestrator, etc.)

markdown
Copy code

Key architectural principles:
- Stateless ingestion services
- Durable event storage
- Idempotent request handling
- Tenant isolation
- Asynchronous processing boundaries

---

## 🧱 Current Implementation Status

### ✅ Stage 1 — Foundation & Event Ingestion (Completed)

The following components are fully implemented:

#### Auth Service
- API key–based authentication
- Multi-tenant support
- PostgreSQL-backed tenant and API key storage

#### Event Ingestor
- HTTP-based event ingestion
- Request validation
- Event enrichment with:
  - `event_id`
  - `tenant_id`
  - `request_id`
  - `occurred_at`
  - `received_at`
  - `idempotency_key`
- Redis-backed idempotency enforcement
- Durable event publishing to Redpanda (Kafka-compatible)
- ACK returned only after broker persistence

#### Infrastructure
- PostgreSQL (state & metadata)
- Redis (idempotency)
- Redpanda (event bus)
- Docker Compose for local development

---

## 🔐 Event Guarantees

EventMesh provides the following guarantees at the ingestion layer:

- **At-least-once delivery**
- **Idempotent processing**
- **Tenant-scoped isolation**
- **Durable persistence before acknowledgment**
- **Safe handling of client retries**

---

## 📦 Event Envelope

All accepted events are converted into a canonical internal representation:

```json
{
  "event_id": "uuid",
  "event_type": "string",
  "tenant_id": "uuid",
  "request_id": "uuid",
  "occurred_at": "timestamp",
  "received_at": "timestamp",
  "idempotency_key": "string",
  "payload": { }
}
This envelope is the contract trusted by all downstream services.

🚀 Roadmap
Upcoming stages:

Stage 2: Rule Engine (event → workflow intent mapping)

Stage 3: Workflow Orchestrator (stateful, crash-safe execution)

Stage 4: Execution Workers (horizontal step execution)

Stage 5: Observability & Reliability

Stage 6: Scaling, load testing, and hardening

🛠 Tech Stack
Language: Go

Event Bus: Redpanda (Kafka-compatible)

Database: PostgreSQL

Cache: Redis

Containerization: Docker / Docker Compose

🧠 Design Philosophy
EventMesh is intentionally built as an infrastructure platform, not a UI
application.

The focus is on:

Correctness over convenience

Explicit failure handling

Clear service boundaries

Production-grade design decisions

📌 Note
This project is being developed incrementally, with each stage designed to
mirror how real backend platforms evolve in production environments.