package repository

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestNewSchemaRepository(t *testing.T) {
	t.Parallel()

	type args struct {
		db *sql.DB
	}
	tests := []struct {
		name string
		args args
		want SchemaRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NewSchemaRepository(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSchemaRepository() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_applyMigration(t *testing.T) {
	t.Parallel()

	type args struct {
		db        *sql.DB
		dir       string
		version   int
		direction string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := applyMigration(tt.args.db, tt.args.dir, tt.args.version, tt.args.direction); (err != nil) != tt.wantErr {
				t.Errorf("applyMigration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_loadMigrations(t *testing.T) {
	t.Parallel()

	type args struct {
		dir       string
		direction string
	}
	tests := []struct {
		name    string
		args    args
		want    []int
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := loadMigrations(tt.args.dir, tt.args.direction)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadMigrations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadMigrations() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_schemaRepository_ApplyMigrations(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	type args struct {
		migrationsDir string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &schemaRepository{
				db: tt.fields.db,
			}
			if err := r.ApplyMigrations(tt.args.migrationsDir); (err != nil) != tt.wantErr {
				t.Errorf("ApplyMigrations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_schemaRepository_GetVersion(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	tests := []struct {
		name    string
		fields  fields
		want    int
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &schemaRepository{
				db: tt.fields.db,
			}
			got, err := r.GetVersion()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_schemaRepository_RollbackMigrations(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	type args struct {
		migrationsDir string
		targetVersion int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &schemaRepository{
				db: tt.fields.db,
			}
			if err := r.RollbackMigrations(tt.args.migrationsDir, tt.args.targetVersion); (err != nil) != tt.wantErr {
				t.Errorf("RollbackMigrations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_schemaRepository_SetVersion(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	type args struct {
		version int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &schemaRepository{
				db: tt.fields.db,
			}
			if err := r.SetVersion(tt.args.version); (err != nil) != tt.wantErr {
				t.Errorf("SetVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
