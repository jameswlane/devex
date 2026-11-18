//go:build unix

package templates_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Answers from a stream", func() {
	It("reads an answers file once, so stdin answers still select the steps", func() {
		fifo := filepath.Join(GinkgoT().TempDir(), "answers")
		Expect(syscall.Mkfifo(fifo, 0o600)).To(Succeed())
		done := make(chan struct{})
		DeferCleanup(func() {
			close(done)
			// Unblock a writer waiting for a reader.
			if reader, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
				_ = reader.Close()
			}
		})
		go func() {
			defer GinkgoRecover()
			// The first reader gets the answers; any later reader gets an
			// empty stream, as a second read of stdin would.
			payload := "databases: [postgres, redis]\n"
			for {
				writer, err := os.OpenFile(fifo, os.O_WRONLY, 0)
				if err != nil {
					return
				}
				_, _ = writer.WriteString(payload)
				_ = writer.Close()
				payload = ""
				select {
				case <-done:
					return
				default:
				}
			}
		}()

		planner := templates.TemplatePlanner{
			Manager:     templates.NewTemplateManager(GinkgoT().TempDir()),
			AppResolver: testAppResolver{},
			System:      map[string]string{"os": "linux"},
		}
		plan, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action:       templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{AnswerFiles: []string{fifo}, NonInteractive: true},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(stepIDs(plan)).To(Equal([]string{"docker-engine", "postgres", "redis"}))
	})
})
