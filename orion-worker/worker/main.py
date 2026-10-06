import json
import os, time, socket, signal, traceback
from pathlib import Path
from dotenv import load_dotenv

import db, stream, executor

load_dotenv(Path(__file__).resolve().parents[2] / ".env")   # adjust the number to your folder depth

RECLAIM_EVERY = 30       # seconds between reclaim checks
MIN_IDLE_MS = 120_000    # must be longer than your longest task

db_url      = os.environ["DATABASE_URL"]
redis_addr  = os.environ["REDIS_ADDR"]
stream_name = os.environ["STREAM_NAME"]
group_name  = os.environ["GROUP_NAME"]
max_len     = int(os.environ["MAX_LEN"])

consumer_name = f"{socket.gethostname()}-{os.getpid()}"

pool   = db.get_pool(db_url)
client = stream.get_redis_stream(redis_addr, stream_name, group_name, max_len)

running = True
def stop(*_):
    global running
    running = False
signal.signal(signal.SIGTERM, stop)   # docker stop
signal.signal(signal.SIGINT, stop)    # ctrl+c


def process(message_id, fields):
    instance_id = fields["task_instance_id"]

    conn = pool.getconn()
    try:
        with conn.cursor() as cursor:
            if not db.mark_running(cursor, instance_id, consumer_name):    # add reclaimed=... later
                stream.acknowledge(client, message_id)
                return

            try:
                task_type, config = db.get_task(cursor, instance_id)   # one query: type + full config
            except Exception:
                stream.acknowledge(client, message_id)
                return

            python_version = db.get_python_version(cursor, instance_id)
            conn.rollback()                                    # end the read transaction

            result = executor.execute(task_type, config, python_version)

            if result.success:
                run_id = db.mark_success(cursor, instance_id)
                ready_tasks = db.claim_ready_tasks(cursor, instance_id)
            else:
                run_id = db.mark_failed(cursor, instance_id)
                ready_tasks = []
            
            db.write_log(cursor, instance_id, json.dumps({
                "stdout": result.stdout,
                "stderr": result.stderr,
                "timed_out": result.timed_out,
            }))
            db.finalize_run(cursor, run_id)
    finally:
        pool.putconn(conn)

    for ready_task_id in ready_tasks:
        stream.enqueue(client, ready_task_id)

    stream.acknowledge(client, message_id)


last_reclaim = time.monotonic()

while running:
    try:
        for message_id, fields in stream.dequeue(client, consumer_name, count=1):
            process(message_id, fields)

        if time.monotonic() - last_reclaim >= RECLAIM_EVERY:
            last_reclaim = time.monotonic()
            for message_id, fields in stream.claim_stale(client, consumer_name, MIN_IDLE_MS):
                process(message_id, fields)

    except Exception:
        traceback.print_exc()
        time.sleep(1)