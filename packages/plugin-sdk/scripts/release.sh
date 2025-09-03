#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}[RELEASE]${NC} $1"
}

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$SDK_DIR"

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -f ".goreleaser.yml" ]; then
    print_error "This script must be run from the plugin-sdk directory"
    exit 1
fi

# Check if git is clean
if ! git diff-index --quiet HEAD --; then
    print_error "Git working directory is not clean. Please commit or stash your changes."
    exit 1
fi

# Get current version or ask for it
if [ -z "$1" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    echo ""
    echo "Current tags:"
    git tag -l "plugin-sdk/v*" | sort -V | tail -5
    exit 1
fi

VERSION="$1"

# Validate version format
if ! [[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
    print_error "Version must be in format v1.2.3 or v1.2.3-beta"
    exit 1
fi

TAG_NAME="plugin-sdk/$VERSION"

print_header "Preparing Plugin SDK release $VERSION"

# Check if tag already exists
if git tag -l | grep -q "^$TAG_NAME$"; then
    print_error "Tag $TAG_NAME already exists"
    exit 1
fi

# Run tests
print_status "Running tests..."
if ! go run github.com/onsi/ginkgo/v2/ginkgo run .; then
    print_error "Tests failed"
    exit 1
fi

# Run linter
print_status "Running linter..."
if ! golangci-lint run; then
    print_error "Linter failed"
    exit 1
fi

# Update go.mod if needed
print_status "Tidying go.mod..."
go mod tidy

# Check if any files were modified
if ! git diff-index --quiet HEAD --; then
    print_warning "go mod tidy made changes. Committing..."
    git add go.mod go.sum
    git commit -m "chore(plugin-sdk): tidy go.mod for release $VERSION"
fi

# Create and push tag
print_status "Creating tag $TAG_NAME..."
git tag -a "$TAG_NAME" -m "Plugin SDK release $VERSION

This release includes:
- Comprehensive Go plugin SDK
- Ginkgo test suite with 72+ test cases
- Support for plugin lifecycle management
- Registry client for plugin distribution
- Cryptographic signature verification
- Background update system

Installation:
go get github.com/jameswlane/devex/packages/plugin-sdk@$VERSION"

print_status "Pushing tag to origin..."
git push origin "$TAG_NAME"

# Wait for GitHub Actions
print_status "Tag pushed successfully!"
print_status "GitHub Actions will now:"
print_status "  1. Run tests and linting"
print_status "  2. Create GitHub release with GoReleaser"
print_status "  3. Publish to Go module proxy"
print_status ""
print_status "View the release progress at:"
print_status "  https://github.com/jameswlane/devex/actions"
print_status ""
print_status "Once complete, the module will be available as:"
print_status "  go get github.com/jameswlane/devex/packages/plugin-sdk@$VERSION"

print_header "Release process initiated successfully! 🚀"