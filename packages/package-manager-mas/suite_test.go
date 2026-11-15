package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMasPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mac App Store Plugin Suite")
}
