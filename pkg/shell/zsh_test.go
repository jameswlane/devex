package shell

import "testing"

func TestSwitchToZsh(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SwitchToZsh(); (err != nil) != tt.wantErr {
				t.Errorf("SwitchToZsh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
