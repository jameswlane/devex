module github.com/jameswlane/devex/packages/package-manager-docker

go 1.23.0

require github.com/jameswlane/devex/packages/plugin-sdk v0.1.0

require golang.org/x/crypto v0.35.0 // indirect

replace github.com/jameswlane/devex/packages/plugin-sdk => ../plugin-sdk
