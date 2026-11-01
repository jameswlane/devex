//go:build e2e

package e2e_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Canonical configuration layout", func() {
	It("initializes a fresh HOME and lists shipped applications", func() {
		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, sandbox.binary, time.Minute, nil, "init", "--profile", "backend", "--interactive=false")
		Expect(err).NotTo(HaveOccurred(), "devex init failed:\n%s", output)
		Expect(filepath.Join(sandbox.paths(home, nil).Defaults(), "applications/development/git.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(sandbox.paths(home, nil).Defaults(), "applications.yaml")).NotTo(BeAnExistingFile())

		output, err = sandbox.run(home, sandbox.binary, time.Minute, nil, "list", "available")
		Expect(err).NotTo(HaveOccurred(), "devex list available failed:\n%s", output)
		Expect(output).To(ContainSubstring("git"))
		Expect(output).To(ContainSubstring("PostgreSQL"))
		Expect(output).NotTo(ContainSubstring("No applications found"))

		output, err = sandbox.run(home, sandbox.binary, time.Minute, nil, "config", "show", "--format", "yaml")
		Expect(err).NotTo(HaveOccurred(), "devex config show failed:\n%s", output)
		Expect(output).To(ContainSubstring(filepath.Join(sandbox.paths(home, nil).Defaults(), "applications")))

		output, err = sandbox.run(home, sandbox.binary, time.Minute, nil, "config", "validate", "--strict")
		Expect(err).NotTo(HaveOccurred(), "devex config validate failed:\n%s", output)
		Expect(output).To(ContainSubstring("All configuration files are valid!"))
		Expect(output).NotTo(ContainSubstring("Warning:"))

		editor := filepath.Join(home, "record-editor")
		Expect(os.WriteFile(editor, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/editor-files.txt\"\n"), 0700)).To(Succeed())
		output, err = sandbox.run(home, sandbox.binary, time.Minute, nil, "config", "edit", "applications", "--editor", editor)
		Expect(err).NotTo(HaveOccurred(), "devex config edit failed:\n%s", output)
		files, err := os.ReadFile(filepath.Join(home, "editor-files.txt"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(files)).To(ContainSubstring("applications/development/git.yaml"))
		Expect(string(files)).To(ContainSubstring("applications/databases/postgresql.yaml"))

		// Scripts rely on `status --format json`: stdout must be valid JSON and the
		// exit code must match the reported state (0 installed, 1 missing or error).
		stdout, stderr, err := sandbox.runWithStderr(home, sandbox.binary, time.Minute, nil, "status", "--app", "git", "--format", "json")
		var results []map[string]any
		Expect(json.Unmarshal([]byte(stdout), &results)).To(Succeed(), "status JSON must parse:\nstdout: %s\nstderr: %s", stdout, stderr)
		Expect(results).To(HaveLen(1))
		Expect(results[0]).To(HaveKeyWithValue("name", "git"))
		switch results[0]["status"] {
		case "not_installed", "error":
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue(), "a missing or failed app must exit non-zero: %v", err)
			Expect(exitErr.ExitCode()).To(Equal(1))
		default:
			Expect(err).NotTo(HaveOccurred(), "status %v must exit 0:\nstderr: %s", results[0]["status"], stderr)
		}
	})
})
