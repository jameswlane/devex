package brew

import (
	"github.com/jameswlane/devex/pkg/datastore"
	"testing"
)

func TestInstall(t *testing.T) {
	type args struct {
		packageName string
		dryRun      bool
		db          *datastore.DB
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
			if err := Install(tt.args.packageName, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
