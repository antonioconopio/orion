package handlers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"

	"orion-api/internal/events"
)

// WSHandler upgrades GET /ws to a WebSocket and streams hub events to the browser.
type WSHandler struct {
	Hub          *events.Hub
	OriginHost   string // host:port allowed to connect, e.g. localhost:3000
	pingInterval time.Duration
}

func NewWSHandler(hub *events.Hub, uiOrigin string) *WSHandler {
	u, _ := url.Parse(uiOrigin)
	return &WSHandler{Hub: hub, OriginHost: u.Host, pingInterval: 30 * time.Second}
}

// Serve godoc
// @Summary      Live events (WebSocket)
// @Description  Upgrades to a WebSocket and pushes small JSON hints such as {"type":"run","run_id":"...","dag_id":"..."} whenever a run or worker changes. Re-fetch the affected resource over REST.
// @Tags         events
// @Success      101  "Switching Protocols"
// @Router       /ws [get]
func (h *WSHandler) Serve(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{h.OriginHost}})
	if err != nil {
		return // Accept already wrote the HTTP error (e.g. 403 for a foreign origin)
	}
	defer conn.CloseNow()

	ch, cancel := h.Hub.Subscribe()
	defer cancel()

	// The browser never sends data; reading is how we notice it went away (and answer pings).
	ctx := conn.CloseRead(r.Context())
	ping := time.NewTicker(h.pingInterval)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok { // the hub dropped us for being too slow; the client reconnects and refetches
				conn.Close(websocket.StatusPolicyViolation, "too slow")
				return
			}
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Ping(pctx)
			pcancel()
			if err != nil {
				return
			}
		}
	}
}
