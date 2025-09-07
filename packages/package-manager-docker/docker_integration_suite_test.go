//go:build integration

package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDockerIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Docker Package Manager Integration Suite")
}
