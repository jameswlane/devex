package dock

import (
	"reflect"
	"testing"
)

func TestCheckIfDesktopFileExists(t *testing.T) {
	t.Parallel()

	type args struct {
		desktopFile string
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		want1 string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, got1 := CheckIfDesktopFileExists(tt.args.desktopFile)
			if got != tt.want {
				t.Errorf("CheckIfDesktopFileExists() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("CheckIfDesktopFileExists() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	type args struct {
		configFile string
	}
	tests := []struct {
		name    string
		args    args
		want    Config
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadConfig(tt.args.configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadConfig() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetFavoriteApps(t *testing.T) {
	t.Parallel()

	type args struct {
		config Config
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

			if err := SetFavoriteApps(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("SetFavoriteApps() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_joinStrings(t *testing.T) {
	t.Parallel()

	type args struct {
		items []string
		sep   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := joinStrings(tt.args.items, tt.args.sep); got != tt.want {
				t.Errorf("joinStrings() = %v, want %v", got, tt.want)
			}
		})
	}
}
