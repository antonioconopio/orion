import redis


class RedisStream:
    def __init__(self, client: redis.Redis, stream_name: str, group_name: str, max_len: int):
        self.client = client
        self.stream_name = stream_name
        self.group_name = group_name
        self.max_len = max_len


def get_redis_stream(redis_addr: str, stream_name: str, group_name: str, max_len: int) -> RedisStream:
    r = redis.Redis.from_url(redis_addr, decode_responses=True)
    r.ping()  # fail fast if Redis isn't reachable

    try:
        r.xgroup_create(stream_name, group_name, id="$", mkstream=True)
    except redis.exceptions.ResponseError as e:
        if "BUSYGROUP" not in str(e):   # group already exists is fine
            raise

    return RedisStream(r, stream_name, group_name, max_len)


def enqueue(stream: RedisStream, task_instance_id: str) -> str:
    return stream.client.xadd(
        stream.stream_name,
        {"task_instance_id": task_instance_id},
        maxlen=stream.max_len,
        approximate=True,
    )


def dequeue(stream: RedisStream, consumer_name: str, count: int = 1, block_ms: int = 2000):
    resp = stream.client.xreadgroup(
        stream.group_name,
        consumer_name,
        {stream.stream_name: ">"},
        count=count,
        block=block_ms,
    )
    # [[stream, [(id, fields), ...]]]  ->  [(id, fields), ...]
    return [msg for _stream, msgs in resp for msg in msgs] if resp else []


def acknowledge(stream: RedisStream, message_id: str) -> None:
    stream.client.xack(stream.stream_name, stream.group_name, message_id)


def claim_stale(stream: RedisStream, consumer_name: str, min_idle_ms: int, count: int = 5):
    result = stream.client.xautoclaim(
        stream.stream_name,
        stream.group_name,
        consumer_name,
        min_idle_time=min_idle_ms,
        start_id="0-0",
        count=count,
    )
    return result[1]   # [(id, fields), ...]