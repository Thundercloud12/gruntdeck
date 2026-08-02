package queue

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

type WorkerRunner struct {
	client *river.Client[pgx.Tx]
}

type jobWorker struct {
	river.WorkerDefaults[ExecuteJobArgs]
	service *execution.Service
}

func newJobWorker(service *execution.Service) *jobWorker {
	return &jobWorker{service: service}
}

func (w *jobWorker) Work(ctx context.Context, job *river.Job[ExecuteJobArgs]) error {
	return w.service.ExecuteTarget(ctx, job.Args.ExecutionID, job.Args.JobID, job.Args.TargetID)
}

func NewWorker(pool *pgxpool.Pool, execService *execution.Service) (*WorkerRunner, error) {
	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, newJobWorker(execService)); err != nil {
		return nil, fmt.Errorf("failed to register worker: %w", err)
	}

	client, err := river.NewClient(
		riverpgxv5.New(pool),
		&river.Config{
			Queues: map[string]river.QueueConfig{
				river.QueueDefault: {MaxWorkers: 100},
			},
			Workers: workers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create river worker client: %w", err)
	}

	return &WorkerRunner{client: client}, nil
}

func (w *WorkerRunner) Start(ctx context.Context) error {
	return w.client.Start(ctx)
}

func (w *WorkerRunner) Stop(ctx context.Context) error {
	return w.client.Stop(ctx)
}
