package installers

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/types"
)

func TestInstallApp(t *testing.T) {
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

			if err := InstallApp(tt.args.app, tt.args.dryRun, tt.args.repo); (err != nil) != tt.wantErr {
				t.Errorf("InstallApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_executeInstallCommand(t *testing.T) {
	t.Parallel()

	type args struct {
		app    types.AppConfig
		repo   repository.Repository
		dryRun bool
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

			if err := executeInstallCommand(tt.args.app, tt.args.repo, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("executeInstallCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_findAppByName(t *testing.T) {
	t.Parallel()

	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want *types.AppConfig
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := findAppByName(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findAppByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleDependencies(t *testing.T) {
	t.Parallel()

	type args struct {
		app    types.AppConfig
		repo   repository.Repository
		dryRun bool
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

			if err := handleDependencies(tt.args.app, tt.args.repo, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("handleDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_preloadDependenciesFromRepo(t *testing.T) {
	t.Parallel()

	type args struct {
		dependencySet map[string]bool
		repo          repository.Repository
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := preloadDependenciesFromRepo(tt.args.dependencySet, tt.args.repo)
			if (err != nil) != tt.wantErr {
				t.Errorf("preloadDependenciesFromRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("preloadDependenciesFromRepo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_retryWithBackoff(t *testing.T) {
	t.Parallel()

	type args struct {
		f func() error
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

			if err := retryWithBackoff(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("retryWithBackoff() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
