package gpg

import "testing"

func TestAddGPGKeyToKeyring(t *testing.T) {
	t.Parallel()

	type args struct {
		filePath string
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

			if err := AddGPGKeyToKeyring(tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("AddGPGKeyToKeyring() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDownloadGPGKey(t *testing.T) {
	t.Parallel()

	type args struct {
		url         string
		destination string
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

			if err := DownloadGPGKey(tt.args.url, tt.args.destination); (err != nil) != tt.wantErr {
				t.Errorf("DownloadGPGKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
