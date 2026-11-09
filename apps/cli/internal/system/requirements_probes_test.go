package system_test

import (
	"context"
	"errors"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/system"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Requirement probes", func() {
	DescribeTable("classifies read-only probes", func(memory string, disk int, probeErr error, req types.SystemRequirements, status system.ValidationStatus) {
		validator := system.NewRequirementsValidatorWithProbes(func(command string) (string, error) {
			if strings.Contains(command, "MemTotal") {
				return memory, probeErr
			}
			if strings.Contains(command, "--version") {
				return "git version 2.45.0", probeErr
			}
			return "", probeErr
		}, func(string) (int, error) { return disk, nil })
		results, err := validator.ValidateRequirements("example", req)
		Expect(err).NotTo(HaveOccurred())
		Expect(results).NotTo(BeEmpty())
		Expect(validator.HasFailures(results)).To(Equal(status == system.ValidationFailed))
		for _, result := range results {
			Expect(result.Status).To(Equal(status))
		}
	},
		Entry("compatible memory and disk", "MemTotal: 4194304 kB", 8192, nil, types.SystemRequirements{MinMemoryMB: 2048, MinDiskSpaceMB: 1024}, system.ValidationPassed),
		Entry("insufficient resources", "MemTotal: 1024 kB", 1, nil, types.SystemRequirements{MinMemoryMB: 2048, MinDiskSpaceMB: 1024}, system.ValidationFailed),
		Entry("absent command", "", 0, errors.New("missing"), types.SystemRequirements{RequiredCommands: []string{"missing"}}, system.ValidationFailed),
		Entry("unsupported tools", "", 0, nil, types.SystemRequirements{RubyVersion: "3.0+", JavaVersion: "17+", KubectlVersion: "1.0+"}, system.ValidationSkipped),
		Entry("unavailable memory probe", "", 0, errors.New("unavailable"), types.SystemRequirements{MinMemoryMB: 1024}, system.ValidationWarning),
		Entry("supported version check", "", 0, nil, types.SystemRequirements{GitVersion: "2.0+"}, system.ValidationPassed),
		Entry("old supported version", "", 0, nil, types.SystemRequirements{GitVersion: "99.0+"}, system.ValidationFailed),
	)
	It("retains supported version checks using fake tool output", func() {
		outputs := map[string]string{
			"which docker": "/usr/bin/docker",
			"docker version --format '{{.Client.Version}}'": "26.0.0",
			"docker compose version --short":                "2.24.0",
			"go version":                                    "go version go1.22.0 linux/amd64",
			"node --version":                                "v20.0.0",
			"python3 --version":                             "Python 3.12.0",
			"git --version":                                 "git version 2.45.0",
		}
		validator := system.NewRequirementsValidatorWithProbes(func(command string) (string, error) {
			output, ok := outputs[command]
			Expect(ok).To(BeTrue(), command)
			return output, nil
		}, nil)
		results, err := validator.ValidateRequirements("example", types.SystemRequirements{DockerVersion: "20.0+", DockerComposeVersion: "2.0+", GoVersion: "1.20+", NodeVersion: "18.0+", PythonVersion: "3.10+", GitVersion: "2.30+"})
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(HaveLen(6))
		for _, result := range results {
			Expect(result.Status).To(Equal(system.ValidationPassed), result.Message)
		}
	})
	It("permits warning-only disk probes and provides a remedy", func() {
		validator := system.NewRequirementsValidatorWithProbes(func(string) (string, error) { return "", errors.New("unavailable") }, func(string) (int, error) { return 0, errors.New("unavailable") })
		results, err := validator.ValidateRequirements("example", types.SystemRequirements{MinDiskSpaceMB: 1024})
		Expect(err).NotTo(HaveOccurred())
		Expect(validator.HasFailures(results)).To(BeFalse())
		Expect(results).To(HaveLen(1))
		Expect(results[0].Status).To(Equal(system.ValidationWarning))
		Expect(results[0].Suggestion).NotTo(BeEmpty())
	})
	It("never sends service options to a probe", func() {
		validator := system.NewRequirementsValidatorWithProbes(func(string) (string, error) {
			Fail("invalid service must not run a command")
			return "", nil
		}, nil)
		results, err := validator.ValidateRequirements("example", types.SystemRequirements{RequiredServices: []string{"--full-restart"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(validator.HasFailures(results)).To(BeTrue())
	})
	It("quotes configured command and service names as literal shell arguments", func() {
		var commands []string
		validator := system.NewRequirementsValidatorWithProbes(func(command string) (string, error) {
			commands = append(commands, command)
			return "", nil
		}, nil)
		_, err := validator.ValidateRequirements("example", types.SystemRequirements{RequiredCommands: []string{"tool'; touch /tmp/unwanted"}, RequiredServices: []string{"db; reboot"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(commands).To(Equal([]string{"command -v 'tool'\"'\"'; touch /tmp/unwanted'", "systemctl is-active -- 'db; reboot'"}))
	})
	It("reports skipped requirements and permits installation", func() {
		var messages []string
		err := system.CheckApplicationRequirements(context.Background(), "example", types.SystemRequirements{RubyVersion: "3.0+"}, func(message string) { messages = append(messages, message) })
		Expect(err).NotTo(HaveOccurred())
		Expect(messages).To(ContainElement(And(ContainSubstring("SKIPPED Ruby 3.0+"), ContainSubstring("Manually verify"))))
	})
	It("returns cancellation before probing", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(system.CheckApplicationRequirements(ctx, "example", types.SystemRequirements{}, nil)).To(MatchError(context.Canceled))
	})
})
