# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Tap the localsend/localsend repository using Homebrew
  brew tap localsend/localsend
  # Install LocalSend using Homebrew
  brew install localsend
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Change to the /tmp directory
  cd /tmp
  # Fetch the latest version number of LocalSend from GitHub API
  LOCALSEND_VERSION=$(curl -s "https://api.github.com/repos/localsend/localsend/releases/latest" | grep -Po '"tag_name": "v\K[^"]*')
  # Download the latest LocalSend .deb package for Linux
  wget -O localsend.deb "https://github.com/localsend/localsend/releases/latest/download/LocalSend-${LOCALSEND_VERSION}-linux-x86-64.deb"
  # Install the LocalSend .deb package
  sudo apt install -y ./localsend.deb
  # Remove the downloaded .deb package
  rm localsend.deb
  # Change back to the previous directory
  cd -
fi
