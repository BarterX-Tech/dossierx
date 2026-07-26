#!/usr/bin/env bash
#
# e2e-comments.sh — the "comments on claims" definition-of-done walkthrough,
# runnable headlessly (no browser). It proves the full serve + comment loop
# end to end against a real, built binary:
#
#   1. build dossierx and stand up a throwaway project with one draft claim;
#   2. start "dossierx serve" on a random high port and wait for it to answer;
#   3. add a comment over HTTP with an explicit ALLOWED Origin (the same
#      same-origin admission a browser tab would pass) -> expect 200 + a minted
#      thread id;
#   4. confirm the open thread now BLOCKS locking (the lock gate refuses and
#      names the thread id);
#   5. resolve the thread with the CLI;
#   6. confirm the claim now LOCKS, and that "status: locked" is on disk.
#
# Any step failing exits non-zero. The serve process and temp dir are always
# cleaned up.

set -euo pipefail

CLAIM_ID="widget.contract.overview"
SERVE_PID=""
TMP=""

fail() {
	printf 'e2e-comments: FAIL: %s\n' "$1" >&2
	if [ -n "$TMP" ] && [ -f "$TMP/serve.out" ]; then
		printf -- '--- serve output ---\n' >&2
		cat "$TMP/serve.out" >&2 || true
		printf -- '--------------------\n' >&2
	fi
	exit 1
}

cleanup() {
	if [ -n "$SERVE_PID" ]; then
		kill "$SERVE_PID" 2>/dev/null || true
		wait "$SERVE_PID" 2>/dev/null || true
	fi
	if [ -n "$TMP" ]; then
		rm -rf "$TMP" || true
	fi
}
trap cleanup EXIT

command -v curl >/dev/null 2>&1 || fail "curl is required but was not found on PATH"
command -v go >/dev/null 2>&1 || fail "go is required but was not found on PATH"

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "$SCRIPT_DIR/.." && pwd)

TMP=$(mktemp -d 2>/dev/null || mktemp -d -t dossierx-e2e)
BIN="$TMP/dossierx"
PROJECT="$TMP/project"
CFG="$PROJECT/project.config.yaml"

echo "e2e-comments: building dossierx ..."
(cd "$REPO_ROOT" && go build -o "$BIN" ./cmd/dossierx) || fail "go build failed"

# --- a throwaway project with one lockable draft claim -----------------------
mkdir -p "$PROJECT/claims"
cat >"$CFG" <<'YAML'
schema_version: 1
facets:
  - contract
modules:
  - widget
claims_dir: claims
YAML
cat >"$PROJECT/claims/overview.yaml" <<'YAML'
id: widget.contract.overview
facet: contract
module: widget
status: draft
body: |
  a claim under review.
governed_by:
  type: none
  reason: e2e walkthrough fixture, not backed by any doctrine claim
YAML

# --- start serve on a random high port ---------------------------------------
echo "e2e-comments: starting dossierx serve ..."
"$BIN" --config "$CFG" serve >"$TMP/serve.out" 2>&1 &
SERVE_PID=$!

BASE=""
for _ in $(seq 1 50); do
	if ! kill -0 "$SERVE_PID" 2>/dev/null; then
		fail "serve exited before printing its URL"
	fi
	BASE=$(grep -o 'http://127\.0\.0\.1:[0-9][0-9]*' "$TMP/serve.out" 2>/dev/null | head -1 || true)
	[ -n "$BASE" ] && break
	sleep 0.2
done
[ -n "$BASE" ] || fail "serve did not print its URL within ~10s"
echo "e2e-comments: serve is at $BASE"

# Wait until the HTTP server actually answers.
ready=""
for _ in $(seq 1 50); do
	code=$(curl -sS -o /dev/null -w '%{http_code}' "$BASE/api/ping" 2>/dev/null || echo "000")
	if [ "$code" = "200" ]; then
		ready=1
		break
	fi
	sleep 0.2
done
[ -n "$ready" ] || fail "serve /api/ping never returned 200"

# --- (3) add a comment over HTTP with an explicit ALLOWED Origin -------------
echo "e2e-comments: adding a comment over HTTP (Origin: $BASE) ..."
add_code=$(curl -sS -o "$TMP/add.json" -w '%{http_code}' \
	-X POST \
	-H "Origin: $BASE" \
	-H "Content-Type: application/json" \
	-d '{"body":"please clarify the retry policy"}' \
	"$BASE/api/claims/$CLAIM_ID/comments" 2>/dev/null || echo "000")
[ "$add_code" = "200" ] || fail "POST add comment: HTTP $add_code (body: $(cat "$TMP/add.json" 2>/dev/null))"

TID=$(grep -o 'c-[0-9a-z][0-9a-z]*' "$TMP/add.json" | head -1 || true)
[ -n "$TID" ] || fail "POST add comment did not return a minted thread id: $(cat "$TMP/add.json")"
echo "e2e-comments: opened thread $TID on $CLAIM_ID"

# --- (4) the open thread must BLOCK locking ----------------------------------
echo "e2e-comments: confirming the open thread blocks locking ..."
if "$BIN" --config "$CFG" lock "$CLAIM_ID" >"$TMP/lock_refused.out" 2>&1; then
	fail "lock succeeded while an open comment thread exists (the gate did not fire)"
fi
grep -q "$TID" "$TMP/lock_refused.out" || fail "lock refusal did not name the open thread id $TID: $(cat "$TMP/lock_refused.out")"
grep -q "status: draft" "$PROJECT/claims/overview.yaml" || fail "claim did not stay draft after a refused lock"

# --- (5) resolve the thread via the CLI --------------------------------------
echo "e2e-comments: resolving thread $TID via the CLI ..."
"$BIN" --config "$CFG" comment resolve "$CLAIM_ID" "$TID" --as human \
	>"$TMP/resolve.out" 2>&1 || fail "comment resolve failed: $(cat "$TMP/resolve.out")"

# --- (6) the claim must now LOCK ---------------------------------------------
echo "e2e-comments: locking $CLAIM_ID (should now succeed) ..."
"$BIN" --config "$CFG" lock "$CLAIM_ID" >"$TMP/lock_ok.out" 2>&1 \
	|| fail "lock refused after the thread was resolved: $(cat "$TMP/lock_ok.out")"
grep -q "status: locked" "$PROJECT/claims/overview.yaml" \
	|| fail "claim is not 'status: locked' on disk after locking"

echo "e2e-comments: PASS — comment added over HTTP, resolved via CLI, claim now locks."
