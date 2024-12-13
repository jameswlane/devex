package check_install

import "testing"

func TestIsAppInstalled(t *testing.T) {
	type args struct {
		appName string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsAppInstalled(tt.args.appName)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsAppInstalled() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsAppInstalled() got = %v, want %v", got, tt.want)
			}
		})
	}
}
