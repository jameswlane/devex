package config

import (
	"testing"
)

func TestLoadAppsConfig(t *testing.T) {
	config, err := LoadAppsConfig("../../config/apps.yaml")
	if err != nil {
		t.Fatalf("Failed to load apps config: %v", err)
	}

	if len(config.Apps) == 0 {
		t.Errorf("Expected at least one app, got 0")
	}

	// Test getting an app by name
	app, err := config.GetAppByName("Redis")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if app.Name != "Redis" {
		t.Errorf("Expected app name 'Redis', but got: %s", app.Name)
	}
}

func TestListAppsByCategory(t *testing.T) {
	config, err := LoadAppsConfig("../../config/apps.yaml")
	if err != nil {
		t.Fatalf("Failed to load apps config: %v", err)
	}

	apps := config.ListAppsByCategory("Database")
	if len(apps) == 0 {
		t.Errorf("Expected at least one app in category 'Database', got 0")
	}
}
