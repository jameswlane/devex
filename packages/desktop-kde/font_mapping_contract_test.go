package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func captureFontStderr(fn func()) string {
	f, err := os.CreateTemp(GinkgoT().TempDir(), "stderr")
	Expect(err).NotTo(HaveOccurred())
	defer func() { Expect(f.Close()).To(Succeed()) }()
	old := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = old }()
	fn()
	_, err = f.Seek(0, 0)
	Expect(err).NotTo(HaveOccurred())
	data, err := io.ReadAll(f)
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

var _ = Describe("Distribution font install contracts", func() {
	It("pins default argv and exact preview for apt-get", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "fc-cache", "kbuildsycoca5")
		fm := NewFontManager()
		var preview string
		warning := captureFontStderr(func() { preview = sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe(nil)).To(Succeed()) }) })
		Expect(warning).To(ContainSubstring("sourcecodepro"))
		var plan [][]string
		Expect(json.Unmarshal([]byte(preview), &plan)).To(Succeed())
		Expect(plan).To(Equal([][]string{append([]string{"apt-get"}, []string{"install", "-y", "fonts-firacode", "fonts-jetbrains-mono", "fonts-hack", "fonts-inconsolata", "fonts-cascadia-code", "fonts-noto", "fonts-liberation"}...), {"fc-cache", "-f", "-v"}, {"kbuildsycoca5", "--noincremental"}}))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fonts-firacode", "fonts-jetbrains-mono", "fonts-hack", "fonts-inconsolata", "fonts-cascadia-code", "fonts-noto", "fonts-liberation"}, ElevationRequested: true}}))
		Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))
	})
	It("pins mapped aliases and explicit native packages for apt-get", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"Fira Code", "jetbrains", "HACK", "Inconsolata", "Cascadia Code", "Liberation", "Noto", "Roboto", "custom-font"})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fonts-firacode", "fonts-jetbrains-mono", "fonts-hack", "fonts-inconsolata", "fonts-cascadia-code", "fonts-liberation", "fonts-noto", "fonts-roboto", "custom-font"}, ElevationRequested: true}}))
	})
	It("rejects explicit unavailable fonts before any apt-get calls", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "fc-cache")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"hack", "sourcecodepro"})).To(MatchError(ContainSubstring("no verified package for apt-get: sourcecodepro")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("fc-cache")).To(BeEmpty())
	})
	It("pins default argv and exact preview for dnf", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf", "fc-cache", "kbuildsycoca5")
		fm := NewFontManager()
		var preview string
		warning := captureFontStderr(func() { preview = sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe(nil)).To(Succeed()) }) })
		Expect(warning).To(BeEmpty())
		var plan [][]string
		Expect(json.Unmarshal([]byte(preview), &plan)).To(Succeed())
		Expect(plan).To(Equal([][]string{append([]string{"dnf"}, []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "source-foundry-hack-fonts", "adobe-source-code-pro-fonts", "levien-inconsolata-fonts", "cascadia-code-fonts", "google-noto-sans-fonts", "liberation-fonts-all", "oxygen-fonts"}...), {"fc-cache", "-f", "-v"}, {"kbuildsycoca5", "--noincremental"}}))
		Expect(recorder.Calls("dnf")).To(BeEmpty())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash})).To(Succeed())
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "source-foundry-hack-fonts", "adobe-source-code-pro-fonts", "levien-inconsolata-fonts", "cascadia-code-fonts", "google-noto-sans-fonts", "liberation-fonts-all", "oxygen-fonts"}, ElevationRequested: true}}))
		Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))
	})
	It("pins mapped aliases and explicit native packages for dnf", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"Fira Code", "jetbrains", "HACK", "Inconsolata", "Cascadia Code", "Liberation", "Noto", "Roboto", "custom-font"})).To(Succeed())
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "source-foundry-hack-fonts", "levien-inconsolata-fonts", "cascadia-code-fonts", "liberation-fonts-all", "google-noto-sans-fonts", "google-roboto-fonts", "custom-font"}, ElevationRequested: true}}))
	})
	It("pins default argv and exact preview for pacman", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "pacman", "fc-cache", "kbuildsycoca5")
		fm := NewFontManager()
		var preview string
		warning := captureFontStderr(func() { preview = sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe(nil)).To(Succeed()) }) })
		Expect(warning).To(ContainSubstring("oxygen"))
		var plan [][]string
		Expect(json.Unmarshal([]byte(preview), &plan)).To(Succeed())
		Expect(plan).To(Equal([][]string{append([]string{"pacman"}, []string{"-S", "--noconfirm", "ttf-fira-code", "ttf-jetbrains-mono", "ttf-hack", "adobe-source-code-pro-fonts", "ttf-inconsolata", "ttf-cascadia-code", "noto-fonts", "ttf-liberation"}...), {"fc-cache", "-f", "-v"}, {"kbuildsycoca5", "--noincremental"}}))
		Expect(recorder.Calls("pacman")).To(BeEmpty())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash})).To(Succeed())
		Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{Args: []string{"-S", "--noconfirm", "ttf-fira-code", "ttf-jetbrains-mono", "ttf-hack", "adobe-source-code-pro-fonts", "ttf-inconsolata", "ttf-cascadia-code", "noto-fonts", "ttf-liberation"}, ElevationRequested: true}}))
		Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))
	})
	It("pins mapped aliases and explicit native packages for pacman", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "pacman")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"Fira Code", "jetbrains", "HACK", "Inconsolata", "Cascadia Code", "Liberation", "Noto", "Roboto", "custom-font"})).To(Succeed())
		Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{Args: []string{"-S", "--noconfirm", "ttf-fira-code", "ttf-jetbrains-mono", "ttf-hack", "ttf-inconsolata", "ttf-cascadia-code", "ttf-liberation", "noto-fonts", "ttf-roboto", "custom-font"}, ElevationRequested: true}}))
	})
	It("rejects explicit unavailable fonts before any pacman calls", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "pacman", "fc-cache")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"hack", "oxygen"})).To(MatchError(ContainSubstring("no verified package for pacman: oxygen")))
		Expect(recorder.Calls("pacman")).To(BeEmpty())
		Expect(recorder.Calls("fc-cache")).To(BeEmpty())
	})
	It("pins default argv and exact preview for zypper", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "zypper", "fc-cache", "kbuildsycoca5")
		fm := NewFontManager()
		var preview string
		warning := captureFontStderr(func() { preview = sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe(nil)).To(Succeed()) }) })
		Expect(warning).To(BeEmpty())
		var plan [][]string
		Expect(json.Unmarshal([]byte(preview), &plan)).To(Succeed())
		Expect(plan).To(Equal([][]string{append([]string{"zypper"}, []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "hack-fonts", "adobe-sourcecodepro-fonts", "google-inconsolata-fonts", "saja-cascadia-code-fonts", "google-noto-sans-fonts", "liberation-fonts", "kde-oxygen-fonts"}...), {"fc-cache", "-f", "-v"}, {"kbuildsycoca5", "--noincremental"}}))
		Expect(recorder.Calls("zypper")).To(BeEmpty())
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash})).To(Succeed())
		Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "hack-fonts", "adobe-sourcecodepro-fonts", "google-inconsolata-fonts", "saja-cascadia-code-fonts", "google-noto-sans-fonts", "liberation-fonts", "kde-oxygen-fonts"}, ElevationRequested: true}}))
		Expect(recorder.Calls("fc-cache")).To(Equal([]sdktest.Call{{Args: []string{"-f", "-v"}}}))
	})
	It("pins mapped aliases and explicit native packages for zypper", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "zypper")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"Fira Code", "jetbrains", "HACK", "Inconsolata", "Cascadia Code", "Liberation", "Noto", "Roboto", "custom-font"})).To(Succeed())
		Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fira-code-fonts", "jetbrains-mono-fonts", "hack-fonts", "google-inconsolata-fonts", "saja-cascadia-code-fonts", "liberation-fonts", "google-noto-sans-fonts", "google-roboto-fonts", "custom-font"}, ElevationRequested: true}}))
	})
	It("rejects stale accepted plans before invoking native tools", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "fc-cache")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"--expected-plan", "stale", "hack"})).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("fc-cache")).To(BeEmpty())
	})
	It("rejects a manager change after preview", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dnf")
		fm := NewFontManager()
		preview := sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe([]string{"hack"})).To(Succeed()) })
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		path, err := exec.LookPath("apt-get")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash, "hack"})).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("dnf")).To(BeEmpty())
	})
	It("deduplicates aliases and historical operands in request order", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"Hack", "fonts-hack", "Source Code Pro", "jetbrainsmono", "JetBrains Mono"})).To(Succeed())
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "source-foundry-hack-fonts", "adobe-source-code-pro-fonts", "jetbrains-mono-fonts"}, ElevationRequested: true}}))
	})
	DescribeTable("reports unavailable aliases without executing any commands", func(manager, alias string) {
		recorder := sdktest.NewRecorder(GinkgoT(), manager, "fc-cache")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"hack", alias})).To(MatchError(ContainSubstring("no verified package for " + manager + ": " + alias)))
		Expect(recorder.Calls(manager)).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
		Expect(recorder.Calls("fc-cache")).To(BeEmpty())
	}, Entry("APT Source Code Pro", "apt-get", "fonts-source-code-pro"),
		Entry("APT Oxygen", "apt-get", "Oxygen"),
		Entry("Fedora Ubuntu", "dnf", "Ubuntu"),
		Entry("Arch Oxygen", "pacman", "Oxygen"))
	It("rejects a cache-tool change after preview", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "fc-cache")
		fm := NewFontManager()
		preview := sdktest.CaptureStdout(GinkgoT(), func() { Expect(fm.Describe([]string{"hack"})).To(Succeed()) })
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(preview[:len(preview)-1])))
		path, err := exec.LookPath("fc-cache")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(path)).To(Succeed())
		Expect(fm.InstallFonts(context.Background(), []string{"--expected-plan", hash, "hack"})).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
	It("routes font previews without a desktop session", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf")
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		preview := sdktest.CaptureStdout(GinkgoT(), func() { Expect(NewKDEPlugin().Execute("describe", []string{"install-fonts", "hack"})).To(Succeed()) })
		var plan [][]string
		Expect(json.Unmarshal([]byte(preview), &plan)).To(Succeed())
		Expect(plan).To(Equal([][]string{{"dnf", "install", "-y", "source-foundry-hack-fonts"}}))
		Expect(recorder.Calls("dnf")).To(BeEmpty())
	})

})
