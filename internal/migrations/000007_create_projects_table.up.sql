CREATE TABLE IF NOT EXISTS projects (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Seed default project
INSERT INTO projects (id, name, description)
VALUES ('default', 'Default Project', 'Default automation workspace')
ON CONFLICT (id) DO NOTHING;

-- Scope resources to projects
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS project_id VARCHAR(255) REFERENCES projects(id) ON DELETE CASCADE DEFAULT 'default';
ALTER TABLE targets ADD COLUMN IF NOT EXISTS project_id VARCHAR(255) REFERENCES projects(id) ON DELETE CASCADE DEFAULT 'default';
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS project_id VARCHAR(255) REFERENCES projects(id) ON DELETE CASCADE DEFAULT 'default';
ALTER TABLE executions ADD COLUMN IF NOT EXISTS project_id VARCHAR(255) REFERENCES projects(id) ON DELETE CASCADE DEFAULT 'default';
