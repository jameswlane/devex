package agentsandbox_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fakeProbeConfig struct {
	goos        string
	files       map[string]string
	existing    map[string]bool
	bins        map[string]string
	runOut      string
	runErr      error
	runCommands []string
}

func (f *fakeProbeConfig) probe() agentsandbox.Probe {
	fixedTime := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	return agentsandbox.Probe{
		GOOS: f.goos,
		Now:  func() time.Time { return fixedTime },
		ReadFile: func(path string) ([]byte, error) {
			if content, ok := f.files[path]; ok {
				return []byte(content), nil
			}
			return nil, os.ErrNotExist
		},
		LookPath: func(name string) (string, error) {
			if path, ok := f.bins[name]; ok {
				return path, nil
			}
			return "", exec.ErrNotFound
		},
		Run: func(_ context.Context, name string, _ ...string) ([]byte, error) {
			f.runCommands = append(f.runCommands, name)
			return []byte(f.runOut), f.runErr
		},
		Stat: func(path string) (os.FileInfo, error) {
			if f.existing[path] {
				return nil, nil
			}
			if _, ok := f.files[path]; ok {
				return nil, nil
			}
			return nil, os.ErrNotExist
		},
		Glob: func(pattern string) ([]string, error) {
			var matches []string
			for path := range f.files {
				if matched, err := pathpkg.Match(pattern, path); err == nil && matched {
					matches = append(matches, path)
				}
			}
			sort.Strings(matches)
			return matches, nil
		},
	}
}

var _ = Describe("Agent sandbox prerequisite check", func() {
	It("reports restricted with the extra-profiles remediation first on Ubuntu 24.04 when extra-profiles exists", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/etc/os-release": "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				"/proc/sys/user/max_user_namespaces":                     "63218\n",
			},
			existing: map[string]bool{
				"/usr/share/apparmor/extra-profiles/bwrap-userns-restrict": true,
			},
			runOut: "bwrap: setting up uid map: Permission denied\n",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(report.UsernsRestricted).To(BeTrue())
		Expect(report.BwrapWorks).To(BeFalse())
		Expect(report.BwrapError).To(ContainSubstring("Permission denied"))
		Expect(report.AppArmorBwrapProfile).To(BeEmpty())
		Expect(report.Remediation).To(HaveLen(2))
		Expect(report.Remediation[0]).To(And(
			ContainSubstring("sudo cp /usr/share/apparmor/extra-profiles/bwrap-userns-restrict /etc/apparmor.d/"),
			ContainSubstring("sudo apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict"),
		))
		Expect(report.Remediation[1]).To(And(
			ContainSubstring("Not recommended"),
			ContainSubstring("sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0"),
		))
		Expect(fake.runCommands).To(Equal([]string{"/usr/bin/bwrap"}))
	})

	It("reports restricted with the sudo tee AppArmor profile remediation when extra-profiles is absent", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/etc/os-release": "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
			},
			runOut: "bwrap: setting up uid map: Permission denied\n",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(report.Remediation).To(HaveLen(2))
		Expect(report.Remediation[0]).To(And(
			ContainSubstring("sudo tee /etc/apparmor.d/devex-bwrap"),
			ContainSubstring("abi <abi/4.0>,"),
			ContainSubstring("profile devex-bwrap \"/usr/bin/bwrap\" flags=(unconfined)"),
			ContainSubstring("sudo apparmor_parser -r /etc/apparmor.d/devex-bwrap"),
		))
		// The command is one pasteable line under its header (no heredoc).
		Expect(strings.Split(report.Remediation[0], "\n")).To(HaveLen(2))
		Expect(report.Remediation[1]).To(ContainSubstring("sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0"))
		Expect(fake.runCommands).To(Equal([]string{"/usr/bin/bwrap"}))
	})

	It("loads an existing bwrap profile instead of writing a new one when bwrap still fails", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/etc/os-release": "ID=ubuntu\nVERSION_ID=\"24.04\"\n",
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				"/etc/apparmor.d/bwrap":                                  "profile bwrap /usr/bin/bwrap flags=(unconfined) {\n  userns,\n}\n",
			},
			existing: map[string]bool{"/usr/share/apparmor/extra-profiles/bwrap-userns-restrict": true},
			runOut:   "bwrap: setting up uid map: Permission denied\n",
			runErr:   errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(report.Remediation[0]).To(ContainSubstring("sudo apparmor_parser -r '/etc/apparmor.d/bwrap'"))
		Expect(strings.Join(report.Remediation, "\n")).NotTo(Or(ContainSubstring("tee"), ContainSubstring("cp ")))
	})

	It("writes the minimal profile for a bwrap outside /usr/bin and never offers Ubuntu's /usr/bin profile", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/local/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				// Ubuntu's profile covers /usr/bin/bwrap only.
				"/etc/apparmor.d/bwrap-userns-restrict": "profile bwrap /usr/bin/bwrap flags=(attach_disconnected) {\n}\n",
			},
			existing: map[string]bool{"/usr/share/apparmor/extra-profiles/bwrap-userns-restrict": true},
			runOut:   "bwrap: setting up uid map: Permission denied\n",
			runErr:   errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(report.Remediation[0]).To(ContainSubstring("profile devex-bwrap \"/usr/local/bin/bwrap\" flags=(unconfined)"))
		Expect(report.Remediation[0]).NotTo(ContainSubstring("extra-profiles"))
		Expect(report.AppArmorBwrapProfile).To(BeEmpty())
	})

	It("quotes a bwrap path with spaces and apostrophes in the generated profile command", func() {
		bwrapPath := "/opt/it's tools/bwrap"
		fake := &fakeProbeConfig{
			goos:   "linux",
			bins:   map[string]string{"bwrap": bwrapPath, "socat": "/usr/bin/socat"},
			files:  map[string]string{"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n"},
			runOut: "bwrap: setting up uid map: Permission denied\n",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())
		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		command := strings.SplitN(report.Remediation[0], "\n", 2)[1]
		printOnly, _, found := strings.Cut(strings.TrimSpace(command), " | sudo tee ")
		Expect(found).To(BeTrue())

		// Run only the printf part through a real shell to see the profile it writes.
		out, err := exec.CommandContext(context.Background(), "sh", "-c", printOnly).Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(out)).To(ContainSubstring(`profile devex-bwrap "/opt/it's tools/bwrap" flags=(unconfined) {` + "\n"))
	})

	It("does not count a comment or a longer path as a profile for bwrap", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				"/etc/apparmor.d/notes":                                  "# see /usr/bin/bwrap\nprofile other /usr/bin/bwrap2 {\n}\n",
			},
			runOut: "bwrap: setting up uid map: Permission denied\n",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.AppArmorBwrapProfile).To(BeEmpty())
		Expect(report.Remediation[0]).To(ContainSubstring("sudo tee /etc/apparmor.d/devex-bwrap"))
	})

	It("matches only profile attachments, including DevEx's quoted and escaped one", func() {
		check := func(bwrapPath, profile string) string {
			fake := &fakeProbeConfig{
				goos: "linux",
				bins: map[string]string{"bwrap": bwrapPath, "socat": "/usr/bin/socat"},
				files: map[string]string{
					"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
					"/etc/apparmor.d/candidate":                              profile,
				},
				runErr: errors.New("exit status 1"),
			}
			return agentsandbox.Check(context.Background(), fake.probe()).AppArmorBwrapProfile
		}
		Expect(check("/usr/bin/bwrap", "profile other /usr/bin/other {\n  /usr/bin/bwrap ix,\n}\n")).To(BeEmpty())
		Expect(check("/usr/bin/bwrap", "profile other /usr/bin/other { # /usr/bin/bwrap\n}\n")).To(BeEmpty())
		Expect(check("/usr/bin/bwrap", "/usr/bin/bwrap flags=(unconfined) {\n}\n")).To(Equal("/etc/apparmor.d/candidate"))

		// Write DevEx's own profile for an unusual path, then detect it.
		path := "/opt/it's [build]/bw*rap"
		fake := &fakeProbeConfig{
			goos:   "linux",
			bins:   map[string]string{"bwrap": path, "socat": "/usr/bin/socat"},
			files:  map[string]string{"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n"},
			runErr: errors.New("exit status 1"),
		}
		command := strings.SplitN(agentsandbox.Check(context.Background(), fake.probe()).Remediation[0], "\n", 2)[1]
		printOnly, _, _ := strings.Cut(strings.TrimSpace(command), " | sudo tee ")
		generated, err := exec.CommandContext(context.Background(), "sh", "-c", printOnly).Output()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(generated)).To(ContainSubstring(`"/opt/it's \[build\]/bw\*rap"`))
		Expect(check(path, string(generated))).To(Equal("/etc/apparmor.d/candidate"))
	})

	It("reports ok when unrestricted and bwrap works", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/etc/os-release":                    "ID=debian\n",
				"/proc/sys/user/max_user_namespaces": "120000\n",
			},
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusOK))
		Expect(report.UsernsRestricted).To(BeFalse())
		Expect(report.BwrapWorks).To(BeTrue())
		Expect(report.BwrapError).To(BeEmpty())
		Expect(report.Remediation).To(BeEmpty())
		Expect(report.MaxUserNamespaces).NotTo(BeNil())
		Expect(*report.MaxUserNamespaces).To(Equal(120000))
		Expect(fake.runCommands).To(Equal([]string{"/usr/bin/bwrap"}))
	})

	It("reports ok when userns is restricted but an AppArmor profile allows bwrap", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/etc/os-release": "ID=ubuntu\nVERSION_ID=\"25.04\"\n",
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				"/etc/apparmor.d/bwrap-userns-restrict":                  "profile bwrap /usr/bin/bwrap { userns, }\n",
			},
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusOK))
		Expect(report.UsernsRestricted).To(BeTrue())
		Expect(report.AppArmorBwrapProfile).To(Equal("/etc/apparmor.d/bwrap-userns-restrict"))
		Expect(report.BwrapWorks).To(BeTrue())
		Expect(report.Remediation).To(BeEmpty())
	})

	It("discovers a custom AppArmor profile in /etc/apparmor.d referencing /usr/bin/bwrap", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
				"/etc/apparmor.d/custom-bwrap":                           "profile bwrap /usr/bin/bwrap flags=(unconfined) { userns, }\n",
			},
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusOK))
		Expect(report.AppArmorBwrapProfile).To(Equal("/etc/apparmor.d/custom-bwrap"))
	})

	DescribeTable("reports missing when bwrap is absent and never executes bwrap",
		func(osRelease, wantInstallCmd string) {
			fake := &fakeProbeConfig{
				goos: "linux",
				bins: map[string]string{},
				files: map[string]string{
					"/etc/os-release": osRelease,
				},
			}

			report := agentsandbox.Check(context.Background(), fake.probe())

			Expect(report.Status).To(Equal(agentsandbox.StatusMissing))
			Expect(report.BwrapPath).To(BeEmpty())
			Expect(report.BwrapWorks).To(BeFalse())
			Expect(fake.runCommands).To(BeEmpty())
			Expect(report.Remediation).To(Equal([]string{
				wantInstallCmd,
				"devex install Bubblewrap socat",
			}))
		},
		Entry("Ubuntu/apt", "ID=ubuntu\n", "sudo apt-get install -y bubblewrap socat"),
		Entry("Fedora/dnf", "ID=fedora\n", "sudo dnf install -y bubblewrap socat"),
		Entry("Arch/pacman", "ID=arch\n", "sudo pacman -S --needed bubblewrap socat"),
		Entry("openSUSE/zypper", "ID=opensuse-tumbleweed\n", "sudo zypper install -y bubblewrap socat"),
	)

	It("reports missing when bwrap works but socat is absent", func() {
		fake := &fakeProbeConfig{
			goos:  "linux",
			bins:  map[string]string{"bwrap": "/usr/bin/bwrap"},
			files: map[string]string{"/etc/os-release": "ID=ubuntu\n"},
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusMissing))
		Expect(report.BwrapWorks).To(BeTrue())
		Expect(report.Remediation).To(Equal([]string{
			"sudo apt-get install -y socat",
			"devex install socat",
		}))
	})

	It("reports ok without socat when the check does not require it", func() {
		fake := &fakeProbeConfig{
			goos:  "linux",
			bins:  map[string]string{"bwrap": "/usr/bin/bwrap"},
			files: map[string]string{"/etc/os-release": "ID=ubuntu\n"},
		}
		probe := fake.probe()
		probe.SocatNotRequired = true

		report := agentsandbox.Check(context.Background(), probe)

		Expect(report.Status).To(Equal(agentsandbox.StatusOK))
		Expect(report.SocatPath).To(BeEmpty())
		Expect(report.Remediation).To(BeEmpty())
	})

	It("reports failed with bwrap error when unrestricted but bwrap still fails", func() {
		fake := &fakeProbeConfig{
			goos:   "linux",
			bins:   map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files:  map[string]string{"/etc/os-release": "ID=ubuntu\n"},
			runOut: "bwrap: Creating new namespace failed: Operation not permitted\n",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusFailed))
		Expect(report.UsernsRestricted).To(BeFalse())
		Expect(report.BwrapWorks).To(BeFalse())
		Expect(report.BwrapError).To(ContainSubstring("Operation not permitted"))
		Expect(report.Remediation).NotTo(BeEmpty())
	})

	It("reports restricted when Debian unprivileged_userns_clone is 0 and bwrap fails", func() {
		fake := &fakeProbeConfig{
			goos: "linux",
			bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
			files: map[string]string{
				"/proc/sys/kernel/unprivileged_userns_clone": "0\n",
			},
			runOut: "bwrap: Creating new namespace failed: Operation not permitted",
			runErr: errors.New("exit status 1"),
		}

		report := agentsandbox.Check(context.Background(), fake.probe())

		Expect(report.Status).To(Equal(agentsandbox.StatusRestricted))
		Expect(report.UnprivilegedUsernsClone).NotTo(BeNil())
		Expect(*report.UnprivilegedUsernsClone).To(Equal(0))
		Expect(report.Remediation).To(ContainElement(ContainSubstring("kernel.unprivileged_userns_clone=1")))
	})

	It("returns unsupported on non-Linux OSes without running any probes", func() {
		readCalled := false
		lookCalled := false
		runCalled := false
		probe := agentsandbox.Probe{
			GOOS: "darwin",
			ReadFile: func(string) ([]byte, error) {
				readCalled = true
				return nil, os.ErrNotExist
			},
			LookPath: func(string) (string, error) {
				lookCalled = true
				return "", exec.ErrNotFound
			},
			Run: func(context.Context, string, ...string) ([]byte, error) {
				runCalled = true
				return nil, nil
			},
		}

		report := agentsandbox.Check(context.Background(), probe)

		Expect(report.Status).To(Equal(agentsandbox.StatusUnsupported))
		Expect(readCalled).To(BeFalse())
		Expect(lookCalled).To(BeFalse())
		Expect(runCalled).To(BeFalse())
	})

	It("never executes sysctl, apparmor_parser, or any command other than bwrap", func() {
		scenarios := []*fakeProbeConfig{
			{
				goos: "linux",
				bins: map[string]string{"bwrap": "/usr/bin/bwrap", "socat": "/usr/bin/socat"},
				files: map[string]string{
					"/proc/sys/kernel/apparmor_restrict_unprivileged_userns": "1\n",
					"/proc/sys/kernel/unprivileged_userns_clone":             "0\n",
					"/proc/sys/user/max_user_namespaces":                     "0\n",
				},
				existing: map[string]bool{
					"/usr/share/apparmor/extra-profiles/bwrap-userns-restrict": true,
				},
				runOut: "bwrap: setting up uid map: Permission denied",
				runErr: errors.New("exit status 1"),
			},
			{
				goos: "linux",
				bins: map[string]string{},
			},
		}

		for _, sc := range scenarios {
			_ = agentsandbox.Check(context.Background(), sc.probe())
			for _, cmd := range sc.runCommands {
				Expect(filepath.Base(cmd)).To(Equal("bwrap"))
			}
		}
	})

	It("saves and reloads reports atomically with 0600 permissions", func() {
		path := filepath.Join(GinkgoT().TempDir(), "state", agentsandbox.ReportFileName)
		maxNS := 65536
		want := agentsandbox.Report{
			BwrapPath:         "/usr/bin/bwrap",
			SocatPath:         "/usr/bin/socat",
			MaxUserNamespaces: &maxNS,
			BwrapWorks:        true,
			Status:            agentsandbox.StatusOK,
			CheckedAt:         time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
			Remediation:       []string{},
		}

		Expect(agentsandbox.SaveReportToPath(path, want)).To(Succeed())

		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		if runtime.GOOS != "windows" {
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		}

		got, err := agentsandbox.LoadReportFromPath(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(Equal(want))
	})
})
