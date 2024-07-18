# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Install lazydocker using Homebrew from the jesseduffield/lazydocker tap
  brew install jesseduffield/lazydocker/lazydocker
  # Install lazydocker using Homebrew (alternative command)
  brew install lazydocker
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Change to the /tmp directory
  cd /tmp
  # Fetch the latest version number of lazydocker from GitHub API
  LAZYDOCKER_VERSION=$(curl -s "https://api.github.com/repos/jesseduffield/lazydocker/releases/latest" | grep -Po '"tag_name": "v\K[^"]*')
  # Download the latest lazydocker tarball for Linux
  curl -sLo lazydocker.tar.gz "https://github.com/jesseduffield/lazydocker/releases/latest/download/lazydocker_${LAZYDOCKER_VERSION}_Linux_x86_64.tar.gz"
  # Extract the lazydocker binary from the tarball
  tar -xf lazydocker.tar.gz lazydocker
  # Install the lazydocker binary to /usr/local/bin
  sudo install lazydocker /usr/local/bin
  # Remove the downloaded tarball and extracted binary
  rm lazydocker.tar.gz lazydocker
  # Change back to the previous directory
  cd -
fi
