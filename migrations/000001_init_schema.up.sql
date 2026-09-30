CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE dags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    schedule TEXT,                       -- cron expression, NULL = manual trigger only
    owner TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dag_id UUID NOT NULL REFERENCES dags(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    task_type TEXT NOT NULL,             -- e.g. "shell", "http", "python"
    config JSONB NOT NULL DEFAULT '{}',  -- task-specific execution params
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dag_id, name)
);

CREATE TABLE task_dependencies (
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on_task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on_task_id),
    CHECK (task_id != depends_on_task_id)
);

CREATE TABLE runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dag_id UUID NOT NULL REFERENCES dags(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued','running','success','failed','cancelled')),
    triggered_by TEXT NOT NULL DEFAULT 'manual',  -- 'manual' | 'schedule'
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending','queued','running','success','failed','retrying','skipped')),
    attempt INT NOT NULL DEFAULT 1,
    worker_id TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    log_location TEXT,                   -- path/URL to logs, not the logs themselves
    UNIQUE (run_id, task_id)
);

CREATE INDEX idx_tasks_dag_id ON tasks(dag_id);
CREATE INDEX idx_runs_dag_id ON runs(dag_id);
CREATE INDEX idx_runs_status ON runs(status);
CREATE INDEX idx_task_instances_run_id ON task_instances(run_id);
CREATE INDEX idx_task_instances_status ON task_instances(status);
