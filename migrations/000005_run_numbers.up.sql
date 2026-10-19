ALTER TABLE dags ADD COLUMN run_counter INT NOT NULL DEFAULT 0;
ALTER TABLE runs ADD COLUMN number INT;

-- number existing runs 1..n per DAG in creation order, then point each DAG's counter at its last number
UPDATE runs r SET number = n.rn
FROM (SELECT id, ROW_NUMBER() OVER (PARTITION BY dag_id ORDER BY created_at, id) AS rn FROM runs) n
WHERE n.id = r.id;
UPDATE dags d SET run_counter = COALESCE((SELECT MAX(number) FROM runs WHERE dag_id = d.id), 0);

ALTER TABLE runs ALTER COLUMN number SET NOT NULL;
ALTER TABLE runs ADD CONSTRAINT runs_dag_id_number_key UNIQUE (dag_id, number);
