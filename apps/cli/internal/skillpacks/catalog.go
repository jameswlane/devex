package skillpacks

import (
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	cliconfig "github.com/jameswlane/devex/apps/cli/config"
	skillinstall "github.com/jameswlane/devex/apps/cli/internal/agentskills/install"
	"gopkg.in/yaml.v3"
)

var (
	packNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	fullSHARegexp  = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

// Catalog holds curated third-party agent skill pack definitions keyed by pack name.
type Catalog struct {
	packs map[string]Pack
}

// Pack defines one curated skill pack from a YAML catalog entry.
type Pack struct {
	Name         string                `yaml:"name"`
	Description  string                `yaml:"description"`
	Source       string                `yaml:"source"`
	Path         string                `yaml:"path"`
	License      string                `yaml:"license"`
	Ref          string                `yaml:"ref"`
	SHA          string                `yaml:"sha"`
	Requirements []Requirement         `yaml:"requirements"`
	Skills       map[string]SkillEntry `yaml:"skills"`
}

// SkillEntry describes metadata for one skill in a Pack.
type SkillEntry struct {
	Depends []string `yaml:"depends"`
	Notes   []string `yaml:"notes"`
}

// Requirement declares an external CLI tool or auth check needed by a pack or
// subset of its skills.
type Requirement struct {
	Command string   `yaml:"command"`
	Check   []string `yaml:"check"`
	Skills  []string `yaml:"skills"`
	Hint    string   `yaml:"hint"`
}

// IsFullSHA reports whether s is a 40-character hexadecimal Git commit SHA.
func IsFullSHA(s string) bool {
	return fullSHARegexp.MatchString(s)
}

// ShortSHA returns the 7-character abbreviated commit SHA when available.
func ShortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

// LoadEmbeddedCatalog loads and validates all curated skill packs embedded in
// apps/cli/config/skill-packs/.
func LoadEmbeddedCatalog() (*Catalog, error) {
	return LoadCatalogFS(cliconfig.SkillPacks, "skill-packs")
}

// LoadCatalogFS loads and validates all *.yaml / *.yml pack files from dir in fsys.
func LoadCatalogFS(fsys fs.FS, dir string) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read skill packs catalog %q: %w", dir, err)
	}
	packs := make([]Pack, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		filePath := path.Join(dir, name)
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, fmt.Errorf("read skill pack %q: %w", filePath, err)
		}
		var pack Pack
		if err := yaml.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("parse skill pack %q: %w", filePath, err)
		}
		packs = append(packs, pack)
	}
	return NewCatalog(packs...)
}

// NewCatalog validates packs and constructs a Catalog.
func NewCatalog(packs ...Pack) (*Catalog, error) {
	cat := &Catalog{packs: make(map[string]Pack, len(packs))}
	for _, pack := range packs {
		if err := pack.Validate(); err != nil {
			return nil, fmt.Errorf("invalid skill pack %q: %w", pack.Name, err)
		}
		if _, exists := cat.packs[pack.Name]; exists {
			return nil, fmt.Errorf("duplicate skill pack %q in catalog", pack.Name)
		}
		pack.SHA = strings.ToLower(pack.SHA)
		cat.packs[pack.Name] = pack
	}
	return cat, nil
}

// Lookup returns a curated pack by name.
func (c *Catalog) Lookup(name string) (Pack, bool) {
	if c == nil {
		return Pack{}, false
	}
	p, ok := c.packs[name]
	return p, ok
}

// Names returns all catalog pack names in sorted order.
func (c *Catalog) Names() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.packs))
	for name := range c.packs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Validate checks a Pack definition for schema, URL, SHA, dependency, and cycle errors.
func (p Pack) Validate() error {
	if !packNameRegexp.MatchString(p.Name) {
		return fmt.Errorf("invalid pack name %q", p.Name)
	}
	if err := validateHTTPSURL(p.Source); err != nil {
		return fmt.Errorf("invalid source %q: %w", p.Source, err)
	}
	if err := validateRelativeSkillsPath(p.Path); err != nil {
		return fmt.Errorf("invalid path %q: %w", p.Path, err)
	}
	if !IsFullSHA(p.SHA) {
		return fmt.Errorf("sha %q must be a 40-character hexadecimal commit hash", p.SHA)
	}
	if len(p.Skills) == 0 {
		return fmt.Errorf("pack must declare at least one skill")
	}
	for skillName, entry := range p.Skills {
		if err := skillinstall.ValidateSkillName(skillName); err != nil {
			return err
		}
		for _, dep := range entry.Depends {
			if _, ok := p.Skills[dep]; !ok {
				return fmt.Errorf("skill %q depends on unknown skill %q", skillName, dep)
			}
		}
	}
	if err := p.detectCycles(); err != nil {
		return err
	}
	for _, req := range p.Requirements {
		if strings.TrimSpace(req.Command) == "" {
			return fmt.Errorf("requirement command must not be empty")
		}
		for _, skillName := range req.Skills {
			if _, ok := p.Skills[skillName]; !ok {
				return fmt.Errorf("requirement %q references unknown skill %q", req.Command, skillName)
			}
		}
	}
	return nil
}

func (p Pack) detectCycles() error {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(p.Skills))
	var visit func(name string, stack []string) error
	visit = func(name string, stack []string) error {
		state[name] = visiting
		stack = append(stack, name)
		deps := append([]string(nil), p.Skills[name].Depends...)
		sort.Strings(deps)
		for _, dep := range deps {
			switch state[dep] {
			case visiting:
				idx := slices.Index(stack, dep)
				cycle := append(append([]string(nil), stack[idx:]...), dep)
				return fmt.Errorf("skill dependency cycle detected: %s", strings.Join(cycle, " -> "))
			case unvisited:
				if err := visit(dep, stack); err != nil {
					return err
				}
			}
		}
		state[name] = visited
		return nil
	}
	for _, name := range p.SkillNames() {
		if state[name] == unvisited {
			if err := visit(name, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// SkillNames returns all declared skill names in sorted order.
func (p Pack) SkillNames() []string {
	names := make([]string, 0, len(p.Skills))
	for name := range p.Skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveClosure computes the full dependency closure for requested skills.
// When requested is empty, every skill in the pack is returned. Unknown skill
// names return an error listing all valid skill names.
func (p Pack) ResolveClosure(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return p.SkillNames(), nil
	}
	valid := p.SkillNames()
	selected := map[string]bool{}
	var add func(name string) error
	add = func(name string) error {
		entry, ok := p.Skills[name]
		if !ok {
			return fmt.Errorf("unknown skill %q in pack %q (valid skills: %s)", name, p.Name, strings.Join(valid, ", "))
		}
		if selected[name] {
			return nil
		}
		selected[name] = true
		for _, dep := range entry.Depends {
			if err := add(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, raw := range requested {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if err := add(name); err != nil {
			return nil, err
		}
	}
	if len(selected) == 0 {
		return p.SkillNames(), nil
	}
	result := make([]string, 0, len(selected))
	for name := range selected {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// DependentsOf returns, for each skill in removeSet, the sorted list of other
// installed skills (not in removeSet) that directly or transitively depend on it.
func (p Pack) DependentsOf(removeSet map[string]bool, installed map[string]bool) map[string][]string {
	blocked := map[string][]string{}
	for skill := range installed {
		if removeSet[skill] {
			continue
		}
		if _, known := p.Skills[skill]; !known {
			continue
		}
		closure, err := p.ResolveClosure([]string{skill})
		if err != nil {
			continue
		}
		for _, dep := range closure {
			if dep == skill {
				continue
			}
			if removeSet[dep] {
				blocked[dep] = append(blocked[dep], skill)
			}
		}
	}
	for dep := range blocked {
		sort.Strings(blocked[dep])
	}
	return blocked
}

// NotesFor returns any catalog notes associated with skills in the given slice.
func (p Pack) NotesFor(skills []string) map[string][]string {
	notes := map[string][]string{}
	for _, skill := range skills {
		if entry, ok := p.Skills[skill]; ok && len(entry.Notes) > 0 {
			notes[skill] = append([]string(nil), entry.Notes...)
		}
	}
	return notes
}

// ParsePackSpec parses a CLI pack argument such as "tstack", "tstack@main", or
// "git+https://github.com/owner/repo@v1.0".
func ParsePackSpec(spec string) (nameOrSource, ref string, isRaw bool) {
	if strings.HasPrefix(spec, "git+") {
		return spec, "", true
	}
	if idx := strings.LastIndex(spec, "@"); idx > 0 {
		return spec[:idx], spec[idx+1:], false
	}
	return spec, "", false
}

// ParseRawSource validates a raw git+https:// source specification and extracts
// its repository URL, optional ref, and derived pack identifier.
func ParseRawSource(raw string) (sourceURL, ref, packName string, err error) {
	if !strings.HasPrefix(raw, "git+") {
		return "", "", "", fmt.Errorf("raw source %q must begin with git+https://", DisplaySource(raw))
	}
	rest := strings.TrimPrefix(raw, "git+")
	if strings.ContainsAny(rest, "?#") {
		return "", "", "", fmt.Errorf("raw source %q must not contain a query or fragment", DisplaySource(raw))
	}
	if lastSlash := strings.LastIndex(rest, "/"); lastSlash >= 0 {
		if at := strings.LastIndex(rest, "@"); at > lastSlash {
			ref = rest[at+1:]
			rest = rest[:at]
			if strings.TrimSpace(ref) == "" {
				return "", "", "", fmt.Errorf("empty ref after '@' in source %q", DisplaySource(raw))
			}
			if !validRawRef(ref) {
				return "", "", "", fmt.Errorf("ref in source %q may contain only letters, digits, '.', '_', '-' and '/'", DisplaySource(raw))
			}
		}
	}
	if err := validateHTTPSURL(rest); err != nil {
		return "", "", "", fmt.Errorf("raw source %q must be an https:// URL: %w", DisplaySource(raw), err)
	}
	parsed, err := url.Parse(rest)
	if err != nil {
		return "", "", "", fmt.Errorf("raw source %q is not a valid URL", DisplaySource(raw))
	}
	cleanPath := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	if cleanPath == "" {
		return "", "", "", fmt.Errorf("raw source %q must include a repository path", DisplaySource(raw))
	}
	return rest, ref, cleanPath, nil
}

// DisplaySource returns source with any user info, query and fragment
// removed, so error messages and output never echo credentials.
func DisplaySource(source string) string {
	prefix := ""
	if strings.HasPrefix(source, "git+") {
		prefix, source = "git+", strings.TrimPrefix(source, "git+")
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Opaque != "" || parsed.Host == "" {
		return "<invalid URL>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return prefix + parsed.String()
}

func validateHTTPSURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		// url.Parse errors quote the input, which may hold credentials.
		return fmt.Errorf("not a valid URL")
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("host must not be empty")
	}
	// Sources are stored in lock files and printed in plans, so they must not
	// carry credentials (or tokens smuggled into a query string).
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("URL must not contain credentials, a query or a fragment")
	}
	return nil
}

func validateRelativeSkillsPath(p string) error {
	if p == "" || p == "." {
		return nil
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || strings.Contains(p, "\x00") {
		return fmt.Errorf("must be a relative slash-separated path")
	}
	cleaned := path.Clean(p)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return fmt.Errorf("must not escape repository root")
	}
	return nil
}

func validRawRef(ref string) bool {
	for _, r := range ref {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-', r == '/':
		default:
			return false
		}
	}
	return !strings.HasPrefix(ref, "-")
}
