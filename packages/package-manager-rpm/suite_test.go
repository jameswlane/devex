package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRPMPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPM Plugin Suite")
}
