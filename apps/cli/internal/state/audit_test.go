package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestAudit(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Installation audit") }

var _ = Describe("Installation audit persistence", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("DEVEX_STATE_DIR", "")
		GinkgoT().Setenv("SUDO_USER", "")
		GinkgoT().Setenv("USER", "auditor")
		viper.Reset()
		DeferCleanup(viper.Reset)
		log.InitTestLogger()
		sdktest.NewRecorder(GinkgoT())
	})
	read := func() []AuditEvent {
		path, err := ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := OpenBoltStore(path, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		events, err := store.ListAudit(AuditFilter{})
		Expect(err).NotTo(HaveOccurred())
		return events
	}
	It("persists correlated start/result snapshots, metadata and skipped steps", func() {
		ctx, done := BeginApplication(WithAuditVersion(context.Background(), "1.2.3"), "editor", "plain", map[string]string{"app": "editor"})
		Expect(AuditStep(ctx, "install", true, func() error { return nil })).To(Succeed())
		done(nil)
		events := read()
		Expect(events).To(HaveLen(12))
		Expect(events[0].Actor).To(Equal("devex-test-user"))
		Expect(events[0].ToolVersion).To(Equal("1.2.3"))
		Expect(events[0].PlanID).To(HavePrefix("sha256:"))
		Expect(events[0].PolicyID).To(Equal("installation-v1"))
		Expect(events[0].Event).To(Equal("start"))
		Expect(events[0].Status).To(Equal(OperationStatusRunning))
		Expect(events[11].ID).To(Equal(events[0].ID))
		Expect(events[11].Status).To(Equal(OperationStatusSucceeded))
		Expect(events[11].CompletedAt).NotTo(BeNil())
		for i, event := range events {
			Expect(event.Sequence).To(Equal(uint64(i + 1)))
			Expect(event.RunID).To(Equal(events[0].RunID))
			Expect(event.SchemaVersion).To(Equal(1))
			Expect(event.Timestamp.IsZero()).To(BeFalse())
		}
		path, err := ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
	})
	It("redacts credentials in errors and metadata before writing any bytes", func() {
		GinkgoT().Setenv("DEPLOY_TOKEN", "environment-secret")
		ctx, done := BeginApplication(context.Background(), "password=target-secret", "plain", map[string]string{"password": "unlabeled-plan-value"})
		err := AuditStep(ctx, "install", true, func() error {
			return fmt.Errorf("password='two word secret' --token flag-secret https://alice:url-secret@example.org/a?token=query-secret Bearer bearer-secret environment-secret Authorization: Bearer header-secret unlabeled-plan-value")
		})
		done(err)
		data, marshalErr := json.Marshal(read())
		Expect(marshalErr).NotTo(HaveOccurred())
		for _, secret := range []string{"target-secret", "two word secret", "flag-secret", "url-secret", "query-secret", "bearer-secret", "environment-secret", "header-secret", "unlabeled-plan-value"} {
			Expect(string(data)).NotTo(ContainSubstring(secret))
		}
		Expect(string(data)).To(ContainSubstring("[REDACTED"))
	})
	It("records cancellation even though the caller context can no longer perform I/O", func() {
		ctx, cancel := context.WithCancel(context.Background())
		ctx, done := BeginApplication(ctx, "editor", "tui", "plan")
		cancel()
		err := AuditStep(ctx, "install", true, func() error { Fail("must not execute after cancellation"); return nil })
		Expect(err).To(MatchError(context.Canceled))
		done(err)
		events := read()
		Expect(events[2].Status).To(Equal(OperationStatusCanceled))
		Expect(events[len(events)-1].Status).To(Equal(OperationStatusCanceled))
	})
	It("retains the newest configurable number of events across reopen and supports filters", func() {
		viper.Set("audit.max_entries", 3)
		_, done := BeginApplication(context.Background(), "editor", "plain", "plan")
		done(nil)
		events := read()
		Expect(events).To(HaveLen(3))
		Expect(events[0].Sequence).To(Equal(uint64(10)))
		path, err := ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := OpenBoltStore(path, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		filtered, err := store.ListAudit(AuditFilter{RunID: events[0].RunID, Target: "editor", Mode: "plain", Event: "result", Status: OperationStatusSucceeded})
		Expect(err).NotTo(HaveOccurred())
		Expect(filtered).To(HaveLen(1))
		empty, err := store.ListAudit(AuditFilter{Target: "unknown"})
		Expect(err).NotTo(HaveOccurred())
		Expect(empty).To(BeEmpty())
		Expect(empty).NotTo(BeNil())
	})
	It("logs a write failure and preserves the operation's success", func() {
		var output bytes.Buffer
		log.InitDefaultLogger(&output)
		DeferCleanup(log.InitTestLogger)
		file := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(file, nil, 0o600)).To(Succeed())
		viper.Set(DBPathConfigKey, filepath.Join(file, "state.db"))
		ctx, done := BeginApplication(context.Background(), "editor", "plain", "plan")
		Expect(AuditStep(ctx, "install", true, func() error { return nil })).To(Succeed())
		done(nil)
		Expect(output.String()).To(ContainSubstring("Installation audit write failed"))
	})
})
