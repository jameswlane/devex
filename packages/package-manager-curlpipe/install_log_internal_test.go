package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
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

var _ = Describe("curlpipe install debug logging", func() {
	It("names the app it tracks after running the script", func() {
		sdktest.NewRecorder(GinkgoT(), "bash", "curl")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")

		script := []byte("echo fixture\n")
		cached := filepath.Join(GinkgoT().TempDir(), "cached.sh")
		Expect(os.WriteFile(cached, script, 0o600)).To(Succeed())

		rec := &debugRecorder{}
		p := NewCurlpipePlugin()
		p.logger = rec

		Expect(p.Execute("install", []string{
			"--artifact-path", cached,
			"--sha256", fmt.Sprintf("%x", sha256.Sum256(script)),
			"https://get.docker.com",
		})).To(Succeed())

		Expect(rec.debug).To(ContainElement("Tracking installation: " + p.extractAppNameFromURL("https://get.docker.com")))
		Expect(rec.debug).NotTo(ContainElement(ContainSubstring("%s")))
	})
})
