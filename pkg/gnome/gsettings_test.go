package gnome

import "testing"

func TestSetGSetting(t *testing.T) {
	t.Parallel()

	type args struct {
		schema string
		key    string
		value  string
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

			if err := SetGSetting(tt.args.schema, tt.args.key, tt.args.value); (err != nil) != tt.wantErr {
				t.Errorf("SetGSetting() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
