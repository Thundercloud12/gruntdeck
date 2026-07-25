package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

func (r *Repository) CreateExecution(
	ctx context.Context,
	exec models.Execution,
) error {

	query := `
		INSERT INTO executions (
			id,
			job_id,
			status,
			started_at,
			ended_at,
			targets_total,
			targets_succeeded,
			targets_failed
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		exec.ID,
		exec.JobID,
		exec.Status,
		exec.StartedAt,
		exec.EndedAt,
		exec.TargetsTotal,
		exec.TargetsSucceeded,
		exec.TargetsFailed,
	)

	if err != nil {
		return fmt.Errorf("create execution: %w", err)
	}

	return nil
}

func (r *Repository) GetExecutionByID(
	ctx context.Context,
	id string,
) (*models.Execution, error) {

	query := `
		SELECT
			id,
			job_id,
			status,
			started_at,
			ended_at,
			targets_total,
			targets_succeeded,
			targets_failed
		FROM executions
		WHERE id = $1
	`

	var exec models.Execution

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&exec.ID,
		&exec.JobID,
		&exec.Status,
		&exec.StartedAt,
		&exec.EndedAt,
		&exec.TargetsTotal,
		&exec.TargetsSucceeded,
		&exec.TargetsFailed,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &exec, nil
}

func (r *Repository) ListExecutions(
	ctx context.Context,
) ([]models.Execution, error) {

	query := `
		SELECT
			id,
			job_id,
			status,
			started_at,
			ended_at,
			targets_total,
			targets_succeeded,
			targets_failed
		FROM executions
		ORDER BY started_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.Execution

	for rows.Next() {

		var exec models.Execution

		err := rows.Scan(
			&exec.ID,
			&exec.JobID,
			&exec.Status,
			&exec.StartedAt,
			&exec.EndedAt,
			&exec.TargetsTotal,
			&exec.TargetsSucceeded,
			&exec.TargetsFailed,
		)
		if err != nil {
			return nil, err
		}

		executions = append(executions, exec)
	}

	return executions, rows.Err()
}

func (r *Repository) UpdateExecution(
	ctx context.Context,
	exec models.Execution,
) error {

	query := `
		UPDATE executions
		SET
			status = $2,
			ended_at = $3,
			targets_total = $4,
			targets_succeeded = $5,
			targets_failed = $6
		WHERE id = $1
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		exec.ID,
		exec.Status,
		exec.EndedAt,
		exec.TargetsTotal,
		exec.TargetsSucceeded,
		exec.TargetsFailed,
	)

	if err != nil {
		return fmt.Errorf("update execution: %w", err)
	}

	return nil
}
