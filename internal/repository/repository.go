package repository

import (
	"context"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

type InventoryRepository interface {
	GetTargetByTags(ctx context.Context, tagi []string) ([]models.Target, error)
	AddTarget(ctx context.Context, target models.Target) error
}

type JobRepository interface {
	GetJobByID(ctx context.Context, jobID string) (*models.Job, error)
}
