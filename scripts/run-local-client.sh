#!/bin/sh

set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd -P)
source_root=$(CDPATH= cd "$script_dir/.." && pwd -P)

exec "$source_root/.agents/skills/dossierx-local-client/scripts/run-local-client.sh" \
	"$source_root" "$@"
