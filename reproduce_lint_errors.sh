#!/bin/bash

echo "Running golangci-lint to reproduce the errors..."
cd /data/GitHub/jameswlane/devex

# First try to run from the cli app directory where the problematic files are
cd apps/cli
golangci-lint run || echo 'golangci-lint not available, trying with go build'

# If golangci-lint is not available, try go build to see compilation errors
if ! command -v golangci-lint &> /dev/null; then
    echo "Trying to build the package to see compilation errors..."
    go build ./pkg/tui/... || echo "Build failed with errors shown above"
fi
