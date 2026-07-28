# install-git-hook.ps1 — Windows convenience wrapper around install-git-hook.sh.
#
# WHY A WRAPPER AND NOT A SECOND IMPLEMENTATION
#
# The thing being installed is a POSIX sh script, because that is what git
# executes: on Windows, git runs hooks through the sh that ships inside Git for
# Windows, not through cmd or PowerShell, and it does not consult the exec bit.
# So the hook is identical on every platform, and the only Windows-specific
# problem left is that PowerShell users may not have bash on PATH even though
# their Git installation certainly contains one.
#
# This file solves exactly that and nothing else. A parallel PowerShell
# implementation of the installer would double the surface that has to stay
# honest about core.hooksPath, worktrees, backups and the confirmation prompt,
# and the two copies would drift — the second one silently, since it is the one
# nobody's CI would exercise.
#
# USAGE (from a repository, in PowerShell):
#
#   .\scripts\install-git-hook.ps1 -y
#   .\scripts\install-git-hook.ps1 --dry-run
#   .\scripts\install-git-hook.ps1 --uninstall
#
# All arguments are passed straight through to install-git-hook.sh; see its
# header for the full list.

$ErrorActionPreference = 'Stop'

$sh = Join-Path $PSScriptRoot 'install-git-hook.sh'
if (-not (Test-Path -LiteralPath $sh)) {
    Write-Error "install-git-hook.sh was not found next to this wrapper ($sh)"
    exit 1
}

# Find a bash. Preference order is deliberate: a bash the user has put on PATH
# is the one they expect to be used, and only if there is none do we go digging
# in the Git installation. git --exec-path points at <git>\mingw64\libexec\
# git-core, from which the bundled bash is two directories up in bin\.
function Find-Bash {
    $onPath = Get-Command bash -ErrorAction SilentlyContinue
    if ($onPath) { return $onPath.Source }

    $git = Get-Command git -ErrorAction SilentlyContinue
    if ($git) {
        $execPath = (& git --exec-path) 2>$null
        if ($execPath) {
            $root = Split-Path (Split-Path (Split-Path $execPath -Parent) -Parent) -Parent
            foreach ($candidate in @(
                    (Join-Path $root 'bin\bash.exe'),
                    (Join-Path $root 'usr\bin\bash.exe'))) {
                if (Test-Path -LiteralPath $candidate) { return $candidate }
            }
        }
    }

    foreach ($candidate in @(
            "$env:ProgramFiles\Git\bin\bash.exe",
            "${env:ProgramFiles(x86)}\Git\bin\bash.exe",
            "$env:LOCALAPPDATA\Programs\Git\bin\bash.exe")) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) { return $candidate }
    }

    return $null
}

$bash = Find-Bash
if (-not $bash) {
    Write-Error @'
No bash was found. Install Git for Windows (which bundles one) or run the
installer from WSL:

  bash scripts/install-git-hook.sh --yes
'@
    exit 1
}

& $bash $sh @args
exit $LASTEXITCODE
