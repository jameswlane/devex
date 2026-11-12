package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent API startup", func() {
	It("keeps read JSON valid without creating configuration or state", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		goBin, env := isolatedGoRunEnv(home)
		binary := filepath.Join(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(ctx, goBin, "build", "-o", binary, "main.go")
		build.Env = env
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		before := startupSnapshot(home)
		env = append(env, "DEVEX_CONFIG_DIR="+filepath.Join(home, "config"), "DEVEX_STATE_DIR="+filepath.Join(home, "state"), "DEVEX_CACHE_DIR="+filepath.Join(home, "cache"), "DEVEX_DATA_DIR="+filepath.Join(home, "data"), "DEVEX_DISABLE_UPDATE_CHECK=true")
		for _, args := range [][]string{{"platform", "--format", "json"}, {"inventory", "--format", "json"}, {"config", "effective", "--format", "json"}, {"config", "show", "--format", "json"}, {"list", "available", "--format", "json"}, {"status", "--app", "missing", "--format", "json"}} {
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Env = env
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			output, err := cmd.Output()
			if args[0] == "status" {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred(), "%v: %s", args, stderr.String())
			}
			Expect(json.Valid(output)).To(BeTrue(), "%v: %s", args, output)
		}
		Expect(startupSnapshot(home)).To(Equal(before))
	})
})

func startupSnapshot(root string) map[string]string {
	entries := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entries[path] = fmt.Sprintf("%v:%d:%v", info.Mode(), info.Size(), info.ModTime())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	return entries
}
