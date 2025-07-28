#!/bin/bash

# DevEx Docker Testing Script
# This script provides easy testing of DevEx in isolated containers

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE} DevEx Docker Testing Environment${NC}"
    echo -e "${BLUE}================================================${NC}"
}

print_usage() {
    echo "Usage: $0 [COMMAND] [OPTIONS]"
    echo ""
    echo "Commands:"
    echo "  build [ubuntu|debian]    Build test container(s)"
    echo "  shell [ubuntu|debian]    Start interactive shell in container"
    echo "  test [ubuntu|debian]     Run automated tests"
    echo "  clean                    Clean up containers and volumes"
    echo "  logs [ubuntu|debian]     Show container logs"
    echo ""
    echo "Options:"
    echo "  --config CONFIG_FILE    Use specific test config file"
    echo ""
    echo "Examples:"
    echo "  $0 build ubuntu         # Build Ubuntu test container"
    echo "  $0 shell ubuntu         # Start shell in Ubuntu container"
    echo "  $0 test ubuntu          # Test in Ubuntu container"
}

build_containers() {
    local distro=${1:-"all"}

    echo -e "${YELLOW}Building containers...${NC}"

    cd "$PROJECT_ROOT"

    if [[ "$distro" == "all" || "$distro" == "ubuntu" ]]; then
        echo -e "${BLUE}Building Ubuntu container...${NC}"
        docker-compose -f docker/docker-compose.test.yml build ubuntu-test
    fi

    if [[ "$distro" == "all" || "$distro" == "debian" ]]; then
        echo -e "${BLUE}Building Debian container...${NC}"
        docker-compose -f docker/docker-compose.test.yml build debian-test
    fi

    echo -e "${GREEN}Build complete!${NC}"
}

start_shell() {
    local distro=${1:-"ubuntu"}

    echo -e "${YELLOW}Starting interactive shell in $distro container...${NC}"

    cd "$PROJECT_ROOT"

    case $distro in
        ubuntu)
            docker-compose -f docker/docker-compose.test.yml run --rm ubuntu-test
            ;;
        debian)
            docker-compose -f docker/docker-compose.test.yml run --rm debian-test
            ;;
        *)
            echo -e "${RED}Unknown distro: $distro${NC}"
            echo "Available: ubuntu, debian"
            exit 1
            ;;
    esac
}

run_tests() {
    local distro=${1:-"ubuntu"}
    local config_file=""

    # Parse additional arguments
    shift
    while [[ $# -gt 0 ]]; do
        case $1 in
            --config)
                config_file="$2"
                shift 2
                ;;
            *)
                echo -e "${RED}Unknown option: $1${NC}"
                exit 1
                ;;
        esac
    done

    echo -e "${YELLOW}Running tests in $distro container...${NC}"

    cd "$PROJECT_ROOT"

    local service_name="${distro}-test"

    # Prepare test commands
    local test_commands="
        echo '=== Building DevEx ===' &&
        go build -o bin/devex ./cmd/main.go &&
        echo '=== DevEx Help ===' &&
        ./bin/devex --help &&
        echo '=== DevEx Version ===' &&
        ./bin/devex --version &&
        echo '=== Running List Test ===' &&
        ./bin/devex list available &&
        echo '=== Tests Complete ==='
    "
    "

    # Copy test config if specified
    if [[ -n "$config_file" ]]; then
        test_commands="
            echo '=== Using test config: $config_file ===' &&
            cp test/configs/$config_file ~/.devex/apps.yaml &&
            $test_commands
        "
    fi

    docker-compose -f docker/docker-compose.test.yml run --rm \
        -e "DEVEX_TEST_MODE=true" \
        "$service_name" \
        bash -c "$test_commands"

    if [[ $? -eq 0 ]]; then
        echo -e "${GREEN}Tests passed!${NC}"
    else
        echo -e "${RED}Tests failed!${NC}"
        exit 1
    fi
}

clean_up() {
    echo -e "${YELLOW}Cleaning up containers and volumes...${NC}"

    cd "$PROJECT_ROOT"

    docker-compose -f docker/docker-compose.test.yml down -v
    docker system prune -f

    echo -e "${GREEN}Cleanup complete!${NC}"
}

show_logs() {
    local distro=${1:-"ubuntu"}

    cd "$PROJECT_ROOT"

    local service_name="${distro}-test"
    docker-compose -f docker/docker-compose.test.yml logs "$service_name"
}

# Main script logic
case "${1:-}" in
    build)
        print_header
        build_containers "$2"
        ;;
    shell)
        print_header
        start_shell "$2"
        ;;
    test)
        print_header
        run_tests "$@"
        ;;
    clean)
        print_header
        clean_up
        ;;
    logs)
        print_header
        show_logs "$2"
        ;;
    help|--help|-h)
        print_header
        print_usage
        ;;
    *)
        print_header
        echo -e "${RED}Error: Unknown command '${1:-}'${NC}"
        echo ""
        print_usage
        exit 1
        ;;
esac
