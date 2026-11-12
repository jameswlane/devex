package configfiles_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Config files in a confirmed install", func() {
	var home, source, destination, marker string
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		source = filepath.Join(home, "source.conf")
		destination = filepath.Join(home, ".config", "tool", "tool.conf")
		marker = filepath.Join(home, "marker")
		Expect(os.WriteFile(source, []byte("confirmed"), 0o600)).To(Succeed())
	})
	pinned := func() context.Context {
		return execinputs.With(context.Background(), execinputs.Inputs{
			Files:   map[string]string{source: execinputs.Fingerprint(source)},
			Markers: map[string]bool{marker: false},
		})
	}

	It("copies a source whose content matches the confirmed plan", func() {
		result := configfiles.Apply(pinned(), "tool", []types.ConfigFile{{Source: source, Destination: destination}}, false, nil)
		Expect(result.Changed).NotTo(HaveOccurred())
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("confirmed"))
	})

	It("refuses a source that changed after confirmation and copies nothing", func() {
		ctx := pinned()
		Expect(os.WriteFile(source, []byte("swapped"), 0o600)).To(Succeed())
		result := configfiles.Apply(ctx, "tool", []types.ConfigFile{{Source: source, Destination: destination}}, false, nil)
		Expect(errors.Is(result.Changed, execinputs.ErrChanged)).To(BeTrue())
		Expect(destination).NotTo(BeAnExistingFile())
	})

	DescribeTable("classifies failed source access against the approved readability", func(approval, failure string) {
		if failure == "read" {
			if runtime.GOOS != "linux" {
				Skip("requires Linux /proc/self/mem for a root-independent read error")
			}
			info, err := os.Stat("/proc/self/mem")
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().IsRegular()).To(BeTrue())
			_, err = os.ReadFile("/proc/self/mem")
			Expect(err).To(HaveOccurred())
		}
		ctx := context.Background()
		if approval == "missing" {
			Expect(os.Remove(source)).To(Succeed())
		}
		if approval != "unpinned" {
			ctx = pinned()
		}
		if approval != "missing" {
			Expect(os.Remove(source)).To(Succeed())
		}
		switch failure {
		case "directory":
			Expect(os.Mkdir(source, 0700)).To(Succeed())
		case "read":
			Expect(os.Symlink("/proc/self/mem", source)).To(Succeed())
		}
		result := configfiles.Apply(ctx, "tool", []types.ConfigFile{{Source: source, Destination: destination}}, false, nil)
		Expect(result.Warnings).To(HaveLen(1))
		if approval == "readable" {
			Expect(errors.Is(result.Changed, execinputs.ErrChanged)).To(BeTrue())
		} else {
			Expect(result.Changed).NotTo(HaveOccurred())
		}
		Expect(destination).NotTo(BeAnExistingFile())
	}, Entry("approved readable then removed", "readable", "missing"), Entry("approved readable then directory", "readable", "directory"), Entry("approved readable then read error", "readable", "read"),
		Entry("unpinned missing stays optional", "unpinned", "missing"), Entry("unpinned directory stays optional", "unpinned", "directory"), Entry("unpinned read error stays optional", "unpinned", "read"),
		Entry("approved missing stays optional", "missing", "missing"), Entry("approved missing then directory stays optional", "missing", "directory"), Entry("approved missing then read error stays optional", "missing", "read"))

	It("refuses a marker whose presence changed after confirmation", func() {
		ctx := pinned()
		Expect(os.WriteFile(marker, nil, 0o600)).To(Succeed())
		result := configfiles.Apply(ctx, "tool", []types.ConfigFile{{Source: source, Destination: destination, RequiresAny: []string{marker}}}, false, nil)
		Expect(errors.Is(result.Changed, execinputs.ErrChanged)).To(BeTrue())
		Expect(destination).NotTo(BeAnExistingFile())
	})

	It("checks nothing for an unconfirmed install", func() {
		Expect(os.WriteFile(source, []byte("anything"), 0o600)).To(Succeed())
		result := configfiles.Apply(context.Background(), "tool", []types.ConfigFile{{Source: source, Destination: destination}}, false, nil)
		Expect(result.Changed).NotTo(HaveOccurred())
		Expect(os.ReadFile(destination)).To(BeEquivalentTo("anything"))
	})
})
