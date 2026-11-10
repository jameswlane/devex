package state_test

import (
	"context"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Application run journal", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("DEVEX_STATE_DIR", GinkgoT().TempDir())
		previous := viper.Get(state.DBPathConfigKey)
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		DeferCleanup(func() { viper.Set(state.DBPathConfigKey, previous) })
	})
	It("records pending apps and cancellation independently of a canceled context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		run, err := state.StartApplicationRun(ctx, []types.CrossPlatformApp{{Name: "completed"}, {Name: "interrupted"}, {Name: "pending"}}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(run.CompleteApp(ctx, "completed", nil)).To(Succeed())
		cancel()
		Expect(run.CompleteApp(ctx, "interrupted", context.Canceled)).To(Succeed())
		Expect(run.Finish(ctx, context.Canceled)).To(Succeed())
		path, err := state.ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := state.OpenBoltStore(path, 0600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		saved, err := store.GetRun(context.Background(), run.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(saved.Status).To(Equal(state.RunStatusCanceled))
		ops, err := store.ListOperationsByRun(context.Background(), run.ID)
		Expect(err).NotTo(HaveOccurred())
		byName := map[string]state.OperationStatus{}
		for _, op := range ops {
			byName[op.StepID] = op.Status
		}
		Expect(byName).To(Equal(map[string]state.OperationStatus{"completed": state.OperationStatusSucceeded, "interrupted": state.OperationStatusCanceled, "pending": state.OperationStatusPending}))
	})
	It("protects a live plan from concurrent replay and releases its lock on finish", func() {
		apps := []types.CrossPlatformApp{{Name: "app"}}
		first, err := state.StartApplicationRun(context.Background(), apps, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = state.StartApplicationRun(context.Background(), apps, nil)
		Expect(err).To(MatchError(ContainSubstring("already has active")))
		Expect(first.Finish(context.Background(), context.Canceled)).To(Succeed())
		second, err := state.StartApplicationRun(state.WithResumeParent(context.Background(), first.ID), apps, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Finish(context.Background(), nil)).To(Succeed())
	})
})
