module github.com/jameswlane/devex/packages/plugins/desktop-fonts

go 1.23

require (
	github.com/jameswlane/devex/packages/shared/plugin-sdk v0.1.0
	github.com/spf13/afero v1.11.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/jameswlane/devex/packages/shared/plugin-sdk => ../../shared/plugin-sdk
