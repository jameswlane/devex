cat <<EOF >~/.local/share/applications/DevEx.desktop
[Desktop Entry]
Version=1.0
Name=DevEx
Comment=DevEx Controls
Exec=alacritty --config-file /home/$USER/.local/share/devex/defaults/alacritty-pane.toml -e devex
Terminal=false
Type=Application
Icon=/home/$USER/.local/share/devex/applications/icons/DevEx.png
Categories=GTK;
StartupNotify=true
EOF
