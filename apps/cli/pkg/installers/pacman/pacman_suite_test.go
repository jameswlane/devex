package pacman_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPacman(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pacman Installer Suite")
}
