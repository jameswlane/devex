package docker

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/types"
)

func TestInstall(t *testing.T) {
	t.Parallel()

	type args struct {
		app    types.AppConfig
		dryRun bool
		repo   repository.Repository
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

			if err := Install(tt.args.app, tt.args.dryRun, tt.args.repo); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
