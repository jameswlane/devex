#!/usr/bin/env bash
set -e

# Function to check if a string is in a file
string_in_file() {
  grep -qF -- "$1" "$2" || return 1
}

# Function to install Homebrew
install_homebrew() {
  echo "Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  local brew_init_script
  if [[ "$OSTYPE" =~ ^darwin ]]; then
    brew_init_script='eval "$(/opt/homebrew/bin/brew shellenv)"'
  elif [[ "$OSTYPE" =~ ^linux ]]; then
    brew_init_script='eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"'
  fi
  if ! string_in_file "$brew_init_script" ~/.profile; then
    echo "$brew_init_script" >> ~/.profile
    eval "$brew_init_script"
  fi
}

# Function to install Gum
install_gum() {
  if [[ "$OSTYPE" =~ ^darwin ]]; then
    echo "Installing Gum on macOS..."
    brew install gum
  elif [[ "$OSTYPE" =~ ^linux ]]; then
    echo "Installing Gum on Linux..."
    if ! grep -qF "charm.sh/apt" /etc/apt/sources.list.d/charm.list 2>/dev/null; then
      sudo mkdir -p /etc/apt/keyrings
      curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
      echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
      sudo apt update
    fi
    sudo apt install -y gum
  fi
}

# Main script starts here
if ! command -v brew &> /dev/null; then
  install_homebrew
else
  echo "Homebrew already installed."
fi

install_gum

echo "Setup completed successfully."
