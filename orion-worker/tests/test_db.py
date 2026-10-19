import pytest

import db
from helpers import seed, status, set_status, run_status

OLD = "NOW() - INTERVAL '10 minutes'"


DIAMOND = dict(tasks=["a", "b", "c", "d"], deps=[("b", "a"), ("c", "a"), ("d", "b"), ("d", "c")])


# ---- mark_running ----

def test_mark_running_only_one_worker_wins(cur):
    run_id, inst = seed(cur, ["a"], status="queued")
    assert db.mark_running(cur, inst["a"], "w1") is True
    assert db.mark_running(cur, inst["a"], "w2") is False   # duplicate delivery is dropped
    cur.execute("SELECT worker_id, started_at IS NOT NULL FROM task_instances WHERE id = %s", (inst["a"],))
    assert cur.fetchone() == ("w1", True)


def test_mark_running_marks_the_run_running(cur):
    run_id, inst = seed(cur, ["a"], status="queued")
    assert run_status(cur, run_id)[0] == "queued"
    db.mark_running(cur, inst["a"], "w1")
    st, started, finished = run_status(cur, run_id)
    assert st == "running" and started is not None and finished is None


def test_mark_running_ignores_pending_and_finished(cur):
    _, inst = seed(cur, ["a", "b"], status="pending")
    assert db.mark_running(cur, inst["a"], "w") is False
    set_status(cur, inst["b"], "success")
    assert db.mark_running(cur, inst["b"], "w", reclaimed=True) is False   # reclaim must not rerun finished work


def test_mark_running_reclaim_restarts_a_running_instance(cur):
    _, inst = seed(cur, ["a"], status="queued")
    db.mark_running(cur, inst["a"], "dead-worker")
    assert db.mark_running(cur, inst["a"], "w2") is False                  # not a reclaim: left alone
    assert db.mark_running(cur, inst["a"], "w2", reclaimed=True) is True
    cur.execute("SELECT worker_id FROM task_instances WHERE id = %s", (inst["a"],))
    assert cur.fetchone()[0] == "w2"


# ---- mark_success / mark_failed ----

def test_mark_success_sets_status_and_logs_together(cur):
    run_id, inst = seed(cur, ["a"], status="running")
    assert db.mark_success(cur, inst["a"], "\nhello") == run_id
    cur.execute("SELECT status, logs, finished_at IS NOT NULL FROM task_instances WHERE id = %s", (inst["a"],))
    assert cur.fetchone() == ("success", "\nhello", True)


def test_mark_success_appends_to_existing_logs(cur):
    _, inst = seed(cur, ["a"], status="running")
    cur.execute("UPDATE task_instances SET logs = 'first' WHERE id = %s", (inst["a"],))
    db.mark_success(cur, inst["a"], "\nsecond")
    cur.execute("SELECT logs FROM task_instances WHERE id = %s", (inst["a"],))
    assert cur.fetchone()[0] == "first\nsecond"


@pytest.mark.parametrize("fn", [db.mark_success, db.mark_failed])
def test_mark_requires_a_running_instance(cur, fn):
    _, inst = seed(cur, ["a"], status="queued")
    with pytest.raises(RuntimeError):
        fn(cur, inst["a"], "log")
    assert status(cur, inst["a"]) == "queued"


def test_mark_failed_skips_all_downstream_and_nothing_else(cur):
    run_id, inst = seed(cur, ["a", "b", "c", "d", "e"], deps=DIAMOND["deps"], status="pending")
    set_status(cur, inst["a"], "running")
    assert db.mark_failed(cur, inst["a"], "\nboom") == run_id
    assert status(cur, inst["a"]) == "failed"
    assert [status(cur, inst[n]) for n in "bcd"] == ["skipped"] * 3     # transitive
    assert status(cur, inst["e"]) == "pending"                           # unrelated task untouched
    cur.execute("SELECT logs FROM task_instances WHERE id = %s", (inst["a"],))
    assert cur.fetchone()[0] == "\nboom"


def test_mark_failed_does_not_touch_siblings_that_already_ran(cur):
    _, inst = seed(cur, DIAMOND["tasks"], deps=DIAMOND["deps"])
    set_status(cur, inst["a"], "success")
    set_status(cur, inst["b"], "success")
    set_status(cur, inst["c"], "running")
    db.mark_failed(cur, inst["c"], "")
    assert status(cur, inst["b"]) == "success"
    assert status(cur, inst["d"]) == "skipped"


# ---- claim_ready_tasks ----

def test_claim_ready_tasks_waits_for_every_parent(cur):
    _, inst = seed(cur, DIAMOND["tasks"], deps=DIAMOND["deps"])
    set_status(cur, inst["a"], "success")
    assert sorted(db.claim_ready_tasks(cur, inst["a"])) == sorted([inst["b"], inst["c"]])
    assert status(cur, inst["d"]) == "pending"

    set_status(cur, inst["b"], "success")                  # c still running
    assert db.claim_ready_tasks(cur, inst["b"]) == []
    assert status(cur, inst["d"]) == "pending"

    set_status(cur, inst["c"], "success")
    assert db.claim_ready_tasks(cur, inst["c"]) == [inst["d"]]
    assert status(cur, inst["d"]) == "queued"
    assert db.claim_ready_tasks(cur, inst["c"]) == []      # a second claim must not hand it out again
    assert db.claim_ready_tasks(cur, inst["b"]) == []


def test_claim_ready_tasks_stamps_queued_at(cur):
    _, inst = seed(cur, ["a", "b"], deps=[("b", "a")])
    set_status(cur, inst["a"], "success")
    db.claim_ready_tasks(cur, inst["a"])
    cur.execute("SELECT queued_at IS NOT NULL FROM task_instances WHERE id = %s", (inst["b"],))
    assert cur.fetchone()[0] is True


def test_claim_ready_tasks_stays_inside_its_own_run(cur):
    run1, inst1 = seed(cur, ["a", "b"], deps=[("b", "a")])
    cur.execute("SELECT dag_id FROM runs WHERE id = %s", (run1,))
    cur.execute("INSERT INTO runs (dag_id, number) VALUES (%s, 2) RETURNING id", (cur.fetchone()[0],))
    run2 = cur.fetchone()[0]
    cur.execute("INSERT INTO task_instances (run_id, task_id, status) "
                "SELECT %s, task_id, 'pending' FROM task_instances WHERE run_id = %s", (run2, run1))
    cur.connection.commit()
    set_status(cur, inst1["a"], "success")
    assert db.claim_ready_tasks(cur, inst1["a"]) == [inst1["b"]]
    cur.execute("SELECT count(*) FROM task_instances WHERE run_id = %s AND status = 'queued'", (run2,))
    assert cur.fetchone()[0] == 0


# ---- finalize_run ----

def test_finalize_run_success(cur):
    run_id, inst = seed(cur, ["a", "b"], status="success")
    db.finalize_run(cur, run_id)
    st, _, finished = run_status(cur, run_id)
    assert st == "success" and finished is not None


def test_finalize_run_failed_if_any_failed(cur):
    run_id, inst = seed(cur, ["a", "b", "c"], status="success")
    set_status(cur, inst["b"], "failed")
    set_status(cur, inst["c"], "skipped")
    db.finalize_run(cur, run_id)
    assert run_status(cur, run_id)[0] == "failed"


@pytest.mark.parametrize("unfinished", ["pending", "queued", "running", "retrying"])
def test_finalize_run_waits_for_unfinished_work(cur, unfinished):
    run_id, inst = seed(cur, ["a", "b"], status="success")
    set_status(cur, inst["b"], unfinished)
    db.finalize_run(cur, run_id)
    st, _, finished = run_status(cur, run_id)
    assert st == "queued" and finished is None


def test_finalize_run_is_idempotent_and_never_reopens(cur):
    run_id, inst = seed(cur, ["a"], status="success")
    db.finalize_run(cur, run_id)
    first = run_status(cur, run_id)
    db.finalize_run(cur, run_id)
    assert run_status(cur, run_id) == first
    cur.execute("UPDATE runs SET status = 'cancelled' WHERE id = %s", (run_id,))
    cur.connection.commit()
    db.finalize_run(cur, run_id)
    assert run_status(cur, run_id)[0] == "cancelled"


# ---- requeue_stale ----

def test_requeue_stale_returns_old_queued_instances_once(cur):
    _, inst = seed(cur, ["old", "fresh", "running"], status="queued")
    cur.execute(f"UPDATE task_instances SET queued_at = {OLD} WHERE id = %s", (inst["old"],))
    cur.execute("UPDATE task_instances SET queued_at = NOW() WHERE id = %s", (inst["fresh"],))
    cur.execute(f"UPDATE task_instances SET status = 'running', queued_at = {OLD} WHERE id = %s", (inst["running"],))
    cur.connection.commit()
    assert db.requeue_stale(cur, 300) == [inst["old"]]
    assert db.requeue_stale(cur, 300) == []     # queued_at was bumped, so another worker will not re-enqueue it


def test_requeue_stale_ignores_instances_without_queued_at(cur):
    seed(cur, ["a"], status="queued")           # rows from before migration 4
    assert db.requeue_stale(cur, 0) == []


# ---- lookups ----

def test_get_task_and_python_version(cur):
    _, inst = seed(cur, ["a"], python_version="3.11", config='{"script": "print(1)"}')
    assert db.get_task(cur, inst["a"]) == ("python", {"script": "print(1)"})
    assert db.get_python_version(cur, inst["a"]) == "3.11"


def test_lookups_return_none_for_unknown_instance(cur):
    unknown = "00000000-0000-0000-0000-000000000000"
    assert db.get_task(cur, unknown) is None
    assert db.get_python_version(cur, unknown) is None

