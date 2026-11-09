package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Presentation startup subprocesses", func() {
	It("launches no subprocesses for help, version or shell completion", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable recorder")
		}
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(GinkgoT().Context(), "go", "build", "-o", binary, ".")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "%s", output)
		recorder := GinkgoT().TempDir()
		record := filepath.Join(GinkgoT().TempDir(), "calls")
		Expect(os.WriteFile(record, nil, 0600)).To(Succeed())
		// Replace every executable visible on the host, including mise shims, so
		// unexpected commands are recorded too, rather than testing a fixed list.
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			entries, readErr := os.ReadDir(dir)
			if readErr != nil {
				continue
			}
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				script := "#!/bin/sh\nprintf '%s\\n' \"$0 $*\" >> \"$DEVEX_PROBE_RECORD\"\n"
				Expect(os.WriteFile(filepath.Join(recorder, entry.Name()), []byte(script), 0700)).To(Succeed())
			}
		}
		for _, args := range [][]string{{"--help"}, {"install", "--help"}, {"--version"}, {"help", "install"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"completion", "powershell"}, {"__complete", "install", ""}, {"__completeNoDesc", "install", ""}} {
			home := GinkgoT().TempDir()
			cmd := exec.CommandContext(GinkgoT().Context(), binary, args...)
			cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+recorder, "DEVEX_PROBE_RECORD="+record, "XDG_CONFIG_HOME="+home, "XDG_DATA_HOME="+home, "XDG_STATE_HOME="+home, "CI=true")
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "%v: %s", args, output)
			calls, err := os.ReadFile(record)
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(string(calls))).To(BeEmpty(), "%v launched subprocesses", args)
			// The same presentation route must also work without dependencies.
			cmd = exec.CommandContext(GinkgoT().Context(), binary, args...)
			cmd.Env = append(os.Environ(), "HOME="+home, "PATH=", "CI=true")
			output, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "%v without tools: %s", args, output)
		}
	})
})

var _ = Describe("Presentation route detection", func() {
	It("respects flag values, command words and the flag terminator", func() {
		Expect(presentationCommand([]string{"--config", "help", "install"})).To(BeFalse())
		Expect(presentationCommand([]string{"install", "--config", "--help"})).To(BeFalse())
		Expect(presentationCommand([]string{"install", "--", "--help"})).To(BeFalse())
		Expect(presentationCommand([]string{"install", "help"})).To(BeFalse())
		Expect(presentationCommand([]string{"install", "--config", "settings.yaml", "--help"})).To(BeTrue())
		Expect(presentationCommand([]string{"--config", "settings.yaml", "help", "install"})).To(BeTrue())
	})
})
