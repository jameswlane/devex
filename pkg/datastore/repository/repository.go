package repository

import (
	"github.com/jameswlane/devex/pkg/db"
)

type Repository interface {
	AppRepository
	SystemRepository
	DB() *db.DB
}

type repository struct {
	appRepo    AppRepository
	systemRepo SystemRepository
	db         *db.DB
}

func NewRepository(db *db.DB) Repository {
	return &repository{
		appRepo:    NewAppRepository(db),
		systemRepo: NewSystemRepository(db.DB),
		db:         db,
	}
}

func (r *repository) DB() *db.DB {
	return r.db
}

// AppRepository Methods
func (r *repository) AddApp(appName string) error {
	return r.appRepo.AddApp(appName)
}

func (r *repository) GetApp(appName string) (bool, error) {
	return r.appRepo.GetApp(appName)
}

func (r *repository) RemoveApp(appName string) error {
	return r.appRepo.RemoveApp(appName)
}

// SystemRepository Methods
func (r *repository) Get(key string) (string, error) {
	return r.systemRepo.Get(key)
}

func (r *repository) Set(key, value string) error {
	return r.systemRepo.Set(key, value)
}

func (r *repository) GetAll() (map[string]string, error) {
	return r.systemRepo.GetAll()
}
