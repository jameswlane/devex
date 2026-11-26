package agentdoctor_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAgentDoctor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Agent Doctor Suite")
}
