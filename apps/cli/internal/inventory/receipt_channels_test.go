package inventory_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Receipt channel recording", func() {
	BeforeEach(func() {
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
	})

	It("records the selected channel name (default or explicit) in the receipt", func() {
		repo := mocks.NewMockRepository()
		app := types.CrossPlatformApp{
			Name: "neovim",
			Linux: types.OSConfig{
				InstallMethod:   "curlpipe",
				DownloadURL:     "https://example.invalid/nvim.tar.gz",
				OfficialSupport: true,
				Alternatives: []types.OSConfig{
					{
						Channel:        "apt-ppa",
						InstallMethod:  "apt",
						InstallCommand: "neovim",
					},
				},
			},
		}
		chosen, err := app.WithChannel("apt-ppa")
		Expect(err).NotTo(HaveOccurred())

		query := func(_ context.Context, manager, pkg string) (bool, string, error) {
			return true, "0.10.0", nil
		}
		err = inventory.Record(context.Background(), repo, chosen, inventory.RecordOptions{
			Query: query,
			Now:   func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
		})
		Expect(err).NotTo(HaveOccurred())

		raw, err := repo.Get(inventory.ReceiptKey("neovim"))
		Expect(err).NotTo(HaveOccurred())
		Expect(raw).To(ContainSubstring(`"channel":"apt-ppa"`))

		receipt, ok := inventory.ReadReceipt(repo, "neovim")
		Expect(ok).To(BeTrue())
		Expect(receipt.Manager).To(Equal("apt"))
		Expect(receipt.Channel).To(Equal("apt-ppa"))
		Expect(receipt.Version).To(Equal("0.10.0"))
	})

	It("reads legacy receipts without a channel field cleanly", func() {
		repo := mocks.NewMockRepository()
		legacy, err := json.Marshal(map[string]any{
			"manager":    "mise",
			"packages":   []string{"codex@latest"},
			"managed_by": "devex",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.Set(inventory.ReceiptKey("codex"), string(legacy))).To(Succeed())

		receipt, ok := inventory.ReadReceipt(repo, "codex")
		Expect(ok).To(BeTrue())
		Expect(receipt.Manager).To(Equal("mise"))
		Expect(receipt.Channel).To(BeEmpty())
	})
})
