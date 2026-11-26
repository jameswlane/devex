package skillpacks_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSkillPacks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SkillPacks Suite")
}
