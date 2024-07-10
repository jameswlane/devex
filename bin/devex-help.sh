HELP=$(gum choose "Hotkeys" "Commands" "Tactile" --header "What do you need help with?" --height 5 | tr '[:upper:]' '[:lower:]')
[ -n "$HELP" ] && gum pager --soft-wrap <$DEVEX_PATH/help/$HELP.md
source $DEVEX_PATH/bin/devex-menu.sh
