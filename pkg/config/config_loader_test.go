package config

import (
	"reflect"
	"testing"
)

func TestLoadCustomOrDefaultFile(t *testing.T) {
	type args struct {
		defaultPath string
		assetType   string
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
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadCustomOrDefaultFile(tt.args.defaultPath, tt.args.assetType)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCustomOrDefaultFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("LoadCustomOrDefaultFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadGnomeExtensionsConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    []GnomeExtension
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadGnomeExtensionsConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadGnomeExtensionsConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadGnomeExtensionsConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadProgrammingLanguagesConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    []ProgrammingLanguage
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadProgrammingLanguagesConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadProgrammingLanguagesConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadProgrammingLanguagesConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadYAMLConfig(t *testing.T) {
	type args struct {
		filePath string
		out      interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := LoadYAMLConfig(tt.args.filePath, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("LoadYAMLConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
