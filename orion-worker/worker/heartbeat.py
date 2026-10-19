import json
import os
import socket
import threading
from datetime import datetime, timezone

import events

KEY_PREFIX = "orion:worker:"   # the API lists workers by scanning this prefix
EVERY = 5                      # seconds between beats
TTL = 15                       # a worker silent this long drops off the list


class Heartbeat:
    """Writes this worker's liveness to Redis. A background thread keeps the key fresh even
    while a long task blocks the main loop; if the process dies the key simply expires."""

    def __init__(self, rdb, worker_id, every=EVERY, ttl=TTL, prefix=KEY_PREFIX):
        self.rdb, self.worker_id, self.every, self.ttl = rdb, worker_id, every, ttl
        self.key = prefix + worker_id
        self.started_at = datetime.now(timezone.utc).isoformat()
        self._lock = threading.Lock()
        self._status, self._instance_id = "idle", None
        self._stop = threading.Event()
        self._thread = None

    def start(self):
        self._write()
        events.publish(self.rdb, {"type": "worker"})
        self._thread = threading.Thread(target=self._loop, daemon=True)
        self._thread.start()

    def stop(self):
        """Clean shutdown: leave the list right away instead of waiting for the TTL."""
        self._stop.set()
        if self._thread:
            self._thread.join()
        try:
            self.rdb.delete(self.key)
        except Exception as e:
            print(f"heartbeat cleanup failed: {e!r}", flush=True)
        events.publish(self.rdb, {"type": "worker"})

    def busy(self, instance_id: str):
        self._set("busy", instance_id)

    def idle(self):
        self._set("idle", None)

    def _set(self, status, instance_id):
        with self._lock:
            if (self._status, self._instance_id) == (status, instance_id):
                return
            self._status, self._instance_id = status, instance_id
        self._write()
        events.publish(self.rdb, {"type": "worker"})

    def _loop(self):
        while not self._stop.wait(self.every):
            self._write()

    def _write(self):
        with self._lock:
            payload = {
                "id": self.worker_id,
                "host": socket.gethostname(),
                "pid": os.getpid(),
                "started_at": self.started_at,
                "status": self._status,
                "current_instance_id": self._instance_id,
            }
        try:
            self.rdb.set(self.key, json.dumps(payload), ex=self.ttl)
        except Exception as e:
            print(f"heartbeat write failed: {e!r}", flush=True)
