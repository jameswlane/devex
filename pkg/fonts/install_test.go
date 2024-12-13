package fonts

import (
	"reflect"
	"testing"
)

func TestInstallFont(t *testing.T) {
	t.Parallel()
	type args struct {
		font Font
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
			if err := InstallFont(tt.args.font); (err != nil) != tt.wantErr {
				t.Errorf("InstallFont() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFonts(t *testing.T) {
	t.Parallel()
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    []Font
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LoadFonts(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFonts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadFonts() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_installFromURL(t *testing.T) {
	t.Parallel()
	type args struct {
		font Font
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
			if err := installFromURL(tt.args.font); (err != nil) != tt.wantErr {
				t.Errorf("installFromURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_installWithHomebrew(t *testing.T) {
	t.Parallel()
	type args struct {
		fontName string
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
			if err := installWithHomebrew(tt.args.fontName); (err != nil) != tt.wantErr {
				t.Errorf("installWithHomebrew() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_installWithOhMyPosh(t *testing.T) {
	t.Parallel()
	type args struct {
		fontName string
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
			if err := installWithOhMyPosh(tt.args.fontName); (err != nil) != tt.wantErr {
				t.Errorf("installWithOhMyPosh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_unzipAndMove(t *testing.T) {
	t.Parallel()
	type args struct {
		zipFile     string
		extractPath string
		dest        string
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
			if err := unzipAndMove(tt.args.zipFile, tt.args.extractPath, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("unzipAndMove() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
