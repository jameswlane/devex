package systemsetup

import "testing"

func TestDisableSleepSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := DisableSleepSettings(); (err != nil) != tt.wantErr {
				t.Errorf("DisableSleepSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := Logout(); (err != nil) != tt.wantErr {
				t.Errorf("Logout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRevertSleepSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := RevertSleepSettings(); (err != nil) != tt.wantErr {
				t.Errorf("RevertSleepSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunInstallers(t *testing.T) {
	t.Parallel()

	type args struct {
		installersDir string
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
			if err := RunInstallers(tt.args.installersDir); (err != nil) != tt.wantErr {
				t.Errorf("RunInstallers() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateApt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := UpdateApt(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateApt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpgradeSystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := UpgradeSystem(); (err != nil) != tt.wantErr {
				t.Errorf("UpgradeSystem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
