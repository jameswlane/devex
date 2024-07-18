# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Install lazygit using Homebrew from the jesseduffield/lazygit tap
  brew install jesseduffield/lazygit/lazygit
  # Install lazygit using Homebrew (alternative command)
  brew install lazygit
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Change to the /tmp directory
  cd /tmp
  # Fetch the latest version number of lazygit from GitHub API
  LAZYGIT_VERSION=$(curl -s "https://api.github.com/repos/jesseduffield/lazygit/releases/latest" | grep -Po '"tag_name": "v\K[^"]*')
  # Download the latest lazygit tarball for Linux
  curl -sLo lazygit.tar.gz "https://github.com/jesseduffield/lazygit/releases/latest/download/lazygit_${LAZYGIT_VERSION}_Linux_x86_64.tar.gz"
  # Extract the lazygit binary from the tarball
  tar -xf lazygit.tar.gz lazygit
  # Install the lazygit binary to /usr/local/bin
  sudo install lazygit /usr/local/bin
  # Remove the downloaded tarball and extracted binary
  rm lazygit.tar.gz lazygit
  # Change back to the previous directory
  cd -
fi
