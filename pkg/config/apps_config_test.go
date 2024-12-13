package config

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/huh"
)

func TestAppsConfig_GetAppByName(t *testing.T) {
	t.Parallel()

	type fields struct {
		Apps []App
	}
	type args struct {
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *App
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
			got, err := c.GetAppByName(tt.args.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAppByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAppByName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppsConfig_ListAppsByCategory(t *testing.T) {
	t.Parallel()

	type fields struct {
		Apps []App
	}
	type args struct {
		category string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []App
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
			if got := c.ListAppsByCategory(tt.args.category); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAppsByCategory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppsConfig_LoadChoicesFromFile(t *testing.T) {
	t.Parallel()

	type fields struct {
		Apps []App
	}
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []huh.Option[string]
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
			got, err := c.LoadChoicesFromFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadChoicesFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadChoicesFromFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadAppsConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		defaultPath string
	}
	tests := []struct {
		name    string
		args    args
		want    AppsConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LoadAppsConfig(tt.args.defaultPath)
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
