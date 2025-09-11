#Requires -Version 5.0
<#
.SYNOPSIS
    Downloads and installs DevEx on Windows.

.DESCRIPTION
    Retrieves the latest DevEx release from GitHub and installs it to the local machine.
    DevEx will handle all dependency installation and environment setup after installation.

.PARAMETER Version
    Specifies a specific version of DevEx to install. By default, installs the latest version.

.PARAMETER InstallDir
    Specifies the installation directory. Defaults to %LOCALAPPDATA%\Programs\DevEx

.EXAMPLE
    irm https://devex.sh/install.ps1 | iex

    Downloads and runs the DevEx installer with default settings.

.EXAMPLE
    .\install.ps1 -Version "0.26.3"

    Installs a specific version of DevEx.
#>
param(
    [Parameter(Mandatory = $false)]
    [string]$Version,
    
    [Parameter(Mandatory = $false)]
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\DevEx"
)

# Set strict mode for better error handling
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-DevExInfo {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Message,
        
        [Parameter(Mandatory = $false)]
        [ConsoleColor]$ForegroundColor = $host.UI.RawUI.ForegroundColor
    )
    
    $backup = $host.UI.RawUI.ForegroundColor
    if ($ForegroundColor -ne $host.UI.RawUI.ForegroundColor) {
        $host.UI.RawUI.ForegroundColor = $ForegroundColor
    }
    Write-Output $Message
    $host.UI.RawUI.ForegroundColor = $backup
}

function Test-Prerequisites {
    # Check PowerShell version
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        Write-DevExInfo "PowerShell 5 or later is required. Please update PowerShell." -ForegroundColor Red
        Write-DevExInfo "Download from: https://microsoft.com/powershell"
        exit 1
    }
    
    # Check TLS 1.2 support
    if ([System.Enum]::GetNames([System.Net.SecurityProtocolType]) -notcontains 'Tls12') {
        Write-DevExInfo "TLS 1.2 is required but not available. Please update .NET Framework." -ForegroundColor Red
        exit 1
    }
    
    # Enable TLS 1.2
    try {
        [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072
    } catch {
        Write-DevExInfo "Failed to enable TLS 1.2: $_" -ForegroundColor Red
        exit 1
    }
}

function Get-SystemInfo {
    $arch = $env:PROCESSOR_ARCHITECTURE
    
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86" {
            # Check if running under WOW64 (32-bit on 64-bit)
            if ($env:PROCESSOR_ARCHITEW6432 -eq "AMD64") {
                return "amd64"
            }
            Write-DevExInfo "32-bit Windows is not supported" -ForegroundColor Red
            exit 1
        }
        default {
            Write-DevExInfo "Unsupported architecture: $arch" -ForegroundColor Red
            exit 1
        }
    }
}

function Get-LatestVersion {
    if ($Version) {
        return $Version
    }
    
    Write-DevExInfo "Fetching latest version..."
    
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/jameswlane/devex/releases/latest" -UseBasicParsing
        $latestVersion = $response.tag_name
        Write-DevExInfo "✓ Latest version: $latestVersion" -ForegroundColor Green
        return $latestVersion
    } catch {
        Write-DevExInfo "Failed to fetch latest version: $_" -ForegroundColor Red
        exit 1
    }
}

function Get-DevExDownload {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Version,
        
        [Parameter(Mandatory = $true)]
        [string]$Architecture,
        
        [Parameter(Mandatory = $true)]
        [string]$TempDir
    )
    
    $fileName = "devex_${Version}_windows_${Architecture}.tar.gz"
    $checksumFile = "devex_checksums.txt"
    $baseUrl = "https://github.com/jameswlane/devex/releases/download/$Version"
    
    Write-DevExInfo "Downloading $fileName..."
    
    try {
        # Download tarball
        $tarballPath = Join-Path $TempDir $fileName
        Invoke-WebRequest -Uri "$baseUrl/$fileName" -OutFile $tarballPath -UseBasicParsing
        
        # Download checksums
        $checksumPath = Join-Path $TempDir $checksumFile
        Invoke-WebRequest -Uri "$baseUrl/$checksumFile" -OutFile $checksumPath -UseBasicParsing
        
        return @{
            TarballPath = $tarballPath
            ChecksumPath = $checksumPath
            FileName = $fileName
        }
    } catch {
        Write-DevExInfo "Download failed: $_" -ForegroundColor Red
        exit 1
    }
}

function Test-Checksum {
    param(
        [Parameter(Mandatory = $true)]
        [hashtable]$Download
    )
    
    Write-DevExInfo "Verifying checksum..."
    
    # Read checksum file
    $checksums = Get-Content $Download.ChecksumPath
    $expectedLine = $checksums | Where-Object { $_ -match $Download.FileName }
    
    if (-not $expectedLine) {
        Write-DevExInfo "Checksum not found for $($Download.FileName)" -ForegroundColor Red
        return $false
    }
    
    # Extract expected hash (format: "filename sha256:hash")
    if ($expectedLine -match "sha256:([a-f0-9]{64})") {
        $expectedHash = $matches[1].ToUpper()
    } else {
        Write-DevExInfo "Invalid checksum format" -ForegroundColor Red
        return $false
    }
    
    # Calculate actual hash
    $actualHash = (Get-FileHash -Path $Download.TarballPath -Algorithm SHA256).Hash
    
    if ($actualHash -eq $expectedHash) {
        Write-DevExInfo "✓ Checksum verified" -ForegroundColor Green
        return $true
    } else {
        Write-DevExInfo "Checksum verification failed!" -ForegroundColor Red
        Write-DevExInfo "Expected: $expectedHash"
        Write-DevExInfo "Actual:   $actualHash"
        return $false
    }
}

function Expand-Tarball {
    param(
        [Parameter(Mandatory = $true)]
        [string]$TarballPath,
        
        [Parameter(Mandatory = $true)]
        [string]$Destination
    )
    
    Write-DevExInfo "Installing DevEx..."
    
    # Create destination directory
    if (-not (Test-Path $Destination)) {
        New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    }
    
    # Extract using tar (available in Windows 10+)
    try {
        # Check if tar is available
        $tarCmd = Get-Command tar -ErrorAction SilentlyContinue
        if ($tarCmd) {
            # Use built-in tar
            & tar -xzf $TarballPath -C $Destination
        } else {
            # Fall back to .NET compression (requires intermediate steps)
            Write-DevExInfo "Using .NET compression libraries..."
            
            # First extract .gz
            $tarPath = $TarballPath -replace '\.gz$', ''
            $gzStream = [System.IO.File]::OpenRead($TarballPath)
            $gzipStream = New-Object System.IO.Compression.GzipStream($gzStream, [System.IO.Compression.CompressionMode]::Decompress)
            $tarStream = [System.IO.File]::Create($tarPath)
            $gzipStream.CopyTo($tarStream)
            $tarStream.Close()
            $gzipStream.Close()
            $gzStream.Close()
            
            # Then handle .tar extraction manually
            Write-DevExInfo "Note: Manual tar extraction not fully implemented. Please ensure tar.exe is available." -ForegroundColor Yellow
            throw "tar.exe not found"
        }
        
        Write-DevExInfo "✓ DevEx installed to $Destination" -ForegroundColor Green
    } catch {
        Write-DevExInfo "Extraction failed: $_" -ForegroundColor Red
        exit 1
    }
}

function Add-ToPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BinPath
    )
    
    Write-DevExInfo ""
    Write-DevExInfo "Adding DevEx to PATH..."
    
    # Get current user PATH
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    
    # Check if already in PATH
    if ($userPath -notlike "*$BinPath*") {
        # Add to user PATH
        $newPath = "$userPath;$BinPath"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        
        # Update current session
        $env:PATH = "$env:PATH;$BinPath"
        
        Write-DevExInfo "✓ Added to PATH" -ForegroundColor Green
    } else {
        Write-DevExInfo "✓ Already in PATH" -ForegroundColor Green
    }
}

function Show-CompletionMessage {
    param(
        [Parameter(Mandatory = $true)]
        [string]$BinPath
    )
    
    Write-DevExInfo ""
    Write-DevExInfo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    Write-DevExInfo "✅ DevEx installed successfully!" -ForegroundColor Green
    Write-DevExInfo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    Write-DevExInfo ""
    Write-DevExInfo "To get started, run:"
    Write-DevExInfo ""
    Write-DevExInfo "  devex setup" -ForegroundColor Yellow
    Write-DevExInfo ""
    Write-DevExInfo "This will guide you through setting up your"
    Write-DevExInfo "development environment with your preferred tools."
    Write-DevExInfo ""
    
    # Check if devex is accessible
    $devexPath = Join-Path $BinPath "devex.exe"
    if (Test-Path $devexPath) {
        Write-DevExInfo "Press Enter to start the setup, or Ctrl+C to exit..."
        Read-Host
        
        try {
            & $devexPath setup
        } catch {
            Write-DevExInfo "Note: You may need to restart your PowerShell session" -ForegroundColor Yellow
            Write-DevExInfo "Then run: devex setup"
        }
    } else {
        Write-DevExInfo "Note: You may need to restart your PowerShell session"
        Write-DevExInfo "Then run: devex setup"
    }
}

# Main installation flow
function Install-DevEx {
    Write-DevExInfo "🚀 DevEx Installer" -ForegroundColor Cyan
    Write-DevExInfo "==================" -ForegroundColor Cyan
    Write-DevExInfo ""
    
    # Check prerequisites
    Test-Prerequisites
    
    # Get system information
    $arch = Get-SystemInfo
    
    # Get version to install
    $version = Get-LatestVersion
    
    # Create temp directory
    $tempDir = Join-Path $env:TEMP "devex-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    try {
        # Download DevEx
        $download = Get-DevExDownload -Version $version -Architecture $arch -TempDir $tempDir
        
        # Verify checksum
        if (-not (Test-Checksum -Download $download)) {
            exit 1
        }
        
        # Extract to installation directory
        Expand-Tarball -TarballPath $download.TarballPath -Destination $InstallDir
        
        # Add to PATH
        $binPath = Join-Path $InstallDir "bin"
        Add-ToPath -BinPath $binPath
        
        # Show completion message
        Show-CompletionMessage -BinPath $binPath
        
    } finally {
        # Cleanup temp files
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

# Run the installer
Install-DevEx