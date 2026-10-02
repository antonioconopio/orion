package tests

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"orion-api/internal/queue"
)

// Triggering a run enqueues its ready task instances; simulated workers then
// consume them. Workers mirror the real contract from queue.go: update
// Postgres first, ack only afterwards.

// runWorker simulates one worker: dequeue -> mark running -> mark success
// (in Postgres) -> ack. It stops when ctx is done or the queue stays empty.
func runWorker(ctx context.Context, t *testing.T, c *apiClient, s *queue.Stream, name string, processed *sync.Map) {
	for ctx.Err() == nil {
		streams, err := s.Dequeue(ctx, name, 2, 200*time.Millisecond)
		if err != nil {
			if ctx.Err() != nil || err.Error() == "redis: nil" {
				return
			}
			t.Errorf("%s: dequeue: %v", name, err)
			return
		}
		for _, st := range streams {
			for _, m := range st.Messages {
				tiID := m.Values["task_instance_id"].(string)
				if _, err := c.pool.Exec(ctx,
					`UPDATE task_instances SET status='success', worker_id=$2, started_at=now(), finished_at=now() WHERE id=$1`,
					tiID, name); err != nil {
					t.Errorf("%s: update %s: %v", name, tiID, err)
					return
				}
				if err := s.Ack(ctx, m.ID); err != nil {
					t.Errorf("%s: ack: %v", name, err)
					return
				}
				processed.Store(tiID, name)
			}
		}
	}
}

func TestQueueEndToEnd(t *testing.T) {
	c := newAPI(t)
	rc, name, s := c.rc, c.streamName, c.stream
	ctx := context.Background()

	// 1. Create a DAG with tasks and trigger a run via the API, which
	// enqueues the ready task instances itself.
	d := c.createDAG("pipeline")
	const taskCount = 8
	for i := 0; i < taskCount; i++ {
		c.createTask(d.ID, string(rune('a'+i)))
	}
	trig := c.trigger(d.ID)
	tis := trig.TaskInstances
	if len(tis) != taskCount || len(trig.Enqueued) != taskCount {
		t.Fatalf("task instances = %d, enqueued = %d, want %d each", len(tis), len(trig.Enqueued), taskCount)
	}

	// 2. Three workers drain the queue concurrently.
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var processed sync.Map
	var wg sync.WaitGroup
	for _, w := range []string{"worker-1", "worker-2", "worker-3"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runWorker(wctx, t, c, s, w, &processed)
		}()
	}
	// Stop once everything is processed.
	go func() {
		for wctx.Err() == nil {
			n := 0
			processed.Range(func(_, _ any) bool { n++; return true })
			if n == taskCount {
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	wg.Wait()

	// 3. Every task instance processed exactly once, visible through the API.
	var gotIDs, wantIDs []string
	processed.Range(func(k, _ any) bool { gotIDs = append(gotIDs, k.(string)); return true })
	for _, ti := range tis {
		wantIDs = append(wantIDs, ti.ID)
	}
	sort.Strings(gotIDs)
	sort.Strings(wantIDs)
	if len(gotIDs) != taskCount || !equal(gotIDs, wantIDs) {
		t.Fatalf("processed %v, want %v", gotIDs, wantIDs)
	}
	for _, ti := range tis {
		var got taskInstanceResp
		c.call("GET", "/task-instances/"+ti.ID, nil, http.StatusOK, &got)
		if got.Status != "success" {
			t.Fatalf("task instance %s status = %q, want success", ti.ID, got.Status)
		}
	}

	// 4. Nothing left pending or undelivered.
	if got := pendingCount(t, rc, name); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
	if _, ids := dequeueIDs(t, s, "worker-late", 10, 100*time.Millisecond); len(ids) != 0 {
		t.Fatalf("queue should be drained, got %v", ids)
	}
}

func TestQueueEndToEndCrashedWorkerIsRecovered(t *testing.T) {
	c := newAPI(t)
	rc, name, s := c.rc, c.streamName, c.stream
	ctx := context.Background()

	d := c.createDAG("flaky")
	c.createTask(d.ID, "only")
	tiID := c.trigger(d.ID).TaskInstances[0].ID

	// worker-crash picks it up, marks it running, then dies before acking.
	_, got := dequeueIDs(t, s, "worker-crash", 1, time.Second)
	if len(got) != 1 || got[0] != tiID {
		t.Fatalf("worker-crash got %v", got)
	}
	if _, err := c.pool.Exec(ctx, `UPDATE task_instances SET status='running', worker_id='worker-crash' WHERE id=$1`, tiID); err != nil {
		t.Fatal(err)
	}
	var mid taskInstanceResp
	c.call("GET", "/task-instances/"+tiID, nil, http.StatusOK, &mid)
	if mid.Status != "running" {
		t.Fatalf("status = %q, want running", mid.Status)
	}

	// The message is invisible to normal reads: nobody else will get it.
	if _, ids := dequeueIDs(t, s, "worker-healthy", 1, 100*time.Millisecond); len(ids) != 0 {
		t.Fatalf("unacked message was redelivered via Dequeue: %v", ids)
	}

	// A healthy worker reclaims it after it has been idle long enough...
	time.Sleep(150 * time.Millisecond)
	claimed, _, err := s.ClaimStale(ctx, "worker-healthy", 100*time.Millisecond)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("ClaimStale = %v, err = %v; want 1 message", claimed, err)
	}
	if claimed[0].Values["task_instance_id"] != tiID {
		t.Fatalf("claimed wrong task instance: %v", claimed[0].Values)
	}

	// ...finishes the work, then acks.
	if _, err := c.pool.Exec(ctx, `UPDATE task_instances SET status='success', worker_id='worker-healthy' WHERE id=$1`, tiID); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(ctx, claimed[0].ID); err != nil {
		t.Fatal(err)
	}

	var final taskInstanceResp
	c.call("GET", "/task-instances/"+tiID, nil, http.StatusOK, &final)
	if final.Status != "success" {
		t.Fatalf("final status = %q, want success", final.Status)
	}
	if got := pendingCount(t, rc, name); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
