if [ $# -eq 0 ]; then
	SUB=$(gum choose "Desktop" "Terminal" "Quit" --height 10 --header "" | tr '[:upper:]' '[:lower:]')
else
	SUB=$1
fi

[ -n "$SUB" ] && [ "$SUB" != "quit" ] && source $DEVEX_PATH/install/$SUB-menu.sh
