package repository

import (
	"database/sql"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/errors"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type repository struct {
	appRepo    *AppRepository
	systemRepo types.SystemRepository
	db         types.Database
}

// NewRepository initializes and returns a Repository instance
// Returns nil if schema initialization fails
func NewRepository(db types.Database) types.Repository {
	// Ensure schema is initialized
	err := datastore.InitializeSchema(db.Conn())
	if err != nil {
		log.Error("Failed to initialize database schema", err)
		return nil
	}

	return &repository{
		appRepo:    NewAppRepository(db),
		systemRepo: NewSystemRepository(db),
		db:         db,
	}
}

// DB returns the underlying database instance
func (r *repository) DB() types.Database {
	return r.db
}

// AppRepository Methods
func (r *repository) AddApp(appName string) error {
	log.Info("Adding app via repository", "appName", appName)
	if err := r.appRepo.AddApp(appName); err != nil {
		log.Error("Failed to add app", err, "appName", appName)
		return err
	}
	log.Info("App added successfully", "appName", appName)
	return nil
}

func (r *repository) GetApp(appName string) (*types.AppConfig, error) {
	log.Info("Retrieving app via repository", "appName", appName)
	app, err := r.appRepo.GetApp(appName)
	if err != nil {
		log.Error("Failed to retrieve app", err, "appName", appName)
		return nil, err
	}
	if app == nil {
		log.Warn("App not found", "appName", appName)
		return nil, nil
	}
	return app, nil
}

func (r *repository) RemoveApp(appName string) error {
	log.Info("Removing app via repository", "appName", appName)
	if err := r.appRepo.RemoveApp(appName); err != nil {
		log.Error("Failed to remove app", err, "appName", appName)
		return err
	}
	log.Info("App removed successfully", "appName", appName)
	return nil
}

func (r *repository) SaveApp(app types.AppConfig) error {
	log.Info("Saving app to repository", "appName", app.Name)
	if err := r.appRepo.AddApp(app.Name); err != nil {
		log.Error("Failed to save app", err, "appName", app.Name)
		return err
	}
	log.Info("App saved successfully", "appName", app.Name)
	return nil
}

func (r *repository) ListApps() ([]types.AppConfig, error) {
	log.Info("Listing all apps from repository")
	apps, err := r.appRepo.ListAllApps()
	if err != nil {
		log.Error("Failed to list apps", err)
		return nil, err
	}
	appConfigs := make([]types.AppConfig, len(apps))
	for i, appName := range apps {
		appConfigs[i] = types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name: appName,
			},
		}
	}
	log.Info("Apps listed successfully", "count", len(appConfigs))
	return appConfigs, nil
}

// SystemRepository Methods
func (r *repository) Get(key string) (string, error) {
	log.Info("Retrieving system configuration", "key", key)
	value, err := r.systemRepo.Get(key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("key '%s' not found", key)
	}
	if err != nil {
		log.Error("Failed to retrieve system configuration", err, "key", key)
		return "", err
	}
	log.Info("System configuration retrieved successfully", "key", key, "value", value)
	return value, nil
}

func (r *repository) Set(key, value string) error {
	log.Info("Setting system configuration", "key", key, "value", value)
	if err := r.systemRepo.Set(key, value); err != nil {
		log.Error("Failed to set system configuration", err, "key", key, "value", value)
		return err
	}
	log.Info("System configuration set successfully", "key", key, "value", value)
	return nil
}

func (r *repository) GetAll() (map[string]string, error) {
	log.Info("Retrieving all system configurations")
	configs, err := r.systemRepo.GetAll()
	if err != nil {
		log.Error("Failed to retrieve all system configurations", err)
		return nil, err
	}
	log.Info("System configurations retrieved successfully", "count", len(configs))
	return configs, nil
}

func (r *repository) DeleteApp(name string) error {
	log.Info("Deleting app from repository", "name", name)
	return r.appRepo.RemoveApp(name)
}
