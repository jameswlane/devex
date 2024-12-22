package utils

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/types"
)

func TestExecAsUser(t *testing.T) {
	t.Parallel()

	type args struct {
		command string
		dryRun  bool
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

			if err := ExecAsUser(tt.args.command, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("ExecAsUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessInstallCommands(t *testing.T) {
	t.Parallel()

	type args struct {
		commands []types.InstallCommand
		repo     repository.Repository
		dryRun   bool
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

			if err := ProcessInstallCommands(tt.args.commands, tt.args.repo, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("ProcessInstallCommands() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReplacePlaceholders(t *testing.T) {
	t.Parallel()

	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ReplacePlaceholders(tt.args.input); got != tt.want {
				t.Errorf("ReplacePlaceholders() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUpdateShellConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		commands []string
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

			if err := UpdateShellConfig(tt.args.commands); (err != nil) != tt.wantErr {
				t.Errorf("UpdateShellConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
