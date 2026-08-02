package queue

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(db), nil)
	if err != nil {
		return fmt.Errorf("failed to create river migrator: %w", err)
	}
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{})
	if err != nil {
		return fmt.Errorf("failed to run river migrations: %w", err)
	}
	return nil
}

func NewWorkerClient(
	db *pgxpool.Pool,
	execService *execution.Service,
) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	if err := river.AddWorkerSafely(workers, NewWorker(execService)); err != nil {
		return nil, fmt.Errorf("failed to register worker: %w", err)
	}

	client, err := river.NewClient(
		riverpgxv5.New(db),
		&river.Config{
			Workers: workers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker client: %w", err)
	}

	return client, nil
}
