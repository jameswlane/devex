#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 DevEx Plugin Development Status${NC}"
echo "================================="
echo

# Check if we're in a git repository
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo -e "${RED}❌ Not in a git repository${NC}"
    exit 1
fi

# Check for staged plugin changes
echo -e "${CYAN}📋 Staged Changes${NC}"
staged_plugins=$(git diff --cached --name-only | grep "^packages/plugins/" | cut -d'/' -f3 | sort -u || true)
if [ -n "$staged_plugins" ]; then
    echo -e "${YELLOW}Plugins with staged changes:${NC}"
    echo "$staged_plugins" | sed 's/^/  - /'
else
    echo "  No staged plugin changes"
fi
echo

# Check for unstaged plugin changes  
echo -e "${CYAN}🔧 Unstaged Changes${NC}"
unstaged_plugins=$(git diff --name-only | grep "^packages/plugins/" | cut -d'/' -f3 | sort -u || true)
if [ -n "$unstaged_plugins" ]; then
    echo -e "${YELLOW}Plugins with unstaged changes:${NC}"
    echo "$unstaged_plugins" | sed 's/^/  - /'
else
    echo "  No unstaged plugin changes"
fi
echo

# Show plugin release status
echo -e "${CYAN}🎯 Plugin Release Status${NC}"
if command -v ./scripts/check-plugin-changes.sh >/dev/null 2>&1; then
    ./scripts/check-plugin-changes.sh
else
    echo "  Plugin check script not found"
fi
echo

# Check for common issues
echo -e "${CYAN}🔍 Health Check${NC}"

# Check for missing go.mod files
echo "Checking for missing go.mod files..."
missing_gomod=0
for plugin_dir in packages/plugins/*/; do
    if [ -d "$plugin_dir" ] && [ ! -f "$plugin_dir/go.mod" ]; then
        echo -e "  ${RED}❌ Missing go.mod: $plugin_dir${NC}"
        missing_gomod=1
    fi
done
if [ $missing_gomod -eq 0 ]; then
    echo -e "  ${GREEN}✅ All plugins have go.mod files${NC}"
fi

# Check for missing package.json files
echo "Checking for missing package.json files..."
missing_packagejson=0
for plugin_dir in packages/plugins/*/; do
    if [ -d "$plugin_dir" ] && [ ! -f "$plugin_dir/package.json" ]; then
        echo -e "  ${RED}❌ Missing package.json: $plugin_dir${NC}"
        missing_packagejson=1
    fi
done
if [ $missing_packagejson -eq 0 ]; then
    echo -e "  ${GREEN}✅ All plugins have package.json files${NC}"
fi

# Check for plugins that might need dependency updates
echo "Checking for plugins that might need 'go mod tidy'..."
needs_tidy=0
for plugin_dir in packages/plugins/*/; do
    if [ -d "$plugin_dir" ] && [ -f "$plugin_dir/go.mod" ]; then
        cd "$plugin_dir"
        if ! go mod tidy -diff >/dev/null 2>&1; then
            echo -e "  ${YELLOW}⚠️  Needs 'go mod tidy': $plugin_dir${NC}"
            needs_tidy=1
        fi
        cd - >/dev/null
    fi
done
if [ $needs_tidy -eq 0 ]; then
    echo -e "  ${GREEN}✅ All plugin dependencies are up to date${NC}"
fi
echo

# Show helpful commands
echo -e "${CYAN}💡 Helpful Commands${NC}"
echo "Plugin-specific commands (use with 'lefthook run'):"
echo -e "  ${GREEN}plugin-check${NC}          - Check which plugins have changes"
echo -e "  ${GREEN}plugin-check-verbose${NC}  - Show changes with commit details"
echo -e "  ${GREEN}plugin-build${NC}          - Build all changed plugins"
echo -e "  ${GREEN}plugin-tidy-all${NC}       - Run 'go mod tidy' on all plugins"
echo -e "  ${GREEN}plugin-format-all${NC}     - Format all plugin Go files"
echo ""
echo "Direct script usage:"
echo -e "  ${GREEN}./scripts/check-plugin-changes.sh${NC}              - Check plugin changes"
echo -e "  ${GREEN}./scripts/release-plugin-individual.sh build${NC}   - Build changed plugins"
echo -e "  ${GREEN}node scripts/determine-plugin-version.js check <plugin>${NC} - Check version"
echo ""

# Show git status summary
if [ -n "$staged_plugins" ] || [ -n "$unstaged_plugins" ]; then
    echo -e "${CYAN}🎬 Next Steps${NC}"
    if [ -n "$unstaged_plugins" ]; then
        echo "1. Stage your changes: git add packages/plugins/..."
    fi
    if [ -n "$staged_plugins" ]; then
        echo "2. Commit with conventional commit format:"
        echo "   git commit -m 'feat(plugin-name): description'"
    fi
    echo "3. Push to trigger automated plugin release"
fi