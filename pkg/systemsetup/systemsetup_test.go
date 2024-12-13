package systemsetup

import "testing"

func TestDisableSleepSettings(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DisableSleepSettings(); (err != nil) != tt.wantErr {
				t.Errorf("DisableSleepSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Logout(); (err != nil) != tt.wantErr {
				t.Errorf("Logout() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRevertSleepSettings(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RevertSleepSettings(); (err != nil) != tt.wantErr {
				t.Errorf("RevertSleepSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunInstallers(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			if err := RunInstallers(tt.args.installersDir); (err != nil) != tt.wantErr {
				t.Errorf("RunInstallers() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateApt(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpdateApt(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateApt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpgradeSystem(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := UpgradeSystem(); (err != nil) != tt.wantErr {
				t.Errorf("UpgradeSystem() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
