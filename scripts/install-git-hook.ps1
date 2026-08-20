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
# and the two copies would drift — the second one silently, since for a long
# time it was the one nobody's CI exercised: the hooks job ran the sh suite on
# windows-latest and never started pwsh at all, which is exactly how the WSL
# defect below shipped. install-git-hook.Tests.ps1 is what ended that; ci.yml's
# hooks job runs it under pwsh on windows-latest, and
# tests/ci_workflow_test.go holds that declaration in place.
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

# Test-WslBashLauncher — is this path Windows' WSL launcher rather than a bash?
#
# C:\Windows\System32\bash.exe exists on any machine with the WSL optional
# feature enabled, System32 is early on PATH, and the thing it launches is a
# LINUX shell inside a VM: hand it "C:\...\install-git-hook.sh" and it resolves
# that string inside the Linux filesystem, dies with "No such file or
# directory" — and because a bash WAS found, the wrapper's no-bash remedy
# message (install Git for Windows / run from WSL yourself) never printed.
# Failing to reject this launcher is therefore worse than finding no bash at
# all. Sysnative is the same file seen from a 32-bit process, where Windows
# redirects "System32" to SysWOW64 and offers Sysnative as the real one.
#
# The comparison is textual and case-insensitive on the directory PREFIX, with
# separators normalised first, because that is what PATH resolution hands us; a
# bash merely NAMED oddly elsewhere is not rejected — the guard is about these
# two directories, which only Windows populates, not about the file's contents.
function Test-WslBashLauncher {
    param([string]$Path)
    if (-not $Path -or -not $env:SystemRoot) { return $false }
    $normalized = $Path.Replace('/', '\')
    foreach ($sysDir in @('System32', 'Sysnative')) {
        $prefix = $env:SystemRoot.TrimEnd('\') + '\' + $sysDir + '\'
        if ($normalized.StartsWith($prefix, [System.StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }
    return $false
}

# Find a bash. Preference order is deliberate: a bash the user has put on PATH
# is the one they expect to be used — UNLESS it is WSL's launcher, which nobody
# "put" anywhere (enabling the WSL feature drops it into System32) and which
# cannot run a script that lives on a C:\ path; see Test-WslBashLauncher. Only
# past that do we go digging in the Git installation. git --exec-path points at
# <git>\mingw64\libexec\git-core, and the Git install root is THREE directories
# up from there (git-core -> libexec -> mingw64 -> <git>) — which is what the
# three Split-Path -Parent calls below compute. The bundled bash is under that
# root, in bin\ or usr\bin\.
function Find-Bash {
    $onPath = Get-Command bash -ErrorAction SilentlyContinue
    if ($onPath -and -not (Test-WslBashLauncher $onPath.Source)) { return $onPath.Source }

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

# THE MAIN GUARD. Everything below runs only when this file is executed as a
# script; dot-sourcing it (`. .\install-git-hook.ps1`) defines the functions
# above and stops here — $MyInvocation.InvocationName is the literal '.' in
# that case and the script's own name or path in every executing case. This is
# the PowerShell spelling of Python's __main__ guard, and it exists so that
# install-git-hook.Tests.ps1 can load Find-Bash and test it WITHOUT running an
# installer whose main body would prompt, resolve a bash, and exit the process
# — that `exit $LASTEXITCODE` at the bottom would take a dot-sourcing session
# down with it.
if ($MyInvocation.InvocationName -ne '.') {
    $sh = Join-Path $PSScriptRoot 'install-git-hook.sh'
    if (-not (Test-Path -LiteralPath $sh)) {
        Write-Error "install-git-hook.sh was not found next to this wrapper ($sh)"
        exit 1
    }

    # NAME $sh HERE, NOT A HARDCODED "scripts/install-git-hook.sh". That literal
    # is where the file lives in the DossierX repository and nowhere else — a
    # reader who fetched just these two files into their own project (README and
    # the router skill both hand out a pinned raw URL for exactly that) is not
    # standing in a directory with a scripts/ folder, and the hardcoded path is
    # a file-not-found for exactly the reader this message is for. install-git-
    # hook.sh had the same defect once, on the recoveries it prints for itself,
    # and DOSSIERX_HOOK_INVOCATION below exists to stop it coming back a second
    # time there. This message is a third place the same mistake could be
    # reintroduced, so: use the resolved path, not a literal.
    #
    # BUT NOT THE WINDOWS SPELLING OF IT ON THE WSL LINE. $sh is a native
    # Windows path — exactly the string the sentence above that line says
    # WSL's bash cannot open (the fix that put the resolved $sh here was right
    # for the "Install Git for Windows" half, which runs on the Windows side,
    # and self-contradicting for the WSL half). So the WSL line hands $sh to
    # wslpath INSIDE the WSL invocation, where the distro's mounts are
    # actually known; nothing computed here, on the Windows side, can know
    # them. What this wrapper still cannot promise — and the message says so —
    # is that the reader's distro mounts this drive at all: on a drive WSL
    # does not mount, or a distro without the interop mounts, no spelling of
    # this path is reachable and no line printed from here can fix that. The
    # single-quote doubling is bash's own escape for a ' inside a
    # single-quoted string, for the rare path that contains one. (The
    # here-string is expandable for that one interpolation; the backtick
    # before `$( keeps the command substitution literal text for PowerShell so
    # it reaches bash intact.)
    $bash = Find-Bash
    if (-not $bash) {
        $shForWsl = $sh.Replace("'", "'\''")
        Write-Error @"
No usable bash was found. A bash under Windows' System32 is WSL's launcher and
cannot run a script on a C:\ path, so it does not count. Install Git for
Windows (which bundles a real one) or run the installer from inside WSL, where
wslpath translates this script's Windows path into one WSL can open (whether
your distro mounts this drive at all is more than this wrapper can check from
Windows; if it does not, no spelling of this command can reach the script):

  bash "`$(wslpath '$shForWsl')" --yes
"@
        exit 1
    }

    # TELL THE SHELL SCRIPT HOW THE READER GOT HERE, so the recoveries it prints
    # are ones this reader can actually type. Every instruction install-git-
    # hook.sh offers — chaining a foreign hook, uninstalling a machine-wide
    # install — is a command line built from its own $0. A reader who reached it
    # through this wrapper is standing in PowerShell and, by this wrapper's
    # whole premise, may have no bash on PATH at all, so `sh ".../install-git-
    # hook.sh" --uninstall` is an instruction that does not run for them. This
    # names the wrapper instead. "powershell" rather than the host that is
    # running right now, because the line has to work when the reader pastes it
    # into whichever shell they have open later, and powershell.exe is the one
    # spelling every supported Windows has (pwsh understands -File the same
    # way, so a reader who substitutes it loses nothing).
    $env:DOSSIERX_HOOK_INVOCATION = "powershell -File `"$PSCommandPath`""

    # AND THE BASH WE RESOLVED, because "a bash exists" and "the name bash finds
    # it" are different facts and the recoveries need the second one. Find-Bash
    # locates a bash by PATH *or* by the Git for Windows install locations, and
    # the second case is the one this wrapper exists for — a machine where Git
    # for Windows is installed and its bin\ is not on PATH. A recovery line that
    # says `bash -c ...` is resolved by PATH when the reader pastes it, so on
    # exactly that machine it fails with "bash is not recognized" after having
    # already created the file it was meant to finish. The script that prints
    # those lines cannot re-derive this — it is running *under* the bash we
    # found, with no portable way to ask which one — so we hand it over.
    $env:DOSSIERX_HOOK_BASH = $bash

    & $bash $sh @args
    exit $LASTEXITCODE
}
