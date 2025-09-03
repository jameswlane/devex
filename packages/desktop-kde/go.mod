module github.com/jameswlane/devex/packages/desktop-kde

go 1.23.0

toolchain go1.24.6

replace github.com/jameswlane/devex/packages/plugin-sdk => ../plugin-sdk

require github.com/jameswlane/devex/packages/plugin-sdk v0.0.0-00010101000000-000000000000

require golang.org/x/crypto v0.35.0 // indirect
