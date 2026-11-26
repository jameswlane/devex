package winprereq_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWinprereq(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Windows Prerequisites Suite")
}
