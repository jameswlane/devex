# DevEx Get - Installation Scripts

Minimal static file server for DevEx installation scripts.

## Purpose

Serves installation scripts with OS detection:
- **get.devex.sh/install** - Auto-detects OS and serves appropriate script
- **get.devex.sh/install.ps1** - Direct access to Windows PowerShell script

## OS Detection

The `/install` endpoint uses User-Agent detection:
- Windows User-Agent → Redirects to `/install.ps1`
- All other OS → Serves bash script for Linux/macOS

## Files

- `public/install` - Linux/macOS bash installation script
- `public/install.ps1` - Windows PowerShell installation script
- `vercel.json` - Vercel configuration with redirects and headers

## Deployment

This app is deployed to `get.devex.sh` subdomain via Vercel.

## Development

```bash
npm run dev
```

Opens local server at http://localhost:3000 serving static files.

## Installer Tests

Run the offline installer tests (no framework dependency):

```bash
bash apps/get/tests/install.sh
pwsh -NoProfile -File apps/get/tests/install.ps1
```

Run these commands from the repository root. CI runs the Bash tests on Linux and
macOS and the PowerShell tests on Windows via `.github/workflows/installers.yml`.
See the [installation guide](../docs/content/docs/installation.mdx#shell-path-configuration)
for `--shell`, `DEVEX_SHELL`, and manual PATH configuration.
