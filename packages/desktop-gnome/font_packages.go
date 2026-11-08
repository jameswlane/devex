package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Package sources and unavailable entries are documented in FONT_PACKAGES.md.
// Keep this desktop-specific catalog aligned with the other desktop plugin;
// the generic plugin SDK deliberately does not own distribution font policy.
var fontPackagesByManager = map[string]map[string]string{
	"apt-get": {
		"firacode":      "fonts-firacode",
		"jetbrainsmono": "fonts-jetbrains-mono",
		"hack":          "fonts-hack",
		"sourcecodepro": "",
		"inconsolata":   "fonts-inconsolata",
		"cascadia":      "fonts-cascadia-code",
		"ubuntu":        "fonts-ubuntu",
		"liberation":    "fonts-liberation",
		"noto":          "fonts-noto",
		"roboto":        "fonts-roboto",
		"oxygen":        "",
	},
	"dnf": {
		"firacode":      "fira-code-fonts",
		"jetbrainsmono": "jetbrains-mono-fonts",
		"hack":          "source-foundry-hack-fonts",
		"sourcecodepro": "adobe-source-code-pro-fonts",
		"inconsolata":   "levien-inconsolata-fonts",
		"cascadia":      "cascadia-code-fonts",
		"ubuntu":        "",
		"liberation":    "liberation-fonts-all",
		"noto":          "google-noto-sans-fonts",
		"roboto":        "google-roboto-fonts",
		"oxygen":        "oxygen-fonts",
	},
	"pacman": {
		"firacode":      "ttf-fira-code",
		"jetbrainsmono": "ttf-jetbrains-mono",
		"hack":          "ttf-hack",
		"sourcecodepro": "adobe-source-code-pro-fonts",
		"inconsolata":   "ttf-inconsolata",
		"cascadia":      "ttf-cascadia-code",
		"ubuntu":        "ttf-ubuntu-font-family",
		"liberation":    "ttf-liberation",
		"noto":          "noto-fonts",
		"roboto":        "ttf-roboto",
		"oxygen":        "",
	},
	"zypper": {
		"firacode":      "fira-code-fonts",
		"jetbrainsmono": "jetbrains-mono-fonts",
		"hack":          "hack-fonts",
		"sourcecodepro": "adobe-sourcecodepro-fonts",
		"inconsolata":   "google-inconsolata-fonts",
		"cascadia":      "saja-cascadia-code-fonts",
		"ubuntu":        "ubuntu-fonts",
		"liberation":    "liberation-fonts",
		"noto":          "google-noto-sans-fonts",
		"roboto":        "google-roboto-fonts",
		"oxygen":        "kde-oxygen-fonts",
	},
}

func fontManager() (string, error) {
	for _, manager := range []string{"apt-get", "dnf", "pacman", "zypper"} {
		if sdk.CommandExists(manager) {
			return manager, nil
		}
	}
	return "", fmt.Errorf("no supported package manager found")
}

func (fm *FontManager) mapFontNames(manager string, args []string) (packages, unavailable []string) {
	seen := make(map[string]bool)
	for _, name := range args {
		key := strings.ToLower(name)
		// Accept display names, historical aliases, and the old Debian operands.
		key = strings.TrimPrefix(key, "fonts-")
		key = strings.NewReplacer(" ", "", "-", "").Replace(key)
		switch key {
		case "jetbrains":
			key = "jetbrainsmono"
		case "cascadiacode":
			key = "cascadia"
		}
		pkg, known := fontPackagesByManager[manager][key]
		if !known {
			// Explicit native package names remain supported.
			pkg = name
		}
		if known && pkg == "" {
			unavailable = append(unavailable, name)
			continue
		}
		if !seen[pkg] {
			packages = append(packages, pkg)
			seen[pkg] = true
		}
	}
	return packages, unavailable
}

// installPlan is shared by preview and execution so the accepted argv cannot drift.
func (fm *FontManager) installPlan(args []string) ([][]string, error) {
	for _, name := range args {
		if err := validateFontName(name); err != nil {
			return nil, fmt.Errorf("invalid font argument %q: %w", name, err)
		}
		if strings.HasPrefix(name, "-") {
			return nil, fmt.Errorf("font package argument %q must not start with '-'", name)
		}
	}
	manager, err := fontManager()
	if err != nil {
		return nil, err
	}
	defaults := len(args) == 0
	if defaults {
		args = fm.defaultFontNames()
	}
	for _, name := range args {
		if manager == "apt-get" && strings.HasSuffix(name, "-") {
			return nil, fmt.Errorf("apt font package argument %q must not end with '-'", name)
		}
	}
	packages, unavailable := fm.mapFontNames(manager, args)
	if len(unavailable) > 0 {
		message := fmt.Sprintf("no verified package for %s: %s", manager, strings.Join(unavailable, ", "))
		if !defaults {
			return nil, fmt.Errorf("%s; install these fonts manually", message)
		}
		fmt.Fprintf(os.Stderr, "Warning: %s; skipping these default fonts\n", message)
	}
	command := []string{manager, "install", "-y"}
	if manager == "pacman" {
		command = []string{manager, "-S", "--noconfirm"}
	}
	plan := [][]string{append(command, packages...)}
	if sdk.CommandExists("fc-cache") {
		plan = append(plan, []string{"fc-cache", "-f", "-v"})
	}
	return plan, nil
}

// Describe prints the exact font install plan without running native commands.
func (fm *FontManager) Describe(args []string) error {
	plan, err := fm.installPlan(args)
	if err != nil {
		return err
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func (fm *FontManager) executeInstall(ctx context.Context, args []string) error {
	var accepted string
	if len(args) > 0 && args[0] == "--expected-plan" {
		if len(args) < 2 {
			return fmt.Errorf("font install plan changed; preview and confirm again")
		}
		accepted = args[1]
		args = args[2:]
		if accepted == "" {
			return fmt.Errorf("font install plan changed; preview and confirm again")
		}
	}
	plan, err := fm.installPlan(args)
	if err != nil {
		return err
	}
	if accepted != "" {
		data, marshalErr := json.Marshal(plan)
		if marshalErr != nil {
			return marshalErr
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != accepted {
			return fmt.Errorf("font install plan changed; preview and confirm again")
		}
	}
	if err := sdk.ExecOperationWithContext(ctx, systemScope, plan[0][0], plan[0][1:]...); err != nil {
		return err
	}
	for _, command := range plan[1:] {
		if err := sdk.ExecOperationWithContext(ctx, userScope, command[0], command[1:]...); err != nil {
			label := "refresh font cache"
			if command[0] == "kbuildsycoca5" {
				label = "update KDE cache"
			}
			fmt.Fprintf(os.Stderr, "Warning: Failed to %s: %v\n", label, err)
		}
	}
	return nil
}

func (fm *FontManager) defaultFontNames() []string {
	return []string{"firacode", "jetbrainsmono", "hack", "sourcecodepro", "inconsolata", "cascadia", "ubuntu", "liberation"}
}
