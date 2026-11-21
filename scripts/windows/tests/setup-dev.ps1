#Requires -Version 5.1
# Run with:
#   powershell -NoProfile -File scripts/windows/tests/setup-dev.ps1
#   pwsh -NoProfile -File scripts/windows/tests/setup-dev.ps1
$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot '..\setup-dev.ps1'
$tokens = $null
$parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) {
    throw ($parseErrors | Out-String)
}

# Import only function definitions so tests never execute the script entry point.
foreach ($definition in $ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] }, $false)) {
    Invoke-Expression $definition.Extent.Text
}

function Assert($Condition, $Message) {
    if (-not $Condition) {
        throw "Assertion failed: $Message"
    }
}

$tempDir = Join-Path ([IO.Path]::GetTempPath()) ('devex-setup-dev-test-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempDir | Out-Null

# Shared mock state reset before every test case.
$script:state = @{}
$script:calls = @{}
$script:messages = @()

function Reset-Mocks {
    $script:state = @{
        Commands       = @{ winget = $true; git = $true; pwsh = $true; wt = $true; mise = $true }
        IsAdmin        = $false
        Registry       = @{
            'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock\AllowDevelopmentWithoutDevLicense' = 1
            'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled'                               = 1
        }
        RepoRoot       = $tempDir
        GitConfig      = @{
            'core.longpaths' = 'true'
            'core.symlinks'  = 'true'
            'core.autocrlf'  = 'true'
        }
        HasGitAttrs    = $true
        MiseExitCode   = 0
        MiseMissing    = @()
        MiseOutput     = ''
        WingetExitCode = 0
        GitSetExitCode = 0
        MiseFixCode    = 0
    }
    $script:calls = @{
        Winget   = New-Object System.Collections.ArrayList
        Registry = New-Object System.Collections.ArrayList
        GitSet   = New-Object System.Collections.ArrayList
        MiseFix  = New-Object System.Collections.ArrayList
    }
    $script:messages = New-Object System.Collections.ArrayList
}

function Write-SetupMessage {
    param([string]$Message)
    [void]$script:messages.Add($Message)
}

function Test-CommandExists {
    param([string]$Name)
    return [bool]$script:state.Commands[$Name]
}

function Test-IsAdministrator {
    return [bool]$script:state.IsAdmin
}

function Get-RegistryValue {
    param([string]$Path, [string]$Name)
    $key = "$Path\$Name"
    if ($script:state.Registry.ContainsKey($key)) {
        return $script:state.Registry[$key]
    }
    return $null
}

function Set-RegistryDword {
    param([string]$Path, [string]$Name, [int]$Value)
    [void]$script:calls.Registry.Add([pscustomobject]@{ Path = $Path; Name = $Name; Value = $Value })
    $script:state.Registry["$Path\$Name"] = $Value
}

function Invoke-WingetInstall {
    param([string]$Id)
    [void]$script:calls.Winget.Add($Id)
    if ($script:state.WingetExitCode -eq 0) {
        switch ($Id) {
            'Git.Git'                   { $script:state.Commands['git'] = $true }
            'Microsoft.PowerShell'      { $script:state.Commands['pwsh'] = $true }
            'Microsoft.WindowsTerminal' { $script:state.Commands['wt'] = $true }
            'jdx.mise'                  { $script:state.Commands['mise'] = $true }
        }
    }
    return $script:state.WingetExitCode
}

function Get-GitRepoRoot {
    if (-not (Test-CommandExists -Name 'git')) {
        return $null
    }
    return $script:state.RepoRoot
}

function Get-GitConfigValue {
    param([string]$RepoRoot, [string]$Key)
    if ($script:state.GitConfig.ContainsKey($Key)) {
        return $script:state.GitConfig[$Key]
    }
    return $null
}

function Set-GitConfigLocal {
    param([string]$RepoRoot, [string]$Key, [string]$Value)
    [void]$script:calls.GitSet.Add([pscustomobject]@{ RepoRoot = $RepoRoot; Key = $Key; Value = $Value })
    if ($script:state.GitSetExitCode -eq 0) {
        $script:state.GitConfig[$Key] = $Value
    }
    return $script:state.GitSetExitCode
}

function Test-RepoFile {
    param([string]$Path)
    if ($Path.EndsWith('.gitattributes')) {
        return [bool]$script:state.HasGitAttrs
    }
    return $false
}

function Get-MiseMissingTools {
    param([string]$RepoRoot)
    return [pscustomobject]@{
        ExitCode = $script:state.MiseExitCode
        Missing  = @($script:state.MiseMissing)
        Output   = $script:state.MiseOutput
    }
}

function Invoke-MiseTrustAndInstall {
    param([string]$RepoRoot)
    [void]$script:calls.MiseFix.Add($RepoRoot)
    if ($script:state.MiseFixCode -eq 0) {
        $script:state.MiseExitCode = 0
        $script:state.MiseMissing = @()
    }
    return $script:state.MiseFixCode
}

try {
    # 1. All checks OK (with .gitattributes handling core.autocrlf) -> exit 0
    Reset-Mocks
    $checks = Get-SetupChecks
    Assert ($checks.Count -eq 11) "Expected 11 checks inside repo, got $($checks.Count)"
    foreach ($c in $checks) {
        Assert ($c.Status -eq 'OK') "Expected $($c.Name) to be OK, got $($c.Status)"
    }
    $autocrlfCheck = $checks | Where-Object { $_.Name -eq 'git core.autocrlf' }
    Assert ($autocrlfCheck.Detail -like '*.gitattributes*') 'core.autocrlf OK detail mentions .gitattributes'
    $exitCode = Invoke-SetupDev
    Assert ($exitCode -eq 0) "All-OK run must exit 0, got $exitCode"
    Assert ($script:messages.Count -eq 11) 'Expected one output line per check'

    # 2. All non-repo checks MISSING / WARN -> exit 1
    Reset-Mocks
    $script:state.Commands = @{ winget = $false; git = $false; pwsh = $false; wt = $false; mise = $false }
    $script:state.Registry = @{}
    $checks = Get-SetupChecks
    $byName = @{}
    foreach ($c in $checks) { $byName[$c.Name] = $c }
    Assert ($byName['winget (App Installer)'].Status -eq 'MISSING') 'winget reports MISSING'
    Assert ($byName['winget (App Installer)'].Fix -like '*App Installer*') 'winget fix mentions App Installer'
    Assert ($byName['Git for Windows'].Status -eq 'MISSING') 'Git reports MISSING'
    Assert ($byName['PowerShell 7 (pwsh)'].Status -eq 'WARN') 'pwsh reports WARN'
    Assert ($byName['Windows Terminal (wt)'].Status -eq 'WARN') 'wt reports WARN'
    Assert ($byName['mise'].Status -eq 'MISSING') 'mise reports MISSING'
    Assert ($byName['Developer Mode'].Status -eq 'MISSING') 'Developer Mode reports MISSING'
    Assert ($byName['Win32 long paths'].Status -eq 'MISSING') 'Win32 long paths reports MISSING'
    Assert ($byName['mise toolchain'].Status -eq 'MISSING') 'mise toolchain reports MISSING when mise absent'
    Assert (-not $byName.ContainsKey('git core.longpaths')) 'Repo-local git checks skipped when git is missing'
    $exitCode = Invoke-SetupDev
    Assert ($exitCode -eq 1) "Missing required checks must exit 1, got $exitCode"

    # 3. Repo-local git checks and mise toolchain MISSING / WARN -> exit 1
    Reset-Mocks
    $script:state.HasGitAttrs = $false
    $script:state.GitConfig = @{
        'core.longpaths' = 'false'
        'core.symlinks'  = 'false'
        'core.autocrlf'  = 'true'
    }
    $script:state.MiseMissing = @('go', 'yamlfmt')
    $checks = Get-SetupChecks
    $byName = @{}
    foreach ($c in $checks) { $byName[$c.Name] = $c }
    Assert ($byName['git core.longpaths'].Status -eq 'MISSING') 'core.longpaths reports MISSING'
    Assert ($byName['git core.symlinks'].Status -eq 'WARN') 'core.symlinks reports WARN'
    Assert ($byName['git core.symlinks'].Fix -like '*no symlinks today*') 'core.symlinks notes repo tracks no symlinks today'
    Assert ($byName['git core.autocrlf'].Status -eq 'MISSING') 'core.autocrlf reports MISSING when no .gitattributes'
    Assert ($byName['mise toolchain'].Status -eq 'MISSING') 'mise toolchain reports MISSING when tools missing'
    Assert ($byName['mise toolchain'].Fix -like '*go, yamlfmt*') 'mise toolchain lists missing tools'
    $exitCode = Invoke-SetupDev
    Assert ($exitCode -eq 1) "Missing git/mise required checks must exit 1, got $exitCode"

    # 3b. mise non-zero exit code (e.g. untrusted config) reports MISSING
    Reset-Mocks
    $script:state.MiseExitCode = 1
    $script:state.MiseOutput = 'Config file is not trusted'
    $checks = Get-SetupChecks
    $miseToolCheck = $checks | Where-Object { $_.Name -eq 'mise toolchain' }
    Assert ($miseToolCheck.Status -eq 'MISSING') 'mise non-zero exit reports MISSING'

    # 3c. core.autocrlf=false without .gitattributes reports OK
    Reset-Mocks
    $script:state.HasGitAttrs = $false
    $script:state.GitConfig['core.autocrlf'] = 'false'
    $checks = Get-SetupChecks
    $autocrlfCheck = $checks | Where-Object { $_.Name -eq 'git core.autocrlf' }
    Assert ($autocrlfCheck.Status -eq 'OK') 'core.autocrlf=false without .gitattributes reports OK'

    # 4. Only recommended (WARN) checks failing -> exit 0
    Reset-Mocks
    $script:state.Commands['pwsh'] = $false
    $script:state.Commands['wt'] = $false
    $script:state.GitConfig['core.symlinks'] = 'false'
    $exitCode = Invoke-SetupDev
    Assert ($exitCode -eq 0) "WARN-only checks must exit 0, got $exitCode"

    # 5. Admin-only fixes skipped when not elevated
    Reset-Mocks
    $script:state.IsAdmin = $false
    $script:state.Registry = @{
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock\AllowDevelopmentWithoutDevLicense' = 0
        'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled'                               = 0
    }
    $exitCode = Invoke-SetupDev -Fix
    Assert ($script:calls.Registry.Count -eq 0) 'Registry must not be touched when not elevated'
    Assert (($script:messages -join "`n") -like '*Administrator*') 'Skipped admin fixes must explain elevation is required'
    Assert ($exitCode -eq 1) "Still-missing admin checks must exit 1, got $exitCode"

    # 6. Admin-only fixes applied when elevated
    Reset-Mocks
    $script:state.IsAdmin = $true
    $script:state.Registry = @{
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock\AllowDevelopmentWithoutDevLicense' = 0
        'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled'                               = 0
    }
    $exitCode = Invoke-SetupDev -Fix
    Assert ($script:calls.Registry.Count -eq 2) "Expected 2 registry writes when elevated, got $($script:calls.Registry.Count)"
    Assert ($script:calls.Registry[0].Path -eq 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock') 'Developer Mode key path'
    Assert ($script:calls.Registry[0].Name -eq 'AllowDevelopmentWithoutDevLicense' -and $script:calls.Registry[0].Value -eq 1) 'Developer Mode value'
    Assert ($script:calls.Registry[1].Path -eq 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem') 'Long paths key path'
    Assert ($script:calls.Registry[1].Name -eq 'LongPathsEnabled' -and $script:calls.Registry[1].Value -eq 1) 'Long paths value'
    Assert ($exitCode -eq 0) "Elevated registry fixes should re-check to exit 0, got $exitCode"

    # 7. -Fix -WhatIf calls NO mutating function (even when elevated and every fixable check is missing)
    Reset-Mocks
    $script:state.IsAdmin = $true
    $script:state.Commands = @{ winget = $true; git = $true; pwsh = $false; wt = $false; mise = $true }
    $script:state.Registry = @{}
    $script:state.HasGitAttrs = $false
    $script:state.GitConfig = @{ 'core.longpaths' = 'false'; 'core.symlinks' = 'false'; 'core.autocrlf' = 'true' }
    $script:state.MiseMissing = @('go')
    $exitCode = Invoke-SetupDev -Fix -WhatIf
    Assert ($script:calls.Winget.Count -eq 0) '-Fix -WhatIf must not call Invoke-WingetInstall'
    Assert ($script:calls.Registry.Count -eq 0) '-Fix -WhatIf must not call Set-RegistryDword'
    Assert ($script:calls.GitSet.Count -eq 0) '-Fix -WhatIf must not call Set-GitConfigLocal'
    Assert ($script:calls.MiseFix.Count -eq 0) '-Fix -WhatIf must not call Invoke-MiseTrustAndInstall'
    Assert ($exitCode -eq 1) '-Fix -WhatIf leaves missing required checks failing with exit 1'

    # 8. -Fix calls the right winget IDs and reminds to open a new shell after mise
    Reset-Mocks
    $script:state.Commands = @{ winget = $true; git = $false; pwsh = $false; wt = $false; mise = $false }
    $exitCode = Invoke-SetupDev -Fix
    Assert (($script:calls.Winget -join ',') -ceq 'Git.Git,Microsoft.PowerShell,Microsoft.WindowsTerminal,jdx.mise') "Unexpected winget IDs: $($script:calls.Winget -join ',')"
    Assert (($script:messages -join "`n") -like '*new shell*') 'Installing mise must tell user to open a new shell'
    Assert ($exitCode -eq 0) "All winget fixes succeeded -> exit 0, got $exitCode"

    # 9. -Fix calls the right git config keys and mise trust/install
    Reset-Mocks
    $script:state.HasGitAttrs = $false
    $script:state.GitConfig = @{
        'core.longpaths' = 'false'
        'core.symlinks'  = 'false'
        'core.autocrlf'  = 'true'
    }
    $script:state.MiseMissing = @('go', 'task')
    $exitCode = Invoke-SetupDev -Fix
    $gitSummary = @($script:calls.GitSet | ForEach-Object { "$($_.Key)=$($_.Value)" }) -join ','
    Assert ($gitSummary -ceq 'core.longpaths=true,core.symlinks=true,core.autocrlf=false') "Unexpected git config calls: $gitSummary"
    Assert ($script:calls.MiseFix.Count -eq 1 -and $script:calls.MiseFix[0] -eq $tempDir) 'mise trust/install called at repo root'
    Assert ($exitCode -eq 0) "Git and mise fixes should re-check to exit 0, got $exitCode"

    # 10. -Fix skips winget installs cleanly when winget itself is missing
    Reset-Mocks
    $script:state.Commands = @{ winget = $false; git = $false; pwsh = $false; wt = $false; mise = $false }
    $exitCode = Invoke-SetupDev -Fix
    Assert ($script:calls.Winget.Count -eq 0) 'Must not invoke winget when winget is missing'
    Assert ($exitCode -eq 1) 'Missing winget must exit 1'

    # 11. Real Test-RepoFile works against temp directory without touching the repo
    $testFile = Join-Path $tempDir '.gitattributes'
    Set-Content -LiteralPath $testFile -Value '* text=auto eol=lf'
    $realTestRepoFile = ($ast.FindAll({
        param($node)
        $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Test-RepoFile'
    }, $false) | Select-Object -First 1).Extent.Text
    Invoke-Expression $realTestRepoFile
    Assert (Test-RepoFile -Path $testFile) 'Real Test-RepoFile returns true for existing file'
    Assert (-not (Test-RepoFile -Path (Join-Path $tempDir 'missing-file'))) 'Real Test-RepoFile returns false for missing file'

    Write-Host 'Windows dev setup tests passed.'
} finally {
    Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
