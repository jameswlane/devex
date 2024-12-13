package docker

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
)

func TestInstall(t *testing.T) {
	t.Parallel()
	type args struct {
		app    App
		dryRun bool
		db     *datastore.DB
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
			if err := Install(tt.args.app, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadApps(t *testing.T) {
	t.Parallel()
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    []App
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LoadApps(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadApps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadApps() got = %v, want %v", got, tt.want)
			}
		})
	}
}
