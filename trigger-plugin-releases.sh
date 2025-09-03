#!/bin/bash

set -e

echo "🚀 Triggering plugin releases by adding build timestamps..."

# Find all plugins
plugins=($(find packages -mindepth 1 -maxdepth 1 -type d -name 'tool-*' -o -name 'desktop-*' -o -name 'package-manager-*' | xargs -I {} basename {}))

echo "Found ${#plugins[@]} plugins to update:"
printf ' - %s\n' "${plugins[@]}"

timestamp=$(date '+%Y-%m-%d %H:%M:%S')

for plugin in "${plugins[@]}"; do
    echo "📦 Updating $plugin..."
    
    plugin_path="packages/$plugin"
    
    # Add a build timestamp comment to the main.go file to trigger a change
    if [ -f "$plugin_path/main.go" ]; then
        # Remove any existing build timestamp comments
        sed -i '/\/\/ Build timestamp:/d' "$plugin_path/main.go"
        
        # Add new build timestamp comment after package declaration
        sed -i "/^package main$/a\\
\\
// Build timestamp: $timestamp" "$plugin_path/main.go"
        
        echo "  ✅ Added build timestamp to main.go"
    else
        echo "  ⚠️  No main.go found in $plugin_path"
    fi
done

echo ""
echo "🎯 Summary:"
echo "  - Updated ${#plugins[@]} plugins"
echo "  - Added build timestamp: $timestamp"
echo "  - This should trigger the plugin workflow to create proper releases"
echo ""
echo "📋 Next steps:"
echo "  1. Commit and push these changes"
echo "  2. Wait for plugin workflow to trigger"
echo "  3. Check that GoReleaser creates binary assets"
echo "  4. Verify registry gets populated"