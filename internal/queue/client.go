package queue

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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
