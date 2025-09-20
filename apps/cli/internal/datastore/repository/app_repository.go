package repository

import (
	"database/sql"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type AppRepository struct {
	db types.Database
}

func NewAppRepository(db types.Database) *AppRepository {
	return &AppRepository{db: db}
}

func (r *AppRepository) AddApp(appName string) error {
	query := `INSERT INTO installed_apps (app_name) VALUES (?)`
	err := r.db.Exec(query, appName)
	return err
}

func (r *AppRepository) GetApp(appName string) (*types.AppConfig, error) {
	query := `SELECT app_name FROM installed_apps WHERE app_name = ? LIMIT 1`
	row := r.db.QueryRow(query, appName)
	var name string
	err := row.Scan(&name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query app: %w", err)
	}

	// Return basic AppConfig with the name
	return &types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: name,
		},
	}, nil
}

func (r *AppRepository) RemoveApp(appName string) error {
	query := `DELETE FROM installed_apps WHERE app_name = ?`
	err := r.db.Exec(query, appName)
	return err
}

func (r *AppRepository) ListAllApps() ([]string, error) {
	log.Info("Retrieving all apps from the database")

	query := `SELECT app_name FROM installed_apps`
	rows, err := r.db.Query(query)
	if err != nil {
		log.Error("Failed to query apps", err)
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("Failed to close rows", err)
		}
	}(rows)

	var apps []string
	for rows.Next() {
		var appName string
		if err := rows.Scan(&appName); err != nil {
			log.Error("Failed to scan app name", err)
			return nil, err
		}
		apps = append(apps, appName)
	}

	// Check for any errors that occurred during iteration
	if err := rows.Err(); err != nil {
		log.Error("Error occurred during row iteration", err)
		return nil, err
	}

	log.Info("Apps retrieved successfully", "count", len(apps))
	return apps, nil
}
