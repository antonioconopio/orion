# orion.

Orion is a self-directed, lightweight DAG workflow orchestrator in the spirit of Apache Airflow. You define a **DAG** (a named set of **tasks** with dependencies), trigger a **run**, and a pool of workers executes the tasks in dependency order while recording status and logs in Postgres.

It is a monorepo of four parts:

| Component | Path | Tech | Role |
| --- | --- | --- | --- |
| API | `orion-api/` | Go 1.26, `net/http`, pgx, go-redis, swaggo | REST API; owns DAG/task/run CRUD, triggers runs, enqueues ready work |
| Worker | `orion-worker/` | Python, psycopg2, redis-py | Consumes the queue, executes tasks, advances the DAG |
| UI | `orion-ui/` | Next.js 16, React 19, Tailwind v4, shadcn, React Flow | Web console (currently a static skeleton) |
| Infra | `orion-infra/` | Docker Compose | Local Postgres 16 and Redis 7 |

Schema migrations live in `migrations/`.

## Status

Orion is a work in progress on branch `feature/python-worker`. The backend pipeline works end to end for **manual runs of Python tasks**. The UI is not connected to the backend yet.

| Area | State |
| --- | --- |
| Postgres schema + migrations | Done (3 migrations) |
| API: DAG, task, run, task-instance endpoints | Done (a few gaps, see [Known gaps](#known-gaps)) |
| API: Redis Streams queue + consumer groups | Done, with unit and end-to-end tests |
| API tests | Done (HTTP and queue; need Postgres + Redis running) |
| Swagger docs | Generated, served at `/swagger/` |
| Worker: consume, execute `python` tasks, mark status, write logs | Working |
| Worker: DAG progression (enqueue downstream tasks, skip on failure, finalize run) | Working |
| Worker: crash recovery via `XAUTOCLAIM` | Working |
| Scheduler (cron `schedule` field) | **Not implemented**; the field is stored but never acted on |
| Task retries | **Not implemented**; the `retrying` status and `attempt` column exist but are unused |
| Other task types (`shell`, `http`) | **Not implemented**; the schema allows any `task_type`, the worker only knows `python` |
| UI | Static skeleton on hardcoded sample data; no API calls (see `orion-ui/README.md`) |
| Auth | None |
| Dockerfiles for API/worker/UI | None; only Postgres and Redis are containerized |

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

Every 30 seconds each worker runs `XAUTOCLAIM` to take over messages that another consumer received but never acked within 120 s (`MIN_IDLE_MS`). The per-task timeout is capped at 100 s so a healthy task can never be mistaken for a dead one. The `queued → running` transition is guarded by `WHERE status = 'queued'`, so a duplicate delivery is dropped instead of re-executed.

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

# Queue: API variables
REDIS_ADDR=localhost:6379
STREAM_NAME=orion:tasks_stream
CONSUMER_GROUP=orion:workers

# Queue: extra worker variables
GROUP_NAME=orion:workers
MAX_LEN=1000
```

> The API and worker read the same `.env` but use **different variable names and formats** for the queue (see [Known gaps](#known-gaps)). `GROUP_NAME` must equal `CONSUMER_GROUP`, and `STREAM_NAME` must be identical. `REDIS_ADDR` is `host:port` for the API, but the worker passes it to `redis.Redis.from_url`, which expects a URL such as `redis://localhost:6379`. To run both from one file, either use separate env files or give the worker its value inline: `REDIS_ADDR=redis://localhost:6379 python worker/main.py`.

### 2. Start Postgres and Redis

```bash
make up      # docker compose up -d (postgres :5432, redis :6379, with healthchecks and volumes)
make down    # stop (volumes are kept)
```

### 3. Apply migrations

```bash
migrate -path migrations -database "$DATABASE_URL" up
# or apply the *.up.sql files in order with psql
```

`000003_add_output_log_instances.up.sql` has no matching `.down.sql`.

### 4. Run the API

```bash
cd orion-api
go run .
```

Health check: `curl localhost:8080/health`. Interactive docs: <http://localhost:8080/swagger/>.

To regenerate Swagger after changing handler annotations: `swag init` (from `orion-api/`).

### 5. Run a worker

```bash
cd orion-worker
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt python-dotenv
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

Dependencies cannot be created through the API yet. To chain tasks, insert rows directly:

```sql
INSERT INTO task_dependencies (task_id, depends_on_task_id) VALUES ('<child-task-id>', '<parent-task-id>');
```

Setting a DAG's `python_version` also currently requires SQL (`UPDATE dags SET python_version = '3.11' WHERE id = ...`).

## API reference

Served by `orion-api` on `PORT` (default 8080). Request and response bodies are JSON. CORS currently allows `http://localhost:5173` only.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness check |
| GET | `/dags` | List DAGs (newest first) |
| POST | `/dags` | Create a DAG (`name` required; `description`, `schedule`, `owner` optional) |
| GET | `/dags/{id}` | Get a DAG |
| PUT | `/dags/{id}` | Partial update (`name`, `description`, `schedule`, `owner`, `is_active`) |
| DELETE | `/dags/{id}` | Delete a DAG and everything under it |
| GET | `/dags/{dagId}/tasks` | List a DAG's tasks |
| POST | `/dags/{dagId}/tasks` | Create a task (`name`, `task_type`, `config`) |
| GET | `/tasks/{id}` | Get a task |
| PUT | `/tasks/{id}` | Replace a task's `name`, `task_type`, `config` |
| DELETE | `/tasks/{id}` | Delete a task |
| GET | `/dags/{dagId}/runs` | List runs for a DAG |
| POST | `/dags/{dagId}/runs` | Trigger a run; returns `{run, task_instances, enqueued}` |
| GET | `/runs/{id}` | Get a run |
| GET | `/dags/{runId}/task-instances` | List a run's task instances (the path segment is a **run** id; Swagger labels it `dagId`) |
| GET | `/task-instances/{id}` | Get a task instance |

Error handling is not uniform yet: DAG endpoints return `{"error": "..."}`, while task list/get, run and task-instance endpoints often return plain-text errors.

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
- There are no automated tests for the Python worker or the UI.

## Repository layout

```
.
├── Makefile                     # up / down for the local stack
├── migrations/                  # SQL migrations (000001–000003)
├── orion-infra/docker-compose.yml
├── orion-api/
│   ├── main.go                  # wiring: env, DB pool, queue, routes, CORS, Swagger
│   ├── docs/                    # generated Swagger (docs.go, swagger.json/yaml)
│   ├── internal/
│   │   ├── db/                  # pgx pool
│   │   ├── handlers/            # dags, tasks, runs, task_instances, health, routes
│   │   ├── models/              # DAG, Task, Run, TaskInstance, TaskDependency
│   │   └── queue/               # Redis Streams wrapper (Enqueue/Dequeue/Ack/ClaimStale)
│   └── tests/                   # HTTP, queue and end-to-end tests
├── orion-worker/
│   ├── requirements.txt
│   └── worker/
│       ├── main.py              # consume loop, reclaim timer, graceful shutdown
│       ├── db.py                # state transitions, DAG progression, logs
│       ├── executor.py          # task executors (python)
│       └── stream.py            # Redis Streams helpers
└── orion-ui/                    # Next.js app (see orion-ui/README.md)
```

## Known gaps

Things I found while documenting that you will hit when running the project:

1. **No scheduler.** `dags.schedule` is stored but nothing triggers runs from it; runs are manual only (`triggered_by` is never set to `schedule`).
2. **No API for dependencies or `python_version`.** Both are only settable through SQL. Create/Update DAG requests do not accept `python_version`.
3. **Env var mismatch between API and worker.** The API reads `CONSUMER_GROUP`; the worker reads `GROUP_NAME` and `MAX_LEN`, which are not in the existing `.env` convention. `REDIS_ADDR` is `host:port` for Go but must be a `redis://` URL for Python.
4. **`python-dotenv` is imported by the worker but missing from `requirements.txt`.**
5. **The worker is a script that runs on import** (no `if __name__ == "__main__"`, and env is read at module level), so it needs to be run from the `worker/` directory with the sibling imports resolving.
6. **Run status never becomes `running`.** The worker only finalizes a run to `success`/`failed`; `runs.started_at` is never set, and a run whose trigger enqueued nothing (e.g. a DAG with no tasks) stays `queued`.
7. **API responses omit data the worker writes.** `Run` and `TaskInstance` reads return only id/dag/status (and `created_at` for runs); `logs`, `output`, `worker_id`, `attempt`, `started_at` and `finished_at` are not returned yet, so the UI cannot show logs from the API. The `TaskInstance` model still has a stale `log_location` field from before migration 2.
8. **Triggering is not transactional.** A failure midway (e.g. Redis down after instances are inserted) leaves a partially created run, and triggering a nonexistent DAG id returns 500 rather than 404.
9. **`/dags/{runId}/task-instances` is a misleading route name** and the Swagger annotation names the parameter `dagId`.
10. **CORS origin is `http://localhost:5173`**, but the Next.js dev server runs on `:3000`; update `main.go` before connecting the UI.
11. **Stray files:** `orion-api/go.mod` marks directly used modules (godotenv, go-redis, cors, swaggo) as `// indirect`; `go mod tidy` will fix this.
12. **Worker trusts task code** (see the security note above) and no authentication exists anywhere.

## Roadmap

1. Wire the UI to the API (fetching, then live updates via SSE/WebSocket).
2. Expose logs, timestamps and worker info through the API; add dependency and `python_version` endpoints.
3. Add a scheduler for cron DAGs.
4. Implement retries, run cancellation, and additional task types (`shell`, `http`).
5. Containerize the API, worker and UI; add CI.
6. Add worker tests and auth.
