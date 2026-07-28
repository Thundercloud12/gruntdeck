# Gruntdeck

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine built in Go. It offers a secure, concurrent, and highly performant orchestration engine to execute multi-step automation workflows across remote nodes over SSH, backed by a PostgreSQL database persistence layer.

---

## Architecture Overview

Below is the complete system architecture of Gruntdeck, illustrating the database layer, repository interfaces, execution engine, SSH client pool, and host-key verification mechanisms.

```mermaid
flowchart TD
    subgraph CLI ["Gruntdeck CLI & Execution Entry Points"]
        GT["gruntdeck CLI<br/>(trust / scan-host)"]
        EX["executor CLI<br/>(Job Runner)"]
    end

    subgraph Storage ["Persistence Layer (PostgreSQL & YAML Fallback)"]
        PG[(PostgreSQL Database<br/>via pgx/v5)]
        YML[YAML Files<br/>inventory.yaml / jobs.yaml]

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
    EX -->|Load Job & Targets| Repos
    GT -->|Manage Host Keys| KH
    
    Repos -->|pgx/v5 Pool| PG
    Repos -.->|Fallback| YML

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

### 1. PostgreSQL Persistence Layer (`pgx/v5`)
* **Repository Pattern (`internal/repository`)**: Abstracted database interfaces (`InventoryRepository`, `JobRepository`, `ExecutionRepository`, `LogRepository`) decoupled from storage backends.
* **High-Performance Connection Pooling (`pgxpool`)**: Built using `github.com/jackc/pgx/v5/pgxpool` for native PostgreSQL pooling, context timeouts, and direct array mapping (`TEXT[]`).
* **Execution & Log Tracking**: Persists overall run metrics (`status`, `started_at`, `ended_at`, `targets_total`, `targets_succeeded`, `targets_failed`) and real-time execution log entries (`log_entries`).
* **YAML Fallback**: Automatically falls back to `inventory.yaml` and `jobs.yaml` if `DATABASE_URL` is unconfigured.

### 2. Advanced Job Step Types
* **Command (`command`)**: Executes arbitrary shell commands on target nodes.
* **File Copier (`file-copy`)**: Transfers local configuration files or assets to remote nodes using high-efficiency SSH stdin piping (zero SFTP/rsync dependencies required).
* **Script Executor (`script`)**: Automatically copies local scripts to a remote temporary directory, grants execution permissions, executes the script with customizable CLI arguments, and guarantees cleanup of the remote script on connection exit.
* **Job Reference (`job-ref`)**: Calls another job's steps recursively on the current target node.

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

To set up PostgreSQL for Gruntdeck, apply the following DDL schema:

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
* PostgreSQL database instance (optional, for DB storage mode)
* Target nodes with SSH enabled and authorized public keys configured.

### Compilation
Build both the `executor` and the `gruntdeck` helper binaries:
```bash
go build -o gruntdeck ./cmd/gruntdeck
go build -o executor ./cmd/executor
```

---

## Configuration & Environment

### PostgreSQL Mode
Set the `DATABASE_URL` environment variable before running `executor`:
```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/gruntdeck?sslmode=disable"
./executor deploy-app
```

### File Mode (Fallback)
If `DATABASE_URL` is omitted, Gruntdeck automatically uses local YAML configuration files:

#### 1. Host Inventory (`inventory.yaml`)
```yaml
targets:
  - host: 127.0.0.1
    port: 22
    user: admin
    key_path: /home/user/.ssh/id_rsa 
    tags: ["web-server", "production", "linux"]
```

#### 2. Job Definitions (`jobs.yaml`)
```yaml
jobs:
  - id: health-check
    name: "System Health Diagnostics"
    target_filter: ["web-server", "production"]
    steps:
      - type: command
        value: "echo 'Running Diagnostics...'"
      - type: command
        value: "df -h"

  - id: deploy-app
    name: "Deploy Application Stack"
    target_filter: ["web-server"]
    steps:
      - type: file-copy
        source_path: "./config_demo.txt"
        dest_path: "/tmp/gruntdeck_test/config_demo.txt"
      - type: script
        source_path: "./script_demo.sh"
        args: ["hello", "world"]
      - type: job-ref
        job_id: "health-check"
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
Trigger a job by passing its ID to the executor:
```bash
$ ./executor deploy-app
```

**Console Output:**
```
Job: Deploy Application Stack | Matching Nodes: 1
============================================================
[keerthan@127.0.0.1] 📁 Copying local ./config_demo.txt to remote /tmp/gruntdeck_test/config_demo.txt...
[keerthan@127.0.0.1] 📁 Successfully copied /tmp/gruntdeck_test/config_demo.txt
[keerthan@127.0.0.1] 📜 Uploading and executing script ./script_demo.sh [hello world]...
[keerthan@127.0.0.1] ➜ === Gruntdeck Demo Script ===
[keerthan@127.0.0.1] ➜ Working directory: /home/keerthan
[keerthan@127.0.0.1] ➜ Arguments received: hello world
[keerthan@127.0.0.1] ➜ Reading transferred config file:
[keerthan@127.0.0.1] ➜ # Gruntdeck Demo Config File
[keerthan@127.0.0.1] ➜ app_name=gruntdeck_demo
[keerthan@127.0.0.1] 🔗 Invoking job reference: health-check
[keerthan@127.0.0.1] ➜ Running Diagnostics...
[keerthan@127.0.0.1] ➜ Filesystem      Size  Used Avail Use% Mounted on
[keerthan@127.0.0.1] ➜ /dev/nvme0n1p2  468G  391G   54G  88% /
============================================================
Execution Summary: 1 Succeeded | 0 Failed
```
