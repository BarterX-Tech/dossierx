#!/bin/bash
# Non-root equivalent of TestCatalogUnwritableTargetDirFailsLoudly (skipped as uid 0).
set -u
S=/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad
D=/tmp/dxgs-v0721-unwritable
rm -rf $D; mkdir -p $D; chmod 755 $D
cp $S/gs/dx-cand $D/dx; cp -r /home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de/testdata/fixture-basic $D/p
rm -f $D/p/build/catalog/catalog.json
chown -R nobody $D/p; chmod -R a+rX $D
chmod 555 $D/p/build/catalog $D/p/build
runuser -u nobody -- bash -c "cd $D/p && HOME=$D $D/dx check" > $D/out.json 2>&1
echo "rc=$?"
python3 -c "import json;d=json.load(open('$D/out.json'));print('ok=',d.get('ok'),'code=',(d.get('error') or {}).get('code'),'msg=',(d.get('error') or {}).get('message','')[:200],'stopped_at=',d.get('stopped_at'))"
test -e $D/p/build/catalog/catalog.json && echo "catalog.json EXISTS (unexpected)" || echo "catalog.json absent (no silent partial write)"
chmod -R u+w $D; rm -rf $D
