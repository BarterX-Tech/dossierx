#!/bin/bash
# Run every proof test the per-ticket graph-safety notes name, by exact name,
# and fail unless each one reports --- PASS (a selector matching nothing, a
# SKIP or a FAIL is a failure). Input: named.txt lines "<pkg dir> <TestName>".
# Run from the repository root.
set -u
LIST=${1:-docs/graph-safety/v0.7.22-release/named.txt}
OUT=${2:-/dev/stdout}
pass=0; bad=0
for pkg in $(cut -d' ' -f1 "$LIST" | sort -u); do
  names=$(awk -v p="$pkg" '$1==p{print $2}' "$LIST" | paste -sd'|' -)
  log=$(go test -count=1 -v -run "^(${names})\$" "./$pkg" 2>&1)
  for n in $(awk -v p="$pkg" '$1==p{print $2}' "$LIST"); do
    if echo "$log" | grep -qE "^--- PASS: ${n} "; then pass=$((pass+1)); echo "PASS $pkg $n"
    else bad=$((bad+1)); echo "NOT-PASS $pkg $n"; echo "$log" | grep -E "^--- (FAIL|SKIP): ${n} "; fi
  done
  echo "$log" | grep -E '^--- SKIP|^    --- SKIP' && bad=$((bad+1))
done
echo "named tests: $pass PASS, $bad not passing"
[ $bad -eq 0 ]
