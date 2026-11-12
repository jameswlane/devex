package stackdetect_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStackDetect(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stack Detect Suite")
}
