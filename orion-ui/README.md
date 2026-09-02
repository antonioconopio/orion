# Orion UI

Frontend for **Orion**, a self-directed distributed DAG workflow orchestration
platform (a lightweight Airflow alternative). This package is the web console:
a dark, mission-control–style interface for observing DAGs, runs, tasks, and
workers.

> **Status:** static UI skeleton complete. Every screen is reachable and looks
> finished, but everything renders from hardcoded example data — there is no
> live data, no fetching, and no real-time behavior wired in yet. See
> [What still needs to be done](#what-still-needs-to-be-done).

## Tech stack

- **Next.js 16** (App Router, Turbopack) + **React 19**
- **TypeScript** (strict, no `any`)
- **Tailwind CSS v4** + **shadcn/ui** primitives (on top of the unified
  `radix-ui` package)
- **Redux Toolkit** + `react-redux` — store is scaffolded but currently unused
  by the presentational layer (see below)
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

Open [http://localhost:3000](http://localhost:3000). The root route redirects to
`/dashboard`.

Other scripts:

```bash
npm run build   # production build
npm run start   # serve the production build
npm run lint    # eslint
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
  types.ts                 # central domain types (Dag, Task, Run, Worker, LogLine…)
  status-style.ts          # status → color/label style maps
  sample-data.ts           # ALL static example data lives here
  utils.ts                 # cn() helper

store/                     # Redux Toolkit store + slices (scaffolded, unused)
hooks/                     # use-mobile (sidebar)
```

**Convention:** organized by feature, not by file type. Everything in this pass
is presentational — components take props for their real data needs, but only
static example values (from `lib/sample-data.ts`) are passed in.

## Screens (current state)

| Route | Screen | What's there |
| --- | --- | --- |
| `/dashboard` | **System overview** | KPI cards (active runs, 24h success rate, failures, workers online), throughput bar chart, worker-pool health list, active-runs + recent-failures lists |
| `/dags` | **DAGs list** | Search/filter bar (visual only), status stat cards, table of DAGs; rows link to DAG detail |
| `/dags/[dagId]` | **DAG detail** | React Flow task graph (colored by last-run status) + run-history table; 404s on unknown id |
| `/runs` | **Runs list** | Status stat cards, search bar (visual only), table of runs; rows link to run detail |
| `/runs/[runId]` | **Run detail** | Tabs: task **Graph** (nodes colored by live task status) and **Timeline** (gantt of task durations); **Logs** panel + per-task log slide-over (Sheet); 404s on unknown id |
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

## Data model

All example data is static and typed. Domain types live in `lib/types.ts`
(`Dag`, `Task`, `TaskGraphNode`, `TaskEdge`, `Run`, `Worker`, `LogLine`, plus
the status unions). All example values live in `lib/sample-data.ts`. The
task-graph topology (`TASK_TOPOLOGY` / `TASK_EDGES`) is defined once and reused
by both the DAG-definition graph and the run graph, each with its own status
map.

## What still needs to be done

This pass is **UI only**. None of the following is implemented yet:

### Data & state
- [ ] **API client / data fetching** — replace `lib/sample-data.ts` with real
      calls to the Go API server. Components already accept props, so this is
      wiring, not restructuring.
- [ ] **Redux integration** — the store (`store/`) and `dagSlice` are scaffolded
      but unused. Decide what actually belongs in global state vs. server state,
      and wire selectors/dispatch into the pages.
- [ ] **Live updates** — subscribe to run/task status changes (Redis Pub/Sub via
      WebSocket/SSE) so the graph nodes, timeline, run progress, and logs update
      in real time instead of showing a static snapshot.
- [ ] **Log streaming** — the log viewer currently renders a fixed array; make
      it tail live and virtualize (`react-window` is already a dependency).

### Functionality (currently visual-only)
- [ ] **Search & filtering** on the DAGs and Runs lists (bars are placeholders).
- [ ] **Filtering / time-range** controls on the dashboard and runs pages.
- [ ] **Actions** — "New Run" / "Register DAG" / "Trigger run" / "Re-run"
      buttons are inert; wire to the API.
- [ ] **Settings & connections** — fields are static; add save/validation and a
      real connections CRUD flow.
- [ ] **Active-route state** works in the sidebar, but there is no breadcrumb or
      deep-link state (e.g. selected node in the graph).

### Screens / gaps
- [ ] Decide the fate of the leftover **`/connections`** and **`/variables`**
      stub pages — either build them out and add to nav, fold `connections`
      fully into Settings, or delete them. They currently exist but are not
      linked.
- [ ] **DAG detail** graph shows last-run status; consider a run selector so the
      graph/history reflect a chosen run.
- [ ] **Empty / loading / error states** — add `loading.tsx` and `error.tsx`
      per route, plus empty-state UI for lists.
- [ ] **Auth / login flow** — none yet (was out of scope).

### Polish
- [ ] Responsive review below the desktop breakpoint (desktop-first for now).
- [ ] Accessibility pass (focus order, ARIA on custom widgets, keyboard nav in
      the graph).
- [ ] Consider pushing the accent color bolder (more saturated electric
      blue/violet) if a stronger "mission-control" look is wanted.

## Out of scope for this pass

No Redux logic, no custom data hooks, no WebSocket/real-time simulation, no real
API calls, no auth, no CI/deploy config — all intentionally deferred to the
integration work above.
