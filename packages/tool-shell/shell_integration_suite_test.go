//go:build integration

package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShellIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shell Tool Integration Suite")
}
