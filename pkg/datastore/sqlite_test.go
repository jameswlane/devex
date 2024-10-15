package datastore

import (
	"os"
	"testing"
)

func TestSQLiteDatastore(t *testing.T) {
	// Create a temporary database
	dbPath := "/tmp/test_installed_apps.db"
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer os.Remove(dbPath) // Clean up

	// Test adding an app
	err = AddInstalledApp(db, "testapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Test checking if app exists
	exists, err := IsAppInDB(db, "testapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if !exists {
		t.Errorf("Expected app to exist, but it does not")
	}

	// Test removing the app
	err = RemoveInstalledApp(db, "testapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Test checking if app was removed
	exists, err = IsAppInDB(db, "testapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if exists {
		t.Errorf("Expected app to not exist, but it does")
	}
}
