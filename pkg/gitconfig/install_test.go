package gitconfig

import (
	"reflect"
	"testing"
)

func TestApplyGitConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		gitConfig *GitConfig
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
			if err := ApplyGitConfig(tt.args.gitConfig); (err != nil) != tt.wantErr {
				t.Errorf("ApplyGitConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadGitConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *GitConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LoadGitConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadGitConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadGitConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
