package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type Producer struct {
	client     *river.Client[pgx.Tx]
	pool       *pgxpool.Pool
	jobs       repository.JobRepository
	inventory  repository.InventoryRepository
	executions repository.ExecutionRepository
}

func NewProducer(pool *pgxpool.Pool) (*Producer, error) {
	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, newJobWorker(nil)); err != nil {
		return nil, fmt.Errorf("failed to register worker in producer: %w", err)
	}

	client, err := river.NewClient(
		riverpgxv5.New(pool),
		&river.Config{
			Workers: workers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create river producer client: %w", err)
	}

	return &Producer{
		client:     client,
		pool:       pool,
		jobs:       repository.NewPostgresJobRepository(pool),
		inventory:  repository.NewPostgresInventoryRepository(pool),
		executions: repository.NewPostgresExecutionRepository(pool),
	}, nil
}

func (p *Producer) EnqueueExecution(ctx context.Context, jobID string) (string, int, error) {
	return p.EnqueueExecutionWithOptions(ctx, jobID, nil)
}

func (p *Producer) EnqueueExecutionWithOptions(ctx context.Context, jobID string, userOptions map[string]string) (string, int, error) {
	if userOptions == nil {
		userOptions = make(map[string]string)
	}

	job, err := p.jobs.GetJobByID(ctx, jobID)
	if err != nil || job == nil {
		return "", 0, fmt.Errorf("job %q not found: %w", jobID, err)
	}

	// Validate & merge options
	mergedOptions := make(map[string]string)
	for _, opt := range job.Options {
		val, exists := userOptions[opt.Name]
		if !exists {
			// Fallback check case-insensitive match
			for k, v := range userOptions {
				if strings.EqualFold(k, opt.Name) {
					val = v
					exists = true
					break
				}
			}
		}

		if !exists || strings.TrimSpace(val) == "" {
			if opt.Required && opt.DefaultValue == "" {
				return "", 0, fmt.Errorf("missing required option %q for job %q", opt.Name, job.Name)
			}
			val = opt.DefaultValue
		}

		if len(opt.Choices) > 0 && val != "" {
			validChoice := false
			for _, choice := range opt.Choices {
				if choice == val {
					validChoice = true
					break
				}
			}
			if !validChoice {
				return "", 0, fmt.Errorf("invalid value %q for option %q; allowed choices: %v", val, opt.Name, opt.Choices)
			}
		}

		mergedOptions[opt.Name] = val
	}

	// Copy over any additional options provided by user that were not in schema
	for k, v := range userOptions {
		if _, exists := mergedOptions[k]; !exists {
			mergedOptions[k] = v
		}
	}

	targets, err := p.inventory.GetTargetByTags(ctx, job.TargetFilter)
	if err != nil {
		return "", 0, fmt.Errorf("failed to load targets for job %q: %w", jobID, err)
	}
	if len(targets) == 0 {
		return "", 0, fmt.Errorf("no targets matched filter %v for job %q", job.TargetFilter, job.Name)
	}

	executionID := uuid.New().String()
	err = p.executions.CreateExecution(ctx, models.Execution{
		ID:           executionID,
		JobID:        jobID,
		Status:       "queued",
		Options:      mergedOptions,
		StartedAt:    time.Now(),
		TargetsTotal: len(targets),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to create execution record: %w", err)
	}

	tx, err := p.pool.Begin(ctx)
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
				Options:     mergedOptions,
			},
		}
	}

	_, err = p.client.InsertManyTx(ctx, tx, insertParams)
	if err != nil {
		return "", 0, fmt.Errorf("failed to insert river jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return executionID, len(targets), nil
}
