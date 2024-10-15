package actions

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/config"
	"time"
)

// InstallLanguage installs a programming language
func InstallLanguage(lang config.ProgrammingLanguage) error {
	fmt.Printf("Installing programming language: %s\n", lang.Name)
	time.Sleep(1 * time.Second) // Simulate language installation
	return nil
}
