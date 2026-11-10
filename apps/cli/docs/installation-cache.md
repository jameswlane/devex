# Installation cache

DevEx caches direct AppImage, deb, and curlpipe downloads when configuration
pins `verification.sha256`. Cache acquisition requires HTTPS and rejects
localhost and literal private IP targets, including redirects. The key includes the URL and expected SHA256.
Unpinned downloads are never stored. Artifacts expire after 30 days.

Every hit is copied to an installation-owned temporary file and verified against
the configured checksum before use. Plugins verify that copy again. Corrupt or
missing entries are evicted and downloaded again online. Clearing the cache
during installation does not remove a copy already handed to an installer.

The TUI curlpipe content validator caches successful checks for one hour, keyed
by application, content version (URL and digest), and validator revision.
Checksum and signature verification always run; installed-state checks are not
cached because local state can change independently of an application version.

Use `devex --verbose install <app>` to see debug cache-hit logs.
`--no-cache` bypasses artifact and validation reads and writes.
`--offline` requires a warm cache and a pinned checksum for direct downloads,
and fails before downloading on a miss. The two flags cannot be combined.
Detached signature verification currently requires online access, even with a
cached payload; offline signed requests fail clearly. These flags do not make
external package managers or scripts network independent.

Flags follow the normal Viper precedence: command flags, environment
(`DEVEX_NO_CACHE`, `DEVEX_OFFLINE`), config keys `no-cache` and `offline`,
then defaults (both false).

Existing management commands include the new downloads and validation metadata:

```sh
devex cache stats
devex cache list --type download
devex cache list --type metadata
devex cache clear --force
devex cache cleanup --no-tui
```

To reproduce reuse, configure a direct-download application with a fixed release
URL and its publisher-provided SHA256, then use already installed compatible
plugins:

```sh
devex cache clear --force
devex --verbose install <app>           # downloads and verifies
devex --verbose --offline install <app> # verified cache hit
devex --no-cache install <app>          # downloads without using the cache
devex cache clear --force
devex --offline install <app>           # explicit offline cache-miss error
```

An already-installed application can be skipped before acquisition; use an
application whose installation is actually needed when exercising this example.
Offline controls acquisition only; the installer may still execute a script or
package tool which itself needs network access.

The CLI-to-plugin artifact handoff requires updated AppImage, curlpipe, and deb
plugins. Release the SDK and those plugins before releasing this CLI.
