ALTER TABLE runs DROP CONSTRAINT runs_dag_id_number_key;
ALTER TABLE runs DROP COLUMN number;
ALTER TABLE dags DROP COLUMN run_counter;
