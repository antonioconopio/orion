ALTER TABLE task_instances
    ADD COLUMN log_location TEXT,
    DROP COLUMN IF EXISTS logs;

ALTER TABLE dags
    DROP COLUMN IF EXISTS python_version;