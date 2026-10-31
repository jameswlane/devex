package utils_test

import (
	"context"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("User operation execution", func() {
	It("expands configuration paths from the intended account instead of root's environment", func() {
		GinkgoT().Setenv("HOME", "/root")
		GinkgoT().Setenv("USER", "root")
		GinkgoT().Setenv("LOGNAME", "root")
		identity := sdk.UserIdentity{Username: "alice", UID: "1000", HomeDir: "/Users/alice"}
		Expect(utils.ExpandUserPlaceholders("$HOME/.config/${USER}/$LOGNAME", identity)).To(Equal("/Users/alice/.config/alice/alice"))
	})
	It("passes arguments literally and honors cancellation", func() {
		if os.Geteuid() == 0 {
			Skip("requires a non-root runner; SDK contracts simulate elevated identity")
		}
		output, err := utils.ExecAsUserWithContext(context.Background(), "printf", "%s", "hello world; $(false)")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("hello world; $(false)"))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = utils.ExecAsUserWithContext(ctx, "printf", "%s", "unused")
		Expect(err).To(MatchError(context.Canceled))
	})
	It("retains the legacy shell-text form for hooks", func() {
		if os.Geteuid() == 0 {
			Skip("requires a non-root runner")
		}
		output, err := utils.ExecAsUserWithContext(context.Background(), "printf '%s' hook")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(output)).To(Equal("hook"))
	})
})
