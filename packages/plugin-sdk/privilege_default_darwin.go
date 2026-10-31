//go:build darwin

package sdk

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

const macOSAuthorizationScript = `on run argv
	set commandLine to ""
	repeat with argument in argv
		set commandLine to commandLine & (quoted form of (argument as text)) & space
	end repeat
	do shell script commandLine with administrator privileges
end run`

var macOSAuthorizationDenial = regexp.MustCompile(`\((-128|-60005|-60006)\)`)

type macOSAuthorizationAdapter struct{}

func (macOSAuthorizationAdapter) Run(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
	if !plan.Elevate {
		return (UnixSudoAdapter{}).Run(ctx, plan, streams)
	}

	args := []string{"-e", macOSAuthorizationScript, "--", plan.Name}
	args = append(args, plan.Args...)
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", args...)
	cmd.Stdin, cmd.Stdout = streams.Stdin, streams.Stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if streams.Stderr != nil {
		cmd.Stderr = io.MultiWriter(streams.Stderr, &stderr)
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return NewAuthorizationRequiredError("start macOS administrator authorization; ensure /usr/bin/osascript is available in an interactive console session", err)
	}
	if match := macOSAuthorizationDenial.FindSubmatch(stderr.Bytes()); match != nil {
		status, _ := strconv.Atoi(string(match[1]))
		return NewAuthorizationDeniedError("macOS administrator authorization was denied or canceled; rerun and approve the prompt", status, err)
	}
	return err
}

func defaultPrivilegeAdapter() PrivilegeAdapter { return macOSAuthorizationAdapter{} }
func defaultElevated() bool                     { return os.Geteuid() == 0 }
