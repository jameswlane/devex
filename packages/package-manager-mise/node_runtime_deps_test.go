package main_test

import (
	"context"
	"debug/elf"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Node runtime dependencies on Linux (#703)", func() {
	var (
		plugin   *main.MisePlugin
		libDir   string
		restores []func()
	)

	newPlugin := func() *main.MisePlugin {
		p := &main.MisePlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise"),
		}
		p.InitLogger(sdk.NewDefaultLogger(false))
		return p
	}

	BeforeEach(func() {
		plugin = newPlugin()
		GinkgoT().Setenv("MISE_LOCAL", "0")
		libDir = GinkgoT().TempDir()
		restores = []func(){
			main.SetTargetOS("linux"),
			main.SetLibatomicPatterns([]string{filepath.Join(libDir, "*", "libatomic.so.1")}),
			main.SetLdconfigPaths(nil),
			main.SetNativeELFMachine(elf.EM_X86_64),
		}
	})

	AfterEach(func() {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	})

	// writeELF writes a minimal ELF64 shared-object header for machine.
	writeELF := func(path string, machine elf.Machine) {
		header := make([]byte, 64)
		copy(header, elf.ELFMAG)
		header[elf.EI_CLASS] = byte(elf.ELFCLASS64)
		header[elf.EI_DATA] = byte(elf.ELFDATA2LSB)
		header[elf.EI_VERSION] = byte(elf.EV_CURRENT)
		binary.LittleEndian.PutUint16(header[16:], uint16(elf.ET_DYN))
		binary.LittleEndian.PutUint16(header[18:], uint16(machine))
		binary.LittleEndian.PutUint32(header[20:], uint32(elf.EV_CURRENT))
		binary.LittleEndian.PutUint16(header[52:], 64)
		Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
		Expect(os.WriteFile(path, header, 0o644)).To(Succeed())
	}

	provideLibatomic := func() {
		writeELF(filepath.Join(libDir, "x86_64-linux-gnu", "libatomic.so.1"), elf.EM_X86_64)
	}

	// fakeLdconfig makes `ldconfig -p` list libatomic.so.1 at path.
	fakeLdconfig := func(path string) {
		if runtime.GOOS == "windows" {
			Skip("the fake ldconfig is a shell script")
		}
		script := filepath.Join(GinkgoT().TempDir(), "ldconfig")
		body := "#!/bin/sh\nprintf '2 libs found in cache\\n\\tlibc.so.6 (libc6,x86-64) => /lib/libc.so.6\\n\\tlibatomic.so.1 (libc6,x86-64) => " + path + "\\n'\n"
		Expect(os.WriteFile(script, []byte(body), 0o755)).To(Succeed())
		restores = append(restores, main.SetLdconfigPaths([]string{script}))
	}

	It("installs libatomic1 with apt-get before mise installs node", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts", "npm:@google/gemini-cli@latest"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
		}))
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"use", "--global", "node@lts", "npm:@google/gemini-cli@latest"}},
		}))
	})

	It("refreshes empty apt lists and retries once", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")
		recorder.Script("apt-get", []string{"install", "-y", "libatomic1"}, sdktest.Response{ExitCode: 100})

		err := plugin.HandleInstall(context.Background(), []string{"node@lts"})

		Expect(err).To(MatchError(ContainSubstring("run `sudo apt-get install -y libatomic1` and retry")))
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
		}))
		Expect(recorder.Calls("mise")).To(BeEmpty())
	})

	It("uses dnf's libatomic package on Fedora", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "dnf")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@25"})).To(Succeed())

		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic"}, ElevationRequested: true},
		}))
	})

	It("skips the system package when libatomic.so.1 is already present", func() {
		provideLibatomic()
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("mise")).To(HaveLen(1))
	})

	It("treats mise's nodejs alias like node (#738)", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"nodejs@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "-y", "libatomic1"}, ElevationRequested: true},
		}))
	})

	It("leaves node@system, which uses the Node already installed, alone (#738)", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@system"})).To(Succeed())
		Expect(plugin.HandleInstall(context.Background(), []string{"nodejs@system"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("ignores a libatomic built for another architecture (#738)", func() {
		writeELF(filepath.Join(libDir, "i386-linux-gnu", "libatomic.so.1"), elf.EM_386)
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(HaveLen(1))
	})

	It("ignores an x32 libatomic, which is 32-bit x86-64 (#738)", func() {
		if strconv.IntSize != 64 {
			Skip("the ELF class check applies to 64-bit builds")
		}
		path := filepath.Join(libDir, "x32", "libatomic.so.1")
		writeELF(path, elf.EM_X86_64)
		raw, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		raw[elf.EI_CLASS] = byte(elf.ELFCLASS32)
		Expect(os.WriteFile(path, raw[:52], 0o644)).To(Succeed())
		x32, err := elf.Open(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(x32.Machine).To(Equal(elf.EM_X86_64))
		Expect(x32.Close()).To(Succeed())
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(HaveLen(1))
	})

	It("ignores a dangling libatomic.so.1 symlink (#738)", func() {
		if runtime.GOOS == "windows" {
			Skip("symlinks need extra privileges on Windows")
		}
		dir := filepath.Join(libDir, "x86_64-linux-gnu")
		Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
		Expect(os.Symlink(filepath.Join(dir, "libatomic.so.1.2.0"), filepath.Join(dir, "libatomic.so.1"))).To(Succeed())
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(HaveLen(1))
	})

	It("finds a libatomic registered only in the loader cache (#738)", func() {
		optLib := filepath.Join(GinkgoT().TempDir(), "opt", "gcc", "lib64", "libatomic.so.1")
		writeELF(optLib, elf.EM_X86_64)
		fakeLdconfig(optLib)
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("mise")).To(HaveLen(1))
	})

	It("does not trust a loader-cache entry whose file is gone (#738)", func() {
		fakeLdconfig(filepath.Join(libDir, "missing", "libatomic.so.1"))
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(HaveLen(1))
	})

	It("leaves node pins below 25, which do not need libatomic, alone", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@24", "npm:cline@latest"})).To(Succeed())
		Expect(plugin.HandleInstall(context.Background(), []string{"node@22.19.0"})).To(Succeed())
		Expect(plugin.HandleInstall(context.Background(), []string{"node@v24.13.0"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("leaves tools other than node alone", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"go@latest"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("does nothing outside Linux", func() {
		restores = append(restores, main.SetTargetOS("darwin"))
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "apt-get")

		Expect(plugin.HandleInstall(context.Background(), []string{"node@lts"})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
})
