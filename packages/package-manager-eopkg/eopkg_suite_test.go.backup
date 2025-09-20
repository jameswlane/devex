package eopkg_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEopkg(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Eopkg Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
