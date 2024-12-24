package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/types"
)

func TestAppConfigValidation(t *testing.T) {
	t.Parallel()

	validApp := types.AppConfig{
		Name:           "test-app",
		InstallMethod:  "docker",
		InstallCommand: "docker run test-app",
	}
	assert.NoError(t, validApp.Validate())

	missingName := types.AppConfig{
		InstallMethod:  "docker",
		InstallCommand: "docker run test-app",
	}
	assert.EqualError(t, missingName.Validate(), "Name is required")

	missingMethod := types.AppConfig{
		Name:           "test-app",
		InstallCommand: "docker run test-app",
	}
	assert.EqualError(t, missingMethod.Validate(), "InstallMethod is required")

	missingCommand := types.AppConfig{
		Name:          "test-app",
		InstallMethod: "docker",
	}
	assert.EqualError(t, missingCommand.Validate(), "install command is required for app test-app")

	validAptApp := types.AppConfig{
		Name:           "test-apt-app",
		InstallMethod:  "apt",
		InstallCommand: "apt-get install test-apt-app",
		AptSources: []types.AptSource{
			{
				KeySource:  "http://example.com/key",
				KeyName:    "example-key",
				SourceRepo: "http://example.com/repo",
				SourceName: "example.list",
			},
		},
	}
	assert.NoError(t, validAptApp.Validate())

	invalidAptApp := types.AppConfig{
		Name:          "test-apt-app",
		InstallMethod: "apt",
		AptSources: []types.AptSource{
			{
				KeySource:  "",
				KeyName:    "",
				SourceRepo: "",
				SourceName: "",
			},
		},
	}
	assert.EqualError(t, invalidAptApp.Validate(), "APT source must have a list_file and repo defined")

	validCurlpipeApp := types.AppConfig{
		Name:          "test-curlpipe-app",
		InstallMethod: "curlpipe",
		DownloadURL:   "http://example.com/download",
	}
	assert.NoError(t, validCurlpipeApp.Validate())

	invalidCurlpipeApp := types.AppConfig{
		Name:          "test-curlpipe-app",
		InstallMethod: "curlpipe",
	}
	assert.EqualError(t, invalidCurlpipeApp.Validate(), "download URL is required for app test-curlpipe-app with install method curlpipe")
}

func TestDockerOptionsValidation(t *testing.T) {
	t.Parallel()

	validOptions := types.DockerOptions{
		ContainerName: "test-container",
	}
	assert.NoError(t, validOptions.Validate())

	missingContainerName := types.DockerOptions{}
	assert.EqualError(t, missingContainerName.Validate(), "ContainerName is required for DockerOptions")
}

func TestAppConfigUnmarshal(t *testing.T) {
	t.Parallel()

	yamlData := `
name: test-app
install_method: docker
install_command: docker run test-app
`
	var app types.AppConfig
	err := yaml.Unmarshal([]byte(yamlData), &app)
	assert.NoError(t, err)
	assert.Equal(t, "test-app", app.Name)
	assert.Equal(t, "docker", app.InstallMethod)
	assert.Equal(t, "docker run test-app", app.InstallCommand)
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()

	app := types.AppConfig{}
	assert.False(t, app.Default) // Ensure Default is false by default
}
