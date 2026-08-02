package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type QueueService struct {
	client     *river.Client[pgx.Tx]
	pool       *pgxpool.Pool
	jobs       repository.JobRepository
	inventory  repository.InventoryRepository
	executions repository.ExecutionRepository
}

func NewQueueService(
	pool *pgxpool.Pool,
	jobs repository.JobRepository,
	inventory repository.InventoryRepository,
	executions repository.ExecutionRepository,
) (*QueueService, error) {
	workers := river.NewWorkers()
	client, err := river.NewClient(
		riverpgxv5.New(pool),
		&river.Config{
			Workers: workers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	return &QueueService{
		client:     client,
		pool:       pool,
		jobs:       jobs,
		inventory:  inventory,
		executions: executions,
	}, nil
}

// Enqueue loads the job & targets, creates an execution record, and inserts tasks into River queue atomically.
func (qs *QueueService) Enqueue(ctx context.Context, jobID string) (string, int, error) {
	job, err := qs.jobs.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		return "", 0, fmt.Errorf("job %q not found: %w", jobID, err)
	}

	targets, err := qs.inventory.GetTargetByTags(ctx, job.TargetFilter)
	if err != nil {
		return "", 0, fmt.Errorf("failed to load targets for job %q: %w", jobID, err)
	}
	if len(targets) == 0 {
		return "", 0, fmt.Errorf("no targets matched filter %v for job %q", job.TargetFilter, job.Name)
	}

	executionID := uuid.New().String()
	err = qs.executions.CreateExecution(ctx, models.Execution{
		ID:           executionID,
		JobID:        jobID,
		Status:       "queued",
		StartedAt:    time.Now(),
		TargetsTotal: len(targets),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to create execution record: %w", err)
	}

	tx, err := qs.pool.Begin(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	insertParams := make([]river.InsertManyParams, len(targets))
	for i, target := range targets {
		insertParams[i] = river.InsertManyParams{
			Args: ExecuteJobArgs{
				ExecutionID: executionID,
				JobID:       jobID,
				TargetID:    target.ID,
			},
		}
	}

	_, err = qs.client.InsertManyTx(ctx, tx, insertParams)
	if err != nil {
		return "", 0, fmt.Errorf("failed to insert river jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return executionID, len(targets), nil
}
