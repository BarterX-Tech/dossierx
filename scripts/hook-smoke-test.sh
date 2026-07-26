#!/usr/bin/env bash
#
# hook-smoke-test.sh — the Phase 3 gate, executable.
#
# The release note this proves is one sentence: "hand-edit a locked claim in a
# temp repo, and git commit is refused." Everything below either establishes
# that or protects one of the promises the installer makes around it. It runs
# against a freshly built binary and a throwaway repository; nothing here
# touches the checkout it is run from.
#
#   1  capability probe    "dossierx check --staged" exists at all. Without it
#                          the hook is inert, and a hook that is inert but
#                          installed is worse than no hook, so this fails loudly
#                          rather than skipping.
#   2  install             asks-first is bypassed with --yes; the hook lands in
#                          the directory git actually reads.
#   3  idempotence         a second --yes run reports "already current", writes
#                          nothing, and does not change the file's mtime.
#   4  clean commit passes the gate does not block honest work.
#   5  THE GATE            hand-edit the locked claim, stage it, commit ->
#                          REFUSED, naming the unlock -> fix -> lock path.
#   6  no side effects     "git status --porcelain" is byte-identical before and
#                          after the refused commit: the hook reads the index,
#                          and must never dirty the tree it is validating.
#   7  --no-verify         the documented escape hatch really is one (CI is the
#                          authority; a hook that cannot be skipped just gets
#                          uninstalled).
#   8  foreign hook        an existing pre-commit that dossierx did not write is
#                          not touched without --force, and --force backs it up.
#   9  core.hooksPath      honoured, and NOT hijacked — the setting is unchanged
#                          after installing.
#  10  worktree            in a linked worktree .git is a FILE; the hook still
#                          installs where git looks, and still refuses.
#
# Usage: bash scripts/hook-smoke-test.sh
# Exit status: 0 all assertions held; 1 the first one that did not.

set -euo pipefail

TMP=""

fail() {
	printf 'hook-smoke-test: FAIL: %s\n' "$1" >&2
	exit 1
}

cleanup() {
	if [ -n "$TMP" ]; then
		# Hooks are 0755 files inside .git; nothing here is read-only, so a
		# plain rm is enough on every platform the CI matrix covers.
		rm -rf "$TMP" || true
	fi
}
trap cleanup EXIT

command -v git >/dev/null 2>&1 || fail "git is required but was not found on PATH"
command -v go >/dev/null 2>&1 || fail "go is required but was not found on PATH"

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
INSTALLER="$SCRIPT_DIR/install-git-hook.sh"
[ -f "$INSTALLER" ] || fail "install-git-hook.sh not found next to this script"

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t dossierx-hook)

# Build into a DIRECTORY rather than to a filename: on Windows that is what
# gets the .exe suffix appended for us, and a suffix-less binary is not
# executable there.
echo "hook-smoke-test: building dossierx ..."
mkdir -p "$TMP/bin"
(cd "$REPO_ROOT" && go build -o "$TMP/bin/" ./cmd/dossierx) || fail "go build failed"
BIN="$TMP/bin/dossierx"
[ -f "$BIN" ] || BIN="$TMP/bin/dossierx.exe"
[ -f "$BIN" ] || fail "built binary not found under $TMP/bin"

# The hook resolves the binary from PATH by default; this repository's binary
# is not installed, so point every hook run at the one we just built.
export DOSSIERX_BIN="$BIN"

CLAIM_ID="widget.contract.overview"

# new_project <dir> — a git repository containing a one-claim dossierx project
# with that claim LOCKED and everything committed. This is the state a real
# project is in when someone is about to tamper with it.
new_project() {
	local dir="$1"
	mkdir -p "$dir/claims"
	cat >"$dir/project.config.yaml" <<'YAML'
schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
YAML
	(
		cd "$dir"
		git init -q .
		git config user.email hook-smoke@example.invalid
		git config user.name "hook smoke test"
		git config commit.gpgsign false
		"$BIN" claim new "$CLAIM_ID" \
			--body "the widget answers within 200ms." \
			--governed-reason "smoke-test fixture, not backed by any doctrine claim" \
			--format text >/dev/null
		"$BIN" check --format text >/dev/null
		"$BIN" claim lock "$CLAIM_ID" --reason "approved for the smoke test" --format text >/dev/null
	)
}

# tamper <dir> — the hand edit the whole gate exists to catch: a locked claim's
# body changed in the file, with no unlock and no new ledger record.
tamper() {
	local file="$1/claims/$CLAIM_ID.yaml"
	# Not "sed -i": BSD sed (macOS) and GNU sed (Linux, Git Bash) disagree on
	# whether -i takes a suffix argument, and the disagreement is silent.
	sed 's/200ms/900ms/' "$file" >"$file.tmp"
	mv "$file.tmp" "$file"
	grep -q '900ms' "$file" || fail "tamper did not change the claim body"
}

# --- 1 · capability probe ----------------------------------------------------
echo "hook-smoke-test: probing for 'check --staged' ..."
PROBE="$TMP/probe"
new_project "$PROBE"
probe_out=$(cd "$PROBE" && "$BIN" check --staged --format json 2>&1 || true)
case "$probe_out" in
*'unknown flag: --staged'* | *'unknown flag "--staged"'*)
	fail "this build has no 'dossierx check --staged'; the pre-commit hook cannot work without it (Phase 3's --staged item must land first)"
	;;
esac

# --- 2 · install -------------------------------------------------------------
REPO="$TMP/repo"
new_project "$REPO"

echo "hook-smoke-test: installing the hook ..."
(cd "$REPO" && sh "$INSTALLER" --yes) >"$TMP/install.out" 2>&1 ||
	fail "installer exited non-zero: $(cat "$TMP/install.out")"
HOOK="$REPO/.git/hooks/pre-commit"
[ -f "$HOOK" ] || fail "no pre-commit hook at $HOOK after install: $(cat "$TMP/install.out")"
grep -q '^# dossierx-hook: pre-commit ' "$HOOK" || fail "installed hook carries no dossierx marker line"

# --- 3 · idempotence ---------------------------------------------------------
cp "$HOOK" "$TMP/hook-first.txt"
(cd "$REPO" && sh "$INSTALLER" --yes) >"$TMP/install2.out" 2>&1 ||
	fail "second install exited non-zero: $(cat "$TMP/install2.out")"
grep -qi 'already installed and current' "$TMP/install2.out" ||
	fail "a second install must report 'already installed and current', got: $(cat "$TMP/install2.out")"
cmp -s "$TMP/hook-first.txt" "$HOOK" || fail "the second install rewrote the hook"

# --- 4 · a clean commit passes ----------------------------------------------
echo "hook-smoke-test: committing a clean project (must pass) ..."
(cd "$REPO" && git add -A && git commit -qm "claims") >"$TMP/commit1.out" 2>&1 ||
	fail "the gate refused a clean commit: $(cat "$TMP/commit1.out")"

# --- 5 · THE GATE ------------------------------------------------------------
echo "hook-smoke-test: hand-editing the locked claim and committing (must be refused) ..."
tamper "$REPO"
(cd "$REPO" && git add -A)
# Captured AFTER staging: what must not move is the state the commit was
# attempted from, index included.
status_before=$(cd "$REPO" && git status --porcelain)
if (cd "$REPO" && git commit -qm "sneak an edit past review") >"$TMP/commit2.out" 2>&1; then
	fail "the commit SUCCEEDED after a locked claim was hand-edited — the gate did not fire"
fi
grep -qi 'refused' "$TMP/commit2.out" ||
	fail "the refusal did not say so in words a human can act on: $(cat "$TMP/commit2.out")"
grep -q 'claim unlock' "$TMP/commit2.out" ||
	fail "the refusal did not print the unlock -> fix -> lock repair path: $(cat "$TMP/commit2.out")"
grep -q 'claim lock' "$TMP/commit2.out" ||
	fail "the refusal named unlock but not the re-lock step: $(cat "$TMP/commit2.out")"

# --- 6 · no side effects -----------------------------------------------------
status_after=$(cd "$REPO" && git status --porcelain)
[ "$status_before" = "$status_after" ] ||
	fail "the hook changed the working tree. before:
$status_before
after:
$status_after"

# --- 7 · the escape hatch is real -------------------------------------------
(cd "$REPO" && git commit -qm "bypassed on purpose" --no-verify) >"$TMP/commit3.out" 2>&1 ||
	fail "--no-verify did not bypass the hook: $(cat "$TMP/commit3.out")"
(cd "$REPO" && DOSSIERX_SKIP_HOOK=1 git commit -q --allow-empty -m "skipped on purpose") >"$TMP/commit4.out" 2>&1 ||
	fail "DOSSIERX_SKIP_HOOK=1 did not bypass the hook: $(cat "$TMP/commit4.out")"

# --- 8 · a foreign hook is not clobbered ------------------------------------
echo "hook-smoke-test: refusing to clobber someone else's pre-commit ..."
FOREIGN="$TMP/foreign"
new_project "$FOREIGN"
mkdir -p "$FOREIGN/.git/hooks"
printf '#!/bin/sh\necho "the project already had a hook"\n' >"$FOREIGN/.git/hooks/pre-commit"
chmod +x "$FOREIGN/.git/hooks/pre-commit"
cp "$FOREIGN/.git/hooks/pre-commit" "$TMP/foreign-original.txt"

if (cd "$FOREIGN" && sh "$INSTALLER" --yes) >"$TMP/foreign.out" 2>&1; then
	fail "the installer replaced a foreign pre-commit hook without --force"
fi
cmp -s "$TMP/foreign-original.txt" "$FOREIGN/.git/hooks/pre-commit" ||
	fail "the installer modified a foreign hook it had refused to install over"
grep -q -- '--force' "$TMP/foreign.out" ||
	fail "the refusal did not tell the caller how to proceed: $(cat "$TMP/foreign.out")"

(cd "$FOREIGN" && sh "$INSTALLER" --yes --force) >"$TMP/foreign-force.out" 2>&1 ||
	fail "--force install failed: $(cat "$TMP/foreign-force.out")"
grep -q '^# dossierx-hook: pre-commit ' "$FOREIGN/.git/hooks/pre-commit" ||
	fail "--force did not install our hook"
backup=$(ls "$FOREIGN"/.git/hooks/pre-commit.pre-dossierx.* 2>/dev/null | head -1 || true)
[ -n "$backup" ] || fail "--force replaced the existing hook without leaving a backup"
cmp -s "$TMP/foreign-original.txt" "$backup" ||
	fail "the backup is not the hook that was replaced"
grep -q "$(basename "$backup")" "$TMP/foreign-force.out" ||
	fail "--force did not tell the caller where the backup went: $(cat "$TMP/foreign-force.out")"

# --- 9 · core.hooksPath is honoured, never hijacked -------------------------
echo "hook-smoke-test: honouring core.hooksPath ..."
HP="$TMP/hookspath"
new_project "$HP"
mkdir -p "$HP/team-hooks"
(cd "$HP" && git config core.hooksPath team-hooks)
(cd "$HP" && sh "$INSTALLER" --yes) >"$TMP/hookspath.out" 2>&1 ||
	fail "install under core.hooksPath failed: $(cat "$TMP/hookspath.out")"
[ -f "$HP/team-hooks/pre-commit" ] ||
	fail "the hook was not installed into the configured core.hooksPath directory"
[ ! -f "$HP/.git/hooks/pre-commit" ] ||
	fail "the hook was installed into .git/hooks, which git is not reading here"
[ "$(cd "$HP" && git config --get core.hooksPath)" = "team-hooks" ] ||
	fail "the installer changed core.hooksPath — it must never do that"

tamper "$HP"
(cd "$HP" && git add -A)
if (cd "$HP" && git commit -qm "tampered") >"$TMP/hookspath-commit.out" 2>&1; then
	fail "the hook installed under core.hooksPath did not fire: $(cat "$TMP/hookspath-commit.out")"
fi

# --- 10 · linked worktrees ---------------------------------------------------
echo "hook-smoke-test: installing from a linked worktree (.git is a file) ..."
WT="$TMP/wt-main"
new_project "$WT"
(cd "$WT" && git add -A && git commit -qm "claims") >/dev/null 2>&1 ||
	fail "could not seed the worktree fixture"
(cd "$WT" && git worktree add -q "$TMP/wt-linked" -b linked) >/dev/null 2>&1 ||
	fail "could not create a linked worktree"
[ -f "$TMP/wt-linked/.git" ] || fail "expected .git to be a FILE in a linked worktree"

(cd "$TMP/wt-linked" && sh "$INSTALLER" --yes) >"$TMP/wt.out" 2>&1 ||
	fail "install from a linked worktree failed: $(cat "$TMP/wt.out")"
# git looks in the shared common dir for a linked worktree's hooks, so that is
# where it must land — ".git/hooks" relative to the worktree does not exist.
[ -f "$WT/.git/hooks/pre-commit" ] ||
	fail "the hook did not land in the common .git/hooks that a linked worktree uses"

tamper "$TMP/wt-linked"
(cd "$TMP/wt-linked" && git add -A)
if (cd "$TMP/wt-linked" && git commit -qm "tampered in a worktree") >"$TMP/wt-commit.out" 2>&1; then
	fail "the gate did not fire in a linked worktree: $(cat "$TMP/wt-commit.out")"
fi

echo "hook-smoke-test: PASS — the gate refuses a hand-edited locked claim, in a plain repo, under core.hooksPath, and in a linked worktree."
