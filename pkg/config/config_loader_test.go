package config

import "testing"

func TestLoadCustomOrDefaultFile(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

func TestLoadYAMLConfig(t *testing.T) {
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

			if err := LoadYAMLConfig(tt.args.filePath, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("LoadYAMLConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
