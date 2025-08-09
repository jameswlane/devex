package installers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInstallersIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Installers Integration Suite")
}
