package gnome

import (
	"reflect"
	"testing"
)

func TestCompileSchemas(t *testing.T) {
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

			if err := CompileSchemas(); (err != nil) != tt.wantErr {
				t.Errorf("CompileSchemas() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInstallGnomeExtension(t *testing.T) {
	t.Parallel()

	type args struct {
		extension GnomeExtension
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

			if err := InstallGnomeExtension(tt.args.extension); (err != nil) != tt.wantErr {
				t.Errorf("InstallGnomeExtension() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadGnomeExtensions(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadGnomeExtensions(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadGnomeExtensions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadGnomeExtensions() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_copySchemaFile(t *testing.T) {
	t.Parallel()

	type args struct {
		schema SchemaFile
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

			if err := copySchemaFile(tt.args.schema); (err != nil) != tt.wantErr {
				t.Errorf("copySchemaFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
