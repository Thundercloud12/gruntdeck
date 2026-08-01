package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure implementation of repository interfaces at compile-time
var _ InventoryRepository = (*PostgresInventoryRepository)(nil)
var _ JobRepository = (*PostgresJobRepository)(nil)
var _ ExecutionRepository = (*PostgresExecutionRepository)(nil)
var _ LogRepository = (*PostgresLogRepository)(nil)

// ==========================================
// InventoryRepository Implementation
// ==========================================

type PostgresInventoryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresInventoryRepository(pool *pgxpool.Pool) *PostgresInventoryRepository {
	return &PostgresInventoryRepository{pool: pool}
}

func (r *PostgresInventoryRepository) GetTargetByTags(ctx context.Context, tags []string) ([]models.Target, error) {
	if len(tags) == 0 {
		return r.ListTargets(ctx)
	}

	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
		WHERE tags @> $1
	`
	rows, err := r.pool.Query(ctx, query, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to query targets by tags: %w", err)
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var t models.Target
		if err := rows.Scan(&t.ID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.Tags); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (r *PostgresInventoryRepository) GetTargetByID(ctx context.Context, id string) (*models.Target, error) {
	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var t models.Target
	if err := row.Scan(&t.ID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.Tags); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("target not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan target: %w", err)
	}

	return &t, nil
}

func (r *PostgresInventoryRepository) ListTargets(ctx context.Context) ([]models.Target, error) {
	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var t models.Target
		if err := rows.Scan(&t.ID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.Tags); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (r *PostgresInventoryRepository) AddTarget(ctx context.Context, target models.Target) error {
	query := `
		INSERT INTO targets (id, host, port, "user", key_path, tags)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, target.ID, target.Host, target.Port, target.User, target.KeyPath, target.Tags)
	if err != nil {
		return fmt.Errorf("failed to add target: %w", err)
	}
	return nil
}

func (r *PostgresInventoryRepository) UpdateTarget(ctx context.Context, target models.Target) error {
	query := `
		UPDATE targets
		SET host = $2, port = $3, "user" = $4, key_path = $5, tags = $6
		WHERE id = $1
	`
	tag, err := r.pool.Exec(ctx, query, target.ID, target.Host, target.Port, target.User, target.KeyPath, target.Tags)
	if err != nil {
		return fmt.Errorf("failed to update target: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("target not found: %s", target.ID)
	}
	return nil
}

func (r *PostgresInventoryRepository) DeleteTarget(ctx context.Context, id string) error {
	query := `DELETE FROM targets WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete target: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("target not found: %s", id)
	}
	return nil
}

// ==========================================
// JobRepository Implementation
// ==========================================

type PostgresJobRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresJobRepository(pool *pgxpool.Pool) *PostgresJobRepository {
	return &PostgresJobRepository{pool: pool}
}

func (r *PostgresJobRepository) GetJobByID(ctx context.Context, jobID string) (*models.Job, error) {
	query := `
		SELECT id, name, target_filter
		FROM jobs
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, jobID)

	var job models.Job
	if err := row.Scan(&job.ID, &job.Name, &job.TargetFilter); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	stepsQuery := `
		SELECT id, job_id, step_order, type, attributes
		FROM job_steps
		WHERE job_id = $1
		ORDER BY step_order ASC
	`
	rows, err := r.pool.Query(ctx, stepsQuery, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query job steps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var step models.JobStep
		if err := rows.Scan(&step.ID, &step.JobID, &step.StepOrder, &step.Type, &step.Attributes); err != nil {
			return nil, fmt.Errorf("failed to scan job step: %w", err)
		}
		job.Steps = append(job.Steps, step)
	}

	return &job, rows.Err()
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context) ([]models.Job, error) {
	query := `
		SELECT id, name, target_filter
		FROM jobs
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(&job.ID, &job.Name, &job.TargetFilter); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, job)
	}

	// Fetch steps for each job
	for i := range jobs {
		stepsQuery := `
			SELECT id, job_id, step_order, type, attributes
			FROM job_steps
			WHERE job_id = $1
			ORDER BY step_order ASC
		`
		stepRows, err := r.pool.Query(ctx, stepsQuery, jobs[i].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query steps for job %s: %w", jobs[i].ID, err)
		}

		for stepRows.Next() {
			var step models.JobStep
			if err := stepRows.Scan(&step.ID, &step.JobID, &step.StepOrder, &step.Type, &step.Attributes); err != nil {
				stepRows.Close()
				return nil, fmt.Errorf("failed to scan step for job %s: %w", jobs[i].ID, err)
			}
			jobs[i].Steps = append(jobs[i].Steps, step)
		}
		stepRows.Close()
	}

	return jobs, rows.Err()
}

func (r *PostgresJobRepository) AddJob(ctx context.Context, job models.Job) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	jobQuery := `
		INSERT INTO jobs (id, name, target_filter)
		VALUES ($1, $2, $3)
	`
	_, err = tx.Exec(ctx, jobQuery, job.ID, job.Name, job.TargetFilter)
	if err != nil {
		return fmt.Errorf("failed to insert job: %w", err)
	}

	stepQuery := `
		INSERT INTO job_steps (id, job_id, step_order, type, attributes)
		VALUES ($1, $2, $3, $4, $5)
	`
	for _, step := range job.Steps {
		_, err := tx.Exec(ctx, stepQuery, step.ID, job.ID, step.StepOrder, step.Type, step.Attributes)
		if err != nil {
			return fmt.Errorf("failed to insert job step: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit job insertion: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) UpdateJob(ctx context.Context, job models.Job) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	jobQuery := `
		UPDATE jobs
		SET name = $2, target_filter = $3
		WHERE id = $1
	`
	tag, err := tx.Exec(ctx, jobQuery, job.ID, job.Name, job.TargetFilter)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", job.ID)
	}

	// Delete existing steps and re-insert
	deleteStepsQuery := `DELETE FROM job_steps WHERE job_id = $1`
	if _, err := tx.Exec(ctx, deleteStepsQuery, job.ID); err != nil {
		return fmt.Errorf("failed to delete existing job steps: %w", err)
	}

	stepQuery := `
		INSERT INTO job_steps (id, job_id, step_order, type, attributes)
		VALUES ($1, $2, $3, $4, $5)
	`
	for _, step := range job.Steps {
		_, err := tx.Exec(ctx, stepQuery, step.ID, job.ID, step.StepOrder, step.Type, step.Attributes)
		if err != nil {
			return fmt.Errorf("failed to insert updated job step: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit job update: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) DeleteJob(ctx context.Context, jobID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	deleteStepsQuery := `DELETE FROM job_steps WHERE job_id = $1`
	if _, err := tx.Exec(ctx, deleteStepsQuery, jobID); err != nil {
		return fmt.Errorf("failed to delete job steps: %w", err)
	}

	deleteJobQuery := `DELETE FROM jobs WHERE id = $1`
	tag, err := tx.Exec(ctx, deleteJobQuery, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit job deletion: %w", err)
	}
	return nil
}

// ==========================================
// ExecutionRepository Implementation
// ==========================================

type PostgresExecutionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresExecutionRepository(pool *pgxpool.Pool) *PostgresExecutionRepository {
	return &PostgresExecutionRepository{pool: pool}
}

func (r *PostgresExecutionRepository) CreateExecution(ctx context.Context, execution models.Execution) error {
	query := `
		INSERT INTO executions (id, job_id, status, started_at, ended_at, targets_total, targets_succeeded, targets_failed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(
		ctx, query,
		execution.ID, execution.JobID, execution.Status,
		execution.StartedAt, execution.EndedAt,
		execution.TargetsTotal, execution.TargetsSucceeded, execution.TargetsFailed,
	)
	if err != nil {
		return fmt.Errorf("failed to create execution record: %w", err)
	}
	return nil
}

func (r *PostgresExecutionRepository) GetExecutionByID(ctx context.Context, id string) (*models.Execution, error) {
	query := `
		SELECT id, job_id, status, started_at, ended_at, targets_total, targets_succeeded, targets_failed
		FROM executions
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var exec models.Execution
	err := row.Scan(
		&exec.ID, &exec.JobID, &exec.Status,
		&exec.StartedAt, &exec.EndedAt,
		&exec.TargetsTotal, &exec.TargetsSucceeded, &exec.TargetsFailed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan execution: %w", err)
	}

	return &exec, nil
}

func (r *PostgresExecutionRepository) ListExecutions(ctx context.Context) ([]models.Execution, error) {
	query := `
		SELECT id, job_id, status, started_at, ended_at, targets_total, targets_succeeded, targets_failed
		FROM executions
		ORDER BY started_at DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var executions []models.Execution
	for rows.Next() {
		var exec models.Execution
		err := rows.Scan(
			&exec.ID, &exec.JobID, &exec.Status,
			&exec.StartedAt, &exec.EndedAt,
			&exec.TargetsTotal, &exec.TargetsSucceeded, &exec.TargetsFailed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution row: %w", err)
		}
		executions = append(executions, exec)
	}

	return executions, rows.Err()
}

func (r *PostgresExecutionRepository) UpdateExecution(ctx context.Context, execution models.Execution) error {
	query := `
		UPDATE executions
		SET status = $2, ended_at = $3, targets_total = $4, targets_succeeded = $5, targets_failed = $6
		WHERE id = $1
	`
	tag, err := r.pool.Exec(
		ctx, query,
		execution.ID, execution.Status, execution.EndedAt,
		execution.TargetsTotal, execution.TargetsSucceeded, execution.TargetsFailed,
	)
	if err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("execution not found: %s", execution.ID)
	}
	return nil
}

// ==========================================
// LogRepository Implementation
// ==========================================

type PostgresLogRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLogRepository(pool *pgxpool.Pool) *PostgresLogRepository {
	return &PostgresLogRepository{pool: pool}
}

func (r *PostgresLogRepository) AddLogEntry(ctx context.Context, log models.LogEntry) error {
	query := `
		INSERT INTO log_entries (id, execution_id, target_id, step_id, timestamp, level, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(
		ctx, query,
		log.ID, log.ExecutionID, log.TargetID, log.StepID,
		log.Timestamp, log.Level, log.Message,
	)
	if err != nil {
		return fmt.Errorf("failed to add log entry: %w", err)
	}
	return nil
}

func (r *PostgresLogRepository) GetLogsByExecutionID(ctx context.Context, executionID string) ([]models.LogEntry, error) {
	query := `
		SELECT id, execution_id, target_id, step_id, timestamp, level, message
		FROM log_entries
		WHERE execution_id = $1
		ORDER BY timestamp ASC
	`
	rows, err := r.pool.Query(ctx, query, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query log entries: %w", err)
	}
	defer rows.Close()

	var logs []models.LogEntry
	for rows.Next() {
		var l models.LogEntry
		err := rows.Scan(
			&l.ID, &l.ExecutionID, &l.TargetID, &l.StepID,
			&l.Timestamp, &l.Level, &l.Message,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}
