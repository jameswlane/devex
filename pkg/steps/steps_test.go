package steps

import (
	"reflect"
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/logger"
)

func TestExecuteSteps(t *testing.T) {
	type args struct {
		stepsList []Step
		dryRun    bool
		db        *datastore.DB
		logger    *logger.Logger
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExecuteSteps(tt.args.stepsList, tt.args.dryRun, tt.args.db, tt.args.logger)
		})
	}
}

func TestGenerateSteps(t *testing.T) {
	tests := []struct {
		name    string
		want    []Step
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSteps()
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSteps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GenerateSteps() got = %v, want %v", got, tt.want)
			}
		})
	}
}
