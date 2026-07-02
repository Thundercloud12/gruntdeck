# Gruntdeck

Gruntdeck is a lightweight, clean-room reimplementation of **Rundeck** written in Go. It aims to provide the robust, pluggable job scheduling and multi-node execution engine of Rundeck without the JVM overhead.

## Architecture & Goals

- **Performance-First**: Fast startup, low memory footprint, and sub-second execution scheduling.
- **Agentless Execution**: Native support for SSH, Local, and pluggable node executors.
- **Workflow & Rules Engine**: Flexible workflow execution supporting parallel/sequential steps, error-handling, and step retries.
- **API-First Design**: Fully scriptable via a clean REST API.

## Project Structure

The project is structured according to Go standard layout guidelines:

- `cmd/gruntdeck/`: Entry point for the server/CLI application.
- `internal/api/`: REST API handlers and router.
- `internal/execution/`: Core execution engine (workflow runner, step executors, dispatchers).
- `internal/job/`: Job definition parser (YAML/JSON) and metadata storage.
- `internal/node/`: Node source parsers (XML/YAML resource files) and node registries.
- `internal/project/`: Project (namespace) manager and directory configurations.
- `internal/scheduler/`: Quartz-like cron-based job scheduler.
- `pkg/`: Reusable packages that can be imported by other projects.

## Getting Started

### Prerequisites

- Go 1.22+ installed on your system.

### Build and Run

```bash
# Build the binary
go build -o gruntdeck cmd/gruntdeck/main.go

# Start the server
./gruntdeck start
```
