// Package queue wraps a single Redis Stream used to hand task instances
// off to the worker pool.
//
// Design decisions baked into this skeleton (change them if you disagree,
// but do it before filling in bodies, not after):
//   - One Stream value = one fixed stream name + one fixed consumer group.
//     No "queueName" parameter floats around separately — if you ever want
//     a second stream, you construct a second *Stream.
//   - Every method takes a ctx from the caller. Nothing stores a
//     long-lived context.Background() internally — same rule as db.NewPool.
//   - Dequeue identifies the calling worker by consumerName, so Redis can
//     track which consumer is holding which unacknowledged message.
package queue

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// BUSYGROUP is the error message Redis returns from XGroupCreate when
	// the group already exists. You'll want to detect this specific
	// string and treat it as success, not failure — group creation needs
	// to be idempotent since it runs every time the API starts up.
	errBusyGroup = "BUSYGROUP"
)

// Stream wraps one Redis Stream + one consumer group on it.
type Stream struct {
	client *redis.Client
	name   string // e.g. "orion:tasks_stream"
	group  string // e.g. "orion:workers"
	maxLen int64  // approximate MAXLEN retention guardrail
}

// NewStream connects to Redis and ensures the consumer group exists.
//
// TODO:
//  1. redis.NewClient(&redis.Options{Addr: addr})
//  2. client.Ping(ctx) to fail fast at startup, same pattern as db.NewPool
//  3. client.XGroupCreateMkStream(ctx, name, group, "$").Err()
//     - "$" means "start the group at the end of the stream" (only new
//       messages after this point get delivered) — right for a group
//       being created for the first time.
//     - if the returned error's string contains errBusyGroup, that means
//       the group already exists from a previous run — swallow that one
//       error and return success. Any other error is real and should
//       propagate.
func NewStream(ctx context.Context, addr, name, group string, maxLen int64) (*Stream, error) {

	redisClient := redis.NewClient(&redis.Options{Addr: addr})

	err := redisClient.Ping(ctx).Err()
	if err != nil {
		return nil, err
	}

	err = redisClient.XGroupCreateMkStream(ctx, name, group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), errBusyGroup) {
		return nil, err
	}

	return &Stream{
		client: redisClient,
		name:   name,
		group:  group,
		maxLen: maxLen,
	}, nil
}


// Enqueue adds a task instance to the stream. Returns the Redis-assigned
// message ID (useful for logging/debugging, not something you need to
// store anywhere).
func (s *Stream) Enqueue(ctx context.Context, taskInstanceID string) (string, error) {

	messageID, err := s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: s.name,
		MaxLen: s.maxLen,
		Approx: true,
		Values: map[string]interface{}{"task_instance_id": taskInstanceID},
	}).Result()

	return messageID, err 
}

// Dequeue reads up to `count` pending messages for this consumer group,
// blocking up to `block` for new messages if none are immediately
// available. consumerName must be unique per worker process (e.g.
// hostname + pid, or a UUID generated at worker startup) — it's how
// Redis knows who to blame if a message never gets acked.
//
// TODO: client.XReadGroup(ctx, &redis.XReadGroupArgs{
//     Group:    s.group,
//     Consumer: consumerName,
//     Streams:  []string{s.name, ">"}, // ">" = only new, never-delivered messages
//     Count:    count,
//     Block:    block,
// }).Result()
//
// Returns []redis.XStream — each has a Messages []redis.XMessage, each
// message has an ID (needed for Ack) and a Values map (your
// "task_instance_id" comes back out of here).
func (s *Stream) Dequeue(ctx context.Context, consumerName string, count int64, block time.Duration) ([]redis.XStream, error) {

	xstreams, err := s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.group,
		Consumer: consumerName,
		Streams:  []string{s.name, ">"},
		Count:    count,
		Block:    block,
	}).Result()

	return xstreams, err
}

// Ack marks a message as successfully processed, removing it from the
// group's pending entries list. Call this only after the task instance's
// status has actually been updated in Postgres — if you ack first and
// the status write fails, the message is gone and nothing will ever
// retry it.

func (s *Stream) Ack(ctx context.Context, messageID string) error {

	err := s.client.XAck(ctx, s.name, s.group, messageID).Err()

	return err 
}

// ClaimStale looks for messages that were delivered to some consumer but
// never acked within minIdle — i.e. a worker probably crashed mid-task —
// and reassigns them to consumerName so they get retried.
//
// This is the piece that makes the whole system crash-tolerant. Without
// it, a dead worker's in-flight tasks sit in the pending list forever.
// You'll likely call this on a timer (e.g. every 30s) from each worker,
// not from the API.
func (s *Stream) ClaimStale(ctx context.Context, consumerName string, minIdle time.Duration) ([]redis.XMessage, string, error) {

	xmessages, start, err := s.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   s.name,
		Group:    s.group,
		Consumer: consumerName,
		MinIdle:  minIdle,
		Start:    "0-0",
	}).Result()

	return xmessages, start, err 
}
