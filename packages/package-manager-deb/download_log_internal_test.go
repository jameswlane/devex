package main

import (
	"os"
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

var _ = Describe("DEB download debug logging", func() {
	It("names the temporary file when it is cleaned up", func() {
		rec := &debugRecorder{}
		d := &DebInstaller{logger: rec}

		f := filepath.Join(GinkgoT().TempDir(), "pkg.deb")
		Expect(os.WriteFile(f, []byte("x"), 0o600)).To(Succeed())

		d.cleanupTempFile(f)

		Expect(rec.debug).To(ConsistOf("Cleaned up temporary file: " + f))
	})

})
