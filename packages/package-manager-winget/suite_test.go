package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWingetPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Winget Plugin Suite")
}
