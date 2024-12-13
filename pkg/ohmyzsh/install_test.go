package ohmyzsh

import "testing"

func TestInstallOhMyZsh(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InstallOhMyZsh(); (err != nil) != tt.wantErr {
				t.Errorf("InstallOhMyZsh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
