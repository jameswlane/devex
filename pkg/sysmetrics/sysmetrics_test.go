package sysmetrics

import "testing"

func TestGetCPUUsage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    float64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetCPUUsage()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCPUUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetCPUUsage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDiskUsage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    float64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetDiskUsage()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDiskUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetDiskUsage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRAMUsage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		want    float64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetRAMUsage()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRAMUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetRAMUsage() got = %v, want %v", got, tt.want)
			}
		})
	}
}
