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

```yaml
version: '3'

vars:
   BIN: "{{.ROOT_DIR}}/bin"

tasks:
   default:
      cmds:
         - task: lint
         - task: test

   install:
      desc: Installs DevEx
      aliases: [i]
      sources:
         - './**/*.go'
      cmds:
         - go install -v ./cmd/devex

   lint:
      desc: Runs golangci-lint
      cmds:
         - golangci-lint run

   test:
      desc: Runs test suite
      cmds:
         - go test ./...
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
