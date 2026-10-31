package config

import devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

func devexHomeDir() (string, error) { p, err := devexpaths.Resolve(); return p.Home, err }
