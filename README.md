# Gruntdeck

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine built in Go. It offers a secure, concurrent, and highly performant orchestration engine to execute multi-step automation workflows across remote nodes over SSH, powered by a 100% PostgreSQL persistence layer.

---

## Architecture Overview

Below is the complete system architecture of Gruntdeck, illustrating the PostgreSQL database layer, repository interfaces, embedded migration engine, SSH worker pool, and host-key verification mechanisms.

```mermaid
flowchart TD
    subgraph CLI ["Gruntdeck CLI & Execution Entry Points"]
        GT["gruntdeck CLI<br/>(trust / scan-host)"]
        EX["executor CLI<br/>(Job Runner)"]
        ENV[".env Configuration<br/>DATABASE_URL"]
    end

    subgraph Storage ["Persistence Layer (PostgreSQL)"]
        PG[(PostgreSQL Database<br/>via pgx/v5)]
        MIG["Embedded Go Migrations<br/>(internal/migrations)"]

        subgraph Repos ["Repository Interfaces (internal/repository)"]
            IR[InventoryRepository]
            JR[JobRepository]
            ER[ExecutionRepository]
            LR[LogRepository]
        end
    end

    subgraph Engine ["Orchestrator & Execution Engine"]
        ORCH[Orchestrator Engine]
        EXEC[Sequential Step Executor]
        
        subgraph StepTypes ["Step Handlers"]
            ST_CMD["Command Step"]
            ST_FILE["File Copy Step"]
            ST_SCR["Script Step (Auto Cleanup)"]
            ST_REF["Job Reference Step"]
        end
    end

    subgraph Security ["Security & Transport Layer"]
        KH[Strict Host-Key Verification<br/>.gruntdeck/known_hosts]
        KA[Keepalive Ping Loop<br/>15s Ping / 10s Timeout]
        SSH[Concurrent SSH Workers]
    end

    subgraph Fleet ["Target Infrastructure"]
        N1[Target Node 1]
        N2[Target Node 2]
        N3[Target Node N]
    end

    %% Flow Connections
    ENV --> EX
    EX -->|Auto Run Migrations| MIG
    MIG --> PG
    EX -->|Load Job & Targets| Repos
    GT -->|Manage Host Keys| KH
    
    Repos -->|pgx/v5 Pool| PG

    EX --> ORCH
    ORCH -->|Match Node Tags| EXEC
    EXEC --> StepTypes

    StepTypes --> Security
    Security --> SSH

    SSH -->|Concurrent SSH Connections| N1
    SSH -->|Concurrent SSH Connections| N2
    SSH -->|Concurrent SSH Connections| N3

    SSH -->|Real-Time Log Stream| LR
    EXEC -->|Track Run Status| ER
```

---

## Key Features

### 1. 100% PostgreSQL Database Native (`pgx/v5`)
* **Environment Configuration (`.env`)**: Automatically pulls `DATABASE_URL` from `.env` or system environment variables upon execution.
* **Embedded Database Migrations**: Uses Go's `embed` package to automatically run schema migrations and initial seed data on application startup.
* **Repository Pattern (`internal/repository`)**: Abstracted database interfaces (`InventoryRepository`, `JobRepository`, `ExecutionRepository`, `LogRepository`) backed by native PostgreSQL connection pooling (`pgxpool`).
* **Execution & Log Tracking**: Persists run metrics (`status`, `started_at`, `ended_at`, `targets_total`, `targets_succeeded`, `targets_failed`) and real-time step output log entries (`log_entries`).

### 2. Advanced Job Step Types
* **Command (`command`)**: Executes arbitrary shell commands on target nodes.
* **File Copier (`file-copy`)**: Transfers local configuration files or assets to remote nodes using high-efficiency SSH stdin piping (zero SFTP/rsync dependencies required).
* **Script Executor (`script`)**: Automatically copies local scripts to a remote temporary directory, grants execution permissions, executes the script with customizable CLI arguments, and guarantees cleanup of the remote script on connection exit.
* **Job Reference (`job-ref`)**: Calls another job's steps recursively on the current target node from PostgreSQL.

### 3. Node-First Step-by-Step Orchestration
* Executes steps sequentially per target node to ensure proper setup pipelines (e.g. file copying -> script setup -> verification).
* Distributes job execution **concurrently** across multiple remote target nodes in parallel using Go routines.
* Logs step progress and node output in real-time, prefixed by target node labels.

### 4. Strict Host-Key Verification (Secure-by-Default)
* Uses OpenSSH-style `known_hosts` verification to completely prevent Man-in-the-Middle (MITM) attacks.
* Disallows automatic/silent trusting of unknown keys by default.
* Includes CLI subcommands to manage trusted hosts:
  * `gruntdeck trust <host>`: Scans, prints SHA256 fingerprints, and prompts the user interactively before trusting.
  * `gruntdeck scan-host <host>`: Fetches and appends keys automatically (useful for non-interactive/CI setups).

### 5. Connection Heartbeats & Keepalives
* Sends connection keepalive requests (`keepalive@openssh.com`) every 15 seconds.
* Terminates dead connections immediately if pings time out for over 10 seconds (averts hangs from silent socket drops).
* Integrates a local context-wrapped loop that shuts down the keepalive routine immediately on function completion, preventing goroutine memory leaks.

---

## Database Schema (PostgreSQL DDL)

Database schema migrations are embedded into the `executor` binary and automatically applied when running jobs:

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
Build both the `executor` and the `gruntdeck` helper binaries:
```bash
go build -o gruntdeck ./cmd/gruntdeck
go build -o executor ./cmd/executor
```

---

## Environment Configuration (`.env`)

Create a `.env` file in your root working directory:
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/gruntdeck?sslmode=disable
```

---

## CLI Usage

### A. Managing Trusted Hosts
To add a host to Gruntdeck's trusted list (`.gruntdeck/known_hosts`):

**Interactive Trust:**
```bash
$ ./gruntdeck trust 192.168.1.100
Connecting to 192.168.1.100...
--------------------------------------------------
Key Type:    ecdsa-sha2-nistp256
Fingerprint: SHA256:z7Qf/K...s8Y
--------------------------------------------------
Do you trust this host? (yes/no): yes
✅ Added 192.168.1.100 to trusted hosts
```

**Non-interactive Auto-Scan:**
```bash
$ ./gruntdeck scan-host 192.168.1.100
Scanning 192.168.1.100...
✅ Automatically added 192.168.1.100 to trusted hosts
```

### B. Running Jobs
Trigger a job by passing its ID to the executor (it automatically loads `.env`):
```bash
$ ./executor deploy-app
```

**Console Output:**
```
🐘 Connecting to PostgreSQL database...
Database schema is up to date.
Job: Deploy Application Stack | Matching Nodes: 1
============================================================
[keerthan@127.0.0.1] 📁 Copying local ./config_demo.txt to remote /tmp/gruntdeck_test/config_demo.txt...
[keerthan@127.0.0.1] 📁 Successfully copied /tmp/gruntdeck_test/config_demo.txt
[keerthan@127.0.0.1] 📜 Uploading and executing script ./script_demo.sh [hello world]...
[keerthan@127.0.0.1] ➜ === Gruntdeck Demo Script ===
[keerthan@127.0.0.1] ➜ Working directory: /home/keerthan
[keerthan@127.0.0.1] ➜ Arguments received: hello world
[keerthan@127.0.0.1] 🔗 Invoking job reference: health-check
[keerthan@127.0.0.1] ➜ Running Diagnostics...
[keerthan@127.0.0.1] ➜ Filesystem      Size  Used Avail Use% Mounted on
[keerthan@127.0.0.1] ➜ /dev/nvme0n1p2  468G  391G   54G  88% /
============================================================
Execution Summary: 1 Succeeded | 0 Failed
```
