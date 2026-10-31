package bootstrap_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync/atomic"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
)

var _ = Describe("PluginBootstrap", func() {
	var (
		pluginBootstrap *bootstrap.PluginBootstrap
		tempHomeDir     string
		ctx             context.Context
	)

	BeforeEach(func() {
		var err error
		tempHomeDir, err = os.MkdirTemp("", "plugin-bootstrap-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Set HOME environment variable to temp directory
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempHomeDir)

		// Clean up in AfterEach
		DeferCleanup(func() {
			os.Setenv("HOME", originalHome)
			os.RemoveAll(tempHomeDir)
		})

		originalResolver := sdk.NewPrivilegeResolver
		resolver := originalResolver()
		resolver.Elevated = false
		account, err := resolver.CurrentUser()
		Expect(err).NotTo(HaveOccurred())
		copyAccount := *account
		copyAccount.HomeDir = tempHomeDir
		resolver.CurrentUser = func() (*user.User, error) { return &copyAccount, nil }
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		DeferCleanup(func() { sdk.NewPrivilegeResolver = originalResolver })
		ctx = context.Background()
	})

	Describe("IsPluginAvailable", func() {
		BeforeEach(func() {
			// Unreachable, so only the local plugin directory can answer.
			originalURL, present := os.LookupEnv("DEVEX_PLUGIN_REGISTRY_URL")
			os.Setenv("DEVEX_PLUGIN_REGISTRY_URL", "http://127.0.0.1:1")
			DeferCleanup(func() {
				if present {
					os.Setenv("DEVEX_PLUGIN_REGISTRY_URL", originalURL)
				} else {
					os.Unsetenv("DEVEX_PLUGIN_REGISTRY_URL")
				}
			})
		})

		It("reports an installed plugin without asking the registry", func() {
			pb, err := bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())
			plugin := filepath.Join(tempHomeDir, ".devex", "plugins", "devex-plugin-package-manager-apt")
			Expect(os.MkdirAll(filepath.Dir(plugin), 0o750)).To(Succeed())
			Expect(os.WriteFile(plugin, []byte("#!/bin/sh\n"), 0o755)).To(Succeed())
			Expect(pb.IsPluginAvailable(ctx, "package-manager-apt")).To(BeTrue())
		})

		It("ignores a plugin file that is not executable", func() {
			pb, err := bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())
			plugin := filepath.Join(tempHomeDir, ".devex", "plugins", "devex-plugin-package-manager-apt")
			Expect(os.MkdirAll(filepath.Dir(plugin), 0o750)).To(Succeed())
			Expect(os.WriteFile(plugin, []byte("#!/bin/sh\n"), 0o644)).To(Succeed())
			Expect(pb.IsPluginAvailable(ctx, "package-manager-apt")).To(BeFalse())
		})
	})

	Describe("NewPluginBootstrap", func() {
		It("uses an overridden HOME for the current non-elevated account", func() {
			resolver := sdk.NewPrivilegeResolver()
			account, err := resolver.CurrentUser()
			Expect(err).NotTo(HaveOccurred())
			copyAccount := *account
			copyAccount.HomeDir = "/unused-account-home"
			resolver.CurrentUser = func() (*user.User, error) { return &copyAccount, nil }
			sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
			b, err := bootstrap.NewPluginBootstrap(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(b.GetManager().GetPluginDir()).To(Equal(filepath.Join(tempHomeDir, ".devex", "plugins")))
			Expect(b.GetManager().GetPluginDir()).To(BeADirectory())
		})
		It("uses the original account home under sudo and prepares user-owned storage", func() {
			resolver := sdk.NewPrivilegeResolver()
			account, err := resolver.CurrentUser()
			Expect(err).NotTo(HaveOccurred())
			resolver.Elevated = true
			resolver.CurrentUser = func() (*user.User, error) {
				return &user.User{Username: "root", Uid: "0", Gid: "0", HomeDir: "/root"}, nil
			}
			resolver.LookupUser = func(string) (*user.User, error) { return account, nil }
			resolver.Getenv = func(key string) string {
				if key == "SUDO_USER" {
					return account.Username
				}
				return ""
			}
			sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
			GinkgoT().Setenv("HOME", "/root")
			b, err := bootstrap.NewPluginBootstrap(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(b.GetManager().GetPluginDir()).To(Equal(filepath.Join(tempHomeDir, ".devex", "plugins")))
			Expect(b.GetManager().GetPluginDir()).To(BeADirectory())
		})
		Context("when skipDownload is false", func() {
			It("should create bootstrap instance with download enabled", func() {
				bootstrap, err := bootstrap.NewPluginBootstrap(false)
				Expect(err).NotTo(HaveOccurred())
				Expect(bootstrap).NotTo(BeNil())

				// Verify plugin directory was created
				pluginDir := filepath.Join(tempHomeDir, ".devex", "plugins")
				Expect(pluginDir).To(BeADirectory())
			})
		})

		Context("when skipDownload is true", func() {
			It("should create bootstrap instance with download disabled", func() {
				bootstrap, err := bootstrap.NewPluginBootstrap(true)
				Expect(err).NotTo(HaveOccurred())
				Expect(bootstrap).NotTo(BeNil())
			})
		})

		Context("when home directory is not accessible", func() {
			BeforeEach(func() {
				GinkgoT().Setenv("HOME", "/nonexistent/directory")
				resolver := sdk.NewPrivilegeResolver()
				account, err := resolver.CurrentUser()
				Expect(err).NotTo(HaveOccurred())
				copyAccount := *account
				copyAccount.HomeDir = "/nonexistent/directory"
				resolver.CurrentUser = func() (*user.User, error) { return &copyAccount, nil }
				sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
			})

			It("should return an error", func() {
				_, err := bootstrap.NewPluginBootstrap(false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get user home directory"))
			})
		})
	})

	Describe("Initialize", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(tempHomeDir, ".devex", "plugins"), 0755)).To(Succeed())
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true) // Skip download for tests
			Expect(err).NotTo(HaveOccurred())
		})

		It("should initialize successfully", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should detect platform information", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			platform := pluginBootstrap.GetPlatform()
			Expect(platform).NotTo(BeNil())
			Expect(platform.OS).NotTo(BeEmpty())
			Expect(platform.Architecture).NotTo(BeEmpty())
		})

		It("should create plugin manager", func() {
			err := pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			manager := pluginBootstrap.GetManager()
			Expect(manager).NotTo(BeNil())
		})
	})

	Describe("Required plugin failures", func() {
		It("preserves the apt HTTP 500 cause through installation after a later connection failure", func() {
			if runtime.GOOS != "linux" {
				Skip("APT platform detection requires Linux")
			}
			// Make platform detection deterministic with only APT on PATH.
			binDir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(binDir, "apt"), []byte("#!/bin/sh\nexit 0\n"), 0755)).To(Succeed())
			GinkgoT().Setenv("PATH", binDir)
			closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			closed.Close()
			var aptRequests, laterRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/registry":
					_ = json.NewEncoder(w).Encode(sdk.PluginRegistry{Plugins: map[string]sdk.PluginMetadata{"package-manager-apt": {}}})
				case "/api/v1/plugins/package-manager-apt":
					aptRequests.Add(1)
					w.WriteHeader(http.StatusInternalServerError)
				default:
					laterRequests.Add(1)
					http.Redirect(w, r, closed.URL, http.StatusTemporaryRedirect)
				}
			}))
			defer server.Close()
			GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
			pb, err := bootstrap.NewPluginBootstrap(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(pb.Initialize(ctx)).To(Succeed())
			Expect(aptRequests.Load()).To(Equal(int32(3)))
			Expect(laterRequests.Load()).To(Equal(int32(3)))
			installers.InitializeWithPluginBootstrap(pb)
			DeferCleanup(func() { installers.InitializeWithPluginBootstrap(nil) })
			installer := installers.GetInstaller(ctx, "apt")
			Expect(installer).NotTo(BeNil())
			err = installer.Install("git", nil)
			Expect(err).To(MatchError(ContainSubstring("package-manager-apt")))
			Expect(err).To(MatchError(ContainSubstring("HTTP 500")))
			Expect(err.Error()).NotTo(ContainSubstring("unreachable"))
		})

		It("returns the original download failure when a command needs the plugin", func() {
			GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "http://127.0.0.1:1")
			Expect(os.MkdirAll(filepath.Join(tempHomeDir, ".devex", "plugins"), 0755)).To(Succeed())
			pluginBootstrap, err := bootstrap.NewPluginBootstrap(false)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginBootstrap.Initialize(ctx)).To(Succeed())
			requiredPlugins := pluginBootstrap.GetPlatform().GetRequiredPlugins()
			Expect(requiredPlugins).NotTo(BeEmpty())

			for _, pluginName := range requiredPlugins {
				err = pluginBootstrap.ExecutePluginWithContext(ctx, pluginName, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(pluginName))
				Expect(err.Error()).To(ContainSubstring("failed to download plugin"))
				Expect(err.Error()).To(ContainSubstring("plugin registry is unreachable"))
			}
		})
	})

	Describe("Platform Detection", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(tempHomeDir, ".devex", "plugins"), 0755)).To(Succeed())
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should detect the current operating system", func() {
			platform := pluginBootstrap.GetPlatform()

			// OS should be one of the supported platforms
			Expect(platform.OS).To(BeElementOf([]string{"linux", "darwin", "windows"}))
		})

		It("should detect available package managers", func() {
			platform := pluginBootstrap.GetPlatform()

			// Should have detected at least one package manager or none if in test environment
			Expect(platform.PackageManagers).To(BeAssignableToTypeOf([]string{}))
		})

		It("should provide platform string representation", func() {
			platform := pluginBootstrap.GetPlatform()
			platformStr := platform.String()

			Expect(platformStr).NotTo(BeEmpty())
			Expect(platformStr).To(ContainSubstring(platform.OS))
		})
	})

	Describe("Plugin Management Commands", func() {
		var mockRootCmd *cobra.Command

		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(tempHomeDir, ".devex", "plugins"), 0755)).To(Succeed())
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Create a mock root command
			mockRootCmd = &cobra.Command{
				Use:   "devex",
				Short: "DevEx CLI",
			}
		})

		It("should register plugin management commands", func() {
			pluginBootstrap.RegisterCommands(mockRootCmd)

			// Verify plugin command was added
			pluginCmd, _, err := mockRootCmd.Find([]string{"plugin"})
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginCmd.Name()).To(Equal("plugin"))
		})

		It("should include subcommands for plugin management", func() {
			pluginBootstrap.RegisterCommands(mockRootCmd)

			pluginCmd, _, err := mockRootCmd.Find([]string{"plugin"})
			Expect(err).NotTo(HaveOccurred())

			// Check for expected subcommands
			subcommands := []string{"list", "search", "install", "remove", "update", "info"}
			for _, subcmd := range subcommands {
				found := false
				for _, cmd := range pluginCmd.Commands() {
					if cmd.Name() == subcmd {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "Expected subcommand '%s' not found", subcmd)
			}
		})
	})

	Describe("Error Handling", func() {
		It("should handle plugin directory creation errors gracefully", func() {
			// Create a file where the plugin directory should be
			pluginDirPath := filepath.Join(tempHomeDir, ".devex", "plugins")
			parentDir := filepath.Dir(pluginDirPath)
			err := os.MkdirAll(parentDir, 0755)
			Expect(err).NotTo(HaveOccurred())

			// Create a file with the same name as the directory
			err = os.WriteFile(pluginDirPath, []byte("blocking file"), 0644)
			Expect(err).NotTo(HaveOccurred())

			_, err = bootstrap.NewPluginBootstrap(false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create plugin directory"))
		})

		It("should initialize with an empty plugin directory", func() {
			Expect(os.MkdirAll(filepath.Join(tempHomeDir, ".devex", "plugins"), 0755)).To(Succeed())
			var err error
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true)
			Expect(err).NotTo(HaveOccurred())

			// An existing empty directory has no plugins to load.
			err = pluginBootstrap.Initialize(ctx)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
