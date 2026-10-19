import json


def seed(cur, tasks, deps=(), status="pending", python_version=None, config="{}"):
    """One DAG, one run, one instance per task. deps is [(child, parent)] by task name."""
    cur.execute("INSERT INTO dags (name, python_version) VALUES ('d', %s) RETURNING id", (python_version,))
    dag_id = cur.fetchone()[0]
    task_ids = {}
    for name in tasks:
        cur.execute("INSERT INTO tasks (dag_id, name, task_type, config) VALUES (%s, %s, 'python', %s) RETURNING id",
                    (dag_id, name, config))
        task_ids[name] = cur.fetchone()[0]
    for child, parent in deps:
        cur.execute("INSERT INTO task_dependencies VALUES (%s, %s)", (task_ids[child], task_ids[parent]))
    cur.execute("INSERT INTO runs (dag_id, number) VALUES (%s, 1) RETURNING id", (dag_id,))
    run_id = cur.fetchone()[0]
    inst = {}
    for name in tasks:
        cur.execute("INSERT INTO task_instances (run_id, task_id, status) VALUES (%s, %s, %s) RETURNING id",
                    (run_id, task_ids[name], status))
        inst[name] = cur.fetchone()[0]
    cur.connection.commit()
    return run_id, inst


def status(cur, instance_id):
    cur.execute("SELECT status FROM task_instances WHERE id = %s", (instance_id,))
    return cur.fetchone()[0]


def set_status(cur, instance_id, new):
    cur.execute("UPDATE task_instances SET status = %s WHERE id = %s", (new, instance_id))
    cur.connection.commit()


def run_status(cur, run_id):
    cur.execute("SELECT status, started_at, finished_at FROM runs WHERE id = %s", (run_id,))
    return cur.fetchone()


class FakeRedis:
    """Records what the worker sends to Redis, so tests need no Redis server."""

    def __init__(self, fail_publish=False):
        self.fail_publish = fail_publish
        self.published, self.acked, self.added, self.sets, self.deleted = [], [], [], [], []

    def publish(self, channel, payload):
        if self.fail_publish:
            raise ConnectionError("redis is down")
        self.published.append(json.loads(payload))

    def xack(self, stream, group, message_id):
        self.acked.append(message_id)

    def xadd(self, stream, fields, maxlen=None, approximate=None):
        self.added.append(fields["task_instance_id"])

    def set(self, key, value, ex=None):
        self.sets.append((key, json.loads(value), ex))

    def delete(self, key):
        self.deleted.append(key)
