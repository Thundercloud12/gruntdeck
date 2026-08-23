package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ensure implementation of repository interfaces at compile-time
var _ ProjectRepository = (*PostgresProjectRepository)(nil)
var _ InventoryRepository = (*PostgresInventoryRepository)(nil)
var _ JobRepository = (*PostgresJobRepository)(nil)
var _ ExecutionRepository = (*PostgresExecutionRepository)(nil)
var _ LogRepository = (*PostgresLogRepository)(nil)
var _ ScheduleRepository = (*PostgresScheduleRepository)(nil)
var _ CredentialRepository = (*PostgresCredentialRepository)(nil)
var _ UserRepository = (*PostgresUserRepository)(nil)
var _ SessionRepository = (*PostgresSessionRepository)(nil)

// ==========================================
// ProjectRepository Implementation
// ==========================================

type PostgresProjectRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresProjectRepository(pool *pgxpool.Pool) *PostgresProjectRepository {
	return &PostgresProjectRepository{pool: pool}
}

func (r *PostgresProjectRepository) CreateProject(ctx context.Context, project models.Project) error {
	query := `
		INSERT INTO projects (id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, project.ID, project.Name, project.Description)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

func (r *PostgresProjectRepository) GetProjectByID(ctx context.Context, id string) (*models.Project, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), created_at, updated_at
		FROM projects
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var p models.Project
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan project: %w", err)
	}

	return &p, nil
}

func (r *PostgresProjectRepository) ListProjects(ctx context.Context) ([]models.Project, error) {
	query := `
		SELECT id, name, COALESCE(description, ''), created_at, updated_at
		FROM projects
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan project row: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *PostgresProjectRepository) DeleteProject(ctx context.Context, id string) error {
	if id == "default" {
		return fmt.Errorf("cannot delete default project")
	}
	query := `DELETE FROM projects WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

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
		return r.ListTargets(ctx, "")
	}

	query := `
		SELECT id, COALESCE(project_id, 'default'), host, port, "user", key_path, COALESCE(credential_id, ''), tags
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
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.CredentialID, &t.Tags); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (r *PostgresInventoryRepository) GetTargetByID(ctx context.Context, id string) (*models.Target, error) {
	query := `
		SELECT id, COALESCE(project_id, 'default'), host, port, "user", key_path, COALESCE(credential_id, ''), tags
		FROM targets
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var t models.Target
	if err := row.Scan(&t.ID, &t.ProjectID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.CredentialID, &t.Tags); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("target not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan target: %w", err)
	}

	return &t, nil
}

func (r *PostgresInventoryRepository) ListTargets(ctx context.Context, projectID string) ([]models.Target, error) {
	var query string
	var args []any
	if projectID != "" {
		query = `
			SELECT id, COALESCE(project_id, 'default'), host, port, "user", key_path, COALESCE(credential_id, ''), tags
			FROM targets
			WHERE project_id = $1
		`
		args = append(args, projectID)
	} else {
		query = `
			SELECT id, COALESCE(project_id, 'default'), host, port, "user", key_path, COALESCE(credential_id, ''), tags
			FROM targets
		`
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var t models.Target
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Host, &t.Port, &t.User, &t.KeyPath, &t.CredentialID, &t.Tags); err != nil {
			return nil, fmt.Errorf("failed to scan target row: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func (r *PostgresInventoryRepository) AddTarget(ctx context.Context, target models.Target) error {
	if target.ProjectID == "" {
		target.ProjectID = "default"
	}
	query := `
		INSERT INTO targets (id, project_id, host, port, "user", key_path, credential_id, tags)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8)
	`
	_, err := r.pool.Exec(ctx, query, target.ID, target.ProjectID, target.Host, target.Port, target.User, target.KeyPath, target.CredentialID, target.Tags)
	if err != nil {
		return fmt.Errorf("failed to add target: %w", err)
	}
	return nil
}

func (r *PostgresInventoryRepository) UpdateTarget(ctx context.Context, target models.Target) error {
	if target.ProjectID == "" {
		target.ProjectID = "default"
	}
	query := `
		UPDATE targets
		SET project_id = $2, host = $3, port = $4, "user" = $5, key_path = $6, credential_id = NULLIF($7, ''), tags = $8
		WHERE id = $1
	`
	tag, err := r.pool.Exec(ctx, query, target.ID, target.ProjectID, target.Host, target.Port, target.User, target.KeyPath, target.CredentialID, target.Tags)
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
		SELECT id, COALESCE(project_id, 'default'), name, target_filter
		FROM jobs
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, jobID)

	var job models.Job
	if err := row.Scan(&job.ID, &job.ProjectID, &job.Name, &job.TargetFilter); err != nil {
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

	optionsQuery := `
		SELECT id, job_id, name, description, type, required, default_value, choices
		FROM job_options
		WHERE job_id = $1
	`
	optRows, err := r.pool.Query(ctx, optionsQuery, jobID)
	if err == nil {
		defer optRows.Close()
		for optRows.Next() {
			var opt models.JobOption
			if err := optRows.Scan(&opt.ID, &opt.JobID, &opt.Name, &opt.Description, &opt.Type, &opt.Required, &opt.DefaultValue, &opt.Choices); err == nil {
				job.Options = append(job.Options, opt)
			}
		}
	}

	return &job, rows.Err()
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context, projectID string) ([]models.Job, error) {
	var query string
	var args []any
	if projectID != "" {
		query = `
			SELECT id, COALESCE(project_id, 'default'), name, target_filter
			FROM jobs
			WHERE project_id = $1
		`
		args = append(args, projectID)
	} else {
		query = `
			SELECT id, COALESCE(project_id, 'default'), name, target_filter
			FROM jobs
		`
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		if err := rows.Scan(&job.ID, &job.ProjectID, &job.Name, &job.TargetFilter); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}
		jobs = append(jobs, job)
	}

	for i := range jobs {
		stepsQuery := `
			SELECT id, job_id, step_order, type, attributes
			FROM job_steps
			WHERE job_id = $1
			ORDER BY step_order ASC
		`
		stepRows, err := r.pool.Query(ctx, stepsQuery, jobs[i].ID)
		if err == nil {
			for stepRows.Next() {
				var step models.JobStep
				if err := stepRows.Scan(&step.ID, &step.JobID, &step.StepOrder, &step.Type, &step.Attributes); err == nil {
					jobs[i].Steps = append(jobs[i].Steps, step)
				}
			}
			stepRows.Close()
		}

		optionsQuery := `
			SELECT id, job_id, name, description, type, required, default_value, choices
			FROM job_options
			WHERE job_id = $1
		`
		optRows, err := r.pool.Query(ctx, optionsQuery, jobs[i].ID)
		if err == nil {
			for optRows.Next() {
				var opt models.JobOption
				if err := optRows.Scan(&opt.ID, &opt.JobID, &opt.Name, &opt.Description, &opt.Type, &opt.Required, &opt.DefaultValue, &opt.Choices); err == nil {
					jobs[i].Options = append(jobs[i].Options, opt)
				}
			}
			optRows.Close()
		}
	}

	return jobs, rows.Err()
}

func (r *PostgresJobRepository) AddJob(ctx context.Context, job models.Job) error {
	if job.ProjectID == "" {
		job.ProjectID = "default"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	jobQuery := `
		INSERT INTO jobs (id, project_id, name, target_filter)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, jobQuery, job.ID, job.ProjectID, job.Name, job.TargetFilter)
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

	optQuery := `
		INSERT INTO job_options (id, job_id, name, description, type, required, default_value, choices)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, opt := range job.Options {
		_, err := tx.Exec(ctx, optQuery, opt.ID, job.ID, opt.Name, opt.Description, opt.Type, opt.Required, opt.DefaultValue, opt.Choices)
		if err != nil {
			return fmt.Errorf("failed to insert job option: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit job insertion: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) UpdateJob(ctx context.Context, job models.Job) error {
	if job.ProjectID == "" {
		job.ProjectID = "default"
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	jobQuery := `
		UPDATE jobs
		SET project_id = $2, name = $3, target_filter = $4
		WHERE id = $1
	`
	tag, err := tx.Exec(ctx, jobQuery, job.ID, job.ProjectID, job.Name, job.TargetFilter)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("job not found: %s", job.ID)
	}

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

	deleteOptsQuery := `DELETE FROM job_options WHERE job_id = $1`
	if _, err := tx.Exec(ctx, deleteOptsQuery, job.ID); err != nil {
		return fmt.Errorf("failed to delete existing job options: %w", err)
	}

	optQuery := `
		INSERT INTO job_options (id, job_id, name, description, type, required, default_value, choices)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	for _, opt := range job.Options {
		_, err := tx.Exec(ctx, optQuery, opt.ID, job.ID, opt.Name, opt.Description, opt.Type, opt.Required, opt.DefaultValue, opt.Choices)
		if err != nil {
			return fmt.Errorf("failed to insert updated job option: %w", err)
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
	if execution.ProjectID == "" {
		execution.ProjectID = "default"
	}
	optionsJSON, err := json.Marshal(execution.Options)
	if err != nil {
		optionsJSON = []byte("{}")
	}

	query := `
		INSERT INTO executions (id, project_id, job_id, status, options, started_at, ended_at, targets_total, targets_succeeded, targets_failed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.pool.Exec(
		ctx, query,
		execution.ID, execution.ProjectID, execution.JobID, execution.Status, optionsJSON,
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
		SELECT id, COALESCE(project_id, 'default'), job_id, status, options, started_at, ended_at, targets_total, targets_succeeded, targets_failed
		FROM executions
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var exec models.Execution
	var optionsRaw []byte

	err := row.Scan(
		&exec.ID, &exec.ProjectID, &exec.JobID, &exec.Status, &optionsRaw,
		&exec.StartedAt, &exec.EndedAt,
		&exec.TargetsTotal, &exec.TargetsSucceeded, &exec.TargetsFailed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan execution: %w", err)
	}

	if len(optionsRaw) > 0 {
		_ = json.Unmarshal(optionsRaw, &exec.Options)
	}
	if exec.Options == nil {
		exec.Options = make(map[string]string)
	}

	return &exec, nil
}

func (r *PostgresExecutionRepository) ListExecutions(ctx context.Context, projectID string) ([]models.Execution, error) {
	var query string
	var args []any
	if projectID != "" {
		query = `
			SELECT id, COALESCE(project_id, 'default'), job_id, status, options, started_at, ended_at, targets_total, targets_succeeded, targets_failed
			FROM executions
			WHERE project_id = $1
			ORDER BY started_at DESC
		`
		args = append(args, projectID)
	} else {
		query = `
			SELECT id, COALESCE(project_id, 'default'), job_id, status, options, started_at, ended_at, targets_total, targets_succeeded, targets_failed
			FROM executions
			ORDER BY started_at DESC
		`
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list executions: %w", err)
	}
	defer rows.Close()

	var executions []models.Execution
	for rows.Next() {
		var exec models.Execution
		var optionsRaw []byte

		err := rows.Scan(
			&exec.ID, &exec.ProjectID, &exec.JobID, &exec.Status, &optionsRaw,
			&exec.StartedAt, &exec.EndedAt,
			&exec.TargetsTotal, &exec.TargetsSucceeded, &exec.TargetsFailed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution row: %w", err)
		}
		if len(optionsRaw) > 0 {
			_ = json.Unmarshal(optionsRaw, &exec.Options)
		}
		if exec.Options == nil {
			exec.Options = make(map[string]string)
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

// ==========================================
// ScheduleRepository Implementation
// ==========================================

type PostgresScheduleRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresScheduleRepository(pool *pgxpool.Pool) *PostgresScheduleRepository {
	return &PostgresScheduleRepository{pool: pool}
}

func (r *PostgresScheduleRepository) CreateSchedule(ctx context.Context, schedule models.Schedule) error {
	if schedule.ProjectID == "" {
		schedule.ProjectID = "default"
	}
	optionsJSON, err := json.Marshal(schedule.Options)
	if err != nil {
		optionsJSON = []byte("{}")
	}

	query := `
		INSERT INTO schedules (id, project_id, job_id, cron_expression, timezone, enabled, options, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err = r.pool.Exec(
		ctx, query,
		schedule.ID, schedule.ProjectID, schedule.JobID, schedule.CronExpression,
		schedule.Timezone, schedule.Enabled, optionsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to create schedule: %w", err)
	}
	return nil
}

func (r *PostgresScheduleRepository) GetScheduleByID(ctx context.Context, id string) (*models.Schedule, error) {
	query := `
		SELECT id, COALESCE(project_id, 'default'), job_id, cron_expression, timezone, enabled, options, created_at, updated_at
		FROM schedules
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var s models.Schedule
	var optionsRaw []byte

	err := row.Scan(
		&s.ID, &s.ProjectID, &s.JobID, &s.CronExpression,
		&s.Timezone, &s.Enabled, &optionsRaw,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("schedule not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan schedule: %w", err)
	}

	if len(optionsRaw) > 0 {
		_ = json.Unmarshal(optionsRaw, &s.Options)
	}
	if s.Options == nil {
		s.Options = make(map[string]string)
	}

	return &s, nil
}

func (r *PostgresScheduleRepository) ListSchedules(ctx context.Context, projectID string) ([]models.Schedule, error) {
	var query string
	var args []any
	if projectID != "" {
		query = `
			SELECT id, COALESCE(project_id, 'default'), job_id, cron_expression, timezone, enabled, options, created_at, updated_at
			FROM schedules
			WHERE project_id = $1
			ORDER BY created_at DESC
		`
		args = append(args, projectID)
	} else {
		query = `
			SELECT id, COALESCE(project_id, 'default'), job_id, cron_expression, timezone, enabled, options, created_at, updated_at
			FROM schedules
			ORDER BY created_at DESC
		`
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var optionsRaw []byte

		err := rows.Scan(
			&s.ID, &s.ProjectID, &s.JobID, &s.CronExpression,
			&s.Timezone, &s.Enabled, &optionsRaw,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule row: %w", err)
		}
		if len(optionsRaw) > 0 {
			_ = json.Unmarshal(optionsRaw, &s.Options)
		}
		if s.Options == nil {
			s.Options = make(map[string]string)
		}
		schedules = append(schedules, s)
	}

	return schedules, rows.Err()
}

func (r *PostgresScheduleRepository) UpdateSchedule(ctx context.Context, schedule models.Schedule) error {
	if schedule.ProjectID == "" {
		schedule.ProjectID = "default"
	}
	optionsJSON, err := json.Marshal(schedule.Options)
	if err != nil {
		optionsJSON = []byte("{}")
	}

	query := `
		UPDATE schedules
		SET project_id = $2, job_id = $3, cron_expression = $4, timezone = $5, enabled = $6, options = $7, updated_at = NOW()
		WHERE id = $1
	`
	tag, err := r.pool.Exec(
		ctx, query,
		schedule.ID, schedule.ProjectID, schedule.JobID, schedule.CronExpression,
		schedule.Timezone, schedule.Enabled, optionsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to update schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schedule not found: %s", schedule.ID)
	}
	return nil
}

func (r *PostgresScheduleRepository) DeleteSchedule(ctx context.Context, id string) error {
	query := `DELETE FROM schedules WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("schedule not found: %s", id)
	}
	return nil
}

// ==========================================
// CredentialRepository Implementation
// ==========================================

type PostgresCredentialRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresCredentialRepository(pool *pgxpool.Pool) *PostgresCredentialRepository {
	return &PostgresCredentialRepository{pool: pool}
}

func (r *PostgresCredentialRepository) CreateCredential(ctx context.Context, cred models.Credential) error {
	query := `
		INSERT INTO credentials (id, name, type, encrypted_data, nonce, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, cred.ID, cred.Name, cred.Type, cred.EncryptedData, cred.Nonce)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}
	return nil
}

func (r *PostgresCredentialRepository) GetCredentialByID(ctx context.Context, id string) (*models.Credential, error) {
	query := `
		SELECT id, name, type, encrypted_data, nonce, created_at, updated_at
		FROM credentials
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var c models.Credential
	if err := row.Scan(&c.ID, &c.Name, &c.Type, &c.EncryptedData, &c.Nonce, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("credential not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan credential: %w", err)
	}

	return &c, nil
}

func (r *PostgresCredentialRepository) ListCredentials(ctx context.Context) ([]models.Credential, error) {
	query := `
		SELECT id, name, type, created_at, updated_at
		FROM credentials
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}
	defer rows.Close()

	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan credential row: %w", err)
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (r *PostgresCredentialRepository) UpdateCredential(ctx context.Context, cred models.Credential) error {
	query := `
		UPDATE credentials
		SET name = $2, type = $3, encrypted_data = $4, nonce = $5, updated_at = NOW()
		WHERE id = $1
	`
	tag, err := r.pool.Exec(ctx, query, cred.ID, cred.Name, cred.Type, cred.EncryptedData, cred.Nonce)
	if err != nil {
		return fmt.Errorf("failed to update credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("credential not found: %s", cred.ID)
	}
	return nil
}

func (r *PostgresCredentialRepository) DeleteCredential(ctx context.Context, id string) error {
	query := `DELETE FROM credentials WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete credential: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("credential not found: %s", id)
	}
	return nil
}

// ==========================================
// UserRepository Implementation
// ==========================================

type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) CreateUser(ctx context.Context, u models.User) error {
	query := `
		INSERT INTO users (id, username, password_hash, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`
	_, err := r.pool.Exec(ctx, query, u.ID, u.Username, u.PasswordHash, u.Role)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		WHERE username = $1
	`
	row := r.pool.QueryRow(ctx, query, username)
	var u models.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %s", username)
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	var u models.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	return &u, nil
}

func (r *PostgresUserRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	query := `
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ==========================================
// SessionRepository Implementation
// ==========================================

type PostgresSessionRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresSessionRepository(pool *pgxpool.Pool) *PostgresSessionRepository {
	return &PostgresSessionRepository{pool: pool}
}

func (r *PostgresSessionRepository) CreateSession(ctx context.Context, s models.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.pool.Exec(ctx, query, s.ID, s.UserID, s.Token, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *PostgresSessionRepository) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > NOW()
	`
	row := r.pool.QueryRow(ctx, query, token)
	var s models.Session
	if err := row.Scan(&s.ID, &s.UserID, &s.Token, &s.ExpiresAt, &s.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}
	return &s, nil
}

func (r *PostgresSessionRepository) DeleteSession(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}
