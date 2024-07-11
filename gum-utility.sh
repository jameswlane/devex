choose() {
  gum choose $@
}

confirm() {
  gum confirm $@
}

file() {
  gum file $@
}

pager() {
  gum pager $@
}

spinner() {
  local message="$1"
  local command="$2"
  local options="${3:-}" # Default to empty if no third argument is provided

  # Execute the command with a spinner, quoting variables and allowing for additional options
  if ! gum spin --spinner dot --title "$message" $options -- bash -c "$command"; then
    echo "Error: Command failed to execute successfully."
    return 1
  fi
}

log() {
  local level="${1:-info}" # Default level to info if not provided
  local message="$2"
  local time="${3:-rfc822}" # Default time format to rfc822 if not provided

  # Ensure there's a message to log
  if [ -z "$message" ]; then
    echo "Error: No message provided for logging."
    return 1
  fi

  # Construct and execute the gum log command with specified format
  gum log "$message" -s -t "$time" -l "$level"
}
