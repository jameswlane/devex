if [ $# -eq 0 ]; then
	SUB=$(gum choose "Theme" "Font" "Install" "Uninstall" "Help" "Update" "Quit" --height 10 --header "" | tr '[:upper:]' '[:lower:]')
else
	SUB=$1
fi

[ -n "$SUB" ] && [ "$SUB" != "quit" ] && source $DEVEX_PATH/bin/devex-$SUB.sh
