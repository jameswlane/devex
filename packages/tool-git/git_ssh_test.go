package main_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"

	main "github.com/jameswlane/devex/packages/tool-git"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Explicit SSH setup", func() {
	var dir, path string
	var output bytes.Buffer
	var calls int
	var runner main.SSHRunner
	BeforeEach(func() {
		var err error
		dir, err = os.MkdirTemp("", "devex-ssh-test-")
		Expect(err).NotTo(HaveOccurred())
		path = filepath.Join(dir, "id_ed25519")
		output.Reset()
		calls = 0
		runner = func(_ context.Context, command string, args ...string) error {
			calls++
			Expect(command).To(Equal("ssh-keygen"))
			Expect(args[:5]).To(Equal([]string{"-q", "-t", "ed25519", "-N", ""}))
			key := args[len(args)-1]
			Expect(os.WriteFile(key, []byte("PRIVATE SENTINEL"), 0600)).To(Succeed())
			return os.WriteFile(key+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGQA8CyEwhkM/5SLJr/WKRQWKUxSuXMYxHdL2UfrOeZ5 comment"), 0644)
		}
	})
	AfterEach(func() { Expect(os.RemoveAll(dir)).To(Succeed()) })
	It("previews generation without invoking subprocesses or writing files", func() {
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path}, &output, runner)).To(Succeed())
		Expect(calls).To(BeZero())
		Expect(output.String()).To(ContainSubstring("unencrypted"))
		_, err := os.Stat(path)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
	It("generates secure files and displays only the public key", func() {
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).To(Succeed())
		Expect(calls).To(Equal(1))
		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		Expect(output.String()).To(ContainSubstring("ssh-ed25519"))
		Expect(output.String()).NotTo(ContainSubstring("PRIVATE SENTINEL"))
		output.Reset()
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "existing", Path: path}, &output, runner)).To(Succeed())
		Expect(calls).To(Equal(1))
	})
	It("never overwrites either member of an existing pair", func() {
		for _, target := range []string{path, path + ".pub"} {
			Expect(os.WriteFile(target, []byte("original"), 0600)).To(Succeed())
			Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).NotTo(Succeed())
			data, err := os.ReadFile(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("original"))
			Expect(os.Remove(target)).To(Succeed())
		}
		Expect(calls).To(BeZero())
	})
	It("propagates subprocess failures and cleans staging files", func() {
		runner = func(context.Context, string, ...string) error { return errors.New("generation failed") }
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).To(MatchError("generation failed"))
		entries, err := os.ReadDir(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("rejects unsafe directories, symlinks, private permissions and malformed public files", func() {
		Expect(os.Chmod(dir, 0755)).To(Succeed())
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).NotTo(Succeed())
		Expect(os.Chmod(dir, 0700)).To(Succeed())
		Expect(os.WriteFile(path, []byte("secret"), 0644)).To(Succeed())
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "existing", Path: path}, &output, runner)).NotTo(Succeed())
		Expect(os.Chmod(path, 0600)).To(Succeed())
		Expect(os.WriteFile(path+".pub", []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nsecret"), 0644)).To(Succeed())
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "existing", Path: path}, &output, runner)).NotTo(Succeed())
		Expect(output.String()).NotTo(ContainSubstring("secret"))
		Expect(os.Remove(path)).To(Succeed())
		Expect(os.Symlink(path+".pub", path)).To(Succeed())
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "existing", Path: path}, &output, runner)).NotTo(Succeed())
	})
	It("rejects a destination created during key generation without replacing it", func() {
		original := runner
		runner = func(ctx context.Context, command string, args ...string) error {
			Expect(os.WriteFile(path, []byte("racing key"), 0600)).To(Succeed())
			return original(ctx, command, args...)
		}
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).NotTo(Succeed())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("racing key"))
	})
	It("rolls back its own private key if public publication loses a race", func() {
		original := runner
		runner = func(ctx context.Context, command string, args ...string) error {
			Expect(os.WriteFile(path+".pub", []byte("other public key"), 0644)).To(Succeed())
			return original(ctx, command, args...)
		}
		Expect(main.SetupSSH(context.Background(), main.SSHOptions{Mode: "generate", Path: path, Apply: true}, &output, runner)).NotTo(Succeed())
		_, err := os.Stat(path)
		Expect(os.IsNotExist(err)).To(BeTrue())
		data, err := os.ReadFile(path + ".pub")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("other public key"))
	})
	It("rejects malformed options and missing files without generating anything", func() {
		for _, opts := range []main.SSHOptions{{Mode: "invalid", Path: path}, {Mode: "generate", Path: "relative"}, {Mode: "existing", Path: path}} {
			Expect(main.SetupSSH(context.Background(), opts, &output, runner)).NotTo(Succeed())
		}
		plugin := main.NewGitPlugin()
		Expect(plugin.HandleSSH(context.Background(), []string{"--path"})).NotTo(Succeed())
		Expect(plugin.HandleSSH(context.Background(), []string{"--unknown"})).NotTo(Succeed())
		Expect(calls).To(BeZero())
	})

})
