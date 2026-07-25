package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

func (r *Repository) GetTargetByID(
	ctx context.Context,
	id string,
) (*models.Target, error) {

	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
		WHERE id = $1
	`

	var target models.Target

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&target.ID,
		&target.Host,
		&target.Port,
		&target.User,
		&target.KeyPath,
		pq.Array(&target.Tags),
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get target by id: %w", err)
	}

	return &target, nil
}

func (r *Repository) GetTargetByTags(
	ctx context.Context, tagi []string,
) ([]models.Target, error) {

	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
		WHERE tags && $1
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
		pq.Array(tagi),
	)
	if err != nil {
		return nil, fmt.Errorf("get targets by tags: %w", err)
	}
	defer rows.Close()

	var targets []models.Target

	for rows.Next() {
		var target models.Target

		err := rows.Scan(
			&target.ID,
			&target.Host,
			&target.Port,
			&target.User,
			&target.KeyPath,
			pq.Array(&target.Tags),
		)
		if err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}

		targets = append(targets, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate targets: %w", err)
	}

	return targets, nil
}

func (r *Repository) ListTarget(
	ctx context.Context,
) ([]models.Target, error) {

	query := `
		SELECT id, host, port, "user", key_path, tags
		FROM targets
	`

	rows, err := r.db.QueryContext(
		ctx,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf("get targets by tags: %w", err)
	}
	defer rows.Close()

	var targets []models.Target

	for rows.Next() {
		var target models.Target

		err := rows.Scan(
			&target.ID,
			&target.Host,
			&target.Port,
			&target.User,
			&target.KeyPath,
			pq.Array(&target.Tags),
		)
		if err != nil {
			return nil, fmt.Errorf("scan target: %w", err)
		}

		targets = append(targets, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate targets: %w", err)
	}

	return targets, nil
}

func (r *Repository) AddTarget(
	ctx context.Context, target models.Target,
) error {

	query := `
		INSERT INTO targets
		(id, host, port, "user", key_path, tags)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		target.ID,
		target.Host,
		target.Port,
		target.User,
		target.KeyPath,
		pq.Array(target.Tags),
	)
	if err != nil {
		return fmt.Errorf("get targets by tags: %w", err)
	}
	if err != nil {
		return fmt.Errorf("add target: %w", err)
	}

	return nil
}

func (r *Repository) UpdateTarget(
	ctx context.Context,
	target models.Target,
) error {

	query := `
		UPDATE targets
		SET
			host = $2,
			port = $3,
			"user" = $4,
			key_path = $5,
			tags = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		target.ID,
		target.Host,
		target.Port,
		target.User,
		target.KeyPath,
		pq.Array(target.Tags),
	)

	if err != nil {
		return fmt.Errorf("update target: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) DeleteTarget(
	ctx context.Context,
	id string,
) error {

	query := `
		DELETE FROM targets
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete target: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
