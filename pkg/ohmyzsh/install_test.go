package ohmyzsh

import "testing"

func TestInstallOhMyZsh(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Add this line to run the subtest in parallel
			if err := InstallOhMyZsh(); (err != nil) != tt.wantErr {
				t.Errorf("InstallOhMyZsh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
