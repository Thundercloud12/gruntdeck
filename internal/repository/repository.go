package repository

import (
	"context"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

type InventoryRepository interface {
	GetTargetByTags(ctx context.Context, tagi []string) ([]models.Target, error)
	GetTargetByID(ctx context.Context, id string) (*models.Target, error)
	ListTargets(ctx context.Context) ([]models.Target, error)

	AddTarget(ctx context.Context, target models.Target) error
	UpdateTarget(ctx context.Context, target models.Target) error
	DeleteTarget(ctx context.Context, id string) error
}

type JobRepository interface {
	GetJobByID(ctx context.Context, jobID string) (*models.Job, error)
	ListJobs(ctx context.Context) ([]models.Job, error)

	AddJob(ctx context.Context, job models.Job) error
	UpdateJob(ctx context.Context, job models.Job) error
	DeleteJob(ctx context.Context, jobID string) error
}

type ExecutionRepository interface {
	CreateExecution(ctx context.Context, execution models.Execution) error
	GetExecutionByID(ctx context.Context, id string) (*models.Execution, error)
	ListExecutions(ctx context.Context) ([]models.Execution, error)

	UpdateExecution(ctx context.Context, execution models.Execution) error
}

type LogRepository interface {
	AddLogEntry(ctx context.Context, log models.LogEntry) error
	GetLogsByExecutionID(ctx context.Context, executionID string) ([]models.LogEntry, error)
}

type ScheduleRepository interface {
	CreateSchedule(ctx context.Context, schedule models.Schedule) error
	GetScheduleByID(ctx context.Context, id string) (*models.Schedule, error)
	ListSchedules(ctx context.Context) ([]models.Schedule, error)

	UpdateSchedule(ctx context.Context, schedule models.Schedule) error
	DeleteSchedule(ctx context.Context, id string) error
}

type CredentialRepository interface {
	CreateCredential(ctx context.Context, cred models.Credential) error
	GetCredentialByID(ctx context.Context, id string) (*models.Credential, error)
	ListCredentials(ctx context.Context) ([]models.Credential, error)
	DeleteCredential(ctx context.Context, id string) error
}

type UserRepository interface {
	CreateUser(ctx context.Context, user models.User) error
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	ListUsers(ctx context.Context) ([]models.User, error)
}

type SessionRepository interface {
	CreateSession(ctx context.Context, session models.Session) error
	GetSessionByToken(ctx context.Context, token string) (*models.Session, error)
	DeleteSession(ctx context.Context, token string) error
}
