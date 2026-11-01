package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShell(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shell Plugin Suite")
}

// Every spec runs with a throwaway HOME so no test can edit the real user's
// shell rc files. Specs that need a specific HOME still override it; Setenv
// restores the previous value after each spec.
var _ = BeforeEach(func() {
	GinkgoT().Setenv("HOME", GinkgoT().TempDir())
})
