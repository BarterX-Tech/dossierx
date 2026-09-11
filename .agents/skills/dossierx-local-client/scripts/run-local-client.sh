#!/bin/sh

set -eu

usage() {
	cat >&2 <<'EOF'
usage: run-local-client.sh <dossierx-source-directory> <client-module-directory> <dossierx-command> [args...]

Run one client's `go tool dossierx` command with the DossierX module replaced
by an explicit local source tree. The temporary workspace is removed on exit.
EOF
}

if [ "$#" -lt 3 ]; then
	usage
	exit 2
fi

source_input=$1
client_input=$2
shift 2

if [ ! -d "$source_input" ]; then
	echo "DossierX source directory does not exist: $source_input" >&2
	exit 2
fi
if [ ! -d "$client_input" ]; then
	echo "client directory does not exist: $client_input" >&2
	exit 2
fi

source_root=$(CDPATH= cd "$source_input" && pwd -P)
client_root=$(CDPATH= cd "$client_input" && pwd -P)

if [ ! -f "$source_root/go.mod" ] || ! grep -Eq '^module[[:space:]]+github\.com/BarterX-Tech/dossierx[[:space:]]*$' "$source_root/go.mod"; then
	echo "source is not the DossierX Go module: $source_root" >&2
	exit 2
fi
if [ ! -d "$source_root/cmd/dossierx" ]; then
	echo "DossierX command source is missing: $source_root/cmd/dossierx" >&2
	exit 2
fi
if [ ! -f "$client_root/go.mod" ]; then
	echo "client directory has no go.mod: $client_root" >&2
	echo "add DossierX as a Go tool before using this helper" >&2
	exit 2
fi

# Accept both a single-line tool directive and the path line inside tool (...).
if ! grep -Eq '^[[:space:]]*(tool[[:space:]]+)?github\.com/BarterX-Tech/dossierx/cmd/dossierx([[:space:]]|$)' "$client_root/go.mod"; then
	echo "client go.mod does not declare the DossierX tool: $client_root/go.mod" >&2
	echo "run: go get -tool github.com/BarterX-Tech/dossierx/cmd/dossierx@<released-version>" >&2
	exit 2
fi

workspace_root=$(mktemp -d "${TMPDIR:-/tmp}/dossierx-client-work.XXXXXX")
trap 'rm -r "$workspace_root"' EXIT

(
	cd "$workspace_root"
	go work init "$client_root"
	go work edit -replace "github.com/BarterX-Tech/dossierx=$source_root"
)

workspace_file=$workspace_root/go.work
resolved_source=$(
	cd "$client_root"
	GOWORK="$workspace_file" go list -m -f '{{if .Replace}}{{.Replace.Dir}}{{end}}' github.com/BarterX-Tech/dossierx
)

if [ -z "$resolved_source" ] || [ ! -d "$resolved_source" ]; then
	echo "local DossierX replacement did not resolve" >&2
	exit 1
fi

resolved_source=$(CDPATH= cd "$resolved_source" && pwd -P)
if [ "$resolved_source" != "$source_root" ]; then
	echo "local DossierX replacement resolved to an unexpected path: $resolved_source" >&2
	exit 1
fi

source_commit=$(git -C "$source_root" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
source_branch=$(git -C "$source_root" symbolic-ref --quiet --short HEAD 2>/dev/null || echo detached)
source_state=
if [ -n "$(git -C "$source_root" status --porcelain 2>/dev/null || true)" ]; then
	source_state=" (dirty)"
fi

echo "DossierX source: $source_root @ $source_commit [$source_branch]$source_state" >&2

(
	cd "$client_root"
	GOWORK="$workspace_file" go tool dossierx "$@"
)
