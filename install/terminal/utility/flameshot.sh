# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Install flameshot using Homebrew
  brew install flameshot
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Install flameshot using apt package manager
  sudo apt install -y flameshot
fi
