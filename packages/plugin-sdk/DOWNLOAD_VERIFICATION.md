# Download verification

Debian/Ubuntu direct downloads (`curlpipe`, `deb`, and `appimage`) accept
integrity metadata on each application's OS configuration, including overrides
and alternatives. Flat legacy application definitions use the same field:

```yaml
linux:
  install_method: curlpipe
  download_url: https://vendor.example/install.sh
  verification:
    policy: required
    sha256: <64 hexadecimal characters>
    signature_url: https://vendor.example/install.sh.asc
    public_key: /home/user/.config/devex/trusted/vendor.asc
    fingerprint: <expected signing key fingerprint>
```

`policy` accepts `optional` (the default) or `required`. Required needs at least
a SHA256 digest or a detached signature URL. Every supplied digest and signature
must verify, even with optional policy. Missing required metadata, malformed
metadata, a mismatched digest, an untrusted signer, and an unavailable signature
fail installation. SHA256 accepts a bare hex digest or a `sha256:` prefix.
A signature requires an explicitly configured **local** public key; the verifier
does not fetch trust keys from the artifact's origin. The optional fingerprint
pins a signer within that trust key. Set both digest and signature to require both.
A signature may be ASCII armored or binary OpenPGP.

Plugin argv equivalents are `--verification-policy`, `--sha256`,
`--signature-url`, `--public-key`, and `--signer-fingerprint`, each followed by
its value. Supply one artifact per metadata set (AppImage also needs its binary
name). For AppImage, keep `install_command: "<URL> <binary_name>"` or supply the
two identifiers through `packages`; its existing install location flags remain
supported. No shell interpolation is performed for metadata values.

Verification happens on downloaded bytes before bash, DEB inspection/dependency
installation, or publication of an AppImage executable. Failed AppImage updates
preserve the existing binary. Optional absent metadata leaves the checks skipped,
not verified; diagnostics explicitly distinguish checksum and signature outcomes
and are written to stderr. Digest/signature checks do not replace or grant script
execution consent: the setup and unattended consent rules from #271 still apply.
APT repository downloads retain APT's own repository signature policy; these
per-artifact pins apply to direct download methods, not repository package names.

Both CLI plugin validators now use SDK registry payload verification and compare
the extracted executable with the installed bytes, without executing the plugin
to establish integrity. Archive hashes describe the archive, not the extracted
executable, so verification downloads a temporary reference payload. This needs
registry/network access and checks the current registry release; an older
installed release must be updated to match. `plugin.public-key` configures the
trusted key for plugin payload signatures (Viper flag/environment/config hierarchy
applies). The default checksum verification path remains enabled. Explicit
`--allow-insecure-plugins` retains its existing bypass behavior, but failed or
bypassed checks are never marked verified. Signature-enabled SDK downloads
reverify the payload even when an executable and matching install record exist.

Other platform-specific direct download managers (RPM, DMG, PKG, Windows
installers) are deferred; this change does not introduce their metadata contracts.
