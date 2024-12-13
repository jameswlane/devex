package brew

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
)

func TestInstall(t *testing.T) {
	t.Parallel()
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := Install(tt.args.packageName, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
