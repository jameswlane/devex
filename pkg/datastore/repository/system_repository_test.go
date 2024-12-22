package repository

import (
	"database/sql"
	"reflect"
	"testing"
)

func TestNewSystemRepository(t *testing.T) {
	t.Parallel()

	type args struct {
		db *sql.DB
	}
	tests := []struct {
		name string
		args args
		want SystemRepository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NewSystemRepository(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSystemRepository() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_systemRepository_Get(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	type args struct {
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &systemRepository{
				db: tt.fields.db,
			}
			got, err := r.Get(tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Get() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_systemRepository_GetAll(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	tests := []struct {
		name    string
		fields  fields
		want    map[string]string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &systemRepository{
				db: tt.fields.db,
			}
			got, err := r.GetAll()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAll() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_systemRepository_Set(t *testing.T) {
	t.Parallel()

	type fields struct {
		db *sql.DB
	}
	type args struct {
		key   string
		value string
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
			r := &systemRepository{
				db: tt.fields.db,
			}
			if err := r.Set(tt.args.key, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
