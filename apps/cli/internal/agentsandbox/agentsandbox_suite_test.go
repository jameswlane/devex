package agentsandbox_test

import (
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentSandbox(t *testing.T) {
	testhelper.ResolveTempDir()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Sandbox Suite")
}
