package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestYUMPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "YUM Plugin Suite")
}
