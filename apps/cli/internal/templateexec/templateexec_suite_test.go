package templateexec_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTemplateExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Execution Runtime Suite")
}
