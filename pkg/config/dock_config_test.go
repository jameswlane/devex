package config

import (
	"reflect"
	"testing"
)

func TestLoadDockConfig(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	type args struct {
		defaultPath string
	}
	tests := []struct {
		name    string
		args    args
		want    DockConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Add this line to run the subtest in parallel
			got, err := LoadDockConfig(tt.args.defaultPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadDockConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadDockConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}
