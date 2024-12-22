package db

import (
	"database/sql"
)

type DB struct {
	*sql.DB
}
