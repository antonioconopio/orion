package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"testing"
)

func TestHealth(t *testing.T) {
	c := newAPI(t)
	var got map[string]string
	c.call("GET", "/health", nil, http.StatusOK, &got)
	if got["status"] != "ok" {
		t.Fatalf("got %v", got)
	}
}

// ---------- DAGs ----------

func TestDAGCreate(t *testing.T) {
	c := newAPI(t)

	var d dagResp
	c.call("POST", "/dags", map[string]any{
		"name": "etl", "description": "nightly", "schedule": "0 2 * * *", "owner": "ana",
	}, http.StatusCreated, &d)

	if d.ID == "" || d.Name != "etl" || !d.IsActive || d.CreatedAt == "" || d.UpdatedAt == "" {
		t.Fatalf("unexpected dag: %+v", d)
	}
	if d.Description == nil || *d.Description != "nightly" || d.Owner == nil || *d.Owner != "ana" ||
		d.Schedule == nil || *d.Schedule != "0 2 * * *" {
		t.Fatalf("optional fields not persisted: %+v", d)
	}

	// Optional fields omitted -> JSON null.
	min := c.createDAG("minimal")
	if min.Description != nil || min.Schedule != nil || min.Owner != nil {
		t.Fatalf("expected nulls, got %+v", min)
	}
}

func TestDAGCreateValidation(t *testing.T) {
	c := newAPI(t)
	c.expectStatus("POST", "/dags", `{not json`, http.StatusBadRequest)
	c.expectStatus("POST", "/dags", map[string]any{"description": "no name"}, http.StatusBadRequest)
	c.expectStatus("POST", "/dags", map[string]any{"name": ""}, http.StatusBadRequest)
}

func TestDAGCreateDuplicateName(t *testing.T) {
	c := newAPI(t)
	c.createDAG("dup")
	// name is UNIQUE; the handler currently maps every DB error to 500
	// (a 409 would be more accurate). Update this if that changes.
	c.expectStatus("POST", "/dags", map[string]any{"name": "dup"}, http.StatusInternalServerError)
}

func TestDAGList(t *testing.T) {
	c := newAPI(t)

	var empty []dagResp
	c.call("GET", "/dags", nil, http.StatusOK, &empty)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty list should be [] not null, got %#v", empty)
	}

	c.createDAG("first")
	c.createDAG("second")

	var list []dagResp
	c.call("GET", "/dags", nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Name != "second" || list[1].Name != "first" {
		t.Fatalf("expected newest first, got %s, %s", list[0].Name, list[1].Name)
	}
}

func TestDAGGet(t *testing.T) {
	c := newAPI(t)
	created := c.createDAG("getme")

	var got dagResp
	c.call("GET", "/dags/"+created.ID, nil, http.StatusOK, &got)
	if got.ID != created.ID || got.Name != "getme" {
		t.Fatalf("got %+v", got)
	}

	c.expectStatus("GET", "/dags/"+randomUUID(), nil, http.StatusNotFound)
	c.expectStatus("GET", "/dags/not-a-uuid", nil, http.StatusBadRequest)
}

func TestDAGUpdate(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("orig")

	// Partial update: only provided fields change.
	var got dagResp
	c.call("PUT", "/dags/"+d.ID, map[string]any{"owner": "bob", "is_active": false}, http.StatusOK, &got)
	if got.Name != "orig" {
		t.Fatalf("name should be untouched, got %q", got.Name)
	}
	if got.Owner == nil || *got.Owner != "bob" || got.IsActive {
		t.Fatalf("update not applied: %+v", got)
	}
	if got.UpdatedAt == d.UpdatedAt {
		t.Fatalf("updated_at should change")
	}

	// Persisted.
	var fetched dagResp
	c.call("GET", "/dags/"+d.ID, nil, http.StatusOK, &fetched)
	if fetched.Owner == nil || *fetched.Owner != "bob" || fetched.IsActive {
		t.Fatalf("not persisted: %+v", fetched)
	}

	// Rename.
	c.call("PUT", "/dags/"+d.ID, map[string]any{"name": "renamed"}, http.StatusOK, &got)
	if got.Name != "renamed" {
		t.Fatalf("rename failed: %+v", got)
	}

	c.expectStatus("PUT", "/dags/"+randomUUID(), map[string]any{"name": "x"}, http.StatusNotFound)
	c.expectStatus("PUT", "/dags/not-a-uuid", map[string]any{"name": "x"}, http.StatusBadRequest)
	c.expectStatus("PUT", "/dags/"+d.ID, `{bad`, http.StatusBadRequest)
}

func TestDAGDelete(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("doomed")
	task := c.createTask(d.ID, "t1")
	run := c.triggerRun(d.ID)

	c.expectStatus("DELETE", "/dags/"+d.ID, nil, http.StatusNoContent)
	c.expectStatus("GET", "/dags/"+d.ID, nil, http.StatusNotFound)
	c.expectStatus("DELETE", "/dags/"+d.ID, nil, http.StatusNotFound)
	c.expectStatus("DELETE", "/dags/not-a-uuid", nil, http.StatusBadRequest)

	// ON DELETE CASCADE removes children.
	c.expectStatus("GET", "/tasks/"+task.ID, nil, http.StatusNotFound)
	c.expectStatus("GET", "/runs/"+run.ID, nil, http.StatusNotFound)
}

// ---------- Tasks ----------

func TestTaskCreate(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")

	var tk taskResp
	c.call("POST", "/dags/"+d.ID+"/tasks", map[string]any{
		"name": "extract", "task_type": "http", "config": map[string]any{"url": "http://x", "retries": 3},
	}, http.StatusCreated, &tk)

	if tk.ID == "" || tk.DAGID != d.ID || tk.Name != "extract" || tk.TaskType != "http" {
		t.Fatalf("unexpected task: %+v", tk)
	}
	var cfg map[string]any
	if err := json.Unmarshal(tk.Config, &cfg); err != nil || cfg["url"] != "http://x" || cfg["retries"] != float64(3) {
		t.Fatalf("config not round-tripped: %s (%v)", tk.Config, err)
	}
}

func TestTaskCreateErrors(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	body := map[string]any{"name": "t", "task_type": "shell", "config": map[string]any{}}

	c.expectStatus("POST", "/dags/"+d.ID+"/tasks", `{bad`, http.StatusBadRequest)
	c.expectStatus("POST", "/dags/not-a-uuid/tasks", body, http.StatusBadRequest)

	// Unknown DAG: FK violation, currently surfaced as 500.
	c.expectStatus("POST", "/dags/"+randomUUID()+"/tasks", body, http.StatusInternalServerError)

	// (dag_id, name) is UNIQUE.
	c.expectStatus("POST", "/dags/"+d.ID+"/tasks", body, http.StatusCreated)
	c.expectStatus("POST", "/dags/"+d.ID+"/tasks", body, http.StatusInternalServerError)
}

func TestTaskList(t *testing.T) {
	c := newAPI(t)
	d1 := c.createDAG("d1")
	d2 := c.createDAG("d2")

	var empty []taskResp
	c.call("GET", "/dags/"+d1.ID+"/tasks", nil, http.StatusOK, &empty)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty list should be [], got %#v", empty)
	}

	c.createTask(d1.ID, "a")
	c.createTask(d1.ID, "b")
	c.createTask(d2.ID, "other")

	var list []taskResp
	c.call("GET", "/dags/"+d1.ID+"/tasks", nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2 (scoped to DAG)", len(list))
	}
	for _, tk := range list {
		if tk.DAGID != d1.ID {
			t.Fatalf("task from wrong dag: %+v", tk)
		}
	}

	c.expectStatus("GET", "/dags/not-a-uuid/tasks", nil, http.StatusBadRequest)
}

func TestTaskGet(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	tk := c.createTask(d.ID, "t")

	var got taskResp
	c.call("GET", "/tasks/"+tk.ID, nil, http.StatusOK, &got)
	if got.ID != tk.ID || got.Name != "t" {
		t.Fatalf("got %+v", got)
	}
	c.expectStatus("GET", "/tasks/"+randomUUID(), nil, http.StatusNotFound)
	c.expectStatus("GET", "/tasks/not-a-uuid", nil, http.StatusBadRequest)
}

func TestTaskUpdate(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	tk := c.createTask(d.ID, "old")

	var got taskResp
	c.call("PUT", "/tasks/"+tk.ID, map[string]any{
		"name": "new", "task_type": "python", "config": map[string]any{"script": "x.py"},
	}, http.StatusOK, &got)
	if got.Name != "new" || got.TaskType != "python" || got.DAGID != d.ID {
		t.Fatalf("got %+v", got)
	}
	var cfg map[string]any
	json.Unmarshal(got.Config, &cfg)
	if cfg["script"] != "x.py" {
		t.Fatalf("config = %s", got.Config)
	}

	body := map[string]any{"name": "n", "task_type": "shell", "config": map[string]any{}}
	c.expectStatus("PUT", "/tasks/"+randomUUID(), body, http.StatusNotFound)
	c.expectStatus("PUT", "/tasks/not-a-uuid", body, http.StatusBadRequest)
	c.expectStatus("PUT", "/tasks/"+tk.ID, `{bad`, http.StatusBadRequest)
}

func TestTaskDelete(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	tk := c.createTask(d.ID, "t")

	c.expectStatus("DELETE", "/tasks/"+tk.ID, nil, http.StatusNoContent)
	c.expectStatus("GET", "/tasks/"+tk.ID, nil, http.StatusNotFound)
	c.expectStatus("DELETE", "/tasks/"+tk.ID, nil, http.StatusNotFound)
	c.expectStatus("DELETE", "/tasks/not-a-uuid", nil, http.StatusBadRequest)

	// The parent DAG is unaffected.
	c.expectStatus("GET", "/dags/"+d.ID, nil, http.StatusOK)
}

// ---------- Runs ----------

func TestRunTrigger(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	t1 := c.createTask(d.ID, "a")
	t2 := c.createTask(d.ID, "b")

	run := c.triggerRun(d.ID)
	if run.ID == "" || run.DAGID != d.ID || run.Status != "queued" {
		t.Fatalf("unexpected run: %+v", run)
	}

	// One task instance per task is created; these have no dependencies, so they are queued.
	var tis []taskInstanceResp
	c.call("GET", "/runs/"+run.ID+"/task-instances", nil, http.StatusOK, &tis)
	if len(tis) != 2 {
		t.Fatalf("task instances = %d, want 2", len(tis))
	}
	seen := map[string]bool{}
	for _, ti := range tis {
		if ti.RunID != run.ID || ti.Status != "queued" {
			t.Fatalf("unexpected task instance: %+v", ti)
		}
		seen[ti.TaskID] = true
	}
	if !seen[t1.ID] || !seen[t2.ID] {
		t.Fatalf("task instances don't cover both tasks: %+v", tis)
	}
}

func TestRunTriggerResponseShape(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	c.createTask(d.ID, "a")

	status, raw := c.do("POST", "/dags/"+d.ID+"/runs", nil)
	if status != http.StatusCreated {
		t.Fatalf("status = %d (%s)", status, raw)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"run", "task_instances", "enqueued"} {
		if _, ok := top[k]; !ok {
			t.Fatalf("response missing %q: %s", k, raw)
		}
	}
}

// Every task without dependencies must be enqueued exactly once, and the
// enqueued messages must be those task instances.
func TestRunTriggerEnqueuesEachIndependentTaskInstanceOnce(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	c.createTask(d.ID, "a")
	c.createTask(d.ID, "b")
	c.createTask(d.ID, "c")

	trig := c.trigger(d.ID)
	if len(trig.TaskInstances) != 3 {
		t.Fatalf("task instances = %d, want 3", len(trig.TaskInstances))
	}
	if len(trig.Enqueued) != 3 {
		t.Fatalf("enqueued message ids = %d, want 3", len(trig.Enqueued))
	}

	var want []string
	for _, ti := range trig.TaskInstances {
		want = append(want, ti.ID)
	}
	got := c.streamTaskInstanceIDs()
	sort.Strings(want)
	sort.Strings(got)
	if !equal(got, want) {
		t.Fatalf("stream holds %v, want each task instance once: %v", got, want)
	}

	// The returned ids are real stream message ids, in stream order.
	msgs, _ := c.rc.XRange(t.Context(), c.streamName, "-", "+").Result()
	for i, m := range msgs {
		if trig.Enqueued[i] != m.ID {
			t.Fatalf("enqueued[%d] = %s, stream has %s", i, trig.Enqueued[i], m.ID)
		}
	}
}

// Only tasks whose dependencies are met (here: none) are enqueued; the
// dependent task instance stays pending until its upstream finishes.
func TestRunTriggerOnlyEnqueuesTasksWithoutDependencies(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	extract := c.createTask(d.ID, "extract")
	load := c.createTask(d.ID, "load")
	c.addDependency(load.ID, extract.ID)

	trig := c.trigger(d.ID)
	if len(trig.TaskInstances) != 2 {
		t.Fatalf("task instances = %d, want 2", len(trig.TaskInstances))
	}
	var extractTI string
	for _, ti := range trig.TaskInstances {
		if ti.TaskID == extract.ID {
			extractTI = ti.ID
		}
	}

	got := c.streamTaskInstanceIDs()
	if len(got) != 1 || got[0] != extractTI {
		t.Fatalf("stream holds %v, want only extract's instance [%s]", got, extractTI)
	}
}

// An empty run would stay queued forever, so it is rejected and nothing is left behind.
func TestRunTriggerEmptyDAGIsRejected(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("empty")

	c.expectStatus("POST", "/dags/"+d.ID+"/runs", nil, http.StatusBadRequest)

	var runs []runResp
	c.call("GET", "/dags/"+d.ID+"/runs", nil, http.StatusOK, &runs)
	if len(runs) != 0 {
		t.Fatalf("rejected trigger left a run behind: %+v", runs)
	}
	if got := c.streamTaskInstanceIDs(); len(got) != 0 {
		t.Fatalf("stream should be empty, has %v", got)
	}
}

func TestRunTriggerErrors(t *testing.T) {
	c := newAPI(t)
	c.expectStatus("POST", "/dags/not-a-uuid/runs", nil, http.StatusBadRequest)
	c.expectStatus("POST", "/dags/"+randomUUID()+"/runs", nil, http.StatusNotFound)
}

func TestRunListAndGet(t *testing.T) {
	c := newAPI(t)
	d1 := c.createDAG("d1")
	d2 := c.createDAG("d2")
	c.createTask(d1.ID, "t")
	c.createTask(d2.ID, "t")

	var empty []runResp
	c.call("GET", "/dags/"+d1.ID+"/runs", nil, http.StatusOK, &empty)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty list should be [], got %#v", empty)
	}

	r1 := c.triggerRun(d1.ID)
	c.triggerRun(d1.ID)
	c.triggerRun(d2.ID)

	var list []runResp
	c.call("GET", "/dags/"+d1.ID+"/runs", nil, http.StatusOK, &list)
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2 (scoped to DAG)", len(list))
	}

	var got runResp
	c.call("GET", "/runs/"+r1.ID, nil, http.StatusOK, &got)
	if got.ID != r1.ID || got.DAGID != d1.ID || got.Status != "queued" {
		t.Fatalf("got %+v", got)
	}

	c.expectStatus("GET", "/runs/"+randomUUID(), nil, http.StatusNotFound)
	c.expectStatus("GET", "/runs/not-a-uuid", nil, http.StatusBadRequest)
	c.expectStatus("GET", "/dags/not-a-uuid/runs", nil, http.StatusBadRequest)
}

// ---------- Task instances ----------

func TestTaskInstanceGetAndList(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	tk := c.createTask(d.ID, "a")
	run := c.triggerRun(d.ID)

	var list []taskInstanceResp
	c.call("GET", "/runs/"+run.ID+"/task-instances", nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}

	var got taskInstanceResp
	c.call("GET", "/task-instances/"+list[0].ID, nil, http.StatusOK, &got)
	if got.ID != list[0].ID || got.RunID != run.ID || got.TaskID != tk.ID || got.Status != "queued" {
		t.Fatalf("got %+v", got)
	}

	c.expectStatus("GET", "/task-instances/"+randomUUID(), nil, http.StatusNotFound)
	c.expectStatus("GET", "/task-instances/not-a-uuid", nil, http.StatusBadRequest)
	c.expectStatus("GET", "/runs/not-a-uuid/task-instances", nil, http.StatusBadRequest)

	// Unknown run id is just an empty list, not a 404.
	var none []taskInstanceResp
	c.call("GET", "/runs/"+randomUUID()+"/task-instances", nil, http.StatusOK, &none)
	if len(none) != 0 {
		t.Fatalf("expected empty, got %v", none)
	}
}

// ---------- Routing ----------

func TestMethodNotAllowed(t *testing.T) {
	c := newAPI(t)
	c.expectStatus("PATCH", "/dags", nil, http.StatusMethodNotAllowed)
	c.expectStatus("POST", "/runs/"+randomUUID(), nil, http.StatusMethodNotAllowed)
	c.expectStatus("GET", "/nope", nil, http.StatusNotFound)
}

// ---------- Dependencies ----------

func TestDependencies(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	a, b, cc := c.createTask(d.ID, "a"), c.createTask(d.ID, "b"), c.createTask(d.ID, "c")
	path := "/dags/" + d.ID + "/dependencies"
	edge := func(task, dependsOn string) map[string]any {
		return map[string]any{"task_id": task, "depends_on_task_id": dependsOn}
	}

	c.call("POST", path, edge(b.ID, a.ID), http.StatusCreated, nil)  // b after a
	c.call("POST", path, edge(cc.ID, b.ID), http.StatusCreated, nil) // c after b

	var deps []struct {
		TaskID          string `json:"task_id"`
		DependsOnTaskID string `json:"depends_on_task_id"`
	}
	c.call("GET", path, nil, http.StatusOK, &deps)
	if len(deps) != 2 {
		t.Fatalf("deps = %+v, want 2", deps)
	}

	c.expectStatus("POST", path, edge(b.ID, a.ID), http.StatusConflict)   // duplicate
	c.expectStatus("POST", path, edge(a.ID, a.ID), http.StatusBadRequest) // self
	c.expectStatus("POST", path, edge(a.ID, b.ID), http.StatusConflict)   // a<->b cycle
	c.expectStatus("POST", path, edge(a.ID, cc.ID), http.StatusConflict)  // a->c->b->a, transitive
	c.expectStatus("POST", path, map[string]any{"task_id": "x"}, http.StatusBadRequest)
	c.expectStatus("POST", "/dags/not-a-uuid/dependencies", edge(a.ID, b.ID), http.StatusBadRequest)

	other := c.createDAG("other")
	o := c.createTask(other.ID, "o")
	c.expectStatus("POST", path, edge(a.ID, o.ID), http.StatusNotFound) // task from another dag
	c.expectStatus("POST", path, edge(a.ID, randomUUID()), http.StatusNotFound)

	del := path + "/" + b.ID + "/" + a.ID
	c.expectStatus("DELETE", del, nil, http.StatusNoContent)
	c.expectStatus("DELETE", del, nil, http.StatusNotFound)
	c.expectStatus("DELETE", path+"/"+b.ID+"/nope", nil, http.StatusBadRequest)
	c.expectStatus("POST", path, edge(a.ID, cc.ID), http.StatusCreated) // legal now that b->a is gone
}

// A dependency created through the API must hold the dependent task back at trigger time.
func TestDependencyHoldsTaskBack(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	a, b := c.createTask(d.ID, "a"), c.createTask(d.ID, "b")
	c.call("POST", "/dags/"+d.ID+"/dependencies",
		map[string]any{"task_id": b.ID, "depends_on_task_id": a.ID}, http.StatusCreated, nil)

	trig := c.trigger(d.ID)
	if len(trig.Enqueued) != 1 {
		t.Fatalf("enqueued = %d, want only the root", len(trig.Enqueued))
	}
	for _, ti := range trig.TaskInstances {
		want := "pending"
		if ti.TaskID == a.ID {
			want = "queued"
		}
		if ti.Status != want {
			t.Fatalf("task %s status = %s, want %s", ti.TaskID, ti.Status, want)
		}
	}
}

// ---------- Fields the worker writes ----------

func TestRunAndInstanceExposeWorkerFields(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	c.createTask(d.ID, "a")
	trig := c.trigger(d.ID)
	runID, tiID := trig.Run.ID, trig.TaskInstances[0].ID

	var run, ti map[string]any
	c.call("GET", "/runs/"+runID, nil, http.StatusOK, &run)
	if run["triggered_by"] != "manual" || run["started_at"] != nil || run["finished_at"] != nil {
		t.Fatalf("fresh run = %+v", run)
	}
	c.call("GET", "/task-instances/"+tiID, nil, http.StatusOK, &ti)
	if ti["attempt"] != float64(1) || ti["worker_id"] != nil || ti["started_at"] != nil ||
		ti["logs"] != nil || ti["output"] != nil {
		t.Fatalf("fresh instance = %+v", ti)
	}
	if _, ok := ti["log_location"]; ok {
		t.Fatalf("stale log_location still returned: %+v", ti)
	}

	// What the worker writes must come back out of the API.
	ctx := context.Background()
	if _, err := c.pool.Exec(ctx, `UPDATE task_instances SET status='success', worker_id='w1',
		started_at=NOW(), finished_at=NOW(), logs='hello', output='{"rows":3}' WHERE id=$1`, tiID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.pool.Exec(ctx, `UPDATE runs SET status='success', started_at=NOW(), finished_at=NOW() WHERE id=$1`, runID); err != nil {
		t.Fatal(err)
	}
	ti, run = nil, nil
	c.call("GET", "/task-instances/"+tiID, nil, http.StatusOK, &ti)
	if ti["worker_id"] != "w1" || ti["logs"] != "hello" || ti["started_at"] == nil || ti["finished_at"] == nil ||
		ti["output"].(map[string]any)["rows"] != float64(3) {
		t.Fatalf("instance = %+v", ti)
	}
	var list []map[string]any
	c.call("GET", "/runs/"+runID+"/task-instances", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0]["logs"] != "hello" {
		t.Fatalf("list = %+v", list)
	}
	c.call("GET", "/runs/"+runID, nil, http.StatusOK, &run)
	if run["started_at"] == nil || run["finished_at"] == nil {
		t.Fatalf("run = %+v", run)
	}
}

// ---------- python_version and error format ----------

func TestDAGPythonVersion(t *testing.T) {
	c := newAPI(t)

	var d struct {
		ID            string  `json:"id"`
		PythonVersion *string `json:"python_version"`
	}
	c.call("POST", "/dags", map[string]any{"name": "none"}, http.StatusCreated, &d)
	if d.PythonVersion != nil {
		t.Fatalf("default python_version = %v, want null", *d.PythonVersion)
	}

	c.call("POST", "/dags", map[string]any{"name": "py", "python_version": "3.11"}, http.StatusCreated, &d)
	if d.PythonVersion == nil || *d.PythonVersion != "3.11" {
		t.Fatalf("created python_version = %v", d.PythonVersion)
	}

	c.call("PUT", "/dags/"+d.ID, map[string]any{"python_version": "3.13"}, http.StatusOK, &d)
	if *d.PythonVersion != "3.13" {
		t.Fatalf("updated python_version = %v", *d.PythonVersion)
	}
	c.call("PUT", "/dags/"+d.ID, map[string]any{"owner": "me"}, http.StatusOK, &d) // unrelated update keeps it
	c.call("GET", "/dags/"+d.ID, nil, http.StatusOK, &d)
	if d.PythonVersion == nil || *d.PythonVersion != "3.13" {
		t.Fatalf("python_version lost on unrelated update: %v", d.PythonVersion)
	}

	for _, bad := range []string{"3", "3.", "2.7", "3.11.2", "python3.11", "3.x", ""} {
		c.expectStatus("POST", "/dags", map[string]any{"name": "bad" + bad, "python_version": bad}, http.StatusBadRequest)
		c.expectStatus("PUT", "/dags/"+d.ID, map[string]any{"python_version": bad}, http.StatusBadRequest)
	}
}

func TestErrorsAreJSON(t *testing.T) {
	c := newAPI(t)
	for _, path := range []string{"/runs/not-a-uuid", "/runs/" + randomUUID(), "/task-instances/not-a-uuid", "/tasks/" + randomUUID()} {
		status, raw := c.do("GET", path, nil)
		var body map[string]string
		if status < 400 || json.Unmarshal(raw, &body) != nil || body["error"] == "" {
			t.Fatalf("GET %s: status %d body %q, want a JSON {\"error\": ...}", path, status, raw)
		}
	}
}
