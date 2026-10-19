# Orion UI

Frontend for **Orion**, a self-directed distributed DAG workflow orchestration
platform (a lightweight Airflow alternative). This package is the web console:
a dark, mission-control–style interface for observing DAGs, runs, tasks, and
workers.

> **Status:** wired to the Orion API. Every screen reads real data, and runs,
> task graphs and workers update live over a WebSocket (no refresh). The
> remaining gaps are listed in [What still needs to be done](#what-still-needs-to-be-done).

## Tech stack

- **Next.js 16** (App Router, Turbopack) + **React 19**
- **TypeScript** (strict, no `any`)
- **Tailwind CSS v4** + **shadcn/ui** primitives (on top of the unified
  `radix-ui` package)
- **Redux Toolkit** + `react-redux`; **RTK Query** (inside Redux Toolkit) is the
  data layer: it fetches, caches and refetches everything from the API
- **@xyflow/react** (React Flow v12) for the task-graph visualizations
- **next-themes** for the dark/light theme
- **lucide-react** for icons

> ⚠️ This repo pins **Next.js 16**, which has breaking changes vs. older
> versions. Before writing code, read the bundled docs in
> `node_modules/next/dist/docs/` (see `AGENTS.md`). The most important one:
> dynamic route `params` is now a **Promise** — in Client Components read it
> with `use(params)`; in Server Components `await` it.

## Getting Started

```bash
npm install
npm run dev
```

The UI needs the Orion API (default `http://localhost:8080`, override with
`NEXT_PUBLIC_API_URL`; see `.env.example`). `NEXT_PUBLIC_` variables are inlined at
build time, so restart `next dev` after changing one.

Open [http://localhost:3000](http://localhost:3000). The root route redirects to
`/dashboard`.

Other scripts:

```bash
npm run build   # production build
npm run start   # serve the production build
npm run lint    # eslint
npm test        # vitest: adapters, graph layout, log parsing, event mapping
```

## Project structure

```
app/
  page.tsx                 # redirects → /dashboard
  layout.tsx               # root layout: sidebar + main, providers, fonts
  globals.css              # theme tokens (dark + light), Tailwind setup
  dashboard/page.tsx       # system overview
  dags/page.tsx            # DAGs list
  dags/[dagId]/page.tsx    # DAG detail (task graph + run history)
  runs/page.tsx            # runs list
  runs/[runId]/page.tsx    # run detail (graph / timeline / logs)
  workers/page.tsx         # worker pool
  settings/page.tsx        # settings + connections
  connections/page.tsx     # LEFTOVER STUB (not in nav)
  variables/page.tsx       # LEFTOVER STUB (not in nav)

components/
  app-sidebar.tsx          # primary navigation
  page-header.tsx          # shared page top bar
  providers.tsx            # Redux + theme + sidebar providers
  task-status-badge.tsx    # 5-state task/run status badge
  task-status-dot.tsx      # 5-state status dot (pulses when running)
  dashboard/               # overview widgets + the DAG table/row/sparkline
  dags/                    # DAGs list filter bar
  runs/                    # run table/row, task timeline (gantt), log viewer(s)
  workers/                 # worker card
  graph/                   # React Flow graph, custom task node, legend
  ui/                      # shadcn primitives (button, card, sheet, tabs, …)

lib/
  types.ts                 # UI domain types (Dag, Run, Task, Worker, LogLine, status unions)
  status-style.ts          # status → color/label style maps
  use-now.ts               # ticking clock hook (relative times, running durations)
  api/types.ts             # API response shapes (snake_case)
  api/adapters.ts          # API → UI types, graph layout, log parsing, formatting (pure, tested)
  api/events.ts            # WebSocket event parsing → cache tags, reconnect backoff (pure, tested)
  api/config.ts            # API_URL / WS_URL from NEXT_PUBLIC_API_URL

store/
  api.ts                   # RTK Query: every endpoint, with the cache tags it provides
  store.ts                 # Redux store (api, connection state)
  slices/connectionSlice.ts  # is the WebSocket up? (header LIVE / OFFLINE)
  slices/dagSlice.ts       # unused scaffold, left as is
hooks/                     # use-mobile (sidebar)
```

**Convention:** organized by feature, not by file type. Pages fetch with the RTK
Query hooks in `store/api.ts`, convert with `lib/api/adapters.ts`, and pass the
result to presentational components as props; components never see raw API shapes.

## Screens (current state)

| Route | Screen | What's there |
| --- | --- | --- |
| `/dashboard` | **System overview** | KPI cards (active runs, 24h success rate, failures, workers online), throughput bar chart, worker-pool health list, active-runs + recent-failures lists |
| `/dags` | **DAGs list** | Search/filter bar (visual only), status stat cards, table of DAGs; rows link to DAG detail |
| `/dags/[dagId]` | **DAG detail** | React Flow task graph (colored by last-run status) + run-history table; 404s on unknown id |
| `/runs` | **Runs list** | Status stat cards, search bar (visual only), table of runs; rows link to run detail |
| `/runs/[runId]` | **Run detail** | Tabs: task **Graph** (nodes colored by live task status) and **Timeline** (gantt of task durations); **Outputs** panel (the JSON each task published) + **Logs** panel + per-task log slide-over (Sheet); 404s on unknown id |
| `/workers` | **Worker pool** | Pool stat cards + grid of worker cards (load bar, slots, region, current task) |
| `/settings` | **Settings / connections** | General config fields + connections list |

### Reusable building blocks

- **`DagGraph`** (`components/graph/`) — static, pan/zoom React Flow graph.
  Takes `nodes` + `edges`; edges into a running/retrying task are highlighted.
  Reused by both DAG detail and run detail.
- **`TaskTimeline`** — gantt view; bar position/width derived from each task's
  start offset and duration.
- **`LogViewer`** / **`LogViewerSheet`** — monospace log panel; the sheet
  version opens a single task's logs in a slide-over.
- **`RunTable`** / **`CompactRunList`** — full and condensed run listings.
- **`TaskStatusBadge`** / **`TaskStatusDot`** — 5-state status vocabulary.

### Design / theme notes

- Dark theme by default; a light theme and toggle also exist (in the sidebar
  footer).
- Deep near-black backgrounds, violet/blue accent, desaturated gray secondary
  text; monospace for IDs, task names, run ids, and logs.
- **Status vocabularies:** DAGs use 3 states (`success` / `failed` /
  `running`); tasks and runs use 5 (`pending` / `running` / `success` /
  `failed` / `retrying`). All color/label styling is centralized in
  `lib/status-style.ts`.

## Data flow and live updates

```
page ──useGetXQuery──► RTK Query cache ──fetch──► Go API
                              ▲
            invalidateTags    │   {"type":"run","run_id":...}  (a hint, no data)
live-updates.tsx ◄──WebSocket──┘
```

1. A page calls a hook such as `useGetRunInstancesQuery(runId)`. RTK Query fetches it
   and caches it under tags (`Instances:<runId>`).
2. `components/live-updates.tsx` (mounted once in `providers.tsx`) holds a WebSocket to
   the API. Each message is only a hint about what changed; `lib/api/events.ts` maps it
   to cache tags, and `invalidateTags` makes RTK Query refetch whatever is on screen.
3. Events that arrive together are batched (150 ms) into one refetch. On reconnect
   after a drop everything is invalidated, because events may have been missed.
4. The header shows **LIVE / CONNECTING / OFFLINE** (`components/live-indicator.tsx`).
5. A worker that is killed sends no event, so the workers query also polls every 10 s.

Why refetch instead of patching the cache from the event: one read path, nothing to
drift out of sync with the database, and a lost event is harmless. If refetching ever
feels slow, patching from event payloads is the next optimization.

### How API data maps to what the components show

| UI shows | Comes from |
| --- | --- |
| DAG status badge | the DAG's last run (`running` while queued/running; `NEVER RUN` if none) |
| Run `#12`, progress `3/4 tasks` | `runs.number`, `tasks_done`/`tasks_total` (done = success, failed or skipped) |
| Task graph positions | computed client-side (`layoutGraph`: column = longest dependency path) |
| Graph node colors | that run's task instances (DAG page: the latest run) |
| Log lines | each instance's `logs`: stdout → INFO, stderr → ERROR on a failed task (WARN otherwise), timeout → ERROR. Timestamps are the instance's finish time (the backend has no per-line times) |
| Worker card | `/workers` heartbeats; one slot per worker, so busy = 100% load |

Statuses: tasks and runs share one vocabulary (`pending`, `queued`, `running`,
`success`, `failed`, `retrying`, `skipped`, `cancelled`); a DAG adds `idle` (never run).

## What still needs to be done

### Functionality
- [ ] **Search & filtering** on the DAGs and Runs lists, and the time-range
      control on Runs (the bars are still visual-only).
- [ ] **DAG / task editor** ("Register DAG" was removed until the YAML spec and a
      task editor exist). DAGs are created through the API for now.
- [ ] **Settings & connections** — still static; no backend for it.
- [ ] **Line-by-line log streaming** — logs appear when a task finishes, because the
      executor captures output at the end (status changes are live).
- [ ] Decide the fate of the leftover **`/connections`** and **`/variables`** stubs.
- [ ] **DAG detail run selector** — the graph always shows the latest run.
- [ ] **Auth / login flow** — none yet.

### Polish
- [ ] `loading.tsx` / `error.tsx` per route (pages have their own loading, error and
      empty states already).
- [ ] Responsive review below the desktop breakpoint (desktop-first for now).
- [ ] Accessibility pass (focus order, ARIA on custom widgets, keyboard nav in the graph).
- [ ] Pre-existing lint findings: one `react-hooks/set-state-in-effect` error in
      `hooks/use-mobile.ts` and unused imports in `components/app-sidebar.tsx`.
