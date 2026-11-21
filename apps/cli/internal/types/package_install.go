package types

import (
	"fmt"
	"strings"
	"unicode"
)

// PackageInstallRequest separates identifiers from native manager options. It is
// serialized to the existing plugin argv protocol, never to a shell program.
type PackageInstallRequest struct {
	Packages []string
	Options  []string
}

// ResolvePackageInstall prefers structured fields and otherwise adapts legacy
// install_command strings. Mixing representations is rejected to avoid ambiguity.
func ResolvePackageInstall(method, command string, packages, options []string) (PackageInstallRequest, error) {
	request := PackageInstallRequest{Packages: append([]string{}, packages...)}
	if len(packages) > 0 && command != "" {
		return request, fmt.Errorf("use packages or install_command, not both")
	}
	tokens := options
	if len(packages) == 0 {
		var err error
		tokens, err = splitPackageCommand(command)
		if err != nil {
			return request, err
		}
		start := 0
		if len(tokens) > 1 && tokens[0] == "sudo" {
			start++
		}
		if start < len(tokens)-1 && (tokens[start] == method || ((method == "apt" || method == "deb") && (tokens[start] == "apt" || tokens[start] == "apt-get"))) {
			start++
		}
		if start < len(tokens)-1 && isPackageManager(tokens[start]) && tokens[start] != method && (method != "deb" || (tokens[start] != "apt" && tokens[start] != "apt-get")) {
			return request, fmt.Errorf("unexpected manager executable %q for %s", tokens[start], method)
		}
		// A consumed executable must introduce a recognized operation.
		if start > 0 {
			if start >= len(tokens) || (tokens[start] != "install" && tokens[start] != "remove" && tokens[start] != "uninstall") {
				return request, fmt.Errorf("manager executable requires install, remove or uninstall")
			}
		}
		verb := ""
		if start < len(tokens) && (start > 0 || len(tokens) > 1) && (tokens[start] == "install" || tokens[start] == "remove" || tokens[start] == "uninstall") {
			verb = tokens[start]
			start++
		}
		tokens = tokens[start:]
		if method == "flatpak" && verb == "install" && len(tokens) > 1 && tokens[0] == "flathub" {
			tokens = tokens[1:]
		}
		if len(options) > 0 {
			parsed, err := ResolvePackageInstall(method, "", []string{"validation"}, options)
			if err != nil {
				return request, err
			}
			tokens = append(append([]string{}, tokens...), parsed.Options...)
		}
	}
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if !strings.HasPrefix(token, "-") {
			if len(packages) > 0 {
				return request, fmt.Errorf("option value %q has no option", token)
			}
			request.Packages = append(request.Packages, token)
			continue
		}
		name, value, inline := strings.Cut(token, "=")
		if method == "zypper" && name == "-t" {
			// Zypper's short spelling; the plugin argv uses the long form.
			name = "--type"
		}
		takesValue, ok := packageOption(method, name)
		if !ok {
			return request, fmt.Errorf("unsupported %s package option %q", method, name)
		}
		if takesValue {
			if !inline {
				i++
				if i >= len(tokens) || strings.HasPrefix(tokens[i], "-") {
					return request, fmt.Errorf("option %s requires a value", name)
				}
				value = tokens[i]
			}
			if value == "" || strings.ContainsAny(value, "\x00\n\r") {
				return request, fmt.Errorf("option %s requires a nonempty single-line value", name)
			}
			if method == "zypper" && value != "package" && value != "pattern" {
				return request, fmt.Errorf("zypper option --type must be package or pattern, not %q", value)
			}
			if method == "winget" && value != "user" && value != "machine" {
				return request, fmt.Errorf("winget option --scope must be user or machine, not %q", value)
			}
			request.Options = append(request.Options, name+"="+value)
		} else {
			if inline {
				return request, fmt.Errorf("option %s does not accept a value", name)
			}
			// Released plugins already choose their own confirmation policy.
			if name != "-y" && name != "--yes" {
				request.Options = append(request.Options, name)
			}
		}
	}
	for _, pkg := range request.Packages {
		if pkg == "" || strings.HasPrefix(pkg, "-") || strings.ContainsAny(pkg, "\x00\n\r") {
			return request, fmt.Errorf("invalid package identifier %q", pkg)
		}
	}
	return request, nil
}

func packageOption(method, name string) (takesValue, supported bool) {
	if name == "-y" || name == "--yes" {
		return false, true
	}
	switch method {
	case "appimage":
		if name == "--cli" || name == "--gui" {
			return false, true
		}
	case "snap":
		switch name {
		case "--channel", "--revision":
			return true, true
		case "--classic", "--devmode", "--jailmode", "--edge", "--beta", "--candidate", "--stable":
			return false, true
		}
	case "brew":
		if name == "--cask" || name == "--formula" {
			return false, true
		}
	case "cargo":
		if name == "--locked" {
			return false, true
		}
	case "zypper":
		// --type selects packages or SUSE software patterns.
		if name == "--type" {
			return true, true
		}
	case "winget":
		// --scope selects a user or machine installer; machine needs elevation.
		if name == "--scope" {
			return true, true
		}
	}
	return false, false
}

// splitPackageCommand handles quotes and escapes without expansion or execution.
// Shell operators outside quotes belong in explicit installation hooks.
func splitPackageCommand(command string) ([]string, error) {
	tokens := []string{}
	var token strings.Builder
	var quote rune
	escaped, started := false, false
	for index, c := range command {
		if c == 0 || c == '\n' || c == '\r' {
			return nil, fmt.Errorf("move shell commands to pre_install or post_install")
		}
		if escaped {
			token.WriteRune(c)
			escaped = false
			started = true
			continue
		}
		if c == '\\' && quote == '"' && index+1 < len(command) && !strings.ContainsRune("\\\"$`", rune(command[index+1])) {
			token.WriteRune(c)
			started = true
			continue
		}
		if c == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			} else {
				token.WriteRune(c)
			}
			started = true
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			started = true
			continue
		}
		if strings.ContainsRune("|;&`", c) || c == '$' {
			return nil, fmt.Errorf("move shell commands to pre_install or post_install")
		}
		if unicode.IsSpace(c) {
			if started {
				tokens = append(tokens, token.String())
				token.Reset()
				started = false
			}
			continue
		}
		token.WriteRune(c)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated quote or escape in install_command")
	}
	if started {
		tokens = append(tokens, token.String())
	}
	return tokens, nil
}

// ShellPackageArguments quotes each structured argument for the legacy streaming
// installer. The plugin path uses argv directly and does not use this adapter.
func (config OSConfig) ShellPackageArguments() (string, error) {
	request, err := ResolvePackageInstall(config.InstallMethod, config.InstallCommand, config.Packages, config.Options)
	if err != nil {
		return "", err
	}
	args := append(append([]string{}, request.Options...), request.Packages...)
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(args, " "), nil
}

func isPackageManager(name string) bool {
	switch name {
	case "sudo", "apt", "apt-get", "snap", "dnf", "yum", "brew", "flatpak", "mas", "pip", "pacman", "winget", "zypper":
		return true
	}
	return false
}

// PackageQueryCommand adapts structured identifiers for the existing read/remove
// interface. Installation options are never applied to a read operation.
func (a AppConfig) PackageQueryCommand() string {
	if len(a.Packages) == 0 {
		return a.WithKindSelector(a.InstallCommand)
	}
	args := append([]string{}, a.Packages...)
	if a.InstallMethod == "brew" {
		for _, option := range a.Options {
			if option == "--cask" || option == "--formula" {
				args = append([]string{option}, args...)
			}
		}
	}
	if a.InstallMethod == "zypper" {
		// Keep the package or pattern selection; it names what to query.
		if request, err := ResolvePackageInstall(a.InstallMethod, "", a.Packages, a.Options); err == nil {
			args = append(append([]string{}, request.Options...), args...)
		}
	}
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return a.InstallMethod + " install " + strings.Join(args, " ")
}

// WithKindSelector appends the app's typed kind selection to a legacy command
// string that does not select a kind itself, so query and removal act on the
// same kind that installation chose: Homebrew's --cask or --formula, or
// Zypper's --type. Appending keeps "brew uninstall NAME" forms parseable.
func (a AppConfig) WithKindSelector(command string) string {
	if (a.InstallMethod != "brew" && a.InstallMethod != "zypper") || strings.TrimSpace(command) == "" {
		return command
	}
	tokens, err := splitPackageCommand(command)
	if err != nil || kindSelector(a.InstallMethod, tokens) != "" {
		return command
	}
	selector := kindSelector(a.InstallMethod, a.Options)
	if selector == "" && command != a.InstallCommand {
		// A selector written only in install_command still applies to removal.
		if installTokens, err := splitPackageCommand(a.InstallCommand); err == nil {
			selector = kindSelector(a.InstallMethod, installTokens)
		}
	}
	if selector == "" {
		return command
	}
	return command + " " + selector
}

// kindSelector returns the last kind selection in tokens, normalized to one
// token, or "" when there is none.
func kindSelector(method string, tokens []string) string {
	selector := ""
	for i, token := range tokens {
		switch {
		case method == "brew" && (token == "--cask" || token == "--formula"):
			selector = token
		case method == "zypper" && strings.HasPrefix(token, "--type="):
			selector = token
		case method == "zypper" && (token == "-t" || token == "--type") && i+1 < len(tokens):
			selector = "--type=" + tokens[i+1]
		}
	}
	return selector
}
