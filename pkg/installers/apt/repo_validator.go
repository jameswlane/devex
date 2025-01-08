package apt

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
)

// ValidateAptRepo ensures the repository string is valid.
func ValidateAptRepo(repo string) error {
	log.Info("Validating APT repository", "repo", repo)

	if repo == "" || len(repo) < 10 || !containsValidKeywords(repo) {
		log.Error("Invalid repository format", fmt.Errorf("repository: %s", repo))
		return fmt.Errorf("invalid repository format: %s", repo)
	}

	log.Info("APT repository validated successfully", "repo", repo)
	return nil
}

func containsValidKeywords(repo string) bool {
	return strings.Contains(repo, "deb") && strings.Contains(repo, "http")
}
