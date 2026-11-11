package assets

import "embed"

// ZshDefaults contains the modular Zsh configuration deployed during opt-in setup.
//
//go:embed zsh/zsh/rc zsh/zsh/shell zsh/zsh/aliases zsh/zsh/init
var ZshDefaults embed.FS
