package agentskills_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentSkills(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Skills Suite")
}
