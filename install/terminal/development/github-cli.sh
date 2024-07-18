# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Install GitHub CLI (gh) using Homebrew
  brew install gh
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Download and install the GitHub CLI (gh) GPG key
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg &&
  # Set the appropriate permissions for the GPG key
  sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg &&
  # Add the GitHub CLI repository to the sources list
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null &&
  # Update the package list
  sudo apt update &&
  # Install the GitHub CLI (gh) package
  sudo apt install gh -y
fi
