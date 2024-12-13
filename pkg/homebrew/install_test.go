package homebrew

import "testing"

func TestInstallHomebrew(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InstallHomebrew(); (err != nil) != tt.wantErr {
				t.Errorf("InstallHomebrew() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
