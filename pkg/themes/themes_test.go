package themes

import (
	"reflect"
	"testing"
)

func TestLoadThemes(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    []Theme
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Add this line to run the subtest in parallel
			got, err := LoadThemes(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadThemes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadThemes() got = %v, want %v", got, tt.want)
			}
		})
	}
}
