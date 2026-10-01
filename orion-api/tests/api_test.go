package tests

import (
	"encoding/json"
	"net/http"
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

	// One pending task instance per task is created.
	var tis []taskInstanceResp
	c.call("GET", "/dags/"+run.ID+"/task-instances", nil, http.StatusOK, &tis)
	if len(tis) != 2 {
		t.Fatalf("task instances = %d, want 2", len(tis))
	}
	seen := map[string]bool{}
	for _, ti := range tis {
		if ti.RunID != run.ID || ti.Status != "pending" {
			t.Fatalf("unexpected task instance: %+v", ti)
		}
		seen[ti.TaskID] = true
	}
	if !seen[t1.ID] || !seen[t2.ID] {
		t.Fatalf("task instances don't cover both tasks: %+v", tis)
	}
}

func TestRunTriggerEmptyDAG(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("empty")
	run := c.triggerRun(d.ID)

	var tis []taskInstanceResp
	c.call("GET", "/dags/"+run.ID+"/task-instances", nil, http.StatusOK, &tis)
	if len(tis) != 0 {
		t.Fatalf("expected no task instances, got %d", len(tis))
	}
}

func TestRunTriggerErrors(t *testing.T) {
	c := newAPI(t)
	c.expectStatus("POST", "/dags/not-a-uuid/runs", nil, http.StatusBadRequest)
	// Unknown DAG -> FK violation, currently 500.
	c.expectStatus("POST", "/dags/"+randomUUID()+"/runs", nil, http.StatusInternalServerError)
}

func TestRunListAndGet(t *testing.T) {
	c := newAPI(t)
	d1 := c.createDAG("d1")
	d2 := c.createDAG("d2")

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
	c.call("GET", "/dags/"+run.ID+"/task-instances", nil, http.StatusOK, &list)
	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}

	var got taskInstanceResp
	c.call("GET", "/task-instances/"+list[0].ID, nil, http.StatusOK, &got)
	if got.ID != list[0].ID || got.RunID != run.ID || got.TaskID != tk.ID || got.Status != "pending" {
		t.Fatalf("got %+v", got)
	}

	c.expectStatus("GET", "/task-instances/"+randomUUID(), nil, http.StatusNotFound)
	c.expectStatus("GET", "/task-instances/not-a-uuid", nil, http.StatusBadRequest)
	c.expectStatus("GET", "/dags/not-a-uuid/task-instances", nil, http.StatusBadRequest)

	// Unknown run id is just an empty list, not a 404.
	var none []taskInstanceResp
	c.call("GET", "/dags/"+randomUUID()+"/task-instances", nil, http.StatusOK, &none)
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
