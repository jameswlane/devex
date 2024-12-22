package datastore

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/db"
)

func TestDB_GetDB(t *testing.T) {
	t.Parallel()
	type fields struct {
		DB *sql.DB
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

			d := &DB{
				DB: tt.fields.DB,
			}
			if got := d.GetDB(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDB() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitDB(t *testing.T) {
	t.Parallel()
	type args struct {
		dbPath string
	}
	tests := []struct {
		name    string
		args    args
		want    *DB
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := InitDB(tt.args.dbPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitDB() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InitDB() got = %v, want %v", got, tt.want)
			}
		})
	}
}
