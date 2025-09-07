#!/bin/bash

# Migration script for converting sdk.ExecCommand() to sdk.ExecCommandWithContext()

set -e

# Directory containing all packages (relative to script location)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
PACKAGES_DIR="$PROJECT_ROOT/packages"

# Function to check if file needs context import
needs_context_import() {
    local file="$1"
    if grep -q "import.*context" "$file"; then
        return 1  # Already has context import
    fi
    if grep -q "sdk\.ExecCommand(" "$file"; then
        return 0  # Needs context import
    fi
    return 1  # Doesn't need import
}

# Function to add context import to a file
add_context_import() {
    local file="$1"
    
    # Check if it already has context import
    if grep -q '"context"' "$file"; then
        echo "  - Context import already exists"
        return
    fi
    
    # Add context import after the first import line
    sed -i '/^import (/,/^)$/{
        /^import (/{
            a\	"context"
        }
    }' "$file"
    
    echo "  - Added context import"
}

# Function to migrate a simple package manager main.go file
migrate_simple_package_manager() {
    local plugin_dir="$1"
    local plugin_name="$2"
    local command_name="$3"
    local main_file="$plugin_dir/main.go"
    
    echo "Migrating simple package manager: $plugin_name"
    
    if [[ ! -f "$main_file" ]]; then
        echo "  - Skipping: main.go not found"
        return
    fi
    
    # Add context import if needed
    if needs_context_import "$main_file"; then
        add_context_import "$main_file"
    fi
    
    # Add replace directive for local SDK
    cd "$plugin_dir"
    go mod edit -replace github.com/jameswlane/devex/packages/plugin-sdk=../plugin-sdk
    echo "  - Added SDK replace directive"
    
    # Create context in Execute method
    sed -i '/Execute.*error.*{/,/^}$/{
        /\.EnsureAvailable()/a\\n\tctx := context.Background()
    }' "$main_file"
    
    # Update function signatures and calls
    # This is a simplified approach for the common pattern
    sed -i 's/func (p \*.*Plugin) handle\([A-Z][a-z]*\)(args \[\]string) error {/func (p *'$plugin_name'Plugin) handle\1(ctx context.Context, args []string) error {/g' "$main_file"
    sed -i 's/p\.handle\([A-Z][a-z]*\)(args)/p.handle\1(ctx, args)/g' "$main_file"
    sed -i 's/sdk\.ExecCommand(/sdk.ExecCommandWithContext(ctx, /g' "$main_file"
    
    echo "  - Updated function signatures and calls"
    
    # Test build
    if go build -o /tmp/test-build . >/dev/null 2>&1; then
        echo "  - ✅ Build successful"
        rm -f /tmp/test-build
    else
        echo "  - ❌ Build failed - manual review needed"
    fi
    
    cd - >/dev/null
}

# List of simple package managers (following same pattern)
declare -A simple_managers=(
    ["package-manager-apk"]="apk"
    ["package-manager-brew"]="brew"
    ["package-manager-emerge"]="emerge"
    ["package-manager-eopkg"]="eopkg"
    ["package-manager-nixflake"]="nix-env"
    ["package-manager-nixpkgs"]="nix-env"
    ["package-manager-pacman"]="pacman"
    ["package-manager-rpm"]="rpm"
    ["package-manager-snap"]="snap"
    ["package-manager-xbps"]="xbps-install"
    ["package-manager-yay"]="yay"
)

# Migrate simple package managers
echo "=== Migrating Simple Package Managers ==="
for plugin_dir in "${!simple_managers[@]}"; do
    command_name="${simple_managers[$plugin_dir]}"
    plugin_name=$(echo "$plugin_dir" | sed 's/package-manager-//' | sed 's/.*/\u&/')  # Capitalize first letter
    full_path="$PACKAGES_DIR/$plugin_dir"
    
    if [[ -d "$full_path" ]]; then
        migrate_simple_package_manager "$full_path" "$plugin_name" "$command_name"
    else
        echo "Skipping: $plugin_dir - directory not found"
    fi
done

echo ""
echo "=== Simple Package Manager Migration Complete ==="
echo "Next steps: Review complex plugins manually:"
echo "  - package-manager-apt (multiple files)"
echo "  - package-manager-docker (multiple files)"
echo "  - package-manager-flatpak (multiple files)"
echo "  - package-manager-mise (multiple files)"
echo "  - package-manager-pip (multiple files)"
echo "  - tool-git (git_*.go files)"
echo "  - tool-shell (shell_*.go files)"
echo "  - desktop-kde (widgets.go)"
echo "  - system-setup (main.go)"