"""process() end to end: real Postgres, fake Redis."""
import executor
import main
import stream
from heartbeat import Heartbeat
from helpers import FakeRedis, seed, set_status, status, run_status

DIAMOND = dict(tasks=["a", "b", "c", "d"], deps=[("b", "a"), ("c", "a"), ("d", "b"), ("d", "c")])


def setup_run(conn, script="print('hi')"):
    """A diamond whose root is queued (as the API leaves it) and whose other tasks wait."""
    with conn.cursor() as cur:
        run_id, inst = seed(cur, **DIAMOND, config='{"script": "%s"}' % script.replace('"', '\\"'))
        for name in "bcd":
            set_status(cur, inst[name], "pending")
        set_status(cur, inst["a"], "queued")
    return run_id, inst


def run_process(pool, inst_id, redis=None, message_id="1-0", hb=None, reclaimed=False):
    redis = redis or FakeRedis()
    client = stream.RedisStream(redis, "s", "g", 100)
    main.process(pool, client, "w1", message_id, {"task_instance_id": inst_id}, reclaimed=reclaimed, hb=hb)
    return redis


def test_success_advances_the_dag(conn, pool):
    run_id, inst = setup_run(conn)
    redis = run_process(pool, inst["a"])
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "success"
        assert [status(cur, inst[n]) for n in "bc"] == ["queued", "queued"]
        assert status(cur, inst["d"]) == "pending"
        cur.execute("SELECT logs FROM task_instances WHERE id = %s", (inst["a"],))
        assert '"stdout": "hi\\n"' in cur.fetchone()[0]
        assert run_status(cur, run_id)[0] == "running"
    assert sorted(redis.added) == sorted([inst["b"], inst["c"]])   # children enqueued
    assert redis.acked == ["1-0"]


def test_failure_skips_downstream_and_finishes_the_run(conn, pool):
    run_id, inst = setup_run(conn, script="raise ValueError('boom')")
    redis = run_process(pool, inst["a"])
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "failed"
        assert [status(cur, inst[n]) for n in "bcd"] == ["skipped"] * 3
        assert run_status(cur, run_id)[0] == "failed"
        cur.execute("SELECT logs FROM task_instances WHERE id = %s", (inst["a"],))
        assert "ValueError: boom" in cur.fetchone()[0]
    assert redis.added == [] and redis.acked == ["1-0"]


def test_events_say_the_run_changed_when_it_starts_and_when_it_is_done(conn, pool):
    run_id, inst = setup_run(conn)
    redis = run_process(pool, inst["a"])
    run_events = [e for e in redis.published if e["type"] == "run"]
    assert len(run_events) == 2 and all(e["run_id"] == run_id for e in run_events)
    assert run_events[0]["dag_id"]


def test_worker_state_is_busy_during_the_task_and_idle_after(conn, pool):
    run_id, inst = setup_run(conn)
    redis = FakeRedis()
    hb = Heartbeat(redis, "w1", prefix="test:worker:")
    run_process(pool, inst["a"], redis=redis, hb=hb)
    states = [(p["status"], p["current_instance_id"]) for _, p, _ in redis.sets]
    assert states == [("busy", inst["a"]), ("idle", None)]


def test_a_duplicate_delivery_is_acked_without_running_or_publishing(conn, pool):
    run_id, inst = setup_run(conn)
    with conn.cursor() as cur:
        set_status(cur, inst["a"], "running")
    redis = run_process(pool, inst["a"])
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "running"      # untouched
    assert redis.acked == ["1-0"] and redis.published == [] and redis.added == []


def test_a_reclaimed_running_instance_is_run(conn, pool):
    run_id, inst = setup_run(conn)
    with conn.cursor() as cur:
        set_status(cur, inst["a"], "running")           # a crashed worker left it like this
    run_process(pool, inst["a"], reclaimed=True)
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "success"


def test_a_redis_outage_does_not_fail_the_task(conn, pool):
    run_id, inst = setup_run(conn)
    run_process(pool, inst["a"], redis=FakeRedis(fail_publish=True))
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "success"


def test_an_executor_crash_fails_the_task_instead_of_leaving_it_running(conn, pool, monkeypatch):
    run_id, inst = setup_run(conn)
    monkeypatch.setattr(executor, "execute", lambda *a, **k: 1 / 0)
    run_process(pool, inst["a"])
    with conn.cursor() as cur:
        assert status(cur, inst["a"]) == "failed"
        cur.execute("SELECT logs FROM task_instances WHERE id = %s", (inst["a"],))
        assert "executor crashed" in cur.fetchone()[0]

