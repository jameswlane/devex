package sdk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

const macOSAuthorizationResultPrefix = "DEVEX_MACOS_AUTHORIZATION_RESULT:"

const macOSAuthorizationScript = `on run argv
	set commandLine to ""
	repeat with argument in argv
		set commandLine to commandLine & (quoted form of (argument as text)) & space
	end repeat
	try
		set commandOutput to do shell script commandLine with administrator privileges
		return "` + macOSAuthorizationResultPrefix + `0" & linefeed & commandOutput
	on error errorMessage number errorNumber
		return "` + macOSAuthorizationResultPrefix + `" & (errorNumber as text) & linefeed & errorMessage
	end try
end run`

type macOSAuthorizationResult struct {
	status int
	output string
}

type macOSAuthorizedCommandError struct {
	status  int
	message string
}

func (e *macOSAuthorizedCommandError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("authorized macOS command exited with status %d", e.status)
	}
	return fmt.Sprintf("authorized macOS command exited with status %d: %s", e.status, e.message)
}

func (e *macOSAuthorizedCommandError) ExitCode() int { return e.status }

type macOSAuthorizationExecutor func(context.Context, []string, io.Reader) ([]byte, []byte, error)

type macOSAuthorizationAdapter struct {
	execute macOSAuthorizationExecutor
}

func (a macOSAuthorizationAdapter) Run(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
	if !plan.Elevate {
		return (UnixSudoAdapter{}).Run(ctx, plan, streams)
	}

	args := []string{"-e", macOSAuthorizationScript, "--", plan.Name}
	args = append(args, plan.Args...)
	execute := a.execute
	if execute == nil {
		execute = executeMacOSAuthorization
	}
	stdout, stderr, err := execute(ctx, args, streams.Stdin)
	if len(stderr) > 0 && streams.Stderr != nil {
		if _, writeErr := streams.Stderr.Write(stderr); writeErr != nil {
			return fmt.Errorf("forward osascript stderr: %w", writeErr)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return NewAuthorizationRequiredError("start macOS administrator authorization; ensure /usr/bin/osascript is available in an interactive console session", err)
		}
		return err
	}

	result, err := parseMacOSAuthorizationResult(stdout)
	if err != nil {
		return err
	}
	if result.status == 0 {
		if err := writeMacOSAuthorizationOutput(streams.Stdout, result.output); err != nil {
			return fmt.Errorf("forward authorized command stdout: %w", err)
		}
		return nil
	}
	if err := writeMacOSAuthorizationOutput(streams.Stderr, result.output); err != nil {
		return fmt.Errorf("forward authorized command stderr: %w", err)
	}
	if isMacOSAuthorizationDenial(result.status) {
		cause := errors.New(result.output)
		if result.output == "" {
			cause = fmt.Errorf("AppleScript authorization error %d", result.status)
		}
		return NewAuthorizationDeniedError("macOS administrator authorization was denied or canceled; rerun and approve the prompt", result.status, cause)
	}
	if result.status > 0 {
		return &macOSAuthorizedCommandError{status: result.status, message: result.output}
	}
	return fmt.Errorf("macOS authorization AppleScript error %d: %s", result.status, result.output)
}

func executeMacOSAuthorization(ctx context.Context, args []string, stdin io.Reader) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", args...)
	cmd.Stdin = stdin
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func parseMacOSAuthorizationResult(data []byte) (macOSAuthorizationResult, error) {
	header, output, found := strings.Cut(string(data), "\n")
	if !found || !strings.HasPrefix(header, macOSAuthorizationResultPrefix) {
		return macOSAuthorizationResult{}, fmt.Errorf("decode macOS authorization result: missing structured status")
	}
	status, err := strconv.Atoi(strings.TrimPrefix(header, macOSAuthorizationResultPrefix))
	if err != nil {
		return macOSAuthorizationResult{}, fmt.Errorf("decode macOS authorization result status: %w", err)
	}
	return macOSAuthorizationResult{status: status, output: output}, nil
}

func isMacOSAuthorizationDenial(status int) bool {
	return status == -128 || status == -60005 || status == -60006
}

func writeMacOSAuthorizationOutput(w io.Writer, output string) error {
	if output == "" || w == nil {
		return nil
	}
	_, err := io.WriteString(w, output)
	return err
}
