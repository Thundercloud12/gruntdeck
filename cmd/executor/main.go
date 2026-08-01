package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/Thundercloud12/gruntdeck/internal/migrations"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: executor <job-id>\nExample: executor health-check")
	}
	jobID := os.Args[1]

	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatalf("DATABASE_URL environment variable is required. Please set it in your .env file or environment.")
	}

	ctx := context.Background()

	fmt.Println("🐘 Connecting to PostgreSQL database...")
	if err := migrations.RunMigrations(dbURL); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer pool.Close()

	jobRepo := repository.NewPostgresJobRepository(pool)
	invRepo := repository.NewPostgresInventoryRepository(pool)
	execRepo := repository.NewPostgresExecutionRepository(pool)
	logRepo := repository.NewPostgresLogRepository(pool)

	execService := execution.New(jobRepo, invRepo, execRepo, logRepo)

	executionID := uuid.New().String()
	if err := execService.ExecuteJob(ctx, executionID, jobID); err != nil {
		os.Exit(1)
	}
}
