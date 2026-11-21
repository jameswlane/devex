//go:build windows

package sdktest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var (
	hostBuildEnv []string
	hostGoBinary string
	hostTempDir  string

	stubOnce sync.Once
	stubPath string
	errStub  error
)

func init() {
	hostBuildEnv = append([]string(nil), os.Environ()...)
	hostGoBinary, _ = exec.LookPath("go")
	hostTempDir = os.TempDir()
}

type stubConfig struct {
	Command     string         `json:"command"`
	BinDir      string         `json:"bin_dir"`
	LogDir      string         `json:"log_dir"`
	EnvKeys     []string       `json:"env_keys"`
	Responses   []stubResponse `json:"responses,omitempty"`
	Fallback    Response       `json:"fallback"`
	Stdin       [][]string     `json:"stdin,omitempty"`
	SudoTargets []string       `json:"sudo_targets,omitempty"`
}

type stubResponse struct {
	Args     []string `json:"args"`
	Response Response `json:"response"`
}

func (r *Recorder) fakePath(command string) string {
	return filepath.Join(r.binDir, command+".exe")
}

func (r *Recorder) writeCommand(command string) {
	r.t.Helper()

	bin, err := ensureWindowsStub()
	if err != nil {
		r.t.Fatalf("build Windows sdktest stub: %v", err)
	}

	target := r.fakePath(command)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err := linkOrCopyFile(bin, target); err != nil {
			r.t.Fatalf("install fake executable %q: %v", command, err)
		}
	}

	cfg := stubConfig{
		Command:  command,
		BinDir:   r.binDir,
		LogDir:   r.logDir,
		EnvKeys:  []string{sdk.UnattendedEnv, "NONINTERACTIVE", "DEBIAN_FRONTEND"},
		Fallback: r.fallbacks[command],
		Stdin:    r.stdin[command],
	}
	for _, scripted := range r.responses[command] {
		cfg.Responses = append(cfg.Responses, stubResponse{
			Args:     scripted.args,
			Response: scripted.response,
		})
	}
	if command == "sudo" {
		for name := range r.responses {
			if name != "sudo" {
				cfg.SudoTargets = append(cfg.SudoTargets, name)
			}
		}
		sort.Strings(cfg.SudoTargets)
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		r.t.Fatalf("marshal fake executable config %q: %v", command, err)
	}
	cfgPath := filepath.Join(r.binDir, command+".sdktest.json")
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		r.t.Fatalf("write fake executable config %q: %v", command, err)
	}
}

func linkOrCopyFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func ensureWindowsStub() (string, error) {
	stubOnce.Do(func() {
		goBin := hostGoBinary
		if goBin == "" {
			var err error
			goBin, err = exec.LookPath("go")
			if err != nil {
				errStub = fmt.Errorf("locate go toolchain: %w", err)
				return
			}
		}
		dir, err := os.MkdirTemp(hostTempDir, "devex-sdktest-stub-*")
		if err != nil {
			errStub = err
			return
		}
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte(windowsStubSource), 0o600); err != nil {
			errStub = err
			return
		}
		out := filepath.Join(dir, "stub.exe")
		cmd := exec.Command(goBin, "build", "-o", out, src)
		cmd.Dir = dir
		cmd.Env = hostBuildEnv
		if output, err := cmd.CombinedOutput(); err != nil {
			errStub = fmt.Errorf("go build stub failed: %w: %s", err, string(output))
			return
		}
		stubPath = out
	})
	return stubPath, errStub
}

const windowsStubSource = `package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Response struct {
	ExitCode int    ` + "`json:\"ExitCode\"`" + `
	Stdout   string ` + "`json:\"Stdout\"`" + `
	Stderr   string ` + "`json:\"Stderr\"`" + `
}

type stubResponse struct {
	Args     []string ` + "`json:\"args\"`" + `
	Response Response ` + "`json:\"response\"`" + `
}

type stubConfig struct {
	Command     string         ` + "`json:\"command\"`" + `
	BinDir      string         ` + "`json:\"bin_dir\"`" + `
	LogDir      string         ` + "`json:\"log_dir\"`" + `
	EnvKeys     []string       ` + "`json:\"env_keys\"`" + `
	Responses   []stubResponse ` + "`json:\"responses,omitempty\"`" + `
	Fallback    Response       ` + "`json:\"fallback\"`" + `
	Stdin       [][]string     ` + "`json:\"stdin,omitempty\"`" + `
	SudoTargets []string       ` + "`json:\"sudo_targets,omitempty\"`" + `
}

func appendBytes(path string, data []byte) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdktest stub: open %s: %v\n", path, err)
		os.Exit(126)
	}
	_, _ = f.Write(data)
	_ = f.Close()
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func emit(resp Response) {
	if resp.Stdout != "" {
		_, _ = io.WriteString(os.Stdout, resp.Stdout)
	}
	if resp.Stderr != "" {
		_, _ = io.WriteString(os.Stderr, resp.Stderr)
	}
	os.Exit(resp.ExitCode)
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdktest stub: executable: %v\n", err)
		os.Exit(126)
	}
	base := strings.TrimSuffix(filepath.Base(exe), ".exe")
	cfgPath := filepath.Join(filepath.Dir(exe), base+".sdktest.json")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sdktest stub: read config %s: %v\n", cfgPath, err)
		os.Exit(126)
	}
	var cfg stubConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "sdktest stub: parse config: %v\n", err)
		os.Exit(126)
	}

	args := os.Args[1:]
	elevated := os.Getenv("DEVEX_SDKTEST_ELEVATED")
	if elevated == "" {
		elevated = "0"
	}

	var buf bytes.Buffer
	buf.WriteString(elevated)
	buf.WriteByte(0)
	buf.WriteString(strconv.Itoa(len(args)))
	buf.WriteByte(0)
	for _, a := range args {
		buf.WriteString(a)
		buf.WriteByte(0)
	}
	appendBytes(filepath.Join(cfg.LogDir, cfg.Command), buf.Bytes())

	for _, key := range cfg.EnvKeys {
		val := os.Getenv(key)
		appendBytes(filepath.Join(cfg.LogDir, cfg.Command+".env."+key), append([]byte(val), 0))
	}

	for i, captured := range cfg.Stdin {
		if equalArgs(captured, args) {
			data, _ := io.ReadAll(os.Stdin)
			appendBytes(filepath.Join(cfg.LogDir, fmt.Sprintf("%s.stdin.%d", cfg.Command, i)), append(data, 0))
		}
	}

	for _, scripted := range cfg.Responses {
		if equalArgs(scripted.Args, args) {
			emit(scripted.Response)
		}
	}

	if cfg.Command == "sudo" {
		dispatchSudo(cfg, args)
		return
	}

	emit(cfg.Fallback)
}

func dispatchSudo(cfg stubConfig, args []string) {
	if len(args) == 1 && args[0] == "-v" {
		os.Exit(0)
	}
	if len(args) == 2 && args[0] == "-n" && args[1] == "true" {
		os.Exit(0)
	}
	if len(args) > 0 && args[0] == "-n" {
		args = args[1:]
	}
	if len(args) == 1 && args[0] == "-v" {
		os.Exit(0)
	}
	if len(args) > 0 && args[0] == "-H" {
		args = args[1:]
		if len(args) > 0 && args[0] == "-u" {
			if len(args) >= 2 {
				args = args[2:]
			} else {
				args = nil
			}
		}
	}
	extraEnv := []string{"DEVEX_SDKTEST_ELEVATED=1"}
	for len(args) > 0 && strings.Contains(args[0], "=") {
		extraEnv = append(extraEnv, args[0])
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) > 0 && args[0] == "env" {
		args = args[1:]
		for len(args) > 0 && strings.Contains(args[0], "=") {
			extraEnv = append(extraEnv, args[0])
			args = args[1:]
		}
	}
	if len(args) > 0 && strings.Contains(args[0], "=") {
		fmt.Fprintf(os.Stderr, "sudo: %s: command not found\n", args[0])
		os.Exit(1)
	}
	if len(args) > 0 {
		target := args[0]
		for _, name := range cfg.SudoTargets {
			fakeExe := filepath.Join(cfg.BinDir, name+".exe")
			fakeBare := filepath.Join(cfg.BinDir, name)
			if target == name || target == fakeExe || target == fakeBare {
				cmd := exec.Command(fakeExe, args[1:]...)
				cmd.Env = append(os.Environ(), extraEnv...)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					var exitErr *exec.ExitError
					if errors.As(err, &exitErr) {
						os.Exit(exitErr.ExitCode())
					}
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
	}
	fmt.Fprintln(os.Stderr, "sdktest: sudo target is not a registered fake")
	os.Exit(127)
}
`
