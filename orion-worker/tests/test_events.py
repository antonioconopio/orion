import events
from helpers import FakeRedis


def test_run_changed_publishes_the_shape_the_api_forwards():
    r = FakeRedis()
    events.run_changed(r, "run-1", "dag-1")
    assert r.published == [{"type": "run", "run_id": "run-1", "dag_id": "dag-1"}]


def test_a_redis_failure_never_raises():
    events.run_changed(FakeRedis(fail_publish=True), "run-1", "dag-1")   # must simply return
