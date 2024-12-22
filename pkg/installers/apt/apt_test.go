package apt

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestInstall(t *testing.T) {
	t.Parallel()

	type args struct {
		packageName string
		dryRun      bool
		repo        repository.Repository
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

			if err := Install(tt.args.packageName, tt.args.dryRun, tt.args.repo); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunAptUpdate(t *testing.T) {
	t.Parallel()

	type args struct {
		forceUpdate bool
		repo        repository.Repository
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

			if err := RunAptUpdate(tt.args.forceUpdate, tt.args.repo); (err != nil) != tt.wantErr {
				t.Errorf("RunAptUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
