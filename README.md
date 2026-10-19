# orion.

Orion is a self-directed, lightweight DAG workflow orchestrator in the spirit of Apache Airflow. You define a **DAG** (a named set of **tasks** with dependencies), trigger a **run**, and a pool of workers executes the tasks in dependency order while recording status and logs in Postgres.

It is a monorepo of four parts:

| Component | Path | Tech | Role |
| --- | --- | --- | --- |
| API | `orion-api/` | Go 1.26, `net/http`, pgx, go-redis, swaggo | REST API; owns DAG/task/run CRUD, triggers runs, enqueues ready work |
| Worker | `orion-worker/` | Python, psycopg2, redis-py | Consumes the queue, executes tasks, advances the DAG |
| UI | `orion-ui/` | Next.js 16, React 19, Tailwind v4, shadcn, React Flow | Web console (currently a static skeleton) |
| Infra | `orion-infra/` | Docker Compose | Local Postgres 16, Redis 7 and the worker container |

Schema migrations live in `migrations/`.

## Status

Orion is a work in progress on branch `feature/python-worker`. The backend pipeline works end to end for **manual runs of Python tasks**. The UI is not connected to the backend yet.

| Area | State |
| --- | --- |
| Postgres schema + migrations | Done (4 migrations) |
| API: DAG, task, run, task-instance endpoints | Done (a few gaps, see [Known gaps](#known-gaps)) |
| API: Redis Streams queue + consumer groups | Done, with unit and end-to-end tests |
| API tests | Done (HTTP and queue; need Postgres + Redis running) |
| Swagger docs | Generated, served at `/swagger/` |
| Worker: consume, execute `python` tasks, mark status, write logs | Working |
| Worker: DAG progression (enqueue downstream tasks, skip on failure, finalize run) | Working |
| Worker: crash recovery via `XAUTOCLAIM` | Working |
| Worker: stuck-`queued` sweeper (re-enqueues instances never delivered) | Working |
| Scheduler (cron `schedule` field) | **Not implemented**; the field is stored but never acted on |
| Task retries | **Not implemented**; the `retrying` status and `attempt` column exist but are unused |
| Other task types (`shell`, `http`) | **Not implemented**; the schema allows any `task_type`, the worker only knows `python` |
| UI | Static skeleton on hardcoded sample data; no API calls (see `orion-ui/README.md`) |
| Auth | None |
| Worker container | Done: `orion-worker/Dockerfile`, a `worker` service in compose, Python 3.10 to 3.13 installed |
| Dockerfiles for API/UI | None |

## How it works

```
        POST /dags/{id}/runs
 UI ───────────────────────────► Go API ──────────────► Postgres
                                   │   1. insert run (queued)           ▲
                                   │   2. insert a task_instance        │
                                   │      (pending) per task            │
                                   │   3. XADD instances with no        │
                                   │      dependencies                  │
                                   ▼                                    │
                             Redis Stream  ◄──── XADD downstream ───┐   │
                          (consumer group)                          │   │
                                   │ XREADGROUP                     │   │
                                   ▼                                │   │
                            Python worker(s) ───────────────────────┴───┘
                              run task in subprocess, update status/logs
```

1. **Define.** `POST /dags` creates a DAG, then `POST /dags/{dagId}/tasks` adds tasks with a `task_type` and JSON `config`.
2. **Trigger.** `POST /dags/{dagId}/runs` creates a `runs` row (`queued`) and one `task_instances` row (`pending`) per task. Instances whose task has no dependencies are pushed onto the Redis Stream. The response lists the run, instances and Redis message IDs.
3. **Execute.** A worker reads a message with `XREADGROUP`, atomically flips the instance `queued → running` (only one worker can win), loads the task, and runs it.
4. **Advance.**
   - On success the worker marks the instance `success`, then flips every `pending` downstream instance whose upstreams are all successful to `queued` and enqueues it.
   - On failure the worker marks it `failed` and marks all transitive downstream `pending` instances `skipped` (recursive CTE).
   - The worker writes stdout/stderr/timeout info to `task_instances.logs`, then finalizes the run: it becomes `failed` if any instance failed, `success` otherwise, once nothing is pending/queued/running/retrying.
5. **Acknowledge.** The message is `XACK`ed only after Postgres is updated and children are enqueued.

### Crash tolerance

Every 30 seconds each worker runs `XAUTOCLAIM` to take over messages that another consumer received but never acked within 120 s (`MIN_IDLE_MS`). The per-task timeout is capped at 100 s so a healthy task can never be mistaken for a dead one. The `queued → running` transition is guarded by `WHERE status = 'queued'`, so a duplicate delivery is dropped instead of re-executed. A reclaimed task is re-run, so tasks should be safe to run twice.

The same loop also runs a sweeper: an instance that is `queued` in Postgres for more than 300 s (`STALE_QUEUED_S`) is re-enqueued. This covers a crash between a worker flipping children to `queued` and the `XADD`, and an API enqueue failure. `queued_at` (migration 4) records when an instance was queued; re-enqueueing a merely slow message is harmless because `mark_running` drops duplicates.

### Task execution (`python` type)

Task `config`:

```json
{
  "script": "print('hello from orion')",
  "env": { "MY_VAR": "value" },
  "timeout": 30
}
```

- `script` (required) runs via `python -c <script>` in a subprocess.
- `env` is the only environment passed besides `PATH` and `PYTHONUNBUFFERED=1`; the worker's own env (including DB credentials) is not inherited.
- `timeout` is in seconds. The default is 60 and the maximum is 100.
- stdout and stderr are each truncated to 10,000 characters.
- The DAG's `python_version` column (e.g. `3.11`) selects the interpreter; the worker looks up `python3.11` on `PATH` and fails the task if it is missing. When `NULL`, the worker's own interpreter is used.

Logs are stored as JSON text appended to `task_instances.logs`: `{"stdout": "...", "stderr": "...", "timed_out": false}`.

**Security note:** tasks are arbitrary code executed in a plain subprocess on the worker host with no sandboxing. Only run Orion where you trust everyone who can call the API (there is no auth).

## Data model

| Table | Purpose |
| --- | --- |
| `dags` | `name` (unique), `description`, `schedule` (cron, nullable), `owner`, `is_active`, `python_version` |
| `tasks` | Belongs to a DAG; `name` (unique per DAG), `task_type`, `config` (JSONB) |
| `task_dependencies` | `(task_id, depends_on_task_id)` edges; self-dependency is blocked by a CHECK |
| `runs` | One execution of a DAG; status `queued`/`running`/`success`/`failed`/`cancelled`; `triggered_by` |
| `task_instances` | One task within one run; status `pending`/`queued`/`running`/`success`/`failed`/`retrying`/`skipped`; `attempt`, `worker_id`, timestamps, `logs`, `output` (JSONB, currently unused) |

Deleting a DAG cascades to its tasks, dependencies, runs and instances.

## Getting started

### Prerequisites

Docker, Go 1.26+, Python 3.10+, Node.js 20+, and the [`migrate`](https://github.com/golang-migrate/migrate) CLI or `psql` for applying migrations.

### 1. Configure `.env` (repo root, git-ignored)

```bash
# Go API
DATABASE_URL=postgres://orion:orion@localhost:5432/orion?sslmode=disable
PORT=8080

# Infra (read by docker compose)
POSTGRES_USER=orion
POSTGRES_PASSWORD=orion
POSTGRES_DB=orion

# Queue: shared by the API and the worker
REDIS_ADDR=localhost:6379
STREAM_NAME=orion:tasks_stream
CONSUMER_GROUP=orion:workers
```

> The API and the worker read the same `.env` and the same variable names. `REDIS_ADDR` can be `host:port` for both: the worker adds the `redis://` scheme itself if it is missing. The worker also accepts an optional `MAX_LEN` (stream length cap, default 1000, the value the API uses).

### 2. Start Postgres and Redis

```bash
make up      # docker compose up -d --build (postgres :5432, redis :6379, plus one worker container)
make down    # stop (volumes are kept)
```

### 3. Apply migrations

```bash
migrate -path migrations -database "$DATABASE_URL" up
# or apply the *.up.sql files in order with psql
```

`000003_add_output_log_instances.up.sql` has no matching `.down.sql`. The API tests apply every `migrations/*.up.sql` to the throwaway `*_test` database.

### 4. Run the API

```bash
cd orion-api
go run .
```

Health check: `curl localhost:8080/health`. Interactive docs: <http://localhost:8080/swagger/>.

To regenerate Swagger after changing handler annotations: `swag init` (from `orion-api/`).

### 5. Run workers

`make up` already starts one containerized worker. To run more, `docker compose -f orion-infra/docker-compose.yml --env-file .env up -d --scale worker=3`. In the container the worker reads `postgres` and `redis` hostnames (set in the compose file, not your `.env`), and `docker stop` waits up to 110 s so a running task can finish. The image has Python 3.10, 3.11, 3.12 and 3.13, so a DAG's `python_version` must be one of those.

To run a worker on your machine instead:

```bash
cd orion-worker
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
cd worker && python main.py
```

Start more workers in other terminals to scale out; each gets a unique consumer name (`hostname-pid`) and they share the stream through the consumer group. Stop with Ctrl+C or SIGTERM; the worker finishes the current message and exits.

### 6. Run the UI

```bash
cd orion-ui
npm install
npm run dev     # http://localhost:3000
```

## Example: build and run a DAG

```bash
API=localhost:8080

# 1. DAG
DAG=$(curl -s -X POST $API/dags -H 'Content-Type: application/json' \
  -d '{"name":"hello","description":"demo","owner":"me"}' | jq -r .id)

# 2. Task
curl -s -X POST $API/dags/$DAG/tasks -H 'Content-Type: application/json' \
  -d '{"name":"say_hi","task_type":"python","config":{"script":"print(\"hi from orion\")"}}'

# 3. Trigger a run
curl -s -X POST $API/dags/$DAG/runs | jq

# 4. Inspect
curl -s $API/dags/$DAG/runs | jq
curl -s $API/task-instances/<instance-id> | jq
```

To chain tasks, make one depend on another (`load` runs only after `transform` succeeds):

```bash
curl -s -X POST $API/dags/$DAG/dependencies -H 'Content-Type: application/json' \
  -d '{"task_id":"<load-task-id>","depends_on_task_id":"<transform-task-id>"}'
```

Self-dependencies, duplicates and cycles are rejected (400 / 409).

A DAG's `python_version` (e.g. `"3.11"`) can be set on create or update (`POST /dags`, `PUT /dags/{id}`); it must look like `3.<minor>`.

## API reference

Served by `orion-api` on `PORT` (default 8080). Request and response bodies are JSON. CORS allows `http://localhost:3000` (the Next.js dev server) only. Errors are always `{"error": "..."}`.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness check |
| GET | `/dags` | List DAGs (newest first) |
| POST | `/dags` | Create a DAG (`name` required; `description`, `schedule`, `owner`, `python_version` optional) |
| GET | `/dags/{id}` | Get a DAG |
| PUT | `/dags/{id}` | Partial update (`name`, `description`, `schedule`, `owner`, `is_active`, `python_version`) |
| DELETE | `/dags/{id}` | Delete a DAG and everything under it |
| GET | `/dags/{dagId}/tasks` | List a DAG's tasks |
| POST | `/dags/{dagId}/tasks` | Create a task (`name`, `task_type`, `config`) |
| GET | `/tasks/{id}` | Get a task |
| PUT | `/tasks/{id}` | Replace a task's `name`, `task_type`, `config` |
| DELETE | `/tasks/{id}` | Delete a task |
| GET | `/dags/{dagId}/runs` | List runs for a DAG |
| POST | `/dags/{dagId}/runs` | Trigger a run; returns `{run, task_instances, enqueued}`. 404 for an unknown DAG, 400 for a DAG with no tasks. Atomic: nothing is left behind if it fails. |
| GET | `/runs/{id}` | Get a run |
| GET | `/dags/{dagId}/dependencies` | List a DAG's dependency edges |
| POST | `/dags/{dagId}/dependencies` | Add an edge (`task_id` runs after `depends_on_task_id`); 409 on duplicate or cycle |
| DELETE | `/dags/{dagId}/dependencies/{taskId}/{dependsOnId}` | Remove an edge |
| GET | `/runs/{runId}/task-instances` | List a run's task instances, including `logs`, `output`, `worker_id`, `attempt` and timestamps |
| GET | `/task-instances/{id}` | Get a task instance |

## Testing

The API tests are black-box tests against real Postgres and Redis:

```bash
make up
cd orion-api
go test ./tests/...
```

- They create a separate `<db>_test` database on the same server and never touch your dev data.
- Redis tests use uniquely named streams and delete them afterwards.
- Overrides: `TEST_DATABASE_URL`, `TEST_REDIS_ADDR` (default `localhost:6379`).
- Coverage: health, DAG/task CRUD and validation, run triggering (only dependency-free tasks are enqueued, once each), run/task-instance reads, queue semantics (idempotent group creation, FIFO, blocking reads, no duplicate delivery across consumers, ack idempotency, stale-claim recovery, `MAXLEN` trimming, context cancellation) and an end-to-end crashed-worker recovery test.
- There are no automated tests for the UI.

The worker has its own `pytest` suite (executor behaviour, and every database state transition: start guard, success/failure, downstream skipping, diamond join, run finalization, stale re-queue). It uses the same `<db>_test` database, so run the Go tests once first to create it:

```bash
cd orion-worker
.venv/bin/pip install -r requirements-dev.txt
.venv/bin/python -m pytest tests
```

## Repository layout

```
.
├── Makefile                     # up / down for the local stack
├── migrations/                  # SQL migrations (000001–000004)
├── orion-infra/docker-compose.yml
├── orion-api/
│   ├── main.go                  # wiring: env, DB pool, queue, routes, CORS, Swagger
│   ├── docs/                    # generated Swagger (docs.go, swagger.json/yaml)
│   ├── internal/
│   │   ├── db/                  # pgx pool
│   │   ├── handlers/            # dags, tasks, dependencies, runs, task_instances, health, routes
│   │   ├── models/              # DAG, Task, Run, TaskInstance, TaskDependency
│   │   └── queue/               # Redis Streams wrapper (Enqueue/Dequeue/Ack/ClaimStale)
│   └── tests/                   # HTTP, queue and end-to-end tests
├── orion-worker/
│   ├── Dockerfile               # worker image (Python 3.10-3.13)
│   ├── requirements.txt
│   ├── requirements-dev.txt     # adds pytest
│   ├── tests/                   # executor and database tests
│   └── worker/
│       ├── main.py              # consume loop, reclaim and sweeper timer, graceful shutdown
│       ├── db.py                # state transitions, DAG progression, logs
│       ├── executor.py          # task executors (python)
│       └── stream.py            # Redis Streams helpers
└── orion-ui/                    # Next.js app (see orion-ui/README.md)
```

## Known gaps

Things I found while documenting that you will hit when running the project:

1. **No scheduler.** `dags.schedule` is stored but nothing triggers runs from it; runs are manual only (`triggered_by` is never set to `schedule`).
2. **Worker trusts task code** (see the security note above) and no authentication exists anywhere.

## Roadmap

1. Wire the UI to the API (fetching, then live updates via SSE/WebSocket).
2. Add a scheduler for cron DAGs.
3. Implement retries, run cancellation, and additional task types (`shell`, `http`).
4. Containerize the API and UI; add CI.
5. Add auth.
