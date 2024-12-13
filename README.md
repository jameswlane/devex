# DevEx

## Setup Environment


Here’s a documentation snippet for how the custom configuration works, which you can add to your README in Markdown format:

---

## Custom Configuration Files

The `devex` tool allows users to provide custom configuration files for applications, GNOME extensions, programming languages, and more. If no custom configuration is provided, `devex` will fall back to default configurations stored within the `assets/` directory.

### Configuration File Structure

Custom configuration files should be stored in the user's home directory under a hidden `.devex` folder with subdirectories for different categories of configuration files:

```
~/.devex/
    ├── apps.yaml
    ├── gnome_extensions.yaml
    ├── programming_languages.yaml
    ├── config/
    │   └── additional_configs.yaml
    └── themes.yaml
```

Each YAML file defines a specific aspect of the `devex` environment. Here’s an overview of each configuration file:

1. **apps.yaml**  
   Defines applications to be installed and managed by `devex`, including installation methods and any dependencies.

2. **gnome_extensions.yaml**  
   Lists the GNOME extensions that should be installed and managed.

3. **programming_languages.yaml**  
   Specifies the programming languages and tools (e.g., Python, Go) to install and configure.

4. **themes.yaml**  
   Defines visual themes and any related configuration.

### Default vs Custom Configuration

The `devex` tool will first check for custom configuration files in the `~/.devex` directory. If it finds a custom file, it will load that instead of the default configuration. If no custom file is found, `devex` will fall back to its built-in defaults stored in the `assets/` directory.

For example, the tool looks for the `apps.yaml` file like this:

- **Custom file path**: `~/.devex/apps.yaml`
- **Default fallback**: `assets/apps.yaml`

This allows users to tailor their installations and configurations without modifying the tool’s default behavior.

### How to Customize

To create a custom configuration, you can copy the relevant YAML file from the `assets/` folder into the `~/.devex/` directory. For example:

```bash
cp assets/apps.yaml ~/.devex/apps.yaml
```

Then, edit `~/.devex/apps.yaml` as needed to add or modify applications.

### Example: apps.yaml

Here’s an example structure of an `apps.yaml` file:

```yaml
apps:
  - name: "Visual Studio Code"
    description: "Code editor from Microsoft"
    category: "Editors"
    install_method: "apt"
    install_command: "code"
    uninstall_command: "apt-get remove -y code"
    dependencies:
      - "gnome-shell"
      - "git"
  - name: "Docker"
    description: "Containerization platform"
    category: "DevOps"
    install_method: "apt"
    install_command: "docker"
    uninstall_command: "apt-get remove -y docker"
```

### Example: gnome_extensions.yaml

```yaml
extensions:
  - id: "clipboard-indicator@tudmotu.com"
    name: "Clipboard Indicator"
  - id: "user-themes@gnome-shell-extensions.gcampax.github.com"
    name: "User Themes"
```

### How to Load Custom Configurations

To load custom configurations, simply run `devex` as you normally would. The tool will automatically detect and load any custom YAML files in the `~/.devex/` directory.

If you ever want to revert to the default configuration, simply remove the corresponding custom YAML file from `~/.devex/`.

---

### Prerequisites

- Go 1.20 or later
- Mise

### Install Mise

To install Mise, follow the instructions on the [Mise GitHub page](https://github.com/mise/mise).

### Install Dependencies

Run the following command to install the required dependencies:

```sh
mise use --global go@1.20
```

## Linting

### Install golangci-lint

To install `golangci-lint`, run the following command:

```sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
```

If using Mise, you can install `golangci-lint` to the Mise installation directory:
```sh
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ~/.local/share/mise/installs/go/1.23.1/bin v1.61.0
```

For more information, refer to the [golangci-lint documentation](https://golangci-lint.run/usage/install/).

### Run Linter

To run the linter, use the following command:

```sh
golangci-lint run
```

## Testing

To run the tests, use the following command:

```sh
go test ./...
```

## Building

To build the application, use the following command:

```sh
go build -o bin/devex cmd/devex/main.go
```

## Common Information

### Project Structure

- `cmd/`: Contains the main application entry point.
- `pkg/`: Contains the library code.
- `config/`: Contains configuration files.

### Dependency Management

This project uses Go modules for dependency management. Dependencies are listed in the `go.mod` file.

### Versioning

This project follows semantic versioning. Tags are used to mark release versions.

### Releasing

To create a new release, push a new tag to the repository:

```sh
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0
```

This will trigger the CI/CD pipeline to build and create a release.

## License

All content created by Rafaël De Jongh and published on www.RafaelDeJongh.com, or anywhere else on the internet is licensed under a Creative Commons Attribution-NonCommercial-NoDerivatives 4.0 International License, unless stated otherwise.

If you use any work or redistribute it in any form, please remember to credit the original author (Rafaël De Jongh) and others if stated, in the description, as a footnote or in the authors/credits section of the website or online or offline publication you are going to use it on.

Requesting permission to use any of the work for any online or offline publication is always required and therefore highly advised. Any inquiry for commercial use of any of my content, please contact me at: info@rafaeldejongh.com
