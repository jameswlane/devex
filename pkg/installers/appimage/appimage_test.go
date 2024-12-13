package appimage

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
)

func TestInstall(t *testing.T) {
	type args struct {
		appName     string
		downloadURL string
		installDir  string
		binary      string
		dryRun      bool
		db          *datastore.DB
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
			if err := Install(tt.args.appName, tt.args.downloadURL, tt.args.installDir, tt.args.binary, tt.args.dryRun, tt.args.db); (err != nil) != tt.wantErr {
				t.Errorf("Install() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_downloadFile(t *testing.T) {
	type args struct {
		url  string
		dest string
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
			if err := downloadFile(tt.args.url, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("downloadFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_extractTarball(t *testing.T) {
	type args struct {
		tarballPath string
		destDir     string
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
			if err := extractTarball(tt.args.tarballPath, tt.args.destDir); (err != nil) != tt.wantErr {
				t.Errorf("extractTarball() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_moveFile(t *testing.T) {
	type args struct {
		src  string
		dest string
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
			if err := moveFile(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("moveFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
