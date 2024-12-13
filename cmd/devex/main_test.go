package main

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
)

func Test_getDefaultsFromConfig(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getDefaultsFromConfig(tt.args.category); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDefaultsFromConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUserSelections(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getUserSelections(tt.args.category); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUserSelections() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_installApps(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			installApps(tt.args.selectedItems, tt.args.category, tt.args.db)
		})
	}
}

func Test_loadConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			loadConfigs()
		})
	}
}

func Test_loadCustomConfig(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			loadCustomConfig(tt.args.filename)
		})
	}
}

func Test_setupConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			setupConfig()
		})
	}
}
