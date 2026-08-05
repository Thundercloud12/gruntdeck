package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Thundercloud12/gruntdeck/internal/api"
	"github.com/Thundercloud12/gruntdeck/internal/migrations"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/Thundercloud12/gruntdeck/internal/scheduler"
	"github.com/Thundercloud12/gruntdeck/web"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pool.Close()

	if err := migrations.RunMigrations(dbURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	if err := queue.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("River migration failed: %v", err)
	}

	producer, err := queue.NewProducer(pool)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}

	jobRepo := repository.NewPostgresJobRepository(pool)
	execRepo := repository.NewPostgresExecutionRepository(pool)
	logRepo := repository.NewPostgresLogRepository(pool)
	invRepo := repository.NewPostgresInventoryRepository(pool)
	schedRepo := repository.NewPostgresScheduleRepository(pool)

	schedService := scheduler.NewService(schedRepo, producer)
	if err := schedService.Start(ctx); err != nil {
		log.Printf("⚠️ Failed to start scheduler service: %v", err)
	}
	defer schedService.Stop()

	apiRouter := api.NewRouter(producer, jobRepo, execRepo, logRepo, invRepo, schedRepo, schedService)

	mux := http.NewServeMux()
	mux.Handle("/api/", apiRouter)
	mux.Handle("/", http.FileServer(http.FS(web.Files)))

	fmt.Printf("🌐 Gruntdeck Web Dashboard & API Server running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("API Server stopped: %v", err)
	}
}
