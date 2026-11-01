package agentskills_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/agentskills"
	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/cobra"
)

// Agent Skills spec: lowercase letters, digits and single hyphens, max 64 chars.
var skillName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func loadSkill() (agentskills.Frontmatter, string) {
	skill, err := agentskills.Skill()
	Expect(err).NotTo(HaveOccurred())
	fm, body, err := agentskills.ParseFrontmatter(skill)
	Expect(err).NotTo(HaveOccurred())
	return fm, body
}

func fixtureSettings() config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		HomeDir: GinkgoT().TempDir(),
		ApplicationOverrides: []types.CrossPlatformApp{{
			Name: "git", Description: "Version control", Category: "Development",
			AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"},
		}},
	}
}

// resolve finds an example's command in the real Cobra tree and parses its
// flags and arguments without running any handler.
func resolve(example string) (*cobra.Command, []string) {
	root := commands.NewRootCmd("test", mocks.NewMockRepository(), fixtureSettings())
	args := strings.Fields(strings.TrimPrefix(example, "devex"))
	cmd, rest, err := root.Find(args)
	Expect(err).NotTo(HaveOccurred(), "command in %q", example)
	// Cobra adds --help and --version when a command executes; add them here too.
	cmd.InitDefaultHelpFlag()
	cmd.InitDefaultVersionFlag()
	Expect(cmd.ParseFlags(rest)).To(Succeed(), "flags in %q", example)
	positional := cmd.Flags().Args()
	if cmd.Args != nil {
		Expect(cmd.Args(cmd, positional)).To(Succeed(), "arguments in %q", example)
	}
	return cmd, positional
}

var _ = Describe("DevEx agent skill", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	})

	It("has valid Agent Skills frontmatter", func() {
		fm, body := loadSkill()
		Expect(fm.Name).To(Equal(agentskills.SkillName))
		Expect(fm.Name).To(MatchRegexp(skillName.String()))
		Expect(len(fm.Name)).To(BeNumerically("<=", 64))
		Expect(fm.Description).NotTo(BeEmpty())
		Expect(len(fm.Description)).To(BeNumerically("<=", 1024))
		Expect(fm.Metadata).To(HaveKey("cli-compatibility"))
		Expect(strings.TrimSpace(body)).NotTo(BeEmpty())
	})

	It("grants no blanket tool permissions", func() {
		skill, err := agentskills.Skill()
		Expect(err).NotTo(HaveOccurred())
		Expect(skill).NotTo(ContainSubstring("allowed-tools"))
	})

	It("only links to files that exist next to it", func() {
		_, body := loadSkill()
		links := regexp.MustCompile(`\]\(([^)#]+)\)`).FindAllStringSubmatch(body, -1)
		for _, link := range links {
			target := link[1]
			if strings.Contains(target, "://") {
				continue
			}
			_, err := os.Stat(filepath.Join(agentskills.SkillName, target))
			Expect(err).NotTo(HaveOccurred(), "broken relative link %q", target)
		}
	})

	It("uses only commands, flags and arguments the CLI actually accepts", func() {
		_, body := loadSkill()
		examples := agentskills.CommandExamples(body)
		Expect(len(examples)).To(BeNumerically(">=", 15), "the skill should show real command examples")
		for _, example := range examples {
			resolve(example)
		}
	})

	It("never teaches --force or sudo in a command example", func() {
		_, body := loadSkill()
		for _, example := range agentskills.CommandExamples(body) {
			Expect(example).NotTo(ContainSubstring("--force"), example)
			Expect(example).NotTo(HavePrefix("sudo "), example)
		}
	})

	It("previews every install and uninstall before running it", func() {
		_, body := loadSkill()
		var installs, previews int
		for _, example := range agentskills.CommandExamples(body) {
			if strings.Contains(example, "--help") {
				continue
			}
			if strings.HasPrefix(example, "devex install") || strings.HasPrefix(example, "devex uninstall") {
				if strings.Contains(example, "--dry-run") {
					previews++
				} else {
					installs++
					Expect(example).To(ContainSubstring("--non-interactive"), "unattended runs must not prompt: %q", example)
				}
			}
		}
		Expect(previews).To(BeNumerically(">=", installs), "every real run needs a preview example")
	})

	Context("representative workflows against fixtures, without package operations", func() {
		It("discovers available applications as JSON", func() {
			cmd, positional := resolve("devex list available --search git --format json")
			var out bytes.Buffer
			cmd.SetOut(&out)
			stdout := captureStdout(func() {
				Expect(cmd.RunE(cmd, positional)).To(Succeed())
			})
			text := out.String() + stdout
			var apps []map[string]any
			Expect(json.Unmarshal([]byte(strings.TrimSpace(text)), &apps)).To(Succeed(), text)
			Expect(apps).NotTo(BeEmpty())
		})

		It("reports an unknown application with a failing status", func() {
			cmd, positional := resolve("devex status --app definitely-unknown --format json")
			var err error
			stdout := captureStdout(func() { err = cmd.RunE(cmd, positional) })
			Expect(err).To(HaveOccurred())
			Expect(strings.TrimSpace(stdout)).To(Equal("[]"))
		})
	})
})
