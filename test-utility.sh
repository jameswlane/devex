source ./gum-utility.sh

# Test choose function
CARD=$(choose --height 15 red blue green yellow black white)
echo "Your color $CARD?"
choose --limit 5 red blue green yellow black white
choose --no-limit --header "Grocery Shopping" red blue green yellow black white

# Test the log function
# Debug Message
log debug "This is a debug message."
# 10 Jul 24 15:44 CDT DEBUG This is a debug message.

# Info Message
log info "This is an info message."
# 10 Jul 24 15:44 CDT INFO This is an info message.

# Warn Message
log warn "This is a warning message."
# 10 Jul 24 15:44 CDT WARN This is a warning message.

# Error Message
log error "This is an error message."
# 10 Jul 24 15:44 CDT ERROR This is an error message.

# Fatal Message
log fatal "This is a fatal message."
# 10 Jul 24 15:44 CDT FATAL This is a fatal message.
