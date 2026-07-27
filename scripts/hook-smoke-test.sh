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
#  11  monorepo            a repository holding TWO dossierx projects: an honest
#                          commit still passes, and a tampered locked claim in
#                          EITHER project is refused. This one is a regression
#                          test: the v1 hook printed "set DOSSIERX_CONFIG",
#                          fell through with no config, got config_not_found
#                          from the binary and treated it as "no project here"
#                          — so the gate was OFF in exactly this layout.
#  12  DOSSIERX_CONFIG     narrowing the hook to one project still gates it.
#  13  root + subdir       the same fail-open in its second shape: a project AT
#                          the repository root plus one in a subdirectory.
#                          Discovery that short-circuits on the root config
#                          checks the root project and silently ignores the
#                          rest, so both are tampered here in turn.
#  14  unstaged config     the gate judges the INDEX, so every input it reads
#                          must come from the index — project.config.yaml
#                          included. An UNSTAGED "claims_dir:" edit pointing at
#                          an empty directory enumerated zero claims, so the
#                          audit ran over nothing and reported nothing: one
#                          uncommitted line disarmed the whole hook.
#  15  assume-unchanged    "git diff" deliberately omits paths carrying the
#                          assume-unchanged / skip-worktree bit, so a gate that
#                          uses the diff to decide "index == worktree" reads the
#                          WORKTREE copy of exactly the paths git was told to
#                          stop looking at — judging bytes nobody is committing
#                          while the tampered index blob goes in.
#  16  non-ASCII path      core.quotepath defaults to TRUE, so git prints a
#                          tracked path holding any non-ASCII byte C-quoted,
#                          double quotes included. Hand that string to
#                          "--config" and it names no file, the binary answers
#                          config_not_found, and the hook refuses — every
#                          commit, forever, in any repository whose project sits
#                          under an accented, CJK or emoji directory name. The
#                          case therefore asserts BOTH halves: an honest commit
#                          (and a commit touching no claim at all) still passes,
#                          and a tampered locked claim is still refused.
#  17  split claims_dir    a claims_dir that points OUTSIDE the config's own
#                          directory ("claims_dir: ../claims") is an ordinary
#                          monorepo layout; --staged used to read it as "outside
#                          the repository, nothing to evaluate" and pass.
#  18  untracked config    an UNTRACKED project.config.yaml must not be allowed
#                          to judge tracked claims: it is a worktree file,
#                          editable without staging anything.
#  19  skipped gate        "check --staged" exits 0 with data.skipped when it had
#                          no index to judge. In a hook that is not a pass.
#  20  scope, one half at  the two single-tree defences that survive: git rm the
#      a time              ledger alone is refused (lock-ledger-absent), and
#                          repointing claims_dir away from its claims alone is
#                          refused (lock-ledger-abandoned). The two files defend
#                          each other. See the case for what is NOT asserted
#                          here any more, and why.
#  21  sanctioned move     the case that keeps 20 from being a trap: a claims_dir
#                          move that TAKES ITS CLAIMS WITH IT still commits, and
#                          the gate is still armed against a hand-edited locked
#                          claim afterwards.
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

# add_project <repo> <subdir> — the same one-claim, one-locked-claim project as
# new_project, but placed INSIDE an existing repository rather than owning one.
# Used to build the monorepo fixture; the project's own directory is where
# dossierx is run from, since config discovery searches upward.
add_project() {
	local repo="$1" sub="$2"
	mkdir -p "$repo/$sub/claims"
	cat >"$repo/$sub/project.config.yaml" <<'YAML'
schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
YAML
	(
		cd "$repo/$sub"
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

# --- 11 · a repository with TWO projects is still gated ----------------------
#
# The regression this pins: the v1 hook could not choose between two configs, so
# it printed a note and ran the binary with no --config at all. The binary
# searched upward from the repository root, found nothing, and returned
# config_not_found — which the hook read as "no dossierx project here, skip".
# A monorepo therefore had NO gate, and said nothing about it. The hook now
# checks every project the index knows about.
echo "hook-smoke-test: gating a repository that holds two dossierx projects ..."
MULTI="$TMP/multi"
mkdir -p "$MULTI"
(
	cd "$MULTI"
	git init -q .
	git config user.email hook-smoke@example.invalid
	git config user.name "hook smoke test"
	git config commit.gpgsign false
)
add_project "$MULTI" p1
add_project "$MULTI" p2
(cd "$MULTI" && sh "$INSTALLER" --yes) >"$TMP/multi-install.out" 2>&1 ||
	fail "install into a multi-project repository failed: $(cat "$TMP/multi-install.out")"

# An honest commit must still pass: refusing everything in a monorepo is the
# other way to have no gate, because that hook gets uninstalled.
(cd "$MULTI" && git add -A && git commit -qm "two projects") >"$TMP/multi-commit1.out" 2>&1 ||
	fail "the gate refused an honest commit in a two-project repository: $(cat "$TMP/multi-commit1.out")"

for proj in p1 p2; do
	tamper "$MULTI/$proj"
	(cd "$MULTI" && git add -A)
	if (cd "$MULTI" && git commit -qm "sneak an edit past review in $proj") >"$TMP/multi-commit-$proj.out" 2>&1; then
		fail "a tampered locked claim in $proj was committed — the hook is not checking every project in the repository: $(cat "$TMP/multi-commit-$proj.out")"
	fi
	grep -qi 'refused' "$TMP/multi-commit-$proj.out" ||
		fail "the $proj refusal did not say so in words a human can act on: $(cat "$TMP/multi-commit-$proj.out")"
	# "skipping the claim-integrity check" is the exact wrong answer here: it is
	# what the v1 hook printed, and it is a lie in a repository that has two.
	if grep -q 'skipping the claim-integrity check' "$TMP/multi-commit-$proj.out"; then
		fail "the hook reported no project in a repository containing two: $(cat "$TMP/multi-commit-$proj.out")"
	fi
	# Undo the tamper so the next project is tested on its own — from HEAD, not
	# from the index. The tamper was STAGED a moment ago, so "git checkout --"
	# would restore the tampered content over the tampered content and leave
	# this project dirty for every iteration that follows. The next project's
	# refusal would then be this project's refusal, and the loop would pass
	# without ever proving the second project is checked at all.
	(cd "$MULTI" && git checkout HEAD -- "$proj/claims/$CLAIM_ID.yaml" && git add -A)
done

# --- 12 · DOSSIERX_CONFIG narrows the hook, and still gates ------------------
tamper "$MULTI/p1"
(cd "$MULTI" && git add -A)
if (cd "$MULTI" && DOSSIERX_CONFIG=p1/project.config.yaml git commit -qm "tampered, narrowed") >"$TMP/multi-commit-cfg.out" 2>&1; then
	fail "DOSSIERX_CONFIG pointed at the tampered project and the commit still passed: $(cat "$TMP/multi-commit-cfg.out")"
fi
grep -qi 'refused' "$TMP/multi-commit-cfg.out" ||
	fail "the narrowed refusal did not say so: $(cat "$TMP/multi-commit-cfg.out")"

# --- 13 · a project at the ROOT plus a project in a subdirectory -------------
#
# The second shape of the same fail-open, and the one case 11 cannot reach. When
# discovery short-circuits on "is there a project.config.yaml at the repository
# root?", a root project answers yes and every OTHER project in the repository
# is silently never checked — so a tampered locked claim in the subdirectory
# commits cleanly while the hook reports nothing at all. Discovery therefore
# asks the index first and unconditionally; the root worktree file is only a
# fallback for a project whose config is not tracked yet.
echo "hook-smoke-test: gating a repository whose projects are at the root AND in a subdirectory ..."
ROOTSUB="$TMP/rootsub"
mkdir -p "$ROOTSUB"
(
	cd "$ROOTSUB"
	git init -q .
	git config user.email hook-smoke@example.invalid
	git config user.name "hook smoke test"
	git config commit.gpgsign false
)
add_project "$ROOTSUB" .
add_project "$ROOTSUB" sub
(cd "$ROOTSUB" && sh "$INSTALLER" --yes) >"$TMP/rootsub-install.out" 2>&1 ||
	fail "install into a root-plus-subdirectory repository failed: $(cat "$TMP/rootsub-install.out")"
(cd "$ROOTSUB" && git add -A && git commit -qm "root project and a subdirectory project") >"$TMP/rootsub-commit1.out" 2>&1 ||
	fail "the gate refused an honest commit in a root-plus-subdirectory repository: $(cat "$TMP/rootsub-commit1.out")"

for proj in . sub; do
	tamper "$ROOTSUB/$proj"
	(cd "$ROOTSUB" && git add -A)
	if (cd "$ROOTSUB" && git commit -qm "sneak an edit past review in $proj") >"$TMP/rootsub-commit.out" 2>&1; then
		fail "a tampered locked claim in '$proj' was committed — discovery short-circuited on the root config and never checked the rest: $(cat "$TMP/rootsub-commit.out")"
	fi
	grep -qi 'refused' "$TMP/rootsub-commit.out" ||
		fail "the '$proj' refusal did not say so in words a human can act on: $(cat "$TMP/rootsub-commit.out")"
	# From HEAD, not the index: the tamper is staged, so "git checkout --" would
	# restore it onto itself and the subdirectory project would then be tested
	# against a repository whose root project is still tampered.
	(cd "$ROOTSUB" && git checkout HEAD -- "$proj/claims/$CLAIM_ID.yaml" && git add -A)
done

# --- 14 · an UNSTAGED project.config.yaml must not disarm the gate -----------
#
# "check --staged" judges the git index, and the argument for that is only worth
# something if EVERY input comes from the index. project.config.yaml is the one
# that decides which claims exist at all: point claims_dir at an empty directory
# and the run enumerates zero claims, audits zero claims, finds zero findings and
# exits 0 — a green gate over a commit that carries a hand-edited locked claim.
#
# The edit is deliberately left UNSTAGED here, because that is the shape that
# made this fail silently: the committed config still says claims_dir: claims,
# the commit itself carries no config change at all, and nothing in the run
# mentions that the file it read is not the file being committed.
echo "hook-smoke-test: an unstaged claims_dir edit must not disarm the gate ..."
CFGSWAP="$TMP/cfgswap"
new_project "$CFGSWAP"
(cd "$CFGSWAP" && sh "$INSTALLER" --yes) >"$TMP/cfgswap-install.out" 2>&1 ||
	fail "install into the config-swap fixture failed: $(cat "$TMP/cfgswap-install.out")"
(cd "$CFGSWAP" && git add -A && git commit -qm "claims") >"$TMP/cfgswap-commit1.out" 2>&1 ||
	fail "the gate refused a clean commit in the config-swap fixture: $(cat "$TMP/cfgswap-commit1.out")"

tamper "$CFGSWAP"
(cd "$CFGSWAP" && git add -A)
# Redirect claims_dir at an empty directory, in the WORKING TREE only. Same sed
# dance as tamper(), for the same BSD/GNU reason.
mkdir -p "$CFGSWAP/decoy"
sed 's/^claims_dir: claims$/claims_dir: decoy/' "$CFGSWAP/project.config.yaml" >"$CFGSWAP/project.config.yaml.tmp"
mv "$CFGSWAP/project.config.yaml.tmp" "$CFGSWAP/project.config.yaml"
grep -q 'claims_dir: decoy' "$CFGSWAP/project.config.yaml" ||
	fail "the claims_dir redirect did not take"
# The fixture only means something while that edit is unstaged and the INDEX
# still says claims_dir: claims.
(cd "$CFGSWAP" && git diff --name-only -- project.config.yaml) | grep -q 'project.config.yaml' ||
	fail "the claims_dir edit was staged; this case must leave it in the working tree only"
(cd "$CFGSWAP" && git show :project.config.yaml) | grep -q 'claims_dir: claims' ||
	fail "the indexed config no longer says claims_dir: claims; the fixture is not testing what it claims to"

if (cd "$CFGSWAP" && git commit -qm "sneak an edit past review behind a claims_dir swap") >"$TMP/cfgswap-commit2.out" 2>&1; then
	fail "a tampered locked claim was committed after an UNSTAGED claims_dir edit — the gate read project.config.yaml from the working tree while reading claims from the index: $(cat "$TMP/cfgswap-commit2.out")"
fi
grep -qi 'refused' "$TMP/cfgswap-commit2.out" ||
	fail "the refusal did not say so in words a human can act on: $(cat "$TMP/cfgswap-commit2.out")"

# --- 15 · assume-unchanged must not substitute the worktree for the index ----
#
# git deliberately omits a path carrying the assume-unchanged (or skip-worktree)
# bit from "git diff", which is fine for git and fatal for anything that uses the
# diff as a proxy for "the worktree file IS the index content". Set the bit on a
# claim whose INDEX blob is tampered and whose worktree copy is pristine, and a
# diff-driven gate reads the pristine copy, approves it, and lets the tampered
# blob commit — the exact substitution --staged exists to prevent.
#
# The control run comes first, without the bit, so a refusal in the second run
# cannot be mistaken for the fixture simply being broken.
echo "hook-smoke-test: assume-unchanged must not swap the worktree in for the index ..."
AU="$TMP/assume-unchanged"
new_project "$AU"
(cd "$AU" && sh "$INSTALLER" --yes) >"$TMP/au-install.out" 2>&1 ||
	fail "install into the assume-unchanged fixture failed: $(cat "$TMP/au-install.out")"
(cd "$AU" && git add -A && git commit -qm "claims") >"$TMP/au-commit1.out" 2>&1 ||
	fail "the gate refused a clean commit in the assume-unchanged fixture: $(cat "$TMP/au-commit1.out")"

tamper "$AU"
(cd "$AU" && git add -A)
# Restore the WORKTREE copy only — "git checkout --" would restore the index
# too, and there would be nothing tampered left to catch.
(cd "$AU" && git show "HEAD:claims/$CLAIM_ID.yaml" >"claims/$CLAIM_ID.yaml")
if grep -q '900ms' "$AU/claims/$CLAIM_ID.yaml"; then
	fail "the worktree copy was supposed to be restored to the approved content"
fi
(cd "$AU" && git show ":claims/$CLAIM_ID.yaml") | grep -q '900ms' ||
	fail "the indexed claim is not tampered; the fixture is not testing what it claims to"

if (cd "$AU" && git commit -qm "tampered in the index only") >"$TMP/au-commit2.out" 2>&1; then
	fail "a claim tampered in the INDEX and pristine in the worktree was committed: $(cat "$TMP/au-commit2.out")"
fi

(cd "$AU" && git update-index --assume-unchanged "claims/$CLAIM_ID.yaml")
if (cd "$AU" && git diff --name-only) | grep -q "$CLAIM_ID"; then
	fail "this git still lists an assume-unchanged path in 'git diff'; the case cannot prove anything on this platform"
fi
if (cd "$AU" && git commit -qm "tampered, and hidden behind assume-unchanged") >"$TMP/au-commit3.out" 2>&1; then
	fail "assume-unchanged hid the tampered claim from the gate and the commit landed — the gate judged the worktree copy of a path git was told to stop looking at: $(cat "$TMP/au-commit3.out")"
fi
grep -qi 'refused' "$TMP/au-commit3.out" ||
	fail "the refusal did not say so in words a human can act on: $(cat "$TMP/au-commit3.out")"
# Leave the bit off, so cleanup and any later case see an ordinary repository.
(cd "$AU" && git update-index --no-assume-unchanged "claims/$CLAIM_ID.yaml")

# --- 16 · a project under a NON-ASCII directory name -------------------------
#
# git's core.quotepath defaults to true, and with it on, "git ls-files" renders
# any tracked path containing a byte outside ASCII as a C-quoted string — with
# the surrounding double quotes as literal characters of the output. A project at
# "café/project.config.yaml" comes back as the 30-character text
# "caf\303\251/project.config.yaml", quotes included. The hook's discovery loop
# then passes that to "dossierx --config", which names no file on disk, so the
# binary answers config_not_found — and a config the hook DISCOVERED and cannot
# open is a refusal, not a skip. The result is a hook that refuses every commit
# on every branch for every developer, including commits touching no claim at
# all, until somebody uninstalls it. The fix is one "-c core.quotepath=false" on
# the ls-files call, and this is the case that would have caught its absence.
#
# Both halves are asserted. "still refuses a tampered claim" alone would pass
# against a hook that refuses unconditionally, which is exactly the bug.
echo "hook-smoke-test: gating a project under a non-ASCII directory name ..."
UNI="$TMP/unicode"
NONASCII='café'
mkdir -p "$UNI"
(
	cd "$UNI"
	git init -q .
	git config user.email hook-smoke@example.invalid
	git config user.name "hook smoke test"
	git config commit.gpgsign false
)
add_project "$UNI" "$NONASCII"
(cd "$UNI" && git add -A)

# The fixture is only worth anything if the tracked path really does carry a
# non-ASCII byte. A filesystem or shell that transliterated the name away would
# leave a pure-ASCII path, and the case would then "pass" while proving nothing
# — so say so out loud rather than reporting a green that is not one. The probe
# asks with quotepath=false on purpose: it is testing the bytes git stored, not
# how git chooses to display them.
uni_tracked=$(cd "$UNI" && git -c core.quotepath=false ls-files -- '*/project.config.yaml' 2>/dev/null || true)
if ! printf '%s' "$uni_tracked" | LC_ALL=C grep -q '[^ -~]'; then
	fail "this platform did not preserve the non-ASCII directory name (git tracked '$uni_tracked'); the case cannot prove anything here"
fi

(cd "$UNI" && sh "$INSTALLER" --yes) >"$TMP/uni-install.out" 2>&1 ||
	fail "install into a non-ASCII-path repository failed: $(cat "$TMP/uni-install.out")"

# Half one: honest work still commits. This is the assertion that fails when the
# discovery query forgets quotepath=false.
(cd "$UNI" && git add -A && git commit -qm "a project under a non-ASCII path") >"$TMP/uni-commit1.out" 2>&1 ||
	fail "the gate refused an honest commit in a repository whose project lives under a non-ASCII path — the hook is almost certainly feeding dossierx a C-quoted path: $(cat "$TMP/uni-commit1.out")"

# The reported shape, exactly: a commit that touches no claim at all.
printf 'unrelated\n' >"$UNI/readme.txt"
(cd "$UNI" && git add readme.txt && git commit -qm "unrelated file, no claims touched") >"$TMP/uni-commit2.out" 2>&1 ||
	fail "the gate refused a commit that touches no claim at all, in a repository whose project lives under a non-ASCII path: $(cat "$TMP/uni-commit2.out")"

# Half two: the gate is still ON. A hook that reached "pass" by skipping the
# project it could not name would satisfy half one and nothing else.
tamper "$UNI/$NONASCII"
(cd "$UNI" && git add -A)
if (cd "$UNI" && git commit -qm "sneak an edit past review under a non-ASCII path") >"$TMP/uni-commit3.out" 2>&1; then
	fail "a tampered locked claim under a non-ASCII path was committed — the hook is not checking a project it cannot name: $(cat "$TMP/uni-commit3.out")"
fi
grep -qi 'refused' "$TMP/uni-commit3.out" ||
	fail "the non-ASCII refusal did not say so in words a human can act on: $(cat "$TMP/uni-commit3.out")"
if grep -q 'skipping the claim-integrity check' "$TMP/uni-commit3.out"; then
	fail "the hook reported no project in a repository that has one under a non-ASCII path: $(cat "$TMP/uni-commit3.out")"
fi

# --- 17 · claims_dir OUTSIDE the config's own directory ----------------------
#
# The ordinary monorepo layout: the config in docs/, the claims beside it rather
# than under it, "claims_dir: ../claims". Nothing in FORMAT.md or README requires
# claims_dir to sit under the config's directory, config validation does not
# reject "..", and check, claim new, claim lock and serve all work on it.
#
# --staged did not. Every pathspec was computed relative to the CONFIG's own
# directory, so claims_dir came out as "../claims", which the engine read as
# "outside the repository, nothing to evaluate" — its deliberate exit-0 escape
# hatch, meant for a tarball checkout with no git at all. The hook then printed
# nothing, exited 0, and committed a hand-edited LOCKED claim, while
# "check --validate" on the identical tree reported lock-content-drift. The
# entry point CI runs and the entry point the hook runs disagreed, and the one
# that disagreed was the hook's.
#
# Both halves are asserted, because "refuses" alone would also be satisfied by a
# hook that refuses this layout unconditionally.
echo "hook-smoke-test: gating a project whose claims_dir points outside its own directory ..."
SPLIT="$TMP/split-claims-dir"
mkdir -p "$SPLIT/docs" "$SPLIT/claims"
cat >"$SPLIT/docs/project.config.yaml" <<'YAML'
schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: ../claims
YAML
(
	cd "$SPLIT"
	git init -q .
	git config user.email hook-smoke@example.invalid
	git config user.name "hook smoke test"
	git config commit.gpgsign false
)
(
	cd "$SPLIT/docs"
	"$BIN" claim new "$CLAIM_ID" \
		--body "the widget answers within 200ms." \
		--governed-reason "smoke-test fixture, not backed by any doctrine claim" \
		--format text >/dev/null
	"$BIN" check --format text >/dev/null
	"$BIN" claim lock "$CLAIM_ID" --reason "approved for the smoke test" --format text >/dev/null
)
# The fixture only means something if the claim really did land outside the
# config's directory.
[ -f "$SPLIT/claims/$CLAIM_ID.yaml" ] ||
	fail "claims_dir: ../claims did not put the claim beside the config; the fixture cannot prove anything"

(cd "$SPLIT" && sh "$INSTALLER" --yes) >"$TMP/split-install.out" 2>&1 ||
	fail "install into the split-claims_dir fixture failed: $(cat "$TMP/split-install.out")"

# Half one: honest work still commits.
(cd "$SPLIT" && git add -A && git commit -qm "a project whose claims live beside its config") >"$TMP/split-commit1.out" 2>&1 ||
	fail "the gate refused an honest commit in a repository whose claims_dir points outside the config's directory: $(cat "$TMP/split-commit1.out")"

# Half two: the gate is ON.
tamper "$SPLIT"
(cd "$SPLIT" && git add -A)
if (cd "$SPLIT" && git commit -qm "sneak an edit past review behind a ../claims layout") >"$TMP/split-commit2.out" 2>&1; then
	fail "a tampered locked claim was committed in a claims_dir: ../claims layout — the gate skipped the whole project and said nothing: $(cat "$TMP/split-commit2.out")"
fi
grep -qi 'refused' "$TMP/split-commit2.out" ||
	fail "the refusal did not say so in words a human can act on: $(cat "$TMP/split-commit2.out")"
if grep -qi 'skipped\|skipping' "$TMP/split-commit2.out"; then
	fail "the hook reported a SKIPPED gate on a layout it can evaluate perfectly well: $(cat "$TMP/split-commit2.out")"
fi

# --- 18 · an UNTRACKED project.config.yaml over tracked claims ---------------
#
# Reading the config from the index (case 14) closed the unstaged-edit bypass,
# but it fell back to the WORKTREE config whenever the config was merely not
# TRACKED — and an untracked config is a working-tree file, editable without
# staging anything. Point its claims_dir at a directory of PRISTINE copies,
# stage a tampered locked claim, and the gate audited the copies, agreed with
# every ledger record, printed "check --staged: OK" and passed the commit: the
# same bypass as case 14, reached through the fallback instead of the file.
#
# The pristine copies are what make it silent. An EMPTY decoy leaves the ledger
# holding records for claims the gate can no longer see, and lock-ledger-
# abandoned refuses the commit for the wrong reason; a decoy that satisfies
# every record refuses nothing at all.
#
# The fallback exists for the first commit of a project, and it survives for
# exactly that: an index with no claims and no ledger in it. Here the index has
# both, so there is no honest reading of the run, and the refusal is a hard
# error rather than the skip-and-pass that hides it.
echo "hook-smoke-test: an untracked project.config.yaml must not judge tracked claims ..."
UNTRACKED="$TMP/untracked-config"
new_project "$UNTRACKED"
# The archive copy: ordinary-looking, tracked, and exactly what the ledger
# approved.
mkdir -p "$UNTRACKED/decoy"
cp "$UNTRACKED/claims/$CLAIM_ID.yaml" "$UNTRACKED/decoy/"
# Commit the claims and the ledger WITHOUT the config, before the hook exists —
# the hook would (correctly, after this fix) refuse to be the thing that creates
# this state.
(cd "$UNTRACKED" && git add claims decoy .dossierx-lock-store.json .dossierx-comment-digest.json && git commit -qm "claims, no config") >"$TMP/untracked-commit1.out" 2>&1 ||
	fail "could not build the untracked-config fixture: $(cat "$TMP/untracked-commit1.out")"
(cd "$UNTRACKED" && git ls-files --error-unmatch project.config.yaml) >/dev/null 2>&1 &&
	fail "the fixture tracked project.config.yaml; this case must leave it untracked"

(cd "$UNTRACKED" && sh "$INSTALLER" --yes) >"$TMP/untracked-install.out" 2>&1 ||
	fail "install into the untracked-config fixture failed: $(cat "$TMP/untracked-install.out")"

tamper "$UNTRACKED"
(cd "$UNTRACKED" && git add claims)
# The redirect, in the untracked config nobody is committing.
sed 's/^claims_dir: claims$/claims_dir: decoy/' "$UNTRACKED/project.config.yaml" >"$UNTRACKED/project.config.yaml.tmp"
mv "$UNTRACKED/project.config.yaml.tmp" "$UNTRACKED/project.config.yaml"
grep -q 'claims_dir: decoy' "$UNTRACKED/project.config.yaml" ||
	fail "the claims_dir redirect did not take"

if (cd "$UNTRACKED" && git commit -qm "sneak an edit past review behind an untracked config") >"$TMP/untracked-commit2.out" 2>&1; then
	fail "a tampered locked claim was committed while project.config.yaml was untracked — the gate judged tracked content against a worktree config: $(cat "$TMP/untracked-commit2.out")"
fi
grep -qi 'refused' "$TMP/untracked-commit2.out" ||
	fail "the refusal did not say so in words a human can act on: $(cat "$TMP/untracked-commit2.out")"
grep -q 'project.config.yaml' "$TMP/untracked-commit2.out" ||
	fail "the refusal did not name the untracked config, so nobody can act on it: $(cat "$TMP/untracked-commit2.out")"

# --- 19 · a SKIPPED gate is not a pass --------------------------------------
#
# "check --staged" exits 0 with data.skipped when there is no index to evaluate.
# That is deliberate — running it by hand outside a repository, or in CI over a
# tarball checkout, must not fail — and it stays. What must not stay is the hook
# treating it as a pass: check_one branched on the exit code and on error.code
# and on nothing else, and --format json puts the warning in the envelope rather
# than on the terminal, so a skipped run was indistinguishable from a clean one.
# No output, exit 0, commit lands, gate never ran.
#
# The reachable shape is claims_dir resolving OUTSIDE the repository altogether,
# where no commit could carry the claims and the engine genuinely has nothing to
# judge. Silence is the wrong answer there too: the hook is installed to guard
# these claims, and "I could not look at them" is a refusal, not an approval.
echo "hook-smoke-test: a SKIPPED gate must not be reported as a pass ..."
SKIPPED="$TMP/skipped-gate"
mkdir -p "$SKIPPED/repo" "$SKIPPED/outside-claims"
cat >"$SKIPPED/repo/project.config.yaml" <<'YAML'
schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: ../outside-claims
YAML
(
	cd "$SKIPPED/repo"
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
# The fixture only means something if the claims really are outside the repo.
[ -f "$SKIPPED/outside-claims/$CLAIM_ID.yaml" ] ||
	fail "the claims did not land outside the repository; the fixture cannot prove anything"
(cd "$SKIPPED/repo" && "$BIN" check --staged --format json) 2>&1 | grep -q '"skipped": true' ||
	fail "this fixture no longer produces a skipped run; the case cannot prove anything"

(cd "$SKIPPED/repo" && sh "$INSTALLER" --yes) >"$TMP/skipped-install.out" 2>&1 ||
	fail "install into the skipped-gate fixture failed: $(cat "$TMP/skipped-install.out")"

if (cd "$SKIPPED/repo" && git add -A && git commit -qm "claims live outside the repository") >"$TMP/skipped-commit.out" 2>&1; then
	fail "a commit passed on a SKIPPED gate — the hook reported OK over a project it never looked at: $(cat "$TMP/skipped-commit.out")"
fi
grep -qi 'refused' "$TMP/skipped-commit.out" ||
	fail "the skipped-gate refusal did not say so in words a human can act on: $(cat "$TMP/skipped-commit.out")"
grep -q 'claims_dir' "$TMP/skipped-commit.out" ||
	fail "the skipped-gate refusal did not point at the likely cause: $(cat "$TMP/skipped-commit.out")"
# And the documented escape hatch still gets the work committed, because a
# refusal nobody can get past is a refusal that gets the hook uninstalled.
(cd "$SKIPPED/repo" && git commit -qm "claims live outside the repository" --no-verify) >"$TMP/skipped-commit2.out" 2>&1 ||
	fail "--no-verify did not get past the skipped-gate refusal: $(cat "$TMP/skipped-commit2.out")"

# --- 20 · the SCOPE sabotages, ONE HALF AT A TIME ----------------------------
#
# WHAT THIS CASE USED TO ASSERT, AND WHY IT NO LONGER DOES. It used to stage the
# whole two-commit tamper in one go —
#
#	commit 1  claims_dir: claims -> archive (a tracked directory with no claims
#	          in it) AND git rm the lock ledger
#	commit 2  rewrite the now-unscoped locked claim freely
#
# — and require commit 1 to be refused by name (claims-scope-narrowed,
# integrity-store-removed). Those two rules came from "check --staged" comparing
# the commit being made against its PARENT, and that comparison has been REMOVED:
# the parent commit is outside the COMMIT but not outside the COMMITTER, so
# --orphan, a second config file or a rebase switched it off at will, while it
# refused two shapes of ordinary work — a plain "git revert" of a commit that
# contained a lock, and a NEW project in a monorepo audited against an unrelated
# retired project's ledger.
#
# So the combined single-commit tamper is now ACCEPTED, and that is a known,
# measured, deliberate cost. It is NOT asserted here in either direction:
# asserting the acceptance would pin the hole open and fail the day somebody
# closes it properly, and asserting the refusal would fail today.
#
# THE COST IS TWO DETECTIONS, NOT ONE. The other is the ERASED REVIEW: a DRAFT
# claim's comments: block deleted together with that claim's key in the digest
# store. It has the same conjunction shape and is pinned, with its own two
# half-tamper assertions, in internal/check/staged_no_parent_test.go — in Go
# rather than here, because it needs a comment thread and a digest store rather
# than a hook.
#
# WHAT IS ASSERTED HERE IS WHY THE SCOPE COLLAPSE COSTS ONE DETECTION AND NOT
# TWO. Neither half of the tamper works on its own, because the ledger and the
# claims defend each other inside a SINGLE tree, with no history involved:
#
#	20a  delete the ledger, leave claims_dir alone -> locked claims with no
#	     approval record anywhere            -> lock-ledger-absent
#	20b  repoint claims_dir, leave the ledger     -> approval records pointing at
#	     alone                                       claims nothing can see
#	                                              -> lock-ledger-abandoned
#
# If either of these ever stops refusing, the cost of the scope collapse is no
# longer one detection, and this case is the thing that says so.
echo "hook-smoke-test: refusing a deleted lock ledger (single tree) ..."
LEDGERRM="$TMP/scope-ledger-rm"
new_project "$LEDGERRM"
(cd "$LEDGERRM" && git add -A && git commit -qm "baseline") ||
	fail "could not commit the deleted-ledger fixture baseline"
(cd "$LEDGERRM" && sh "$INSTALLER" --yes) >"$TMP/ledgerrm-install.out" 2>&1 ||
	fail "install into the deleted-ledger fixture failed: $(cat "$TMP/ledgerrm-install.out")"

(cd "$LEDGERRM" && git rm -q .dossierx-lock-store.json) ||
	fail "could not stage the ledger deletion"
if (cd "$LEDGERRM" && git commit -qm "chore: drop the lock store") >"$TMP/ledgerrm-commit.out" 2>&1; then
	fail "a commit that deletes the lock ledger while locked claims remain was ACCEPTED — lock-ledger-absent did not fire: $(cat "$TMP/ledgerrm-commit.out")"
fi
grep -q 'lock-ledger-absent' "$TMP/ledgerrm-commit.out" ||
	fail "the refusal did not name lock-ledger-absent, so the single-tree half of the scope defence is not what refused: $(cat "$TMP/ledgerrm-commit.out")"

echo "hook-smoke-test: refusing a claims_dir that strands its claims (single tree) ..."
SCOPE="$TMP/scope"
new_project "$SCOPE"
mkdir -p "$SCOPE/archive"
printf 'an ordinary tracked directory with no claims in it\n' >"$SCOPE/archive/NOTES.md"
(cd "$SCOPE" && git add -A && git commit -qm "baseline") ||
	fail "could not commit the scope fixture baseline"
(cd "$SCOPE" && sh "$INSTALLER" --yes) >"$TMP/scope-install.out" 2>&1 ||
	fail "install into the scope fixture failed: $(cat "$TMP/scope-install.out")"

# Repoint claims_dir at the innocent tracked directory and LEAVE THE LEDGER
# ALONE. Its records now describe claims that are still in the repository, still
# reading status: locked, and outside everything the gate can see.
sed 's|claims_dir: claims|claims_dir: archive|' "$SCOPE/project.config.yaml" >"$SCOPE/project.config.yaml.tmp"
mv "$SCOPE/project.config.yaml.tmp" "$SCOPE/project.config.yaml"
(cd "$SCOPE" && git add -A) || fail "could not stage the claims_dir repoint"
if (cd "$SCOPE" && git commit -qm "chore: relocate claims") >"$TMP/scope-commit.out" 2>&1; then
	fail "a commit that repointed claims_dir away from its locked claims was ACCEPTED — lock-ledger-abandoned did not fire: $(cat "$TMP/scope-commit.out")"
fi
grep -q 'lock-ledger-abandoned' "$TMP/scope-commit.out" ||
	fail "the refusal did not name lock-ledger-abandoned, so the single-tree half of the scope defence is not what refused: $(cat "$TMP/scope-commit.out")"

# --- 21 · the SANCTIONED move ------------------------------------------------
#
# A gate with no way to reorganise a project is a trap, and a trap gets
# uninstalled. There is no flag and no escape hatch for this on purpose: a move
# that takes its claims with it strands nothing, so the rule has nothing to fire
# on. This is that flow, and it has to COMMIT.
echo "hook-smoke-test: a claims_dir move that takes its claims with it still commits ..."
MOVE="$TMP/scope-move"
new_project "$MOVE"
(cd "$MOVE" && git add -A && git commit -qm "baseline") ||
	fail "could not commit the sanctioned-move fixture baseline"
(cd "$MOVE" && sh "$INSTALLER" --yes) >"$TMP/move-install.out" 2>&1 ||
	fail "install into the sanctioned-move fixture failed: $(cat "$TMP/move-install.out")"

mkdir -p "$MOVE/docs"
(cd "$MOVE" && git mv claims docs/claims) || fail "git mv of the claims directory failed"
sed 's|claims_dir: claims|claims_dir: docs/claims|' "$MOVE/project.config.yaml" >"$MOVE/project.config.yaml.tmp"
mv "$MOVE/project.config.yaml.tmp" "$MOVE/project.config.yaml"
(cd "$MOVE" && git add -A && git commit -qm "chore: move claims under docs/") >"$TMP/scope-move.out" 2>&1 ||
	fail "the sanctioned claims_dir move was REFUSED — the scope guard is a trap: $(cat "$TMP/scope-move.out")"

# And the gate is still ARMED afterwards, rather than merely quiet: the moved
# claim is still being judged, so a hand edit to it is still refused. An
# "accepted" that silently audits nothing would be the very failure case 20
# exists for, reached through the sanctioned door.
(cd "$MOVE" && "$BIN" check --staged --format json) >"$TMP/scope-move-status.out" 2>&1 || true
grep -q '"skipped"' "$TMP/scope-move-status.out" &&
	fail "after the sanctioned move the gate evaluates nothing: $(cat "$TMP/scope-move-status.out")"
tamper "$MOVE/docs"
(cd "$MOVE" && git add -A) || fail "could not stage the post-move tamper"
if (cd "$MOVE" && git commit -qm "docs: tweak") >"$TMP/scope-move-tamper.out" 2>&1; then
	fail "after a sanctioned claims_dir move the gate stopped catching a hand-edited locked claim: $(cat "$TMP/scope-move-tamper.out")"
fi

echo "hook-smoke-test: PASS — the gate refuses a hand-edited locked claim, in a plain repo, under core.hooksPath, in a linked worktree, in every project of a two-project repository, in both projects when one of them is at the repository root, behind an unstaged claims_dir swap, behind an UNTRACKED config, behind assume-unchanged, under a claims_dir that points outside the config's own directory, and under a non-ASCII directory name — refuses a commit that DELETES the lock ledger (lock-ledger-absent) and, separately, one that repoints claims_dir away from tracked locked claims (lock-ledger-abandoned), refuses rather than reports OK when it could not evaluate anything at all — while still letting honest commits through in every one of them, including a claims_dir move that takes its claims with it. Doing BOTH of those in one change is one of the two detections this release knowingly gave up with the parent-commit comparison; see case 20, and internal/check/staged_no_parent_test.go for the other (a draft claim's comments block erased together with its digest-store key)."
