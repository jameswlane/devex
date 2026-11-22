// Command fakewinget is a native fake winget executable used by Windows
// contract tests so they run without /bin/sh and exercise real 32-bit HRESULT
// exit codes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type callRecord struct {
	Args []string `json:"args"`
}

type scriptedResponse struct {
	Args     []string `json:"args"`
	ExitCode uint32   `json:"exit_code"`
	Stdout   string   `json:"stdout,omitempty"`
	Stderr   string   `json:"stderr,omitempty"`
}

func main() {
	args := os.Args[1:]
	if logPath := os.Getenv("DEVEX_FAKE_WINGET_LOG"); logPath != "" {
		line, err := json.Marshal(callRecord{Args: args})
		if err != nil {
			fmt.Fprintf(os.Stderr, "fakewinget: marshal call: %v\n", err)
			os.Exit(1)
		}
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fakewinget: open log: %v\n", err)
			os.Exit(1)
		}
		_, writeErr := f.Write(append(line, '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			fmt.Fprintf(os.Stderr, "fakewinget: write log: %v / %v\n", writeErr, closeErr)
			os.Exit(1)
		}
	}

	respPath := os.Getenv("DEVEX_FAKE_WINGET_RESPONSES")
	if respPath == "" {
		return
	}
	data, err := os.ReadFile(respPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		fmt.Fprintf(os.Stderr, "fakewinget: read responses: %v\n", err)
		os.Exit(1)
	}
	var responses []scriptedResponse
	if err := json.Unmarshal(data, &responses); err != nil {
		fmt.Fprintf(os.Stderr, "fakewinget: decode responses: %v\n", err)
		os.Exit(1)
	}
	for _, resp := range responses {
		if equalArgs(resp.Args, args) {
			if resp.Stdout != "" {
				_, _ = os.Stdout.WriteString(resp.Stdout)
			}
			if resp.Stderr != "" {
				_, _ = os.Stderr.WriteString(resp.Stderr)
			}
			os.Exit(int(resp.ExitCode))
		}
	}
}

func equalArgs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
