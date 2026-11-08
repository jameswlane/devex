# Package identifiers and native options

Package-manager definitions can separate identifiers from options:

```yaml
linux:
  install_method: snap
  packages: [nvim]
  options: [--classic, --channel=latest/edge]
```

`packages` is an array of literal identifiers, including versions, URLs, or local
archives. `options` is an array of native argument tokens. Values may use either
`--channel=latest/edge` or two tokens `[--channel, latest/edge]`. Spaces, quotes,
backslashes and shell characters inside structured values remain literal. No
shell expansion occurs on the plugin path. Do not combine `packages` with
`install_command`; options may supplement a legacy `install_command`.

Options are checked against manager-specific allowlists. Snap accepts `--classic`,
`--devmode`, `--jailmode`, `--stable`, `--candidate`, `--beta`, `--edge`, `--channel`
and `--revision`; the last two require nonempty values. Homebrew accepts `--cask`
and `--formula`, and Cargo accepts `--locked`. Legacy `-y` and `--yes` are consumed
without forwarding because plugins own confirmation policy. Unknown options,
missing values, option-shaped package identifiers and empty identifiers fail
before plugin execution. Add options to the allowlist with regression coverage
when adding support for other native options. Docker container configuration
continues to use the existing `docker_options` fields.

## Migration and compatibility

Existing shipped `install_command` definitions continue working, including
`nvim --classic` in neovim.yaml and `ghostty --classic` in ghostty.yaml. No bulk
migration is required. Convert custom definitions to arrays incrementally;
`install_command` remains supported for this release with no removal scheduled.

Legacy strings support quoted and escaped arguments, optional `sudo` and manager
prefixes followed by `install`, `remove`, or `uninstall`, and the existing Flatpak
`flathub` selector. Prefix executables are consumed, never sent as package names.
A mismatched manager prefix is rejected. A lone name such as `pip` or an explicit
structured identifier named `apt` remains a legitimate package. Shell operators
outside quotes are rejected: move shell programs to pre/post installation hooks.
Legacy strings do not perform variable or command expansion. Unknown options that
were previously silently discarded now produce an actionable error.

The plugin wire protocol remains `install [native-options...] [identifiers...]`,
using separate argv elements. No JSON envelope, new plugin command, or SDK release
is required. Released plugins that already forward native options remain
compatible. Snap's native option forwarding landed with #187; deploy that Snap
plugin before this CLI when upgrading installations with an older Snap plugin.
Discovery and removal use identifiers only, retaining Homebrew's cask selector.
The legacy streaming installer shell-quotes each structured argument.
