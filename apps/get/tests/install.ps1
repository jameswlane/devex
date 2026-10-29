# Run with pwsh -NoProfile -File apps/get/tests/install.ps1 (no Pester dependency).
$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot '../public/install.ps1'
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw ($parseErrors | Out-String) }
# Import only definitions so tests cannot trigger a real installation.
foreach ($definition in $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false)) {
    Invoke-Expression $definition.Extent.Text
}
function Assert($Condition, $Message) { if (-not $Condition) { throw $Message } }
function Assert-Throws([scriptblock]$Action) {
    $threw = $false
    try { & $Action | Out-Null } catch { $threw = $true }
    Assert $threw 'Expected an exception'
}
$temp = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $env:LOCALAPPDATA = $temp
    $env:TEMP = $temp
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    $env:PROCESSOR_ARCHITEW6432 = ''
    Assert ((Get-SystemInfo) -ceq 'amd64') 'AMD64 detection'
    $env:PROCESSOR_ARCHITEW6432 = 'ARM64'
    Assert ((Get-SystemInfo) -ceq 'arm64') 'WOW64 detection'
    $env:PROCESSOR_ARCHITEW6432 = ''
    $env:PROCESSOR_ARCHITECTURE = 'x86'
    Assert-Throws { Get-SystemInfo }
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'

    function Invoke-RestMethod {
        @(
            @{ tag_name = 'packages/plugin/v99.0.0'; draft = $false; prerelease = $false },
            @{ tag_name = 'v2.0.0'; draft = $false; prerelease = $true },
            @{ tag_name = 'v1.2.3'; draft = $false; prerelease = $false }
        )
    }
    $latest = @(Get-LatestVersion)
    Assert ($latest.Count -eq 1 -and $latest[0] -ceq 'v1.2.3') 'Latest returns only one CLI tag'
    $script:uris = @()
    function Invoke-WebRequest {
        param($Uri, $OutFile, [switch]$UseBasicParsing)
        $script:uris += $Uri
        Set-Content -LiteralPath $OutFile -Value 'archive'
    }
    $download = @(Get-DevExDownload -Version 'v1.2.3' -Architecture 'amd64' -TempDir $temp -BaseUrl 'http://localhost:1234/')
    Assert ($download.Count -eq 1 -and $download[0] -is [hashtable]) 'Download returns one hashtable'
    $download = $download[0]
    Assert ($script:uris[0] -ceq 'http://localhost:1234/v1.2.3/devex_1.2.3_windows_amd64.tar.gz') 'Release URL'
    $hash = (Get-FileHash $download.TarballPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content $download.ChecksumPath "$hash  $($download.FileName)"
    $result = @(Test-Checksum $download)
    Assert ($result.Count -eq 1 -and $result[0] -eq $true) 'Checksum returns one boolean'
    foreach ($line in @(
        "$hash  $($download.FileName).other",
        "sha256:$hash  $($download.FileName)",
        "$('0' * 64)  $($download.FileName)",
        "$hash $($download.FileName)",
        "$hash  $($download.FileName)`n$hash  $($download.FileName)"
    )) {
        Set-Content $download.ChecksumPath $line
        Assert-Throws { Test-Checksum $download }
    }
    # On Linux, exercise extraction and argument forwarding with a local fake executable.
    if ([Environment]::OSVersion.Platform -eq [PlatformID]::Unix) {
        $savedPath = $env:PATH
        try {
            $env:PATH = 'C:\DevEx-other;C:\Another'
            Add-ToPath -BinPath 'C:\DevEx'
            Add-ToPath -BinPath 'c:\devex\'
            Assert (@($env:PATH -split ';' | Where-Object { $_ -ieq 'C:\DevEx' }).Count -eq 1) 'Exact PATH entry added only once'
        } finally { $env:PATH = $savedPath }
        $fixture = Join-Path $temp 'fixture'
        New-Item -ItemType Directory -Path (Join-Path $fixture 'bin') | Out-Null
        $fakeExe = Join-Path $fixture 'bin/devex.exe'
        Set-Content $fakeExe "#!/bin/sh`nprintf '%s\n' `"`$@`" > `"`$DEVEX_TEST_LOG`"" -NoNewline
        & chmod +x $fakeExe
        $archive = Join-Path $temp 'fixture.tar.gz'
        & tar -czf $archive -C $fixture bin
        Assert ($LASTEXITCODE -eq 0) 'Create fixture archive'
        $env:DEVEX_TEST_ARCHIVE = $archive
        $env:DEVEX_TEST_LOG = Join-Path $temp 'args.txt'
        function Invoke-WebRequest {
            param($Uri, $OutFile, [switch]$UseBasicParsing)
            $script:uris += $Uri
            if ($Uri.EndsWith('devex_checksums.txt')) {
                $hash = (Get-FileHash $env:DEVEX_TEST_ARCHIVE -Algorithm SHA256).Hash
                if ($script:corrupt) { $hash = '0' * 64 }
                Set-Content $OutFile "$hash  devex_1.2.3_windows_amd64.tar.gz"
            } else { Copy-Item $env:DEVEX_TEST_ARCHIVE $OutFile }
        }
        $script:pathCalls = 0
        function Add-ToPath { param($BinPath) $script:pathCalls++ }
        function Read-Host { throw 'Unexpected interactive prompt' }
        $script:corrupt = $false
        $destination = Join-Path $temp 'install with spaces'
        foreach ($version in @('1.2.3', 'v1.2.3')) {
            Install-DevEx -Version $version -NoSetup -InstallDir $destination -BaseUrl 'http://localhost:1234'
            Assert (Test-Path (Join-Path $destination 'bin/devex.exe')) 'Installed executable'
            Assert (-not (Test-Path $env:DEVEX_TEST_LOG)) 'NoSetup never runs setup'
        }
        Install-DevEx -Version '1.2.3' -NonInteractive -Config 'config with spaces.yml' -InstallDir $destination -BaseUrl 'http://localhost:1234'
        $argsPassed = @(Get-Content $env:DEVEX_TEST_LOG)
        Assert (($argsPassed -join '|') -ceq 'install|--non-interactive|--config|config with spaces.yml') 'NonInteractive and Config arguments'
        Remove-Item $env:DEVEX_TEST_LOG
        Install-DevEx -Version '1.2.3' -NoSetup -NonInteractive -InstallDir $destination -BaseUrl 'http://localhost:1234'
        Assert (-not (Test-Path $env:DEVEX_TEST_LOG)) 'NoSetup takes precedence'
        $script:corrupt = $true
        $badDestination = Join-Path $temp 'bad-install'
        Assert-Throws { Install-DevEx -Version '1.2.3' -NoSetup -InstallDir $badDestination -BaseUrl 'http://localhost:1234' }
        Assert (-not (Test-Path $badDestination)) 'Corrupt checksum installs nothing'
        Assert ($script:pathCalls -eq 4) 'Corrupt checksum never changes PATH'
        Assert (@(Get-ChildItem $temp -Filter 'devex-install-*').Count -eq 0) 'Temporary directories cleaned'
    }
    # Fail before extraction or PATH changes; test both file and iex error handling.
    $pwsh = (Get-Process -Id $PID).Path
    & $pwsh -NoProfile -File $scriptPath -BaseUrl 'http://localhost:1' 6>$null
    Assert ($LASTEXITCODE -eq 1) 'Script failure must exit nonzero'
    $original = Get-Content -Raw $scriptPath
    $source = $original.Replace("[string]`$BaseUrl = 'https://github.com/jameswlane/devex/releases/download'", "[string]`$BaseUrl = 'http://localhost:1'")
    Assert ($source -cne $original) 'iex test points BaseUrl at localhost'
    # A child shell, so an installer that calls exit cannot end this test run.
    $output = & $pwsh -NoProfile -Command { param($src) Invoke-Expression $src 6>$null; 'iex-returned' } -args $source
    Assert ($LASTEXITCODE -eq 0) 'iex failure must not exit the shell'
    Assert (@($output) -contains 'iex-returned') 'iex failure returned to the caller'
    Write-Host 'PowerShell installer tests passed.'
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force
}
