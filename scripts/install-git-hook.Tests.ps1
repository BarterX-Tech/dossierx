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
# The end-to-end block at the bottom is the complementary half: it executes
# the wrapper AS A SCRIPT, for real, against a throwaway repository, so the
# main body (path resolution, argument pass-through, exit-code propagation) is
# run by a machine and not only read by one. It needs a real git and a real
# bash, both of which every runner in the hooks matrix has.
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
