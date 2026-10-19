import json
import os, time, socket, signal, traceback
from pathlib import Path
from dotenv import load_dotenv

import db, stream, executor, events
from heartbeat import Heartbeat

RECLAIM_EVERY = 30       # seconds between reclaim checks
MIN_IDLE_MS = (executor.MAX_TIMEOUT + 20) * 1000   # a healthy task must never look dead
STALE_QUEUED_S = 300     # a re-enqueue of a merely slow message is harmless: mark_running drops duplicates

running = True
def stop(*_):
    global running
    running = False


def process(pool, client, consumer_name, message_id, fields, reclaimed=False, hb=None):
    instance_id = fields["task_instance_id"]
    rdb = client.client

    conn = pool.getconn()
    try:
        with conn.cursor() as cursor:
            if not db.mark_running(cursor, instance_id, consumer_name, reclaimed):
                stream.acknowledge(client, message_id)
                return

            if hb:
                hb.busy(instance_id)
            task = db.get_task(cursor, instance_id)   # (type, config) or None
            python_version = db.get_python_version(cursor, instance_id)
            ids = db.get_run_and_dag(cursor, instance_id)
            inputs = db.get_inputs(cursor, instance_id)   # parents' outputs, by task name
            conn.rollback()                                    # end the read transaction

            if ids:
                events.run_changed(rdb, *ids)   # the task just started

            # the instance is 'running' now, so every path below must end in success/failed
            if task is None:
                result = executor.ExecResult(False, None, "", "task not found for instance")
            else:
                try:
                    result = executor.execute(*task, python_version, inputs)
                except Exception as e:
                    result = executor.ExecResult(False, None, "", f"executor crashed: {e!r}")

            log = "\n" + json.dumps({
                "stdout": result.stdout,
                "stderr": result.stderr,
                "timed_out": result.timed_out,
            })

            # log is written in the same transaction as the status flip
            if result.success:
                run_id = db.mark_success(cursor, instance_id, log, result.output)
                ready_tasks = db.claim_ready_tasks(cursor, instance_id)
            else:
                run_id = db.mark_failed(cursor, instance_id, log)
                ready_tasks = []

            db.finalize_run(cursor, run_id)
            if ids:
                events.run_changed(rdb, *ids)   # status, logs, skipped/queued children and run status are all committed
    finally:
        pool.putconn(conn)
        if hb:
            hb.idle()

    for ready_task_id in ready_tasks:
        stream.enqueue(client, ready_task_id)

    stream.acknowledge(client, message_id)


def main():
    load_dotenv(Path(__file__).resolve().parents[2] / ".env")   # adjust the number to your folder depth

    db_url      = os.environ["DATABASE_URL"]
    redis_addr  = os.environ["REDIS_ADDR"]
    stream_name = os.environ["STREAM_NAME"]
    group_name  = os.environ["CONSUMER_GROUP"]              # same variable the Go API reads
    max_len     = int(os.environ.get("MAX_LEN", "1000"))    # the API hardcodes 1000

    consumer_name = f"{socket.gethostname()}-{os.getpid()}"

    pool   = db.get_pool(db_url)
    client = stream.get_redis_stream(redis_addr, stream_name, group_name, max_len)

    signal.signal(signal.SIGTERM, stop)   # docker stop
    signal.signal(signal.SIGINT, stop)    # ctrl+c

    hb = Heartbeat(client.client, consumer_name)
    hb.start()

    last_reclaim = time.monotonic()

    while running:
        try:
            for message_id, fields in stream.dequeue(client, consumer_name, count=1):
                process(pool, client, consumer_name, message_id, fields, hb=hb)

            if time.monotonic() - last_reclaim >= RECLAIM_EVERY:
                last_reclaim = time.monotonic()
                for message_id, fields in stream.claim_stale(client, consumer_name, MIN_IDLE_MS):
                    process(pool, client, consumer_name, message_id, fields, reclaimed=True, hb=hb)

                # 'queued' in Postgres but never delivered (crash between claim and XADD)
                conn = pool.getconn()
                try:
                    with conn.cursor() as cursor:
                        stale = db.requeue_stale(cursor, STALE_QUEUED_S)
                finally:
                    pool.putconn(conn)
                for instance_id in stale:
                    stream.enqueue(client, instance_id)

        except Exception:
            traceback.print_exc()
            time.sleep(1)

    hb.stop()


if __name__ == "__main__":
    main()
