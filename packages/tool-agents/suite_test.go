package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestToolAgents(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tool Agents Plugin Suite")
}
