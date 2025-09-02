#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PLUGINS_DIR="packages/plugins"
REGISTRY_DIR="apps/registry"
FAILED_PLUGINS=()
SUCCESSFUL_PLUGINS=()

# Function to log with timestamp
log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

# Function to build a single plugin
build_single_plugin() {
    local plugin=$1
    local plugin_dir="$PLUGINS_DIR/$plugin"
    
    log "${BLUE}Building plugin: $plugin${NC}"
    
    # Check if plugin directory exists
    if [ ! -d "$plugin_dir" ]; then
        log "${RED}❌ Plugin directory not found: $plugin_dir${NC}"
        return 1
    fi
    
    # Check if main.go exists
    if [ ! -f "$plugin_dir/main.go" ]; then
        log "${RED}❌ main.go not found for plugin: $plugin${NC}"
        return 1
    fi
    
    # Build the plugin
    (
        cd "$plugin_dir"
        
        # Update dependencies
        log "Updating dependencies for $plugin..."
        if go mod tidy; then
            log "${GREEN}✅ Dependencies updated for $plugin${NC}"
        else
            log "${RED}❌ Failed to update dependencies for $plugin${NC}"
            return 1
        fi
        
        # Run tests if available
        if [ -f "Taskfile.yml" ] && grep -q "test:" Taskfile.yml; then
            log "Running tests for $plugin..."
            if task test; then
                log "${GREEN}✅ Tests passed for $plugin${NC}"
            else
                log "${RED}❌ Tests failed for $plugin${NC}"
                return 1
            fi
        fi
        
        # Build
        log "Building $plugin..."
        if go build -v -o "devex-plugin-$plugin" .; then
            log "${GREEN}✅ Build successful for $plugin${NC}"
        else
            log "${RED}❌ Build failed for $plugin${NC}"
            return 1
        fi
        
        # Test plugin info
        if ./devex-plugin-$plugin --plugin-info > /dev/null 2>&1; then
            log "${GREEN}✅ Plugin info test passed for $plugin${NC}"
        else
            log "${RED}❌ Plugin info test failed for $plugin${NC}"
            return 1
        fi
    )
}

# Function to release a single plugin
release_single_plugin() {
    local plugin=$1
    local version=${2:-$(date +%Y.%m.%d)}
    
    log "${BLUE}Releasing plugin: $plugin v$version${NC}"
    
    # Create goreleaser config for this plugin
    cat > ".goreleaser.$plugin.yml" << EOF
version: 2
project_name: devex-plugin-$plugin

builds:
  - id: $plugin
    main: ./$PLUGINS_DIR/$plugin
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
    owner: ${OWNER:-jameswlane}
    name: ${REPO:-devex}
  name_template: "Plugin {{ .ProjectName }} v{{ .Version }}"
  draft: false
  prerelease: false
  mode: append
EOF

    # Determine OS-specific constraints for system plugins
    case $plugin in
        system-linux)
            sed -i '/goos:/,/goarch:/{/goos:/!b;n;c\      - linux' ".goreleaser.$plugin.yml"
            ;;
        system-macos)
            sed -i '/goos:/,/goarch:/{/goos:/!b;n;c\      - darwin' ".goreleaser.$plugin.yml"
            ;;
        system-windows)
            sed -i '/goos:/,/goarch:/{/goos:/!b;n;c\      - windows' ".goreleaser.$plugin.yml"
            ;;
    esac
    
    # Create a tag for this plugin
    local plugin_tag="${plugin}-v${version}"
    
    # Tag if not exists
    if ! git rev-parse "$plugin_tag" >/dev/null 2>&1; then
        git tag -a "$plugin_tag" -m "Release $plugin v$version"
    fi
    
    # Run goreleaser
    if goreleaser release --config ".goreleaser.$plugin.yml" --clean; then
        log "${GREEN}✅ Successfully released $plugin${NC}"
        rm ".goreleaser.$plugin.yml"
        return 0
    else
        log "${RED}❌ Failed to release $plugin${NC}"
        rm -f ".goreleaser.$plugin.yml"
        return 1
    fi
}

# Function to process plugins in parallel
process_plugins_parallel() {
    local plugins=("$@")
    local pids=()
    local max_jobs=${MAX_JOBS:-4}
    
    log "${BLUE}Processing ${#plugins[@]} plugins with max $max_jobs parallel jobs${NC}"
    
    for plugin in "${plugins[@]}"; do
        # Wait if we've reached max parallel jobs
        while [ ${#pids[@]} -ge $max_jobs ]; do
            for i in "${!pids[@]}"; do
                if ! kill -0 "${pids[$i]}" 2>/dev/null; then
                    wait "${pids[$i]}"
                    local exit_code=$?
                    unset 'pids[$i]'
                    pids=("${pids[@]}")  # Reindex array
                    break
                fi
            done
            sleep 0.1
        done
        
        # Start plugin processing in background
        (
            if build_single_plugin "$plugin"; then
                if [ "${RELEASE_MODE:-build}" = "release" ]; then
                    if release_single_plugin "$plugin"; then
                        echo "$plugin" >> /tmp/successful_plugins.txt
                    else
                        echo "$plugin" >> /tmp/failed_plugins.txt
                    fi
                else
                    echo "$plugin" >> /tmp/successful_plugins.txt
                fi
            else
                echo "$plugin" >> /tmp/failed_plugins.txt
            fi
        ) &
        
        pids+=($!)
    done
    
    # Wait for all remaining jobs
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    # Collect results
    if [ -f /tmp/successful_plugins.txt ]; then
        mapfile -t SUCCESSFUL_PLUGINS < /tmp/successful_plugins.txt
        rm /tmp/successful_plugins.txt
    fi
    
    if [ -f /tmp/failed_plugins.txt ]; then
        mapfile -t FAILED_PLUGINS < /tmp/failed_plugins.txt
        rm /tmp/failed_plugins.txt
    fi
}

# Function to display summary
show_summary() {
    echo
    log "${BLUE}========== BUILD SUMMARY ==========${NC}"
    
    if [ ${#SUCCESSFUL_PLUGINS[@]} -gt 0 ]; then
        log "${GREEN}✅ Successful plugins (${#SUCCESSFUL_PLUGINS[@]}):${NC}"
        for plugin in "${SUCCESSFUL_PLUGINS[@]}"; do
            echo "   - $plugin"
        done
    fi
    
    if [ ${#FAILED_PLUGINS[@]} -gt 0 ]; then
        log "${RED}❌ Failed plugins (${#FAILED_PLUGINS[@]}):${NC}"
        for plugin in "${FAILED_PLUGINS[@]}"; do
            echo "   - $plugin"
        done
    fi
    
    echo
    log "${BLUE}Total: ${#SUCCESSFUL_PLUGINS[@]} succeeded, ${#FAILED_PLUGINS[@]} failed${NC}"
    
    # Exit with error if any plugins failed
    if [ ${#FAILED_PLUGINS[@]} -gt 0 ]; then
        return 1
    fi
}

# Main function
main() {
    local command=${1:-help}
    shift || true
    
    case $command in
        build)
            local plugins=("$@")
            if [ ${#plugins[@]} -eq 0 ]; then
                plugins=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \;))
            fi
            
            RELEASE_MODE=build
            process_plugins_parallel "${plugins[@]}"
            show_summary
            ;;
            
        release)
            local plugins=("$@")
            if [ ${#plugins[@]} -eq 0 ]; then
                plugins=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \;))
            fi
            
            RELEASE_MODE=release
            process_plugins_parallel "${plugins[@]}"
            show_summary
            ;;
            
        test)
            local plugin=$1
            if [ -z "$plugin" ]; then
                log "${RED}Usage: $0 test <plugin-name>${NC}"
                exit 1
            fi
            
            build_single_plugin "$plugin"
            ;;
            
        help|--help|-h)
            cat << EOF
Individual Plugin Release Tool

Usage: $0 [command] [options]

Commands:
  build [plugins...]     Build specified plugins (or all if none specified)
  release [plugins...]   Build and release specified plugins
  test <plugin>         Test a single plugin build
  help                  Show this help message

Environment Variables:
  MAX_JOBS              Maximum parallel jobs (default: 4)
  GITHUB_TOKEN          Required for releases
  OWNER                 GitHub owner (default: jameswlane)
  REPO                  GitHub repo (default: devex)

Examples:
  $0 build                          # Build all plugins
  $0 build package-manager-apt      # Build specific plugin
  $0 release                        # Release all plugins
  $0 release desktop-gnome desktop-kde  # Release specific plugins
  $0 test package-manager-docker   # Test single plugin

This tool builds plugins in parallel and continues even if some fail,
providing a summary at the end showing which succeeded and which failed.
EOF
            ;;
            
        *)
            log "${RED}Unknown command: $command${NC}"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Check requirements
check_requirements() {
    local missing=()
    
    command -v go >/dev/null || missing+=("go")
    command -v git >/dev/null || missing+=("git")
    
    if [ "${RELEASE_MODE:-build}" = "release" ]; then
        command -v goreleaser >/dev/null || missing+=("goreleaser")
        
        if [ -z "$GITHUB_TOKEN" ]; then
            log "${RED}GITHUB_TOKEN environment variable is required for releases${NC}"
            exit 1
        fi
    fi
    
    if [ ${#missing[@]} -ne 0 ]; then
        log "${RED}Missing required tools: ${missing[*]}${NC}"
        exit 1
    fi
}

# Initialize temp files
rm -f /tmp/successful_plugins.txt /tmp/failed_plugins.txt

# Run
check_requirements
main "$@"