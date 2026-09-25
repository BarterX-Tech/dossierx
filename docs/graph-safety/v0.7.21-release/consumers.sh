#!/bin/bash
# Targeted consumer-contract tests (claim show/list, lock singleton/batch, catalog,
# graph payload, constitution, ledger, lifecycle) on the candidate tree.
set -u
S=/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad/r0721
cd /home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de
go test -count=1 -v -p 1 -timeout 20m -run 'Test[A-Za-z0-9_]*(Lock|Readiness|Policy|ClaimShow|ClaimList|Catalog|Graph|Batch|Unlock|Reaudit|Flag|RestsOn|Constitution|Ledger|Drift|Carry|Retired)' ./cmd/dossierx/ > $S/cmd.txt 2>&1
echo exit=$? >> $S/cmd.txt
go test -count=1 -v -p 1 -timeout 20m -run 'Test[A-Za-z0-9_]*(Catalog|Lock|Lifecycle|Readiness|ProjectClaim|Ledger|Graph|RestsOn|Edge|Drift)' ./tests/ > $S/tests.txt 2>&1
echo exit=$? >> $S/tests.txt
for f in cmd tests; do
  echo "$f: $(tail -1 $S/$f.txt) top-PASS=$(grep -c '^--- PASS' $S/$f.txt) all-PASS=$(grep -c -- '--- PASS' $S/$f.txt) FAIL=$(grep -c -- '--- FAIL' $S/$f.txt) SKIP=$(grep -c -- '--- SKIP' $S/$f.txt)"
done
