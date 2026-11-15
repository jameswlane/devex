#!/bin/sh
set -eu

#region logging setup
if [ "${MISE_DEBUG-}" = "true" ] || [ "${MISE_DEBUG-}" = "1" ]; then
  debug() {
    echo "$@" >&2
  }
else
  debug() {
    :
  }
fi

if [ "${MISE_QUIET-}" = "1" ] || [ "${MISE_QUIET-}" = "true" ]; then
  info() {
    :
  }
else
  info() {
    echo "$@" >&2
  }
fi

warn() {
  printf '%s\n' "$*" >&2
}

error() {
  echo "$@" >&2
  exit 1
}

unsupported_arch() {
  arch="$1"
  warn "unsupported architecture: $arch"
  warn ""
  warn "mise does not provide prebuilt binaries for this platform."
  warn "If Rust/Cargo is available, install from source with:"
  warn "  cargo install --locked mise"
  exit 1
}
#endregion

#region environment setup
get_os() {
  os="$(uname -s)"
  if [ "$os" = Darwin ]; then
    echo "macos"
  elif [ "$os" = Linux ]; then
    echo "linux"
  else
    error "unsupported OS: $os"
  fi
}

get_arch() {
  musl=""
  if type ldd >/dev/null 2>/dev/null; then
    if [ "${MISE_INSTALL_MUSL-}" = "1" ] || [ "${MISE_INSTALL_MUSL-}" = "true" ]; then
      musl="-musl"
    elif [ "$(uname -o)" = "Android" ]; then
      # Android (Termux) always uses musl
      musl="-musl"
    else
      libc=$(ldd /bin/ls | grep 'musl' | head -1 | cut -d ' ' -f1)
      if [ -n "$libc" ]; then
        musl="-musl"
      fi
    fi
  fi
  arch="$(uname -m)"
  if [ "$arch" = x86_64 ]; then
    echo "x64$musl"
  elif [ "$arch" = aarch64 ] || [ "$arch" = arm64 ]; then
    echo "arm64$musl"
  elif [ "$arch" = armv7l ]; then
    echo "armv7$musl"
  else
    unsupported_arch "$arch"
  fi
}

get_ext() {
  if [ -n "${MISE_INSTALL_EXT:-}" ]; then
    echo "$MISE_INSTALL_EXT"
  elif [ -n "${MISE_VERSION:-}" ] && echo "$MISE_VERSION" | grep -q '^v2024'; then
    # 2024 versions don't have zstd tarballs
    echo "tar.gz"
  elif tar_supports_zstd; then
    echo "tar.zst"
  else
    echo "tar.gz"
  fi
}

tar_supports_zstd() {
  if ! command -v zstd >/dev/null 2>&1; then
    false
  # tar is bsdtar
  elif tar --version | grep -q 'bsdtar'; then
    true
  # busybox tar reports a "1.3x" version that matches the GNU check below, but it
  # cannot decompress .tar.zst itself. Detect it so we fall back to the zstd pipe
  # (or a .tar.gz download) instead of running `tar -xf` on a zstd tarball.
  elif tar --version 2>&1 | grep -qi 'busybox'; then
    false
  # tar version is >= 1.31
  elif tar --version | grep -q '1\.\(3[1-9]\|[4-9][0-9]\)'; then
    true
  else
    false
  fi
}

shasum_bin() {
  if command -v shasum >/dev/null 2>&1; then
    echo "shasum"
  elif command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
  else
    error "mise install requires shasum or sha256sum but neither is installed. Aborting."
  fi
}

github_release_url() {
  echo "https://github.com/jdx/mise/releases/download/v$1"
}

mirror_release_url() {
  echo "https://mise.jdx.dev/v$1"
}

# Prints the remote file at $1 to stdout. Returns non-zero on any failure.
fetch_text() {
  url="$1"
  if command -v curl >/dev/null 2>&1; then
    debug ">" curl --compressed -fsSL "$url"
    curl --compressed -fsSL "$url"
  elif command -v wget >/dev/null 2>&1; then
    debug ">" wget -qO - "$url"
    wget -qO - "$url"
  else
    error "mise standalone install specific version requires curl or wget but neither is installed. Aborting."
  fi
}

# Reads SHASUMS256.txt lines on stdin and prints the first well-formed SHA-256
# line for exactly file $1 (listed as "$1" or "./$1"). Prints nothing if
# there is none.
select_checksum() {
  awk -v f="$1" '
    NF == 2 && length($1) == 64 && $1 ~ /^[0-9a-f]+$/ {
      name = $2
      sub(/^\*/, "", name)
      if (name == f || name == "./" f) { print $1 "  ./" f; exit }
    }
  '
}

# Prints the line for asset $2 from SHASUMS256.txt in the release directory at
# $1. Prints nothing if the file cannot be fetched or has no valid line for it.
fetch_checksum() {
  # TODO: verify with minisign or gpg if available
  fetch_text "$1/SHASUMS256.txt" | select_checksum "$2" || true
}

get_checksum() {
  version=$1
  os=$2
  arch=$3
  ext=$4
  current_version="v2026.11.8"
  current_version="${current_version#v}"

  # For current version use static checksum otherwise
  # use checksum from releases
  if [ "$version" = "$current_version" ]; then
    checksum_linux_x86_64="d2d5a46f507bba772686e2c8f2316b3a4439dac49a37a30db5c0e7bb077c5431  ./mise-v2026.11.8-linux-x64.tar.gz"
    checksum_linux_x86_64_musl="459ac6f5c1581b44987347af09ddcb518ef255631a098e3ec0430982555eb138  ./mise-v2026.11.8-linux-x64-musl.tar.gz"
    checksum_linux_arm64="b36da3c4892380325d44dee3678ba618de191254d8a4aed8011ee66b28a6cbd7  ./mise-v2026.11.8-linux-arm64.tar.gz"
    checksum_linux_arm64_musl="8b3b9d3508a4fc1bee5f90b0bea1926f5656f40f58e40374616daeaaf8d3587c  ./mise-v2026.11.8-linux-arm64-musl.tar.gz"
    checksum_linux_armv7="5664ca0af99b07dc571266e920cfae85e2be02675a1000e0884862e4c1837c60  ./mise-v2026.11.8-linux-armv7.tar.gz"
    checksum_linux_armv7_musl="17aeaaafd2d26e67f4b34dbe6616edc6f3fb0733b4fc8a25aa2cb98bec00c020  ./mise-v2026.11.8-linux-armv7-musl.tar.gz"
    checksum_macos_x86_64="355729a12f3d3b23059ad372855a82f5c8720d1d3fb63dbe5068bc9b4952201c  ./mise-v2026.11.8-macos-x64.tar.gz"
    checksum_macos_arm64="7a7f9342559cc4ca670054337ecf497e69d832a05f66e531628381dc92b88f48  ./mise-v2026.11.8-macos-arm64.tar.gz"
    checksum_linux_x86_64_zstd="3276ae3726b71cc6a438c8c4b5b2b2d6dbc00952239996db145eaf748ce95c55  ./mise-v2026.11.8-linux-x64.tar.zst"
    checksum_linux_x86_64_musl_zstd="22699a4c3c7bb4242488b66686b1749a5584a974dae5f4daf1182ae392d7a99a  ./mise-v2026.11.8-linux-x64-musl.tar.zst"
    checksum_linux_arm64_zstd="3e5092a49a2a56200e770e28a8225d5f212f302b0c2141421ccb9c86ce5793aa  ./mise-v2026.11.8-linux-arm64.tar.zst"
    checksum_linux_arm64_musl_zstd="ef9f18d0cae6ec0b0483d4af01c182bc0bfc3b2b4e24140e26300427133583b9  ./mise-v2026.11.8-linux-arm64-musl.tar.zst"
    checksum_linux_armv7_zstd="f5b475980599d7d0882f867983497f42c231b0d436690c6d629db9a8f027eae5  ./mise-v2026.11.8-linux-armv7.tar.zst"
    checksum_linux_armv7_musl_zstd="1ee5fc1866bb17db4b6a718f6148f1c76864fa4769eff9985aab0653960f8b7b  ./mise-v2026.11.8-linux-armv7-musl.tar.zst"
    checksum_macos_x86_64_zstd="244c50552cf35507cffc15384de14f4c77cb3c9f40c6124cc964005dadb9aa75  ./mise-v2026.11.8-macos-x64.tar.zst"
    checksum_macos_arm64_zstd="8fd751c185d0681bb3f60d2606e4f5ddb34d67fe1d0273e4ca2affa678b95385  ./mise-v2026.11.8-macos-arm64.tar.zst"

    # TODO: refactor this, it's a bit messy
    if [ "$ext" = "tar.zst" ]; then
      if [ "$os" = "linux" ]; then
        if [ "$arch" = "x64" ]; then
          echo "$checksum_linux_x86_64_zstd"
        elif [ "$arch" = "x64-musl" ]; then
          echo "$checksum_linux_x86_64_musl_zstd"
        elif [ "$arch" = "arm64" ]; then
          echo "$checksum_linux_arm64_zstd"
        elif [ "$arch" = "arm64-musl" ]; then
          echo "$checksum_linux_arm64_musl_zstd"
        elif [ "$arch" = "armv7" ]; then
          echo "$checksum_linux_armv7_zstd"
        elif [ "$arch" = "armv7-musl" ]; then
          echo "$checksum_linux_armv7_musl_zstd"
        else
          warn "no checksum for $os-$arch"
        fi
      elif [ "$os" = "macos" ]; then
        if [ "$arch" = "x64" ]; then
          echo "$checksum_macos_x86_64_zstd"
        elif [ "$arch" = "arm64" ]; then
          echo "$checksum_macos_arm64_zstd"
        else
          warn "no checksum for $os-$arch"
        fi
      else
        warn "no checksum for $os-$arch"
      fi
    else
      if [ "$os" = "linux" ]; then
        if [ "$arch" = "x64" ]; then
          echo "$checksum_linux_x86_64"
        elif [ "$arch" = "x64-musl" ]; then
          echo "$checksum_linux_x86_64_musl"
        elif [ "$arch" = "arm64" ]; then
          echo "$checksum_linux_arm64"
        elif [ "$arch" = "arm64-musl" ]; then
          echo "$checksum_linux_arm64_musl"
        elif [ "$arch" = "armv7" ]; then
          echo "$checksum_linux_armv7"
        elif [ "$arch" = "armv7-musl" ]; then
          echo "$checksum_linux_armv7_musl"
        else
          warn "no checksum for $os-$arch"
        fi
      elif [ "$os" = "macos" ]; then
        if [ "$arch" = "x64" ]; then
          echo "$checksum_macos_x86_64"
        elif [ "$arch" = "arm64" ]; then
          echo "$checksum_macos_arm64"
        else
          warn "no checksum for $os-$arch"
        fi
      else
        warn "no checksum for $os-$arch"
      fi
    fi
  else
    # Always verify against GitHub Releases, never the host the archive came
    # from, so a compromised mise.jdx.dev cannot serve a matching archive and
    # checksum. There is deliberately no fallback to mise.jdx.dev here.
    checksum="$(fetch_checksum "$(github_release_url "$version")" "mise-v${version}-${os}-${arch}.${ext}")"
    if [ -z "$checksum" ]; then
      warn "no checksum for mise $version and $os-$arch"
    else
      echo "$checksum"
    fi
  fi
}

#endregion

# Downloads $1 into directory $2 and prints the path of the downloaded file.
# Returns non-zero if the download fails.
download_file() {
  url="$1"
  download_dir="$2"
  filename="$(basename "$url")"
  file="$download_dir/$filename"

  if command -v curl >/dev/null 2>&1; then
    debug ">" curl -#fLo "$file" "$url"
    curl -#fLo "$file" "$url" >&2 || return 1
  else
    if command -v wget >/dev/null 2>&1; then
      debug ">" wget -qO "$file" "$url"
      stderr=$(mktemp)
      if ! wget -O "$file" "$url" >"$stderr" 2>&1; then
        warn "wget failed: $(cat "$stderr")"
        rm -f "$stderr"
        return 1
      fi
      rm "$stderr"
    else
      error "mise standalone install requires curl or wget but neither is installed. Aborting."
    fi
  fi

  echo "$file"
}

# Prints the version of an installed mise binary (the first field of
# `mise version`, e.g. "2025.6.0"), with any leading "v" stripped. Prints
# nothing if the binary is missing or fails to report a version.
installed_mise_version() {
  bin="$1"
  if [ -x "$bin" ]; then
    installed_version="$("$bin" version 2>/dev/null | head -n1 | cut -d' ' -f1)"
    echo "${installed_version#v}"
  fi
}

# Keep bootstrap dependencies to POSIX sh/awk and curl or wget. The native CLI
# additionally supports calendar durations and absolute cutoff dates.
release_age_cutoff() {
  awk -v age="$1" -v now="$(date +%s)" 'BEGIN {
    if (age !~ /^[0-9]+[smhdw]$/) exit 1
    unit = substr(age, length(age)); value = substr(age, 1, length(age)-1)
    seconds = (unit == "s" ? 1 : unit == "m" ? 60 : unit == "h" ? 3600 : unit == "d" ? 86400 : 604800)
    printf "%.0f\n", now - value * seconds
  }'
}

select_release() {
  awk -v cutoff="$1" '
    NF != 2 || $1 !~ /^v[0-9]+\.[0-9]+\.[0-9]+$/ || $2 !~ /^[0-9]+$/ { invalid = 1; next }
    $2 <= cutoff {
      split(substr($1, 2), v, ".")
      if (!selected || v[1]+0 > year || (v[1]+0 == year && v[2]+0 > month) || (v[1]+0 == year && v[2]+0 == month && v[3]+0 > patch)) {
        selected = substr($1, 2); year = v[1]+0; month = v[2]+0; patch = v[3]+0
      }
    }
    END { if (invalid || !selected) exit 1; print selected }
  '
}

resolve_release() {
  if [ -n "${MISE_VERSION:-}" ]; then
    printf '%s\n' "${MISE_VERSION#v}"
    return
  fi
  age="${MISE_SELF_UPDATE_MINIMUM_RELEASE_AGE-${MISE_MINIMUM_RELEASE_AGE-24h}}"
  cutoff=$(release_age_cutoff "$age") || error "invalid minimum release age: $age (use an integer with s, m, h, d, or w)"
  if command -v curl >/dev/null 2>&1; then
    index=$(curl -fsSL https://mise.jdx.dev/releases.tsv) || error "could not fetch mise release index"
  elif command -v wget >/dev/null 2>&1; then
    index=$(wget -qO- https://mise.jdx.dev/releases.tsv) || error "could not fetch mise release index"
  else
    error "mise install requires curl or wget"
  fi
  selected=$(printf '%s\n' "$index" | select_release "$cutoff") || error "no eligible mise release or invalid release index; lower MISE_SELF_UPDATE_MINIMUM_RELEASE_AGE or set MISE_VERSION"
  info "mise: selected $selected (minimum release age: $age)"
  printf '%s\n' "$selected"
}

install_mise() {
  version="$(resolve_release)"
  version="${version#v}"
  current_version="v2026.11.8"
  current_version="${current_version#v}"
  os="${MISE_INSTALL_OS:-$(get_os)}"
  arch="${MISE_INSTALL_ARCH:-$(get_arch)}"
  ext="${MISE_INSTALL_EXT:-$(get_ext)}"
  install_path="${MISE_INSTALL_PATH:-$HOME/.local/bin/mise}"
  install_dir="$(dirname "$install_path")"
  install_from_github="${MISE_INSTALL_FROM_GITHUB:-}"

  # Keep a newer manually installed release when resolving an unpinned install.
  if [ -z "${MISE_VERSION:-}" ] && [ -x "$install_path" ]; then
    existing_version="$(installed_mise_version "$install_path")"
    [ -n "$existing_version" ] || error "cannot compare installed mise version; set MISE_VERSION explicitly"
    if [ -n "$existing_version" ]; then
      compare_version="${existing_version%-DEBUG}"
      newest=$(printf 'v%s 0\nv%s 0\n' "$compare_version" "$version" | select_release 0) || error "cannot compare installed mise version; set MISE_VERSION explicitly"
      if [ "$newest" = "$compare_version" ]; then
        info "mise: $install_path is already at $existing_version, keeping installed version"
        return 0
      fi
    fi
  fi

  # Opt-in: skip the download/install if the binary already at the install
  # path matches the requested version. Only the install path is checked (not
  # the wider PATH) so that skipping never leaves install_path missing.
  skip_if_exists="${MISE_INSTALL_SKIP_IF_EXISTS-}"
  if [ "$skip_if_exists" = "1" ] || [ "$skip_if_exists" = "true" ]; then
    if [ -x "$install_path" ]; then
      existing_version="$(installed_mise_version "$install_path")"
      if [ -n "$existing_version" ] && [ "$existing_version" = "$version" ]; then
        info "mise: $install_path is already at version $version, skipping install"
        return 0
      fi
    fi
  fi
  # Download from mise.jdx.dev first and fall back to GitHub Releases when that
  # fails, for example because mise.jdx.dev no longer hosts an older release.
  # The checksum never comes from mise.jdx.dev: it is embedded in this script
  # for the built-in version and read from GitHub Releases for any other.
  asset="mise-v${version}-${os}-${arch}.${ext}"
  github_url="$(github_release_url "$version")/$asset"
  if [ "$install_from_github" = "1" ] || [ "$install_from_github" = "true" ]; then
    tarball_url="$github_url"
    fallback_url=""
  elif [ "$version" = "$current_version" ] && [ -n "${MISE_TARBALL_URL-}" ]; then
    tarball_url="$MISE_TARBALL_URL"
    fallback_url=""
  else
    tarball_url="$(mirror_release_url "$version")/$asset"
    fallback_url="$github_url"
  fi

  info "mise: installing mise..."
  download_dir="$(mktemp -d)"
  if ! cache_file=$(download_file "$tarball_url" "$download_dir"); then
    [ -n "$fallback_url" ] || error "mise: failed to download $tarball_url"
    info "mise: could not download from mise.jdx.dev, trying GitHub Releases"
    cache_file=$(download_file "$fallback_url" "$download_dir") || error "mise: failed to download $fallback_url"
  fi
  debug "mise-setup: tarball=$cache_file"

  debug "validating checksum"
  # Never extract an archive without a checksum for exactly that file: an empty
  # or unrelated line must not reach `shasum -c`, whose handling of it varies.
  cache_name="$(basename "$cache_file")"
  checksum="$(get_checksum "$version" "$os" "$arch" "$ext" | select_checksum "$cache_name")"
  [ -n "$checksum" ] || error "mise: no checksum found for $cache_name, refusing to install it unverified"
  cd "$(dirname "$cache_file")"
  printf '%s\n' "$checksum" | "$(shasum_bin)" -c >/dev/null || error "mise: checksum mismatch for $cache_name"

  # extract tarball
  if [ -d "$install_path" ]; then
    error "MISE_INSTALL_PATH '$install_path' is a directory. Please set it to a file path, e.g. '$install_path/mise'."
  fi
  mkdir -p "$install_dir"
  rm -f "$install_path"
  extract_dir="$(mktemp -d)"
  cd "$extract_dir"
  if [ "$ext" = "tar.zst" ] && ! tar_supports_zstd; then
    zstd -d -c "$cache_file" | tar --no-same-owner -xf -
  else
    tar --no-same-owner -xf "$cache_file"
  fi
  if [ "$(id -u)" = "0" ]; then
    chown 0:0 mise/bin/mise
    chmod 755 mise/bin/mise
  fi
  mv mise/bin/mise "$install_path"

  # cleanup
  cd / # Move out of $extract_dir before removing it
  rm -rf "$download_dir"
  rm -rf "$extract_dir"

  info "mise: installed successfully to $install_path"
}

after_finish_help() {
  case "${SHELL:-}" in
  */zsh)
    info "mise: run the following to activate mise in your shell:"
    info "echo \"eval \\\"\\\$($install_path activate zsh)\\\"\" >> \"${ZDOTDIR-$HOME}/.zshrc\""
    info ""
    info "mise: run \`mise doctor\` to verify this is set up correctly"
    ;;
  */bash)
    info "mise: run the following to activate mise in your shell:"
    info "echo \"eval \\\"\\\$($install_path activate bash)\\\"\" >> ~/.bashrc"
    info ""
    info "mise: run \`mise doctor\` to verify this is set up correctly"
    ;;
  */fish)
    info "mise: run the following to activate mise in your shell:"
    info "echo \"$install_path activate fish | source\" >> ~/.config/fish/config.fish"
    info ""
    info "mise: run \`mise doctor\` to verify this is set up correctly"
    ;;
  *)
    info "mise: run \`$install_path --help\` to get started"
    ;;
  esac
}

install_mise
if [ "${MISE_INSTALL_HELP-}" != 0 ]; then
  after_finish_help
fi
