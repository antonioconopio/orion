package handlers

import (
	"net/http"
	"orion-api/internal/events"
	"orion-api/internal/queue"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes wires every API endpoint onto mux. It lives here (rather
// than in main) so tests exercise the exact same routing table as prod.
func RegisterRoutes(mux *http.ServeMux, pool *pgxpool.Pool, queue *queue.Stream, hub *events.Hub, uiOrigin string) {
	dagHandler := NewDAGHandler(pool)
	taskHandler := NewTaskHandler(pool)
	runHandler := NewRunHandler(pool, queue)
	taskInstanceHandler := NewTaskInstanceHandler(pool)
	dependencyHandler := NewDependencyHandler(pool)
	statsHandler := NewStatsHandler(pool)
	workerHandler := NewWorkerHandler(pool, queue.Client())
	wsHandler := NewWSHandler(hub, uiOrigin)

	// Health endpoint
	mux.HandleFunc("/health", Health)

	// Dag endpoints
	mux.HandleFunc("GET /dags", dagHandler.List)
	mux.HandleFunc("POST /dags", dagHandler.Create)
	mux.HandleFunc("GET /dags/{id}", dagHandler.Get)
	mux.HandleFunc("PUT /dags/{id}", dagHandler.Update)
	mux.HandleFunc("DELETE /dags/{id}", dagHandler.Delete)

	// Task endpoints
	mux.HandleFunc("GET /dags/{dagId}/tasks", taskHandler.List)
	mux.HandleFunc("GET /tasks/{id}", taskHandler.Get)
	mux.HandleFunc("POST /dags/{dagId}/tasks", taskHandler.Create)
	mux.HandleFunc("PUT /tasks/{id}", taskHandler.Update)
	mux.HandleFunc("DELETE /tasks/{id}", taskHandler.Delete)

	// Dependency endpoints
	mux.HandleFunc("GET /dags/{dagId}/dependencies", dependencyHandler.List)
	mux.HandleFunc("POST /dags/{dagId}/dependencies", dependencyHandler.Create)
	mux.HandleFunc("DELETE /dags/{dagId}/dependencies/{taskId}/{dependsOnId}", dependencyHandler.Delete)

	// Run endpoints
	mux.HandleFunc("GET /dags/{dagId}/runs", runHandler.List)
	mux.HandleFunc("GET /runs", runHandler.ListAll)
	mux.HandleFunc("GET /runs/{id}", runHandler.Get)
	mux.HandleFunc("POST /dags/{dagId}/runs", runHandler.Trigger)

	// Workers and live events
	mux.HandleFunc("GET /workers", workerHandler.List)
	mux.HandleFunc("GET /ws", wsHandler.Serve)

	// Dashboard
	mux.HandleFunc("GET /stats", statsHandler.Get)

	// Task Instance endpoints
	mux.HandleFunc("GET /runs/{runId}/task-instances", taskInstanceHandler.List)
	mux.HandleFunc("GET /task-instances/{id}", taskInstanceHandler.Get)
}
