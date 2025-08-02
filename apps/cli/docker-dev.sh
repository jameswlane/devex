#!/bin/bash
# DevEx CLI Development Testing Script
# This script builds the CLI and creates a Docker environment for fast testing

set -e

echo "🚀 Building DevEx CLI for testing..."

# Build the CLI binary
task build

# Ensure assets directory exists
if [ ! -d "assets" ]; then
    echo "⚠️  Warning: assets directory not found, creating empty one"
    mkdir -p assets
fi

echo "🐳 Building Docker development environment..."

# Build the Docker image
docker build -f Dockerfile.dev -t devex-dev .

echo "✅ Docker environment ready!"
echo ""
echo "Usage:"
echo "  # Start interactive testing environment"
echo "  ./docker-dev.sh run"
echo ""
echo "  # Run devex setup directly"
echo "  ./docker-dev.sh setup"
echo ""
echo "  # Run with verbose logging"
echo "  ./docker-dev.sh setup-verbose"
echo ""
echo "  # Test automated mode"
echo "  ./docker-dev.sh setup-auto"

# Handle command line arguments
case "${1:-help}" in
    "run")
        echo "🔧 Starting interactive DevEx development environment..."
        echo "   You can now run 'devex setup' or other commands"
        echo "   Type 'exit' to return to host"
        echo ""
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev
        ;;
    "setup")
        echo "🔧 Running 'devex setup' in development environment..."
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev \
            devex setup
        ;;
    "setup-verbose")
        echo "🔧 Running 'devex setup --verbose' in development environment..."
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev \
            devex setup --verbose
        ;;
    "setup-auto")
        echo "🔧 Running 'devex setup --non-interactive' in development environment..."
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev \
            devex setup --non-interactive
        ;;
    "logs")
        echo "🔧 Running 'devex setup --verbose' and showing logs..."
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev \
            bash -c "devex setup --verbose; echo '📋 Logs:'; find /home/devex -name '*.log' -exec cat {} \;"
        ;;
    "debug")
        echo "🔍 Starting debug environment with shell access..."
        echo "   Run 'devex setup --verbose' to test"
        echo "   Check logs with: find /home/devex -name '*.log' -exec cat {} \;"
        docker run -it --rm \
            --privileged \
            -v /var/run/docker.sock:/var/run/docker.sock \
            devex-dev \
            bash
        ;;
    "help"|*)
        echo ""
        echo "DevEx CLI Development Testing Environment"
        echo ""
        echo "Commands:"
        echo "  run           - Start interactive environment"
        echo "  setup         - Run 'devex setup' directly"
        echo "  setup-verbose - Run 'devex setup --verbose'"
        echo "  setup-auto    - Run 'devex setup --non-interactive'"
        echo "  logs          - Run setup and show all logs"
        echo "  debug         - Start with shell for manual testing"
        echo "  help          - Show this help"
        ;;
esac