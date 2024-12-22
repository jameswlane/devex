package asciiart

import "testing"

func TestRenderArt(t *testing.T) {
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

			RenderArt()
		})
	}
}
