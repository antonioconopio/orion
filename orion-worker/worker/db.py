from psycopg2 import pool as pg_pool
import psycopg2

def get_pool(db_url: str, minconn: int = 1, maxconn: int = 5):
    return pg_pool.SimpleConnectionPool(minconn, maxconn, db_url)   # let startup errors raise

def close_pool(pool):
    pool.closeall()


def mark_running(cursor, instance_id: str, worker_id: str, reclaimed: bool = False) -> bool:
    try:
        cursor.execute(
            """
            UPDATE task_instances
            SET status = 'running', started_at = NOW(), worker_id = %(worker)s
            WHERE id = %(id)s
              AND (status = 'queued' OR (%(reclaimed)s AND status = 'running'))
            RETURNING id
            """,
            {"id": instance_id, "worker": worker_id, "reclaimed": reclaimed},
        )
        won = cursor.fetchone() is not None
        cursor.connection.commit()
        return won
    except Exception:
        cursor.connection.rollback()
        raise

def mark_failed(cursor, instance_id: str) -> str:
    conn = cursor.connection
    try:
        cursor.execute(
            """
            UPDATE task_instances
            SET status = 'failed', finished_at = NOW()
            WHERE id = %s AND status = 'running'
            RETURNING run_id, task_id
            """,
            (instance_id,),
        )
        row = cursor.fetchone()
        if row is None:
            raise RuntimeError(f"task instance {instance_id} is not running")
        run_id, task_id = row

        cursor.execute(
            """
            WITH RECURSIVE downstream AS (
                SELECT task_id FROM task_dependencies
                WHERE depends_on_task_id = %(task_id)s
              UNION
                SELECT td.task_id FROM task_dependencies td
                JOIN downstream d ON td.depends_on_task_id = d.task_id
            )
            UPDATE task_instances
            SET status = 'skipped', finished_at = NOW()
            WHERE run_id = %(run_id)s
              AND status = 'pending'
              AND task_id IN (SELECT task_id FROM downstream)
            """,
            {"task_id": task_id, "run_id": run_id},
        )
        conn.commit()
        return run_id
    except Exception:
        conn.rollback()
        raise

def mark_success(cursor, instance_id: str) -> str:
    try:
        cursor.execute(
            """
            UPDATE task_instances
            SET status = 'success', finished_at = NOW()
            WHERE id = %s AND status = 'running'
            RETURNING run_id
            """,
            (instance_id,),
        )
        row = cursor.fetchone()
        if row is None:
            raise RuntimeError(f"task instance {instance_id} is not running")
        cursor.connection.commit()
        return row[0]
    except Exception:
        cursor.connection.rollback()
        raise
    
def finalize_run(cursor: psycopg2.extensions.cursor, run_id: str):
    try:
        cursor.execute(
            """
            UPDATE runs
            SET status = CASE
                    WHEN EXISTS (
                        SELECT 1 FROM task_instances
                        WHERE run_id = %(run_id)s AND status = 'failed'
                    ) THEN 'failed'
                    ELSE 'success'
                END,
                finished_at = NOW()
            WHERE id = %(run_id)s
              AND status NOT IN ('success', 'failed', 'cancelled')
              AND NOT EXISTS (
                  SELECT 1 FROM task_instances
                  WHERE run_id = %(run_id)s
                    AND status IN ('pending', 'queued', 'running', 'retrying')
              )
            """,
            {"run_id": run_id},
        )
        cursor.connection.commit()
    except Exception:
        cursor.connection.rollback()
        raise

def claim_ready_tasks(cursor: psycopg2.extensions.cursor, instance_id: str) -> list[str]:
    try:
        cursor.execute(
            """
            UPDATE task_instances ti
            SET status = 'queued'
            WHERE ti.status = 'pending'
              AND ti.run_id = (SELECT run_id FROM task_instances WHERE id = %s)
              AND ti.task_id IN (
                  SELECT td.task_id
                  FROM task_dependencies td
                  JOIN task_instances done ON done.task_id = td.depends_on_task_id
                  WHERE done.id = %s
              )
              AND NOT EXISTS (
                  SELECT 1
                  FROM task_dependencies td
                  JOIN task_instances up
                    ON up.task_id = td.depends_on_task_id AND up.run_id = ti.run_id
                  WHERE td.task_id = ti.task_id
                    AND up.status <> 'success'
              )
            RETURNING ti.id
            """,
            (instance_id, instance_id),
        )
        
        ids = [row[0] for row in cursor.fetchall()]
        cursor.connection.commit()
        return ids
    except Exception:
        cursor.connection.rollback()
        raise

def write_log(cursor: psycopg2.extensions.cursor, instance_id: str, message: str):
    try:
        cursor.execute(
            """
            UPDATE task_instances
            SET logs = CONCAT(COALESCE(logs, ''), %s)
            WHERE id = %s

            """,
            ("\n" + message, instance_id),
        )
        cursor.connection.commit()
    except Exception:
        cursor.connection.rollback()
        raise

def get_task(cursor, instance_id: str):
    try:
        cursor.execute(
            """
            SELECT t.task_type, t.config
            FROM task_instances ti
            JOIN tasks t ON t.id = ti.task_id
            WHERE ti.id = %s
            """,
            (instance_id,),
        )
        row = cursor.fetchone()
        return (row[0], row[1] or {}) if row else None
    except Exception:
        cursor.connection.rollback()
        raise
    
def get_python_version(cursor: psycopg2.extensions.cursor, instance_id: str) -> str | None:
    try:
        cursor.execute(
            """
            SELECT d.python_version
            FROM task_instances ti
            JOIN tasks t ON t.id = ti.task_id
            JOIN dags d ON d.id = t.dag_id
            WHERE ti.id = %s
            """,
            (instance_id,),
        )
        row = cursor.fetchone()
        return row[0] if row else None
    except Exception:
        cursor.connection.rollback()
        raise