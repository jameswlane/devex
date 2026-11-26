package setup_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/winprereq"
)

var _ = Describe("Windows prerequisites preflight", func() {
	var (
		values    map[string]uint32
		denied    map[string]bool
		writes    []string
		onPath    map[string]bool
		installed [][]string
		out       *bytes.Buffer
		preflight setup.WindowsPreflight
	)

	BeforeEach(func() {
		values = map[string]uint32{}
		denied = map[string]bool{}
		writes = nil
		onPath = map[string]bool{}
		installed = nil
		out = &bytes.Buffer{}
		preflight = setup.WindowsPreflight{
			Probe: winprereq.Probe{
				ReadDWORD: func(_, value string) (uint32, error) {
					if denied[value] {
						return 0, errors.New("Access is denied.")
					}
					data, ok := values[value]
					if !ok {
						return 0, fmt.Errorf("missing: %w", fs.ErrNotExist)
					}
					return data, nil
				},
				WriteDWORD: func(_, value string, data uint32) error {
					writes = append(writes, value)
					values[value] = data
					return nil
				},
				LookPath: func(file string) (string, error) {
					if onPath[file] {
						return file + ".exe", nil
					}
					return "", errors.New("not found")
				},
			},
			Out: out,
			InstallApps: func(_ context.Context, names []string) error {
				installed = append(installed, names)
				return nil
			},
		}
	})

	allOn := func() {
		values["AllowDevelopmentWithoutDevLicense"] = 1
		values["LongPathsEnabled"] = 1
		onPath["pwsh"] = true
		onPath["wt"] = true
	}

	It("reports everything on and changes nothing", func() {
		allOn()
		preflight.Elevated = true
		preflight.Selected = map[string]bool{winprereq.LongPaths: true}
		Expect(preflight.Run(context.Background())).To(Succeed())
		Expect(out.String()).To(ContainSubstring("✓ Developer Mode: on"))
		Expect(out.String()).To(ContainSubstring("✓ Windows Terminal: on"))
		Expect(out.String()).NotTo(ContainSubstring("Manual steps"))
		Expect(writes).To(BeEmpty())
		Expect(installed).To(BeEmpty())
	})

	Context("when prerequisites are off", func() {
		It("previews each planned change in dry-run without applying anything", func() {
			preflight.Elevated = true
			preflight.DryRun = true
			preflight.Selected = map[string]bool{winprereq.PowerShell: true}
			Expect(preflight.Run(context.Background())).To(Succeed())
			text := out.String()
			Expect(text).To(ContainSubstring("✗ Win32 long paths: off"))
			Expect(text).To(ContainSubstring("Planned changes (preview, nothing is applied)"))
			Expect(text).To(ContainSubstring(`Would enable Developer Mode: set HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock\AllowDevelopmentWithoutDevLicense to 1, only with --windows-prerequisites=developer-mode`))
			Expect(text).To(ContainSubstring("Would install PowerShell 7 with winget (Microsoft.PowerShell), selected with --windows-prerequisites"))
			Expect(text).To(ContainSubstring("Would install Windows Terminal with winget (Microsoft.WindowsTerminal)"))
			Expect(writes).To(BeEmpty())
			Expect(installed).To(BeEmpty())
		})

		It("applies only the changes selected in advance when not interactive", func() {
			preflight.Elevated = true
			preflight.Selected = map[string]bool{winprereq.LongPaths: true, winprereq.WindowsTerminal: true}
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(writes).To(Equal([]string{"LongPathsEnabled"}))
			Expect(installed).To(Equal([][]string{{"Windows Terminal"}}))
			Expect(out.String()).To(ContainSubstring("✓ Enabled Win32 long paths"))
			Expect(out.String()).To(ContainSubstring("Skipped Developer Mode: pass --windows-prerequisites=developer-mode"))
			Expect(out.String()).To(ContainSubstring("Skipped PowerShell 7: pass --windows-prerequisites=powershell"))
		})

		It("asks before each change when interactive", func() {
			preflight.Elevated = true
			preflight.Interactive = true
			preflight.In = strings.NewReader("y\nn\nyes\n\n")
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(writes).To(Equal([]string{"AllowDevelopmentWithoutDevLicense"}))
			Expect(installed).To(Equal([][]string{{"PowerShell"}}))
			Expect(out.String()).To(ContainSubstring("Enable Developer Mode"))
			Expect(out.String()).To(ContainSubstring("[y/N]"))
			Expect(out.String()).To(ContainSubstring(`Skipped. To do it later: reg add "HKLM\SYSTEM\CurrentControlSet\Control\FileSystem" /v LongPathsEnabled`))
		})

		It("reports a failed install with the manual winget step and keeps going", func() {
			preflight.Selected = map[string]bool{winprereq.PowerShell: true}
			preflight.InstallApps = func(context.Context, []string) error { return errors.New("winget unavailable") }
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Could not install PowerShell: winget unavailable"))
			Expect(out.String()).To(ContainSubstring("winget install --id Microsoft.PowerShell --exact --source winget"))
		})
	})

	Context("when DevEx is not elevated", func() {
		It("never writes the registry and prints the exact manual step, even when selected", func() {
			allOn()
			delete(values, "LongPathsEnabled")
			preflight.Selected = map[string]bool{winprereq.LongPaths: true}
			preflight.Interactive = true
			preflight.In = strings.NewReader("y\n")
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(writes).To(BeEmpty())
			Expect(out.String()).To(ContainSubstring("Win32 long paths (DevEx is not elevated"))
			Expect(out.String()).To(ContainSubstring(`reg add "HKLM\SYSTEM\CurrentControlSet\Control\FileSystem" /v LongPathsEnabled /t REG_DWORD /d 1 /f`))
			Expect(out.String()).NotTo(ContainSubstring("[y/N]"))
		})

		It("still offers tool installs", func() {
			allOn()
			onPath["pwsh"] = false
			preflight.Selected = map[string]bool{winprereq.PowerShell: true}
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(installed).To(Equal([][]string{{"PowerShell"}}))
		})
	})

	Context("when the registry cannot be read", func() {
		It("reports unknown and gives the manual step without writing", func() {
			allOn()
			denied["AllowDevelopmentWithoutDevLicense"] = true
			preflight.Elevated = true
			preflight.Selected = map[string]bool{winprereq.DeveloperMode: true}
			Expect(preflight.Run(context.Background())).To(Succeed())
			Expect(out.String()).To(ContainSubstring("? Developer Mode: unknown (could not read"))
			Expect(out.String()).To(ContainSubstring("DevEx could not read its state"))
			Expect(out.String()).To(ContainSubstring(`reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock" /v AllowDevelopmentWithoutDevLicense`))
			Expect(writes).To(BeEmpty())
		})
	})

	It("stops when setup is cancelled before installing tools", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		preflight.Selected = map[string]bool{winprereq.PowerShell: true}
		Expect(preflight.Run(ctx)).To(MatchError(context.Canceled))
		Expect(installed).To(BeEmpty())
	})
})
