package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Validation startup", func() {
	It("returns typed strict diagnostics without creating application state", func() {
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(GinkgoT().Context(), "go", "build", "-o", binary, ".")
		Expect(build.Run()).To(Succeed())
		home := GinkgoT().TempDir()
		configDir := filepath.Join(home, ".local/share/devex/config/applications")
		Expect(os.MkdirAll(configDir, 0750)).To(Succeed())
		path := filepath.Join(configDir, "bad.yaml")
		Expect(os.WriteFile(path, []byte("name: broken\nlinux:\n  install_method: aptt\n  install_command: broken\n"), 0600)).To(Succeed())
		cmd := exec.CommandContext(GinkgoT().Context(), binary, "config", "validate", "--strict", "--format", "json")
		cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local/share"), "XDG_STATE_HOME="+filepath.Join(home, ".local/state"), "DEVEX_TEAM_CONFIG_DIR=")
		output, err := cmd.Output()
		Expect(err).To(HaveOccurred())
		var report struct {
			Valid  bool `json:"valid"`
			Errors int  `json:"errors"`
		}
		Expect(json.Unmarshal(output, &report)).To(Succeed(), "%s", output)
		Expect(report.Valid).To(BeFalse())
		Expect(report.Errors).To(BeNumerically(">", 0))
		Expect(filepath.Join(home, ".local/state")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
	})
})
