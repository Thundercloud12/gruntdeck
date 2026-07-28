CREATE TABLE IF NOT EXISTS targets (
    id VARCHAR(255) PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL DEFAULT '22',
    "user" TEXT NOT NULL,
    key_path TEXT NOT NULL,
    tags TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS jobs (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    target_filter TEXT[] NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS job_steps (
    id VARCHAR(255) PRIMARY KEY,
    job_id VARCHAR(255) NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    step_order INT NOT NULL,
    type VARCHAR(50) NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb
);

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

CREATE TABLE IF NOT EXISTS log_entries (
    id VARCHAR(255) PRIMARY KEY,
    execution_id VARCHAR(255) NOT NULL REFERENCES executions(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL,
    step_id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL
);
