package winprereq_test

import (
	"errors"
	"fmt"
	"io/fs"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/winprereq"
)

// fakeRegistry is an in-memory HKLM; a value in denied fails to read.
type fakeRegistry struct {
	values map[string]uint32
	denied map[string]bool
	writes []string
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{values: map[string]uint32{}, denied: map[string]bool{}}
}

func (r *fakeRegistry) probe(onPath ...string) winprereq.Probe {
	found := map[string]bool{}
	for _, name := range onPath {
		found[name] = true
	}
	return winprereq.Probe{
		ReadDWORD: func(key, value string) (uint32, error) {
			if r.denied[value] {
				return 0, errors.New("Access is denied.")
			}
			data, ok := r.values[value]
			if !ok {
				return 0, fmt.Errorf("value missing: %w", fs.ErrNotExist)
			}
			return data, nil
		},
		WriteDWORD: func(key, value string, data uint32) error {
			r.writes = append(r.writes, key+`\`+value)
			r.values[value] = data
			return nil
		},
		LookPath: func(file string) (string, error) {
			if found[file] {
				return `C:\bin\` + file + ".exe", nil
			}
			return "", errors.New("not found")
		},
	}
}

func byID(results []winprereq.Result) map[string]winprereq.Result {
	m := map[string]winprereq.Result{}
	for _, r := range results {
		m[r.ID] = r
	}
	return m
}

var _ = Describe("Windows prerequisite checks", func() {
	It("reports every prerequisite on when the registry and PATH have them", func() {
		reg := newFakeRegistry()
		reg.values["AllowDevelopmentWithoutDevLicense"] = 1
		reg.values["LongPathsEnabled"] = 1
		results := winprereq.Check(reg.probe("pwsh", "wt"))
		Expect(results).To(HaveLen(4))
		for _, r := range results {
			Expect(r.State).To(Equal(winprereq.StateOn), r.Name)
		}
		Expect(byID(results)[winprereq.PowerShell].Detail).To(Equal(`C:\bin\pwsh.exe`))
	})

	It("reports off for a zero value, a missing value and a missing tool", func() {
		reg := newFakeRegistry()
		reg.values["AllowDevelopmentWithoutDevLicense"] = 0
		results := byID(winprereq.Check(reg.probe()))
		Expect(results[winprereq.DeveloperMode].State).To(Equal(winprereq.StateOff))
		Expect(results[winprereq.LongPaths].State).To(Equal(winprereq.StateOff))
		Expect(results[winprereq.PowerShell].State).To(Equal(winprereq.StateOff))
		Expect(results[winprereq.WindowsTerminal].State).To(Equal(winprereq.StateOff))
	})

	It("reports unknown with the reason when the registry cannot be read", func() {
		reg := newFakeRegistry()
		reg.denied["LongPathsEnabled"] = true
		result := byID(winprereq.Check(reg.probe()))[winprereq.LongPaths]
		Expect(result.State).To(Equal(winprereq.StateUnknown))
		Expect(result.Detail).To(ContainSubstring(`HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled`))
		Expect(result.Detail).To(ContainSubstring("Access is denied"))
	})

	It("reports registry checks unknown when no registry is available", func() {
		results := byID(winprereq.Check(winprereq.Probe{}))
		Expect(results[winprereq.DeveloperMode].State).To(Equal(winprereq.StateUnknown))
		Expect(results[winprereq.PowerShell].State).To(Equal(winprereq.StateUnknown))
	})

	It("enables a registry setting and verifies it by reading it back", func() {
		reg := newFakeRegistry()
		p := winprereq.Prerequisites[0]
		Expect(winprereq.Enable(reg.probe(), p)).To(Succeed())
		Expect(reg.writes).To(ConsistOf(`SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock\AllowDevelopmentWithoutDevLicense`))
	})

	It("fails when a written value does not read back as on", func() {
		reg := newFakeRegistry()
		probe := reg.probe()
		probe.WriteDWORD = func(string, string, uint32) error { return nil }
		Expect(winprereq.Enable(probe, winprereq.Prerequisites[1])).To(MatchError(ContainSubstring("still off")))
	})

	It("gives exact manual steps", func() {
		Expect(winprereq.Prerequisites[1].ManualStep()).To(Equal(`reg add "HKLM\SYSTEM\CurrentControlSet\Control\FileSystem" /v LongPathsEnabled /t REG_DWORD /d 1 /f`))
		Expect(winprereq.Prerequisites[2].ManualStep()).To(Equal("winget install --id Microsoft.PowerShell --exact --source winget"))
	})

	Describe("ParseSelection", func() {
		It("accepts known IDs and all", func() {
			selected, err := winprereq.ParseSelection([]string{" Long-Paths ", ""})
			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(Equal(map[string]bool{winprereq.LongPaths: true}))
			selected, err = winprereq.ParseSelection([]string{"all"})
			Expect(err).NotTo(HaveOccurred())
			Expect(selected).To(HaveLen(4))
		})

		It("rejects an unknown ID and names the valid ones", func() {
			_, err := winprereq.ParseSelection([]string{"wsl"})
			Expect(err).To(MatchError(ContainSubstring("developer-mode, long-paths, powershell, windows-terminal")))
		})
	})
})
