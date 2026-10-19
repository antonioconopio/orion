# orion.

Orion is a self-directed, lightweight DAG workflow orchestrator in the spirit of Apache Airflow. You define a **DAG** (a named set of **tasks** with dependencies), trigger a **run**, and a pool of workers executes the tasks in dependency order while recording status and logs in Postgres.

It is a monorepo of four parts:

| Component | Path | Tech | Role |
| --- | --- | --- | --- |
| API | `orion-api/` | Go 1.26, `net/http`, pgx, go-redis, swaggo | REST API; owns DAG/task/run CRUD, triggers runs, enqueues ready work |
| Worker | `orion-worker/` | Python, psycopg2, redis-py | Consumes the queue, executes tasks, advances the DAG |
| UI | `orion-ui/` | Next.js 16, React 19, Tailwind v4, shadcn, React Flow | Web console: DAGs, runs, live task graph, logs, workers (live over WebSocket) |
| Infra | `orion-infra/` | Docker Compose | Postgres 16, Redis 7, migrations, and the API, UI and worker containers |

Schema migrations live in `migrations/`.

## Status

Orion is a work in progress on branch `feature/python-worker`. The backend pipeline works end to end for **manual runs of Python tasks**. The UI is wired to the API and updates live over a WebSocket.

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
| UI | Wired to the API (RTK Query); runs, task graphs and workers update live (see `orion-ui/README.md`) |
| Live updates | Workers and the API publish events to Redis Pub/Sub; the API forwards them over `GET /ws` |
| Worker visibility | Each worker heartbeats to Redis; `GET /workers` lists the live ones |
| Auth | None |
| Worker container | Done: `orion-worker/Dockerfile`, a `worker` service in compose, Python 3.10 to 3.13 installed |
| Containers | Everything runs in Docker Compose: Postgres, Redis, a one-shot migration step, API, UI and worker (see Quick start) |

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

### Live updates

Browsers learn about changes through a WebSocket, but the WebSocket never carries data, only a hint:

```
 worker / API ──PUBLISH──► Redis channel orion:events ──► API hub ──► every browser at GET /ws
                                                                          │
                             browser marks the affected data stale ◄──────┘ then refetches over REST
```

- After each commit that changes a run (a task starting, finishing, children queued or skipped, the run finishing) the worker publishes `{"type":"run","run_id":...,"dag_id":...}`; the trigger endpoint does the same for a new run; workers publish `{"type":"worker"}` when they become busy, idle or leave.
- The browser turns each hint into "this cached data is stale" (RTK Query tags) and refetches it. The database stays the source of truth, so a lost event (Pub/Sub drops messages when nobody listens) is harmless: on reconnect the UI refetches everything.
- A slow browser is dropped by the hub rather than stalling the others; the UI reconnects with exponential backoff (1 s up to 15 s).
- Publishing is best-effort: a Redis failure is logged and never fails a task or a request.
- `UI_ORIGIN` (default `http://localhost:3000`) is the one browser origin allowed for both CORS and the WebSocket.

**Workers.** There is no worker table. Each worker writes `orion:worker:<id>` to Redis every 5 s with a 15 s expiry (from a background thread, so a long task cannot stall it). A cleanly stopped worker deletes its key at once; a killed one drops off within about 15 s. A killed worker sends no event, so the UI polls `/workers` every 10 s as well.

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

#### Passing data between tasks

A task publishes a result, and the tasks that depend on it read it. Both go through files, so stdout stays free for logs:

- **`ORION_OUTPUT`**: the path of a file. Write JSON to it to publish this task's output.
- **`ORION_INPUTS`**: the path of a JSON file mapping each direct parent task's *name* to its output (`null` if that parent published nothing). A task with no parents gets `{}`.

```python
# task "total", which depends on "extract"
import json, os
numbers = json.load(open(os.environ["ORION_INPUTS"]))["extract"]["numbers"]
json.dump({"sum": sum(numbers)}, open(os.environ["ORION_OUTPUT"], "w"))
```

Rules:
- Output must be valid JSON of at most **64 KB**. Anything else fails the task (invalid JSON, over the limit, `NaN`/`Infinity`, or a `\u0000` character, which Postgres cannot store) and the reason is written to its log, so a child never reads wrong data. An empty or missing file, or the JSON `null`, just means "no output".
- Only a task that **succeeds** publishes an output; whatever a failed or timed-out task wrote is discarded. Because a child only runs once all its parents succeeded, it always sees real parent outputs.
- Inputs come from the same run only, and only from direct parents (a task that needs a grandparent's data can read it from the parent's output).
- The files are in a private temporary directory that is deleted when the task ends. Both are files (not environment variables) because Linux caps one environment variable at about 128 KB, which a few 64 KB parent outputs would exceed.
- A task's own `env` config cannot override `ORION_OUTPUT` or `ORION_INPUTS`.
- The output is stored in `task_instances.output` in the same transaction as the status and logs, returned by the API, and shown in the UI's run page ("Outputs").

**Security note:** tasks are arbitrary code executed in a plain subprocess on the worker host with no sandboxing. Only run Orion where you trust everyone who can call the API (there is no auth).

## Data model

| Table | Purpose |
| --- | --- |
| `dags` | `name` (unique), `description`, `schedule` (cron, nullable), `owner`, `is_active`, `python_version`, `run_counter` (hands out run numbers) |
| `tasks` | Belongs to a DAG; `name` (unique per DAG), `task_type`, `config` (JSONB) |
| `task_dependencies` | `(task_id, depends_on_task_id)` edges; self-dependency is blocked by a CHECK |
| `runs` | One execution of a DAG; `number` (1, 2, 3... per DAG, unique); status `queued`/`running`/`success`/`failed`/`cancelled`; `triggered_by` |
| `task_instances` | One task within one run; status `pending`/`queued`/`running`/`success`/`failed`/`retrying`/`skipped`; `attempt`, `worker_id`, timestamps, `logs`, `output` (JSONB, the task's published result) |

Deleting a DAG cascades to its tasks, dependencies, runs and instances.

## Getting started

### Prerequisites

Docker, Go 1.26+, Python 3.10+, Node.js 20+, and the [`migrate`](https://github.com/golang-migrate/migrate) CLI or `psql` for applying migrations.

### Quick start: everything in Docker

You only need Docker. Create the `.env` shown in step 1 below (copy the block as it is), then:

```bash
make up      # builds the images and starts everything; the first build takes a few minutes
```

| What | Where |
| --- | --- |
| Web console | <http://localhost:3000> |
| API + Swagger docs | <http://localhost:8080> · <http://localhost:8080/swagger/> |
| Postgres / Redis | `localhost:5432` / `localhost:6379` |

`make up` starts, in order: Postgres and Redis, a one-shot `migrate` container that applies the SQL migrations and exits, then the API, one worker and the UI. A fresh database is empty: create a DAG with the [example below](#example-build-and-run-a-dag), then open the console and press **Trigger run**.

```bash
make down                                                                                   # stop (data is kept in Docker volumes)
docker compose -f orion-infra/docker-compose.yml --env-file .env up -d --scale worker=3    # more workers
docker compose -f orion-infra/docker-compose.yml --env-file .env logs -f api               # follow one service's logs
```

Notes:
- Open the UI at exactly `http://localhost:3000` (not `127.0.0.1`): the API only allows that origin for CORS and the WebSocket (`UI_ORIGIN` in the compose file).
- The UI image has the API address (`http://localhost:8080`) baked in at build time (`NEXT_PUBLIC_API_URL` build arg in the compose file); change it and run `make up` again to rebuild.
- To develop the API or UI outside Docker, stop that container first so the port is free, e.g. `docker compose -f orion-infra/docker-compose.yml --env-file .env stop api ui`, then follow the manual steps below.
- Dev reset (deletes all data): `docker compose -f orion-infra/docker-compose.yml --env-file .env down -v`.

### Running pieces by hand (development)

The rest of this section runs each part on your machine instead (needs Go 1.26+, Python 3.10+, Node 20+ and the `migrate` CLI).

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

### 2. Start Postgres and Redis (and, with `make up`, everything else)

```bash
make up      # docker compose up -d --build: the whole stack (see Quick start)
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

The UI calls the API at `NEXT_PUBLIC_API_URL` (default `http://localhost:8080`; see `orion-ui/.env.example`). Start the API first, plus a worker if you want runs to execute.

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

Served by `orion-api` on `PORT` (default 8080). Run objects include `number`, `dag_name`, `tasks_total` and `tasks_done`; task instances include `task_name`. Request and response bodies are JSON. CORS allows `http://localhost:3000` (the Next.js dev server) only. Errors are always `{"error": "..."}`.

| Method | Path | Description |
| --- | --- | --- |
| GET | `/health` | Liveness check |
| GET | `/dags` | List DAGs (newest first), each with `last_run`, `success_rate`, `avg_duration_seconds` and `recent_durations` (last 20 finished runs) |
| POST | `/dags` | Create a DAG (`name` required; `description`, `schedule`, `owner`, `python_version` optional) |
| GET | `/dags/{id}` | Get a DAG with the same summary fields |
| PUT | `/dags/{id}` | Partial update (`name`, `description`, `schedule`, `owner`, `is_active`, `python_version`) |
| DELETE | `/dags/{id}` | Delete a DAG and everything under it |
| GET | `/dags/{dagId}/tasks` | List a DAG's tasks |
| POST | `/dags/{dagId}/tasks` | Create a task (`name`, `task_type`, `config`) |
| GET | `/tasks/{id}` | Get a task |
| PUT | `/tasks/{id}` | Replace a task's `name`, `task_type`, `config` |
| DELETE | `/tasks/{id}` | Delete a task |
| GET | `/dags/{dagId}/runs` | List runs for a DAG |
| POST | `/dags/{dagId}/runs` | Trigger a run; returns `{run, task_instances, enqueued}`. 404 for an unknown DAG, 400 for a DAG with no tasks. Atomic: nothing is left behind if it fails. |
| GET | `/runs` | List runs across all DAGs, newest first (`limit` up to 200, default 50; optional `status`, `dag_id`) |
| GET | `/runs/{id}` | Get a run |
| GET | `/dags/{dagId}/dependencies` | List a DAG's dependency edges |
| POST | `/dags/{dagId}/dependencies` | Add an edge (`task_id` runs after `depends_on_task_id`); 409 on duplicate or cycle |
| DELETE | `/dags/{dagId}/dependencies/{taskId}/{dependsOnId}` | Remove an edge |
| GET | `/runs/{runId}/task-instances` | List a run's task instances, including `logs`, `output`, `worker_id`, `attempt` and timestamps |
| GET | `/task-instances/{id}` | Get a task instance |
| GET | `/stats` | Dashboard numbers: `active_runs`, `success_rate_24h`, `failed_24h`, `throughput_24h` (24 hourly buckets) |
| GET | `/workers` | Live workers from their Redis heartbeats (`status` idle/busy, `current_task`) |
| GET | `/ws` | WebSocket: pushes `run` and `worker` hints (see Live updates); needs an allowed `Origin` |

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
- Coverage: realtime (hub fan-out, slow-client drop, WebSocket delivery and origin check, `/workers`), the UI read model (run numbers, global runs list, DAG summaries, stats), health, DAG/task CRUD and validation, run triggering (only dependency-free tasks are enqueued, once each), run/task-instance reads, queue semantics (idempotent group creation, FIFO, blocking reads, no duplicate delivery across consumers, ack idempotency, stale-claim recovery, `MAXLEN` trimming, context cancellation) and an end-to-end crashed-worker recovery test.
- The UI's pure logic (API-to-UI adapters, graph layout, log parsing, event mapping) is covered by `vitest` (`cd orion-ui && npm test`). The pages themselves are checked by hand against a running stack.

The worker has its own `pytest` suite (executor behaviour, every database state transition, the full `process()` pipeline against real Postgres with a fake Redis, events and heartbeats). It uses the same `<db>_test` database, so run the Go tests once first to create it:

```bash
cd orion-worker
.venv/bin/pip install -r requirements-dev.txt
.venv/bin/python -m pytest tests
```

## Test and benchmark results

Measured on 2026-10-19 on one machine: Apple M4 Pro (12 cores, 24 GB), macOS 26.6, Docker Desktop 29.5 (12 CPUs, 8 GB) running Postgres, Redis, the API and the workers. Everything is local, so read these as relative numbers, not production capacity. Each figure is a single run, not an average of many.

### Automated tests

| Suite | Tests | What it runs against |
| --- | --- | --- |
| Go API (`orion-api/tests`) | 56 | real Postgres and Redis; the hub and WebSocket tests also pass under `go test -race` |
| Python worker (`orion-worker/tests`) | 109 | real Postgres, fake Redis |
| UI (`orion-ui`, `npm test`) | 65 | pure functions (adapters, graph layout, log parsing, event mapping) |
| **Total** | **230** | all passing |

### Throughput and scaling

Each run is a four-task diamond (`extract` → `transform_a`, `transform_b` → `load`). Runs were triggered 20 at a time through the API, and the clock stopped when every run had finished.

**Orchestration overhead** (tasks that only `print`, so this measures queueing, database round-trips and spawning the Python subprocess, not task work):

| Workers | Runs | Tasks | Time | Tasks/s |
| --- | --- | --- | --- | --- |
| 1 | 40 | 160 | 1.3 s | 122 |
| 3 | 40 | 160 | 0.6 s | 266 |
| 3 (repeat) | 40 | 160 | 0.6 s | 260 |

Adding workers helps less here (about 2.2× for 3 workers) because the shared Postgres and Redis become the limit when tasks are this small.

**Horizontal scaling** (tasks that each sleep 0.5 s, 6 runs = 24 tasks, so 12 s of work):

| Workers | Time | Tasks/s | Speedup |
| --- | --- | --- | --- |
| 1 | 12.8 s | 1.87 | 1.0× |
| 3 | 4.4 s | 5.41 | 2.9× |
| 6 | 2.3 s | 10.28 | 5.5× |

With tasks that take real time, throughput grows almost linearly with the number of workers.

### Live-update latency

How long after a run's last task is committed to Postgres does a browser see the change? Method: a WebSocket client connected to `GET /ws` and triggered 30 runs one after another; for each run, the arrival time of its final event was compared with the `finished_at` of its last task, after correcting for the 11 ms offset between the Docker VM clock and the host clock (measured five times, 10.4 to 12.3 ms).

| Median | p95 | Min | Max |
| --- | --- | --- | --- |
| 14 ms | 16 ms | 13 ms | 16 ms |

That interval covers the worker's remaining commits for the run, the Redis publish, the hub and the WebSocket write. Each run produced about 9 events. The browser then refetches the data over REST, which is not included in this number.

### Failure behaviour

| Scenario | Result |
| --- | --- |
| `kill -9` a worker mid-task | The task stayed `running` for about 150 s (a 120 s idle threshold plus the 30 s check interval), then another worker reclaimed it and the run finished `success`. |
| Worker stopped with `docker stop` | Left the worker list immediately (it deletes its heartbeat key); a running task finished first. |
| Worker killed with `kill -9` | Left the worker list after 8 to 12 s (heartbeat TTL is 15 s). |
| API stopped while the UI is open | The UI shows OFFLINE within seconds, keeps the last data, and returns to LIVE by itself once the API is back. |
| Failing task in a diamond | The task is `failed`, its downstream task is `skipped`, its sibling still finishes, and the run ends `failed`. |
| Six runs triggered at once on 3 workers | 24 of 24 tasks succeeded and none ran twice. |

Reading these correctly: delivery is at-least-once, so a task that is reclaimed after a crash runs again; tasks should be safe to repeat.

## Repository layout

```
.
├── Makefile                     # up / down for the local stack
├── migrations/                  # SQL migrations (000001–000005)
├── orion-infra/docker-compose.yml   # postgres, redis, migrate, api, ui, worker
├── orion-api/
│   ├── Dockerfile               # static Go binary on alpine
│   ├── main.go                  # wiring: env, DB pool, queue, routes, CORS, Swagger
│   ├── docs/                    # generated Swagger (docs.go, swagger.json/yaml)
│   ├── internal/
│   │   ├── db/                  # pgx pool
│   │   ├── events/              # event type + hub (Redis Pub/Sub -> WebSocket clients)
│   │   ├── handlers/            # dags, tasks, dependencies, runs, task_instances, stats, workers, ws, health, routes
│   │   ├── models/              # DAG(Summary), Task, Run, TaskInstance, TaskDependency, Worker, Stats
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
│       ├── events.py            # publish run/worker events (best-effort)
│       ├── heartbeat.py         # liveness key in Redis
│       └── stream.py            # Redis Streams helpers
└── orion-ui/                    # Dockerfile (Next.js standalone build); Next.js console: RTK Query data layer + WebSocket live updates (see orion-ui/README.md)
```

## Known gaps

Things I found while documenting that you will hit when running the project:

1. **No scheduler.** `dags.schedule` is stored but nothing triggers runs from it; runs are manual only (`triggered_by` is never set to `schedule`).
2. **Task logs appear when a task finishes**, not line by line: the executor captures output at the end. Status changes are live; log text is not.
3. **No DAG or task editor in the UI.** DAGs, tasks and dependencies are created through the API (the YAML spec and editor are the next step).
4. **Worker trusts task code** (see the security note above) and no authentication exists anywhere.

## Roadmap

1. Add a scheduler for cron DAGs.
2. Implement retries, run cancellation, and additional task types (`shell`, `http`).
3. Add CI.
4. Add auth.
