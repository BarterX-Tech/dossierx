#!/bin/bash
# Brief-state differential (docs/graph-safety/nit-192-briefs.md, re-run for the
# v0.7.22 release). Usage: brief-state.sh <scratch dir> <repo root> <dx binary>
# A: fixture-graph-demo with a locked brief edited since approval AND resting
#    on a claim that moved; B: the same corpus with no briefs at all (briefs/
#    removed, the store's briefs map dropped and its version put back to 3).
# Every claim-side artifact must be byte-identical between A and B.
set -u
R=$1; W=$2; DX=$3
export DOSSIERX_ACTOR=proof
P=$R/bs
prep() {
  rm -rf "$P"; cp -R "$W/testdata/fixture-graph-demo" "$P"; cd "$P" || exit 1
  "$DX" brief lock briefs/graph/reading-the-graph.md --reason proof --format text >/dev/null || { echo "brief lock failed"; exit 1; }
  sed -i '' 's/nothing it infers\./nothing it infers, edited./' briefs/graph/reading-the-graph.md
  grep -q 'nothing it infers, edited\.' briefs/graph/reading-the-graph.md || { echo "brief edit did not apply"; exit 1; }
  f=$(grep -l "^id: engine.contract.build-is-pure" -r claims)
  sed -i '' 's/^summary: \(.*\)$/summary: \1 (moved)/' "$f"
}
out() {
  cd "$P" || exit 1
  "$DX" check > "$R/$1.check.json" 2>&1; echo "check rc=$?" >> "$R/$1.rc"
  "$DX" check --validate > "$R/$1.validate.json" 2>&1; echo "validate rc=$?" >> "$R/$1.rc"
  "$DX" claim list > "$R/$1.list.json"
  for m in $(ls claims | grep -v '\.'); do
    "$DX" manifest show "$m" --isolation; "$DX" manifest show "$m" --integration
  done > "$R/$1.manifest.json"
  cp build/catalog/catalog.json "$R/$1.catalog.json"
  grep -v 'id="dossierx-briefs"' build/viewer/index.html \
    | sed -E 's/20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z/TS/g' > "$R/$1.viewer-claims.html"
  grep -o '<script type="application/json" id="dossierx-graph">.*</script>' build/viewer/index.html \
    | sed -E 's/20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z/TS/g' > "$R/$1.graph.json"
  for c in $(grep -rl "^status: locked" claims | sort); do cat "$c"; done > "$R/$1.claimfiles.txt"
  python3 - "$R/$1" <<'PY'
import json,sys
p=sys.argv[1]
for k in ("check","validate"):
    d=json.load(open(p+"."+k+".json"))
    dd=d.get("data") or {}
    ld=sorted({(f.get("rule"),f.get("claim_id")) for f in dd.get("ledger_findings") or []})
    lf=sorted({(f.get("lint"),f.get("claim_id")) for f in dd.get("lint_findings") or []})
    err=(d.get("error") or {}).get("code")
    print(k,"error:",err,"ledger:",ld,"lint:",lf)
PY
}
rm -f "$R"/A.rc "$R"/B.rc
prep; out A > "$R/A.findings"
prep
rm -rf briefs
python3 - build/ledger/lock-store.json <<'PY'
import json,sys
p=sys.argv[1]; d=json.load(open(p)); d.pop("briefs",None); d["version"]=3
json.dump(d,open(p,"w"),indent=2)
PY
out B > "$R/B.findings"
cd "$R" || exit 1
for k in list.json manifest.json catalog.json graph.json viewer-claims.html claimfiles.txt; do
  if cmp -s "A.$k" "B.$k"; then echo "$k identical ($(wc -c < "A.$k" | tr -d ' ') B)"; else echo "$k DIFF"; fi
done
echo "--- A (with briefs)"; cat A.rc A.findings
echo "--- B (no briefs)"; cat B.rc B.findings
