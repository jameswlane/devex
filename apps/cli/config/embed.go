package config

import "embed"

// Files contains the canonical configuration tree written by devex init.
//
//go:embed applications environments system desktop dotfiles.yaml security.yaml
var Files embed.FS

// SkillPacks contains curated third-party agent skill pack catalog definitions.
//
//go:embed skill-packs
var SkillPacks embed.FS
