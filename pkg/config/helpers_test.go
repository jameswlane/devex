package config

import (
	"reflect"
	"testing"
)

func TestLoadCustomOrDefault(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	type args struct {
		defaultPath string
		customPath  string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Add this line to run the subtest in parallel
			got, err := LoadCustomOrDefault(tt.args.defaultPath, tt.args.customPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCustomOrDefault() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadCustomOrDefault() got = %v, want %v", got, tt.want)
			}
		})
	}
}
