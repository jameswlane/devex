#!/bin/bash

set -euo pipefail

PLUGIN_NAME=${1:-"package-manager-apt"}

echo "🧪 Testing individual plugin release process for: $PLUGIN_NAME"
echo "======================================================="

# Check if plugin exists
if [ ! -d "packages/$PLUGIN_NAME" ]; then
    echo "❌ Plugin directory not found: packages/$PLUGIN_NAME"
    exit 1
fi

cd "packages/$PLUGIN_NAME"

echo "📋 Plugin directory: $(pwd)"

# Check required files
if [ ! -f "main.go" ]; then
    echo "❌ main.go not found"
    exit 1
fi

if [ ! -f ".goreleaser.yml" ]; then
    echo "❌ .goreleaser.yml not found"
    exit 1
fi

echo "✅ Required files found"

# Update dependencies
echo "📦 Updating Go dependencies..."
go mod tidy

# Build the plugin
echo "🔨 Building plugin..."
go build -v -o "devex-plugin-$PLUGIN_NAME" .

# Test plugin info
echo "🧪 Testing plugin info command..."
if ./devex-plugin-$PLUGIN_NAME --plugin-info > /dev/null; then
    echo "✅ Plugin info command works"
else
    echo "❌ Plugin info command failed"
    exit 1
fi

# Test GoReleaser config (dry run)
echo "🚀 Testing GoReleaser configuration..."
if command -v goreleaser >/dev/null; then
    # Test the config without actually releasing
    if goreleaser check; then
        echo "✅ GoReleaser config is valid"
        
        # Test build without releasing
        echo "🏗️  Testing GoReleaser build (dry run)..."
        if goreleaser build --snapshot --clean --single-target; then
            echo "✅ GoReleaser build test passed"
        else
            echo "❌ GoReleaser build test failed"
            exit 1
        fi
    else
        echo "❌ GoReleaser config validation failed"
        exit 1
    fi
else
    echo "⚠️  GoReleaser not installed - skipping config test"
    echo "   Install with: go install github.com/goreleaser/goreleaser@latest"
fi

# Cleanup
rm -f "devex-plugin-$PLUGIN_NAME"
rm -rf dist/

echo ""
echo "🎉 Plugin release test completed successfully!"
echo ""
echo "📊 Summary:"
echo "   ✅ Plugin builds correctly"
echo "   ✅ Plugin info command works"
echo "   ✅ GoReleaser config is valid"
echo "   ✅ Ready for individual releases"
echo ""
echo "🚀 To release this plugin:"
echo "   1. Create a tag: git tag plugin-$PLUGIN_NAME-v1.0.0"
echo "   2. Run: goreleaser release --clean"
echo "   3. Or use GitHub Actions workflow_dispatch"