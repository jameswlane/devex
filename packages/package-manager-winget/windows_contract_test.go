//go:build windows

package main_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type winCallRecord struct {
	Args []string `json:"args"`
}

type winScriptedResponse struct {
	Args     []string `json:"args"`
	ExitCode uint32   `json:"exit_code"`
	Stdout   string   `json:"stdout,omitempty"`
	Stderr   string   `json:"stderr,omitempty"`
}

type winContract struct {
	name       string
	command    string
	args       []string
	nativeArgv [][]string
	elevated   bool
}

var (
	winAgree   = []string{"--accept-source-agreements", "--disable-interactivity"}
	winChange  = []string{"--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	winRemoval = []string{"--silent", "--accept-source-agreements", "--disable-interactivity"}
)

func winArgv(parts ...[]string) []string {
	var out []string
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

var winContracts = []winContract{
	{name: "install", command: "install", args: []string{"Git.Git"}, nativeArgv: [][]string{
		winArgv([]string{"install", "--id", "Git.Git", "--exact"}, winChange, []string{"--no-upgrade"}),
	}},
	{name: "install of several packages", command: "install", args: []string{"Git.Git", "sharkdp.bat"}, nativeArgv: [][]string{
		winArgv([]string{"install", "--id", "Git.Git", "--exact"}, winChange, []string{"--no-upgrade"}),
		winArgv([]string{"install", "--id", "sharkdp.bat", "--exact"}, winChange, []string{"--no-upgrade"}),
	}},
	{name: "user-scope install", command: "install", args: []string{"--scope=user", "Git.Git"}, nativeArgv: [][]string{
		winArgv([]string{"install", "--id", "Git.Git", "--exact"}, winChange, []string{"--no-upgrade", "--scope", "user"}),
	}},
	{name: "machine-scope install", command: "install", args: []string{"--scope", "machine", "Docker.DockerDesktop"}, elevated: true, nativeArgv: [][]string{
		winArgv([]string{"install", "--id", "Docker.DockerDesktop", "--exact"}, winChange, []string{"--no-upgrade", "--scope", "machine"}),
	}},
	{name: "Store install", command: "install", args: []string{"9NBLGGH4NNS1"}, nativeArgv: [][]string{
		winArgv([]string{"install", "--id", "9NBLGGH4NNS1", "--exact"}, winChange, []string{"--no-upgrade"}),
	}},
	{name: "remove", command: "remove", args: []string{"Git.Git"}, nativeArgv: [][]string{
		winArgv([]string{"uninstall", "--id", "Git.Git", "--exact"}, winRemoval),
	}},
	{name: "machine-scope remove", command: "remove", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		winArgv([]string{"uninstall", "--id", "Git.Git", "--exact"}, winRemoval, []string{"--scope", "machine"}),
	}},
	{name: "upgrade", command: "upgrade", args: []string{"Git.Git"}, nativeArgv: [][]string{
		winArgv([]string{"upgrade", "--id", "Git.Git", "--exact"}, winChange),
	}},
	{name: "machine-scope upgrade", command: "upgrade", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		winArgv([]string{"upgrade", "--id", "Git.Git", "--exact"}, winChange, []string{"--scope", "machine"}),
	}},
	{name: "update", command: "update", nativeArgv: [][]string{{"source", "update", "--disable-interactivity"}}},
	{name: "search", command: "search", args: []string{"visual", "studio"}, nativeArgv: [][]string{
		winArgv([]string{"search", "--query", "visual studio"}, winAgree),
	}},
	{name: "list", command: "list", nativeArgv: [][]string{winArgv([]string{"list"}, winAgree)}},
	{name: "is-installed", command: "is-installed", args: []string{"Git.Git"}, nativeArgv: [][]string{
		winArgv([]string{"list", "--id", "Git.Git", "--exact"}, winAgree),
	}},
	{name: "machine-scope is-installed", command: "is-installed", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		winArgv([]string{"list", "--id", "Git.Git", "--exact", "--scope", "machine"}, winAgree),
	}},
}

var _ = Describe("Windows winget recorder contracts", Ordered, func() {
	var (
		fakeBinDir       string
		pluginBinDir     string
		logPath          string
		responsesPath    string
		responses        []winScriptedResponse
		elevated         bool
		originalResolver func() sdk.PrivilegeResolver
		plugin           *main.WingetPlugin
	)

	BeforeAll(func() {
		buildRoot, err := os.MkdirTemp("", "devex-winget-win-contracts-*")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _ = os.RemoveAll(buildRoot) })

		fakeBinDir = filepath.Join(buildRoot, "bin")
		pluginBinDir = filepath.Join(buildRoot, "plugins")
		Expect(os.MkdirAll(fakeBinDir, 0o700)).To(Succeed())
		Expect(os.MkdirAll(pluginBinDir, 0o700)).To(Succeed())

		fakeExe := filepath.Join(fakeBinDir, "winget.exe")
		buildFake := exec.Command("go", "build", "-o", fakeExe, "./testdata/fakewinget")
		out, err := buildFake.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "build fakewinget: %s", string(out))

		pluginExe := filepath.Join(pluginBinDir, "devex-plugin-package-manager-winget.exe")
		buildPlugin := exec.Command("go", "build", "-o", pluginExe, ".")
		out, err = buildPlugin.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "build winget plugin: %s", string(out))
	})

	BeforeEach(func() {
		elevated = false
		responses = nil
		workDir := GinkgoT().TempDir()
		logPath = filepath.Join(workDir, "winget.calls.jsonl")
		responsesPath = filepath.Join(workDir, "winget.responses.json")

		GinkgoT().Setenv("PATH", fakeBinDir)
		GinkgoT().Setenv("DEVEX_FAKE_WINGET_LOG", logPath)
		GinkgoT().Setenv("DEVEX_FAKE_WINGET_RESPONSES", responsesPath)
		GinkgoT().Setenv(sdk.UnattendedEnv, "")
		GinkgoT().Setenv("CI", "")

		originalResolver = sdk.NewPrivilegeResolver
		originalAdapter := sdk.DefaultPrivilegeAdapter
		DeferCleanup(func() {
			sdk.NewPrivilegeResolver = originalResolver
			sdk.DefaultPrivilegeAdapter = originalAdapter
		})

		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				Elevated:    elevated,
				CurrentUser: user.Current,
				LookupUser:  user.Lookup,
				Getenv:      os.Getenv,
			}
		}
		sdk.DefaultPrivilegeAdapter = sdk.NewWindowsPrivilegeAdapter(func() (bool, error) {
			return elevated, nil
		})

		plugin = main.NewWingetPlugin()
	})

	script := func(args []string, exitCode uint32, stdout, stderr string) {
		GinkgoHelper()
		responses = append(responses, winScriptedResponse{
			Args:     args,
			ExitCode: exitCode,
			Stdout:   stdout,
			Stderr:   stderr,
		})
		data, err := json.Marshal(responses)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(responsesPath, data, 0o600)).To(Succeed())
	}

	calls := func() [][]string {
		GinkgoHelper()
		data, err := os.ReadFile(logPath)
		if os.IsNotExist(err) {
			return nil
		}
		Expect(err).NotTo(HaveOccurred())
		var recorded [][]string
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			var rec winCallRecord
			Expect(json.Unmarshal(scanner.Bytes(), &rec)).To(Succeed())
			recorded = append(recorded, rec.Args)
		}
		Expect(scanner.Err()).NotTo(HaveOccurred())
		return recorded
	}

	It("covers every advertised command and exposes scope-aware launch privileges", func() {
		info := plugin.Info()
		covered := map[string]bool{}
		for _, contract := range winContracts {
			covered[contract.command] = true
		}
		names := make([]string, 0, len(info.Commands))
		for _, cmd := range info.Commands {
			names = append(names, cmd.Name)
			Expect(cmd.Privilege).NotTo(BeNil(), cmd.Name)
			Expect(cmd.Privilege.Scope).To(Equal(sdk.ScopeUser), cmd.Name)
		}
		for _, name := range names {
			Expect(covered[name]).To(BeTrue(), "missing contract for %s", name)
		}
		for _, cmd := range []string{"install", "is-installed", "remove", "upgrade"} {
			Expect(sdk.PluginOperationPrivilege(info, []string{cmd, "Git.Git"}).Scope).To(Equal(sdk.ScopeUser), cmd)
			Expect(sdk.PluginOperationPrivilege(info, []string{cmd, "--scope=user", "Git.Git"}).Scope).To(Equal(sdk.ScopeUser), cmd)
			Expect(sdk.PluginOperationPrivilege(info, []string{cmd, "--scope=machine", "Git.Git"}).Scope).To(Equal(sdk.ScopeSystem), cmd)
			Expect(sdk.PluginOperationPrivilege(info, []string{cmd, "--scope", "machine", "Git.Git"}).Scope).To(Equal(sdk.ScopeSystem), cmd)
		}
	})

	for _, contract := range winContracts {
		It(fmt.Sprintf("honors exact native argv and Windows privilege policy for %s", contract.name), func() {
			elevated = contract.elevated
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(calls()).To(Equal(contract.nativeArgv))
		})

		It(fmt.Sprintf("refuses %s when process elevation does not match scope", contract.name), func() {
			elevated = !contract.elevated
			err := plugin.Execute(contract.command, contract.args)
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue(), "expected authorization error, got %v", err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
			Expect(calls()).To(BeEmpty())
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.name), func() {
			elevated = contract.elevated
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			Expect(calls()).To(BeEmpty())
		})
	}

	DescribeTable("rejects invalid identifiers and options before winget runs",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			Expect(calls()).To(BeEmpty())
		},
		Entry("install without identifiers", "install", nil, "no package identifiers specified"),
		Entry("install with only a scope", "install", []string{"--scope=user"}, "no package identifiers specified"),
		Entry("remove without identifiers", "remove", nil, "no package identifiers specified"),
		Entry("upgrade without identifiers", "upgrade", nil, "no package identifiers specified"),
		Entry("is-installed without identifiers", "is-installed", nil, "no package identifiers specified"),
		Entry("install by display name", "install", []string{"git"}, `invalid winget package identifier "git"`),
		Entry("install with a path", "install", []string{`C:\tools\git.exe`}, "invalid winget package identifier"),
		Entry("install with an empty segment", "install", []string{"Git..Git"}, "invalid winget package identifier"),
		Entry("install with a lowercase Store ID", "install", []string{"9nblggh4nns1"}, "invalid winget package identifier"),
		Entry("install with a native flag", "install", []string{"--force", "Git.Git"}, `unsupported option "--force"`),
		Entry("install with an invalid scope", "install", []string{"--scope=global", "Git.Git"}, `--scope must be user or machine, not "global"`),
		Entry("install with conflicting scopes", "install", []string{"--scope=user", "--scope=machine", "Git.Git"}, "conflicting --scope values"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option", "search", []string{"--id"}, "invalid search term"),
		Entry("update with arguments", "update", []string{"Git.Git"}, "update takes no arguments"),
		Entry("list with arguments", "list", []string{"Git.Git"}, "list takes no arguments"),
	)

	Context("real 32-bit Windows HRESULT interpretation", func() {
		install := winArgv([]string{"install", "--id", "Git.Git", "--exact"}, winChange, []string{"--no-upgrade"})
		upgrade := winArgv([]string{"upgrade", "--id", "Git.Git", "--exact"}, winChange)
		remove := winArgv([]string{"uninstall", "--id", "Git.Git", "--exact"}, winRemoval)
		query := func(id string) []string { return winArgv([]string{"list", "--id", id, "--exact"}, winAgree) }

		It("treats PACKAGE_ALREADY_INSTALLED and INSTALL_ALREADY_INSTALLED as success", func() {
			script(install, 0x8A150061, "", "")
			Expect(plugin.Execute("install", []string{"Git.Git", "sharkdp.bat"})).To(Succeed())
			Expect(calls()).To(HaveLen(2))

			responses = nil
			script(install, 0x8A15010D, "", "")
			Expect(plugin.Execute("install", []string{"Git.Git"})).To(Succeed())
		})

		It("treats UPDATE_NOT_APPLICABLE as current on upgrade", func() {
			script(upgrade, 0x8A15002B, "", "")
			Expect(plugin.Execute("upgrade", []string{"Git.Git"})).To(Succeed())
		})

		It("maps declined UAC (0x800704C7) to AuthorizationDenied with the real HRESULT", func() {
			script(install, 0x800704C7, "", "")
			err := plugin.Execute("install", []string{"Git.Git"})
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue(), "expected authorization error, got %v", err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationDenied))
			Expect(uint32(sdk.ExitStatus(err))).To(Equal(uint32(0x800704C7)))
		})

		DescribeTable("adds guidance while preserving real HRESULT exit status",
			func(command string, native []string, hr uint32, message string) {
				script(native, hr, "", "")
				err := plugin.Execute(command, []string{"Git.Git"})
				Expect(err).To(MatchError(ContainSubstring(message)))
				Expect(uint32(sdk.ExitStatus(err))).To(Equal(hr))
			},
			Entry("unknown identifier on install", "install", install, uint32(0x8A150014), "no winget package has the identifier Git.Git"),
			Entry("absent package on remove", "remove", remove, uint32(0x8A150014), "Git.Git is not installed"),
			Entry("absent package on upgrade", "upgrade", upgrade, uint32(0x8A150014), "Git.Git is not installed"),
			Entry("cancelled installer", "install", install, uint32(0x8A15010C), "possibly by a declined UAC prompt"),
			Entry("command requires admin", "remove", remove, uint32(0x8A150019), "requires administrator rights"),
			Entry("installer prohibits elevation", "install", install, uint32(0x8A150056), "cannot run from an elevated terminal"),
			Entry("admin context action prohibited", "install", install, uint32(0x8A15007D), "cannot run from an elevated terminal"),
			Entry("no applicable installer", "install", install, uint32(0x8A150010), "has no installer for this system"),
		)

		It("maps NO_APPLICATIONS_FOUND in is-installed to NotInstalledError", func() {
			script(query("sharkdp.bat"), 0x8A150014, "No installed package found.\n", "")
			err := plugin.Execute("is-installed", []string{"Git.Git", "sharkdp.bat"})
			var absent *sdk.NotInstalledError
			Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
			Expect(absent.Packages).To(Equal([]string{"sharkdp.bat"}))
			Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
		})

		It("runs machine-scope is-installed from an elevated process and returns status", func() {
			elevated = true
			machineInstalled := winArgv([]string{"list", "--id", "Docker.DockerDesktop", "--exact", "--scope", "machine"}, winAgree)
			machineAbsent := winArgv([]string{"list", "--id", "Missing.Tool", "--exact", "--scope", "machine"}, winAgree)
			script(machineInstalled, 0, "Docker Desktop Docker.DockerDesktop 4.40.0\n", "")
			script(machineAbsent, 0x8A150014, "No installed package found.\n", "")

			Expect(plugin.Execute("is-installed", []string{"--scope=machine", "Docker.DockerDesktop"})).To(Succeed())

			err := plugin.Execute("is-installed", []string{"--scope=machine", "Missing.Tool"})
			var absent *sdk.NotInstalledError
			Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
			Expect(absent.Packages).To(Equal([]string{"Missing.Tool"}))
			Expect(calls()).To(Equal([][]string{machineInstalled, machineAbsent}))
		})
	})

	Context("SDK executable launch privilege enforcement", func() {
		It("enforces per-scope launch privilege before starting the plugin binary", func() {
			manager := sdk.NewExecutableManager(pluginBinDir)
			var stdout, stderr bytes.Buffer
			io := sdk.OperationIO{Stdout: &stdout, Stderr: &stderr}

			// Machine scope from a non-elevated process fails before the plugin launches.
			elevated = false
			err := manager.ExecutePluginWithIO(context.Background(), "package-manager-winget", []string{"install", "--scope=machine", "Docker.DockerDesktop"}, io)
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue(), "expected authorization error, got %v", err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
			Expect(calls()).To(BeEmpty())

			// User scope from an elevated process fails before the plugin launches.
			elevated = true
			err = manager.ExecutePluginWithIO(context.Background(), "package-manager-winget", []string{"install", "--scope=user", "Git.Git"}, io)
			auth = nil
			Expect(errors.As(err, &auth)).To(BeTrue(), "expected authorization error, got %v", err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
			Expect(calls()).To(BeEmpty())

			// Launch the compiled plugin binary at the scope matching the real host
			// token (GitHub windows-latest runners are elevated; local terminals are
			// typically non-elevated) so the child process's own token check passes.
			hostElevated := sdk.NewPrivilegeResolver().Elevated
			if originalResolver != nil {
				hostElevated = originalResolver().Elevated
			}
			elevated = hostElevated
			if hostElevated {
				Expect(manager.ExecutePluginWithIO(context.Background(), "package-manager-winget", []string{"install", "--scope=machine", "Docker.DockerDesktop"}, io)).To(Succeed())
				Expect(calls()).To(Equal([][]string{
					winArgv([]string{"install", "--id", "Docker.DockerDesktop", "--exact"}, winChange, []string{"--no-upgrade", "--scope", "machine"}),
				}))
			} else {
				Expect(manager.ExecutePluginWithIO(context.Background(), "package-manager-winget", []string{"install", "--scope=user", "Git.Git"}, io)).To(Succeed())
				Expect(calls()).To(Equal([][]string{
					winArgv([]string{"install", "--id", "Git.Git", "--exact"}, winChange, []string{"--no-upgrade", "--scope", "user"}),
				}))
			}
		})
	})

	Context("unattended UAC prevention", func() {
		It("returns authorization-required for unattended default-scope changes without launching winget", func() {
			GinkgoT().Setenv(sdk.UnattendedEnv, "1")
			for _, command := range []string{"install", "upgrade", "remove"} {
				err := plugin.Execute(command, []string{"Git.Git"})
				var auth *sdk.AuthorizationError
				Expect(errors.As(err, &auth)).To(BeTrue(), "%s: expected authorization error, got %v", command, err)
				Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired), command)
			}
			Expect(calls()).To(BeEmpty())
		})

		It("allows explicitly scoped unattended operations when elevation matches", func() {
			GinkgoT().Setenv(sdk.UnattendedEnv, "1")

			elevated = false
			Expect(plugin.Execute("install", []string{"--scope=user", "Git.Git"})).To(Succeed())
			Expect(plugin.Execute("is-installed", []string{"Git.Git"})).To(Succeed())
			Expect(plugin.Execute("update", nil)).To(Succeed())

			elevated = true
			Expect(plugin.Execute("install", []string{"--scope=machine", "Docker.DockerDesktop"})).To(Succeed())
			Expect(calls()).To(HaveLen(4))
		})
	})

	It("returns an actionable error when winget is missing from PATH", func() {
		GinkgoT().Setenv("PATH", GinkgoT().TempDir())
		Expect(plugin.Execute("install", []string{"Git.Git"})).To(MatchError(ContainSubstring("winget is not installed or not on PATH")))
		Expect(calls()).To(BeEmpty())
	})
})
