module github.com/jameswlane/devex/packages/package-manager-emerge

go 1.24.0

require github.com/jameswlane/devex/packages/plugin-sdk v0.0.1

require (
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace github.com/jameswlane/devex/packages/plugin-sdk => ../plugin-sdk
