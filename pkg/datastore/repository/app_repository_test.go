package repository

import (
	"reflect"
	"sync"
	"testing"

	"github.com/jameswlane/devex/pkg/db"
)

func TestNewAppRepository(t *testing.T) {
	t.Parallel()

	type args struct {
		db *db.DB
	}
	tests := []struct {
		name string
		args args
		want AppRepository
	}{
		// TODO: Add test cases.
	}
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := NewAppRepository(tt.args.db); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAppRepository() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_appRepository_AddApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		db    *db.DB
		cache map[string]bool
		mu    *sync.RWMutex
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
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &appRepository{
				db:    tt.fields.db,
				cache: tt.fields.cache,
				mu:    tt.fields.mu,
			}
			if err := r.AddApp(tt.args.appName); (err != nil) != tt.wantErr {
				t.Errorf("AddApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_appRepository_GetApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		db    *db.DB
		cache map[string]bool
		mu    *sync.RWMutex
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
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &appRepository{
				db:    tt.fields.db,
				cache: tt.fields.cache,
				mu:    tt.fields.mu,
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

func Test_appRepository_RemoveApp(t *testing.T) {
	t.Parallel()

	type fields struct {
		db    *db.DB
		cache map[string]bool
		mu    *sync.RWMutex
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
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &appRepository{
				db:    tt.fields.db,
				cache: tt.fields.cache,
				mu:    tt.fields.mu,
			}
			if err := r.RemoveApp(tt.args.appName); (err != nil) != tt.wantErr {
				t.Errorf("RemoveApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
