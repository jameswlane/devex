package utils

import (
	"os"
	"os/user"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type OSUserProvider struct{}

func (OSUserProvider) Current() (*user.User, error) {
	log.Info("Fetching current user")
	usr, err := user.Current()
	if err != nil {
		log.Error("Failed to fetch current user", err)
		return nil, ErrUserNotFound
	}
	log.Info("Current user fetched successfully", "username", usr.Username)
	return usr, nil
}

func (OSUserProvider) Lookup(username string) (*user.User, error) {
	log.Info("Looking up user", "username", username)
	usr, err := user.Lookup(username)
	if err != nil {
		log.Error("Failed to look up user", err, "username", username)
		return nil, ErrUserNotFound
	}
	log.Info("User lookup successful", "username", usr.Username)
	return usr, nil
}

var UserProviderInstance types.UserProvider = OSUserProvider{}

// CurrentUserEnvOverride fetches the current user, allowing for overrides via an environment variable.
func CurrentUserEnvOverride(envVar string) (*user.User, error) {
	log.Info("Fetching current user with environment override", "envVar", envVar)

	username := os.Getenv(envVar)
	if username != "" {
		log.Info("Environment variable override detected", "username", username)
		return UserProviderInstance.Lookup(username)
	}

	log.Info("No environment variable override, fetching current user")
	return UserProviderInstance.Current()
}
