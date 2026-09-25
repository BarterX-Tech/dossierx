#!/bin/bash
# Build test binaries as root, run the root-sensitive tests as uid nobody so
# permission-based cases execute instead of skipping/failing on uid 0.
set -u
REPO=/home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de
D=/tmp/dxgs-v0721-nonroot
rm -rf $D; mkdir -p $D/tmp; chmod 755 $D; chmod 1777 $D/tmp
cd $REPO
for p in lock comments serve render/components; do
  n=$(echo $p | tr / _)
  go test -c -o $D/$n.test ./internal/$p/ || echo "buildfail $p"
  mkdir -p $D/src/$n
  cp -r internal/$p/. $D/src/$n/ 2>/dev/null
done
chmod -R a+rX $D
run() { # name dir regex
  echo "== $1 -run '$3'"
  (cd $D/src/$2 && runuser -u nobody -- env HOME=$D/tmp TMPDIR=$D/tmp $D/$2.test -test.count=1 -test.v -test.run "$3" 2>&1 | grep -E '^(=== RUN|--- |PASS|FAIL|ok)|_test.go' )
}
run lock lock 'TestAcquireFileLockCreatesTheSentinelDirectory'
run comments comments 'TestEdge_ReadOnlyClaimFile_CleanError_NoPartialWrite|TestReadOnlyProjectDirWritesNothingOnEitherAttempt'
run serve serve 'TestClaimAsset_AnUnreadableTreeKeepsThePreviousIndex'
run render_components render_components 'TestOverrideFile_UnreadableFileIsHardError'
