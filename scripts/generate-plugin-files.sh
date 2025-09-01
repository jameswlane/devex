#!/bin/bash

# Script to generate missing plugin files for all package manager plugins

set -e

PLUGINS_DIR="/data/GitHub/jameswlane/devex/packages/plugins"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Plugin configurations
declare -A PLUGIN_CONFIGS=(
    ["package-manager-appimage"]="AppImage:AppImage package manager for Linux:appimage,linux:linux:AppImage"
    ["package-manager-brew"]="Homebrew:Homebrew package manager for macOS and Linux:brew,homebrew,macos,linux:darwin,linux:brew"
    ["package-manager-curlpipe"]="Curl Pipe:Direct download and installation via curl:curl,download,script:darwin,linux,windows:curl"
    ["package-manager-deb"]="DEB:Debian package installer:deb,debian,package:linux:dpkg"
    ["package-manager-dnf"]="DNF:DNF package manager for Fedora/RHEL:dnf,fedora,rhel,linux:linux:dnf"
    ["package-manager-docker"]="Docker:Docker container management:docker,containers,linux:darwin,linux,windows:docker"
    ["package-manager-emerge"]="Emerge:Portage package manager for Gentoo:emerge,portage,gentoo,linux:linux:emerge"
    ["package-manager-eopkg"]="Eopkg:Eopkg package manager for Solus:eopkg,solus,linux:linux:eopkg"
    ["package-manager-flatpak"]="Flatpak:Flatpak universal package manager:flatpak,universal,linux:linux:flatpak"
    ["package-manager-mise"]="Mise:Mise development tool version manager:mise,tools,development:darwin,linux,windows:mise"
    ["package-manager-nixflake"]="Nix Flake:Nix flake package manager:nix,flake,functional:darwin,linux:nix"
    ["package-manager-nixpkgs"]="Nixpkgs:Nix packages collection:nix,nixpkgs,functional:darwin,linux:nix-env"
    ["package-manager-pacman"]="Pacman:Pacman package manager for Arch Linux:pacman,arch,linux:linux:pacman"
    ["package-manager-pip"]="Pip:Python package installer:pip,python,packages:darwin,linux,windows:pip"
    ["package-manager-rpm"]="RPM:RPM package manager:rpm,redhat,linux:linux:rpm"
    ["package-manager-snap"]="Snap:Snap universal package manager:snap,universal,linux:linux:snap"
    ["package-manager-xbps"]="XBPS:XBPS package manager for Void Linux:xbps,void,linux:linux:xbps-install"
    ["package-manager-yay"]="Yay:Yay AUR helper for Arch Linux:yay,aur,arch,linux:linux:yay"
    ["package-manager-zypper"]="Zypper:Zypper package manager for openSUSE:zypper,opensuse,suse,linux:linux:zypper"
)

generate_go_mod() {
    local plugin_name="$1"
    cat > "${PLUGINS_DIR}/${plugin_name}/go.mod" << EOF
module github.com/jameswlane/devex/packages/plugins/${plugin_name}

go 1.23

require (
	github.com/jameswlane/devex/packages/shared/plugin-sdk v0.1.0
)

replace github.com/jameswlane/devex/packages/shared/plugin-sdk => ../../shared/plugin-sdk
EOF
}

generate_package_json() {
    local plugin_name="$1"
    local display_name="$2"
    local description="$3"
    local tags="$4"
    local platforms="$5"
    local command="$6"
    
    # Convert comma-separated values to JSON arrays
    local json_tags=$(echo "$tags" | sed 's/,/", "/g' | sed 's/^/["/' | sed 's/$/"]/')
    local json_platforms=$(echo "$platforms" | sed 's/,/", "/g' | sed 's/^/["/' | sed 's/$/"]/')
    
    cat > "${PLUGINS_DIR}/${plugin_name}/package.json" << EOF
{
    "name": "@devex/plugin-${plugin_name}",
    "version": "1.0.0",
    "description": "${display_name} plugin for DevEx CLI",
    "main": "main.go",
    "scripts": {
        "build": "task build",
        "test": "task test",
        "lint": "task lint"
    },
    "keywords": ${json_tags},
    "author": "DevEx Team",
    "license": "GPL-3.0",
    "repository": {
        "type": "git",
        "url": "https://github.com/jameswlane/devex",
        "directory": "packages/plugins/${plugin_name}"
    },
    "devex": {
        "plugin": {
            "type": "package-manager",
            "platforms": ${json_platforms},
            "dependencies": ["${command}"],
            "priority": 10,
            "supports": {
                "install": true,
                "remove": true,
                "update": true,
                "search": true,
                "list": true
            }
        }
    }
}
EOF
}

generate_taskfile() {
    local plugin_name="$1"
    local platforms="$2"
    
    local build_commands=""
    IFS=',' read -ra PLATFORM_ARRAY <<< "$platforms"
    for platform in "${PLATFORM_ARRAY[@]}"; do
        case $platform in
            "darwin")
                build_commands+="\n      - GOOS=darwin GOARCH=amd64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-darwin-amd64 ."
                build_commands+="\n      - GOOS=darwin GOARCH=arm64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-darwin-arm64 ."
                ;;
            "linux")
                build_commands+="\n      - GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-linux-amd64 ."
                build_commands+="\n      - GOOS=linux GOARCH=arm64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-linux-arm64 ."
                ;;
            "windows")
                build_commands+="\n      - GOOS=windows GOARCH=amd64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-windows-amd64.exe ."
                build_commands+="\n      - GOOS=windows GOARCH=arm64 go build -ldflags=\"-s -w -X main.version={{.VERSION | default \\\"dev\\\"}}\" -o dist/devex-plugin-${plugin_name}-windows-arm64.exe ."
                ;;
        esac
    done
    
    cat > "${PLUGINS_DIR}/${plugin_name}/Taskfile.yml" << EOF
version: '3'

vars:
  PLUGIN_NAME: devex-plugin-${plugin_name}

tasks:
  build:
    desc: Build ${plugin_name} plugin for all platforms
    cmds:
      - mkdir -p dist${build_commands}

  test:
    desc: Run tests
    cmds:
      - go test -v ./...

  lint:
    desc: Run linters
    cmds:
      - golangci-lint run

  install:local:
    desc: Install plugin locally for testing
    cmds:
      - task: build
      - mkdir -p ~/.devex/plugins
      - cp dist/\{{.PLUGIN_NAME}}-\{{OS}}-\{{ARCH}}* ~/.devex/plugins/\{{.PLUGIN_NAME}}
      - chmod +x ~/.devex/plugins/\{{.PLUGIN_NAME}}

  clean:
    desc: Clean build artifacts
    cmds:
      - rm -rf dist/
EOF
}

generate_main_go() {
    local plugin_name="$1"
    local display_name="$2"
    local description="$3"
    local tags="$4"
    local command="$6"
    
    # Convert plugin-name to PluginName for Go struct
    local struct_name=$(echo "$plugin_name" | sed 's/-/ /g' | sed 's/package manager //g' | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) tolower(substr($i,2))} 1' | sed 's/ //g')
    
    # Convert comma-separated tags to Go slice
    local go_tags=$(echo "$tags" | sed 's/,/", "/g' | sed 's/^/"/' | sed 's/$/"/')
    
    cat > "${PLUGINS_DIR}/${plugin_name}/main.go" << EOF
package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// ${struct_name}Plugin implements the ${display_name} package manager
type ${struct_name}Plugin struct {
	*sdk.PackageManagerPlugin
}

// New${struct_name}Plugin creates a new ${display_name} plugin
func New${struct_name}Plugin() *${struct_name}Plugin {
	info := sdk.PluginInfo{
		Name:        "${plugin_name}",
		Version:     version,
		Description: "${description}",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{${go_tags}},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using ${display_name}",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using ${display_name}",
				Usage:       "Remove one or more packages from the system",
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Update package repository information",
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages",
			},
		},
	}

	return &${struct_name}Plugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "${command}"),
	}
}

// Execute handles command execution
func (p *${struct_name}Plugin) Execute(command string, args []string) error {
	p.EnsureAvailable()

	switch command {
	case "install":
		return p.handleInstall(args)
	case "remove":
		return p.handleRemove(args)
	case "update":
		return p.handleUpdate(args)
	case "search":
		return p.handleSearch(args)
	case "list":
		return p.handleList(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *${struct_name}Plugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "${command}", cmdArgs...)
}

func (p *${struct_name}Plugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "${command}", cmdArgs...)
}

func (p *${struct_name}Plugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "${command}", "update")
}

func (p *${struct_name}Plugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "${command}", "search", searchTerm)
}

func (p *${struct_name}Plugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "${command}", "list")
}

func main() {
	plugin := New${struct_name}Plugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
EOF
}

# Main execution
echo "Generating plugin files..."

for plugin_name in "${!PLUGIN_CONFIGS[@]}"; do
    # Skip if plugin already has all files
    if [[ -f "${PLUGINS_DIR}/${plugin_name}/go.mod" ]] && 
       [[ -f "${PLUGINS_DIR}/${plugin_name}/main.go" ]] && 
       [[ -f "${PLUGINS_DIR}/${plugin_name}/package.json" ]] && 
       [[ -f "${PLUGINS_DIR}/${plugin_name}/Taskfile.yml" ]]; then
        echo "✓ Skipping $plugin_name (already has all files)"
        continue
    fi
    
    echo "Generating files for $plugin_name..."
    
    # Parse configuration
    IFS=':' read -r display_name description tags platforms command <<< "${PLUGIN_CONFIGS[$plugin_name]}"
    
    # Generate files
    generate_go_mod "$plugin_name"
    generate_main_go "$plugin_name" "$display_name" "$description" "$tags" "$platforms" "$command"
    generate_package_json "$plugin_name" "$display_name" "$description" "$tags" "$platforms" "$command"
    generate_taskfile "$plugin_name" "$platforms"
    
    echo "✓ Generated files for $plugin_name"
done

echo "Plugin file generation complete!"