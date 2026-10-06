ALTER TABLE dags
    ADD COLUMN python_version TEXT;

ALTER TABLE task_instances
    ADD COLUMN logs TEXT,
    DROP COLUMN log_location;