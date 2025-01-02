package fs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
)

func TestWriteFile(t *testing.T) {
	t.Parallel()
	log.UseTestLogger()
	fs.UseMemMapFs()
	defer fs.UseOsFs()

	err := fs.WriteFile("/test/file.txt", []byte("Hello, World!"), 0o644)
	assert.NoError(t, err)

	content, err := fs.ReadFile("/test/file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(content))
}
