# ~/.zprofile
eval "$(mise activate zsh --shims)"

# Vim  Prepend mise shims to PATH
#let $PATH = $HOME . '/.local/share/mise/shims:' . $PATH

# nvim-- Prepend mise shims to PATH
# vim.env.PATH = vim.env.HOME .. "/.local/share/mise/shims:" .. vim.env.PATH

#JetBrains
# ln -s ~/.local/share/mise ~/.asdf

# VS Code
# launch.json
#{
#  "configurations": [
#    {
#      "type": "node",
#      "request": "launch",
#      "name": "Launch Program",
#      "program": "${file}",
#      "args": [],
#      "osx": {
#        "runtimeExecutable": "mise"
#      },
#      "linux": {
#        "runtimeExecutable": "mise"
#      },
#      "runtimeArgs": [
#        "x",
#        "--",
#        "node"
#      ]
#    }
#  ]
#}
