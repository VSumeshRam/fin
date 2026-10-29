# Enterprise AI FinOps Gateway — Production Architecture

This document outlines the verified, production architecture of the **Enterprise AI FinOps Gateway**, correcting the topology, security boundaries, and pipeline flow.

---

## 1. Architectural Audit: Flaws in the Legacy / Initial Diagram

An initial high-level diagram suggested:
`Autonomous Agents -> Go L7 Gateway -> [Redis, FastAPI Sidecar, Cloud LLM] -> Go Proxy -> Client`

This representation had 5 fundamental architectural inaccuracies:
1. **Separated Gateway vs. Proxy:** Autonomous Agents **are** the clients. The Go L7 Gateway is an integrated Layer 7 reverse proxy whose middleware pipeline wraps the outbound proxy handler. There are not two separate gateways.
2. **Zero-Trust Violation (PII):** Redis does **NOT** store PII maps. In [`internal/middleware/pii_masker.go`](../internal/middleware/pii_masker.go), PII tokenization and unmasking are strictly **in-memory within the Go request context (`r.Context()`)**. Persisting PII to Redis would violate GDPR/HIPAA.
3. **Misplaced Semantic Cache:** The Python FastAPI sidecar does **not** perform semantic caching; it is a stateless helper generating vector embeddings and extracting normalized entities. The actual **Semantic Cache engine** (vector search, cosine similarity $\ge 0.90$, entity matching, and Redis caching) runs inside the Go Gateway middleware for microsecond latency.
4. **Omission of Core Security Controls:** The diagram lacked the **MCP Tool-Level Firewall** (intercepting unauthorized tool executions like `drop_database` with `403 Forbidden`) and the **Streaming Guillotine** (in-flight SSE scanner severing TCP sockets on data leaks with `451`).
5. **Missing Observability & Control Plane:** Lacked the **single-writer thread-safe asynchronous ledger worker** (`audit_logs.jsonl`) and the **Executive React Web Dashboard** running on port 3000.

---

## 2. Corrected Production Component Architecture

```mermaid
flowchart TD
    %% Styling
    classDef client fill:#0284c7,stroke:#0369a1,stroke-width:2px,color:#fff
    classDef gateway fill:#0f172a,stroke:#38bdf8,stroke-width:2px,color:#fff
    classDef middleware fill:#1e293b,stroke:#64748b,stroke-width:1px,color:#f8fafc
    classDef security fill:#881337,stroke:#f43f5e,stroke-width:2px,color:#fff
    classDef cache fill:#064e3b,stroke:#10b981,stroke-width:2px,color:#fff
    classDef storage fill:#7c2d12,stroke:#f97316,stroke-width:2px,color:#fff
    classDef sidecar fill:#4c1d95,stroke:#8b5cf6,stroke-width:2px,color:#fff
    classDef upstream fill:#1e1b4b,stroke:#6366f1,stroke-width:2px,color:#fff
    classDef ui fill:#042f2e,stroke:#14b8a6,stroke-width:2px,color:#fff

    %% Client Tier
    subgraph ClientTier ["Autonomous AI Agent Tier (Inbound Clients)"]
        Agent["Autonomous Agents<br/>(CrewAI / LangGraph / Custom SDK)"]:::client
    end

    %% Core Go Gateway
    subgraph GoGateway ["Enterprise FinOps Gateway (Go L7 Reverse Proxy :8080)"]
        %% Edge Pipeline
        AuditLogger["Audit Logger Middleware<br/>(Injects AuditRecord Pointer)"]:::middleware
        InboundAdapter["Universal Inbound Adapter<br/>(StandardPayload Decoupling)"]:::middleware
        BudgetGuard["Budget Guard Middleware<br/>(Atomic Token Burn Enforcement)"]:::middleware
        LoopBreaker["Loop Breaker Middleware<br/>(Sliding Window 429 Throttle)"]:::middleware
        StateReplay["State Replay Middleware<br/>(Non-Streaming Idempotency Cache)"]:::cache
        
        BranchRouter{"Branch Router<br/>(payload.Stream?)"}:::gateway

        %% Non-Streaming Branch
        subgraph NonStreamingChain ["Non-Streaming Branch"]
            PIIMaskerNS["PII Masker<br/>(Zero-Trust In-Memory Masking)"]:::middleware
            MCPFirewall["MCP Tool Firewall<br/>(RBAC Tool Interception: 403 Forbidden)"]:::security
            SemanticCache["Semantic Cache Engine<br/>(Cosine Sim >= 0.90 + Entity Gate)"]:::cache
        end

        %% Streaming Branch
        subgraph StreamingChain ["Streaming Branch (SSE)"]
            PIIMaskerS["PII Masker<br/>(In-Memory Stream Tokenizer)"]:::middleware
            Guillotine["Streaming Guillotine Interceptor<br/>(In-Flight Scanner -> Socket Sever 451)"]:::security
        end

        OutboundProxy["Outbound Proxy & Stream Translator<br/>(StreamingResponseWriter Buffer)"]:::gateway
    end

    %% Auxiliary Storage & ML
    subgraph AuxiliaryTier ["State & Intelligence Infrastructure"]
        Redis[("Redis Cluster / In-Memory<br/>- Token Budgets<br/>- Loop Counters<br/>- Idempotency Hashes<br/>- Vector Cache List")]:::storage
        MLSidecar["Python FastAPI ML Sidecar (:8000)<br/>- Dense Vector Embeddings<br/>- Title-Case Entity Extraction"]:::sidecar
    end

    %% Upstream Inference Providers
    subgraph UpstreamTier ["Cloud LLM Inference Providers"]
        CloudLLM["Cloud Inference APIs<br/>(OpenAI / Anthropic / OpenRouter)"]:::upstream
    end

    %% Observability & UI
    subgraph ObservabilityTier ["Observability & Control Plane"]
        LedgerWorker["Async Ledger Worker<br/>(Single-Writer Buffered Channel)"]:::middleware
        LedgerFile[("audit_logs.jsonl<br/>Thread-Safe Sequential Append")]:::storage
        Dashboard["Executive Web Dashboard (:3000)<br/>(React 18 + Tailwind + Recharts)"]:::ui
    end

    %% Request Inbound Flow
    Agent -->|"1. HTTP POST (OpenAI / Anthropic Format)"| AuditLogger
    AuditLogger --> InboundAdapter
    InboundAdapter --> BudgetGuard
    BudgetGuard --> LoopBreaker
    LoopBreaker --> StateReplay
    StateReplay --> BranchRouter

    %% Routing
    BranchRouter -->|"stream: false"| PIIMaskerNS
    PIIMaskerNS --> MCPFirewall
    MCPFirewall --> SemanticCache
    SemanticCache --> OutboundProxy

    BranchRouter -->|"stream: true"| PIIMaskerS
    PIIMaskerS --> Guillotine
    Guillotine --> OutboundProxy

    %% Outbound Forwarding
    OutboundProxy -->|"2. Forward Normalized Payload"| CloudLLM
    CloudLLM -->|"3. Streaming SSE / JSON Response"| OutboundProxy

    %% Middleware Interactivity
    BudgetGuard <-->|"Atomic Balance Deductions"| Redis
    LoopBreaker <-->|"Sliding Window Counts"| Redis
    StateReplay <-->|"Idempotency Checkpoints"| Redis
    SemanticCache <-->|"Fetch Vector Entries"| Redis
    SemanticCache <-->|"Vectorize & Extract Entities"| MLSidecar

    %% Security Interventions
    MCPFirewall -.->|"Rogue Tool Call (drop_database) -> 403 Forbidden"| Agent
    Guillotine -.->|"Data Leak / Loop -> Cancel TCP Socket (451)"| Agent

    %% Ledger & UI Flow
    AuditLogger -.->|"Non-Blocking Channel Send"| LedgerWorker
    LedgerWorker -->|"Sequential File Flush"| LedgerFile
    Dashboard <-->|"Poll Budgets"| Redis
    Dashboard <-->|"Read Historical Telemetry"| LedgerFile
```

---

## 3. Technology Stack Reference

| Component | Technology | Role |
|---|---|---|
| **L7 Core Proxy** | Go 1.23+ (`net/http`) | Universal decoupled schema translation and high-concurrency middleware pipeline. |
| **State & Policy Store** | Redis v9 / Miniredis | Distributed token quota tracking, loop breaker sliding window counters, idempotency cache, and vector entry storage. |
| **Zero-Trust Privacy** | Go Context (`r.Context()`) | In-memory tokenization and unmasking of PII (SSNs, emails); zero persistence to disk or Redis. |
| **Semantic Intelligence** | Python FastAPI (`ml-sidecar`) | Dense text embeddings, cosine distance normalization, and entity extraction. |
| **Agent Tool Governance** | MCP Firewall & JSON RBAC | Intercepts rogue model tool execution payloads (`drop_database`) before reaching agent runtime. |
| **Streaming Protection** | Streaming Guillotine (`http.Flusher`) | In-flight SSE character scanning; severs upstream TCP sockets on hallucination loops or data leaks. |
| **Audit Persistence** | Async Single-Writer Ledger | Thread-safe Go channel-based sequential logging to `audit_logs.jsonl` with zero HTTP latency penalty. |
| **Executive Control Plane** | React 18 + Tailwind CSS + Recharts | Real-time browser dashboard on port `3000` with live KPI cards, quota warning alerts, and perimeter telemetry log. |
