#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PLUGINS_DIR="packages/plugins"
REGISTRY_DIR="apps/registry"

echo -e "${GREEN}🚀 DevEx Plugin Release System${NC}"
echo "=================================="

# Function to get changed plugins since last release
get_changed_plugins() {
    local last_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    local changed_plugins=()

    if [ -z "$last_tag" ]; then
        echo -e "${YELLOW}No previous tag found, releasing all plugins${NC}" >&2
        changed_plugins=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \;))
    else
        echo "Checking for changes since $last_tag..." >&2
        local changed_files=$(git diff --name-only $last_tag..HEAD 2>/dev/null)

        for file in $changed_files; do
            if [[ $file == $PLUGINS_DIR/* ]]; then
                local plugin_name=$(echo $file | cut -d'/' -f3)
                if [[ -n "$plugin_name" && ! " ${changed_plugins[@]} " =~ " ${plugin_name} " ]]; then
                    changed_plugins+=("$plugin_name")
                fi
            fi
        done
    fi

    echo "${changed_plugins[@]}"
}

# Function to bump plugin version
bump_plugin_version() {
    local plugin_name=$1
    local bump_type=${2:-patch}
    local plugin_dir="$PLUGINS_DIR/$plugin_name"

    if [ ! -f "$plugin_dir/package.json" ]; then
        echo -e "${RED}No package.json found for plugin $plugin_name${NC}"
        return 1
    fi

    cd "$plugin_dir"

    # Use npm version to bump version
    npm version $bump_type --no-git-tag-version
    local new_version=$(node -p "require('./package.json').version")

    echo -e "${GREEN}Bumped $plugin_name to v$new_version${NC}"
    cd - > /dev/null

    echo $new_version
}

# Function to build specific plugins
build_plugins() {
    local plugins=("$@")

    if [ ${#plugins[@]} -eq 0 ]; then
        echo -e "${YELLOW}No plugins to build${NC}"
        return 0
    fi

    echo -e "${GREEN}Building plugins: ${plugins[*]}${NC}"

    # Use turbo to build only changed plugins
    local filter_args=""
    for plugin in "${plugins[@]}"; do
        filter_args="$filter_args --filter=packages/plugins/$plugin"
    done

    pnpm turbo run build $filter_args
}

# Function to create plugin releases
create_plugin_releases() {
    local plugins=("$@")
    local release_tag="plugins-$(date +%Y%m%d-%H%M%S)"

    # Create a lightweight tag for plugin releases
    git tag -a "$release_tag" -m "Plugin release: ${plugins[*]}"

    # Generate custom goreleaser config for plugins only
    cat > .goreleaser.plugins.yml << EOF
version: 2
project_name: devex-plugins

builds:
EOF

    # Add builds for each changed plugin
    for plugin in "${plugins[@]}"; do
        case $plugin in
            package-manager-*)
                add_package_manager_build $plugin
                ;;
            system-*)
                add_system_plugin_build $plugin
                ;;
            *)
                add_generic_plugin_build $plugin
                ;;
        esac
    done

    # Add common configuration
    cat >> .goreleaser.plugins.yml << EOF

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: 'checksums.txt'

release:
  github:
    owner: $OWNER
    name: $REPO
  name_template: "Plugins {{ .Tag }}"
  mode: replace

after:
  hooks:
    - cmd: node scripts/generate-registry.js {{ .Tag }}
      env:
        - GITHUB_TOKEN={{ .Env.GITHUB_TOKEN }}
EOF

    # Run goreleaser with plugin-specific config
    goreleaser release --config .goreleaser.plugins.yml --clean

    # Clean up temporary config
    rm .goreleaser.plugins.yml
}

add_package_manager_build() {
    local plugin=$1
    cat >> .goreleaser.plugins.yml << EOF
  - id: $plugin
    main: ./packages/plugins/$plugin
    binary: devex-plugin-$plugin
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
EOF
}

add_system_plugin_build() {
    local plugin=$1
    local os_filter=""

    case $plugin in
        system-linux)
            os_filter="
    goos:
      - linux"
            ;;
        system-macos)
            os_filter="
    goos:
      - darwin"
            ;;
        system-windows)
            os_filter="
    goos:
      - windows"
            ;;
        *)
            os_filter="
    goos:
      - linux
      - darwin
      - windows"
            ;;
    esac

    cat >> .goreleaser.plugins.yml << EOF
  - id: $plugin
    main: ./packages/plugins/$plugin
    binary: devex-plugin-$plugin
    env:
      - CGO_ENABLED=0$os_filter
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
EOF
}

add_generic_plugin_build() {
    local plugin=$1
    cat >> .goreleaser.plugins.yml << EOF
  - id: $plugin
    main: ./packages/plugins/$plugin
    binary: devex-plugin-$plugin
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
EOF
}

# Function to update plugin registry
update_registry() {
    local version=$1
    echo -e "${GREEN}Updating plugin registry...${NC}"

    node scripts/generate-registry.js $version

    # Deploy to Vercel if in CI
    if [ -n "$VERCEL_TOKEN" ]; then
        echo -e "${GREEN}Deploying registry to Vercel...${NC}"
        cd $REGISTRY_DIR
        npx vercel --prod --token $VERCEL_TOKEN
        cd - > /dev/null
    fi
}

# Function to validate plugins before release
validate_plugins() {
    local plugins=("$@")
    local failed=false

    echo -e "${GREEN}Validating plugins...${NC}"

    for plugin in "${plugins[@]}"; do
        local plugin_dir="$PLUGINS_DIR/$plugin"

        # Check if plugin directory exists
        if [ ! -d "$plugin_dir" ]; then
            echo -e "${RED}❌ Plugin directory not found: $plugin_dir${NC}"
            failed=true
            continue
        fi

        # Check if main.go exists
        if [ ! -f "$plugin_dir/main.go" ]; then
            echo -e "${RED}❌ main.go not found for plugin: $plugin${NC}"
            failed=true
            continue
        fi

        # Update dependencies before building
        echo "Updating dependencies for $plugin..."
        cd "$plugin_dir"
        if ! go mod tidy; then
            echo -e "${RED}❌ Failed to update dependencies for plugin: $plugin${NC}"
            failed=true
            cd - > /dev/null
            continue
        fi

        # Try to build the plugin
        echo "Building $plugin for validation..."
        if ! go build -o /tmp/test-$plugin .; then
            echo -e "${RED}❌ Failed to build plugin: $plugin${NC}"
            failed=true
            cd - > /dev/null
            continue
        fi

        # Test plugin info command
        if ! /tmp/test-$plugin --plugin-info > /dev/null; then
            echo -e "${RED}❌ Plugin info command failed for: $plugin${NC}"
            failed=true
        else
            echo -e "${GREEN}✅ Plugin $plugin validated${NC}"
        fi

        # Cleanup
        rm -f /tmp/test-$plugin
        cd - > /dev/null
    done

    if [ "$failed" = true ]; then
        echo -e "${RED}Plugin validation failed${NC}"
        exit 1
    fi

    echo -e "${GREEN}All plugins validated successfully${NC}"
}

# Main script logic
main() {
    local command=${1:-"auto"}

    case $command in
        "auto")
            echo "🔍 Detecting changed plugins..."
            local changed_plugins=($(get_changed_plugins))

            if [ ${#changed_plugins[@]} -eq 0 ]; then
                echo -e "${YELLOW}No plugin changes detected${NC}"
                exit 0
            fi

            echo -e "${GREEN}Changed plugins: ${changed_plugins[*]}${NC}"
            validate_plugins "${changed_plugins[@]}"
            build_plugins "${changed_plugins[@]}"

            # Bump versions and create release
            local release_version="plugins-$(date +%Y.%m.%d)"
            create_plugin_releases "${changed_plugins[@]}"
            update_registry $release_version
            ;;

        "all")
            echo "🔄 Releasing all plugins..."
            local all_plugins=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \;))
            validate_plugins "${all_plugins[@]}"
            build_plugins "${all_plugins[@]}"

            local release_version="plugins-all-$(date +%Y.%m.%d)"
            create_plugin_releases "${all_plugins[@]}"
            update_registry $release_version
            ;;

        "plugin")
            if [ -z "$2" ]; then
                echo -e "${RED}Usage: $0 plugin <plugin-name> [bump-type]${NC}"
                exit 1
            fi

            local plugin_name=$2
            local bump_type=${3:-patch}

            echo "🔧 Releasing single plugin: $plugin_name"
            validate_plugins "$plugin_name"
            build_plugins "$plugin_name"

            local new_version=$(bump_plugin_version $plugin_name $bump_type)
            create_plugin_releases "$plugin_name"
            update_registry "plugin-$plugin_name-v$new_version"
            ;;

        "validate")
            local plugins_to_validate=("$@")
            plugins_to_validate=("${plugins_to_validate[@]:1}") # Remove 'validate' from array

            if [ ${#plugins_to_validate[@]} -eq 0 ]; then
                plugins_to_validate=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \;))
            fi

            validate_plugins "${plugins_to_validate[@]}"
            ;;

        "help"|"-h"|"--help")
            cat << EOF
DevEx Plugin Release System

Usage: $0 [command] [options]

Commands:
  auto                    Detect changed plugins and release them (default)
  all                     Release all plugins
  plugin <name> [type]    Release specific plugin with version bump
                         bump types: major, minor, patch (default: patch)
  validate [plugins...]   Validate plugins without releasing
  help                    Show this help message

Examples:
  $0                                    # Auto-detect and release changed plugins
  $0 all                               # Release all plugins
  $0 plugin package-manager-apt minor  # Release APT plugin with minor version bump
  $0 validate                          # Validate all plugins
  $0 validate package-manager-apt      # Validate specific plugin

Environment Variables:
  GITHUB_TOKEN    Required for creating releases
  VERCEL_TOKEN    Required for deploying registry (optional in CI)
EOF
            ;;

        *)
            echo -e "${RED}Unknown command: $command${NC}"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Check required tools
check_requirements() {
    local missing=()

    command -v node >/dev/null || missing+=("node")
    command -v pnpm >/dev/null || missing+=("pnpm")
    command -v go >/dev/null || missing+=("go")
    command -v git >/dev/null || missing+=("git")
    command -v goreleaser >/dev/null || missing+=("goreleaser")

    if [ ${#missing[@]} -ne 0 ]; then
        echo -e "${RED}Missing required tools: ${missing[*]}${NC}"
        echo "Please install the missing tools and try again"
        exit 1
    fi

    if [ -z "$GITHUB_TOKEN" ]; then
        echo -e "${RED}GITHUB_TOKEN environment variable is required${NC}"
        exit 1
    fi
}

# Run checks and main function
check_requirements
main "$@"
