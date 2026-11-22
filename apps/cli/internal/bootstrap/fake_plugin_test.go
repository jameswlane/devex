package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/gomega"
)

type fakePluginConfig struct {
	Name       string `json:"name"`
	Mode       string `json:"mode"`
	MarkerPath string `json:"marker_path,omitempty"`
}

var (
	fakePluginHostTempDir string
	fakePluginHostGoBin   string
	fakePluginHostEnv     []string

	fakePluginOnce sync.Once
	fakePluginBin  string
	errFakePlugin  error
)

func init() {
	fakePluginHostTempDir = os.TempDir()
	fakePluginHostGoBin, _ = exec.LookPath("go")
	fakePluginHostEnv = append([]string(nil), os.Environ()...)
}

const fakePluginSource = `package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Name       string ` + "`json:\"name\"`" + `
	Mode       string ` + "`json:\"mode\"`" + `
	MarkerPath string ` + "`json:\"marker_path,omitempty\"`" + `
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := strings.TrimSuffix(filepath.Base(exe), ".exe")
	cfgPath := filepath.Join(filepath.Dir(exe), ".fake-plugin-"+base+".json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "--plugin-info" {
		fmt.Printf("{\"name\":%q,\"version\":\"1.0.0\"}\n", cfg.Name)
		os.Exit(0)
	}
	switch cfg.Mode {
	case "apt-retry":
		if _, err := os.Stat(cfg.MarkerPath); os.IsNotExist(err) {
			if err := os.WriteFile(cfg.MarkerPath, nil, 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "E: Failed to fetch https://repo/git.deb 503 Service Unavailable")
			os.Exit(100)
		}
		fmt.Print("{\"installed\":true}\n")
	case "apt-fail":
		if err := appendLine(cfg.MarkerPath, "attempt\n"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "E: Failed to fetch https://repo/git.deb 503 Service Unavailable")
		os.Exit(100)
	case "dnf-retry":
		if err := appendLine(cfg.MarkerPath, "attempt\n"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		raw, err := os.ReadFile(cfg.MarkerPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if bytes.Count(raw, []byte("\n")) == 1 {
			fmt.Fprintln(os.Stderr, ">>> Status code: 503 for http://repo/git.rpm (IP: 127.0.0.1)")
			fmt.Fprintln(os.Stderr, "Failed to download packages")
			fmt.Fprintln(os.Stderr, "Error: exit status 1")
			os.Exit(1)
		}
	case "touch":
		if err := os.WriteFile(cfg.MarkerPath, nil, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "unattended":
		val := os.Getenv("DEVEX_UNATTENDED")
		if val == "" {
			val = "0"
		}
		fmt.Printf("%s\n", val)
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			fmt.Printf("%s\n", scanner.Text())
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown fake plugin mode: %s\n", cfg.Mode)
		os.Exit(1)
	}
}
`

func ensureFakePluginBinary() (string, error) {
	fakePluginOnce.Do(func() {
		dir, err := os.MkdirTemp(fakePluginHostTempDir, "devex-bootstrap-fake-plugin-*")
		if err != nil {
			errFakePlugin = err
			return
		}
		src := filepath.Join(dir, "main.go")
		if err := os.WriteFile(src, []byte(fakePluginSource), 0o600); err != nil {
			errFakePlugin = err
			return
		}
		bin := testhelper.Executable(dir, "fake-plugin")
		goBin := fakePluginHostGoBin
		if goBin == "" {
			goBin = "go"
		}
		cmd := exec.Command(goBin, "build", "-o", bin, src)
		cmd.Env = fakePluginHostEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			errFakePlugin = fmt.Errorf("build fake plugin: %w: %s", err, strings.TrimSpace(string(out)))
			return
		}
		fakePluginBin = bin
	})
	return fakePluginBin, errFakePlugin
}

func writeFakePlugin(dir string, cfg fakePluginConfig) {
	bin, err := ensureFakePluginBinary()
	Expect(err).NotTo(HaveOccurred())
	raw, err := os.ReadFile(bin)
	Expect(err).NotTo(HaveOccurred())

	base := "devex-plugin-" + cfg.Name
	target := testhelper.Executable(dir, base)
	Expect(os.WriteFile(target, raw, 0o700)).To(Succeed())

	cfgBytes, err := json.Marshal(cfg)
	Expect(err).NotTo(HaveOccurred())
	cfgPath := filepath.Join(dir, ".fake-plugin-"+base+".json")
	Expect(os.WriteFile(cfgPath, cfgBytes, 0o600)).To(Succeed())
}
