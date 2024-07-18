# Checks if the operating system is macOS (Darwin).
if [[ "$OSTYPE" =~ ^darwin ]]; then
  # Sets the iTerm2 version to be downloaded.
  ITERM2_VERSION="3_5_3"
  # Constructs the download URL for the specified iTerm2 version.
  ITERM2_URL="https://iterm2.com/downloads/stable/iTerm2-$ITERM2_VERSION.zip"
  # Downloads the iTerm2 zip file silently to the Downloads folder.
  curl -sLo $HOME/Downloads/iTerm2-$ITER2_VERSION.zip $ITERM2_URL
  # Unzips the iTerm2 file quietly to the Applications folder, installing the application.
  unzip -q $HOME/Downloads/iTerm2-$ITER2_VERSION.zip -d /Applications/
  # Removes the downloaded zip file to clean up.
  rm $HOME/Downloads/iTerm2-$ITER2_VERSION.zip
fi
