package config

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/types"
)

func TestAppsConfig_ListAppsByCategories(t *testing.T) {
	t.Parallel()

	type fields struct {
		Apps []types.AppConfig
	}
	type args struct {
		categories []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []types.AppConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &AppsConfig{
				Apps: tt.fields.Apps,
			}
			got, err := c.ListAppsByCategories(tt.args.categories)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListAppsByCategories() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAppsByCategories() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadAppsConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		want    AppsConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadAppsConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadAppsConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadAppsConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_loadYAMLWithCache(t *testing.T) {
	t.Parallel()

	type args struct {
		filePath string
		out      any
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

			if err := loadYAMLWithCache(tt.args.filePath, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("loadYAMLWithCache() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateApp(t *testing.T) {
	t.Parallel()
	type args struct {
		app types.AppConfig
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

			if err := validateApp(tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("validateApp() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
