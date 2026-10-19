import json

CHANNEL = "orion:events"   # the Go API's hub subscribes to this and forwards to browsers


def publish(rdb, event: dict) -> None:
    """Tell browsers something changed. Events are only hints (the database is the source of
    truth), so a failure here must never fail a task."""
    try:
        rdb.publish(CHANNEL, json.dumps(event))
    except Exception as e:
        print(f"event publish failed: {e!r}", flush=True)


def run_changed(rdb, run_id: str, dag_id: str) -> None:
    publish(rdb, {"type": "run", "run_id": run_id, "dag_id": dag_id})
