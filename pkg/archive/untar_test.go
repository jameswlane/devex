package archive

import "testing"

func TestDownloadTarGz(t *testing.T) {
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

			if err := DownloadTarGz(tt.args.url, tt.args.destination); (err != nil) != tt.wantErr {
				t.Errorf("DownloadTarGz() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUntar(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := Untar(tt.args.src, tt.args.dest); (err != nil) != tt.wantErr {
				t.Errorf("Untar() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
