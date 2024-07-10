# This script installs Gum, a tool for enhancing the developer experience (DevEx) by providing customizable command-line interfaces.
# Gum is installed differently based on the operating system.

# Checks if the operating system is macOS (Darwin).
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Installs Gum using Homebrew for macOS users.
  brew install gum
fi

# Checks if the operating system is Linux.
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Creates a directory for apt keyrings if it doesn't already exist.
  sudo mkdir -p /etc/apt/keyrings
  # Downloads the GPG key for the Gum repository and adds it to the keyrings directory.
  curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
  # Adds the Gum repository to the list of apt sources.
  echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
  # Updates the apt package list and installs Gum.
  sudo apt update && sudo apt install gum
fi
