package config

import (
	"reflect"
	"testing"
)

func TestGetDefaults(t *testing.T) {
	t.Parallel()

	type args struct {
		configName string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := GetDefaults(tt.args.configName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDefaults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDefaults() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		homeDir string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			SetupConfig(tt.args.homeDir)
		})
	}
}

func Test_loadFirstAvailableConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		paths []string
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

			if err := loadFirstAvailableConfig(tt.args.paths...); (err != nil) != tt.wantErr {
				t.Errorf("loadFirstAvailableConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
