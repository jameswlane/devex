package utils

import "testing"

func TestExtendSudoSession(t *testing.T) {
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

			if err := ExtendSudoSession(); (err != nil) != tt.wantErr {
				t.Errorf("ExtendSudoSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunShellCommand(t *testing.T) {
	t.Parallel()

	type args struct {
		command string
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

			if err := RunShellCommand(tt.args.command); (err != nil) != tt.wantErr {
				t.Errorf("RunShellCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
