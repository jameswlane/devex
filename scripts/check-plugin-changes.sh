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

echo -e "${BLUE}🔍 DevEx Plugin Change Detector${NC}"
echo "=================================="

# Function to get last release tag for a plugin
get_last_plugin_tag() {
    local plugin=$1
    git tag -l "${plugin}-v*" --sort=-version:refname 2>/dev/null | head -1
}

# Function to check if plugin has changes
plugin_has_changes() {
    local plugin=$1
    local last_tag=$2
    
    if [ -z "$last_tag" ]; then
        # No previous release, so it has changes
        return 0
    fi
    
    # Check for changes since last tag
    local changes=$(git diff --name-only "$last_tag"..HEAD -- "$PLUGINS_DIR/$plugin" 2>/dev/null | wc -l)
    [ "$changes" -gt 0 ]
}

# Function to analyze commit types
analyze_commits() {
    local plugin=$1
    local last_tag=$2
    
    local cmd="git log --pretty=format:'%s' "
    if [ -n "$last_tag" ]; then
        cmd+="$last_tag..HEAD"
    fi
    cmd+=" -- $PLUGINS_DIR/$plugin"
    
    local commits=$(eval "$cmd" 2>/dev/null)
    
    local has_breaking=false
    local has_feat=false
    local has_fix=false
    
    while IFS= read -r commit; do
        if [[ "$commit" =~ ^[a-zA-Z]+!: ]] || [[ "$commit" =~ BREAKING\ CHANGE ]]; then
            has_breaking=true
        elif [[ "$commit" =~ ^feat(\(.+\))?:  ]]; then
            has_feat=true
        elif [[ "$commit" =~ ^fix(\(.+\))?:  ]]; then
            has_fix=true
        fi
    done <<< "$commits"
    
    if [ "$has_breaking" = true ]; then
        echo "major"
    elif [ "$has_feat" = true ]; then
        echo "minor"
    elif [ "$has_fix" = true ]; then
        echo "patch"
    else
        echo "patch"
    fi
}

# Main logic
main() {
    local all_plugins=($(find $PLUGINS_DIR -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort))
    local changed_plugins=()
    local unchanged_plugins=()
    
    echo -e "${BLUE}Checking ${#all_plugins[@]} plugins for changes...${NC}"
    echo
    
    for plugin in "${all_plugins[@]}"; do
        local last_tag=$(get_last_plugin_tag "$plugin")
        local current_version="0.0.0"
        
        # Get current version from package.json
        if [ -f "$PLUGINS_DIR/$plugin/package.json" ]; then
            current_version=$(jq -r '.version // "0.0.0"' "$PLUGINS_DIR/$plugin/package.json")
        fi
        
        if plugin_has_changes "$plugin" "$last_tag"; then
            local bump_type=$(analyze_commits "$plugin" "$last_tag")
            local version_info=""
            
            if [ -n "$last_tag" ]; then
                version_info="$current_version → next: $bump_type bump"
            else
                version_info="$current_version (first release)"
            fi
            
            changed_plugins+=("$plugin|$version_info|$bump_type")
            echo -e "${GREEN}✓${NC} $plugin - ${YELLOW}$version_info${NC}"
            
            # Show recent commits
            if [ "$VERBOSE" = "true" ]; then
                echo "  Recent commits:"
                git log --oneline -5 --pretty=format:"    %s" "$last_tag"..HEAD -- "$PLUGINS_DIR/$plugin" 2>/dev/null || true
                echo
            fi
        else
            unchanged_plugins+=("$plugin")
            if [ "$SHOW_ALL" = "true" ]; then
                echo -e "${BLUE}-${NC} $plugin - v$current_version (no changes)"
            fi
        fi
    done
    
    # Summary
    echo
    echo -e "${BLUE}========== SUMMARY ==========${NC}"
    echo -e "${GREEN}Changed plugins:${NC} ${#changed_plugins[@]}"
    echo -e "${BLUE}Unchanged plugins:${NC} ${#unchanged_plugins[@]}"
    
    if [ ${#changed_plugins[@]} -gt 0 ]; then
        echo
        echo -e "${YELLOW}Plugins ready for release:${NC}"
        for plugin_info in "${changed_plugins[@]}"; do
            IFS='|' read -r plugin version bump <<< "$plugin_info"
            echo "  - $plugin ($version)"
        done
        
        echo
        echo -e "${BLUE}To release these plugins, run:${NC}"
        echo "  git push origin main"
        echo "  # or manually trigger the workflow"
    fi
}

# Parse command line arguments
VERBOSE=false
SHOW_ALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -a|--all)
            SHOW_ALL=true
            shift
            ;;
        -h|--help)
            cat << EOF
Plugin Change Detector

Usage: $0 [options]

Options:
  -v, --verbose    Show recent commits for changed plugins
  -a, --all        Show all plugins, including unchanged ones
  -h, --help       Show this help message

Examples:
  $0              # Show only changed plugins
  $0 -v           # Show changed plugins with recent commits
  $0 -a           # Show all plugins
  $0 -v -a        # Show all plugins with commits for changed ones

This tool detects which plugins have changes since their last release
and determines the appropriate version bump based on conventional commits.
EOF
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use '$0 --help' for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main