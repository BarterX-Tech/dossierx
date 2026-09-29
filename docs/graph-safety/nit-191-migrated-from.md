# Graph-safety evidence: migrated_from removed (NIT-191, v0.7.22)

Scope: the claim schema and `lock.LockedClaimHash`, the approval-integrity
hash the lock ledger signs. `migrated_from` was never an edge: it was not in
`ContentHash` (the dependency-drift baseline), the graph payload, the catalog,
or any readiness traversal. The only graph-adjacent consumer is the approval
check (`readiness.go` local approval, `lock.Audit`'s `lock-content-drift`),
which compares a ledger record's hash against `LockedClaimHash` of the claim
on disk.

- Candidate: commit `b6e3225` on `claude/nit-191-kill-migrated-from-nxzeqq`,
  based on `release/v0.7.22` at `da6b88d` (the v0.7.21 tag).
- Comparison baseline: `v0.7.21` (`da6b88d`), built as a second binary.
- Environment: go1.26.0 linux/amd64, cloud sandbox running as root.

## Contract change (Nitin, 2026-09-29, Linear NIT-191)

The field, the `supersede` lint, `claim list --migrated` and the viewer row
are gone. A claim file carrying `migrated_from` fails strict decode with a
hint naming the upgrade fold. Claims that carried a note re-lock once.

## Preserved invariants

- **A claim that never carried a note keeps its `LockedClaimHash`.** The hash
  wrote `migrated_from=s0:` for every claim (the field was not in
  `lockedClaimHashOmitWhenEmpty`). `lockedClaimHashRetiredEmpty` keeps writing
  exactly that line in its sorted place. Evidence:
  `TestLockedClaimHashOmitsSourcesAndTracksOnlyWhenEmpty` still matches the
  pinned constant `3baf7120…`; the 30 locked claims in `fixture-graph-demo`
  and `fixture-theme-flat` still verify against their committed ledgers
  (`TestCommittedFixtureViewersAreNotStale`, plain `check` exit 0). With the
  line removed, both fail (constant moves to `ee979896…`; every fixture claim
  reports `lock-content-drift`). Both were run to confirm.
- **A claim that carried a note fails the approval check until re-locked.**
  End to end, v0.7.21 locked three claims (a: note naming an existing file,
  b: free-text note, c: no note). After the fold under the candidate:

  | claim | local_approved | dependency_ready | review_pending | ready | ledger finding |
  |---|---|---|---|---|---|
  | a | false | true | true | false | lock-content-drift |
  | b | false | true | true | false | lock-content-drift |
  | c | true | true | false | true | none |

  After `claim unlock` and `claim lock --reason --proposal` on a and b, plain
  `check` is clean (0 ledger findings, 0 lint errors).
- **`ContentHash` is unchanged**, so no dependent's drift baseline moves and no
  `review_pending` propagates from this change alone.
- **Read-only paths write nothing**: `check --validate` over the folded corpus
  leaves `lock-store.json` byte-identical.
- **Projection output is unchanged**: regenerated viewers for all five
  committed fixtures differ from the baseline only in the generation
  timestamp and one CSS comment; every `catalog.json` is byte-identical.
- Stored claim snapshots (`DependencyReceipt.Content`, ledger
  `LedgerRecord.Content`) that carry a `MigratedFrom` key still decode: the
  store's strictness is top-level only, and the key is dropped on the next
  write. Neither `ContentHash` of a receipt nor any readiness fact reads it.

## Known transient

Between unlock and re-lock of a note-only claim (b above), the viewer's
"edited since approval" panel says "The wording is unchanged since approval."
and lists no moved field: the retained approved snapshot no longer has the
field to diff. v0.7.21's `build_role` removal had the same shape. It clears at
re-lock.

## Complexity

One constant line per claim hash: O(1) extra bytes per claim, O(V) per
corpus. No traversal, record, witness or output-size term changes.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause or baseline changed; the
approval-integrity row it would exercise is covered directly above. This is
not release approval.

Commands: `go test -count=1 ./...` (all packages pass except three
read-only-directory tests in `internal/comments` and `internal/digest`, which
fail identically on v0.7.21 as root and pass as an unprivileged user), the
browser suite (`make viewer-test`) with Chromium 1194, where every browser test
passes and only the two goreleaser dry-run tests fail because no goreleaser
binary is installed here, and the end-to-end fold above.
