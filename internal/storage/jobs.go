package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/lib/pq"
)

func (r *Repository) GetJobByID(
	ctx context.Context,
	jobID string,
) (*models.Job, error) {

	query := `
		SELECT id, name, target_filter
		FROM jobs
		WHERE id = $1
	`

	var job models.Job

	err := r.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.ID,
		&job.Name,
		pq.Array(&job.TargetFilter),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get job: %w", err)
	}

	stepQuery := `
		SELECT
			id,
			job_id,
			step_order,
			type,
			attributes
		FROM job_steps
		WHERE job_id = $1
		ORDER BY step_order
	`

	rows, err := r.db.QueryContext(ctx, stepQuery, jobID)
	if err != nil {
		return nil, fmt.Errorf("get job steps: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			step      models.JobStep
			attrsJSON []byte
		)

		err := rows.Scan(
			&step.ID,
			&step.JobID,
			&step.StepOrder,
			&step.Type,
			&attrsJSON,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(attrsJSON, &step.Atrributes); err != nil {
			return nil, err
		}

		job.Steps = append(job.Steps, step)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *Repository) AddJob(
	ctx context.Context,
	job models.Job,
) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	jobQuery := `
		INSERT INTO jobs
		(id, name, target_filter)
		VALUES ($1, $2, $3)
	`

	_, err = tx.ExecContext(
		ctx,
		jobQuery,
		job.ID,
		job.Name,
		pq.Array(job.TargetFilter),
	)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	stepQuery := `
		INSERT INTO job_steps
		(id, job_id, step_order, type, attributes)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, step := range job.Steps {

		attrsJSON, err := json.Marshal(step.Atrributes)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(
			ctx,
			stepQuery,
			step.ID,
			job.ID,
			step.StepOrder,
			step.Type,
			attrsJSON,
		)
		if err != nil {
			return fmt.Errorf("insert step: %w", err)
		}
	}

	return tx.Commit()

}

func (r *Repository) ListJobs(ctx context.Context) ([]models.Job, error) {
	query := `
		SELECT id
		FROM Job
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("get targets by tags: %w", err)
	}
	var jobs []models.Job

	for rows.Next() {
		var id string

		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		job, err := r.GetJobByID(ctx, id)

		if err != nil {

			return nil, err
		}

		jobs = append(jobs, *job)
	}
	return jobs, rows.Err()
}

func (r *Repository) UpdateJob(ctx context.Context, job models.Job) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	updateJob := `
		UPDATE jobs
		SET
			name = $2,
			target_filter = $3
		WHERE id = $1
	`

	_, err = tx.ExecContext(
		ctx,
		updateJob,
		job.ID,
		job.Name,
		pq.Array(job.TargetFilter),
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`DELETE FROM job_steps WHERE job_id = $1`,
		job.ID,
	)
	if err != nil {
		return err
	}

	insertStep := `
		INSERT INTO job_steps
		(id, job_id, step_order, type, attributes)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, step := range job.Steps {

		attrsJSON, err := json.Marshal(step.Atrributes)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(
			ctx,
			insertStep,
			step.ID,
			job.ID,
			step.StepOrder,
			step.Type,
			attrsJSON,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) DeleteJob(
	ctx context.Context,
	jobID string,
) error {

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.ExecContext(
		ctx,
		`DELETE FROM job_steps WHERE job_id = $1`,
		jobID,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`DELETE FROM jobs WHERE id = $1`,
		jobID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
