package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type systemRepository struct {
	db types.Database
}

func NewSystemRepository(db types.Database) types.SystemRepository {
	return &systemRepository{db: db}
}

func (r *systemRepository) Get(key string) (string, error) {
	log.Info("Retrieving system configuration", "key", key)

	query := "SELECT value FROM system_data WHERE key = ? LIMIT 1"
	ctx := context.Background()
	row := r.db.Conn().QueryRowContext(ctx, query, key)

	var value string
	err := row.Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("key '%s' not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to query system configuration: %w", err)
	}

	return value, nil
}

func (r *systemRepository) Set(key, value string) error {
	log.Info("Setting system configuration", "key", key, "value", value)

	query := `INSERT INTO system_data (key, value, updated_at) VALUES (?, ?, ?)
          ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	ctx := context.Background()
	_, err := r.db.Conn().ExecContext(ctx, query, key, value, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set system configuration for key '%s': %w", key, err)
	}

	log.Info("System configuration set successfully", "key", key, "value", value)
	return nil
}

func (r *systemRepository) GetAll() (map[string]string, error) {
	log.Info("Retrieving all system configurations")

	query := "SELECT key, value FROM system_data"
	ctx := context.Background()
	rows, err := r.db.Conn().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query system configurations: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("Failed to close rows", err)
		}
	}(rows)

	configurations := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		configurations[key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return configurations, nil
}
