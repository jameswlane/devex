package installers

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
)

func TestInstallApp(t *testing.T) {
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
			if err := InstallApp(tt.args.app, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("InstallApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
