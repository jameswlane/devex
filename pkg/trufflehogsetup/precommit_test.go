package trufflehogsetup

import "testing"

func TestCreatePreCommitConfig(t *testing.T) {
	t.Parallel()
	type args struct {
		useDocker bool
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
			if err := CreatePreCommitConfig(tt.args.useDocker); (err != nil) != tt.wantErr {
				t.Errorf("CreatePreCommitConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstallPreCommitHook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := InstallPreCommitHook(); (err != nil) != tt.wantErr {
				t.Errorf("InstallPreCommitHook() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
