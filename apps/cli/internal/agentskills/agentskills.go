// Package agentskills ships the DevEx Agent Skill (https://agentskills.io) so
// coding agents get version-matched, tested instructions for using the CLI.
// The canonical source is devex/SKILL.md; CLI and help changes must update it
// in the same pull request.
package agentskills

import (
	"bufio"
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed devex
var files embed.FS

// SkillName is the skill's name and directory.
const SkillName = "devex"

// Frontmatter is the YAML header of a SKILL.md file.
type Frontmatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Metadata    map[string]string `yaml:"metadata"`
}

// Skill returns the embedded SKILL.md.
func Skill() (string, error) {
	data, err := files.ReadFile(SkillName + "/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("read embedded skill: %w", err)
	}
	return string(data), nil
}

// ParseFrontmatter splits a SKILL.md into its YAML frontmatter and body.
func ParseFrontmatter(skill string) (Frontmatter, string, error) {
	const delimiter = "---\n"
	if !strings.HasPrefix(skill, delimiter) {
		return Frontmatter{}, "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	header, body, found := strings.Cut(strings.TrimPrefix(skill, delimiter), "\n"+delimiter)
	if !found {
		return Frontmatter{}, "", fmt.Errorf("SKILL.md frontmatter is not closed")
	}
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(header), &fm); err != nil {
		return Frontmatter{}, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, body, nil
}

// CommandExamples returns every `devex ...` line inside fenced code blocks.
func CommandExamples(body string) []string {
	var examples []string
	inBlock := false
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "```") {
			inBlock = !inBlock
			continue
		}
		if inBlock && (line == "devex" || strings.HasPrefix(line, "devex ")) {
			examples = append(examples, line)
		}
	}
	return examples
}
