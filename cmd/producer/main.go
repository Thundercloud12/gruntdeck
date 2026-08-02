package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Thundercloud12/gruntdeck/internal/migrations"
	"github.com/Thundercloud12/gruntdeck/internal/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: producer <job-id>\nExample: producer deploy-app")
	}
	jobID := os.Args[1]

	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required.")
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

	executionID, targetCount, err := producer.EnqueueExecution(ctx, jobID)
	if err != nil {
		log.Fatalf("Enqueue failed: %v", err)
	}

	fmt.Printf("🚀 Successfully enqueued Job '%s' across %d target nodes.\nExecution ID: %s\n", jobID, targetCount, executionID)
}
