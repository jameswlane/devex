package remoterun_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRemoteRun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Remote Run Suite")
}
