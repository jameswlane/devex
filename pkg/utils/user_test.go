package utils

import "testing"

func TestGetShellRCPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetShellRCPath()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetShellRCPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetShellRCPath() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUserShell(t *testing.T) {
	t.Parallel()

	type args struct {
		username string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := getUserShell(tt.args.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("getUserShell() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getUserShell() got = %v, want %v", got, tt.want)
			}
		})
	}
}
