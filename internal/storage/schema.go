package storage

const Schema = `
CREATE TABLE IF NOT EXISTS queues (
    name VARCHAR(255) PRIMARY KEY,
    task_timeout BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(20) PRIMARY KEY,
    queue_name VARCHAR(255) REFERENCES queues(name),
    status VARCHAR(20) NOT NULL,
    data JSONB,
    assigned_to VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_queue_name ON tasks(queue_name);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);

CREATE INDEX IF NOT EXISTS idx_tasks_combined ON tasks(queue_name, status, created_at, assigned_to);

CREATE INDEX IF NOT EXISTS idx_queues_name ON queues(name);



`
