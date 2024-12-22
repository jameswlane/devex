package homebrew

import "testing"

func TestInstallHomebrew(t *testing.T) {
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

			if err := InstallHomebrew(); (err != nil) != tt.wantErr {
				t.Errorf("InstallHomebrew() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
