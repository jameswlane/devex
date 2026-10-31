package installers

import (
	"context"
	"errors"
	"os/user"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install hook privilege policy", func() {
	It("refuses pre-install hooks as plain root before any command is dispatched", func() {
		original := sdk.NewPrivilegeResolver
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{Elevated: true, CurrentUser: func() (*user.User, error) { return &user.User{Username: "root", Uid: "0", HomeDir: "/root"}, nil }, Getenv: func(string) string { return "" }}
		}
		err := runInstallCommands(context.Background(), []types.InstallCommand{{Shell: "this-command-must-never-run"}})
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
	})
})
