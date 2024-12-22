package commands

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestCreateSystemCommand(t *testing.T) {
	t.Parallel()

	type args struct {
		systemRepo repository.SystemRepository
	}
	tests := []struct {
		name string
		args args
		want *cobra.Command
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := CreateSystemCommand(tt.args.systemRepo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateSystemCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
