#!/bin/bash
# CLI Docker Script
# Builds the CLI, sets up a Docker container, and runs the CLI in an isolated environment.

set -e

# Directories and parameters
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CLI_BINARY="$PROJECT_ROOT/bin/devex"
DOCKER_IMAGE="cli-test-env"
DEFAULT_DISTRO="debian" # Default to Debian
DISTRO="${DISTRO:-$DEFAULT_DISTRO}" # Use $DISTRO environment variable or fallback to default
DOCKERFILE="$PROJECT_ROOT/docker/Dockerfile.$DISTRO"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE}  CLI Docker Test Environment${NC}"
    echo -e "${BLUE}================================================${NC}"
}

validate_distro() {
    echo -e "${YELLOW}📦 Validating distro: $DISTRO${NC}"

    if [[ ! -f "$DOCKERFILE" ]]; then
        echo -e "${RED}❌ No Dockerfile found for distro: $DISTRO${NC}"
        echo -e "${YELLOW}Available Dockerfiles:${NC}"
        ls "$PROJECT_ROOT/docker/Dockerfile."* | sed "s|$PROJECT_ROOT/docker/Dockerfile.||g"
        exit 1
    fi

    echo -e "${GREEN}✅ Dockerfile for $DISTRO is valid: $DOCKERFILE${NC}"
}

build_cli() {
    echo -e "${YELLOW}🔨 Building CLI binary...${NC}"

    cd "$PROJECT_ROOT"
    task build

    if [[ ! -f "$CLI_BINARY" ]]; then
        echo -e "${RED}❌ CLI build failed: $CLI_BINARY not found!${NC}"
        exit 1
    fi

    echo -e "${GREEN}✅ CLI build complete!${NC}"
}

prepare_assets() {
    echo -e "${YELLOW}📁 Preparing assets directory...${NC}"

    if [[ ! -d "$PROJECT_ROOT/assets" ]]; then
        echo -e "${YELLOW}⚠️  Warning: assets directory not found, creating empty one!${NC}"
        mkdir -p "$PROJECT_ROOT/assets"
    fi

    echo -e "${GREEN}✅ Assets ready!${NC}"
}

build_docker_image() {
    echo -e "${YELLOW}🐳 Building Docker image for $DISTRO...${NC}"

    docker build -f "$DOCKERFILE" -t "$DOCKER_IMAGE:$DISTRO" "$PROJECT_ROOT"

    echo -e "${GREEN}✅ Docker image built: $DOCKER_IMAGE:$DISTRO${NC}"
}

run_cli_in_docker() {
    local args="$*"

    echo -e "${YELLOW}🚀 Running CLI in Docker container (${DISTRO})...${NC}"
    echo -e "${BLUE}Args:${NC} $args"

    docker run -it --rm \
        --user testuser \
        --privileged \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v "$PROJECT_ROOT/assets:/home/testuser/.local/share/devex/assets" \
        -v "$PROJECT_ROOT/bin:/home/testuser/.local/share/devex/bin" \
        -v "$PROJECT_ROOT/config:/home/testuser/.local/share/devex/config" \
        -v "$PROJECT_ROOT/help:/home/testuser/.local/share/devex/help" \
        "$DOCKER_IMAGE:$DISTRO" \
        bash -c "
            echo '=== Debug Info ==='
            ls -la ~/.local/share/devex
            ls -la ~/.local/share/devex/bin
            echo '=== Testing binary execution ==='
            echo 'Architecture info:'
            uname -a
            echo 'Attempting to run devex with full path:'
            /home/testuser/.local/share/devex/bin/devex $args
            EXIT_CODE=\$?
            echo '=== Command completed with exit code:' \$EXIT_CODE '==='
        "
}

interactive_shell() {
    echo -e "${YELLOW}🔧 Starting interactive shell environment in Docker (${DISTRO})...${NC}"

    docker run -it --rm \
        --user testuser \
        --privileged \
        -v /var/run/docker.sock:/var/run/docker.sock \
        -v "$PROJECT_ROOT/assets:/home/testuser/.local/share/devex/assets" \
        -v "$PROJECT_ROOT/bin:/home/testuser/.local/share/devex/bin" \
        -v "$PROJECT_ROOT/config:/home/testuser/.local/share/devex/config" \
        -v "$PROJECT_ROOT/help:/home/testuser/.local/share/devex/help" \
        "$DOCKER_IMAGE:$DISTRO" \
        bash
}

inspect_logs() {
    echo -e "${YELLOW}📋 Inspecting logs in Docker container (${DISTRO})...${NC}"

    docker run -it --rm \
        --user testuser \
        --privileged \
        -v /var/run/docker.sock:/var/run/docker.sock \
        "$DOCKER_IMAGE:$DISTRO" \
        bash -c "find /home/testuser/.local/share/devex -name '*.log' -exec cat {} \;"
}

clean_up() {
    echo -e "${YELLOW}🧹 Cleaning up Docker environment...${NC}"

    docker system prune -f
    echo -e "${GREEN}✅ Cleanup complete!${NC}"
}

show_usage() {
    echo ""
    echo "CLI Docker Test Environment"
    echo ""
    echo "Usage:"
    echo "  $0 [COMMAND] [ARGS]"
    echo ""
    echo "Commands:"
    echo "  build               - Build the CLI binary and Docker image"
    echo "  run [ARGS]          - Run the CLI with optional arguments"
    echo "  shell               - Start an interactive shell in the container"
    echo "  logs                - Inspect logs within the container"
    echo "  setup               - Run the 'devex setup' command"
    echo "  setup-verbose       - Run 'devex setup --verbose'"
    echo "  setup-auto          - Run 'devex setup --non-interactive'"
    echo "  clean               - Clean up Docker containers and volumes"
    echo ""
    echo "Options:"
    echo "  --distro DISTRO     - Specify the target Linux distribution (default: $DEFAULT_DISTRO)"
    echo ""
    echo "Examples:"
    echo "  $0 build --distro ubuntu        # Build for Ubuntu"
    echo "  $0 run --distro suse --help     # Test on SUSE"
    echo "  $0 setup --distro debian        # Run setup on Debian"
    echo "  $0 logs --distro ubuntu         # Inspect logs for Ubuntu"
    echo "  $0 clean                        # Cleanup environment"
}

# Main script logic
COMMAND="${1:-}"
shift # Shift arguments to process further options

while [[ $# -gt 0 ]]; do
    case "$1" in
        --distro)
            DISTRO="$2"
            DOCKERFILE="$PROJECT_ROOT/docker/Dockerfile.$DISTRO"
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

case "${COMMAND}" in
    build)
        print_header
        validate_distro
        build_cli
        prepare_assets
        build_docker_image
        ;;
    run)
        validate_distro
        shift
        run_cli_in_docker "$@"
        ;;
    shell)
        validate_distro
        interactive_shell
        ;;
    logs)
        validate_distro
        inspect_logs
        ;;
    setup)
        validate_distro
        run_cli_in_docker "setup"
        ;;
    setup-verbose)
        validate_distro
        run_cli_in_docker "setup --verbose"
        ;;
    setup-auto)
        validate_distro
        run_cli_in_docker "setup --non-interactive"
        ;;
    clean)
        print_header
        clean_up
        ;;
    help|--help|-h)
        print_header
        show_usage
        ;;
    *)
        print_header
        echo -e "${RED}❌ Unknown command: $COMMAND${NC}"
        show_usage
        exit 1
        ;;
esac
