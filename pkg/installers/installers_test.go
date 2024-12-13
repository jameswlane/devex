package installers

import (
	"testing"

)

func TestInstallApp(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			if err := InstallApp(tt.args.app, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("InstallApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
