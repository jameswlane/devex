module github.com/jameswlane/devex/packages/package-manager-nixflake

go 1.26.0

require github.com/jameswlane/devex/packages/plugin-sdk v0.0.1

require (
	github.com/ProtonMail/go-crypto v1.5.2 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/jameswlane/devex/packages/plugin-sdk => ../plugin-sdk
