package plugin_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPluginManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugin Manager Suite")
}
