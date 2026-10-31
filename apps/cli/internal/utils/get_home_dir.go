package utils

import sdk "github.com/jameswlane/devex/packages/plugin-sdk"

func GetHomeDir() (string, error) {
	identity, err := sdk.NewPrivilegeResolver().ResolveUser(nil)
	if err != nil {
		return "", err
	}
	return identity.HomeDir, nil
}
