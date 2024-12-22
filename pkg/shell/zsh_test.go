package shell

import "testing"

func TestSwitchToZsh(t *testing.T) {
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

			if err := SwitchToZsh(); (err != nil) != tt.wantErr {
				t.Errorf("SwitchToZsh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
