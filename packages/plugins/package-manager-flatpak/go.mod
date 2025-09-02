module github.com/jameswlane/devex/packages/plugins/package-manager-flatpak

go 1.23

require github.com/jameswlane/devex/packages/shared/plugin-sdk v0.1.0

require golang.org/x/crypto v0.31.0 // indirect

replace github.com/jameswlane/devex/packages/shared/plugin-sdk => ../../shared/plugin-sdk
