package commands

import (
	"bytes"
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Inventory command", func() {
	var output bytes.Buffer
	execute := func(repo *mocks.MockRepository, args ...string) error {
		output.Reset()
		cmd := NewInventoryCmd(repo, config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir()})
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{}, args...))
		return cmd.ExecuteContext(context.Background())
	}
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
	})

	It("is a read-only route", func() {
		Expect(readAPICommand(NewInventoryCmd(nil, config.CrossPlatformSettings{}))).To(BeTrue())
	})

	It("reports an empty inventory", func() {
		Expect(execute(mocks.NewMockRepository())).To(Succeed())
		Expect(output.String()).To(ContainSubstring("does not manage any applications"))
	})

	It("exports JSON with the stable schema and fails --check on drift", func() {
		repo := mocks.NewMockRepository()
		Expect(repo.AddApp("legacy-tool")).To(Succeed())
		Expect(execute(repo, "--format", "json")).To(Succeed())
		var result inventory.Inventory
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.SchemaVersion).To(Equal(inventory.SchemaVersion))
		Expect(result.Drift).To(ContainElement(HaveField("Kind", "no_definition")))

		Expect(execute(repo, "--check")).To(MatchError(ContainSubstring("drift")))
		Expect(execute(repo, "--format", "xml")).To(MatchError(ContainSubstring("unsupported format")))
	})

	It("exports YAML with the same schema keys as JSON", func() {
		repo := mocks.NewMockRepository()
		Expect(repo.AddApp("legacy-tool")).To(Succeed())
		keys := func(value any) []string {
			mapping, ok := value.(map[string]any)
			Expect(ok).To(BeTrue(), "%v is not a mapping", value)
			out := make([]string, 0, len(mapping))
			for key := range mapping {
				out = append(out, key)
			}
			return out
		}

		Expect(execute(repo, "--format", "yaml")).To(Succeed())
		var document map[string]any
		Expect(yaml.Unmarshal(output.Bytes(), &document)).To(Succeed())
		Expect(keys(document)).To(ConsistOf("schema_version", "generated_at", "devex_version", "machine", "items", "drift"))
		Expect(document["schema_version"]).To(Equal(inventory.SchemaVersion))
		Expect(keys(document["machine"])).To(ConsistOf("id", "os", "arch"))
		items, ok := document["items"].([]any)
		Expect(ok).To(BeTrue())
		Expect(items).To(HaveLen(1))
		Expect(keys(items[0])).To(ConsistOf("name", "manager", "packages", "state", "managed_by", "definition_recorded", "detail"))
		drift, ok := document["drift"].([]any)
		Expect(ok).To(BeTrue())
		Expect(keys(drift[0])).To(ConsistOf("app", "kind", "detail"))

		Expect(execute(repo, "--format", "yaml", "--include-hostname")).To(Succeed())
		document = nil
		Expect(yaml.Unmarshal(output.Bytes(), &document)).To(Succeed())
		Expect(keys(document["machine"])).To(ContainElement("hostname"))
	})
})
