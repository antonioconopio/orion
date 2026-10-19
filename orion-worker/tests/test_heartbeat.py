import time

from heartbeat import Heartbeat
from helpers import FakeRedis


def make(**kw):
    r = FakeRedis()
    return r, Heartbeat(r, "host-123", prefix="test:worker:", **kw)


def test_start_registers_an_idle_worker_with_a_ttl():
    r, hb = make(ttl=15)
    hb.start()
    hb.stop()
    key, payload, ttl = r.sets[0]
    assert key == "test:worker:host-123" and ttl == 15
    assert payload["id"] == "host-123" and payload["status"] == "idle" and payload["current_instance_id"] is None
    assert {"host", "pid", "started_at"} <= payload.keys()


def test_busy_and_idle_are_written_immediately_and_announced():
    r, hb = make()
    hb.start()
    before = len(r.published)
    hb.busy("inst-1")
    assert r.sets[-1][1]["status"] == "busy" and r.sets[-1][1]["current_instance_id"] == "inst-1"
    hb.idle()
    assert r.sets[-1][1]["status"] == "idle" and r.sets[-1][1]["current_instance_id"] is None
    assert len(r.published) == before + 2 and all(e == {"type": "worker"} for e in r.published)
    hb.stop()


def test_repeating_the_same_state_is_a_no_op():
    r, hb = make()
    hb.start()
    hb.idle()
    hb.idle()
    n_sets, n_events = len(r.sets), len(r.published)
    hb.busy("x")
    hb.busy("x")
    assert len(r.sets) == n_sets + 1 and len(r.published) == n_events + 1
    hb.stop()


def test_the_background_thread_keeps_refreshing_the_key():
    r, hb = make(every=0.02)
    hb.start()
    time.sleep(0.2)
    hb.stop()
    assert len(r.sets) >= 4


def test_stop_removes_the_key_and_ends_the_thread():
    r, hb = make(every=0.02)
    hb.start()
    hb.stop()
    assert r.deleted == ["test:worker:host-123"] and not hb._thread.is_alive()
    count = len(r.sets)
    time.sleep(0.1)
    assert len(r.sets) == count    # no beats after stop


def test_redis_errors_are_swallowed():
    class Broken(FakeRedis):
        def set(self, *a, **k):
            raise ConnectionError("down")

        def delete(self, *a, **k):
            raise ConnectionError("down")

    hb = Heartbeat(Broken(fail_publish=True), "host-1", prefix="test:worker:")
    hb.start()
    hb.busy("x")
    hb.stop()
