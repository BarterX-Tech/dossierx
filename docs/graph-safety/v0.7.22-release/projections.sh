#!/bin/bash
# Compare each committed fixture's catalog.json and viewer graph payload
# (the dossierx-graph block, render stamps masked) at v0.7.21 and at the
# candidate. Run from the repository root. $G is a scratch directory.
set -u
G=${G:?scratch dir}
BASE=${BASE:-da6b88d4}
CAND=${CAND:-HEAD}
for n in basic conformance-v1 graph-demo portability theme-flat; do
  cat="testdata/fixture-$n/build/catalog/catalog.json"
  view="testdata/fixture-$n/build/viewer/index.html"
  if cmp -s <(git show "${BASE}:${cat}") <(git show "${CAND}:${cat}"); then c=identical; else c=DIFF; fi
  for r in "$BASE" "$CAND"; do
    git show "${r}:${view}" | grep -o '<script type="application/json" id="dossierx-graph">.*</script>' \
      | sed -E 's/20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z/TS/g' > "$G/graph-$n-$r.json"
  done
  if cmp -s "$G/graph-$n-$BASE.json" "$G/graph-$n-$CAND.json"; then g=identical; else g=DIFF; fi
  echo "$n catalog=$c ($(git show "${CAND}:${cat}" | wc -c | tr -d ' ') B) graph-payload=$g ($(wc -c < "$G/graph-$n-$CAND.json" | tr -d ' ') B)"
done
