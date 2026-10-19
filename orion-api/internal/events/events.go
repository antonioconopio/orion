// Package events carries "something changed" notices from whoever changes state (the API's
// trigger endpoint, the Python workers) to browsers.
//
// Flow: publisher -> Redis Pub/Sub channel -> Hub (one subscription in the API) -> every WebSocket client.
// Events are only hints: the database stays the source of truth, so a lost event is harmless
// (Pub/Sub drops messages when nobody is listening) and clients simply refetch.
package events

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// Channel is the Redis Pub/Sub channel workers and the API publish to.
const Channel = "orion:events"

// Event types.
const (
	TypeRun    = "run"    // something about a run or its task instances changed
	TypeWorker = "worker" // a worker started, finished a task, or went away
)

// Event is the JSON message sent to browsers. Workers publish the same shape from Python.
type Event struct {
	Type  string `json:"type"`
	RunID string `json:"run_id,omitempty"`
	DAGID string `json:"dag_id,omitempty"`
}

// Publish sends ev to every Hub. Callers treat a failure as non-fatal.
func Publish(ctx context.Context, rdb *redis.Client, ev Event) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return rdb.Publish(ctx, Channel, payload).Err()
}
