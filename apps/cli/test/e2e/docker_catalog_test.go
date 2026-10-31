//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Docker catalog install lifecycle", Ordered, Label("destructive", "pm:apt", "docker"), func() {
	var home, config string
	BeforeAll(func() {
		if os.Getenv("DEVEX_E2E_DESTRUCTIVE") != "1" || os.Getenv("DEVEX_E2E_DOCKER") != "1" {
			Skip("requires DEVEX_E2E_DESTRUCTIVE=1 and DEVEX_E2E_DOCKER=1 in a privileged disposable sandbox")
		}
		if currentDistro() != "debian-12" {
			Skip("Docker catalog lifecycle runs on Debian 12")
		}
		home = sandbox.isolatedHome()
		// The runner stages the actual catalog entry; do not duplicate its setup
		// in a hand-maintained inline fixture.
		catalog := filepath.Join(envDefault("DEVEX_E2E_TESTDATA", "/opt/devex-e2e/testdata"), "catalog", "docker.yaml")
		data, err := os.ReadFile(catalog)
		Expect(err).NotTo(HaveOccurred())
		var app map[string]any
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		data, err = yaml.Marshal(map[string]any{"applications": []any{app}})
		Expect(err).NotTo(HaveOccurred())
		config = filepath.Join(home, "docker-config.yaml")
		Expect(os.WriteFile(config, data, 0600)).To(Succeed())
	})

	It("installs Docker from the official Debian repository and sets group membership", func() {
		output, err := sandbox.run(home, sandbox.binary, packageInstallTimeout, nil,
			"install", "--non-interactive", "--config", config)
		Expect(err).NotTo(HaveOccurred(), "Docker install failed:\n%s", output)
		Expect(output).To(ContainSubstring("Log out and log back in"))
		source, err := os.ReadFile("/etc/apt/sources.list.d/docker.list")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(source)).To(ContainSubstring("https://download.docker.com/linux/debian bookworm stable"))
		for _, pkg := range []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"} {
			dpkgInstalled(pkg, "docker", "--version")(home)
		}
		output, err = sandbox.run(home, "bash", time.Minute, nil, "-c", `id -nG "$(id -un)"`)
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(MatchRegexp(`\bdocker\b`))
	})

	It("runs docker version as the invoking user in a fresh docker-group session", func() {
		// Container sandboxes have no systemd. Start the installed engine using
		// vfs and no bridge so this tests Docker without requiring overlayfs.
		output, err := sandbox.run(home, "sudo", time.Minute, nil, "-n", "bash", "-c",
			`nohup dockerd --storage-driver=vfs --iptables=false --bridge=none >/tmp/devex-e2e-dockerd.log 2>&1 & echo $! >/tmp/devex-e2e-dockerd.pid`)
		Expect(err).NotTo(HaveOccurred(), "start dockerd:\n%s", output)
		DeferCleanup(func() {
			output, err := sandbox.run(home, "sudo", time.Minute, nil, "-n", "bash", "-c", `kill "$(cat /tmp/devex-e2e-dockerd.pid)"`)
			Expect(err).NotTo(HaveOccurred(), "stop dockerd:\n%s", output)
		})
		Eventually(func(g Gomega) {
			output, err := sandbox.run(home, "sg", time.Minute, nil, "docker", "-c", "docker version")
			g.Expect(err).NotTo(HaveOccurred(), "docker version:\n%s", output)
			g.Expect(output).To(ContainSubstring("Server:"))
			g.Expect(output).To(ContainSubstring("Client:"))
		}).WithTimeout(time.Minute).WithPolling(time.Second).Should(Succeed())
	})

	It("removes the packages while preserving Docker data", func() {
		output, err := sandbox.run(home, "sudo", time.Minute, nil, "-n", "touch", "/var/lib/docker/devex-e2e-preserve")
		Expect(err).NotTo(HaveOccurred(), "%s", output)
		output, err = sandbox.run(home, sandbox.binary, packageInstallTimeout, nil,
			"uninstall", "--app", "Docker", "--force", "--config", config)
		Expect(err).NotTo(HaveOccurred(), "Docker uninstall failed:\n%s", output)
		dpkgRemoved("docker-ce-cli", "docker")(home)
		output, err = sandbox.run(home, "sudo", time.Minute, nil, "-n", "test", "-f", "/var/lib/docker/devex-e2e-preserve")
		Expect(err).NotTo(HaveOccurred(), "Docker data must survive uninstall:\n%s", output)
	})
})
