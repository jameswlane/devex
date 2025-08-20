#!/bin/bash
# Enhanced CLI Docker Test Script
# Comprehensive testing framework for DevEx CLI across multiple distributions

set -euo pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CLI_BINARY="$PROJECT_ROOT/bin/devex"
DOCKER_IMAGE="devex-test-env"
DEFAULT_DISTRO="debian"
DISTRO="${DISTRO:-$DEFAULT_DISTRO}"
DOCKERFILE="$PROJECT_ROOT/docker/Dockerfile.$DISTRO"

# Available distributions
SUPPORTED_DISTROS=(
    "debian"
    "ubuntu" 
    "arch"
    "redhat"
    "suse"
    "alpine"
    "fedora"
    "centos-stream"
    "manjaro"
)

# Test configuration
PARALLEL_TESTS=${PARALLEL_TESTS:-false}
TEST_TIMEOUT=${TEST_TIMEOUT:-300}  # 5 minutes default
TEST_SUITE=${TEST_SUITE:-"all"}
VERBOSE=${VERBOSE:-false}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "${PURPLE}[DEBUG]${NC} $1"
    fi
}

print_header() {
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${CYAN}  DevEx CLI Enhanced Docker Testing Framework${NC}"
    echo -e "${CYAN}================================================================${NC}"
    echo -e "${BLUE}Distribution:${NC} $DISTRO"
    echo -e "${BLUE}Test Suite:${NC} $TEST_SUITE"
    echo -e "${BLUE}Parallel Tests:${NC} $PARALLEL_TESTS"
    echo -e "${CYAN}================================================================${NC}"
}

validate_distro() {
    log_info "Validating distribution: $DISTRO"
    
    if [[ ! " ${SUPPORTED_DISTROS[*]} " =~ " ${DISTRO} " ]]; then
        log_error "Unsupported distribution: $DISTRO"
        echo -e "${YELLOW}Supported distributions:${NC}"
        printf '%s\n' "${SUPPORTED_DISTROS[@]}"
        exit 1
    fi

    if [[ ! -f "$DOCKERFILE" ]]; then
        log_error "No Dockerfile found for distro: $DISTRO"
        echo -e "${YELLOW}Available Dockerfiles:${NC}"
        ls "$PROJECT_ROOT/docker/Dockerfile."* | sed "s|$PROJECT_ROOT/docker/Dockerfile.||g"
        exit 1
    fi

    log_success "Distribution $DISTRO is valid: $DOCKERFILE"
}

build_cli() {
    log_info "Building CLI binary..."
    
    cd "$PROJECT_ROOT"
    if ! task build; then
        log_error "CLI build failed"
        exit 1
    fi

    if [[ ! -f "$CLI_BINARY" ]]; then
        log_error "CLI build failed: $CLI_BINARY not found"
        exit 1
    fi

    log_success "CLI build complete: $CLI_BINARY"
}

prepare_assets() {
    log_info "Preparing assets directory..."
    
    if [[ ! -d "$PROJECT_ROOT/assets" ]]; then
        log_warning "Assets directory not found, creating empty one"
        mkdir -p "$PROJECT_ROOT/assets"
    fi
    
    log_success "Assets ready"
}

build_docker_image() {
    log_info "Building Docker image for $DISTRO..."
    
    docker build -f "$DOCKERFILE" -t "$DOCKER_IMAGE:$DISTRO" "$PROJECT_ROOT"
    
    log_success "Docker image built: $DOCKER_IMAGE:$DISTRO"
}


# CLI Test Functions
test_basic_commands() {
    log_info "Testing basic CLI commands..."
    
    local tests=(
        "--version:Check version command"
        "--help:Check help command"
        "help:Check help subcommand"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

test_configuration_commands() {
    log_info "Testing configuration commands..."
    
    local tests=(
        "config --help:Config help"
        "config validate:Validate configuration"
        "init --help:Init help"
        "init --dry-run:Init dry run"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

test_application_commands() {
    log_info "Testing application management commands..."
    
    local tests=(
        "list --help:List help"
        "list categories:List categories"
        "add --help:Add help"
        "remove --help:Remove help"
        "install --help:Install help"
        "install --dry-run:Install dry run"
        "uninstall --help:Uninstall help"
        "status --help:Status help"
        "status:Check status"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

test_system_commands() {
    log_info "Testing system commands..."
    
    local tests=(
        "system --help:System help"
        "detect --help:Detect help"
        "shell --help:Shell help"
        "cache --help:Cache help"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

test_template_commands() {
    log_info "Testing template commands..."
    
    local tests=(
        "template --help:Template help"
        "template list:List templates"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

test_recovery_commands() {
    log_info "Testing recovery commands..."
    
    local tests=(
        "rollback --help:Rollback help"
        "undo --help:Undo help"
    )
    
    for test in "${tests[@]}"; do
        local cmd="${test%%:*}"
        local desc="${test##*:}"
        
        log_debug "Running: $desc"
        run_cli_test "$cmd" "$desc"
    done
}

run_cli_test() {
    local cmd="$1"
    local desc="$2"
    local timeout="${3:-$TEST_TIMEOUT}"
    
    log_debug "Executing: devex $cmd"
    
    if timeout "$timeout" docker run --rm \
        --user testuser \
        -v "$PROJECT_ROOT/assets:/home/testuser/.local/share/devex/assets" \
        -v "$PROJECT_ROOT/bin:/home/testuser/.local/share/devex/bin" \
        -v "$PROJECT_ROOT/config:/home/testuser/.local/share/devex/config" \
        -v "$PROJECT_ROOT/help:/home/testuser/.local/share/devex/help" \
        "$DOCKER_IMAGE:$DISTRO" \
        bash -c "/home/testuser/.local/share/devex/bin/devex $cmd" >/dev/null 2>&1; then
        log_success "$desc: PASSED"
        return 0
    else
        log_error "$desc: FAILED"
        return 1
    fi
}

run_interactive_test() {
    local cmd="$1"
    
    log_info "Running interactive test: $cmd"
    
    docker run -it --rm \
        --user testuser \
        -v "$PROJECT_ROOT/assets:/home/testuser/.local/share/devex/assets" \
        -v "$PROJECT_ROOT/bin:/home/testuser/.local/share/devex/bin" \
        -v "$PROJECT_ROOT/config:/home/testuser/.local/share/devex/config" \
        -v "$PROJECT_ROOT/help:/home/testuser/.local/share/devex/help" \
        "$DOCKER_IMAGE:$DISTRO" \
        bash -c "/home/testuser/.local/share/devex/bin/devex $cmd"
}

run_test_suite() {
    local suite="$1"
    local failed_tests=0
    
    log_info "Running test suite: $suite"
    
    case "$suite" in
        "basic"|"all")
            test_basic_commands || ((failed_tests++))
            ;&
        "config"|"all")
            test_configuration_commands || ((failed_tests++))
            ;&
        "apps"|"all")
            test_application_commands || ((failed_tests++))
            ;&
        "system"|"all")
            test_system_commands || ((failed_tests++))
            ;&
        "templates"|"all")
            test_template_commands || ((failed_tests++))
            ;&
        "recovery"|"all")
            test_recovery_commands || ((failed_tests++))
            ;;
        *)
            log_error "Unknown test suite: $suite"
            exit 1
            ;;
    esac
    
    if [[ $failed_tests -eq 0 ]]; then
        log_success "All tests in suite '$suite' passed!"
    else
        log_error "$failed_tests test(s) failed in suite '$suite'"
        exit 1
    fi
}

run_parallel_tests() {
    log_info "Running tests in parallel across distributions..."
    
    local pids=()
    local results=()
    
    for distro in "${SUPPORTED_DISTROS[@]}"; do
        (
            export DISTRO="$distro"
            export VERBOSE="false"
            log_info "Starting tests for $distro"
            
            if validate_distro && build_docker_image && run_test_suite "$TEST_SUITE"; then
                echo "$distro:PASSED"
            else
                echo "$distro:FAILED"
            fi
        ) &
        pids+=($!)
    done
    
    # Wait for all tests to complete
    for pid in "${pids[@]}"; do
        wait "$pid"
    done
    
    log_success "Parallel testing completed"
}

interactive_shell() {
    log_info "Starting interactive shell environment..."
    
    docker run -it --rm \
        --user testuser \
        -v "$PROJECT_ROOT/assets:/home/testuser/.local/share/devex/assets" \
        -v "$PROJECT_ROOT/bin:/home/testuser/.local/share/devex/bin" \
        -v "$PROJECT_ROOT/config:/home/testuser/.local/share/devex/config" \
        -v "$PROJECT_ROOT/help:/home/testuser/.local/share/devex/help" \
        "$DOCKER_IMAGE:$DISTRO" \
        bash
}

inspect_logs() {
    log_info "Inspecting container logs..."
    
    docker run --rm \
        --user testuser \
        "$DOCKER_IMAGE:$DISTRO" \
        bash -c "find /home/testuser/.local/share/devex -name '*.log' -exec cat {} \\; 2>/dev/null || echo 'No logs found'"
}

benchmark_tests() {
    log_info "Running performance benchmarks..."
    
    local start_time
    local end_time
    local duration
    
    start_time=$(date +%s)
    run_test_suite "$TEST_SUITE"
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    log_success "Benchmark completed in ${duration}s"
}

clean_up() {
    log_info "Cleaning up Docker environment..."
    
    # Remove test images
    docker images -q "$DOCKER_IMAGE" | xargs -r docker rmi -f >/dev/null 2>&1 || true
    
    # Clean up system
    docker system prune -f >/dev/null 2>&1
    
    log_success "Cleanup complete"
}

show_usage() {
    cat << EOF

DevEx CLI Enhanced Docker Testing Framework

Usage:
  $0 [COMMAND] [OPTIONS]

Commands:
  build                    - Build CLI binary and Docker image
  test [SUITE]            - Run test suite (basic|config|apps|system|templates|recovery|all)
  test-parallel           - Run tests across all distributions in parallel
  test-interactive CMD    - Run interactive test with specific command
  shell                   - Start interactive shell in container
  logs                    - Inspect logs within container
  benchmark              - Run performance benchmarks
  clean                  - Clean up Docker containers and images

Test Suites:
  basic                  - Basic CLI commands (--version, --help)
  config                 - Configuration management commands
  apps                   - Application management commands
  system                 - System and detection commands
  templates              - Template management commands
  recovery               - Recovery and rollback commands
  all                    - Run all test suites

Options:
  --distro DISTRO        - Target distribution (default: $DEFAULT_DISTRO)
  --parallel             - Enable parallel testing
  --timeout SECONDS      - Test timeout in seconds (default: $TEST_TIMEOUT)
  --verbose              - Enable verbose output
  --test-suite SUITE     - Specify test suite to run

Supported Distributions:
$(printf '  %s\n' "${SUPPORTED_DISTROS[@]}")

Examples:
  $0 build --distro ubuntu           # Build for Ubuntu
  $0 test basic --distro alpine      # Run basic tests on Alpine
  $0 test-parallel                   # Test all distros in parallel
  $0 benchmark --distro arch         # Benchmark on Arch Linux
  $0 shell --distro fedora           # Interactive shell on Fedora
  $0 clean                           # Clean up environment

Environment Variables:
  DISTRO                 - Target distribution
  PARALLEL_TESTS         - Enable parallel testing (true/false)
  TEST_TIMEOUT           - Test timeout in seconds
  TEST_SUITE             - Test suite to run
  VERBOSE                - Enable verbose output (true/false)

EOF
}

# Parse command line arguments
COMMAND="${1:-}"
shift || true

while [[ $# -gt 0 ]]; do
    case "$1" in
        --distro)
            DISTRO="$2"
            DOCKERFILE="$PROJECT_ROOT/docker/Dockerfile.$DISTRO"
            shift 2
            ;;
        --parallel)
            PARALLEL_TESTS=true
            shift
            ;;
        --timeout)
            TEST_TIMEOUT="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --test-suite)
            TEST_SUITE="$2"
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

# Main script logic
case "${COMMAND}" in
    build)
        print_header
        validate_distro
        build_cli
        prepare_assets
        build_docker_image
        ;;
    test)
        SUITE="${1:-$TEST_SUITE}"
        validate_distro
        run_test_suite "$SUITE"
        ;;
    test-parallel)
        print_header
        build_cli
        prepare_assets
        run_parallel_tests
        ;;
    test-interactive)
        validate_distro
        CMD="${1:-setup}"
        run_interactive_test "$CMD"
        ;;
    shell)
        validate_distro
        interactive_shell
        ;;
    logs)
        validate_distro
        inspect_logs
        ;;
    benchmark)
        print_header
        validate_distro
        benchmark_tests
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
        log_error "Unknown command: $COMMAND"
        show_usage
        exit 1
        ;;
esac