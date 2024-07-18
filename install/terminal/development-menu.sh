# Define an array of optional applications to choose from
OPTIONAL_APPS=("docker" "github-cli" "lazydocker" "lazygit" "mise" "podman" "pre-commit" "dev-env" "set-git" "toolbox" "trufflehog" "vscode")

# Set the default selected optional applications
DEFAULT_OPTIONAL_APPS='docker,mise,github-cli'

# Use gum to present a selection menu for optional apps and export the selected apps
export DEVELOPER_APPS=$(gum choose "${OPTIONAL_APPS[@]}" --no-limit --selected $DEFAULT_OPTIONAL_APPS --height 7 --header "Select optional apps" | tr ' ' '-')

# Check if any optional apps were selected
if [[ -v DEVELOPER_APPS ]]; then
  # Assign the selected apps to a variable
  apps=$DEVELOPER_APPS

  # If there are selected apps, iterate over each app
  if [[ -n "$apps" ]]; then
    for app in $apps; do
      # Source the installation script for each selected app
      source "$DEVEX_PATH/install/development/${app,,}.sh"
    done
  fi
fi

source $DEVEX_PATH/install/install-menu.sh
