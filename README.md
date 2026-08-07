# Enterprise AI FinOps Gateway 🛡️💸

> A high-throughput, deterministic Go reverse proxy designed to secure, audit, and financially govern autonomous Agentic AI traffic.

Autonomous AI agents introduce massive infrastructure risks: infinite retry loops that drain API budgets, unauthorized Model Context Protocol (MCP) tool executions, and mid-stream PII data leaks. 

The **FinOps Gateway** acts as a zero-trust network firewall and tollbooth. Placed between your internal applications and upstream LLMs (OpenAI, Anthropic), it intercepts traffic at the network layer to enforce financial budgets, mask PII, and sever rogue TCP connections mid-hallucination.

## 🏗️ System Architecture

```text
       [ Autonomous AI Agent / Internal App ]
                        │
                        ▼ (HTTP / SSE)
┌───────────────────────────────────────────────────────────────┐
│ Enterprise FinOps Gateway (Go)                                │
│                                                               │
│ 1. Budget Guard (Atomic Redis Lua Decrement)                  │
│ 2. Loop Breaker (SHA-256 Sliding Window)                      │
│ 3. State Replay (Idempotent Fast-Forwarding)                  │
│ 4. PII Masker (Bidirectional Regex Scrubber)                  │
│ 5. MCP Tool Firewall (JSON RBAC Interceptor)                  │
│ 6. Entity-Gated Cache (Vector + NER verification) ◄──┐        │
│ 7. Streaming Kill Switch (http.Flusher / TCP Cut)    │        │
└───────────────────────┬──────────────────────────────┼────────┘
                        │                              ▼
                        │                  ┌──────────────────────┐
                        │                  │ Python ML Sidecar    │
                        ▼                  │ (FastAPI Ext-Proc)   │
             [ Upstream LLM / OpenAI ]     └──────────────────────┘
```

## 🚀 Key Capabilities

### Phase V1: The Deterministic Foundation
**Atomic Budget Guard**: Executes single-threaded Lua scripts in Redis to strictly enforce X-Team-ID token budgets, eliminating TOCTOU (Time-of-Check to Time-of-Use) race conditions.

**Circuit Breaking (Loop Prevention)**: Maintains a Redis sliding window of SHA-256 prompt hashes. Automatically trips a 429 Too Many Requests circuit breaker if an agent enters an infinite retry loop.

**Asynchronous Audit Ledger**: Utilizes Go worker pools and memory channels to flush request telemetry to PostgreSQL without blocking the live HTTP proxy bridge.

### Phase V2: The Semantic Layer (Ext-Proc Pattern)
**Entity-Gated Semantic Cache**: Outbound prompts are evaluated by a Python ML Sidecar. Cache hits are only validated if Cosine Similarity is >= 0.95 AND Named Entity Recognition (NER) matrices perfectly match, eliminating false-positive data contamination.

**Bidirectional PII Masker**: Dynamically mutates payload structs to intercept and tokenize PII (Emails, SSNs) prior to upstream routing, and utilizes a custom ResponseRecorder to unmask the data before returning it to the client.

### Phase V3: The Agentic Firewall
**MCP Tool-Level RBAC**: Inspects upstream JSON payloads for tool_calls. Blocks unauthorized internal tool executions based on strict team policies.

**Streaming Token Kill Switch**: Wraps the standard library http.ResponseWriter with an http.Flusher. Scans Server-Sent Events (SSE) in real-time. If a data leak or hallucination loop is detected, it injects a clean finish_reason: content_filter chunk and triggers a context.CancelFunc to sever the TCP socket instantly.

**Idempotent State Replay**: Hashes complete conversation arrays. If an agent crashes at step 14, the gateway intercepts the retries and fast-forwards the agent through steps 1-13 using cached tool payloads, reducing retry compute costs to $0.

## 🛠️ Tech Stack
**Core**: Go (net/http, httputil)

**ML Sidecar**: Python, FastAPI, spaCy (NER), Sentence-Transformers

**State & Caching**: Redis 7 (In-Memory Hot Path)

**Audit Ledger**: PostgreSQL 15 (Cold Path Storage)

**Deployment**: Docker Compose

## ⚡ Quick Start
```bash
# 1. Spin up the infrastructure (Postgres, Redis, ML Sidecar)
docker-compose up -d

# 2. Run the Go Proxy Server
go run cmd/gateway/main.go

# 3. Execute the automated verification harness
bash scripts/test_v3.sh
```
