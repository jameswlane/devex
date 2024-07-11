#!/bin/bash
set -e

# Define the base directory where scripts are organized by category
base_dir="$HOME/.local/share/devex/install"

# Step 1: List all category directories
categories=($(ls -d $base_dir/*/ | xargs -n 1 basename))

# Step 2: Use `gum` to present these categories to the user for selection
selected_categories=$(gum choose --no-limit "${categories[@]}")

# Convert selected categories from string to array
IFS=$'\n' selected_categories=($selected_categories)

# Step 3 & 4: For each selected category, source all scripts within
for category in "${selected_categories[@]}"; do
  echo "Installing $category category..."
  for script in $base_dir/$category/*.sh; do
    if gum confirm "Do you want to run $(basename "$script")?"; then
      source "$script"
    fi
  done
done
