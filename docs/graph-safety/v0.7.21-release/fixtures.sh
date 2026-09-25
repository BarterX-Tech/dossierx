#!/bin/bash
# Regenerate every committed fixture catalog with the candidate binary and
# compare it with the committed golden.
set -u
S=/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad
REPO=/home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de
mkdir -p $S/r0721/fx
for f in fixture-basic fixture-conformance-v1 fixture-graph-demo fixture-portability fixture-theme-flat; do
  rm -rf $S/r0721/fx/$f
  cp -r $REPO/testdata/$f $S/r0721/fx/$f
  (cd $S/r0721/fx/$f && $S/gs/dx-cand check > ../$f.check.json 2>&1; echo "$f check rc=$?")
  if cmp -s $S/r0721/fx/$f/build/catalog/catalog.json $REPO/testdata/$f/build/catalog/catalog.json; then
    echo "  catalog byte-identical to committed ($(wc -c < $REPO/testdata/$f/build/catalog/catalog.json) bytes)"
  else
    echo "  catalog DIFFERS from committed"
  fi
done
