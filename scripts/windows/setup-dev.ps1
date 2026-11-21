#Requires -Version 5.1
<#
.SYNOPSIS
Checks and optionally fixes Windows prerequisites for developing and running DevEx.
.DESCRIPTION
Read-only by default. Prints one line per check with its name, status
(OK, MISSING, or WARN), and the fix when needed. Exits 1 if any required
check fails, and 0 otherwise.

With -Fix, applies fixable items through ShouldProcess (so -Fix -WhatIf
changes nothing), then checks again. Admin-only registry fixes are skipped
when the shell is not elevated.
.PARAMETER Fix
Apply fixable prerequisites and re-run the checks.
#>
[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [switch]$Fix
)

function Write-SetupMessage {
    param([string]$Message)
    Write-Host $Message
}

function Test-CommandExists {
    param([string]$Name)
    return ($null -ne (Get-Command -Name $Name -ErrorAction SilentlyContinue))
}

function Test-IsAdministrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-RegistryValue {
    param(
        [string]$Path,
        [string]$Name
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        return $null
    }
    $item = Get-ItemProperty -LiteralPath $Path -Name $Name -ErrorAction SilentlyContinue
    if ($null -eq $item) {
        return $null
    }
    $property = $item.PSObject.Properties[$Name]
    if ($null -eq $property) {
        return $null
    }
    return $property.Value
}

function Set-RegistryDword {
    param(
        [string]$Path,
        [string]$Name,
        [int]$Value
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        New-Item -Path $Path -Force | Out-Null
    }
    New-ItemProperty -Path $Path -Name $Name -Value $Value -PropertyType DWord -Force | Out-Null
}

function Invoke-WingetInstall {
    param([string]$Id)
    # Send tool output to the host so only the exit code is returned.
    & winget install --id $Id -e --source winget | Out-Host
    return $LASTEXITCODE
}

function Get-GitRepoRoot {
    if (-not (Test-CommandExists -Name 'git')) {
        return $null
    }
    $previousEap = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = & git rev-parse --show-toplevel 2>$null
        if ($LASTEXITCODE -ne 0 -or -not $output) {
            return $null
        }
        return ($output | Select-Object -First 1).ToString().Trim()
    } finally {
        $ErrorActionPreference = $previousEap
    }
}

function Get-GitConfigValue {
    param(
        [string]$RepoRoot,
        [string]$Key
    )
    $previousEap = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = & git -C $RepoRoot config --get $Key 2>$null
        if ($LASTEXITCODE -ne 0 -or $null -eq $output) {
            return $null
        }
        return ($output | Select-Object -First 1).ToString().Trim()
    } finally {
        $ErrorActionPreference = $previousEap
    }
}

function Set-GitConfigLocal {
    param(
        [string]$RepoRoot,
        [string]$Key,
        [string]$Value
    )
    & git -C $RepoRoot config --local $Key $Value | Out-Host
    return $LASTEXITCODE
}

function Test-RepoFile {
    param([string]$Path)
    return (Test-Path -LiteralPath $Path -PathType Leaf)
}

function Get-MiseMissingTools {
    param([string]$RepoRoot)
    $previousEap = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        if ($RepoRoot) {
            $raw = & mise -C $RepoRoot ls --current --missing --no-header 2>&1
        } else {
            $raw = & mise ls --current --missing --no-header 2>&1
        }
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousEap
    }
    $lines = @($raw | ForEach-Object { $_.ToString().Trim() } | Where-Object { $_ })
    $missing = @()
    if ($exitCode -eq 0) {
        $missing = @($lines | ForEach-Object { ($_ -split '\s+')[0] })
    }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Missing  = $missing
        Output   = ($lines -join ' ')
    }
}

function Invoke-MiseTrustAndInstall {
    param([string]$RepoRoot)
    if ($RepoRoot) {
        & mise -C $RepoRoot trust | Out-Host
        if ($LASTEXITCODE -ne 0) { return $LASTEXITCODE }
        & mise -C $RepoRoot install | Out-Host
        return $LASTEXITCODE
    }
    & mise trust | Out-Host
    if ($LASTEXITCODE -ne 0) { return $LASTEXITCODE }
    & mise install | Out-Host
    return $LASTEXITCODE
}

function New-CheckResult {
    param(
        [string]$Name,
        [string]$Status,
        [bool]$Required,
        [string]$Detail = '',
        [string]$Fix = '',
        [string]$FixKind = 'None',
        [string]$FixTarget = '',
        [bool]$RequiresAdmin = $false,
        [string]$RegistryPath = '',
        [string]$RegistryName = '',
        [int]$RegistryValue = 0,
        [string]$RepoRoot = '',
        [string]$GitKey = '',
        [string]$GitValue = '',
        [bool]$NeedsNewShell = $false
    )
    return [pscustomobject]@{
        Name          = $Name
        Status        = $Status
        Required      = $Required
        Detail        = $Detail
        Fix           = $Fix
        FixKind       = $FixKind
        FixTarget     = $FixTarget
        RequiresAdmin = $RequiresAdmin
        RegistryPath  = $RegistryPath
        RegistryName  = $RegistryName
        RegistryValue = $RegistryValue
        RepoRoot      = $RepoRoot
        GitKey        = $GitKey
        GitValue      = $GitValue
        NeedsNewShell = $NeedsNewShell
    }
}

function Format-CheckLine {
    param([pscustomobject]$Check)
    $tag = ('[' + $Check.Status + ']').PadRight(9)
    $message = if ($Check.Status -eq 'OK') { $Check.Detail } else { $Check.Fix }
    return "$tag $($Check.Name) - $message"
}

function Get-SetupChecks {
    $checks = New-Object System.Collections.ArrayList

    $hasWinget = Test-CommandExists -Name 'winget'
    [void]$checks.Add((New-CheckResult `
        -Name 'winget (App Installer)' `
        -Status $(if ($hasWinget) { 'OK' } else { 'MISSING' }) `
        -Required $true `
        -Detail 'installed' `
        -Fix 'Install "App Installer" from the Microsoft Store' `
        -FixKind 'Manual'))

    $hasGit = Test-CommandExists -Name 'git'
    [void]$checks.Add((New-CheckResult `
        -Name 'Git for Windows' `
        -Status $(if ($hasGit) { 'OK' } else { 'MISSING' }) `
        -Required $true `
        -Detail 'installed' `
        -Fix 'winget install --id Git.Git -e --source winget' `
        -FixKind 'Winget' `
        -FixTarget 'Git.Git'))

    $hasPwsh = Test-CommandExists -Name 'pwsh'
    [void]$checks.Add((New-CheckResult `
        -Name 'PowerShell 7 (pwsh)' `
        -Status $(if ($hasPwsh) { 'OK' } else { 'WARN' }) `
        -Required $false `
        -Detail 'installed' `
        -Fix 'winget install --id Microsoft.PowerShell -e --source winget' `
        -FixKind 'Winget' `
        -FixTarget 'Microsoft.PowerShell'))

    $hasWt = Test-CommandExists -Name 'wt'
    [void]$checks.Add((New-CheckResult `
        -Name 'Windows Terminal (wt)' `
        -Status $(if ($hasWt) { 'OK' } else { 'WARN' }) `
        -Required $false `
        -Detail 'installed' `
        -Fix 'winget install --id Microsoft.WindowsTerminal -e --source winget' `
        -FixKind 'Winget' `
        -FixTarget 'Microsoft.WindowsTerminal'))

    $hasMise = Test-CommandExists -Name 'mise'
    [void]$checks.Add((New-CheckResult `
        -Name 'mise' `
        -Status $(if ($hasMise) { 'OK' } else { 'MISSING' }) `
        -Required $true `
        -Detail 'installed' `
        -Fix 'winget install --id jdx.mise -e --source winget (then open a new shell)' `
        -FixKind 'Winget' `
        -FixTarget 'jdx.mise' `
        -NeedsNewShell $true))

    $devModePath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'
    $devModeName = 'AllowDevelopmentWithoutDevLicense'
    $devModeValue = Get-RegistryValue -Path $devModePath -Name $devModeName
    $devModeOk = ($null -ne $devModeValue -and [int]$devModeValue -eq 1)
    [void]$checks.Add((New-CheckResult `
        -Name 'Developer Mode' `
        -Status $(if ($devModeOk) { 'OK' } else { 'MISSING' }) `
        -Required $true `
        -Detail 'enabled (non-admin symlinks allowed)' `
        -Fix "Run as Administrator: set $devModePath\$devModeName = 1 (or enable Developer Mode in Windows Settings)" `
        -FixKind 'RegistryDword' `
        -RequiresAdmin $true `
        -RegistryPath $devModePath `
        -RegistryName $devModeName `
        -RegistryValue 1))

    $longPathsPath = 'HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem'
    $longPathsName = 'LongPathsEnabled'
    $longPathsValue = Get-RegistryValue -Path $longPathsPath -Name $longPathsName
    $longPathsOk = ($null -ne $longPathsValue -and [int]$longPathsValue -eq 1)
    [void]$checks.Add((New-CheckResult `
        -Name 'Win32 long paths' `
        -Status $(if ($longPathsOk) { 'OK' } else { 'MISSING' }) `
        -Required $true `
        -Detail 'enabled' `
        -Fix "Run as Administrator: set $longPathsPath\$longPathsName = 1" `
        -FixKind 'RegistryDword' `
        -RequiresAdmin $true `
        -RegistryPath $longPathsPath `
        -RegistryName $longPathsName `
        -RegistryValue 1))

    $repoRoot = Get-GitRepoRoot
    if ($repoRoot) {
        $gitLongPaths = Get-GitConfigValue -RepoRoot $repoRoot -Key 'core.longpaths'
        $gitLongPathsOk = ($null -ne $gitLongPaths -and $gitLongPaths -ieq 'true')
        [void]$checks.Add((New-CheckResult `
            -Name 'git core.longpaths' `
            -Status $(if ($gitLongPathsOk) { 'OK' } else { 'MISSING' }) `
            -Required $true `
            -Detail 'true' `
            -Fix 'git config --local core.longpaths true' `
            -FixKind 'GitConfig' `
            -RepoRoot $repoRoot `
            -GitKey 'core.longpaths' `
            -GitValue 'true'))

        $gitSymlinks = Get-GitConfigValue -RepoRoot $repoRoot -Key 'core.symlinks'
        $gitSymlinksOk = ($null -ne $gitSymlinks -and $gitSymlinks -ieq 'true')
        [void]$checks.Add((New-CheckResult `
            -Name 'git core.symlinks' `
            -Status $(if ($gitSymlinksOk) { 'OK' } else { 'WARN' }) `
            -Required $false `
            -Detail 'true (repo tracks no symlinks today)' `
            -Fix 'git config --local core.symlinks true (recommended; repo tracks no symlinks today)' `
            -FixKind 'GitConfig' `
            -RepoRoot $repoRoot `
            -GitKey 'core.symlinks' `
            -GitValue 'true'))

        $gitAutocrlf = Get-GitConfigValue -RepoRoot $repoRoot -Key 'core.autocrlf'
        $hasGitAttributes = Test-RepoFile -Path (Join-Path $repoRoot '.gitattributes')
        if ($hasGitAttributes) {
            $autocrlfDetail = if ($gitAutocrlf) {
                ".gitattributes handles line endings (overrides core.autocrlf=$gitAutocrlf)"
            } else {
                '.gitattributes handles line endings'
            }
            [void]$checks.Add((New-CheckResult `
                -Name 'git core.autocrlf' `
                -Status 'OK' `
                -Required $false `
                -Detail $autocrlfDetail `
                -FixKind 'None'))
        } else {
            $autocrlfOk = ($null -ne $gitAutocrlf -and $gitAutocrlf -ieq 'false')
            [void]$checks.Add((New-CheckResult `
                -Name 'git core.autocrlf' `
                -Status $(if ($autocrlfOk) { 'OK' } else { 'MISSING' }) `
                -Required $true `
                -Detail 'false' `
                -Fix 'git config --local core.autocrlf false' `
                -FixKind 'GitConfig' `
                -RepoRoot $repoRoot `
                -GitKey 'core.autocrlf' `
                -GitValue 'false'))
        }
    }

    if (-not $hasMise) {
        [void]$checks.Add((New-CheckResult `
            -Name 'mise toolchain' `
            -Status 'MISSING' `
            -Required $true `
            -Fix 'Install mise first, open a new shell, then run: mise trust && mise install' `
            -FixKind 'None'))
    } else {
        $miseCheck = Get-MiseMissingTools -RepoRoot $repoRoot
        if ($miseCheck.ExitCode -ne 0) {
            [void]$checks.Add((New-CheckResult `
                -Name 'mise toolchain' `
                -Status 'MISSING' `
                -Required $true `
                -Fix 'mise trust && mise install (mise check failed; config may need trust)' `
                -FixKind 'Mise' `
                -RepoRoot $repoRoot))
        } elseif ($miseCheck.Missing.Count -gt 0) {
            $missingList = $miseCheck.Missing -join ', '
            [void]$checks.Add((New-CheckResult `
                -Name 'mise toolchain' `
                -Status 'MISSING' `
                -Required $true `
                -Fix "mise trust && mise install (missing: $missingList)" `
                -FixKind 'Mise' `
                -RepoRoot $repoRoot))
        } else {
            [void]$checks.Add((New-CheckResult `
                -Name 'mise toolchain' `
                -Status 'OK' `
                -Required $true `
                -Detail 'all configured tools installed' `
                -FixKind 'None'))
        }
    }

    return @($checks)
}

function Invoke-SetupFix {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param(
        [pscustomobject[]]$Checks
    )

    $isAdmin = Test-IsAdministrator
    $wingetCheck = $Checks | Where-Object { $_.Name -eq 'winget (App Installer)' } | Select-Object -First 1
    $hasWinget = ($null -ne $wingetCheck -and $wingetCheck.Status -eq 'OK')

    foreach ($check in $Checks) {
        if ($check.Status -eq 'OK') {
            continue
        }
        switch ($check.FixKind) {
            'Manual' {
                Write-SetupMessage "Skipping $($check.Name): cannot be fixed automatically. $($check.Fix)."
            }
            'Winget' {
                if (-not $hasWinget) {
                    Write-SetupMessage "Skipping $($check.Name): winget is not installed."
                    continue
                }
                if ($PSCmdlet.ShouldProcess($check.Name, "winget install --id $($check.FixTarget) -e --source winget")) {
                    $code = Invoke-WingetInstall -Id $check.FixTarget
                    if ($code -ne 0) {
                        Write-SetupMessage "Failed to install $($check.Name) via winget (exit $code)."
                    } elseif ($check.NeedsNewShell) {
                        Write-SetupMessage "Installed $($check.Name). Open a new shell so PATH picks it up, then run: mise trust && mise install"
                    }
                }
            }
            'RegistryDword' {
                if ($check.RequiresAdmin -and -not $isAdmin) {
                    Write-SetupMessage "Skipping $($check.Name): requires an elevated (Administrator) shell. Re-run this script as Administrator."
                    continue
                }
                $target = "$($check.RegistryPath)\$($check.RegistryName)"
                if ($PSCmdlet.ShouldProcess($target, "Set DWORD to $($check.RegistryValue)")) {
                    Set-RegistryDword -Path $check.RegistryPath -Name $check.RegistryName -Value $check.RegistryValue
                }
            }
            'GitConfig' {
                $target = $check.RepoRoot
                $action = "git config --local $($check.GitKey) $($check.GitValue)"
                if ($PSCmdlet.ShouldProcess($target, $action)) {
                    $code = Set-GitConfigLocal -RepoRoot $check.RepoRoot -Key $check.GitKey -Value $check.GitValue
                    if ($code -ne 0) {
                        Write-SetupMessage "Failed to set $($check.GitKey) (exit $code)."
                    }
                }
            }
            'Mise' {
                $target = if ($check.RepoRoot) { $check.RepoRoot } else { (Get-Location).Path }
                if ($PSCmdlet.ShouldProcess($target, 'mise trust && mise install')) {
                    $code = Invoke-MiseTrustAndInstall -RepoRoot $check.RepoRoot
                    if ($code -ne 0) {
                        Write-SetupMessage "mise trust/install failed (exit $code)."
                    }
                }
            }
            default {
                # Informational or depends on another tool being installed first.
            }
        }
    }
}

function Invoke-SetupDev {
    [CmdletBinding(SupportsShouldProcess = $true)]
    param(
        [switch]$Fix
    )

    $checks = Get-SetupChecks
    foreach ($check in $checks) {
        Write-SetupMessage (Format-CheckLine -Check $check)
    }

    if ($Fix) {
        $fixParams = @{ Checks = $checks }
        if ($PSBoundParameters.ContainsKey('WhatIf')) { $fixParams['WhatIf'] = $PSBoundParameters['WhatIf'] }
        if ($PSBoundParameters.ContainsKey('Confirm')) { $fixParams['Confirm'] = $PSBoundParameters['Confirm'] }
        Invoke-SetupFix @fixParams

        Write-SetupMessage ''
        Write-SetupMessage 'Re-checking prerequisites...'
        $checks = Get-SetupChecks
        foreach ($check in $checks) {
            Write-SetupMessage (Format-CheckLine -Check $check)
        }
    }

    $failedRequired = @($checks | Where-Object { $_.Required -and $_.Status -ne 'OK' })
    if ($failedRequired.Count -gt 0) {
        return 1
    }
    return 0
}

$invokeParams = @{ Fix = $Fix }
if ($PSBoundParameters.ContainsKey('WhatIf')) { $invokeParams['WhatIf'] = $PSBoundParameters['WhatIf'] }
if ($PSBoundParameters.ContainsKey('Confirm')) { $invokeParams['Confirm'] = $PSBoundParameters['Confirm'] }
$exitCode = Invoke-SetupDev @invokeParams
exit $exitCode
