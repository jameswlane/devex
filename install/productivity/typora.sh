# Check if the operating system type is Darwin (macOS)
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Define the URL for the .dmg file
  DMG_URL="https://download.typora.io/mac/Typora.dmg"
  # Create a temporary directory
  TEMP_DIR=$(mktemp -d)
  # Define the path for the downloaded .dmg file
  DMG_FILE="$TEMP_DIR/Typora.dmg"
  # Define the mount point for the .dmg file
  MOUNT_POINT="/Volumes/Typora"
  # Download the .dmg file to the temporary directory
  curl -L -o "$DMG_FILE" "$DMG_URL"
  # Mount the .dmg file
  hdiutil attach "$DMG_FILE" -mountpoint "$MOUNT_POINT"
  # Copy the application to the /Applications directory
  cp -R "$MOUNT_POINT/Typora.app" /Applications/
  # Unmount the .dmg file
  hdiutil detach "$MOUNT_POINT"
  # Remove the temporary directory
  rm -rf "$TEMP_DIR"
fi

# Check if the operating system type is Linux
if [[ "$OSTYPE" =~ ^linux ]]; then
  # Download and add the Typora public key to the trusted keys
  wget -qO - https://typora.io/linux/public-key.asc | sudo tee /etc/apt/trusted.gpg.d/typora.asc
  # Add the Typora repository to the sources list
  sudo add-apt-repository -y 'deb https://typora.io/linux ./'
  # Update the package list
  sudo apt update
  # Install Typora
  sudo apt install -y typora
fi
