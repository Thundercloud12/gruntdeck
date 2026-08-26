# Gruntdeck ⚡

[![CI](https://github.com/Thundercloud12/gruntdeck/actions/workflows/ci.yml/badge.svg)](https://github.com/Thundercloud12/gruntdeck/actions/workflows/ci.yml)
[![Release](https://github.com/Thundercloud12/gruntdeck/actions/workflows/release.yml/badge.svg)](https://github.com/Thundercloud12/gruntdeck/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine, built in Go. It runs multi-step automation workflows across remote nodes over SSH, backed by a 100% PostgreSQL persistence layer and a decoupled background job queue (**River**) — no JVM, no ZooKeeper, no external message broker.

---

## Table of Contents

- [Screenshots](#-screenshots)
- [Features](#-features-overview)
- [Quickstart with Docker Compose](#-quickstart-with-docker-compose-recommended)
- [Architecture](#-architecture)
- [Local Development (without Docker)](#-local-development-without-docker)
- [Configuration Reference](#-configuration-reference)
- [Tech Stack](#-tech-stack)
- [CI/CD & Automated Releases](#-cicd--automated-releases)
- [License](#-license)

---

## 📸 Screenshots

| Projects | Jobs |
|---|---|
| ![Projects tab](docs/screenshots/projects.png) | ![Jobs tab](docs/screenshots/jobs.png) |

| Schedules | Executions |
|---|---|
| ![Schedules tab](docs/screenshots/schedules.png) | ![Executions tab](docs/screenshots/executions.png) |

| Target Nodes | Key Storage |
|---|---|
| ![Target Nodes tab](docs/screenshots/targets.png) | ![Key Storage tab](docs/screenshots/credentials.png) |

<details>
<summary><strong>Mobile view</strong> (responsive down to 390px)</summary>
<br>

| Projects | Jobs |
|---|---|
| <img src="docs/screenshots/mobile-projects.png" width="280"/> | <img src="docs/screenshots/mobile-jobs.png" width="280"/> |

</details>

---

## ✨ Features Overview

- **📁 Multi-Project Workspaces**: Namespace-isolated environments for teams and projects with project-scoped resource permissions.
- **📋 Rundeck-Style Job Show Screen**:
  - **Stats Sub-Tab**: Computes total runs, success rate %, and average duration.
  - **Activity Sub-Tab**: Detailed execution history per job with status pills and duration tracking.
- **👁️ "Follow Execution" Display Modes**:
  - **`Nodes` View**: Accordion cards grouping log entries by remote server IP/hostname.
  - **`Log Output` View**: Real-time monospaced terminal output stream.
  - **`HTML` View**: Sandboxed `<iframe>` layout rendering formatted HTML markup and reports output by scripts.
- **🔒 AES-256-GCM Key Vault**: Secure storage for SSH private keys, remote passwords, and API tokens.
- **⏰ Cron Scheduler**: Timezone-aware background schedule execution engine.
- **🚀 Decoupled Task Queue**: Powered by PostgreSQL-backed **River** queue for reliable background processing.
- **📱 Responsive Console UI**: Framework-free HTML/CSS/JS dashboard that works down to phone width, with no build step.

---

## ⚡ Quickstart with Docker Compose (Recommended)

Run the full Gruntdeck stack (PostgreSQL + API Web Server + Background Executor Worker) with a single command:

```bash
docker compose up -d
```

Open your browser at **http://localhost:8080** and sign in with:
- **Username**: `admin`
- **Password**: `adminpassword`

> ⚠️ **Before exposing this beyond localhost**, change `ADMIN_PASSWORD` and `GRUNTDECK_MASTER_KEY` in `docker-compose.yml` — the defaults are checked into this repo and are not secret.

---

## 🏗️ Architecture

Gruntdeck ships as a single all-in-one binary (`cmd/server`) that serves the web UI, the REST API, the cron scheduler, and the job queue in one process — or as decoupled `producer`/`executor` binaries for horizontally-scaled deployments.

```mermaid
flowchart TD
    UI["Browser UI<br/>(vanilla HTML/CSS/JS, web/)"]

    subgraph SERVER["cmd/server — Web + API + Scheduler"]
        API["REST API & Handlers<br/>internal/api"]
        SCHED["Cron Scheduler<br/>internal/scheduler"]
    end

    subgraph WORKER["cmd/executor — Background Worker"]
        RIVER["River Queue Engine<br/>internal/queue"]
        EXEC["Execution Service<br/>internal/execution"]
        SSHPKG["SSH Client<br/>internal/ssh"]
    end

    VAULT["AES-256-GCM Vault<br/>internal/secrets"]
    REPO["Repositories<br/>internal/repository"]
    DB[("PostgreSQL<br/>jobs · executions · targets · credentials")]
    NODES[["Remote Target Nodes"]]

    UI -->|HTTPS| API
    API --> REPO
    API -->|enqueue via queue producer| RIVER
    SCHED -->|enqueue on cron tick| RIVER
    RIVER --> EXEC
    EXEC --> SSHPKG
    SSHPKG -->|SSH exec / SCP| NODES
    EXEC --> REPO
    API --> VAULT
    EXEC --> VAULT
    REPO <--> DB
    RIVER <--> DB
```

**Request flow:**
1. The UI (or `producer` CLI) calls the API, which enqueues an `ExecuteJobArgs` task via the **River** queue producer — or the **Scheduler** enqueues it on a cron tick.
2. River persists the job in Postgres and dispatches it to a worker (in-process in `cmd/server`, or a standalone `cmd/executor` process).
3. The **Execution Service** walks the job's steps (command / script / file-copy / job-ref), resolving credentials through the encrypted **Secrets Vault**.
4. The **SSH package** opens a connection to each target node and streams command output back, which is persisted per-line to Postgres for the "Follow Execution" log views.
5. All state (jobs, executions, targets, schedules, credentials, projects, users) lives in **PostgreSQL**, accessed exclusively through the **Repository** layer.

### Project structure

```
cmd/
  server/      → primary binary: web UI + REST API + scheduler + in-process queue worker
  executor/    → standalone background worker (advanced/decoupled deployments)
  producer/    → CLI to manually enqueue a job by ID
  gruntdeck/   → standalone SSH command-line utility
internal/
  api/         → HTTP handlers & routing
  execution/   → job step orchestration (command/script/file-copy/job-ref)
  migrations/  → embedded SQL migrations, applied automatically on startup
  models/      → domain types shared across layers
  queue/       → River queue producer & worker wiring
  repository/  → PostgreSQL persistence (interfaces + implementations)
  scheduler/   → cron-based schedule execution (robfig/cron)
  secrets/     → AES-256-GCM credential encryption
  ssh/         → remote command execution over SSH
  variables/   → runtime job/step variable interpolation
web/           → static HTML/CSS/JS dashboard, embedded into the server binary
docs/          → project website & README assets
```

---

## 🖥️ Local Development (without Docker)

Requires Go 1.25+ and a running PostgreSQL instance.

```bash
# 1. Start Postgres however you like, e.g.:
docker run -d --name gruntdeck-db -p 5432:5432 \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=gruntdeck \
  postgres:15-alpine

# 2. Configure environment
cp .env.example .env
# edit .env — set GRUNTDECK_MASTER_KEY and ADMIN_PASSWORD

# 3. Run the all-in-one server (migrations run automatically on startup)
go run ./cmd/server
```

For a decoupled deployment, run the executor as a separate worker process and enqueue jobs from the CLI:

```bash
go run ./cmd/executor
go run ./cmd/producer deploy-app --APP_VERSION=v2.0.1 --ENVIRONMENT=production
```

---

## ⚙️ Configuration Reference

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
| `GRUNTDECK_MASTER_KEY` | Yes | — | Encrypts stored credentials (AES-256-GCM). Any string works — it's SHA-256-hashed internally. Generate one with `openssl rand -hex 32`. |
| `ADMIN_PASSWORD` | Yes | — | Password for the bootstrapped initial admin user. Only used the very first time the server starts against an empty database. |
| `ADMIN_USERNAME` | No | `admin` | Username for the bootstrapped initial admin user. |
| `PORT` | No | `8080` | HTTP port `cmd/server` listens on. |

---

## 🧰 Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.25 |
| Database | PostgreSQL |
| Job Queue | [River](https://riverqueue.com/) (Postgres-backed) |
| Scheduler | [robfig/cron](https://github.com/robfig/cron) |
| Remote Execution | `golang.org/x/crypto/ssh` |
| Frontend | Vanilla HTML / CSS / JS, served via Go `embed.FS` — no build step |
| Auth | bcrypt password hashing, HttpOnly/SameSite session cookies |
| Secrets | AES-256-GCM |
| Deployment | Docker Compose, GitHub Actions, GHCR |

---

## 🚢 CI/CD & Automated Releases

Gruntdeck includes GitHub Actions workflows (`.github/workflows/`):
- **`ci.yml`**: Runs `go build` and `go test` against a real Postgres service container on every push/PR.
- **`release.yml`**: Cross-compiles release binaries (Linux `amd64`/`arm64`, macOS `arm64`) and publishes them as GitHub Releases, plus a multi-arch Docker image to `ghcr.io`, whenever a version tag is pushed (e.g. `git tag v1.0.0 && git push origin --tags`).

---

## 📜 License

MIT License. Free for enterprise and personal production use. See [LICENSE](LICENSE).
