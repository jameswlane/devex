package selfupdate_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSelfUpdate(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Self Update Suite") }
