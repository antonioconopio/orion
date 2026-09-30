package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/cors"

	"orion-api/internal/db"
	"orion-api/internal/handlers"

	_ "orion-api/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// side-effect import registers the generated spec

// side-effect import registers the generated spec

// @title        Orion API
// @version      1.0
// @description  API for the Orion DAG orchestration platform
// @host         localhost:8080
// @BasePath     /
func main() {
	godotenv.Load(".env") // load .env file if present

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		log.Fatalf("failed to create DB pool: %v", err)
	}
	defer pool.Close()

	dagHandler := handlers.NewDAGHandler(pool)
	taskHandler := handlers.NewTaskHandler(pool)
	runHandler := handlers.NewRunHandler(pool)
	taskInstanceHandler := handlers.NewTaskInstanceHandler(pool)

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", handlers.Health)

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

	// Run endpoints
	
	mux.HandleFunc("GET /dags/{dagId}/runs", runHandler.List)
	mux.HandleFunc("GET /runs/{id}", runHandler.Get)
	mux.HandleFunc("POST /dags/{dagId}/runs", runHandler.Trigger)

	// Task Instance endpoints
	mux.HandleFunc("GET /dags/{runId}/task-instances", taskInstanceHandler.List)
	mux.HandleFunc("GET /task-instances/{id}", taskInstanceHandler.Get)

	// Swagger docs endpoint
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"}, // your Vite dev server, not "*"
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler(mux)

	log.Printf("orion-api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}