# Pick a preconfigured theme
THEME_NAMES=("Tokyo Night" "Catppuccin" "Nord" "Everforest" "Gruvbox" "Kanagawa" "Rose Pine")
THEME=$(gum choose "${THEME_NAMES[@]}" "<< Back" --header "Choose your theme" --height 10 | tr '[:upper:]' '[:lower:]' | sed 's/ /-/g')

if [ -n "$THEME" ] && [ "$THEME" != "<<-back" ]; then
  # Install theme in Gnome, Terminal, and both default editors
  # shellcheck disable=SC1090
  source "$DEVEX_PATH/themes/gnome/$THEME.sh"

  if [ ! -f ~/.config/nvim/plugin/after/transparency.lua ]; then
      mkdir -p ~/.config/nvim/plugin/after
      cp "$DEVEX_PATH"/configs/neovim/transparency.lua ~/.config/nvim/plugin/after/transparency.lua
  fi

  # Chrome theme update
  if gum confirm "Do you want to update Chrome theme? If yes, Chrome will be restarted!"; then
    pkill -f chrome

    while pgrep -f chrome > /dev/null; do
        sleep 0.1
    done

    # shellcheck disable=SC1090
    source "$DEVEX_PATH/themes/chrome/$THEME.sh"
    google-chrome > /dev/null 2>&1 &
  fi

  cp "$DEVEX_PATH/themes/neovim/$THEME.lua" ~/.config/nvim/lua/plugins/theme.lua

  # Translate to specific VSC theme name
  if [ "$THEME" == "gruvbox" ]; then
    VSC_THEME="Gruvbox Dark Medium"
    VSC_EXTENSION="jdinhlife.gruvbox"
  elif [ "$THEME" == "catppuccin" ]; then
    VSC_THEME="Catppuccin Macchiato"
    VSC_EXTENSION="Catppuccin.catppuccin-vsc"
  elif [ "$THEME" == "tokyo-night" ]; then
    VSC_THEME="Tokyo Night"
    VSC_EXTENSION="enkia.tokyo-night"
  elif [ "$THEME" == "everforest" ]; then
    VSC_THEME="Everforest Dark"
    VSC_EXTENSION="sainnhe.everforest"
  elif [ "$THEME" == "rose-pine" ]; then
    VSC_THEME="Rosé Pine Dawn"
    VSC_EXTENSION="mvllow.rose-pine"
  elif [ "$THEME" == "nord" ]; then
    VSC_THEME="Nord"
    VSC_EXTENSION="arcticicestudio.nord-visual-studio-code"
  elif [ "$THEME" == "kanagawa" ]; then
    VSC_THEME="Kanagawa"
    VSC_EXTENSION="qufiwefefwoyn.kanagawa"
  fi

  code --install-extension $VSC_EXTENSION > /dev/null
  sed -i "s/\"workbench.colorTheme\": \".*\"/\"workbench.colorTheme\": \"$VSC_THEME\"/g" ~/.config/Code/User/settings.json
fi

source $DEVEX_PATH/bin/devex-menu.sh
