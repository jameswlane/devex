#!/bin/bash

set -euo pipefail

# Colors for output (disabled for better visibility)
GREEN=''
YELLOW=''
BLUE=''
NC=''

TEMPLATE_FILE="templates/plugin-goreleaser.yml"
PACKAGES_DIR="packages"

# Function to determine supported OS list for a plugin
get_plugin_goos() {
    local plugin_name=$1
    
    case $plugin_name in
        # Desktop plugins - Linux only
        desktop-*)
            echo "      - linux"
            ;;
        # Linux-specific package managers
        package-manager-apt|package-manager-dnf|package-manager-pacman|package-manager-rpm|\
        package-manager-snap|package-manager-flatpak|package-manager-emerge|package-manager-eopkg|\
        package-manager-xbps|package-manager-yay|package-manager-zypper|package-manager-apk|\
        package-manager-appimage|package-manager-deb)
            echo "      - linux"
            ;;
        # macOS-specific package managers  
        package-manager-brew)
            echo "      - darwin
      - linux"
            ;;
        # Cross-platform package managers
        package-manager-curlpipe|package-manager-docker|package-manager-mise|package-manager-pip)
            echo "      - darwin
      - linux
      - windows"
            ;;
        # Nix package managers - Linux and macOS
        package-manager-nixflake|package-manager-nixpkgs)
            echo "      - darwin
      - linux"
            ;;
        # System tools - cross-platform
        system-setup|tool-git|tool-shell|tool-stackdetector)
            echo "      - darwin
      - linux
      - windows"
            ;;
        *)
            # Default to Linux only for unknown plugins
            echo "      - linux"
            ;;
    esac
}

# Function to get usage example for a plugin
get_usage_example() {
    local plugin_name=$1
    
    case $plugin_name in
        desktop-*)
            local desktop_env=${plugin_name#desktop-}
            echo "install desktop --environment $desktop_env"
            ;;
        package-manager-*)
            local pm=${plugin_name#package-manager-}
            echo "install packages --package-manager $pm <package-name>"
            ;;
        system-setup)
            echo "system setup"
            ;;
        tool-git)
            echo "tool git config"
            ;;
        tool-shell)
            echo "tool shell configure"
            ;;
        tool-stackdetector)
            echo "tool stackdetector analyze"
            ;;
        *)
            echo "plugin $plugin_name --help"
            ;;
    esac
}

# Function to generate goreleaser config for a plugin
generate_plugin_config() {
    local plugin_dir=$1
    local plugin_name=$(basename "$plugin_dir")
    local binary_name="devex-plugin-$plugin_name"
    local plugin_id="plugin-${plugin_name//[-_]/-}"
    
    echo -e "${BLUE}Generating .goreleaser.yml for $plugin_name${NC}"
    
    # Check if plugin directory exists and has main.go
    if [ ! -d "$plugin_dir" ]; then
        echo -e "${YELLOW}⚠️  Plugin directory not found: $plugin_dir${NC}"
        return 1
    fi
    
    if [ ! -f "$plugin_dir/main.go" ]; then
        echo -e "${YELLOW}⚠️  main.go not found in: $plugin_dir${NC}"
        return 1
    fi
    
    # Get plugin-specific configuration
    local goos_list=$(get_plugin_goos "$plugin_name")
    local usage_example=$(get_usage_example "$plugin_name")
    
    # Copy template and customize
    cp "$TEMPLATE_FILE" "$plugin_dir/.goreleaser.yml"
    
    # Replace placeholders
    sed -i "s/__PLUGIN_NAME__/$binary_name/g" "$plugin_dir/.goreleaser.yml"
    sed -i "s/__PLUGIN_ID__/$plugin_id/g" "$plugin_dir/.goreleaser.yml"
    sed -i "s/__BINARY_NAME__/$binary_name/g" "$plugin_dir/.goreleaser.yml"
    sed -i "s/__USAGE_EXAMPLE__/$usage_example/g" "$plugin_dir/.goreleaser.yml"
    
    # Replace GOOS list (more complex replacement)
    sed -i "/goos: __GOOS_LIST__/c\\
    goos:$goos_list" "$plugin_dir/.goreleaser.yml"
    
    echo -e "${GREEN}✅ Generated .goreleaser.yml for $plugin_name${NC}"
}

# Main function
main() {
    echo -e "${GREEN}🚀 Generating individual GoReleaser configs for all plugins${NC}"
    echo "================================================================="
    
    # Check if template exists
    if [ ! -f "$TEMPLATE_FILE" ]; then
        echo "❌ Template file not found: $TEMPLATE_FILE"
        exit 1
    fi
    
    local generated=0
    local skipped=0
    
    # Find all plugin directories
    for plugin_dir in packages/*; do
        if [ -d "$plugin_dir" ] && [ "$(basename "$plugin_dir")" != "plugin-sdk" ]; then
            if generate_plugin_config "$plugin_dir"; then
                ((generated++))
            else
                ((skipped++))
            fi
        fi
    done
    
    echo
    echo -e "${GREEN}📊 Summary:${NC}"
    echo -e "${GREEN}✅ Generated: $generated configs${NC}"
    if [ $skipped -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Skipped: $skipped plugins${NC}"
    fi
    
    echo
    echo -e "${GREEN}🎉 Individual GoReleaser configs generated successfully!${NC}"
    echo
    echo -e "${BLUE}Next steps:${NC}"
    echo "1. Review generated configs in each plugin directory"
    echo "2. Test individual plugin releases with: goreleaser release --skip-publish"
    echo "3. Update CI/CD workflows to use individual configs"
}

main "$@"