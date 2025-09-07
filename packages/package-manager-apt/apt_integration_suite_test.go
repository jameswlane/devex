//go:build integration

package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPTIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APT Package Manager Integration Suite")
}
