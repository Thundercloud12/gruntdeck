# Gruntdeck

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine built in Go. It offers a secure, concurrent, and highly performant orchestration engine to execute multi-step automation workflows across remote nodes over SSH, powered by a 100% PostgreSQL persistence layer and decoupled background job queue (**River**).

---

## Architecture & Layering

The system is strictly layered following the **Single Responsibility Principle (SRP)**. Every layer performs a distinct role without knowing the internal implementation details of adjacent layers.

```
Producer CLI
      │ (Calls QueueService.Enqueue)
      ▼
Queue Package (internal/queue)
      │ (Creates Execution & Inserts River Jobs)
      ▼
River (PostgreSQL Job Queue Engine)
      │ (Dispatches ExecuteJobArgs tasks to Worker)
      ▼
Worker (internal/queue/worker.go)
      │ (Delegates execution)
      ▼
Execution Service (internal/execution/service.go)
      │ (Orchestrates target steps & status tracking)
      ▼
Repositories (internal/repository)
      │ (PostgreSQL DDL & Query Persistence)
      ▼
Postgres (Database Storage)
      │ (Pipes commands to remote nodes)
      ▼
SSH (internal/ssh)
```

### Detailed Layer Breakdown

```mermaid
flowchart TD
    subgraph Layer1 ["1. Producer CLI (cmd/producer)"]
        PROD["producer <job-id>"]
    end

    subgraph Layer2 ["2. Queue Package (internal/queue)"]
        QS["QueueService.Enqueue(ctx, jobID)"]
    end

    subgraph Layer3 ["3. River Engine (PostgreSQL Queue)"]
        RIVER[("River Queue<br/>river_job table")]
    end

    subgraph Layer4 ["4. Worker (internal/queue/worker.go)"]
        WORKER["Worker.Work(ctx, job)"]
    end

    subgraph Layer5 ["5. Execution Service (internal/execution)"]
        EXEC_SERVICE["ExecutionService.ExecuteTarget(...)"]
        STEPS["Step Handlers<br/>(command / script / file-copy / job-ref)"]
    end

    subgraph Layer6 ["6. Repositories (internal/repository)"]
        REPOS["JobRepository | InventoryRepository<br/>ExecutionRepository | LogRepository"]
    end

    subgraph Layer7 ["7. Postgres Database"]
        PG[(PostgreSQL Instance)]
    end

    subgraph Layer8 ["8. SSH Layer (internal/ssh)"]
        SSH_CLIENT["SSH Workers & Keepalive Loops"]
        FLEET["Target Nodes (Fleet)"]
    end

    %% Flow Connections
    PROD -->|Triggers| QS
    QS -->|Creates Execution Record & Inserts Tasks| RIVER
    RIVER -->|Dispatches Task| WORKER
    WORKER -->|Delegates Execution| EXEC_SERVICE
    EXEC_SERVICE --> STEPS
    STEPS -->|Load Models & Save Logs| REPOS
    REPOS -->|pgxpool SQL Queries| PG
    STEPS -->|Executes Remote Commands| SSH_CLIENT
    SSH_CLIENT --> FLEET
```

---

## Single Responsibility Principles (SRP)

| Layer | Component | Responsibility |
| :--- | :--- | :--- |
| **1. Producer CLI** | `cmd/producer` | Reads CLI args, loads `.env`, and calls `QueueService.Enqueue(ctx, jobID)`. |
| **2. Queue Package** | `internal/queue` | Loads targets, creates an `Execution` record, and inserts tasks into River atomically using `InsertManyTx`. |
| **3. River Queue** | `riverqueue/river` | Manages background task persistence, worker polling, locking, and retries inside PostgreSQL. |
| **4. Worker** | `internal/queue/worker.go` | Implements `river.Worker[ExecuteJobArgs]`, bridging River queue tasks to `ExecutionService.ExecuteTarget`. |
| **5. Execution Service**| `internal/execution` | Handles target step execution pipelines, step type parsing, and log recording. |
| **6. Repositories** | `internal/repository` | Encapsulates SQL queries and data mapping for jobs, targets, executions, and log entries. |
| **7. Postgres** | PostgreSQL | Stores persistent state, jobs, execution history, log entries, and River task queues. |
| **8. SSH Layer** | `internal/ssh` | Handles OpenSSH key authentication, strict `known_hosts` verification, command streaming, and keepalives. |

---

## Database Schema (PostgreSQL DDL)

Application schema migrations and River queue tables are automatically initialized on startup:

```sql
-- Target Inventory Table
CREATE TABLE IF NOT EXISTS targets (
    id VARCHAR(255) PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL DEFAULT '22',
    "user" TEXT NOT NULL,
    key_path TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}'
);

-- Jobs Table
CREATE TABLE IF NOT EXISTS jobs (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    target_filter TEXT[] NOT NULL DEFAULT '{}'
);

-- Job Steps Table
CREATE TABLE IF NOT EXISTS job_steps (
    id VARCHAR(255) PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb
);

-- Execution Tracking Table
CREATE TABLE IF NOT EXISTS executions (
    id VARCHAR(255) PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    targets_total INT NOT NULL DEFAULT 0,
    targets_succeeded INT NOT NULL DEFAULT 0,
    targets_failed INT NOT NULL DEFAULT 0
);

-- Execution Logs Table
CREATE TABLE IF NOT EXISTS log_entries (
    id VARCHAR(255) PRIMARY KEY,
    execution_id VARCHAR(255) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL
);
```

---

## Getting Started

### Prerequisites
* Go 1.25+
* PostgreSQL database instance (local PostgreSQL container or managed service)
* Target nodes with SSH enabled and authorized public keys configured.

### Compilation
Build the `producer`, `executor`, and `gruntdeck` binaries:
```bash
go build -o producer ./cmd/producer
go build -o executor ./cmd/executor
go build -o gruntdeck ./cmd/gruntdeck
```

---

## Environment Configuration (`.env`)

Create a `.env` file in your root working directory:
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/gruntdeck?sslmode=disable
```

---

## Workflow & CLI Usage

### 1. Managing Trusted Hosts
To add a host to Gruntdeck's trusted list (`.gruntdeck/known_hosts`):
```bash
$ ./gruntdeck trust 192.168.1.100
```

### 2. Starting the Executor Worker Daemon
Start the long-running worker daemon in the background or terminal window:
```bash
$ ./executor
🐘 Connecting to PostgreSQL database...
Database schema is up to date.
🚀 River Queue Worker started. Waiting for execution jobs...
```

### 3. Enqueuing Jobs with the Producer
Trigger a job execution using the `producer` CLI:
```bash
$ ./producer deploy-app
🚀 Successfully enqueued Job 'deploy-app' across 1 target nodes.
Execution ID: e80f2d48-831e-450a-8bf8-d897f26d2e05
```
