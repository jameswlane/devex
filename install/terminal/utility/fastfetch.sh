# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Install fastfetch using Homebrew
  brew install fastfetch
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Change to the /tmp directory
  cd /tmp
  # Download the latest fastfetch .deb package for Linux
  curl -sLo fastfetch-linux-amd64.deb "https://github.com/fastfetch-cli/fastfetch/releases/latest/download/fastfetch-linux-amd64.deb"
  # Install the downloaded .deb package using apt
  sudo apt install ./fastfetch-linux-amd64.deb
  # Remove the downloaded .deb package
  rm fastfetch-linux-amd64.deb
  # Change back to the previous directory
  cd -
fi
