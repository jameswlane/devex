package repository

import (
	"database/sql"
	"time"
)

type SystemRepository interface {
	Get(key string) (string, error)
	Set(key, value string) error
	GetAll() (map[string]string, error)
}

type systemRepository struct {
	db *sql.DB
}

func NewSystemRepository(db *sql.DB) SystemRepository {
	return &systemRepository{db: db}
}

func (r *systemRepository) Get(key string) (string, error) {
	var value string
	query := "SELECT value FROM system_data WHERE key = ? LIMIT 1"
	err := r.db.QueryRow(query, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // No value found
	}
	return value, err
}

func (r *systemRepository) Set(key, value string) error {
	query := `INSERT INTO system_data (key, value, updated_at) VALUES (?, ?, ?)
              ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	_, err := r.db.Exec(query, key, value, time.Now())
	return err
}

func (r *systemRepository) GetAll() (map[string]string, error) {
	rows, err := r.db.Query("SELECT key, value FROM system_data")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	data := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		data[key] = value
	}
	return data, nil
}
