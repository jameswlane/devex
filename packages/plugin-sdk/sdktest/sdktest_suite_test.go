package sdktest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSDKTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SDK Test Helpers Suite")
}
