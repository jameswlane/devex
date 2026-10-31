#!/bin/bash
# Run with bash apps/get/tests/install.sh; no test framework or network required.
# Assertions deliberately match literal shell variables in generated config.
# shellcheck disable=SC2016
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
installer="$script_dir/../public/install"
# Import definitions without main's final invocation, as in the PowerShell tests.
# shellcheck source=/dev/null
source <(sed '$d' "$installer")

test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
mkdir -p "$test_dir/fixture/bin" "$test_dir/releases"
printf '#!/bin/sh\nexit 0\n' > "$test_dir/fixture/bin/devex"
for platform in linux darwin; do
    tar -czf "$test_dir/releases/devex_1.2.3_${platform}_amd64.tar.gz" -C "$test_dir/fixture" bin
done
(cd "$test_dir/releases" && for archive in *.tar.gz; do
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$archive"
    else
        shasum -a 256 "$archive"
    fi
done) > "$test_dir/releases/devex_checksums.txt"

fail() { echo "FAIL: $*" >&2; exit 1; }
assert_contains() { [[ "$1" == *"$2"* ]] || fail "Expected output to contain: $2"; }

# Exercise the complete main flow with local release downloads and simulated OS.
fetch() { cp "$test_dir/releases/${1##*/}" "$2"; }
uname() {
    case "$1" in
        -s) echo "$test_os" ;;
        -m) echo x86_64 ;;
        *) fail "Unexpected uname argument: $1" ;;
    esac
}

run_installer() (
    if [ "$current_shell" = unset ]; then
        unset SHELL
    else
        export SHELL="$current_shell"
    fi
    export DEVEX_SHELL="$env_shell"
    main --version 1.2.3 --no-setup --install-dir "$INSTALL_DIR" --base-url https://test.invalid "$@"
)

case_number=0
test_shell() {
    local test_os="$1" current_shell="$2" env_shell="$3" selected="$4" existing="$5"
    shift 5
    case_number=$((case_number + 1))
    export HOME="$test_dir/home-$case_number"
    INSTALL_DIR="$HOME/install with spaces"
    mkdir -p "$HOME"
    local configs=(.bashrc .bash_profile .zshrc .config/fish/config.fish)
    local config expected output first_output
    if [ "$existing" = yes ]; then
        for config in "${configs[@]}"; do
            mkdir -p "$(dirname "$HOME/$config")"
            printf '# Existing %s\n' "$config" > "$HOME/$config"
            cp "$HOME/$config" "$HOME/$(basename "$config").before"
        done
    fi

    first_output=$(run_installer "$@")
    output=$(run_installer "$@")
    [ -x "$INSTALL_DIR/bin/devex" ] || fail "Executable was not installed"
    assert_contains "$output" 'To add DevEx to another shell manually'
    assert_contains "$output" 'export PATH="$PATH"'
    assert_contains "$output" 'set -gx PATH $PATH'
    for config in "${configs[@]}"; do
        expected=no
        case "$selected:$config" in
            bash:.bashrc|zsh:.zshrc|fish:.config/fish/config.fish) expected=yes ;;
            bash:.bash_profile) [ "$test_os" != darwin ] || expected=yes ;;
        esac
        if [ "$expected" = yes ]; then
            [ "$(grep -cF "$INSTALL_DIR/bin" "$HOME/$config" || :)" -eq 1 ] ||
                fail "Expected one PATH entry in $config for $selected on $test_os"
            assert_contains "$first_output" "Adding to $HOME/$config"
            assert_contains "$output" "Already configured: $HOME/$config"
            if [ "$selected" = fish ]; then
                assert_contains "$(cat "$HOME/$config")" 'set -gx PATH $PATH'
            else
                assert_contains "$(cat "$HOME/$config")" 'export PATH='
            fi
        elif [ "$existing" = yes ]; then
            cmp "$HOME/$config" "$HOME/$(basename "$config").before" ||
                fail "Changed another shell's config: $config"
        else
            [ ! -e "$HOME/$config" ] || fail "Created another shell's config: $config"
        fi
    done
    if [ "$existing" = no ] && [ "$selected" != fish ]; then
        [ ! -e "$HOME/.config" ] || fail "Created another shell's config directory"
    fi
    if [ "$selected" = unknown ]; then
        assert_contains "$output" 'no shell config was changed'
        assert_contains "$output" '--shell bash'
    fi
}

for test_os in linux darwin; do
    for existing in yes no; do
        for selected in bash zsh fish; do
            test_shell "$test_os" "/bin/$selected" '' "$selected" "$existing"
        done
        test_shell "$test_os" /bin/unknown '' unknown "$existing"
        test_shell "$test_os" unset '' unknown "$existing"
        test_shell "$test_os" /bin/bash fish fish "$existing"
        test_shell "$test_os" /bin/fish zsh bash "$existing" --shell bash
        test_shell "$test_os" /bin/unknown invalid zsh "$existing" --shell zsh
    done
done

assert_rejected() {
    local output
    if output=$(run_installer "$@" 2>&1); then
        fail "Invalid shell option was accepted: $*"
    fi
    assert_contains "$output" 'Error:'
    [ ! -e "$INSTALL_DIR" ] || fail 'Invalid shell selection installed files'
}
test_os=linux current_shell=/bin/bash env_shell=''
export HOME="$test_dir/invalid-home"
INSTALL_DIR="$HOME/install"
assert_rejected --shell
assert_rejected --shell ''
assert_rejected --shell tcsh
assert_rejected --shell /bin/bash
env_shell=invalid
assert_rejected
help_output=$(main --help)
assert_contains "$help_output" '--shell SHELL'
assert_contains "$help_output" 'DEVEX_SHELL'
echo "Bash installer tests passed ($case_number shell isolation scenarios, plus validation and help)."
