package agentapi_test

import (
	"bytes"
	"errors"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type failingListRepo struct{ types.Repository }

func (failingListRepo) ListApps() ([]types.AppConfig, error) {
	return nil, errors.New("database is locked")
}

var _ = Describe("GetInstalledAppsCache", func() {
	It("logs the database error when installed apps cannot be listed", func() {
		var buf bytes.Buffer
		log.InitDefaultLogger(&buf)
		DeferCleanup(log.InitTestLogger)

		cache := agentapi.GetInstalledAppsCache(failingListRepo{Repository: mocks.NewMockRepository()})

		Expect(cache).To(BeEmpty())
		Expect(buf.String()).To(ContainSubstring("Failed to retrieve installed apps from database"))
		Expect(buf.String()).To(ContainSubstring("database is locked"))
		Expect(buf.String()).NotTo(ContainSubstring("%v"))
	})
})
