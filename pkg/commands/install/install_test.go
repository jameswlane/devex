package install

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestCreateInstallCommand(t *testing.T) {
	t.Parallel()

	type args struct {
		homeDir string
	}
	tests := []struct {
		name string
		args args
		want *cobra.Command
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CreateInstallCommand(tt.args.homeDir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateInstallCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_installComponents(t *testing.T) {
	t.Parallel()

	type args struct {
		repo   repository.Repository
		dryRun bool
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			installComponents(tt.args.repo, tt.args.dryRun)
		})
	}
}

func Test_runInstall(t *testing.T) {
	t.Parallel()

	type args struct {
		homeDir string
		dryRun  bool
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runInstall(tt.args.homeDir, tt.args.dryRun)
		})
	}
}
