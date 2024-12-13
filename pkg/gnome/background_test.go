package gnome

import "testing"

func TestSetBackground(t *testing.T) {
	t.Parallel()

	type args struct {
		imagePath string
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
			if err := SetBackground(tt.args.imagePath); (err != nil) != tt.wantErr {
				t.Errorf("SetBackground() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
