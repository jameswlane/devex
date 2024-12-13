package main

import (
	"github.com/jameswlane/devex/pkg/datastore"
	"reflect"
	"testing"
)

func Test_getDefaultsFromConfig(t *testing.T) {
	type args struct {
		category string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDefaultsFromConfig(tt.args.category); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultsFromConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUserSelections(t *testing.T) {
	type args struct {
		category string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUserSelections(tt.args.category); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUserSelections() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_installApps(t *testing.T) {
	type args struct {
		selectedItems []string
		category      string
		db            *datastore.DB
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installApps(tt.args.selectedItems, tt.args.category, tt.args.db)
		})
	}
}

func Test_loadConfigs(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadConfigs()
		})
	}
}

func Test_loadCustomConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadCustomConfig(tt.args.filename)
		})
	}
}

func Test_setupConfig(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupConfig()
		})
	}
}
