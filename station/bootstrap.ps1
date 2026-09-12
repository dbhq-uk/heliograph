# =============================================================================
#  bootstrap.ps1 - plant the heliograph station where there is no bash
# =============================================================================
#     .\bootstrap.ps1 <target-dir>
#     .\bootstrap.ps1 <target-dir> -Flavour powershell   # the default here
#     .\bootstrap.ps1 <target-dir> -Flavour bash         # for a machine with bash
#     .\bootstrap.ps1 <target-dir> -Flavour both
#
#  THE THIRD WAY TO PLANT A STATION, and the only one that needs nothing
#  installed. `heliograph bootstrap` needs the Go binary; bootstrap.sh needs
#  bash. This needs a copy of this repository and the PowerShell that is
#  already on the machine - which is the whole premise of the PowerShell
#  station, so the plant may not be the one step that assumes otherwise.
#
#  IT DEFAULTS TO THE POWERSHELL PAYLOAD, and bootstrap.sh defaults to bash.
#  Each defaults to the payload matching the interpreter that ran it, because
#  that is overwhelmingly what somebody reaching for it wants: an operator who
#  had bash would have used bootstrap.sh.
#
#  THE TARGET SHOULD BE ITS OWN PRIVATE REPO, not this one and not a repo that
#  holds anything else. Captured logs are published to it - that is how a run
#  escapes a machine nobody can reach - so whatever the operator's commands
#  print ends up in that repo's history permanently. A transport repo is cheap
#  to create and cheap to delete; a shared one is neither.
#
#  Nothing here is overwritten silently: an existing file is left alone and
#  reported, so re-running this to pick up a newer payload is safe.
# =============================================================================
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)][string] $Target,
    [ValidateSet('bash', 'powershell', 'both')][string] $Flavour = 'powershell'
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$Here = Split-Path -Parent $MyInvocation.MyCommand.Path

# The order matters and is the same one the other two bootstraps use: nothing is
# overwritten, so whichever is first supplies the files the two payloads share.
$roots = switch ($Flavour) {
    'bash' { , @('bash') }
    'powershell' { , @('powershell') }
    'both' { , @('bash', 'powershell') }
}

foreach ($r in $roots) {
    if (-not (Test-Path -LiteralPath (Join-Path $Here $r) -PathType Container)) {
        throw "this checkout carries no '$r' payload at $(Join-Path $Here $r)"
    }
}

[void](New-Item -ItemType Directory -Force -Path $Target)
$Target = (Resolve-Path -LiteralPath $Target).Path

foreach ($r in $roots) {
    if ($Target -eq (Resolve-Path -LiteralPath (Join-Path $Here $r)).Path) {
        throw 'refusing to bootstrap the toolkit over itself'
    }
}

# A STATION'S OWN RUNTIME STATE IS NOT PART OF THE PAYLOAD.
#
# Every one of these is written by a running station, is local to one machine,
# and is in the payload's .gitignore because it is nobody else's. Planting one
# hands a brand-new transport repo the last machine's lock pid, its approved
# step hashes, or - worst - .station-env, which holds a token.
#
# Get-ChildItem does not read .gitignore, so it has to be told. A checkout that
# has ever run a station or its tests has these sitting in it, invisible to
# `git status`. bootstrap.sh prunes the same list and station/embed_test.go
# refuses to let them into the binary at all.
$RuntimeState = @(
    '.station.lock', '.station-state', '.station-approved', '.station-approved-ps',
    '.station-delivery', '.station-env', '.station-env-ps', '.station-relay-state',
    '.agent-service.pid', '.station-service.log'
)

# `terraform init` drops a provider binary next to each Azure template, and
# azurerm alone is over 300MB. Without this prune a bootstrapped transport repo
# went from about 200KB to 913MB - measured, not guessed - and that repo gets
# cloned on a locked-down control node, sometimes over the link that is the
# reason this tool exists.
$PruneDirs = @('.terraform', '.git', 'node_modules')

$copied = 0
$skipped = 0

foreach ($r in $roots) {
    $src = (Resolve-Path -LiteralPath (Join-Path $Here $r)).Path
    $prefix = if ($roots.Count -gt 1) { "$r`: " } else { '' }

    # -Force, or every dotfile in the payload is silently missed: ops-logs's
    # .gitkeep, the Terraform lock files, .funcignore. A payload that looks
    # complete and is not is the worst shape this script could produce.
    $files = Get-ChildItem -LiteralPath $src -Recurse -File -Force | Sort-Object FullName
    foreach ($f in $files) {
        $rel = $f.FullName.Substring($src.Length).TrimStart('\', '/')
        $parts = $rel -split '[\\/]'
        $pruned = $false
        foreach ($p in $parts) { if ($PruneDirs -contains $p) { $pruned = $true } }
        if ($pruned) { continue }
        if ($RuntimeState -contains $f.Name) { continue }

        # The payload ships its ignore and attributes files WITHOUT a leading
        # dot, so that they govern the transport repo rather than the repo
        # carrying them. Restore the dot here, at the root only: the Azure
        # templates have their own .funcignore and it is already correct.
        $dest = $rel
        if ($rel -ceq 'gitignore') { $dest = '.gitignore' }
        if ($rel -ceq 'gitattributes') { $dest = '.gitattributes' }

        $full = Join-Path $Target $dest
        if (Test-Path -LiteralPath $full) {
            Write-Output "  exists, left alone : $prefix$dest"
            $skipped++
            continue
        }
        $dir = Split-Path -Parent $full
        if ($dir -and -not (Test-Path -LiteralPath $dir)) {
            [void](New-Item -ItemType Directory -Force -Path $dir)
        }
        Copy-Item -LiteralPath $f.FullName -Destination $full
        Write-Output "  installed          : $prefix$dest"
        $copied++
    }
}

Write-Output ''
Write-Output "heliograph: $copied file(s) installed, $skipped left alone, in $Target"

if (-not (Test-Path -LiteralPath (Join-Path $Target '.git'))) {
    Write-Output ''
    Write-Output "$Target is not a git repository yet, and git is the default transport. Next:"
    Write-Output "  cd $Target; git init; git add -A; git commit -m 'heliograph: transport repo'"
    Write-Output '  then add a PRIVATE remote and push.'
    Write-Output ''
    Write-Output 'If this estate has no git host, the station can use a file share instead:'
    Write-Output '  set TRANSPORT=share, SHARE_DIR and SHARE_SCOPE, and skip the repo entirely.'
}

Write-Output ''
switch ($Flavour) {
    'bash' { Write-Output 'Then, on the far side: ./start.sh --check, then ./station.sh' }
    default {
        Write-Output 'Then, on the far side:'
        Write-Output '  .\start.ps1 --check     # will this work here, changing nothing'
        Write-Output '  .\run.ps1 env           # one step, by hand'
        Write-Output '  .\station.ps1           # the loop: poll, run, deliver, repeat'
    }
}
