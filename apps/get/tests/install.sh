#!/bin/bash
# Run with bash apps/get/tests/install.sh; no test framework or network required.
# Assertions deliberately match literal shell variables in generated config.
# shellcheck disable=SC2016,SC2329
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
installer="$script_dir/../public/install"
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
# Import definitions without main's final invocation, as in the PowerShell tests.
sed '$d' "$installer" > "$test_dir/installer-definitions.sh"
# shellcheck source=/dev/null
source "$test_dir/installer-definitions.sh"

mkdir -p "$test_dir/fixture/bin" "$test_dir/fixture/config" "$test_dir/releases"
printf 'fixture: true\n' > "$test_dir/fixture/config/example.yaml"
printf '#!/bin/sh\nexit 0\n' > "$test_dir/fixture/bin/devex"
for platform in linux darwin; do
    tar -czf "$test_dir/releases/devex_1.2.3_${platform}_amd64.tar.gz" -C "$test_dir/fixture" bin config
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
    export DEVEX_DATA_DIR="$HOME/data"
    INSTALL_DIR="$HOME/install with spaces"
    mkdir -p "$HOME"
    local configs=(.bashrc .bash_profile .profile .zshrc .config/fish/config.fish)
    local config expected output first_output
    if [ "$existing" != no ]; then
        for config in "${configs[@]}"; do
            if [ "$existing" = profile ] && [ "$config" = .bash_profile ]; then
                continue
            fi
            mkdir -p "$(dirname "$HOME/$config")"
            printf '# Existing %s\n' "$config" > "$HOME/$config"
            cp "$HOME/$config" "$HOME/$(basename "$config").before"
        done
    fi

    first_output=$(run_installer "$@")
    output=$(run_installer "$@")
    [ -x "$INSTALL_DIR/devex" ] || fail "Executable was not installed"
    [ -f "$DEVEX_DATA_DIR/config/example.yaml" ] || fail "Bundled config was not installed in the data directory"
    [ ! -d "$INSTALL_DIR/config" ] || fail "Bundled config was mixed with binaries"
    assert_contains "$output" 'To add DevEx to another shell manually'
    assert_contains "$output" 'export PATH="$PATH"'
    assert_contains "$output" "set -gx PATH \$PATH \"$INSTALL_DIR\""
    for config in "${configs[@]}"; do
        expected=no
        case "$selected:$config" in
            bash:.bashrc|zsh:.zshrc|fish:.config/fish/config.fish) expected=yes ;;
            bash:.bash_profile) [ "$test_os" != darwin ] || [ "$existing" = profile ] || expected=yes ;;
            bash:.profile) [ "$test_os" != darwin ] || [ "$existing" != profile ] || expected=yes ;;
        esac
        if [ "$expected" = yes ]; then
            [ "$(grep -cF "# DevEx CLI" "$HOME/$config" || :)" -eq 1 ] ||
                fail "Expected one PATH entry in $config for $selected on $test_os"
            assert_contains "$first_output" "Adding to $HOME/$config"
            assert_contains "$output" "Already configured: $HOME/$config"
            if [ "$selected" = fish ]; then
                assert_contains "$(cat "$HOME/$config")" 'set -gx PATH $PATH'
            else
                assert_contains "$(cat "$HOME/$config")" 'export PATH='
            fi
            if [ "$existing" != no ]; then
                assert_contains "$(cat "$HOME/$config")" "# Existing $config"
            elif [ "$config" = .bash_profile ]; then
                assert_contains "$(cat "$HOME/$config")" '[ -f "$HOME/.profile" ] && . "$HOME/.profile"'
                assert_contains "$(cat "$HOME/$config")" '[ -f "$HOME/.bashrc" ] && . "$HOME/.bashrc"'
            fi
        elif [ -f "$HOME/$(basename "$config").before" ]; then
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

# A macOS bash login must keep reading an existing .profile when .bash_profile is absent.
test_shell darwin /bin/bash '' bash profile

# Control characters make printf %q emit Bash-only quoting; fish must keep double quotes.
test_os=linux current_shell=/bin/fish env_shell=''
export HOME="$test_dir/fish-manual-home"
export DEVEX_DATA_DIR="$HOME/data"
INSTALL_DIR="$HOME/"$'install\twith-tab'
fish_output=$(run_installer)
fish_path_line=$(tail -n 1 "$HOME/.config/fish/config.fish")
assert_contains "$fish_output" "    $fish_path_line"

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
# Latest version: the redirect wins without an API call; nested plugin tags and
# redirect failures fall back to the release listing.
api_calls=0
fetch_github_api() {
    api_calls=$((api_calls + 1))
    printf '%s\n' '[{"tag_name": "packages/tool-git/v0.2.0"}, {"tag_name": "v4.5.6"}]'
}
latest_release_redirect_tag() { echo v7.8.9; }
[ "$(latest_cli_version)" = v7.8.9 ] || fail 'redirect tag was not used'
[ "$api_calls" -eq 0 ] || fail 'release listing was queried despite a redirect tag'
latest_release_redirect_tag() { echo packages/plugin-sdk/v0.3.0; }
[ "$(latest_cli_version)" = v4.5.6 ] || fail 'nested plugin tag was accepted as the CLI version'
latest_release_redirect_tag() { return 1; }
[ "$(latest_cli_version)" = v4.5.6 ] || fail 'redirect failure did not fall back to the release listing'
fetch_github_api() { return 22; }
[ -z "$(latest_cli_version || true)" ] || fail 'a version was reported when every lookup failed'

# API requests send a token only when one is set (fresh definitions, real fetch_github_api).
token_request() {
    bash -c 'source "$1"; curl() { printf "%s\n" "$*"; }; fetch_github_api https://api.test' _ "$test_dir/installer-definitions.sh"
}
[[ "$(GITHUB_TOKEN='' GH_TOKEN='' token_request)" != *Authorization* ]] || fail 'Authorization header sent without a token'
assert_contains "$(GITHUB_TOKEN=secret token_request)" 'Authorization: Bearer secret'
assert_contains "$(GITHUB_TOKEN='' GH_TOKEN=other token_request)" 'Authorization: Bearer other'

help_output=$(main --help)
assert_contains "$help_output" '--shell SHELL'
assert_contains "$help_output" 'DEVEX_SHELL'
# An existing PATH entry must leave every shell rc file untouched.
test_os=linux current_shell=/bin/bash env_shell=''
export HOME="$test_dir/on-path-home"
export DEVEX_DATA_DIR="$HOME/data"
INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$HOME"
PATH="$PATH:$INSTALL_DIR" on_path_output=$(PATH="$PATH:$INSTALL_DIR" run_installer)
assert_contains "$on_path_output" 'Already on PATH:'
[ ! -e "$HOME/.bashrc" ] || fail 'Wrote shell config for a directory already on PATH'
[ -x "$INSTALL_DIR/devex" ] || fail 'Default local bin executable missing'
# Upgrades remove the old generated PATH entry in the selected shell only,
# including when the new directory is already on the process PATH.
for selected in bash zsh fish; do
    export HOME="$test_dir/legacy-$selected"
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$HOME/.config/fish"
    printf 'export PATH="$PATH:%s/.local/share/devex/bin"\n# keep bash\n' "$HOME" > "$HOME/.bashrc"
    cp "$HOME/.bashrc" "$HOME/.zshrc"
    printf 'set -gx PATH $PATH "%s/.local/share/devex/bin"\n# keep fish\n' "$HOME" > "$HOME/.config/fish/config.fish"
    case "$selected" in
        bash) selected_rc="$HOME/.bashrc"; other_rc="$HOME/.zshrc" ;;
        zsh) selected_rc="$HOME/.zshrc"; other_rc="$HOME/.bashrc" ;;
        fish) selected_rc="$HOME/.config/fish/config.fish"; other_rc="$HOME/.bashrc" ;;
    esac
    cp "$other_rc" "$HOME/other.before"
    PATH="$PATH:$INSTALL_DIR" configure_path linux "$selected" >/dev/null
    if grep -qF '.local/share/devex/bin' "$selected_rc"; then fail 'Legacy PATH entry remains'; fi
    assert_contains "$(cat "$selected_rc")" '# keep'
    cmp "$other_rc" "$HOME/other.before" || fail 'Changed an unselected shell during upgrade'
done
echo "Bash installer tests passed ($case_number shell isolation scenarios, plus validation and help)."
