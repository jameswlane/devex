package sdk_test

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shared paths", func() {
	It("uses XDG defaults on Linux and macOS", func() {
		for _, platform := range []string{"linux", "darwin"} {
			p := sdk.ResolvePathsFor(platform, "/home/alice", func(string) string { return "" })
			Expect(p.Config).To(Equal("/home/alice/.config/devex"))
			Expect(p.Data).To(Equal("/home/alice/.local/share/devex"))
			Expect(p.State).To(Equal("/home/alice/.local/state/devex"))
			Expect(p.Cache).To(Equal("/home/alice/.cache/devex"))
			Expect(p.Bin).To(Equal("/home/alice/.local/bin"))
		}
	})
	It("honors every XDG and DevEx override with DevEx taking precedence", func() {
		for _, key := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
			env := map[string]string{"XDG_" + key + "_HOME": "/xdg", "DEVEX_" + key + "_DIR": "/override"}
			get := func(k string) string { return env[k] }
			values := func(p sdk.Paths) map[string]string {
				return map[string]string{"CONFIG": p.Config, "DATA": p.Data, "STATE": p.State, "CACHE": p.Cache}
			}
			Expect(values(sdk.ResolvePathsFor("linux", "/home/alice", get))[key]).To(Equal("/override"))
			delete(env, "DEVEX_"+key+"_DIR")
			Expect(values(sdk.ResolvePathsFor("linux", "/home/alice", get))[key]).To(Equal("/xdg/devex"))
			env["XDG_"+key+"_HOME"] = "relative"
			Expect(values(sdk.ResolvePathsFor("linux", "/home/alice", get))[key]).NotTo(ContainSubstring("relative"))
		}
	})
	It("maps Windows config to roaming and remaining directories to local app data", func() {
		p := sdk.ResolvePathsFor("windows", "/users/alice", func(k string) string {
			if k == "APPDATA" {
				return "/roaming"
			}
			if k == "LOCALAPPDATA" {
				return "/local"
			}
			return ""
		})
		Expect(p.Config).To(Equal(filepath.Join("/roaming", "DevEx")))
		Expect(p.Data).To(Equal(filepath.Join("/local", "DevEx")))
		Expect(p.State).To(Equal(p.Data))
		Expect(p.Cache).To(Equal(p.Data))
	})
	It("resolves the sudo invoking account rather than root HOME", func() {
		original := sdk.NewPrivilegeResolver
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true, CurrentUser: func() (*user.User, error) { return &user.User{Uid: "0", HomeDir: "/root"}, nil }, LookupUser: func(name string) (*user.User, error) {
				Expect(name).To(Equal("alice"))
				return &user.User{Uid: "1000", HomeDir: "/home/alice"}, nil
			}, Getenv: func(k string) string {
				if k == "SUDO_USER" {
					return "alice"
				}
				return "/root"
			}}
		}
		p, err := sdk.ResolvePaths()
		Expect(err).NotTo(HaveOccurred())
		Expect(p.Home).To(Equal("/home/alice"))
	})
	It("falls back to the OS home if the privilege resolver fails", func() {
		original := sdk.NewPrivilegeResolver
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{CurrentUser: func() (*user.User, error) { return nil, fmt.Errorf("account unavailable") }}
		}
		home, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())
		Expect(sdk.PathsForHome("").Home).To(Equal(home))
		Expect(filepath.IsAbs(sdk.PathsForHome("").Backups())).To(BeTrue())
	})
})
