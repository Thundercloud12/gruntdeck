package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/Thundercloud12/gruntdeck/internal/migrations"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required. Please set it in your .env file or environment.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("🐘 Connecting to PostgreSQL database...")
	if err := migrations.RunMigrations(dbURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pool.Close()

	if err := queue.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("River migration failed: %v", err)
	}

	jobRepo := repository.NewPostgresJobRepository(pool)
	invRepo := repository.NewPostgresInventoryRepository(pool)
	execRepo := repository.NewPostgresExecutionRepository(pool)
	logRepo := repository.NewPostgresLogRepository(pool)

	execService := execution.New(jobRepo, invRepo, execRepo, logRepo)

	workerClient, err := queue.NewWorkerClient(pool, execService)
	if err != nil {
		log.Fatalf("Failed to create worker client: %v", err)
	}

	fmt.Println("🚀 River Queue Worker started. Waiting for execution jobs...")
	if err := workerClient.Start(ctx); err != nil {
		log.Fatalf("Failed to start worker client: %v", err)
	}

	<-ctx.Done()
	fmt.Println("🛑 Shutting down River Queue Worker gracefully...")
	if err := workerClient.Stop(context.Background()); err != nil {
		log.Printf("Error stopping worker client: %v", err)
	}
	fmt.Println("Bye!")
}
