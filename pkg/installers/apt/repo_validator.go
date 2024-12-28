package apt

import (
	"fmt"
	"strings"
)

// ValidateAptRepo ensures the repository string is valid.
func ValidateAptRepo(repo string) error {
	if repo == "" || len(repo) < 10 || !containsValidKeywords(repo) {
		return fmt.Errorf("invalid repository format: %s", repo)
	}
	return nil
}

func containsValidKeywords(repo string) bool {
	return strings.Contains(repo, "deb") && strings.Contains(repo, "http")
}
