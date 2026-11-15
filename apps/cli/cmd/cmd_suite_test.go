package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
)

func TestCmd(t *testing.T) {
	t.Parallel()
	testhelper.ResolveTempDir()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}
