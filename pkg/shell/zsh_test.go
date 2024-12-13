package shell

import "testing"

func TestSwitchToZsh(t *testing.T) {
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
			if err := SwitchToZsh(); (err != nil) != tt.wantErr {
				t.Errorf("SwitchToZsh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
