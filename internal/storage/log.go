package postgres

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

func (r *Repository) AddLogEntry(
	ctx context.Context,
	log models.LogEntry,
) error {

	query := `
		INSERT INTO logs (
			id,
			execution_id,
			target_id,
			step_id,
			timestamp,
			level,
			message
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		log.ID,
		log.ExecutionID,
		log.TargetID,
		log.StepID,
		log.Timestamp,
		log.Level,
		log.Message,
	)

	if err != nil {
		return fmt.Errorf("insert log: %w", err)
	}

	return nil
}

func (r *Repository) GetLogsByExecutionID(
	ctx context.Context,
	executionID string,
) ([]models.LogEntry, error) {

	query := `
		SELECT
			id,
			execution_id,
			target_id,
			step_id,
			timestamp,
			level,
			message
		FROM logs
		WHERE execution_id = $1
		ORDER BY timestamp
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		executionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.LogEntry

	for rows.Next() {

		var log models.LogEntry

		err := rows.Scan(
			&log.ID,
			&log.ExecutionID,
			&log.TargetID,
			&log.StepID,
			&log.Timestamp,
			&log.Level,
			&log.Message,
		)
		if err != nil {
			return nil, err
		}

		logs = append(logs, log)
	}

	return logs, rows.Err()
}
