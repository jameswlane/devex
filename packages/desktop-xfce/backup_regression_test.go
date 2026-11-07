package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("XFCE backup and restore regressions", func() {
	var recorder *sdktest.Recorder
	var dir string
	var plugin *XFCEPlugin
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), xfceFakes...)
		dir = GinkgoT().TempDir()
		GinkgoT().Setenv("XDG_CONFIG_HOME", dir)
		GinkgoT().Setenv("XDG_CONFIG_DIRS", dir)
		plugin = NewXFCEPlugin()
	})
	native := func(xml string) {
		path := filepath.Join(dir, "xfce4", "xfconf", "xfce-perchannel-xml", "xfwm4.xml")
		Expect(os.MkdirAll(filepath.Dir(path), 0700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(xml), 0600)).To(Succeed())
	}
	It("recovers native types and complete array values when verbose and plain match", func() {
		native(`<channel name="xfwm4" version="1.0"><property name="general" type="empty"><property name="names" type="array"><value type="string" value="[int] 5"/><value type="int" value="7"/><value type="string" value="line&#10;two &amp; three"/></property><property name="title" type="string" value="[int] 5"/></property></channel>`)
		for _, prop := range []string{"/general/names", "/general/title"} {
			output := "Value is an array with 3 items:\n\n[int] 5\n7\nline\ntwo & three\n"
			if prop == "/general/title" {
				output = "[int] 5\n"
			}
			recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop}, sdktest.Response{Stdout: output})
			recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop, "-v"}, sdktest.Response{Stdout: output})
		}
		result, err := readXFCEProperty(context.Background(), "xfwm4", "/general/names")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Type).To(Equal("array"))
		Expect(result.Values).To(Equal([]xfceValue{{Type: "string", Value: "[int] 5"}, {Type: "int", Value: "7"}, {Type: "string", Value: "line\ntwo & three"}}))
		result, err = readXFCEProperty(context.Background(), "xfwm4", "/general/title")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Type).To(Equal("string"))
		Expect(*result.Value).To(Equal("[int] 5"))
	})
	DescribeTable("rejects stale or unverifiable native values instead of backing them up",
		func(xml, live string) {
			native(xml)
			for _, args := range [][]string{{"-c", "xfwm4", "-p", "/names"}, {"-c", "xfwm4", "-p", "/names", "-v"}} {
				recorder.Script("xfconf-query", args, sdktest.Response{Stdout: live})
			}
			_, err := readXFCEProperty(context.Background(), "xfwm4", "/names")
			Expect(err).To(MatchError("native xfconf XML does not match the current value; allow xfconfd to flush settings and retry backup"))
		},
		Entry("stale scalar", `<channel name="xfwm4"><property name="names" type="string" value="old"/></channel>`, "current\n"),
		Entry("stale scalar with typed-looking live text", `<channel name="xfwm4"><property name="names" type="int" value="5"/></channel>`, "[int] 5\n"),
		Entry("stale array body", `<channel name="xfwm4"><property name="names" type="array"><value type="string" value="old"/></property></channel>`, "Wert ist ein Array mit 1 Element:\n\ncurrent\n"),
		Entry("stale count with matching body", `<channel name="xfwm4"><property name="names" type="array"><value type="string" value="one&#10;two"/></property></channel>`, "Value is an array with 2 items:\n\none\ntwo\n"),
		Entry("array became scalar", `<channel name="xfwm4"><property name="names" type="array"/></channel>`, "current\n"),
		Entry("missing array count", `<channel name="xfwm4"><property name="names" type="array"/></channel>`, "Array:\n\n"),
		Entry("scalar became array", `<channel name="xfwm4"><property name="names" type="string" value="old"/></channel>`, "Value is an array with 0 items:\n\n"),
	)
	DescribeTable("preserves matching localized arrays including empty and multiline items",
		func(xml, live string, expected []xfceValue) {
			native(xml)
			for _, args := range [][]string{{"-c", "xfwm4", "-p", "/names"}, {"-c", "xfwm4", "-p", "/names", "-v"}} {
				recorder.Script("xfconf-query", args, sdktest.Response{Stdout: live})
			}
			result, err := readXFCEProperty(context.Background(), "xfwm4", "/names")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Type).To(Equal("array"))
			Expect(result.Values).To(Equal(expected))
		},
		Entry("localized multiline and empty items", `<channel name="xfwm4"><property name="names" type="array"><value type="string" value="line&#10;two&#10;"/><value type="string" value=""/><value type="int" value="7"/></property></channel>`, "Wert ist ein Array mit 3 Elementen:\n\nline\ntwo\n\n\n7\n", []xfceValue{{Type: "string", Value: "line\ntwo\n"}, {Type: "string", Value: ""}, {Type: "int", Value: "7"}}),
		Entry("localized empty", `<channel name="xfwm4"><property name="names" type="array"/></channel>`, "Wert ist ein Array mit 0 Elementen:\n\n", []xfceValue(nil)),
	)
	DescribeTable("retains literal typed-looking strings when native XML is unavailable",
		func(xml string) {
			native(xml)
			for _, args := range [][]string{{"-c", "xfwm4", "-p", "/names"}, {"-c", "xfwm4", "-p", "/names", "-v"}} {
				recorder.Script("xfconf-query", args, sdktest.Response{Stdout: "[int] 5\n"})
			}
			result, err := readXFCEProperty(context.Background(), "xfwm4", "/names")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Type).To(BeEmpty())
			Expect(*result.Value).To(Equal("[int] 5"))
		},
		Entry("malformed XML", "<channel"),
		Entry("missing property", `<channel name="xfwm4"/>`),
	)
	It("warns about stale XML and saves only consistent current properties", func() {
		native(`<channel name="xfwm4"><property name="names" type="array"><value type="string" value="old"/></property><property name="title" type="string" value="current"/></channel>`)
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/names\n/title\n"})
		for _, prop := range []string{"/names", "/title"} {
			live := "current\n"
			if prop == "/names" {
				live = "Value is an array with 1 items:\n\ncurrent\n"
			}
			recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop}, sdktest.Response{Stdout: live})
			recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", prop, "-v"}, sdktest.Response{Stdout: live})
		}
		output := sdktest.CaptureStdout(GinkgoT(), func() { Expect(plugin.handleBackup([]string{dir})).To(Succeed()) })
		Expect(output).To(ContainSubstring("Warning: Skipping xfwm4/names: native xfconf XML does not match the current value"))
		content := readBackup(dir)
		Expect(content).NotTo(ContainSubstring("old"))
		Expect(content).NotTo(ContainSubstring(`name="/names"`))
		Expect(content).To(ContainSubstring(`name="/title" type="string" value="current"`))
	})

	DescribeTable("retains compatible typed read decoding",
		func(plain, verbose, kind string, count int) {
			result, err := decodeXFCEProperty("/names", plain, verbose)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Type).To(Equal(kind))
			Expect(result.Values).To(HaveLen(count))
		},
		Entry("scalar", "hello\n", "[string] hello\n", "string", 0),
		Entry("two-item array", "Value is an array with 2 items:\n\nMain\nWork\n", "Value is an array with 2 items:\n\n[string] Main\n[string] Work\n", "array", 2),
	)
	DescribeTable("reports unusable native XML",
		func(content string) {
			native(content)
			_, err := readNativeXFCEProperty("xfwm4", "/names")
			Expect(err).To(HaveOccurred())
		},
		Entry("malformed", "<channel"),
		Entry("mismatched channel", `<channel name="other"><property name="names" type="array"/></channel>`),
		Entry("unsupported item", `<channel name="xfwm4"><property name="names" type="array"><value type="unknown" value="x"/></property></channel>`),
		Entry("array with scalar", `<channel name="xfwm4"><property name="names" type="array" value=""/></channel>`),
	)
	It("uses system defaults for missing properties and user values for overrides", func() {
		native(`<channel name="xfwm4"><property name="names" type="array"><value type="string" value="User"/></property></channel>`)
		system := GinkgoT().TempDir()
		GinkgoT().Setenv("XDG_CONFIG_DIRS", system)
		path := filepath.Join(system, "xfce4", "xfconf", "xfce-perchannel-xml", "xfwm4.xml")
		Expect(os.MkdirAll(filepath.Dir(path), 0700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(`<channel name="xfwm4"><property name="names" type="array"><value type="string" value="System"/></property><property name="other" type="array"/></channel>`), 0600)).To(Succeed())
		result, err := readNativeXFCEProperty("xfwm4", "/names")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Values).To(Equal([]xfceValue{{Type: "string", Value: "User"}}))
		result, err = readNativeXFCEProperty("xfwm4", "/other")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Type).To(Equal("array"))
		Expect(result.Values).To(BeEmpty())
	})

	It("round-trips a zero-element native array through backup and D-Bus restore", func() {
		native(`<channel name="xfwm4" version="1.0"><property name="names" type="array"/></channel>`)
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
		recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: "/names\n"})
		for _, args := range [][]string{{"-c", "xfwm4", "-p", "/names"}, {"-c", "xfwm4", "-p", "/names", "-v"}} {
			recorder.Script("xfconf-query", args, sdktest.Response{Stdout: "Value is an array with 0 items:\n\n"})
		}
		Expect(plugin.handleBackup([]string{dir})).To(Succeed())
		backup, err := parseXFCEBackup([]byte(readBackup(dir)))
		Expect(err).NotTo(HaveOccurred())
		Expect(backup.Channels[0].Properties[0].Type).To(Equal("array"))
		Expect(backup.Channels[0].Properties[0].Values).To(BeEmpty())
		files, err := filepath.Glob(filepath.Join(dir, "xfce-settings-*.xml"))
		Expect(err).NotTo(HaveOccurred())
		recorder.Reset()
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		Expect(plugin.handleRestore(files)).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(BeEmpty())
		Expect(recorder.Calls("gdbus")).To(Equal([]sdktest.Call{{Args: []string{"call", "--session", "--dest", "org.xfce.Xfconf", "--object-path", "/org/xfce/Xfconf", "--method", "org.xfce.Xfconf.SetProperty", "xfwm4", "/names", "<@av []>"}}}))
	})
	It("reports an empty-array native write failure without pretending to restore it", func() {
		recorder.Fallback("gdbus", sdktest.Response{ExitCode: 42, Stderr: "session unavailable\n"})
		err := restoreXFCEProperty(context.Background(), "xfwm4", xfceProperty{Name: "/names", Type: "array"})
		Expect(err).To(MatchError("exit status 42: session unavailable"))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(recorder.Calls("xfconf-query")).To(BeEmpty())
	})

	It("forces one-item arrays with -a", func() {
		Expect(restoreXFCEProperty(context.Background(), "xfwm4", xfceProperty{Name: "/names", Type: "array", Values: []xfceValue{{Type: "string", Value: "Main"}}})).To(Succeed())
		Expect(recorder.Calls("xfconf-query")).To(Equal([]sdktest.Call{{Args: []string{"-c", "xfwm4", "-p", "/names", "-n", "-a", "-t", "string", "-s", "Main"}}}))
	})
	DescribeTable("preserves command errors and stderr",
		func(stderr, expected string) {
			recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{ExitCode: 42, Stderr: stderr})
			_, err := xfconfQuery(context.Background(), "-l")
			Expect(err).To(MatchError(expected))
			var exit *exec.ExitError
			Expect(errors.As(err, &exit)).To(BeTrue())
			Expect(exit.ExitCode()).To(Equal(42))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
		},
		Entry("nonempty stderr", "connection refused\n", "exit status 42: connection refused"),
		Entry("no stderr", "", "exit status 42"),
	)
	It("leaves successful stdout unchanged despite stderr", func() {
		recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n", Stderr: "notice\n"})
		output, err := xfconfQuery(context.Background(), "-l")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(output)).To(Equal("Channels:\n"))
	})
	DescribeTable("rejects zero captured properties without replacing an existing backup",
		func(listing string, fail bool) {
			recorder.Script("xfconf-query", []string{"-l"}, sdktest.Response{Stdout: "Channels:\n  xfwm4\n"})
			recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-l"}, sdktest.Response{Stdout: listing})
			if fail {
				recorder.Fallback("xfconf-query", sdktest.Response{ExitCode: 42})
			} else {
				recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/theme"}, sdktest.Response{Stdout: "first\nsecond\n"})
				recorder.Script("xfconf-query", []string{"-c", "xfwm4", "-p", "/theme", "-v"}, sdktest.Response{Stdout: "first\nsecond\n"})
			}
			path := filepath.Join(dir, "xfce-settings-"+time.Now().Format("2006-01-02")+".xml")
			Expect(os.WriteFile(path, []byte("keep this"), 0600)).To(Succeed())
			Expect(plugin.handleBackup([]string{dir})).To(MatchError(ContainSubstring("no properties could be captured")))
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("keep this"))
		},
		Entry("empty readable channel", "Channel contains no properties.\n", false),
		Entry("every property fails", "/theme\n", true),
		Entry("every property undecodable", "/theme\n", false),
	)
	It("fails a blocked property query promptly even after capturing another property", func() {
		bin := GinkgoT().TempDir()
		// A real blocked subprocess exercises CommandContext rather than a mock error.
		script := "#!/bin/sh\ncase \"$*\" in\n'-l') printf 'Channels:\\n  xfwm4\\n';;\n'-c xfwm4 -l') printf '/good\\n/blocked\\n';;\n*'/good -v') printf '[string] good\\n';;\n*'/good') printf 'good\\n';;\n*) exec /bin/sleep 10;;\nesac\n"
		Expect(os.WriteFile(filepath.Join(bin, "xfconf-query"), []byte(script), 0700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		start := time.Now()
		err := plugin.handleBackupContext(ctx, []string{dir})
		Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
		Expect(time.Since(start)).To(BeNumerically("<", 2*time.Second))
		Expect(filepath.Glob(filepath.Join(dir, "xfce-settings-*.xml"))).To(BeEmpty())
	})
	It("stops further writes when a running restore command times out", func() {
		bin := GinkgoT().TempDir()
		calls := filepath.Join(dir, "calls")
		GinkgoT().Setenv("XFCE_TEST_CALLS", calls)
		Expect(os.WriteFile(filepath.Join(bin, "xfconf-query"), []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$XFCE_TEST_CALLS\"\nexec /bin/sleep 10\n"), 0700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		path := filepath.Join(dir, "restore.xml")
		Expect(os.WriteFile(path, []byte(backupFixtureXML), 0600)).To(Succeed())
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		err := plugin.handleRestoreContext(ctx, []string{path})
		Expect(errors.Is(err, context.DeadlineExceeded)).To(BeTrue())
		data, err := os.ReadFile(calls)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("-c xfwm4 -p /general/click_to_focus -n -t bool -s true\n"))
	})
	It("fails an expired backup before any native call", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := plugin.handleBackupContext(ctx, []string{dir})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(recorder.Calls("xfconf-query")).To(BeEmpty())
		Expect(filepath.Glob(filepath.Join(dir, "xfce-settings-*.xml"))).To(BeEmpty())
	})
	It("does not continue restoring after cancellation", func() {
		path := filepath.Join(dir, "restore.xml")
		Expect(os.WriteFile(path, []byte(backupFixtureXML), 0600)).To(Succeed())
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(errors.Is(plugin.handleRestoreContext(ctx, []string{path}), context.Canceled)).To(BeTrue())
		Expect(recorder.Calls("xfconf-query")).To(BeEmpty())
	})
})
