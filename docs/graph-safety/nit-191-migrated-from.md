# Graph-safety evidence: migrated_from removed (NIT-191, v0.7.22)

Scope: the claim schema and `lock.LockedClaimHash`, the approval-integrity
hash the lock ledger signs. `migrated_from` was never an edge: it was not in
`ContentHash` (the dependency-drift baseline), the graph payload, the catalog,
or any readiness traversal. The only graph-adjacent consumer is the approval
check (`readiness.go` local approval, `lock.Audit`'s `lock-content-drift`),
which compares a ledger record's hash against `LockedClaimHash` of the claim
on disk.

- Candidate: the head of `claude/nit-191-kill-migrated-from-nxzeqq` (PR #126),
  based on `release/v0.7.22` at `da6b88d` (the v0.7.21 tag).
- Comparison baseline: `v0.7.21` (`da6b88d`), built as a second binary.
- Environment: go1.26.0 linux/amd64, cloud sandbox running as root.

## Contract change (Nitin, 2026-09-29, Linear NIT-191)

The field, the `supersede` lint, `claim list --migrated` and the viewer row
are gone. A claim file carrying `migrated_from` fails strict decode with a
hint naming the upgrade fold. `LockedClaimHash` signed `migrated_from` even
when empty, so its line leaves the hash and **every** locked claim's hash
moves once; each re-locks on the human's approval (Nitin confirmed this in
the project thread).

## Preserved invariants

- **A moved hash fails closed.** End to end, v0.7.21 locked three claims
  (a: note naming an existing file, b: free-text note, c: no note). After
  the fold under the candidate, all three report `lock-content-drift`,
  `local_approved: false`, `ready: false`. After `claim unlock` and
  `claim lock --dry-run` / `--reason --proposal <snapshot>` on each, all
  three are locked and `ready: true`, and plain `check` is clean (0 ledger
  findings, 0 lint errors).
- **The committed fixture ledgers were re-hashed record by record, only
  where the old hash matched.** 361 fixture claims were hashed under the
  v0.7.21 rule and the candidate rule; every committed ledger hash equal to
  a claim's old hash was replaced by its new hash (30 records in
  `fixture-graph-demo` and `fixture-theme-flat`, the 30 locked claims). No
  other file in the tree held an old hash. `TestCommittedFixtureViewersAreNotStale`
  re-verifies them. The pinned `lockedClaimHashNoOptionalFields` constant
  moves to `ee979896…`.
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

Between unlock and re-lock, a claim whose only change is the hash (no note,
or a deleted note) shows "The wording is unchanged since approval." in the
viewer's "edited since approval" panel with no moved field listed: the
retained approved snapshot has no field left to diff. v0.7.21's `build_role`
removal had the same shape. It clears at re-lock.

## Complexity

One fewer line per claim hash. No traversal, record, witness or output-size term changes.

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
