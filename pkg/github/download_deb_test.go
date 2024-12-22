package github

import (
	"net/http"
	"testing"
)

func TestDownloadDeb(t *testing.T) {
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

			if err := DownloadDeb(tt.args.url, tt.args.destination); (err != nil) != tt.wantErr {
				t.Errorf("DownloadDeb() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetLatestDebURL(t *testing.T) {
	t.Parallel()

	type args struct {
		owner   string
		repo    string
		baseURL string
		client  *http.Client
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

			got, err := GetLatestDebURL(tt.args.owner, tt.args.repo, tt.args.baseURL, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestDebURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLatestDebURL() got = %v, want %v", got, tt.want)
			}
		})
	}
}
