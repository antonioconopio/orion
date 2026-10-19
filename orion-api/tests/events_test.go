package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"orion-api/internal/events"
	"orion-api/internal/handlers"
)

// publishRun publishes a run event the way a worker does.
func (c *apiClient) publishRun(runID string) {
	c.t.Helper()
	ev := events.Event{Type: events.TypeRun, RunID: runID}
	if err := events.Publish(context.Background(), c.stream.Client(), ev); err != nil {
		c.t.Fatal(err)
	}
}

// waitForRunEvent reads from ch until it sees an event for runID. Other events are skipped:
// the channel is shared with any real worker or API running against the same Redis.
func waitForRunEvent(t *testing.T, next func() ([]byte, bool), runID string) events.Event {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		type result struct {
			msg []byte
			ok  bool
		}
		got := make(chan result, 1)
		go func() { m, ok := next(); got <- result{m, ok} }()
		select {
		case r := <-got:
			if !r.ok {
				t.Fatal("event channel closed before the expected event arrived")
			}
			var ev events.Event
			if err := json.Unmarshal(r.msg, &ev); err != nil {
				t.Fatalf("bad event %q: %v", r.msg, err)
			}
			if ev.RunID == runID {
				return ev
			}
		case <-deadline:
			t.Fatalf("no event for run %s within 3s", runID)
		}
	}
}

func hubNext(ch <-chan []byte) func() ([]byte, bool) {
	return func() ([]byte, bool) { m, ok := <-ch; return m, ok }
}

func TestHubDeliversPublishedEvents(t *testing.T) {
	c := newAPI(t)
	ch, cancel := c.hub.Subscribe()
	defer cancel()
	id := randomUUID()
	c.publishRun(id)
	ev := waitForRunEvent(t, hubNext(ch), id)
	if ev.Type != events.TypeRun {
		t.Fatalf("type = %q", ev.Type)
	}
}

func TestHubFansOutToEveryClient(t *testing.T) {
	c := newAPI(t)
	a, cancelA := c.hub.Subscribe()
	b, cancelB := c.hub.Subscribe()
	defer cancelA()
	defer cancelB()
	id := randomUUID()
	c.publishRun(id)
	waitForRunEvent(t, hubNext(a), id)
	waitForRunEvent(t, hubNext(b), id)
}

func TestHubDropsAClientThatFallsBehind(t *testing.T) {
	c := newAPI(t)
	slow, cancel := c.hub.Subscribe() // never read from
	defer cancel()
	fast, cancelFast := c.hub.Subscribe()
	defer cancelFast()
	go func() { // keep the fast client drained
		for range fast {
		}
	}()

	for i := 0; i < 200; i++ {
		c.publishRun(randomUUID())
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-slow:
			if !ok {
				return // closed: dropped, and publishing never blocked
			}
		case <-deadline:
			t.Fatal("slow client was never dropped")
		}
	}
}

func TestTriggerPublishesARunEvent(t *testing.T) {
	c := newAPI(t)
	d := c.createDAG("d")
	c.createTask(d.ID, "t")
	ch, cancel := c.hub.Subscribe()
	defer cancel()

	run := c.triggerRun(d.ID)
	ev := waitForRunEvent(t, hubNext(ch), run.ID)
	if ev.Type != events.TypeRun || ev.DAGID != d.ID {
		t.Fatalf("event = %+v, want a run event for dag %s", ev, d.ID)
	}
}

func wsURL(c *apiClient) string { return "ws" + strings.TrimPrefix(c.srv.URL, "http") + "/ws" }

func TestWebSocketStreamsEvents(t *testing.T) {
	c := newAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL(c), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{testUIOrigin}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	id := randomUUID()
	go func() { // the hub registers the client right after the upgrade; publish until it arrives
		for i := 0; i < 20 && ctx.Err() == nil; i++ {
			c.publishRun(id)
			time.Sleep(50 * time.Millisecond)
		}
	}()
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var ev events.Event
		json.Unmarshal(msg, &ev)
		if ev.RunID == id {
			return
		}
	}
}

func TestWebSocketRejectsAForeignOrigin(t *testing.T) {
	c := newAPI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, resp, err := websocket.Dial(ctx, wsURL(c), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://evil.example"}},
	})
	if err == nil {
		t.Fatal("a foreign origin must not be able to connect")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("response = %+v, want 403", resp)
	}
}

// ---------- /workers ----------

func TestWorkersEndpoint(t *testing.T) {
	c := newAPI(t)
	ctx := context.Background()
	rdb := c.stream.Client()

	h := handlers.NewWorkerHandler(c.pool, rdb)
	h.Prefix = "orion:test:worker:" + randomUUID() + ":" // keep clear of real workers' keys
	get := func() []map[string]any {
		rec := httptest.NewRecorder()
		h.List(rec, httptest.NewRequest("GET", "/workers", nil))
		if rec.Code != 200 {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var out []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out == nil {
			t.Fatalf("want a JSON array, got %q (%v)", rec.Body, err)
		}
		return out
	}
	if got := get(); len(got) != 0 {
		t.Fatalf("no workers yet, got %v", got)
	}

	d := c.createDAG("etl")
	c.createTask(d.ID, "extract")
	trig := c.trigger(d.ID)
	instanceID := trig.TaskInstances[0].ID

	beat := func(id, status string, instance *string) {
		body, _ := json.Marshal(map[string]any{
			"id": id, "host": "host-" + id, "pid": 7, "started_at": time.Now().UTC().Format(time.RFC3339),
			"status": status, "current_instance_id": instance,
		})
		if err := rdb.Set(ctx, h.Prefix+id, body, 30*time.Second).Err(); err != nil {
			t.Fatal(err)
		}
	}
	beat("w-b", "busy", &instanceID)
	beat("w-a", "idle", nil)
	rdb.Set(ctx, h.Prefix+"junk", "not json", 30*time.Second)
	t.Cleanup(func() { rdb.Del(ctx, h.Prefix+"w-a", h.Prefix+"w-b", h.Prefix+"junk") })

	got := get()
	if len(got) != 2 || got[0]["id"] != "w-a" || got[1]["id"] != "w-b" {
		t.Fatalf("want [w-a w-b] sorted, junk ignored: %v", got)
	}
	if got[0]["status"] != "idle" || got[0]["current_task"] != nil {
		t.Fatalf("idle worker = %v", got[0])
	}
	if got[1]["status"] != "busy" || got[1]["current_task"] != "etl / extract" || got[1]["current_run_id"] != trig.Run.ID {
		t.Fatalf("busy worker = %v", got[1])
	}
}
