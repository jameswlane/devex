#!/bin/bash
# Quick CLI testing script - builds and runs devex locally for fast iteration

set -e

echo "🔨 Building DevEx CLI..."
task build

echo "✅ Build complete! DevEx CLI ready for testing."
echo ""
echo "Testing commands:"
echo "  ./bin/devex setup --verbose          # Interactive setup with verbose logs"
echo "  ./bin/devex setup --non-interactive  # Automated setup"
echo "  ./bin/devex --help                   # Show help"
echo ""

# If arguments provided, run them
if [ $# -gt 0 ]; then
    echo "🚀 Running: ./bin/devex $*"
    echo ""
    ./bin/devex "$@"
else
    echo "💡 Tip: Run './test-cli.sh setup --verbose' to test setup directly"
fi