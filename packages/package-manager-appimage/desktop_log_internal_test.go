package main

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// debugRecorder captures Debug messages; other levels are discarded.
type debugRecorder struct {
	debug []string
}

func (r *debugRecorder) Printf(string, ...any)       {}
func (r *debugRecorder) Println(string, ...any)      {}
func (r *debugRecorder) Success(string, ...any)      {}
func (r *debugRecorder) Warning(string, ...any)      {}
func (r *debugRecorder) ErrorMsg(string, ...any)     {}
func (r *debugRecorder) Info(string, ...any)         {}
func (r *debugRecorder) Warn(string, ...any)         {}
func (r *debugRecorder) Error(string, error, ...any) {}
func (r *debugRecorder) Debug(msg string, keyvals ...any) {
	// Mirror sdk.DefaultLogger: the message is printed as-is.
	r.debug = append(r.debug, msg)
}

var _ = Describe("AppImage desktop entry debug logging", func() {
	It("names the desktop file it creates and removes", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		rec := &debugRecorder{}
		p := NewAppimagePlugin()
		p.logger = rec

		Expect(p.createDesktopEntry("demo", "/opt/demo.AppImage")).To(Succeed())
		Expect(p.RemoveDesktopEntry("demo")).To(Succeed())

		file := filepath.Join(home, ".local", "share", "applications", "demo.desktop")
		Expect(rec.debug).To(Equal([]string{
			"Created desktop entry: " + file,
			"Removed desktop entry: " + file,
		}))
	})
})
