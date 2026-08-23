# Gruntdeck ⚡

Gruntdeck is a lightweight, clean-room reimplementation of the **Rundeck** execution engine built in Go. It offers a secure, concurrent, and highly performant orchestration engine to execute multi-step automation workflows across remote nodes over SSH, powered by a 100% PostgreSQL persistence layer and decoupled background job queue (**River**).

---

## ⚡ Quickstart with Docker Compose (Recommended)

Run the full Gruntdeck stack (PostgreSQL + API Web Server + Background Executor Worker) with a single command:

```bash
docker compose up -d
```

Open your browser at **http://localhost:8080** and sign in with:
- **Username**: `admin`
- **Password**: `adminpassword`

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

---

## 🏗️ Architecture & Layering

The system is strictly layered following the **Single Responsibility Principle (SRP)**.

```
Web / Producer CLI
      │ (Calls QueueProducer.Enqueue)
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

---

## ⚙️ Environment Configuration (`.env`)

Create a `.env` file in your root working directory:
```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/gruntdeck?sslmode=disable
GRUNTDECK_MASTER_KEY=01234567890123456789012345678901
ADMIN_USERNAME=admin
ADMIN_PASSWORD=adminpassword
```

---

## 🚢 CI/CD & Automated Releases

Gruntdeck includes GitHub Actions workflows (`.github/workflows/`):
- **`ci.yml`**: Automatically runs `go test` and builds binaries on every push/PR.
- **`release.yml`**: Automatically cross-compiles release binaries (Linux `amd64`/`arm64`, macOS `arm64`) and publishes GitHub releases when pushing a version tag (e.g., `git tag v1.0.0 && git push origin --tags`).

---

## 📜 License

MIT License. Free for enterprise and personal production use.
