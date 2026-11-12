package utils_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("Copy hooks in a confirmed install", func() {
	var source, destination string
	var ctx context.Context
	BeforeEach(func() {
		if os.Geteuid() == 0 {
			Skip("copies switch users when running as root")
		}
		dir := GinkgoT().TempDir()
		source, destination = filepath.Join(dir, "source"), filepath.Join(dir, "destination")
		Expect(os.WriteFile(source, []byte("confirmed"), 0o600)).To(Succeed())
		ctx = execinputs.With(context.Background(), execinputs.Inputs{Files: map[string]string{source: execinputs.Fingerprint(source)}})
	})

	It("copies the confirmed content", func() {
		Expect(utils.CopyUserFileWithContext(ctx, source, destination)).To(Succeed())
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("confirmed"))
	})

	It("refuses a source that changed after confirmation and writes nothing", func() {
		Expect(os.WriteFile(source, []byte("swapped"), 0o600)).To(Succeed())
		err := utils.CopyUserFileWithContext(ctx, source, destination)
		Expect(errors.Is(err, execinputs.ErrChanged)).To(BeTrue())
		Expect(destination).NotTo(BeAnExistingFile())
	})

	It("refuses a source that was not part of the confirmed plan", func() {
		other := filepath.Join(filepath.Dir(source), "other")
		Expect(os.WriteFile(other, []byte("confirmed"), 0o600)).To(Succeed())
		Expect(errors.Is(utils.CopyUserFileWithContext(ctx, other, destination), execinputs.ErrChanged)).To(BeTrue())
	})
})
