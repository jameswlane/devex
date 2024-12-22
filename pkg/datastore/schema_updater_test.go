package datastore

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestApplySchemaUpdates(t *testing.T) {
	t.Parallel()

	type args struct {
		repo repository.SchemaRepository
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := ApplySchemaUpdates(tt.args.repo); (err != nil) != tt.wantErr {
				t.Errorf("ApplySchemaUpdates() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
