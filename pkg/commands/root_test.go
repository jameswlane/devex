package commands

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterRootCommand(t *testing.T) {
	t.Parallel()

	type args struct {
		homeDir string
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

			if got := RegisterRootCommand(tt.args.homeDir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegisterRootCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
