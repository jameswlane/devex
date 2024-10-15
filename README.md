# DevEx

## Setup Environment

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
go build -o myapp ./...
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
