#Requires -Version 5.1
<#
.SYNOPSIS
Installs DevEx on Windows: irm https://get.devex.sh/install.ps1 | iex
.PARAMETER Version
CLI version, with or without a leading v. Defaults to the latest CLI release.
.PARAMETER NoSetup
Install only, without prompting or running setup.
.PARAMETER NonInteractive
Run devex install --non-interactive without prompting.
.PARAMETER Config
Configuration path passed to setup or install.
.PARAMETER InstallDir
Installation directory. Defaults to %LOCALAPPDATA%\Programs\DevEx.
.PARAMETER BaseUrl
Release download base URL for testing. Requires an explicit Version.
#>
[CmdletBinding()]
param(
    [string]$Version,
    [switch]$NoSetup,
    [switch]$NonInteractive,
    [string]$Config,
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\DevEx",
    [string]$BaseUrl = 'https://github.com/jameswlane/devex/releases/download'
)

function Get-SystemInfo {
    $architecture = $env:PROCESSOR_ARCHITECTURE
    if ($env:PROCESSOR_ARCHITEW6432) { $architecture = $env:PROCESSOR_ARCHITEW6432 }
    switch ($architecture) {
        'AMD64' { return 'amd64' }
        'ARM64' { return 'arm64' }
        default { throw "Unsupported Windows architecture: $architecture" }
    }
}

function Get-LatestVersion {
    Write-Host 'Fetching latest CLI version...'
    for ($page = 1; ; $page++) {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/jameswlane/devex/releases?per_page=100&page=$page" -UseBasicParsing
        # Windows PowerShell 5.1 emits a JSON array as one object; piping enumerates it in every version.
        $releases = @($response | ForEach-Object { $_ })
        foreach ($release in $releases) {
            if ($release.tag_name -cmatch '^v[0-9]+\.[0-9]+\.[0-9]+$' -and -not $release.draft -and -not $release.prerelease) {
                return $release.tag_name
            }
        }
        if ($releases.Count -lt 100) { throw 'No CLI release found.' }
    }
}

function Get-DevExDownload {
    param([string]$Version, [string]$Architecture, [string]$TempDir, [string]$BaseUrl)
    $fileName = "devex_$($Version.Substring(1))_windows_${Architecture}.tar.gz"
    $download = @{
        FileName = $fileName
        TarballPath = Join-Path $TempDir $fileName
        ChecksumPath = Join-Path $TempDir 'devex_checksums.txt'
    }
    $releaseUrl = "$($BaseUrl.TrimEnd('/'))/$Version"
    Write-Host "Downloading $fileName..."
    # Windows PowerShell 5.1's progress bar slows downloads; this local copy ends with the function.
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri "$releaseUrl/$fileName" -OutFile $download.TarballPath -UseBasicParsing | Out-Null
    Invoke-WebRequest -Uri "$releaseUrl/devex_checksums.txt" -OutFile $download.ChecksumPath -UseBasicParsing | Out-Null
    return $download
}

function Test-Checksum {
    param([hashtable]$Download)
    $hashes = @(foreach ($line in Get-Content -LiteralPath $Download.ChecksumPath) {
        if ($line -cmatch '^([a-fA-F0-9]{64})  (.+)$' -and $Matches[2] -ceq $Download.FileName) {
            $Matches[1]
        }
    })
    if ($hashes.Count -ne 1) { throw "Expected exactly one valid checksum for $($Download.FileName)." }
    $actual = (Get-FileHash -LiteralPath $Download.TarballPath -Algorithm SHA256).Hash
    if ($actual -ine $hashes[0]) { throw "Checksum verification failed for $($Download.FileName)." }
    Write-Host 'Checksum verified.'
    return $true
}

function Add-ToPath {
    param([string]$BinPath)
    # Compare complete entries, ignoring Windows path casing and trailing separators.
    foreach ($target in @('User', 'Process')) {
        $pathValue = [Environment]::GetEnvironmentVariable('PATH', $target)
        $entries = @($pathValue -split ';' | Where-Object { $_ })
        $found = @($entries | Where-Object { $_.TrimEnd('\', '/') -ieq $BinPath.TrimEnd('\', '/') })
        if ($found.Count -eq 0) {
            [Environment]::SetEnvironmentVariable('PATH', (($entries + $BinPath) -join ';'), $target)
        }
    }
}

function Install-DevEx {
    param(
        [string]$Version,
        [switch]$NoSetup,
        [switch]$NonInteractive,
        [string]$Config,
        [string]$InstallDir,
        [string]$BaseUrl
    )
    if ($BaseUrl -ne 'https://github.com/jameswlane/devex/releases/download' -and -not $Version) {
        throw '-BaseUrl requires an explicit -Version.'
    }
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    $tag = $Version
    if ($tag) {
        if ($tag -cnotmatch '^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$') { throw "Invalid CLI version: $tag" }
        if (-not $tag.StartsWith('v')) { $tag = "v$tag" }
    } else {
        $tag = Get-LatestVersion
    }
    $architecture = Get-SystemInfo
    # Prefer Windows' bsdtar: GNU tar from Git's Unix tools reads "C:\..." as a remote host.
    $tar = if ($env:SystemRoot) { Join-Path $env:SystemRoot 'System32\tar.exe' }
    if (-not ($tar -and (Test-Path -LiteralPath $tar))) {
        $tar = (Get-Command tar -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
    }
    $tempDir = Join-Path $env:TEMP ("devex-install-" + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $tempDir -ErrorAction Stop | Out-Null
    try {
        $download = Get-DevExDownload -Version $tag -Architecture $architecture -TempDir $tempDir -BaseUrl $BaseUrl
        Test-Checksum -Download $download | Out-Null
        New-Item -ItemType Directory -Path $InstallDir -Force -ErrorAction Stop | Out-Null
        & $tar -xzf $download.TarballPath -C $InstallDir
        if ($LASTEXITCODE -ne 0) { throw "Archive extraction failed (exit $LASTEXITCODE)." }
        $binPath = Join-Path ([IO.Path]::GetFullPath($InstallDir)) 'bin'
        $executable = Join-Path $binPath 'devex.exe'
        if (-not (Test-Path -LiteralPath $executable -PathType Leaf)) { throw 'Archive did not contain bin/devex.exe.' }
        Add-ToPath -BinPath $binPath
        Write-Host "DevEx $tag installed to $InstallDir."
        $arguments = @('setup')
        if ($NonInteractive) { $arguments = @('install', '--non-interactive') }
        if ($Config) { $arguments += @('--config', $Config) }
        if ($NoSetup -or (-not $NonInteractive -and [Console]::IsInputRedirected)) {
            $nextCommand = 'devex setup'
            if ($Config) { $nextCommand += " --config '" + $Config.Replace("'", "''") + "'" }
            Write-Host "Next: $nextCommand"
            return
        }
        if (-not $NonInteractive) {
            $answer = Read-Host 'Start DevEx setup? [y/N]'
            if ($answer -notmatch '^(y|yes)$') { Write-Host 'Next: devex setup'; return }
        }
        & $executable @arguments
        if ($LASTEXITCODE -ne 0) { throw "DevEx failed (exit $LASTEXITCODE)." }
    } finally {
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# A downloaded script executed through iex must never terminate the caller's shell.
$previousErrorActionPreference = $ErrorActionPreference
try {
    $ErrorActionPreference = 'Stop'
    Install-DevEx -Version $Version -NoSetup:$NoSetup -NonInteractive:$NonInteractive -Config $Config -InstallDir $InstallDir -BaseUrl $BaseUrl
    if ($PSCommandPath -and $MyInvocation.InvocationName -ne '.') { $global:LASTEXITCODE = 0 }
} catch {
    Write-Host "DevEx installation failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($PSCommandPath -and $MyInvocation.InvocationName -ne '.') { $global:LASTEXITCODE = 1; exit 1 }
    return
} finally {
    $ErrorActionPreference = $previousErrorActionPreference
}
