package templates

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	templateIDPattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	stepIDPattern         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	pluginNamePattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	actionNamePattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	resourceNamePattern   = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.@-]*$`)
	commandBasePattern    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
	environmentKeyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func validateTemplateID(id string) error {
	if !templateIDPattern.MatchString(id) {
		return fmt.Errorf("template id must start with a letter or number and contain only letters, numbers, dots, underscores, and hyphens")
	}
	return nil
}

func validateStepID(id string) error {
	if !stepIDPattern.MatchString(id) {
		return fmt.Errorf("step id must start with a letter or number and contain only letters, numbers, dots, underscores, and hyphens")
	}
	return nil
}

func validatePluginName(name string) error {
	if !pluginNamePattern.MatchString(name) {
		return fmt.Errorf("plugin name must start with a letter or number and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}

func validateLifecycleCommand(name string) error {
	if !actionNamePattern.MatchString(name) {
		return fmt.Errorf("command/action name must start with a letter or number and contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}

func validateShellCommandBase(command string) error {
	if isTemplated(command) {
		return fmt.Errorf("shell command names cannot contain template interpolation; pass dynamic values as typed args")
	}
	if filepath.Base(command) != command {
		return fmt.Errorf("shell command must be a base executable name, not a path")
	}
	if !commandBasePattern.MatchString(command) {
		return fmt.Errorf("shell command must start with a letter or number and contain only letters, numbers, dots, underscores, plus signs, and hyphens")
	}
	return nil
}

func validateEnvironment(env map[string]string) error {
	for key, value := range env {
		if !environmentKeyPattern.MatchString(key) {
			return fmt.Errorf("invalid environment variable name '%s'", key)
		}
		if hasUnsafeScalar(value) {
			return fmt.Errorf("environment variable '%s' contains unsafe control characters", key)
		}
	}
	return nil
}

func validatePathValue(label string, path string, required bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		if required {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
	if strings.ContainsRune(path, '\x00') || strings.ContainsAny(path, "\r\n") {
		return fmt.Errorf("%s contains unsafe control characters", label)
	}
	if strings.ContainsAny(path, "|;&`$<>") {
		return fmt.Errorf("%s contains shell metacharacters", label)
	}
	if !isTemplated(path) && containsPathTraversal(path) {
		return fmt.Errorf("%s must not contain path traversal", label)
	}
	return nil
}

func validateServiceName(name string) error {
	if !resourceNamePattern.MatchString(name) {
		return fmt.Errorf("service name must start with a letter or number and contain only letters, numbers, dots, underscores, at signs, and hyphens")
	}
	return nil
}

func validateContainerName(name string) error {
	if !resourceNamePattern.MatchString(name) {
		return fmt.Errorf("container name must start with a letter or number and contain only letters, numbers, dots, underscores, at signs, and hyphens")
	}
	return nil
}

func validatePortMapping(mapping string) error {
	hostAndContainer := strings.TrimSpace(mapping)
	if hostAndContainer == "" {
		return fmt.Errorf("port mapping is required")
	}
	if strings.ContainsAny(hostAndContainer, " \t\r\n|;&`$<>") {
		return fmt.Errorf("port mapping contains unsafe characters")
	}
	if slash := strings.LastIndex(hostAndContainer, "/"); slash >= 0 {
		proto := hostAndContainer[slash+1:]
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("port mapping protocol must be tcp or udp")
		}
		hostAndContainer = hostAndContainer[:slash]
	}
	parts := strings.Split(hostAndContainer, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return fmt.Errorf("port mapping must be host:container or ip:host:container")
	}
	if len(parts) == 3 && parts[0] == "" {
		return fmt.Errorf("port mapping host ip is empty")
	}
	for _, port := range parts[len(parts)-2:] {
		if err := validatePortNumber(port); err != nil {
			return err
		}
	}
	return nil
}

func validatePortNumber(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port mapping ports must be numbers from 1 to 65535")
	}
	return nil
}

func validateShellRequest(command string, args []string) error {
	if err := validateShellCommandBase(command); err != nil {
		return err
	}
	for _, arg := range args {
		if err := validateShellArg(arg); err != nil {
			return err
		}
	}
	return nil
}

func validateShellArg(arg string) error {
	if strings.ContainsRune(arg, '\x00') || strings.ContainsAny(arg, "\r\n") {
		return fmt.Errorf("shell argument contains unsafe control characters")
	}
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return nil
	}
	if isShellOperator(trimmed) {
		return fmt.Errorf("shell argument must not be a shell operator")
	}
	if strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "&") ||
		strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "<") {
		return fmt.Errorf("shell argument must not start with a shell operator")
	}
	if strings.HasPrefix(trimmed, "$(") || strings.HasPrefix(trimmed, "`") {
		return fmt.Errorf("shell argument must not contain command substitution")
	}
	return nil
}

func isShellOperator(value string) bool {
	switch value {
	case "|", "||", "&", "&&", ";", ">", ">>", "<", "<<", "2>":
		return true
	default:
		return false
	}
}

func validateDockerLikeArgs(args []string) error {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case strings.HasPrefix(arg, "--name="):
			value := strings.TrimPrefix(arg, "--name=")
			if isTemplated(value) {
				continue
			}
			if err := validateContainerName(value); err != nil {
				return err
			}
		case arg == "--name" && index+1 < len(args):
			index++
			if isTemplated(args[index]) {
				continue
			}
			if err := validateContainerName(args[index]); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "--port="):
			value := strings.TrimPrefix(arg, "--port=")
			if isTemplated(value) {
				continue
			}
			if err := validatePortMapping(value); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "-p="):
			value := strings.TrimPrefix(arg, "-p=")
			if isTemplated(value) {
				continue
			}
			if err := validatePortMapping(value); err != nil {
				return err
			}
		case (arg == "--port" || arg == "-p") && index+1 < len(args):
			index++
			if isTemplated(args[index]) {
				continue
			}
			if err := validatePortMapping(args[index]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePlannedPluginRequest(request ExecutionRequest) error {
	if err := validatePluginName(request.Plugin); err != nil {
		return err
	}
	if err := validateLifecycleCommand(request.Command); err != nil {
		return err
	}
	if err := validateEnvironment(request.Env); err != nil {
		return err
	}
	if err := validatePathValue("plugin working_dir", request.WorkingDir, false); err != nil {
		return err
	}
	return validateDockerLikeArgs(request.Args)
}

func validatePlannedAppRequest(request ExecutionRequest) error {
	if err := validatePluginName(request.Plugin); err != nil {
		return err
	}
	if err := validateLifecycleCommand(request.Command); err != nil {
		return err
	}
	return validateDockerLikeArgs(request.Args)
}

func validatePlannedShellRequest(request ExecutionRequest) error {
	if request.Plugin != "tool-shell" {
		return fmt.Errorf("shell steps must dispatch through tool-shell")
	}
	if err := validatePathValue("shell working_dir", request.WorkingDir, false); err != nil {
		return err
	}
	if err := validateEnvironment(request.Env); err != nil {
		return err
	}
	return validateShellRequest(request.Command, request.Args)
}

func validatePlannedFileRequest(request ExecutionRequest) error {
	switch request.Command {
	case "write", "copy", "remove", "mkdir":
	default:
		return fmt.Errorf("unsupported file operation '%s'", request.Command)
	}
	source, _ := request.Metadata["source"].(string)
	destination, _ := request.Metadata["destination"].(string)
	if err := validatePathValue("file source", source, request.Command == "copy"); err != nil {
		return err
	}
	if err := validatePathValue("file destination", destination, request.Command != "remove"); err != nil {
		return err
	}
	if request.Command == "remove" && source == "" && destination == "" {
		return fmt.Errorf("file remove requires source or destination")
	}
	return nil
}

func validatePlannedServiceRequest(request ExecutionRequest) error {
	switch request.Command {
	case "enable", "disable", "start", "stop", "restart", "status":
	default:
		return fmt.Errorf("unsupported service action '%s'", request.Command)
	}
	if len(request.Args) != 1 {
		return fmt.Errorf("service step requires exactly one service name")
	}
	return validateServiceName(request.Args[0])
}

func validateResourceDeclarations(resources []TemplateResource) error {
	for _, resource := range resources {
		if strings.TrimSpace(resource.Type) == "" {
			return fmt.Errorf("resource type is required")
		}
		if strings.TrimSpace(resource.Name) == "" {
			return fmt.Errorf("resource name is required")
		}
		if !isTemplated(resource.Name) && !resourceNamePattern.MatchString(resource.Name) {
			return fmt.Errorf("resource name must start with a letter or number and contain only letters, numbers, dots, underscores, at signs, and hyphens")
		}
	}
	return nil
}

func validateOnError(value string) error {
	switch ErrorPolicy(value) {
	case "", ErrorPolicyStop, ErrorPolicyContinue, ErrorPolicyRollback:
		return nil
	default:
		return fmt.Errorf("unsupported on_error value '%s'", value)
	}
}

func containsPathTraversal(path string) bool {
	for _, part := range strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return true
		}
	}
	return false
}

func hasUnsafeScalar(value string) bool {
	return strings.ContainsRune(value, '\x00') || strings.ContainsAny(value, "\r\n|;&`$")
}

func isTemplated(value string) bool {
	return strings.Contains(value, "{{") || strings.Contains(value, "}}")
}
