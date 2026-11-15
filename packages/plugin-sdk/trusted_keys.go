package sdk

import (
	_ "embed"
	"strings"
)

// miseReleaseKey is mise's release signing key (jdx), exported from
// https://mise.jdx.dev/gpg-key.pub and pinned by MiseReleaseKeyFingerprint.
//
//go:embed trustkeys/mise-release.asc
var miseReleaseKey []byte

const (
	// MiseInstallScriptURL is mise's official installer script.
	MiseInstallScriptURL = "https://mise.run"
	// MiseInstallSignatureURL is the OpenPGP signed copy of that script
	// published with every mise release.
	MiseInstallSignatureURL = "https://github.com/jdx/mise/releases/latest/download/install.sh.sig"
	// MiseReleaseKeyFingerprint is the fingerprint mise documents for its release key.
	MiseReleaseKeyFingerprint = "24853EC9F655CE80B48E6C3A8B81C9D17413A06D"
)

// builtInTrustKeys are vendor release keys compiled into DevEx, selected only
// by an exact pinned signer fingerprint.
var builtInTrustKeys = map[string][]byte{
	MiseReleaseKeyFingerprint: miseReleaseKey,
}

// builtInTrustKey returns the compiled-in key for metadata that pins a known
// fingerprint and names no local key file. A local key always takes precedence.
func builtInTrustKey(metadata ArtifactVerification) []byte {
	if metadata.PublicKeyPath != "" {
		return nil
	}
	return builtInTrustKeys[strings.ToUpper(strings.ReplaceAll(metadata.Fingerprint, " ", ""))]
}

// MiseInstallVerification requires the mise installer script to carry a valid
// signature from mise's pinned release key before DevEx executes it. The
// script then checks the mise binary against the checksums it embeds.
func MiseInstallVerification() ArtifactVerification {
	return ArtifactVerification{
		Policy:       "required",
		SignatureURL: MiseInstallSignatureURL,
		Fingerprint:  MiseReleaseKeyFingerprint,
	}
}
