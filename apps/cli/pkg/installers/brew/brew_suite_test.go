package brew

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBrew(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Brew Installer Suite")
}
