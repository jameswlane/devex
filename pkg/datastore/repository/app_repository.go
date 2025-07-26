package repository

import (
	"database/sql"
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
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

func (r *AppRepository) GetApp(appName string) (bool, error) {
	query := `SELECT 1 FROM installed_apps WHERE app_name = ? LIMIT 1`
	row := r.db.QueryRow(query, appName)
	var exists int
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to query app: %w", err)
	}
	return true, nil
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
	defer rows.Close()

	var apps []string
	for rows.Next() {
		var appName string
		if err := rows.Scan(&appName); err != nil {
			log.Error("Failed to scan app name", err)
			return nil, err
		}
		apps = append(apps, appName)
	}

	log.Info("Apps retrieved successfully", "count", len(apps))
	return apps, nil
}
