# =============================================================================
#  Find-GitBash.ps1 - which bash a Windows station uses, decided once
# =============================================================================
#  Dot-sourced by station.ps1 and by service.ps1. It was station.ps1's alone,
#  and service.ps1's header still says it does not look for bash - which was
#  true and became a problem the moment service.ps1 needed to run station-env.sh
#  to check a non-git station's configuration.
#
#  A SECOND COPY WOULD BE A SECOND ANSWER. The WSL trap below is the whole
#  reason this is careful, and a file that got it wrong would report a station
#  configuration as unusable because it asked the wrong bash - or, worse, launch
#  a Linux distribution.
# =============================================================================

function Find-GitBash {
    if ($env:HELIOGRAPH_BASH) {
        if (Test-Path $env:HELIOGRAPH_BASH) { return $env:HELIOGRAPH_BASH }
        throw "HELIOGRAPH_BASH is set to '$($env:HELIOGRAPH_BASH)' but nothing is there."
    }

    foreach ($key in @(
        'HKLM:\SOFTWARE\GitForWindows',
        'HKLM:\SOFTWARE\WOW6432Node\GitForWindows',
        'HKCU:\SOFTWARE\GitForWindows'
    )) {
        try {
            $install = (Get-ItemProperty -Path $key -Name InstallPath -ErrorAction Stop).InstallPath
            $candidate = Join-Path $install 'bin\bash.exe'
            if (Test-Path $candidate) { return $candidate }
        } catch { }
    }

    foreach ($p in @(
        "$env:ProgramFiles\Git\bin\bash.exe",
        "${env:ProgramFiles(x86)}\Git\bin\bash.exe",
        "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe"
    )) {
        if ($p -and (Test-Path $p)) { return $p }
    }

    $onPath = Get-Command bash.exe -ErrorAction SilentlyContinue
    if ($onPath -and $onPath.Source -notmatch '\\System32\\') { return $onPath.Source }

    throw @"
station.ps1: cannot find the bash that Git for Windows installs.

heliograph runs its steps in bash, and git is its transport, so a control node
needs Git for Windows either way. Install it from https://git-scm.com/download/win
and run this again.

If git is installed somewhere unusual, point at it directly:
    `$env:HELIOGRAPH_BASH = 'D:\tools\Git\bin\bash.exe'

A bash.exe in System32 is deliberately ignored. That one is WSL, which launches
a Linux distribution rather than Git bash, and using it would fail in a way that
says nothing about heliograph.
"@
}
