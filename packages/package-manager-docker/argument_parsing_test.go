package main_test

import (
	"reflect"
	"strings"
	"testing"

	main "github.com/jameswlane/devex/packages/package-manager-docker"
)

func TestBuildDockerRunArgsFromTemplateInstallFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerRunArgs([]string{
		"postgres:16",
		"--name=devex-postgres",
		"--port=127.0.0.1:5432:5432",
		"--env=POSTGRES_HOST_AUTH_METHOD=trust",
		"--volume=pgdata:/var/lib/postgresql/data",
		"--restart=unless-stopped",
		"--detach",
	})
	if err != nil {
		t.Fatalf("BuildDockerRunArgs returned error: %v", err)
	}

	expected := []string{
		"run",
		"-d",
		"--name", "devex-postgres",
		"-p", "127.0.0.1:5432:5432",
		"-e", "POSTGRES_HOST_AUTH_METHOD=trust",
		"-v", "pgdata:/var/lib/postgresql/data",
		"--restart", "unless-stopped",
		"postgres:16",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerRunArgsSupportsSpaceSeparatedFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerRunArgs([]string{
		"redis:7",
		"--name", "devex-redis",
		"-p", "127.0.0.1:6379:6379",
	})
	if err != nil {
		t.Fatalf("BuildDockerRunArgs returned error: %v", err)
	}

	expected := []string{"run", "-d", "--name", "devex-redis", "-p", "127.0.0.1:6379:6379", "redis:7"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerRunArgsRejectsUnsupportedInstallFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	_, err := plugin.BuildDockerRunArgs([]string{"redis:7", "--privileged"})
	if err == nil || !strings.Contains(err.Error(), "unsupported docker install flag") {
		t.Fatalf("expected unsupported flag error, got %v", err)
	}
}

func TestBuildDockerRemoveArgsKeepsFlagsSeparateFromContainerNames(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, containers, err := plugin.BuildDockerRemoveArgs([]string{
		"devex-postgres",
		"--volumes",
		"--force",
	})
	if err != nil {
		t.Fatalf("BuildDockerRemoveArgs returned error: %v", err)
	}

	expectedArgs := []string{"rm", "-v", "-f", "devex-postgres"}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("unexpected remove args:\nwant: %#v\n got: %#v", expectedArgs, args)
	}
	expectedContainers := []string{"devex-postgres"}
	if !reflect.DeepEqual(containers, expectedContainers) {
		t.Fatalf("unexpected containers:\nwant: %#v\n got: %#v", expectedContainers, containers)
	}
}

func TestBuildDockerComposeArgsNormalizesGlobalFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerComposeArgs([]string{
		"up",
		"--file=compose.yaml",
		"--project=devex-stack",
		"--detach",
	})
	if err != nil {
		t.Fatalf("BuildDockerComposeArgs returned error: %v", err)
	}

	expected := []string{
		"compose",
		"-f", "compose.yaml",
		"-p", "devex-stack",
		"up",
		"-d",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected compose args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerComposeArgsRejectsUnsafeComposeFiles(t *testing.T) {
	plugin := main.NewDockerPlugin()

	unsafePaths := []string{
		"",
		"../../../etc/passwd",
		"/etc/shadow",
		"path; rm -rf /",
		"path && curl evil.com",
	}
	for _, path := range unsafePaths {
		_, err := plugin.BuildDockerComposeArgs([]string{"up", "--file=" + path})
		if err == nil || !strings.Contains(err.Error(), "invalid compose file path") {
			t.Fatalf("expected invalid compose file path for %q, got %v", path, err)
		}
	}
}
