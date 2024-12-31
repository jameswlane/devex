# DevEx

DevEx is a powerful CLI tool designed to streamline the setup and management of development environments. It simplifies the installation of applications, configuration of programming languages, and customization of themes.

---

## Features

- **Custom Configuration Management**: Tailor application, GNOME extension, and programming language setups with YAML files.
- **Automated Releases**: Leverage `commitizen`, `semantic-release`, and `goreleaser` for seamless versioning and publishing.
- **Task Automation**: Use `Taskfile` for efficient script execution and workflow management.
- **Community Support**: Engage with contributors through GitHub Issues, Discussions, and Wiki.
- **Prettier Formatting**: Standardize YAML and Markdown files with Prettier.
- **Comprehensive Website**: Access guides, documentation, and updates at [devex.sh](https://devex.sh).

---

## Getting Started

### Prerequisites

- **Go**: Version 1.20 or later.
- **Mise**: Install from the [Mise GitHub page](https://github.com/mise/mise).
- **Prettier**: Ensure Prettier is installed for formatting YAML and Markdown.

### Installation

To install DevEx:

```bash
task install
```

---

## Configuration

### Custom Configuration Files

Custom configurations are stored under `~/.devex/`:

```plaintext
~/.devex/
    ├── apps.yaml
    ├── gnome_extensions.yaml
    ├── programming_languages.yaml
    ├── config/
    │   └── additional_configs.yaml
    └── themes.yaml
```

### Default vs Custom Configuration

DevEx prioritizes custom configurations in `~/.devex/`. If not found, it falls back to defaults in the `assets/` directory.

#### Example: `apps.yaml`

```yaml
apps:
   - name: "Visual Studio Code"
     description: "Code editor from Microsoft"
     category: "Editors"
     install_method: "apt"
     install_command: "code"
     dependencies:
        - "gnome-shell"
        - "git"
```

### Formatting

To format configuration files, run:

```bash
prettier --write "**/*.{yaml,md}"
```

---

## Taskfile Integration

We use `Task` for task automation. Below is an overview of the `Taskfile.yml`:

### Default Task

- **default**: Runs linting and testing.
  ```bash
  task default
  ```

### Build Tasks

- **build**: Builds the Go project.
  ```bash
  task build
  ```

- **build:local**: Builds the Go project for local development.
  ```bash
  task build:local
  ```

### Installation Task

- **install**: Installs DevEx.
  ```bash
  task install
  ```

### Setup Python Environment

- **setup:python**: Sets up Python environment and installs requirements.
  ```bash
  task setup:python
  ```

### Manage Go Modules

- **mod**: Downloads and tidies Go modules.
  ```bash
  task mod
  ```

### Clean Up Temporary Files

- **clean**: Cleans temp files and folders.
  ```bash
  task clean
  ```

### Linting Tasks

- **lint**: Runs golangci-lint.
  ```bash
  task lint
  ```

- **lint:fix**: Runs golangci-lint and fixes issues.
  ```bash
  task lint:fix
  ```

- **lint:staticcheck**: Runs staticcheck.
  ```bash
  task lint:staticcheck
  ```

### Vulnerability Checks

- **vulncheck**: Runs vulnerability checks.
  ```bash
  task vulncheck
  ```

### Testing Tasks

- **test**: Runs test suite.
  ```bash
  task test
  ```

- **test:all**: Runs test suite with additional tags.
  ```bash
  task test:all
  ```

- **test:testify**: Runs tests with testify.
  ```bash
  task test:testify
  ```

- **test:ginkgo**: Runs tests with Ginkgo.
  ```bash
  task test:ginkgo
  ```

### Mock Generation

- **mockgen**: Generates mocks for interfaces.
  ```bash
  task mockgen
  ```

### Prettier Formatting

- **prettier:check**: Checks if files are formatted with Prettier.
  ```bash
  task prettier:check
  ```

- **prettier:fix**: Formats files with Prettier.
  ```bash
  task prettier:fix
  ```

### Documentation Tasks

- **docs:build**: Builds the MkDocs site.
  ```bash
  task docs:build
  ```

- **docs:serve**: Serves MkDocs documentation locally.
  ```bash
  task docs:serve
  ```

### Code Visualization

- **callvis**: Generates a visualization of code.
  ```bash
  task callvis
  ```

### Static Analysis

- **gocritic**: Runs Go Critic for advanced analysis.
  ```bash
  task gocritic
  ```

### CLI Tasks

- **cli:generate**: Generates CLI commands.
  ```bash
  task cli:generate
  ```

### GoReleaser Tasks

- **goreleaser:test**: Tests the release process without publishing.
  ```bash
  task goreleaser:test
  ```

- **goreleaser:install**: Installs GoReleaser.
  ```bash
  task goreleaser:install
  ```

### Release Management

- **release:\***: Prepares the project for a new release.
  ```bash
  task release:<version>
  ```

### Package Listing

- **packages**: Lists Go packages.
  ```bash
  task packages
  ```

To execute tasks, simply run:

```bash
task <task-name>
```

---

## Development

### Testing

Run all tests:

```bash
task test
```

### Linting

Install `golangci-lint`:

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
```

Run the linter:

```bash
task lint
```

### Building

Build the application:

```bash
go build -o bin/devex cmd/devex/main.go
```

---

## Automated Releases

DevEx uses `commitizen`, `semantic-release`, and `goreleaser` for automated versioning and releases.

To prepare a release:

```bash
task release:<version>
```

Where `<version>` can be `major`, `minor`, `patch`, or a specific semantic version (e.g., `1.2.3`).

---

## Community and Support

### GitHub Features

- **[Issues](https://github.com/jameswlane/devex/issues)**: Report bugs or request features.
- **[Discussions](https://github.com/jameswlane/devex/discussions)**: Ask questions or share ideas.
- **[Docs](https://docs.devex.sh)**: View documentation.
- **[Projects](https://github.com/jameswlane/devex/projects)**: Track project progress.
- **[Wiki](https://github.com/jameswlane/devex/wiki)**: Access in-depth documentation.
- **[Security](https://github.com/jameswlane/devex/security)**: Report vulnerabilities.
- **[Pulse](https://github.com/jameswlane/devex/pulse)**: View project activity.

### Website

Visit the official website at [devex.sh](https://devex.sh) for documentation, guides, and updates.

---

## Contributing

Contributions are welcome! Refer to the [Contributing Guide](.github/CONTRIBUTING.md) for details.

### Code of Conduct

We expect all contributors to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## License

DevEx is licensed under the [GNU GPL v3 License](LICENSE).

---

## Security

For security concerns, please refer to our [Security Policy](SECURITY.md).
