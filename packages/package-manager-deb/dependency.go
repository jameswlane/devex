package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// missingDependency is one Depends group with no installed alternative.
// Name is the group's first alternative without version or architecture;
// Spec is the whole group, normalized, for apt to resolve.
type missingDependency struct {
	Name string
	Spec string
}

var (
	dependencyAlternative = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]*)(?::([a-z0-9-]+))?\s*(?:\(\s*(<<|<=|=|>=|>>|<|>)\s*([A-Za-z0-9.+~:-]+)\s*\))?$`)
)

// getMissingDependencies checks for missing dependencies of a .deb package
func (d *DebInstaller) getMissingDependencies(debFile string) ([]missingDependency, error) {
	// Get package dependencies
	ctx := context.Background()
	output, err := sdk.ExecCommandOutputWithContext(ctx, "dpkg-deb", "-f", debFile, "Depends")
	if err != nil {
		return nil, fmt.Errorf("read Depends field: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return nil, nil // No dependencies
	}

	var missing []missingDependency
	for _, group := range strings.Split(output, ",") {
		var first string
		var specs []string
		specValid := true
		anyInstalled := false
		// Split alternatives before removing each member's version constraint.
		for _, alternative := range strings.Split(group, "|") {
			raw := strings.TrimSpace(alternative)
			name := raw
			// This best-effort preflight ignores versions and architecture qualifiers;
			// dpkg -i remains the authoritative dependency check.
			if idx := strings.IndexAny(name, "( :\t\r\n"); idx >= 0 {
				name = name[:idx]
			}
			if name == "" {
				continue
			}
			if first == "" {
				first = name
			}
			if spec, ok := normalizeAlternative(raw); ok {
				specs = append(specs, spec)
			} else {
				specValid = false
			}
			installed, err := d.isPackageInstalled(name)
			if err != nil {
				return nil, err
			}
			if installed {
				anyInstalled = true
				break
			}
		}
		if !anyInstalled && first != "" {
			dep := missingDependency{Name: first}
			if specValid {
				dep.Spec = strings.Join(specs, " | ")
			}
			missing = append(missing, dep)
		}
	}

	return missing, nil
}

// normalizeAlternative rewrites one Depends alternative as
// "name[:arch] [(op version)]", rejecting anything else so only a plain
// dependency expression reaches apt.
func normalizeAlternative(raw string) (string, bool) {
	m := dependencyAlternative.FindStringSubmatch(raw)
	if m == nil {
		return "", false
	}
	spec := m[1]
	if m[2] != "" {
		spec += ":" + m[2]
	}
	if m[3] != "" {
		// Debian Policy: the obsolete < and > mean <= and >=.
		op := map[string]string{"<": "<=", ">": ">="}[m[3]]
		if op == "" {
			op = m[3]
		}
		spec += " (" + op + " " + m[4] + ")"
	}
	return spec, true
}

// dependencyNames lists the first alternative of each missing group, once.
func dependencyNames(deps []missingDependency) []string {
	names := make([]string, 0, len(deps))
	seen := make(map[string]bool, len(deps))
	for _, dep := range deps {
		if !seen[dep.Name] {
			seen[dep.Name] = true
			names = append(names, dep.Name)
		}
	}
	return names
}

// installDependencies installs missing dependencies using apt. It asks
// apt-get satisfy to resolve each whole group first, so versioned
// alternatives and virtual packages (for example libasound2, provided by
// libasound2t64 on Ubuntu 24.04) resolve the way apt would resolve them for
// the package itself. apt older than 2.0 has no satisfy; the fallback
// installs each group's first alternative by name.
func (d *DebInstaller) installDependencies(ctx context.Context, deps []missingDependency) error {
	if len(deps) == 0 {
		return nil
	}

	names := dependencyNames(deps)
	// Validate all dependency names first
	for _, dep := range names {
		if err := d.validatePackageName(dep); err != nil {
			return fmt.Errorf("invalid dependency name '%s': %w", dep, err)
		}
	}

	// Update package lists first
	d.logger.Println("Updating package lists...")
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		// Best-effort: apt-get install below fails if stale lists matter.
		d.logger.Warning("Failed to update package lists: %v", err)
	}

	// --no-remove: a dependency preflight must never uninstall packages to
	// make room; dpkg -i reports whatever stays unmet.
	var specs []missingDependency
	var byName []missingDependency
	for _, dep := range deps {
		if dep.Spec == "" {
			byName = append(byName, dep)
		} else {
			specs = append(specs, dep)
		}
	}
	if len(specs) > 0 {
		satisfyArgs := []string{"satisfy", "-y", "--no-remove"}
		for _, dep := range specs {
			satisfyArgs = append(satisfyArgs, dep.Spec)
		}
		err := sdk.ExecCommandWithContext(ctx, true, "apt-get", satisfyArgs...)
		if err == nil {
			if len(byName) == 0 {
				d.logger.Success("Successfully installed dependencies: %s", strings.Join(satisfyArgs[3:], ", "))
				return nil
			}
			d.logger.Success("Successfully installed dependencies: %s", strings.Join(satisfyArgs[3:], ", "))
			// Only the groups apt could not be handed whole remain.
			names = dependencyNames(byName)
		} else {
			d.logger.Warning("apt-get satisfy could not resolve the dependencies (%v); installing them by name", err)
		}
	}

	// Install dependencies
	cmdArgs := append([]string{"install", "-y", "--no-remove"}, names...)
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", cmdArgs...); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	d.logger.Success("Successfully installed dependencies: %s", strings.Join(names, ", "))
	return nil
}
