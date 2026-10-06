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
	"orion-api/internal/queue"

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
	godotenv.Load("../.env") // load .env file if present

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

	queue, err := queue.NewStream(ctx, os.Getenv("REDIS_ADDR"), os.Getenv("STREAM_NAME"), os.Getenv("CONSUMER_GROUP"), 1000)
	if err != nil {
		log.Fatalf("failed to create queue: %v", err)
	}

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, pool, queue)

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