package zypper_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestZypper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zypper Suite")
}
