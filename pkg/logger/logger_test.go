package logger

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestInitLogger(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want *Logger
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := InitLogger(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InitLogger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_ExecCommandWithLogging(t *testing.T) {
	t.Parallel()
	type fields struct {
		logs []string
	}
	type args struct {
		cmd *exec.Cmd
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &Logger{
				logs: tt.fields.logs,
			}
			if err := l.ExecCommandWithLogging(tt.args.cmd); (err != nil) != tt.wantErr {
				t.Errorf("ExecCommandWithLogging() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLogger_GetLogs(t *testing.T) {
	t.Parallel()
	type fields struct {
		logs []string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l := &Logger{
				logs: tt.fields.logs,
			}
			if got := l.GetLogs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetLogs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger_LogError(t *testing.T) {
	t.Parallel()
	type fields struct {
		logs []string
	}
	type args struct {
		msg string
		err error
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
			l := &Logger{
				logs: tt.fields.logs,
			}
			l.LogError(tt.args.msg, tt.args.err)
		})
	}
}

func TestLogger_LogInfo(t *testing.T) {
	t.Parallel()
	type fields struct {
		logs []string
	}
	type args struct {
		msg string
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
			l := &Logger{
				logs: tt.fields.logs,
			}
			l.LogInfo(tt.args.msg)
		})
	}
}
