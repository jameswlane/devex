INSTALLER=$(gum file $DEVEX_PATH/install)
[ -n "$INSTALLER" ] && gum confirm "Run installer?" && source $INSTALLER
clear
source $DEVEX_PATH/bin/devex.sh
