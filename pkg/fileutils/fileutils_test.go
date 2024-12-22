package fileutils

import "testing"

func TestCopyConfigFiles(t *testing.T) {
	t.Parallel()

	type args struct {
		srcDir string
		dstDir string
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

			if err := CopyConfigFiles(tt.args.srcDir, tt.args.dstDir); (err != nil) != tt.wantErr {
				t.Errorf("CopyConfigFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	type args struct {
		src string
		dst string
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

			if err := CopyFile(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCopyToBin(t *testing.T) {
	t.Parallel()

	type args struct {
		src    string
		binDir string
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

			if err := CopyToBin(tt.args.src, tt.args.binDir); (err != nil) != tt.wantErr {
				t.Errorf("CopyToBin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
