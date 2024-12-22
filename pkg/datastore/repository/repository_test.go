package repository

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/db"
)

func TestNewRepository(t *testing.T) {
	t.Parallel()

	type args struct {
		db *db.DB
	}
	tests := []struct {
		name string
		args args
		want Repository
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NewRepository(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewRepository() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_repository_AddApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
	}
	type args struct {
		appName string
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

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
			}
			if err := r.AddApp(tt.args.appName); (err != nil) != tt.wantErr {
				t.Errorf("AddApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_repository_DB(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
	}
	tests := []struct {
		name   string
		fields fields
		want   *db.DB
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
			}
			if got := r.DB(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DB() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_repository_Get(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
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

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
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

func Test_repository_GetAll(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
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

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
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

func Test_repository_GetApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
	}
	type args struct {
		appName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
			}
			got, err := r.GetApp(tt.args.appName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetApp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_repository_RemoveApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
	}
	type args struct {
		appName string
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

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
			}
			if err := r.RemoveApp(tt.args.appName); (err != nil) != tt.wantErr {
				t.Errorf("RemoveApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_repository_Set(t *testing.T) {
	t.Parallel()

	type fields struct {
		appRepo    AppRepository
		systemRepo SystemRepository
		db         *db.DB
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

			r := &repository{
				appRepo:    tt.fields.appRepo,
				systemRepo: tt.fields.systemRepo,
				db:         tt.fields.db,
			}
			if err := r.Set(tt.args.key, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
