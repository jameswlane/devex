package setupundo_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSetupUndo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Setup Undo Suite")
}
