#!/bin/bash
# Cross-release readiness differential, v0.7.21 against the candidate.
# Reuses the v0.7.21 record's harness unchanged; both trees take the same
# adapter (model.RestsOnIDs exists in both). Run from the repository root.
set -eu
G=${G:?scratch dir}
H=docs/graph-safety/v0.7.21-release
rm -rf "$G/base" "$G/cand"; mkdir -p "$G/base" "$G/cand"
git archive da6b88d4 | tar -x -C "$G/base"
git archive HEAD | tar -x -C "$G/cand"
for t in base cand; do
  cp "$H/xver_harness_test.go.txt" "$G/$t/internal/readiness/zz_xver_harness_test.go"
  cp "$H/xver_adapter_candidate_test.go.txt" "$G/$t/internal/readiness/zz_xver_adapter_test.go"
  (cd "$G/$t" && DX_XVER_OUT="$G/xver-$t.jsonl" go test -count=1 -v -run '^TestZZCrossVersionDifferential$' ./internal/readiness)
done
cmp "$G/xver-base.jsonl" "$G/xver-cand.jsonl" && echo "xver: byte-identical"
