package setup

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/apps/cli/assets"
	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
	"github.com/spf13/viper"
)

const (
	ohMyZshRepository   = "https://github.com/ohmyzsh/ohmyzsh.git"
	ohMyZshRevision     = "6010fb6260f359e71d99ba25f05f9215809b3ed2"
	defaultOhMyZshTheme = "robbyrussell"
)

var (
	ohMyZshPluginName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	ohMyZshGit        = func(ctx context.Context, args ...string) error {
		output, err := exec.CommandContext(ctx, "git", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
		}
		return nil
	}
)

type ohMyZshSelection struct {
	enabled bool
	theme   string
	plugins []string
}

type ohMyZshAssetPlan struct {
	path   string
	data   []byte
	create bool
}

type ohMyZshPlan struct {
	home              string
	rcFile            string
	frameworkDir      string
	defaultsDir       string
	theme             string
	plugins           []string
	clone             bool
	alreadyLoads      bool
	existingRCEntries []string
	assets            []ohMyZshAssetPlan
}

func configuredOhMyZsh(enabled bool) ohMyZshSelection {
	selection := ohMyZshSelection{
		enabled: enabled,
		theme:   defaultOhMyZshTheme,
		plugins: []string{"git"},
	}
	if viper.IsSet("setup.oh_my_zsh.theme") {
		selection.theme = strings.TrimSpace(viper.GetString("setup.oh_my_zsh.theme"))
	}
	if viper.IsSet("setup.oh_my_zsh.plugins") {
		selection.plugins = nil
		if raw, ok := viper.Get("setup.oh_my_zsh.plugins").(string); ok {
			selection.plugins = strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
		} else {
			selection.plugins = viper.GetStringSlice("setup.oh_my_zsh.plugins")
		}
	}
	return selection
}

func (m *SetupModel) getOhMyZshPlan() (*ohMyZshPlan, error) {
	selection := m.selections.ohMyZsh
	if !selection.enabled || m.getSelectedShell() != "zsh" {
		return nil, nil
	}
	home := m.homeDir()
	if home == "" {
		return nil, fmt.Errorf("resolve home directory for Oh My Zsh")
	}
	if strings.ContainsAny(selection.theme, "\x00\r\n") {
		return nil, fmt.Errorf("setup.oh_my_zsh.theme must not contain newlines")
	}
	for _, plugin := range selection.plugins {
		if !ohMyZshPluginName.MatchString(plugin) {
			return nil, fmt.Errorf("invalid Oh My Zsh plugin %q: use letters, digits, dots, underscores, or hyphens", plugin)
		}
	}

	plan := &ohMyZshPlan{
		home:         home,
		rcFile:       filepath.Join(home, ".zshrc"),
		frameworkDir: filepath.Join(home, ".oh-my-zsh"),
		defaultsDir:  filepath.Join(devexpaths.ForHome(home).Data, "defaults", "zsh"),
		theme:        selection.theme,
		plugins:      append([]string(nil), selection.plugins...),
	}
	exists, entries, err := managed.Read(plan.rcFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", plan.rcFile, err)
	}
	plan.existingRCEntries = entries
	if exists {
		content, err := os.ReadFile(plan.rcFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", plan.rcFile, err)
		}
		if sourcesOhMyZshOutsideBlock(string(content), home) {
			plan.alreadyLoads = true
			return plan, nil
		}
	}

	info, err := os.Lstat(plan.frameworkDir)
	switch {
	case os.IsNotExist(err):
		plan.clone = true
	case err != nil:
		return nil, fmt.Errorf("inspect %s: %w", plan.frameworkDir, err)
	case !info.IsDir():
		return nil, fmt.Errorf("%s exists but is not an Oh My Zsh directory; move it before opting in", plan.frameworkDir)
	default:
		if _, err := os.Stat(filepath.Join(plan.frameworkDir, "oh-my-zsh.sh")); err != nil {
			return nil, fmt.Errorf("%s exists but is not a complete Oh My Zsh installation: %w", plan.frameworkDir, err)
		}
	}

	for _, name := range []string{"rc", "shell", "aliases", "init"} {
		data, err := assets.ZshDefaults.ReadFile("zsh/zsh/" + name)
		if err != nil {
			return nil, fmt.Errorf("read bundled Zsh %s: %w", name, err)
		}
		path := filepath.Join(plan.defaultsDir, name)
		asset := ohMyZshAssetPlan{path: path, data: data}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			asset.create = true
		} else if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		}
		plan.assets = append(plan.assets, asset)
	}
	return plan, nil
}

func sourcesOhMyZshOutsideBlock(content, home string) bool {
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case managed.Start:
			inBlock = true
			continue
		case managed.End:
			inBlock = false
			continue
		}
		if inBlock || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		normalized := strings.NewReplacer(`"`, "", `'`, "", "${ZSH}", "$ZSH", "${HOME}", "$HOME").Replace(trimmed)
		fields := strings.Fields(normalized)
		if len(fields) < 2 || (fields[0] != "source" && fields[0] != ".") {
			continue
		}
		target := strings.TrimSuffix(fields[1], ";")
		switch target {
		case "$ZSH/oh-my-zsh.sh", "$HOME/.oh-my-zsh/oh-my-zsh.sh", "~/.oh-my-zsh/oh-my-zsh.sh", filepath.Join(home, ".oh-my-zsh", "oh-my-zsh.sh"):
			return true
		}
	}
	return false
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func zshSourceEntry(path, home string) string {
	relative, err := filepath.Rel(home, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(filepath.ToSlash(relative))
		return `source "$HOME/` + escaped + `"`
	}
	return "source " + shellSingleQuote(path)
}

func (p *ohMyZshPlan) rcEntries() []string {
	if p.alreadyLoads {
		return nil
	}
	return []string{
		zshSourceEntry(filepath.Join(p.defaultsDir, "rc"), p.home),
		`export ZSH="$HOME/.oh-my-zsh"`,
		"ZSH_THEME=" + shellSingleQuote(p.theme),
		"plugins=(" + strings.Join(p.plugins, " ") + ")",
		`source "$ZSH/oh-my-zsh.sh"`,
	}
}

func ohMyZshOwnsRCEntry(entry string) bool {
	entry = strings.TrimSpace(entry)
	if strings.HasPrefix(entry, "export ZSH=") || strings.HasPrefix(entry, "ZSH_THEME=") || strings.HasPrefix(entry, "plugins=(") || entry == `source "$ZSH/oh-my-zsh.sh"` {
		return true
	}
	if !strings.HasPrefix(entry, "source ") {
		return false
	}
	target := strings.Trim(strings.TrimSpace(strings.TrimPrefix(entry, "source ")), `"'`)
	return strings.HasSuffix(target, "/defaults/zsh/rc") && (strings.HasPrefix(target, "$HOME/") || filepath.IsAbs(target))
}

func (m *SetupModel) applyOhMyZsh(ctx context.Context) error {
	plan, err := m.getOhMyZshPlan()
	if err != nil || plan == nil {
		return err
	}
	if plan.alreadyLoads {
		m.installation.addChange("your ~/.zshrc already loads Oh My Zsh; it is left unchanged")
		return nil
	}
	if plan.clone {
		if err := m.cloneOhMyZsh(ctx, plan); err != nil {
			return err
		}
		m.installation.addChange(fmt.Sprintf("Oh My Zsh installed at %s (pinned commit %s)", plan.frameworkDir, ohMyZshRevision))
	} else {
		m.installation.addChange(fmt.Sprintf("Existing Oh My Zsh reused unchanged at %s", plan.frameworkDir))
	}

	for _, asset := range plan.assets {
		if !asset.create {
			continue
		}
		created, err := deployZshDefault(asset)
		if err != nil {
			return err
		}
		if !created {
			continue
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(asset.data))
		m.systemUndo.changes = append(m.systemUndo.changes, setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "Created bundled Zsh default " + asset.path,
			Path: asset.path, Checksum: checksum,
		})
		m.installation.addChange("Deployed bundled Zsh default: " + asset.path)
	}

	entries := make([]string, 0, len(plan.existingRCEntries)+len(plan.rcEntries()))
	for _, entry := range plan.existingRCEntries {
		if !ohMyZshOwnsRCEntry(entry) {
			entries = append(entries, entry)
		}
	}
	entries = append(entries, plan.rcEntries()...)
	changed, err := managed.Apply(plan.rcFile, entries)
	if err != nil {
		return fmt.Errorf("configure Oh My Zsh in %s: %w", plan.rcFile, err)
	}
	if changed {
		m.installation.addChange(fmt.Sprintf("Updated the DevEx block in %s with bundled Zsh and Oh My Zsh setup; first-write backup uses %s.devex-bak-*", plan.rcFile, plan.rcFile))
	} else {
		m.installation.addChange("Bundled Zsh and Oh My Zsh rc configuration already up to date")
	}
	return nil
}

func (m *SetupModel) cloneOhMyZsh(ctx context.Context, plan *ohMyZshPlan) (err error) {
	if err := os.Mkdir(plan.frameworkDir, 0o700); err != nil {
		return fmt.Errorf("create Oh My Zsh destination: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			if cleanupErr := os.RemoveAll(plan.frameworkDir); err == nil && cleanupErr != nil {
				err = fmt.Errorf("remove incomplete Oh My Zsh clone: %w", cleanupErr)
			}
		}
	}()
	commands := [][]string{
		{"-C", plan.frameworkDir, "init"},
		{"-C", plan.frameworkDir, "remote", "add", "origin", ohMyZshRepository},
		{"-C", plan.frameworkDir, "fetch", "--depth", "1", "origin", ohMyZshRevision},
		{"-C", plan.frameworkDir, "checkout", "--detach", "FETCH_HEAD"},
	}
	for _, args := range commands {
		if err := ohMyZshGit(ctx, args...); err != nil {
			return fmt.Errorf("install pinned Oh My Zsh: %w", err)
		}
	}
	marker := filepath.Join(plan.frameworkDir, setupundo.OhMyZshMarker)
	if err := os.WriteFile(marker, []byte(setupundo.OhMyZshMarkerContent(ohMyZshRevision)), 0o600); err != nil {
		return fmt.Errorf("record DevEx Oh My Zsh ownership: %w", err)
	}
	m.systemUndo.changes = append(m.systemUndo.changes, setupundo.Change{
		Kind: setupundo.KindOhMyZsh, Description: "Oh My Zsh clone " + plan.frameworkDir,
		Path: plan.frameworkDir, Value: ohMyZshRevision,
	})
	complete = true
	return nil
}

func deployZshDefault(asset ohMyZshAssetPlan) (created bool, err error) {
	if err := os.MkdirAll(filepath.Dir(asset.path), 0o700); err != nil {
		return false, fmt.Errorf("create bundled Zsh directory: %w", err)
	}
	file, err := os.OpenFile(asset.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create bundled Zsh default %s: %w", asset.path, err)
	}
	defer func() {
		if !created {
			_ = os.Remove(asset.path)
		}
	}()
	if _, err = file.Write(asset.data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return false, fmt.Errorf("write bundled Zsh default %s: %w", asset.path, err)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close bundled Zsh default %s: %w", asset.path, closeErr)
	}
	return true, nil
}
