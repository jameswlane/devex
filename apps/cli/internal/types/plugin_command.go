package types

import (
	"fmt"
	"regexp"
	"strings"
)

// InstallMethodPlugin installs an app by running a plugin command, such as
// `package-manager-mise ensure-installed`. Its install_command (and optional
// uninstall_command) is "<plugin> <command> [args...]".
const InstallMethodPlugin = "plugin"

var pluginNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ParsePluginCommand splits a plugin install method command into the plugin
// name and the arguments to run it with.
func ParsePluginCommand(command string) (plugin string, args []string, err error) {
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return "", nil, fmt.Errorf("plugin command %q must name a plugin and a command, such as \"package-manager-mise ensure-installed\"", command)
	}
	if !pluginNamePattern.MatchString(fields[0]) {
		return "", nil, fmt.Errorf("plugin command %q has an invalid plugin name %q (use lowercase letters, digits and dashes)", command, fields[0])
	}
	if strings.HasPrefix(fields[1], "-") {
		return "", nil, fmt.Errorf("plugin command %q must name a command before any flags", command)
	}
	return fields[0], fields[1:], nil
}

// InstallPluginName returns the plugin that installs an app with this method
// and install command: the named plugin for the plugin method (empty when the
// command is malformed), and package-manager-<method> otherwise.
func InstallPluginName(method, command string) string {
	if method == InstallMethodPlugin {
		plugin, _, err := ParsePluginCommand(command)
		if err != nil {
			return ""
		}
		return plugin
	}
	return "package-manager-" + method
}
