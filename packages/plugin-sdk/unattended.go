package sdk

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

// UnattendedEnv is the backwards-compatible host-to-plugin execution contract.
const UnattendedEnv = "DEVEX_UNATTENDED"

// Unattended reports whether this plugin must execute without user input.
// Hosts set DEVEX_UNATTENDED=1; CI=true also enables unattended execution.
func Unattended() bool {
	return os.Getenv(UnattendedEnv) == "1" || strings.EqualFold(os.Getenv("CI"), "true")
}

type unattendedContextKey struct{}

// WithUnattended requests unattended execution without changing process globals.
func WithUnattended(ctx context.Context) context.Context {
	return context.WithValue(ctx, unattendedContextKey{}, true)
}

func unattendedContext(ctx context.Context) bool {
	enabled, _ := ctx.Value(unattendedContextKey{}).(bool)
	return enabled || Unattended()
}

// unattendedInput returns the stdin an unattended operation may read. It closes
// inherited process handles (a terminal or a host pipe), which could block on
// or answer prompts, but keeps data the caller supplies in memory, such as a
// file body piped to tee: that is operation input, not user input.
func unattendedInput(stdin io.Reader) io.Reader {
	if _, inherited := stdin.(*os.File); inherited {
		return nil
	}
	return stdin
}

func operationEnvironment(request ExecutionRequest) []string {
	environment := append([]string(nil), request.Env...)
	if request.Unattended {
		environment = append(environment, UnattendedEnv+"=1")
	}
	return environment
}

func configureOperationCommand(cmd *exec.Cmd, request ExecutionRequest) {
	cmd.Dir = request.Dir
	if environment := operationEnvironment(request); len(environment) > 0 {
		cmd.Env = append(os.Environ(), environment...)
	}
	if request.Unattended {
		cmd.Stdin = nil
	}
}

func validEnvironmentAssignment(entry string) bool {
	key, _, found := strings.Cut(entry, "=")
	if !found || key == "" || strings.ContainsRune(entry, 0) {
		return false
	}
	for i, char := range key {
		if char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || i > 0 && char >= '0' && char <= '9' {
			continue
		}
		return false
	}
	return true
}
