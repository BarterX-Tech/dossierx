#!/bin/bash
# Paired, alternating isolated runs of TestConformanceEndToEndGraphScaleBounds/chain-128
# on the candidate (20072b7) and v0.7.20 test binaries, same host, same load window.
set -u
S=/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad
REPO=/home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de
OUT=$S/r0721/chain128
mkdir -p $OUT
cd $REPO && go test -c -o $OUT/check-cand.test ./internal/check/ || exit 1
cd $S/v0720 && go test -c -o $OUT/check-0720.test ./internal/check/ || exit 1
N=${N:-12}
: > $OUT/runs.tsv
for i in $(seq 1 $N); do
  for v in cand 0720; do
    if [ $v = cand ]; then dir=$REPO/internal/check; else dir=$S/v0720/internal/check; fi
    load=$(cut -d' ' -f1 /proc/loadavg)
    line=$(cd $dir && $OUT/check-$v.test -test.count=1 -test.v -test.run 'TestConformanceEndToEndGraphScaleBounds/chain-128$' 2>&1 | tee $OUT/$v-$i.log | grep -E 'elapsed=|took|allocated' | head -1)
    res=$(grep -E '^--- (PASS|FAIL): TestConformanceEndToEndGraphScaleBounds$' $OUT/$v-$i.log | awk '{print $2}')
    el=$(echo "$line" | sed -n 's/.*elapsed=\([^ ]*\).*/\1/p')
    [ -z "$el" ] && el=$(echo "$line" | sed -n 's/.*took \([^,]*\),.*/\1/p')
    al=$(echo "$line" | sed -n 's/.*TotalAlloc=\([0-9]*\).*/\1/p')
    cb=$(echo "$line" | sed -n 's/.*catalog_bytes=\([0-9]*\).*/\1/p')
    vb=$(echo "$line" | sed -n 's/.*viewer_bytes=\([0-9]*\).*/\1/p')
    printf "%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n" $v $i "$res" "$el" "$al" "$cb" "$vb" "$load" | tee -a $OUT/runs.tsv
  done
done
