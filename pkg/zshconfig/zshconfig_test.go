package zshconfig

import "testing"

func TestBackupAndCopyZSHConfig(t *testing.T) {
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
			if err := BackupAndCopyZSHConfig(); (err != nil) != tt.wantErr {
				t.Errorf("BackupAndCopyZSHConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstallZSH(t *testing.T) {
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
			if err := InstallZSH(); (err != nil) != tt.wantErr {
				t.Errorf("InstallZSH() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstallZSHConfig(t *testing.T) {
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
			if err := InstallZSHConfig(); (err != nil) != tt.wantErr {
				t.Errorf("InstallZSHConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_backupAndCopyFile(t *testing.T) {
	t.Parallel()
	type args struct {
		homeDir    string
		filename   string
		sourcePath string
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
			if err := backupAndCopyFile(tt.args.homeDir, tt.args.filename, tt.args.sourcePath); (err != nil) != tt.wantErr {
				t.Errorf("backupAndCopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
