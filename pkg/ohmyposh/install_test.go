package ohmyposh

import "testing"

func TestInstallOhMyPosh(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InstallOhMyPosh(); (err != nil) != tt.wantErr {
				t.Errorf("InstallOhMyPosh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
