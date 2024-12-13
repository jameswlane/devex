package layout

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestLayoutModel_RenderView(t *testing.T) {
	t.Parallel()
	type fields struct {
		StepsPane   viewport.Model
		LogsPane    viewport.Model
		ProgressBar progress.Model
		SystemInfo  string
	}
	type args struct {
		steps   []string
		logs    []string
		percent float64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := LayoutModel{
				StepsPane:   tt.fields.StepsPane,
				LogsPane:    tt.fields.LogsPane,
				ProgressBar: tt.fields.ProgressBar,
				SystemInfo:  tt.fields.SystemInfo,
			}
			if got := m.RenderView(tt.args.steps, tt.args.logs, tt.args.percent); got != tt.want {
				t.Errorf("RenderView() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLayoutModel_UpdateLogsPane(t *testing.T) {
	t.Parallel()
	type fields struct {
		StepsPane   viewport.Model
		LogsPane    viewport.Model
		ProgressBar progress.Model
		SystemInfo  string
	}
	type args struct {
		logs []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &LayoutModel{
				StepsPane:   tt.fields.StepsPane,
				LogsPane:    tt.fields.LogsPane,
				ProgressBar: tt.fields.ProgressBar,
				SystemInfo:  tt.fields.SystemInfo,
			}
			l.UpdateLogsPane(tt.args.logs)
		})
	}
}

func TestNewLayoutModel(t *testing.T) {
	t.Parallel()
	type args struct {
		systemInfo string
		stepsWidth int
		logsWidth  int
		height     int
	}
	tests := []struct {
		name string
		args args
		want LayoutModel
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NewLayoutModel(tt.args.systemInfo, tt.args.stepsWidth, tt.args.logsWidth, tt.args.height); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewLayoutModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderLogs(t *testing.T) {
	t.Parallel()
	type args struct {
		logs  []string
		width int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderLogs(tt.args.logs, tt.args.width); got != tt.want {
				t.Errorf("renderLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderProgressBar(t *testing.T) {
	t.Parallel()
	type args struct {
		progressBar progress.Model
		percent     float64
		width       int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderProgressBar(tt.args.progressBar, tt.args.percent, tt.args.width); got != tt.want {
				t.Errorf("renderProgressBar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderSteps(t *testing.T) {
	t.Parallel()
	type args struct {
		steps []string
		width int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderSteps(tt.args.steps, tt.args.width); got != tt.want {
				t.Errorf("renderSteps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_renderTopBar(t *testing.T) {
	t.Parallel()
	type args struct {
		systemInfo string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := renderTopBar(tt.args.systemInfo); got != tt.want {
				t.Errorf("renderTopBar() = %v, want %v", got, tt.want)
			}
		})
	}
}
