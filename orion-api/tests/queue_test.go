package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"orion-api/internal/queue"
)

const testGroup = "orion:test-workers"

// newTestStream builds a Stream on a private stream name.
func newTestStream(t *testing.T, maxLen int64) (*queue.Stream, *redis.Client, string) {
	t.Helper()
	rc := redisClient(t)
	name := uniqueStreamName(t, rc)
	s, err := queue.NewStream(context.Background(), redisAddr(), name, testGroup, maxLen)
	if err != nil {
		t.Fatalf("NewStream: %v", err)
	}
	return s, rc, name
}

func pendingCount(t *testing.T, rc *redis.Client, stream string) int64 {
	t.Helper()
	p, err := rc.XPending(context.Background(), stream, testGroup).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	return p.Count
}

// dequeueIDs reads up to count messages and returns (messageIDs, taskInstanceIDs).
func dequeueIDs(t *testing.T, s *queue.Stream, consumer string, count int64, block time.Duration) ([]string, []string) {
	t.Helper()
	streams, err := s.Dequeue(context.Background(), consumer, count, block)
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		t.Fatalf("Dequeue(%s): %v", consumer, err)
	}
	var msgIDs, tiIDs []string
	for _, st := range streams {
		for _, m := range st.Messages {
			msgIDs = append(msgIDs, m.ID)
			tiIDs = append(tiIDs, m.Values["task_instance_id"].(string))
		}
	}
	return msgIDs, tiIDs
}

func TestQueueNewStreamIsIdempotent(t *testing.T) {
	rc := redisClient(t)
	name := uniqueStreamName(t, rc)
	ctx := context.Background()

	// Second call hits BUSYGROUP and must still succeed.
	for i := 0; i < 2; i++ {
		if _, err := queue.NewStream(ctx, redisAddr(), name, testGroup, 100); err != nil {
			t.Fatalf("NewStream call %d: %v", i+1, err)
		}
	}

	groups, err := rc.XInfoGroups(ctx, name).Result()
	if err != nil || len(groups) != 1 || groups[0].Name != testGroup {
		t.Fatalf("groups = %+v, err = %v", groups, err)
	}
}

func TestQueueNewStreamUnreachableRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Port 1 is never a Redis server; must fail fast, not hang or panic.
	if _, err := queue.NewStream(ctx, "127.0.0.1:1", "orion:test:x", testGroup, 100); err == nil {
		t.Fatal("expected an error connecting to an unreachable redis")
	}
}

func TestQueueEnqueueDequeueAck(t *testing.T) {
	s, rc, name := newTestStream(t, 1000)
	ctx := context.Background()
	tiID := randomUUID()

	msgID, err := s.Enqueue(ctx, tiID)
	if err != nil || msgID == "" {
		t.Fatalf("Enqueue: id=%q err=%v", msgID, err)
	}
	if n, _ := rc.XLen(ctx, name).Result(); n != 1 {
		t.Fatalf("stream length = %d, want 1", n)
	}

	msgIDs, tiIDs := dequeueIDs(t, s, "worker-1", 10, time.Second)
	if len(msgIDs) != 1 || msgIDs[0] != msgID || tiIDs[0] != tiID {
		t.Fatalf("dequeued msgs=%v tis=%v, want %s / %s", msgIDs, tiIDs, msgID, tiID)
	}

	// Delivered but not acked -> pending.
	if got := pendingCount(t, rc, name); got != 1 {
		t.Fatalf("pending = %d, want 1 before ack", got)
	}

	if err := s.Ack(ctx, msgID); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if got := pendingCount(t, rc, name); got != 0 {
		t.Fatalf("pending = %d, want 0 after ack", got)
	}
}

func TestQueueFIFOOrder(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)
	ctx := context.Background()

	var want []string
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("ti-%d", i)
		want = append(want, id)
		if _, err := s.Enqueue(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	_, got := dequeueIDs(t, s, "w", 10, time.Second)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestQueueDequeueEmptyBlocksThenTimesOut(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)

	start := time.Now()
	streams, err := s.Dequeue(context.Background(), "w", 1, 300*time.Millisecond)
	elapsed := time.Since(start)

	if !errors.Is(err, redis.Nil) {
		t.Fatalf("err = %v (streams=%v), want redis.Nil on empty queue", err, streams)
	}
	if elapsed < 250*time.Millisecond {
		t.Fatalf("returned after %v; should have blocked ~300ms", elapsed)
	}
}

func TestQueueDequeueUnblocksOnEnqueue(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)
	ctx := context.Background()

	go func() {
		time.Sleep(150 * time.Millisecond)
		s.Enqueue(ctx, "late-arrival")
	}()

	start := time.Now()
	_, tiIDs := dequeueIDs(t, s, "w", 1, 5*time.Second)
	if len(tiIDs) != 1 || tiIDs[0] != "late-arrival" {
		t.Fatalf("got %v", tiIDs)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("blocked read didn't wake promptly: %v", time.Since(start))
	}
}

func TestQueueDequeueRespectsCount(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s.Enqueue(ctx, fmt.Sprintf("ti-%d", i))
	}

	first, _ := dequeueIDs(t, s, "w", 2, time.Second)
	second, _ := dequeueIDs(t, s, "w", 10, time.Second)
	if len(first) != 2 || len(second) != 3 {
		t.Fatalf("batch sizes = %d, %d; want 2, 3", len(first), len(second))
	}
}

func TestQueueGroupOnlyDeliversMessagesAfterCreation(t *testing.T) {
	rc := redisClient(t)
	name := uniqueStreamName(t, rc)
	ctx := context.Background()

	// A message that exists before the group does ("$" start id).
	if err := rc.XAdd(ctx, &redis.XAddArgs{Stream: name, Values: map[string]any{"task_instance_id": "old"}}).Err(); err != nil {
		t.Fatal(err)
	}
	s, err := queue.NewStream(ctx, redisAddr(), name, testGroup, 100)
	if err != nil {
		t.Fatal(err)
	}
	s.Enqueue(ctx, "new")

	_, got := dequeueIDs(t, s, "w", 10, time.Second)
	if len(got) != 1 || got[0] != "new" {
		t.Fatalf("got %v, want only [new]", got)
	}
}

func TestQueueConsumersShareWorkWithoutDuplicates(t *testing.T) {
	s, rc, name := newTestStream(t, 1000)
	ctx := context.Background()

	const total = 20
	enqueued := map[string]bool{}
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("ti-%d", i)
		enqueued[id] = true
		s.Enqueue(ctx, id)
	}

	// Two workers alternate pulling small batches until the queue drains.
	seen := map[string]string{} // ti id -> consumer
	for i := 0; i < total; i++ {
		for _, w := range []string{"worker-a", "worker-b"} {
			msgIDs, tiIDs := dequeueIDs(t, s, w, 3, 100*time.Millisecond)
			for j, ti := range tiIDs {
				if prev, dup := seen[ti]; dup {
					t.Fatalf("%s delivered to both %s and %s", ti, prev, w)
				}
				seen[ti] = w
				if err := s.Ack(ctx, msgIDs[j]); err != nil {
					t.Fatal(err)
				}
			}
		}
		if len(seen) == total {
			break
		}
	}

	if len(seen) != total {
		t.Fatalf("delivered %d of %d messages", len(seen), total)
	}
	for id := range enqueued {
		if _, ok := seen[id]; !ok {
			t.Fatalf("%s never delivered", id)
		}
	}
	if got := pendingCount(t, rc, name); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}

func TestQueueAckIsIdempotentAndIgnoresUnknownIDs(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)
	ctx := context.Background()

	msgID, _ := s.Enqueue(ctx, "x")
	dequeueIDs(t, s, "w", 1, time.Second)

	if err := s.Ack(ctx, msgID); err != nil {
		t.Fatal(err)
	}
	if err := s.Ack(ctx, msgID); err != nil {
		t.Fatalf("second Ack: %v", err)
	}
	if err := s.Ack(ctx, "0-1"); err != nil {
		t.Fatalf("Ack unknown id: %v", err)
	}
}

func TestQueueClaimStaleReassignsCrashedWorkersMessages(t *testing.T) {
	s, rc, name := newTestStream(t, 1000)
	ctx := context.Background()

	msgID, _ := s.Enqueue(ctx, "in-flight")
	// worker-dead takes the message and "crashes" without acking.
	dequeueIDs(t, s, "worker-dead", 1, time.Second)

	// Too fresh to be considered stale.
	msgs, _, err := s.ClaimStale(ctx, "worker-live", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("claimed %d fresh messages, want 0", len(msgs))
	}

	time.Sleep(150 * time.Millisecond)

	msgs, _, err = s.ClaimStale(ctx, "worker-live", 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].ID != msgID || msgs[0].Values["task_instance_id"] != "in-flight" {
		t.Fatalf("claimed %+v, want the in-flight message", msgs)
	}

	// Ownership moved to worker-live.
	ext, err := rc.XPendingExt(ctx, &redis.XPendingExtArgs{Stream: name, Group: testGroup, Start: "-", End: "+", Count: 10}).Result()
	if err != nil || len(ext) != 1 || ext[0].Consumer != "worker-live" {
		t.Fatalf("pending = %+v, err = %v; want owned by worker-live", ext, err)
	}

	// The new owner can finish and ack it.
	if err := s.Ack(ctx, msgID); err != nil {
		t.Fatal(err)
	}
	if got := pendingCount(t, rc, name); got != 0 {
		t.Fatalf("pending = %d after ack, want 0", got)
	}
}

func TestQueueClaimStaleSkipsAckedMessages(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)
	ctx := context.Background()

	msgID, _ := s.Enqueue(ctx, "done")
	dequeueIDs(t, s, "w1", 1, time.Second)
	s.Ack(ctx, msgID)

	time.Sleep(60 * time.Millisecond)
	msgs, _, err := s.ClaimStale(ctx, "w2", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("claimed acked messages: %+v", msgs)
	}
}

func TestQueueMaxLenTrimsOldEntries(t *testing.T) {
	s, rc, name := newTestStream(t, 100)
	ctx := context.Background()

	const total = 1000
	for i := 0; i < total; i++ {
		if _, err := s.Enqueue(ctx, fmt.Sprintf("ti-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	// MAXLEN ~ is approximate (trims whole radix nodes), so only assert
	// it's bounded well below `total` and not over-trimmed below the cap.
	n, err := rc.XLen(ctx, name).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n >= total || n < 100 {
		t.Fatalf("stream length = %d, want in [100, %d)", n, total)
	}
}

func TestQueueRespectsContextCancellation(t *testing.T) {
	s, _, _ := newTestStream(t, 1000)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := s.Dequeue(ctx, "w", 1, 2*time.Second)
	// Dequeue's block (2s) is far longer than the ctx deadline (100ms): a
	// worker shutting down via ctx should be released promptly.
	if time.Since(start) > time.Second {
		t.Fatalf("Dequeue ignored ctx cancellation; returned after %v with err=%v", time.Since(start), err)
	}
}
