package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Selection kinds the plugin passes to zypper's --type option.
const (
	kindPackage = "package"
	kindPattern = "pattern"
)

// selection is a validated install, remove or is-installed request: one
// resolvable kind and the identifiers of that kind.
type selection struct {
	kind  string
	names []string
}

// exactName matches a package or pattern name, as is-installed queries them
// exactly. Install and remove also accept zypper's version relations.
var exactName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9+_.:-]*$`)

// packageSpec matches a name with an optional version relation, such as
// git>=2.40 or vim=9.1.
var packageSpec = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9+_.:-]*(?:(?:<|<=|=|>=|>)[A-Za-z0-9+_.:~-]+)?$`)

// parseSelection reads the --type/-t option (default package) and the
// identifiers. One kind applies to the whole request, as in zypper itself.
func parseSelection(args []string) (selection, error) {
	sel := selection{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value, isType, err := typeOption(args, &i)
		if err != nil {
			return sel, err
		}
		if isType {
			if value != kindPackage && value != kindPattern {
				return sel, fmt.Errorf("unsupported zypper type %q: use package or pattern", value)
			}
			if sel.kind != "" && sel.kind != value {
				return sel, fmt.Errorf("a request selects one type; got %s and %s", sel.kind, value)
			}
			sel.kind = value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return sel, fmt.Errorf("invalid package name: %q", arg)
		}
		sel.names = append(sel.names, arg)
	}
	if sel.kind == "" {
		sel.kind = kindPackage
	}
	if len(sel.names) == 0 {
		return sel, fmt.Errorf("no packages specified")
	}
	valid := packageSpec
	if sel.kind == kindPattern {
		valid = exactName
	}
	for _, name := range sel.names {
		if !valid.MatchString(name) {
			return sel, fmt.Errorf("invalid %s name: %q", sel.kind, name)
		}
	}
	return sel, nil
}

// typeOption recognizes --type=V, --type V and -t V at args[*i], advancing *i
// past a separate value.
func typeOption(args []string, i *int) (string, bool, error) {
	arg := args[*i]
	if value, ok := strings.CutPrefix(arg, "--type="); ok {
		return value, true, nil
	}
	if arg != "--type" && arg != "-t" {
		return "", false, nil
	}
	if *i+1 >= len(args) {
		return "", true, fmt.Errorf("option %s requires a value", arg)
	}
	*i++
	return args[*i], true, nil
}
