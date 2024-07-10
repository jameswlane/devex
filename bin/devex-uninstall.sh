UNINSTALLER=$(gum file $DEVEX_PATH/uninstall)
[ -n "$UNINSTALLER" ] && gum confirm "Run uninstaller?" && source $UNINSTALLER
clear
source $DEVEX_PATH/bin/devex.sh
