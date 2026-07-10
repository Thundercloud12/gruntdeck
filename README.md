# Gruntdeck

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine built in Go. It offers a secure, concurrent, and highly performant execution pipeline to run structured jobs across remote nodes over SSH.

---

## Key Features

### 1. Advanced Job Step Types
* **Command (`command`)**: Executes arbitrary shell commands on target nodes.
* **File Copier (`file-copy`)**: Transfers local configuration files or assets to remote nodes using high-efficiency SSH stdin piping (zero SFTP/rsync dependencies required).
* **Script Executor (`script`)**: Automatically copies local scripts to a remote temporary directory, grants execution permissions, executes the script with customizable CLI arguments, and guarantees cleanup of the remote script on connection exit.
* **Job Reference (`job-ref`)**: Calls another job's steps recursively on the current target node.

### 2. Node-First Step-by-Step Orchestration
* Executes steps sequentially per target node to ensure proper setup pipelines (e.g. file copying -> script setup -> verification).
* Distributes job execution **concurrently** across multiple remote target nodes in parallel using Go routines.
* Logs step progress and node output in real-time, prefixed by target node labels.

### 3. Strict Host-Key Verification (Secure-by-Default)
* Uses OpenSSH-style `known_hosts` verification to completely prevent Man-in-the-Middle (MITM) attacks.
* Disallows automatic/silent trusting of unknown keys by default.
* Includes CLI subcommands to manage trusted hosts:
  * `gruntdeck trust <host>`: Scans, prints SHA256 fingerprints, and prompts the user interactively before trusting.
  * `gruntdeck scan-host <host>`: Fetches and appends keys automatically (useful for non-interactive/CI setups).

### 4. Connection Heartbeats & Keepalives
* Sends connection keepalive requests (`keepalive@openssh.com`) every 15 seconds.
* Terminates dead connections immediately if pings time out for over 10 seconds (averts hangs from silent socket drops).
* Integrates a local context-wrapped loop that shuts down the keepalive routine immediately on function completion, preventing goroutine memory leaks.

---

## Getting Started

### Prerequisites
* Go 1.25+
* Target nodes must have SSH enabled and authorized public keys configured.

### Compilation
Build both the `executor` and the `gruntdeck` helper binaries:
```bash
go build -o gruntdeck ./cmd/gruntdeck
go build -o executor ./cmd/executor
```

---

## Configuration

### 1. Host Inventory (`inventory.yaml`)
Define target hosts, ports, users, keys, and matching tags:
```yaml
targets:
  - host: 127.0.0.1
    port: 22
    user: admin
    key_path: /home/user/.ssh/id_rsa 
    tags: ["web-server", "production", "linux"]
```

### 2. Job Definitions (`jobs.yaml`)
Define jobs with target filters and structured step objects:
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
        source_path: "./configs/app.conf"
        dest_path: "/tmp/app.conf"
      - type: script
        source_path: "./scripts/deploy.sh"
        args: ["--port", "8080"]
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
**Example Log Output:**
```
Job: Deploy Application Stack | Matching Nodes: 1
============================================================
[admin@127.0.0.1] 📁 Copying local ./configs/app.conf to remote /tmp/app.conf...
[admin@127.0.0.1] 📁 Successfully copied /tmp/app.conf
[admin@127.0.0.1] 📜 Uploading and executing script ./scripts/deploy.sh [--port 8080]...
[admin@127.0.0.1] ➜ Starting Server deployment...
[admin@127.0.0.1] ➜ Done.
[admin@127.0.0.1] 🔗 Invoking job reference: health-check
[admin@127.0.0.1] ➜ Running Diagnostics...
[admin@127.0.0.1] ➜ Filesystem      Size  Used Avail Use% Mounted on
[admin@127.0.0.1] ➜ /dev/sda1       387G  347G   40G  90% /
============================================================
Execution Summary: 1 Succeeded | 0 Failed
```
