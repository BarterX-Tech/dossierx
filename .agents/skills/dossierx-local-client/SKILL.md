---
name: dossierx-local-client
description: Verify an unreleased local DossierX checkout or worktree against a client DossierX project when the user supplies a source path or asks to test a library change before release. Do not use for publishing releases or permanently changing a client's dependency pin.
---

# DossierX local client verification

Use the exact DossierX source tree the user names while keeping the client's
committed release pin unchanged. The source may be a main checkout or a feature
worktree, and it may contain uncommitted changes.

The deterministic runner is
[`scripts/run-local-client.sh`](scripts/run-local-client.sh), relative to this
skill directory. It creates a temporary Go workspace, applies a local module
replacement there, runs the client's existing `go tool dossierx`, and removes
the workspace. Do not recreate that mechanism with a persistent `go.work`, a
client `replace` directive, `go install`, or a binary found on `PATH`.

## Resolve the two paths

Treat the path supplied by the user as `dossierx_source`. Resolve it to an
absolute path and verify all of these before running anything:

- `go.mod` declares `module github.com/BarterX-Tech/dossierx`;
- `cmd/dossierx` exists;
- the Git branch, HEAD commit, upstream if any, and dirty state are known.

Use an explicit client path when the user supplies one. Otherwise, start from
the current working directory and choose the nearest directory that contains
both `go.mod` and `project.config.yaml`. Do not scan unrelated directories on
the machine. If there is not exactly one reasonable client module, ask for its
path rather than guessing.

The client `go.mod` must declare:

```go
tool github.com/BarterX-Tech/dossierx/cmd/dossierx
```

Record the client's initial Git status and the hashes of `go.mod` and `go.sum`
when present. Existing dirty files belong to the user.

## Run the local source

Invoke the runner from this skill directory:

```sh
<skill-dir>/scripts/run-local-client.sh \
  <dossierx-source> <client-module> \
  check --validate --format json
```

If the user only says “verify this client,” the read-only check above is the
minimum run. Parse the JSON envelope and report `ok`, lint errors, warnings,
ledger findings, and relevant next steps. A warning is not an error, and an
existing client finding is not automatically a regression in the library.

The generic check proves that this engine tree can read and validate this
client corpus. It does not by itself prove the feature that changed. Inspect the
source diff and the user's stated feature, then add the smallest observable
client scenario that reaches that behavior:

- For viewer or live-rendering changes, run `serve` through the same runner and
  inspect the real served page with an available browser tool. Stop the server
  when the check is complete.
- For generated catalog or viewer output, use write-mode `check` only when the
  user asked to regenerate the real client. Otherwise use a disposable client
  copy and state that it is a copy.
- For claim, ledger, comment, lock, or other state-changing behavior, do not
  mutate the real client merely to test the library. Use a disposable fixture
  or client copy unless the user separately authorized the real operation.
- Run relevant DossierX repository tests for the changed engine behavior. Keep
  those results separate from client compatibility evidence.

Do not silently broaden “verify” into fixing client claims, refreshing build
orders, editing dependencies, exporting skills, or publishing anything.

## Close the proof

After the run, verify that the client's `go.mod` and `go.sum` hashes are
unchanged, no `go.work` or `go.work.sum` appeared, and pre-existing working-tree
state is preserved. Report:

- exact DossierX source path, commit, branch, and dirty state;
- exact client module path;
- commands and observable outcomes;
- whether a real served or generated artifact was inspected;
- client files changed by the DossierX command, if any;
- what the evidence does and does not establish.

The runner prints the source identity to stderr. Treat that line as execution
evidence; `dossierx version` from a source build may still show the unstamped
development version and is not enough to identify the checkout.

If the user asks for client CI to test an unreleased revision, that is a
different, repository-changing mode. Push the DossierX revision first, then pin
its commit SHA with `go get -tool` on a temporary client integration branch.
Do not use a moving branch name, and do not make this change during an ordinary
local verification request.

The demonstrated workflow and its limits are recorded in
[the local client verification lesson](../../../docs/lessons/2026-09-10-local-client-source-verification.md).
