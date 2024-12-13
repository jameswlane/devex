package asciiart

import "testing"

func TestRenderArt(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // Add this line to run the subtest in parallel
			RenderArt()
		})
	}
}
