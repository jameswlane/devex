package view

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/layout"
	"github.com/jameswlane/devex/pkg/logger"
	"github.com/jameswlane/devex/pkg/steps"
)

func TestNewViewModel(t *testing.T) {
	type args struct {
		systemInfo string
		width      int
		height     int
	}
	tests := []struct {
		name string
		args args
		want ViewModel
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewViewModel(tt.args.systemInfo, tt.args.width, tt.args.height); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewViewModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViewModel_ExecuteSteps(t *testing.T) {
	type fields struct {
		layout     layout.LayoutModel
		logs       []string
		steps      []string
		maxLogSize int
	}
	type args struct {
		stepsList []steps.Step
		dryRun    bool
		db        *datastore.DB
		logger    *logger.Logger
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ViewModel{
				layout:     tt.fields.layout,
				logs:       tt.fields.logs,
				steps:      tt.fields.steps,
				maxLogSize: tt.fields.maxLogSize,
			}
			v.ExecuteSteps(tt.args.stepsList, tt.args.dryRun, tt.args.db, tt.args.logger)
		})
	}
}

func TestViewModel_Render(t *testing.T) {
	type fields struct {
		layout     layout.LayoutModel
		logs       []string
		steps      []string
		maxLogSize int
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := ViewModel{
				layout:     tt.fields.layout,
				logs:       tt.fields.logs,
				steps:      tt.fields.steps,
				maxLogSize: tt.fields.maxLogSize,
			}
			if got := v.Render(); got != tt.want {
				t.Errorf("Render() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViewModel_addLog(t *testing.T) {
	type fields struct {
		layout     layout.LayoutModel
		logs       []string
		steps      []string
		maxLogSize int
	}
	type args struct {
		logEntry string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ViewModel{
				layout:     tt.fields.layout,
				logs:       tt.fields.logs,
				steps:      tt.fields.steps,
				maxLogSize: tt.fields.maxLogSize,
			}
			v.addLog(tt.args.logEntry)
		})
	}
}

func TestViewModel_updateLogsPane(t *testing.T) {
	type fields struct {
		layout     layout.LayoutModel
		logs       []string
		steps      []string
		maxLogSize int
	}
	tests := []struct {
		name   string
		fields fields
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ViewModel{
				layout:     tt.fields.layout,
				logs:       tt.fields.logs,
				steps:      tt.fields.steps,
				maxLogSize: tt.fields.maxLogSize,
			}
			v.updateLogsPane()
		})
	}
}

func TestViewModel_updateStepsPane(t *testing.T) {
	type fields struct {
		layout     layout.LayoutModel
		logs       []string
		steps      []string
		maxLogSize int
	}
	type args struct {
		stepsList []steps.Step
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &ViewModel{
				layout:     tt.fields.layout,
				logs:       tt.fields.logs,
				steps:      tt.fields.steps,
				maxLogSize: tt.fields.maxLogSize,
			}
			v.updateStepsPane(tt.args.stepsList)
		})
	}
}
