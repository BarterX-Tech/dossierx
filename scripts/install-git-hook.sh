#!/bin/sh
#
# install-git-hook.sh — install DossierX's pre-commit integrity gate into a
# git repository, ASKING FIRST.
#
# WHY A SCRIPT AND NOT A CLI COMMAND
#
# v0.3.0 fixes the CLI surface at 19 leaves, and writing an executable into
# someone's .git is not a thing a docs engine should be able to do as a side
# effect of some other verb. So the hook install is this: a standalone,
# self-contained script that a bootstrap agent runs AFTER the human has said
# yes, and that refuses to write anything when it cannot see that yes (no
# --yes flag and no terminal to ask on => abort). The script is deliberately
# one file with the hook body embedded, so an agent can copy it verbatim into
# a project that has the dossierx BINARY but not this repository.
#
# WHAT THE HOOK IS FOR, AND WHAT IT IS NOT
#
# Claims are YAML in git. Nothing can PREVENT an edit; the goal is that no
# out-of-band edit of a LOCKED claim is silent. The hook runs
# "dossierx check --staged" over the index and refuses the commit when a
# locked claim moved without an approval record on the ledger. Every tool
# commits through git — editor, agent, script — so one gate covers all of
# them and there is no per-harness adapter to maintain.
#
# The hook is FAST FEEDBACK, NOT THE AUTHORITY. Clean merges, rebases,
# cherry-picks and reverts do not fire pre-commit at all, and --no-verify
# skips it by design. CI is the authority: see scripts/ci/dossierx-check.yml.
#
# THE FOUR THINGS THIS SCRIPT IS CAREFUL ABOUT
#
#   core.hooksPath  Resolved, never set. A repo whose hooks live elsewhere
#                   (husky, lefthook, pre-commit.com, a shared team dir) is
#                   installed into THAT directory. Pointing core.hooksPath at
#                   a directory of our own would silently disable every other
#                   hook the project runs, which is a hostile thing to do to
#                   someone who said yes to "add a pre-commit check".
#   worktrees       In a linked worktree .git is a FILE, not a directory, so
#                   ".git/hooks" is simply wrong. git rev-parse --git-path
#                   answers correctly in both layouts (and resolves to the
#                   shared common dir, which is where git actually looks).
#   Windows         The hook body is POSIX sh; git for Windows executes hooks
#                   with its own bundled sh, so no exec bit and no
#                   interpreter on PATH is required. This installer runs under
#                   Git Bash or WSL; install-git-hook.ps1 is a thin wrapper
#                   for PowerShell users who do not have bash on PATH. Both
#                   paths are exercised by scripts/hook-smoke-test.sh, which
#                   CI runs on windows-latest.
#   idempotence     Re-running is a no-op with a one-line report. An existing
#                   hook that dossierx did not write is NEVER replaced without
#                   --force, and --force backs it up and says where.
#
# USAGE
#
#   scripts/install-git-hook.sh [options]
#
#     -y, --yes        the human already said yes; do not prompt
#         --dry-run    print exactly what would happen; write nothing
#         --force      replace a pre-existing foreign pre-commit hook,
#                      backing it up first
#         --uninstall  remove the dossierx hook (never a foreign one)
#         --print-hook write the hook body to stdout and exit (for review, or
#                      for chaining it from a hook you already have)
#         --repo DIR   operate on the repository containing DIR
#     -h, --help       this text
#
# Exit status: 0 installed / already current / dry run; 1 declined, refused,
# or failed.

set -eu

marker='# dossierx-hook: pre-commit v1'

# ---------------------------------------------------------------------------
# The hook body.
#
# Kept at column 0 in an unindented, quoted heredoc so what is written is
# byte-for-byte what is read here: the installer decides "already installed
# and current" by comparing this output against the file on disk, and any
# indentation stripping or variable expansion would make that comparison a
# lie. Nothing in it is expanded at install time — the hook resolves the
# binary, the config and the repository for itself, at commit time, in the
# state the commit is actually being made from.
# ---------------------------------------------------------------------------
hook_body() {
	cat <<'PRECOMMIT_HOOK'
#!/bin/sh
# dossierx-hook: pre-commit v1
#
# Refuses a commit that changes a LOCKED claim without an approval record.
#
# Managed by dossierx's install-git-hook.sh. Do not edit in place: the
# installer compares this file byte-for-byte with the version it carries, so a
# local edit turns every later install into a "replace it?" prompt. Change the
# installer and re-run it instead.
#
# WHAT IT RUNS
#
#   dossierx check --staged
#
# --staged reads the content being COMMITTED (git show :<path>), not the
# working tree. That matters twice over: "git add -p" commits a subset of your
# edits and only the staged subset is being asked about, and the check writes
# nothing, so the hook cannot dirty the tree it is validating.
#
# WHAT IT DOES NOT COVER
#
# pre-commit does not fire on a clean merge, a rebase, a cherry-pick or a
# revert, and "git commit --no-verify" skips it outright. This hook is fast
# feedback. CI running the same gate on the pull request is the authority.
#
# ESCAPE HATCHES, both deliberate and both loud in CI:
#
#   git commit --no-verify       skip every hook, once
#   DOSSIERX_SKIP_HOOK=1 git ... skip this hook, once
#
# ENVIRONMENT
#
#   DOSSIERX_BIN     path to the dossierx binary (default: "dossierx" on PATH)
#   DOSSIERX_CONFIG  path to project.config.yaml. Only needed when the repo
#                    holds MORE THAN ONE dossierx project; with one, the hook
#                    finds it via the index.
#   DOSSIERX_SKIP_HOOK=1  skip this run

set -u

if [ "${DOSSIERX_SKIP_HOOK:-}" = "1" ]; then
	exit 0
fi

bin=${DOSSIERX_BIN:-dossierx}
if ! command -v "$bin" >/dev/null 2>&1; then
	# Refusing rather than passing is the honest choice: a hook that waves
	# commits through whenever the tool is missing is a hook that is off
	# exactly when someone has a broken environment. The bypass is one flag
	# away and named in the message.
	printf '%s\n' \
		'dossierx: COMMIT REFUSED — the dossierx binary was not found on PATH.' \
		'' \
		'  install it:   go install github.com/BarterX-Tech/dossierx/cmd/dossierx@latest' \
		'  or point at it: DOSSIERX_BIN=/path/to/dossierx git commit ...' \
		'  or skip once:   git commit --no-verify' \
		'  or remove the hook: scripts/install-git-hook.sh --uninstall' \
		>&2
	exit 1
fi

# Locate the project config. git runs hooks from the top level of the working
# tree, and dossierx searches UPWARD from the working directory, so a project
# that does not live at the repository root would never be found. Asking the
# INDEX (rather than walking the filesystem) is both faster and correctly
# scoped to tracked files.
config=${DOSSIERX_CONFIG:-}
if [ -z "$config" ] && [ ! -f project.config.yaml ]; then
	matches=$(git ls-files -- 'project.config.yaml' '*/project.config.yaml' 2>/dev/null || true)
	count=$(printf '%s' "$matches" | grep -c . || true)
	if [ "$count" = "1" ]; then
		config=$matches
	elif [ "${count:-0}" -gt 1 ]; then
		printf '%s\n' \
			'dossierx: this repository contains more than one project.config.yaml;' \
			'          set DOSSIERX_CONFIG to the one this hook should check.' >&2
	fi
fi

run_check() {
	if [ -n "$config" ]; then
		"$bin" --config "$config" check --staged --format "$1" 2>&1
	else
		"$bin" check --staged --format "$1" 2>&1
	fi
}

# The JSON run is the branch: error.code is the only stable signal, and
# matching on prose would break the first time a message is reworded. The
# human-readable re-run happens only on the failure path, where a second
# ~200ms read-only pass is worth a report someone can act on.
report=$(run_check json)
status=$?
if [ "$status" -eq 0 ]; then
	exit 0
fi

case "$report" in
*'unknown flag: --staged'* | *'unknown flag "--staged"'*)
	printf '%s\n' \
		'dossierx: COMMIT REFUSED — this dossierx build has no "check --staged",' \
		'          so the hook cannot see what you are committing.' \
		'' \
		'  upgrade:  go install github.com/BarterX-Tech/dossierx/cmd/dossierx@latest' \
		'  or remove the hook: scripts/install-git-hook.sh --uninstall' \
		>&2
	exit 1
	;;
esac

# No project here is not a violation. A repository can hold a dossierx project
# in one branch and not another, and a hook that blocks every commit on a
# branch without claims would just get uninstalled. CI still fails on the
# branch that does have them.
case "$report" in
*'"code"'*'"config_not_found"'*)
	printf '%s\n' 'dossierx: no project.config.yaml found; skipping the claim-integrity check.' >&2
	exit 0
	;;
esac

printf '%s\n' 'dossierx: COMMIT REFUSED — staged claims did not pass the integrity gate.' '' >&2
run_check text >&2 || true
printf '\n%s\n' \
'The approval path for anything already LOCKED is unlock -> fix -> lock:

  1. dossierx claim unlock <id> --reason "<why the human agreed to reopen it>"
  2. make the edit — or revert it, if the change was not meant to happen
  3. dossierx claim lock   <id> --reason "<the human approval, in their words>"

Then stage BOTH the claim file and the lock ledger (.dossierx-lock-store.json,
.dossierx-comment-digest.json) and commit again — the ledger is a tracked file
and CI reads it.

reaudit is the DRIFT tool, not the general edit tool; it will refuse a claim
that is not already review_pending.

To commit anyway: git commit --no-verify. CI runs the same gate on the pull
request, so this postpones the refusal rather than removing it.' >&2
exit 1
PRECOMMIT_HOOK
}

# ---------------------------------------------------------------------------
# Plumbing.
# ---------------------------------------------------------------------------

die() {
	printf 'install-git-hook: %s\n' "$1" >&2
	exit 1
}

usage() {
	# The header of this file IS the documentation; printing the flag list
	# twice is how the two drift apart. Piped-from-stdin invocations have no
	# readable $0, so fall back to the one line that matters.
	if [ -r "$0" ]; then
		sed -n '/^# USAGE/,/^# Exit status/p' "$0" | sed 's/^# \{0,1\}//'
	else
		printf '%s\n' 'usage: install-git-hook.sh [-y|--yes] [--dry-run] [--force] [--uninstall] [--print-hook] [--repo DIR]'
	fi
}

assume_yes=0
dry_run=0
force=0
uninstall=0
repo_dir=

while [ $# -gt 0 ]; do
	case "$1" in
	-y | --yes) assume_yes=1 ;;
	--dry-run) dry_run=1 ;;
	--force) force=1 ;;
	--uninstall) uninstall=1 ;;
	--print-hook)
		hook_body
		exit 0
		;;
	--repo)
		[ $# -ge 2 ] || die "--repo needs a directory"
		repo_dir=$2
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "unknown option: $1 (try --help)" ;;
	esac
	shift
done

command -v git >/dev/null 2>&1 || die "git was not found on PATH"

if [ -n "$repo_dir" ]; then
	cd "$repo_dir" || die "cannot enter $repo_dir"
fi

[ "$(git rev-parse --is-inside-work-tree 2>/dev/null || echo false)" = "true" ] ||
	die "not inside a git working tree (a bare repository has no commits to hook)"

# Resolve where git will actually LOOK for hooks. --git-path answers for the
# plain layout, for a linked worktree (where .git is a file and hooks live in
# the shared common dir), and for core.hooksPath — all three, without this
# script needing to know which one it is in. --path-format=absolute needs git
# 2.31+; older git gets a path relative to the current directory, which we
# absolutise ourselves (including the C:/ and C:\ forms Git for Windows
# reports).
hooks_dir=$(git rev-parse --path-format=absolute --git-path hooks 2>/dev/null) || hooks_dir=
if [ -z "$hooks_dir" ]; then
	hooks_dir=$(git rev-parse --git-path hooks) || die "could not resolve the hooks directory"
	case "$hooks_dir" in
	/* | [A-Za-z]:[/\\]*) ;;
	*) hooks_dir="$PWD/$hooks_dir" ;;
	esac
fi
target="$hooks_dir/pre-commit"

configured_hooks_path=$(git config --get core.hooksPath 2>/dev/null || true)

# Classify what is already there. "ours" is decided by the marker line rather
# than by a full comparison so that an OLDER dossierx hook is still recognised
# as ours and can be replaced without the foreign-hook ceremony.
state=absent
if [ -e "$target" ]; then
	if grep -q '^# dossierx-hook: pre-commit ' "$target" 2>/dev/null; then
		if hook_body | cmp -s - "$target"; then
			state=current
		else
			state=outdated
		fi
	else
		state=foreign
	fi
fi

if [ -n "$configured_hooks_path" ]; then
	printf '%s\n' \
		"note: this repository sets core.hooksPath = $configured_hooks_path" \
		"      installing there. This script never sets or changes core.hooksPath —" \
		"      repointing it would silently disable every other hook you run." ""
fi

# --- uninstall --------------------------------------------------------------
if [ "$uninstall" -eq 1 ]; then
	case "$state" in
	absent)
		printf '%s\n' "nothing to do: no pre-commit hook at $target"
		exit 0
		;;
	foreign)
		die "the pre-commit hook at $target was not written by dossierx; remove it yourself if you meant to"
		;;
	esac
	printf '%s\n' "dossierx wants to remove its pre-commit hook at $target"
	if [ "$dry_run" -eq 1 ]; then
		printf '%s\n' "(--dry-run: nothing written)"
		exit 0
	fi
	if [ "$assume_yes" -ne 1 ]; then
		[ -t 0 ] || die "no terminal to ask on; re-run with --yes if the human agreed"
		printf 'Remove it? [y/N] '
		read -r answer || answer=
		case "$answer" in
		y | Y | yes | Yes | YES) ;;
		*)
			printf '%s\n' "aborted; nothing was removed"
			exit 1
			;;
		esac
	fi
	rm -f "$target"
	printf '%s\n' "removed $target" \
		"CI is still the authority — leave the workflow in place."
	exit 0
fi

# --- install ----------------------------------------------------------------
if [ "$state" = current ]; then
	# Idempotent by design: a bootstrap agent that re-runs this on every
	# session must not produce a prompt, a diff, or a new file mtime.
	printf '%s\n' "already installed and current: $target"
	exit 0
fi

backup=
case "$state" in
absent)
	plan="create the dossierx pre-commit hook at $target"
	;;
outdated)
	plan="replace an OLDER dossierx pre-commit hook at $target"
	;;
foreign)
	if [ "$force" -ne 1 ]; then
		printf '%s\n' \
			"refusing to touch $target: there is already a pre-commit hook there that dossierx did not write." \
			"" \
			"Two ways forward:" \
			"" \
			"  replace it   re-run with --force. The existing hook is copied to" \
			"               <hook>.pre-dossierx.<timestamp> first, and this script tells you where." \
			"" \
			"  chain it     keep your hook and call ours from it:" \
			"" \
			"                 scripts/install-git-hook.sh --print-hook > \\" \
			"                     \"$hooks_dir/dossierx-pre-commit\"" \
			"                 chmod +x \"$hooks_dir/dossierx-pre-commit\"" \
			"" \
			"               then add this to your own pre-commit, wherever you want" \
			"               the claim gate to run:" \
			"" \
			"                 \"\$(dirname \"\$0\")/dossierx-pre-commit\" || exit 1" \
			>&2
		exit 1
	fi
	backup="$target.pre-dossierx.$(date +%Y%m%d%H%M%S)"
	plan="replace the EXISTING non-dossierx pre-commit hook at $target
       (backing it up to $backup)"
	;;
esac

printf '%s\n' "dossierx wants to $plan" "" \
	"The hook runs \"dossierx check --staged\" and refuses a commit in which a" \
	"locked claim changed without an approval record on the lock ledger. It" \
	"writes nothing, reads the index rather than your working tree, and is" \
	"skippable with \"git commit --no-verify\"." ""

if [ "$dry_run" -eq 1 ]; then
	printf '%s\n' "(--dry-run: nothing written)"
	exit 0
fi

if [ "$assume_yes" -ne 1 ]; then
	# A hook is an executable that runs on someone's machine without them
	# asking again. No terminal and no explicit --yes means nobody has said
	# yes, and the correct move is to stop — never to write "helpfully".
	[ -t 0 ] || die "no terminal to ask on; re-run with --yes once the human has agreed"
	printf 'Install it? [y/N] '
	read -r answer || answer=
	case "$answer" in
	y | Y | yes | Yes | YES) ;;
	*)
		printf '%s\n' "aborted; nothing was written"
		exit 1
		;;
	esac
fi

mkdir -p "$hooks_dir" || die "could not create $hooks_dir"

if [ -n "$backup" ]; then
	cp "$target" "$backup" || die "could not back up $target to $backup"
fi

# Write through a temporary file in the same directory and rename over the
# target, so a hook that is mid-write is never what git picks up.
tmp="$target.dossierx-install.$$"
hook_body >"$tmp" || die "could not write $tmp"
# chmod is a no-op on a Windows filesystem and that is fine: git for Windows
# runs hooks through its bundled sh and does not consult the exec bit.
chmod 0755 "$tmp" 2>/dev/null || true
mv -f "$tmp" "$target" || die "could not install $target"

printf '%s\n' "installed $target"
# Not "[ -n "$backup" ] && printf ...": under set -e a false test as the last
# command of the script would exit 1 on the success path.
if [ -n "$backup" ]; then
	printf '%s\n' "your previous hook is at $backup"
fi
printf '%s\n' "" \
	"Two things this hook does NOT do:" \
	"  · it does not fire on clean merges, rebases, cherry-picks or reverts —" \
	"    git simply does not run pre-commit for those. Add the CI workflow" \
	"    (scripts/ci/dossierx-check.yml); CI is the authority." \
	"  · it does not check anything you did not stage. --staged reads the index." \
	"" \
	"Commit .dossierx-lock-store.json and .dossierx-comment-digest.json — they" \
	"are tracked artifacts, and the gate is vacuous without them."
