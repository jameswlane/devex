//go:build windows

package sdk

import (
	"context"
	"os/user"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/windows"
)

// Run on Windows with: go test -ginkgo.focus='Windows native privilege wiring' .
// The rest of the existing SDK suite includes Unix shell fixtures.
var _ = Describe("Windows native privilege wiring", func() {
	It("selects the token-aware run adapter", func() {
		Expect(defaultPrivilegeAdapter()).To(BeAssignableToTypeOf(windowsPrivilegeAdapter{}))
		Expect(PrivilegeAdapterForOS("windows")).To(BeAssignableToTypeOf(windowsPrivilegeAdapter{}))
	})
	It("reads actual token elevation rather than administrator membership", func() {
		elevated, err := windowsTokenElevated()
		Expect(err).NotTo(HaveOccurred())
		Expect(elevated).To(Equal(windows.GetCurrentProcessToken().IsElevated()))
		Expect(defaultPrivilegeResolver().Elevated).To(Equal(elevated))
	})
	It("ignores SUDO_USER and keeps the current Windows account", func() {
		GinkgoT().Setenv("SUDO_USER", "untrusted-account-that-does-not-exist")
		current, err := user.Current()
		Expect(err).NotTo(HaveOccurred())
		resolver := defaultPrivilegeResolver()
		resolver.Elevated = true
		identity, err := resolver.ResolveUser(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(identity.UID).To(Equal(current.Uid))
		Expect(identity.HomeDir).To(Equal(current.HomeDir))
	})
	It("routes legacy privileged registry and WSL calls through system scope", func() {
		originalResolver, originalAdapter := NewPrivilegeResolver, DefaultPrivilegeAdapter
		DeferCleanup(func() { NewPrivilegeResolver, DefaultPrivilegeAdapter = originalResolver, originalAdapter })
		NewPrivilegeResolver = func() PrivilegeResolver {
			return PrivilegeResolver{CurrentUser: user.Current, LookupUser: user.Lookup, Getenv: privilegeGetenv}
		}
		adapter := &nativeRunAdapter{err: NewAuthorizationRequiredError("administrator required", windowsElevationRequired)}
		DefaultPrivilegeAdapter = adapter
		for _, command := range []string{"reg", "wsl"} {
			err := ExecCommandWithContext(context.Background(), true, command, "literal argument")
			Expect(err).To(BeIdenticalTo(adapter.err))
			Expect(adapter.plan.Privilege.Scope).To(Equal(ScopeSystem))
			Expect(adapter.plan.Elevate).To(BeTrue())
			Expect(adapter.plan.Name).To(Equal(command))
			Expect(adapter.plan.Args).To(Equal([]string{"literal argument"}))
		}
	})
})
