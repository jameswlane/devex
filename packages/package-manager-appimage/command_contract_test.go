package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordingLogger struct {
	sdk.Logger
	output bytes.Buffer
}

func (l *recordingLogger) Printf(format string, args ...any) { fmt.Fprintf(&l.output, format, args...) }

type recordingTransport struct {
	calls     []string
	status    int
	getStatus int
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls = append(r.calls, req.Method+" "+req.URL.String())
	status := r.status
	if req.Method == http.MethodGet && r.getStatus != 0 {
		status = r.getStatus
	}
	return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d fixture", status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader("AppImage fixture")), Request: req}, nil
}

var _ = Describe("AppImage operation contracts", func() {
	const url = "https://example.test/tool.AppImage"
	var plugin *AppimagePlugin
	var recorder *sdktest.Recorder
	var transport *recordingTransport
	var home string
	var logger *recordingLogger
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "AppImage")
		plugin = NewAppimagePlugin()
		logger = &recordingLogger{Logger: sdk.NewDefaultLogger(true)}
		plugin.logger = logger
		home = os.Getenv("HOME")
		transport = &recordingTransport{status: http.StatusOK}
		previous := http.DefaultTransport
		http.DefaultTransport = transport
		DeferCleanup(func() { http.DefaultTransport = previous })
	})
	AfterEach(func() {
		// These operations use Go HTTP/filesystem APIs and never invoke a manager or sudo.
		Expect(recorder.Calls("AppImage")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	writeBinary := func(dir, name string, mode os.FileMode) string {
		path := filepath.Join(home, dir, name)
		Expect(os.MkdirAll(filepath.Dir(path), 0700)).To(Succeed())
		Expect(os.WriteFile(path, []byte("fixture"), mode)).To(Succeed())
		return path
	}
	It("has a handler contract for every advertised command", func() {
		var advertised []string
		for _, c := range plugin.Info().Commands {
			advertised = append(advertised, c.Name)
		}
		Expect(advertised).To(ConsistOf("install", "remove", "list", "is-installed", "validate-url"))
	})
	for _, location := range []string{"gui", "cli"} {
		It("installs in the user "+location+" location without confirmation", func() {
			Expect(plugin.Execute("install", []string{url, "tool", "--" + location})).To(Succeed())
			dir := "Applications"
			if location == "cli" {
				dir = filepath.Join(".local", "bin")
			}
			binary := filepath.Join(home, dir, "tool")
			data, err := os.ReadFile(binary)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("AppImage fixture"))
			info, err := os.Stat(binary)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0755)))
			desktop := filepath.Join(home, ".local", "share", "applications", "tool.desktop")
			if location == "gui" {
				data, err = os.ReadFile(desktop)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(ContainSubstring("Exec=" + binary + "\n"))
			} else {
				_, err = os.Stat(desktop)
				Expect(os.IsNotExist(err)).To(BeTrue())
			}
			Expect(transport.calls).To(Equal([]string{"HEAD " + url, "GET " + url}))
		})
	}
	It("uses the GUI location by default", func() {
		Expect(plugin.Execute("install", []string{url, "tool"})).To(Succeed())
		_, err := os.Stat(filepath.Join(home, "Applications", "tool"))
		Expect(err).NotTo(HaveOccurred())
	})
	It("skips already installed executables without HTTP requests", func() {
		writeBinary("Applications", "tool", 0755)
		Expect(plugin.Execute("install", []string{url, "tool"})).To(Succeed())
		Expect(transport.calls).To(BeEmpty())
	})
	It("propagates URL accessibility failures during install", func() {
		transport.status = http.StatusNotFound
		Expect(plugin.Execute("install", []string{url, "tool"})).To(MatchError(ContainSubstring("URL returned status 404")))
		Expect(transport.calls).To(Equal([]string{"HEAD " + url}))
	})
	It("propagates download failures after successful URL validation", func() {
		transport.getStatus = http.StatusInternalServerError
		Expect(plugin.Execute("install", []string{url, "tool"})).To(MatchError(ContainSubstring("failed to download AppImage: bad status: 500")))
		Expect(transport.calls).To(Equal([]string{"HEAD " + url, "GET " + url}))
	})

	It("propagates file creation failures during install", func() {
		writeBinary(".", "Applications", 0600)
		Expect(plugin.Execute("install", []string{url, "tool"})).To(MatchError(ContainSubstring("failed to create installation directory")))
	})
	It("removes both installed locations and the desktop entry", func() {
		paths := []string{writeBinary("Applications", "tool", 0755), writeBinary(filepath.Join(".local", "bin"), "tool", 0755), writeBinary(filepath.Join(".local", "share", "applications"), "tool.desktop", 0644)}
		Expect(plugin.Execute("remove", []string{"tool"})).To(Succeed())
		for _, path := range paths {
			_, err := os.Stat(path)
			Expect(os.IsNotExist(err)).To(BeTrue())
		}
	})
	It("lists both user directories without modifying their contents", func() {
		gui := writeBinary("Applications", "gui-tool", 0755)
		cli := writeBinary(filepath.Join(".local", "bin"), "cli-tool", 0755)
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(logger.output.String()).To(ContainSubstring("gui-tool"))
		Expect(logger.output.String()).To(ContainSubstring("cli-tool"))
		for _, path := range []string{gui, cli} {
			_, err := os.Stat(path)
			Expect(err).NotTo(HaveOccurred())
		}
	})
	It("validates URLs with HEAD requests only", func() {
		Expect(plugin.Execute("validate-url", []string{url, url + "?version=2"})).To(Succeed())
		Expect(transport.calls).To(Equal([]string{"HEAD " + url, "HEAD " + url + "?version=2"}))
	})
	It("records the current warning-only URL validation failure behavior", func() {
		transport.status = http.StatusForbidden
		Expect(plugin.Execute("validate-url", []string{url})).To(Succeed())
		Expect(transport.calls).To(Equal([]string{"HEAD " + url}))
	})
	It("records warning-only directory read failures from list", func() {
		writeBinary(".", "Applications", 0600)
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(logger.output.String()).To(ContainSubstring("Error reading directory"))
	})
	PIt("propagates list errors: directory read failures currently return success", func() {
		writeBinary(".", "Applications", 0600)
		Expect(plugin.Execute("list", nil)).To(HaveOccurred())
	})
	It("records warning-only filesystem failures from remove", func() {
		path := filepath.Join(home, "Applications", "tool")
		Expect(os.MkdirAll(path, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(path, "child"), []byte("fixture"), 0600)).To(Succeed())
		Expect(plugin.Execute("remove", []string{"tool"})).To(Succeed())
		_, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
	})
	PIt("propagates remove errors: failed removal of an installed path currently returns success", func() {
		path := filepath.Join(home, "Applications", "tool")
		Expect(os.MkdirAll(path, 0755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(path, "child"), []byte("fixture"), 0600)).To(Succeed())
		Expect(plugin.Execute("remove", []string{"tool"})).To(HaveOccurred())
	})
	PIt("propagates validate-url errors: inaccessible URLs currently return success", func() {
		transport.status = http.StatusForbidden
		Expect(plugin.Execute("validate-url", []string{url})).To(HaveOccurred())
	})

	for _, fixture := range []struct {
		name      string
		dir       string
		mode      os.FileMode
		installed bool
	}{
		{"GUI executable", "Applications", 0755, true},
		{"CLI executable", filepath.Join(".local", "bin"), 0755, true},
		{"non-executable", "Applications", 0644, false},
		{"absent", "", 0, false},
	} {
		It("returns the advertised installed exit status for "+fixture.name, func() {
			if fixture.dir != "" {
				writeBinary(fixture.dir, "tool", fixture.mode)
			}
			executable, err := os.Executable()
			Expect(err).NotTo(HaveOccurred())
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAppimagePlugin$")
			cmd.Env = append(os.Environ(), "DEVEX_APPIMAGE_STATUS_HELPER=tool")
			output, err := cmd.CombinedOutput()
			if fixture.installed {
				Expect(err).NotTo(HaveOccurred(), string(output))
			} else {
				var exitErr *exec.ExitError
				Expect(err).To(BeAssignableToTypeOf(exitErr), string(output))
				Expect(err.(*exec.ExitError).ExitCode()).To(Equal(sdk.ExitCodeNotInstalled))
			}
			Expect(string(output)).To(ContainSubstring("tool"))
		})
	}
	It("returns a failed lookup as an error, not as absence", func() {
		if os.Geteuid() == 0 {
			Skip("root can traverse any directory")
		}
		apps := filepath.Join(home, "Applications")
		Expect(os.MkdirAll(apps, 0o755)).To(Succeed())
		Expect(os.Chmod(apps, 0)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(apps, 0o755)).To(Succeed()) })

		err := plugin.Execute("is-installed", []string{"tool"})

		Expect(err).To(MatchError(ContainSubstring("permission denied")))
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
	})
	It("reports installed when the other location confirms it despite a lookup error", func() {
		if os.Geteuid() == 0 {
			Skip("root can traverse any directory")
		}
		apps := filepath.Join(home, "Applications")
		Expect(os.MkdirAll(apps, 0o755)).To(Succeed())
		writeBinary(filepath.Join(".local", "bin"), "tool", 0o755)
		Expect(os.Chmod(apps, 0)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(apps, 0o755)).To(Succeed()) })

		Expect(plugin.Execute("is-installed", []string{"tool"})).To(Succeed())
	})
	It("rejects unknown commands", func() { Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown")) })
	for _, command := range []string{"install", "remove", "is-installed", "validate-url"} {
		It("rejects missing arguments for "+command, func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(transport.calls).To(BeEmpty())
		})
	}
	It("requires both install arguments", func() {
		Expect(plugin.Execute("install", []string{url})).To(HaveOccurred())
		Expect(transport.calls).To(BeEmpty())
	})
})
