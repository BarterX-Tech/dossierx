#!/bin/bash
# Minimal repro: v0.7.20 (policy v1, untouched store) locks a, then b rests_on a;
# the candidate cannot read that store.
set -u
S=/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad
D=$S/r0721/repro
rm -rf $D; mkdir -p $D/claims
cd $D
printf 'schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n' > project.config.yaml
printf 'id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  A.\ngoverned_by:\n  type: none\n  reason: r\n' > claims/a.yaml
printf 'id: widget.contract.b\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  B.\nrests_on:\n  - widget.contract.a\ngoverned_by:\n  type: none\n  reason: r\n' > claims/b.yaml
for c in a b; do
  tok=$($S/gs/dx-0720 claim lock widget.contract.$c --dry-run | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["snapshot"])')
  $S/gs/dx-0720 claim lock widget.contract.$c --reason "approve $c" --proposal "$tok" > /dev/null; echo "v0.7.20 lock $c rc=$?"
done
python3 -c "import json;d=json.load(open('build/ledger/lock-store.json'));print('v0.7.20 store: version',d['version'],'policy_version',d.get('policy_version'),'keys',sorted(d))"
# Fold (upgrading skill step 2/6): drop governed_by, add summary, root rests_on none.
for c in a b; do python3 - claims/$c.yaml <<'EOF'
import sys; p=sys.argv[1]; t=open(p).read().split("governed_by:")[0]
t=t.replace("facet: contract\n","summary: fixture.\nfacet: contract\n")
if "rests_on" not in t: t+="rests_on:\n  none: true\n  reason: root\n"
open(p,"w").write(t)
EOF
done
mkdir -p claims/widget; printf 'summary: m.\nprovides:\n  - widget.contract.a\n  - widget.contract.b\ndepends_on: []\n' > claims/widget/manifest.yaml
$S/gs/dx-cand claim unlock widget.contract.b --reason "upgrade" | python3 -c 'import json,sys;d=json.load(sys.stdin);print("candidate claim unlock b:",d["ok"],d["error"]["code"],d["error"]["message"][-120:])'
$S/gs/dx-cand claim show widget.contract.a | python3 -c 'import json,sys;r=json.load(sys.stdin)["data"]["readiness"];print("candidate claim show a (no rests_on itself): local_approved",r["local_approved"],"causes",[c["source_kind"] for c in r.get("causes") or []])'
