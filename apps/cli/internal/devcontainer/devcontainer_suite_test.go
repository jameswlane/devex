package devcontainer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDevcontainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Devcontainer Suite")
}
