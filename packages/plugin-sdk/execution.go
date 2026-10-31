package sdk

import "os/exec"

// NewCommandContext constructs all subprocesses launched by the SDK. It defaults
// to exec.CommandContext and may be replaced before concurrent use, for example
// by sdktest to reject commands outside its fake-executable allowlist.
// A replacement must preserve context cancellation and return a non-nil Cmd;
// rejected commands carry Cmd.Err so Start, Run, and Output fail before execution.
var NewCommandContext = exec.CommandContext
