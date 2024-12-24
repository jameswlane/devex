package repository

import (
	"fmt"
	"sync"

	"github.com/jameswlane/devex/pkg/db"
)

type AppRepository interface {
	AddApp(appName string) error
	GetApp(appName string) (bool, error)
	RemoveApp(appName string) error
}

type appRepository struct {
	db    *db.DB
	cache map[string]bool
	mu    *sync.RWMutex
}

func NewAppRepository(db *db.DB) AppRepository {
	return &appRepository{
		db:    db,
		cache: make(map[string]bool),
		mu:    &sync.RWMutex{},
	}
}

func (r *appRepository) AddApp(appName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	insertQuery := `INSERT INTO installed_apps (app_name) VALUES (?)`
	_, err := r.db.Exec(insertQuery, appName)
	if err != nil {
		return fmt.Errorf("failed to insert app: %v", err)
	}

	// Update cache
	r.cache[appName] = true
	return nil
}

func (r *appRepository) GetApp(appName string) (bool, error) {
	if r.mu == nil {
		return false, fmt.Errorf("appRepository is not properly initialized")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	if exists, found := r.cache[appName]; found {
		return exists, nil
	}

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM installed_apps WHERE app_name = ? LIMIT 1)`
	err := r.db.QueryRow(query, appName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check app existence: %v", err)
	}

	r.cache[appName] = exists
	return exists, nil
}

func (r *appRepository) RemoveApp(appName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	deleteQuery := `DELETE FROM installed_apps WHERE app_name = ?`
	_, err := r.db.Exec(deleteQuery, appName)
	if err != nil {
		return fmt.Errorf("failed to delete app: %v", err)
	}

	// Update cache
	delete(r.cache, appName)
	return nil
}
