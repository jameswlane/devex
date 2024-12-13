package config

import (
	"reflect"
	"testing"
)

func TestLoadDockConfig(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
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
