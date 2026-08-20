# install-git-hook.Tests.ps1 — Pester suite for the PowerShell wrapper.
#
# WHY THIS FILE EXISTS. Three of the last release's most serious defects lived
# in the two installer scripts, and the PowerShell one was executed by no CI
# job at all — the hooks job ran the sh suite through bash on windows-latest
# and never started pwsh. An LLM found the Find-Bash defect by READING the
# script; nothing had ever run it. This file is the running. ci.yml's hooks
# job invokes it under pwsh on windows-latest, and tests/ci_workflow_test.go
# pins that declaration so the step cannot be deleted silently.
#
# WHAT THIS FILE PROVES, AND WHAT IT CANNOT. The Find-Bash blocks below prove
# the GUARD: given a `bash` on PATH whose source is Windows' System32 (or
# Sysnative) launcher, Find-Bash refuses it and falls through to the Git for
# Windows candidates, and given nothing usable it returns $null so the wrapper
# prints its remedy instead of dying inside WSL. They prove it with MOCKS, so
# they are deterministic and run on a machine with no WSL at all. What they do
# NOT prove is WSL's actual behaviour — that C:\Windows\System32\bash.exe
# really resolves a C:\ path inside the Linux filesystem and fails. It does,
# but GitHub-hosted Windows runners cannot run WSL distros, so no test in this
# repository can observe it; that boundary is stated here rather than implied
# away, the same way gate/method.yaml states what a file cannot promise about
# the harness that reads it. If WSL's launcher ever moves out of System32,
# this suite goes green while the guard misses it — the guard is keyed to the
# directory, because the directory is the only stable, testable handle.
#
# The end-to-end blocks at the bottom are the complementary half: they execute
# the wrapper AS A SCRIPT, for real, so the main body (path resolution,
# argument pass-through, exit-code propagation, the no-bash remedy) is run by
# a machine and not only read by one. The install block needs a real git and a
# real bash, both of which every runner in the hooks matrix has; the no-bash
# block needs the opposite and manufactures it, by starting a child process
# whose environment can find neither.
#
# Pester v5 syntax; Pester ships preinstalled on windows-latest.

BeforeAll {
    # Dot-sourcing defines Find-Bash and Test-WslBashLauncher WITHOUT running
    # the installer: the wrapper's main body sits behind an InvocationName
    # guard precisely so this line is safe. If that guard is ever removed,
    # this dot-source becomes an attempted install (and an `exit`), which is
    # loud — not a quiet suite over nothing.
    . (Join-Path $PSScriptRoot 'install-git-hook.ps1')
    $script:wrapper = Join-Path $PSScriptRoot 'install-git-hook.ps1'
    $script:shInstaller = Join-Path $PSScriptRoot 'install-git-hook.sh'
}

Describe 'Find-Bash' {
    BeforeEach {
        # Pin the environment the guard reads, so the same assertions hold on
        # a Windows runner and on a maintainer's non-Windows machine alike.
        $script:savedSystemRoot = $env:SystemRoot
        $env:SystemRoot = 'C:\Windows'
    }
    AfterEach {
        $env:SystemRoot = $script:savedSystemRoot
    }

    It 'rejects WSL''s System32 launcher and falls through to the Git for Windows candidates' {
        # The defect this pins: with WSL enabled, `bash` on PATH is
        # C:\Windows\System32\bash.exe. The original Find-Bash returned it,
        # the install died inside the Linux VM with "No such file or
        # directory", and because A bash had been found, neither remedy
        # message ever printed.
        Mock Get-Command { [pscustomobject]@{ Source = 'C:\Windows\System32\bash.exe' } } -ParameterFilter { $Name -eq 'bash' }
        # git absent, so the exec-path branch is skipped and nothing in this
        # test executes a real binary: the result is decided by mocks alone.
        Mock Get-Command { $null } -ParameterFilter { $Name -eq 'git' }
        # Default: no candidate exists — except the first Git for Windows one.
        Mock Test-Path { $false }
        Mock Test-Path { $true } -ParameterFilter { $LiteralPath -eq "$env:ProgramFiles\Git\bin\bash.exe" }

        $found = Find-Bash

        $found | Should -Not -Be 'C:\Windows\System32\bash.exe'
        $found | Should -Be "$env:ProgramFiles\Git\bin\bash.exe"
    }

    It 'rejects the Sysnative spelling of the same launcher' {
        # A 32-bit PowerShell sees the real System32 as Sysnative; same file,
        # different prefix, same wrong answer if accepted.
        Mock Get-Command { [pscustomobject]@{ Source = 'C:\Windows\Sysnative\bash.exe' } } -ParameterFilter { $Name -eq 'bash' }
        Mock Get-Command { $null } -ParameterFilter { $Name -eq 'git' }
        Mock Test-Path { $false }
        Mock Test-Path { $true } -ParameterFilter { $LiteralPath -eq "$env:ProgramFiles\Git\bin\bash.exe" }

        Find-Bash | Should -Be "$env:ProgramFiles\Git\bin\bash.exe"
    }

    It 'still prefers a real bash the user put on PATH' {
        # The guard must not over-reject: PATH-first is the wrapper's
        # documented preference order, and only the two Windows system
        # directories are suspect.
        Mock Get-Command { [pscustomobject]@{ Source = 'C:\Program Files\Git\bin\bash.exe' } } -ParameterFilter { $Name -eq 'bash' }

        Find-Bash | Should -Be 'C:\Program Files\Git\bin\bash.exe'
    }

    It 'returns $null when the only bash is the WSL launcher, so the remedy message can print' {
        # The second half of the defect: the old code''s failure was not just
        # the wrong bash but the SILENCED remedy — a found-but-unusable bash
        # meant neither "install Git for Windows" nor "run from WSL" was ever
        # shown. $null is what routes the wrapper to that message.
        Mock Get-Command { [pscustomobject]@{ Source = 'C:\Windows\System32\bash.exe' } } -ParameterFilter { $Name -eq 'bash' }
        Mock Get-Command { $null } -ParameterFilter { $Name -eq 'git' }
        Mock Test-Path { $false }

        Find-Bash | Should -BeNullOrEmpty
    }
}

Describe 'Test-WslBashLauncher' {
    BeforeEach {
        $script:savedSystemRoot = $env:SystemRoot
        $env:SystemRoot = 'C:\Windows'
    }
    AfterEach {
        $env:SystemRoot = $script:savedSystemRoot
    }

    It 'matches the launcher case-insensitively and across separator spellings' {
        Test-WslBashLauncher 'C:\WINDOWS\system32\bash.exe' | Should -BeTrue
        Test-WslBashLauncher 'C:/Windows/System32/bash.exe' | Should -BeTrue
    }

    It 'does not condemn a bash that merely lives under a similar name' {
        Test-WslBashLauncher 'C:\Windows\System32Tools\bash.exe' | Should -BeFalse
        Test-WslBashLauncher 'C:\Program Files\Git\bin\bash.exe' | Should -BeFalse
    }

    It 'declines to judge when SystemRoot is not set rather than guessing one' {
        $env:SystemRoot = $null
        Test-WslBashLauncher 'C:\Windows\System32\bash.exe' | Should -BeFalse
    }
}

Describe 'install-git-hook.ps1 executed as a script' {
    # The mocked blocks above never run the wrapper's main body; this one
    # does, so that "CI executes the PowerShell path" is a fact about a
    # process that started and exited, not about a file that parses. The
    # assertions are exit codes and file hashes, not message text.
    BeforeEach {
        $script:repo = Join-Path ([System.IO.Path]::GetTempPath()) ("dossierx-ps1-" + [guid]::NewGuid().ToString('n'))
        New-Item -ItemType Directory -Path $script:repo | Out-Null
        git -C $script:repo init -q . 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "git init failed; this suite needs a real git" }
    }
    AfterEach {
        if ($script:repo -and (Test-Path -LiteralPath $script:repo)) {
            Remove-Item -LiteralPath $script:repo -Recurse -Force
        }
    }

    It 'installs, through a bash Find-Bash actually found, the same bytes the sh installer installs' {
        Push-Location $script:repo
        try {
            & $script:wrapper --yes 2>&1 | Out-Null
            $LASTEXITCODE | Should -Be 0
        }
        finally { Pop-Location }
        $viaWrapper = Join-Path $script:repo '.git/hooks/pre-commit'
        Test-Path -LiteralPath $viaWrapper | Should -BeTrue

        # The reference install, made by handing the sh script to the same
        # bash directly — if the two files differ, the wrapper altered what it
        # only exists to launch.
        $bash = Find-Bash
        $bash | Should -Not -BeNullOrEmpty
        $ref = Join-Path ([System.IO.Path]::GetTempPath()) ("dossierx-sh-" + [guid]::NewGuid().ToString('n'))
        New-Item -ItemType Directory -Path $ref | Out-Null
        try {
            git -C $ref init -q . 2>&1 | Out-Null
            Push-Location $ref
            try {
                & $bash $script:shInstaller --yes 2>&1 | Out-Null
                $LASTEXITCODE | Should -Be 0
            }
            finally { Pop-Location }
            $viaSh = Join-Path $ref '.git/hooks/pre-commit'
            (Get-FileHash -LiteralPath $viaWrapper -Algorithm SHA256).Hash |
                Should -Be (Get-FileHash -LiteralPath $viaSh -Algorithm SHA256).Hash
        }
        finally {
            Remove-Item -LiteralPath $ref -Recurse -Force
        }
    }

    It 'hands the sh installer its own invocation, so printed recoveries name this wrapper' {
        # The defect this pins: every recovery install-git-hook.sh prints is
        # built from how it was invoked, and a reader who came through this
        # wrapper is standing in PowerShell — possibly with no bash on PATH at
        # all — so a `sh ...` line is an instruction that does not run for
        # them. The wrapper exports DOSSIERX_HOOK_INVOCATION naming itself;
        # --help is the cheapest output that interpolates it (the usage line),
        # so that is what is asserted: the wrapper's own file name appears,
        # and the repository-relative usage line it used to print does not.
        Push-Location $script:repo
        try {
            $help = (& $script:wrapper --help 2>&1) -join "`n"
            $LASTEXITCODE | Should -Be 0
        }
        finally { Pop-Location }
        $help | Should -Match ([regex]::Escape('install-git-hook.ps1'))
        $help | Should -Not -Match ([regex]::Escape('  scripts/install-git-hook.sh [options]'))
    }

    It 'propagates the sh installer''s exit code instead of laundering it to 0' {
        Push-Location $script:repo
        try {
            # An unknown option makes install-git-hook.sh die with exit 1; a
            # wrapper that swallowed that would report every failed install
            # as a success, which is the quietest possible defect.
            & $script:wrapper --no-such-option 2>&1 | Out-Null
            $LASTEXITCODE | Should -Be 1
        }
        finally { Pop-Location }
    }
}

Describe 'the no-bash remedy' {
    # The finding this pins (wsl-remedy-names-a-windows-path): the message
    # whose first sentence explains that WSL's bash cannot open a C:\ path
    # used to end by offering `bash "C:\...\install-git-hook.sh" --yes` — the
    # resolved WINDOWS path, i.e. the exact command that sentence rules out.
    # The remedy must translate the path where it is used, with wslpath inside
    # the WSL invocation, because only the distro knows its own mounts.
    #
    # This is a real execution of the wrapper's no-bash branch, not a mock:
    # the child process is handed a PATH that resolves neither bash nor git
    # and Program Files roots pointing at an empty directory, so Find-Bash
    # returns $null for real and the branch that prints the remedy runs.
    # What this CANNOT prove is that the printed line succeeds inside an
    # actual WSL distro — GitHub-hosted Windows runners cannot run one (the
    # header above says why), and the wrapper's own message states the same
    # limit (a drive the distro does not mount). What it proves is the shape:
    # the line bash is handed is a wslpath translation carrying the real
    # resolved installer path, and never the bare Windows spelling the
    # message itself rules out.
    It 'hands WSL a wslpath translation, not the Windows spelling of the path' {
        # The running PowerShell, by absolute path, so the child starts even
        # though the PATH it inherits resolves nothing.
        $pwshExe = (Get-Process -Id $PID).Path
        $empty = Join-Path ([System.IO.Path]::GetTempPath()) ("dossierx-nobash-" + [guid]::NewGuid().ToString('n'))
        New-Item -ItemType Directory -Path $empty | Out-Null
        $saved = @{
            PATH                = $env:PATH
            ProgramFiles        = $env:ProgramFiles
            'ProgramFiles(x86)' = ${env:ProgramFiles(x86)}
            LOCALAPPDATA        = $env:LOCALAPPDATA
        }
        try {
            # Every location Find-Bash consults, emptied: PATH for the
            # Get-Command lookups (bash AND git, so the exec-path branch is
            # skipped too), the three install roots for the candidate list.
            $env:PATH = $empty
            $env:ProgramFiles = $empty
            ${env:ProgramFiles(x86)} = $empty
            $env:LOCALAPPDATA = $empty
            # ASSERT THE PRECONDITION BEFORE TRUSTING THE RUN. Find-Bash is
            # dot-sourced in BeforeAll, so it can be asked, in this very
            # environment, whether it still finds a bash. The first CI run of
            # this test failed as `Expected 1, but got 0` — the wrapper
            # installed instead of refusing — and that message says only that
            # the branch under test never ran, not WHY. A test that cannot
            # build its own condition must say what it found, or the next
            # reader is left guessing at a runner they cannot see.
            # ASK THE CHILD, NOT ONLY THIS PROCESS. The parent's answer and the
            # child's disagreed once already: Find-Bash returned $null here
            # while a child started from the same environment installed
            # successfully. Whatever a fresh pwsh reaches that this runspace
            # does not is the thing this test has to blank, so the probe runs
            # in a child of the same shape as the one under test.
            # THE CHILD SETS ITS OWN ENVIRONMENT, because inheriting it does not
            # work here. Measured on the runner: with all four names blanked in
            # this process, a child still answered
            # FOUND:C:\Program Files\Git\bin\bash.exe — the real Program Files,
            # not the empty directory. Whatever restores it between processes,
            # the fix is not to guess at the mechanism but to stop depending on
            # it: the child is handed the blanking as code it runs itself.
            $blank = "`$env:PATH='$empty'; `$env:ProgramFiles='$empty'; " +
                "`${env:ProgramFiles(x86)}='$empty'; `$env:LOCALAPPDATA='$empty'; "
            $probe = (& $pwshExe -NoProfile -Command (
                $blank + ". '$script:wrapper'; `$b = Find-Bash; if (`$b) { 'FOUND:' + `$b } else { 'NULL' }"
            ) 2>&1) -join "`n"
            $stillFound = Find-Bash
            if ($probe -notmatch 'NULL') {
                throw ("the no-bash environment is not bash-free IN A CHILD, which is the process " +
                    "shape under test. A child of this environment answered: " + $probe + "`n" +
                    "PATH, ProgramFiles, ProgramFiles(x86) and LOCALAPPDATA were all pointed at an " +
                    "empty directory, so that path names a location Find-Bash consults which this " +
                    "test does not blank. Blank it here rather than deleting the assertion below: " +
                    "a remedy nobody can reach is the finding this test pins.")
            }
            if ($stillFound) {
                throw ("the no-bash environment is not bash-free: Find-Bash still returned '" +
                    $stillFound + "'. PATH, ProgramFiles, ProgramFiles(x86) and LOCALAPPDATA were " +
                    "all pointed at an empty directory, so this is a location Find-Bash consults " +
                    "that this test does not blank. Blank it here rather than deleting the " +
                    "assertion below: a remedy nobody can reach is the finding this test pins.")
            }
            $out = (& $pwshExe -NoProfile -Command (
                $blank + "& '$script:wrapper' --yes"
            ) 2>&1) -join "`n"
            $exit = $LASTEXITCODE
        }
        finally {
            $env:PATH = $saved.PATH
            $env:ProgramFiles = $saved.ProgramFiles
            ${env:ProgramFiles(x86)} = $saved.'ProgramFiles(x86)'
            $env:LOCALAPPDATA = $saved.LOCALAPPDATA
            Remove-Item -LiteralPath $empty -Recurse -Force
        }

        # No bash means no install: the branch under test is the failing one,
        # and a 0 here would mean the child found a bash and this test proved
        # nothing about the remedy — a pass over the wrong branch, not a pass.
        #
        # THE CHILD'S OUTPUT IS IN THE FAILURE, because the parent's Find-Bash
        # already answered $null above and the child still installed: the two
        # processes disagree about what is reachable, and only the child can
        # say what it found. Without this the message is `Expected 1, but got
        # 0` twice over, which names the symptom and hides the cause.
        if ($exit -ne 1) {
            throw ("the wrapper exited $exit, not 1, though Find-Bash answered `$null in this " +
                "very environment moments earlier — so the child process reaches a bash the " +
                "parent does not. What the child printed:`n" + $out)
        }

        # The remedy's bash argument is a command substitution over wslpath,
        # carrying the same resolved installer path the wrapper computed...
        $out | Should -Match ([regex]::Escape('bash "$(wslpath '''))
        $out | Should -Match ([regex]::Escape("wslpath '$script:shInstaller'"))
        # ...and nowhere does bash get a bare drive-letter path as its direct
        # argument — the exact shape the finding's reader pasted into WSL and
        # watched fail with "No such file or directory".
        $out | Should -Not -Match 'bash\s+"[A-Za-z]:[\\/]'
    }
}
