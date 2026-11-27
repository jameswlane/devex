package localmodels_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLocalModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LocalModels Suite")
}
