package events

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// clientBuffer is how many events a browser may fall behind before it is dropped.
const clientBuffer = 64

// Hub holds one Redis subscription and copies each message to every connected client.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[chan []byte]struct{}{}}
}

// Start subscribes to Channel and forwards messages until ctx is cancelled. It returns once the
// subscription is confirmed, so events published after Start returns are never missed.
func (h *Hub) Start(ctx context.Context, rdb *redis.Client) error {
	sub := rdb.Subscribe(ctx, Channel)
	if _, err := sub.Receive(ctx); err != nil {
		sub.Close()
		return err
	}
	go func() {
		defer sub.Close()
		for msg := range sub.Channel() { // go-redis reconnects this on its own
			h.broadcast([]byte(msg.Payload))
		}
	}()
	go func() { <-ctx.Done(); sub.Close() }()
	return nil
}

// Subscribe registers a client. The channel is closed if the client falls too far behind;
// call cancel when done.
func (h *Hub) Subscribe() (events <-chan []byte, cancel func()) {
	ch := make(chan []byte, clientBuffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.remove(ch) }
}

func (h *Hub) remove(ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
	}
}

// broadcast never blocks: a slow client is dropped instead of stalling everyone else.
func (h *Hub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			delete(h.clients, ch)
			close(ch)
		}
	}
}
