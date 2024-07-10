set_font() {
  local font_name=$1
  local url=$2
  local file_type=$3
  local file_name="${font_name/ Nerd Font/}"

  if ! fc-list | grep -i "$font_name" > /dev/null; then
    cd /tmp || exit
    wget -O "$file_name.zip" "$url"
    unzip "$file_name.zip" -d "$file_name"
    cp "$file_name"/*."$file_type" ~/.local/share/fonts
    rm -rf "$file_name.zip" "$file_name"
    fc-cache
    cd - || exit
  fi

  gsettings set org.gnome.desktop.interface monospace-font-name "$font_name 10"
  sed -i "s/\"editor.fontFamily\": \".*\"/\"editor.fontFamily\": \"$font_name\"/g" ~/.config/Code/User/settings.json
}

if [ "$#" -gt 1 ]; then
  choice=${!#}
else
  choice=$(gum choose "Cascadia Mono" "Fira Mono" "JetBrains Mono" "Meslo" "<< Back" --height 7 --header "Choose your programming font")
fi

case $choice in
  "Cascadia Mono")
    set_font "CaskaydiaMono Nerd Font" "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/CascadiaMono.zip" "ttf"
    ;;
  "Fira Mono")
    set_font "FiraMono Nerd Font" "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/FiraMono.zip" "otf"
    ;;
  "JetBrains Mono")
    set_font "JetBrainsMono Nerd Font" "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip" "ttf"
    ;;
  "Meslo")
    set_font "MesloLGS Nerd Font" "https://github.com/ryanoasis/nerd-fonts/releases/latest/download/Meslo.zip" "Meslo" "ttf"
    ;;
esac

source $DEVEX_PATH/bin/devex-menu.sh
