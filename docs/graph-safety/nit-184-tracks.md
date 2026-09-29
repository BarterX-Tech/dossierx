# Graph-safety evidence: tracks removed (NIT-184, v0.7.22)

Scope: the claim and config schema, `lock.LockedClaimHash` (the approval
hash the lock ledger signs), and the catalog and graph projections that
carried track membership. Tracks were never an edge: membership was not in
`ContentHash` (the dependency-drift baseline), joined no cycle walk, and was
read by no readiness traversal. The graph payload carried it as node and
group metadata for a filter, and the catalog as a per-claim list.

- Candidate: the head of `claude/nit-184-kill-tracks-znikw3`, based on
  `release/v0.7.22` at `6d6fedf` (NIT-191 merged).
- Comparison baseline: `release/v0.7.22` at `6d6fedf`, built as a second
  binary.
- Environment: go1.26.0 linux/amd64, cloud sandbox running as root.

## Contract change (Nitin, 2026-09-29, Linear NIT-180 and NIT-184)

A feature is a brief; tracks retire with no dead code. `tracks` on a claim
fails strict decode (`invalid_claim`) and `tracks` in the config fails
(`invalid_config`); both hints open with `tracks-retired` and name the
upgrading skill's "tracks are gone" fold. `tracks` sat in
`lockedClaimHashOmitWhenEmpty`, so its empty form wrote nothing to the hash:
only a claim that carried tracks moves, unlike `migrated_from`.

## Preserved invariants

- **A moved hash fails closed, and only where it moved.** End to end, the
  baseline binary locked three claims: `a` (owning a track), `b` (no track)
  and `c` (no track, `rests_on: a`). Under the candidate the unfolded corpus
  refuses at load with `tracks-retired`. After the fold, `check --validate`
  reports exactly one ledger finding, `lock-content-drift` on `a`
  (`local_approved: false`, `ready: false`); `b` stays locally approved and
  ready with no finding.
- **No baseline moves and no review state is written.** `ContentHash` is
  unchanged, so `c`'s dependency baseline on `a` still matches. `c` reports
  the existing live, inherited `upstream_dependency_review`
  (`source_kind: approval_content_drift`) and `dependency_unapproved` while
  `a` is unapproved, as for any drifted dependency; nothing is persisted to
  `c`'s file. After `claim unlock` and `claim lock --dry-run` /
  `--reason --proposal <snapshot>` on `a` alone, all three are locked and
  `ready: true`, and plain `check` is clean (0 ledger findings).
- **Read-only paths write nothing**: `check --validate` over the folded corpus
  leaves `lock-store.json` byte-identical (sha256 checked).
- **The committed fixture ledgers need no re-hash.** Every ledger record
  carrying stored content was hashed under the baseline and the candidate
  rule (14 records across `fixture-graph-demo` and `fixture-theme-flat`). No
  locked fixture claim carried tracks, so no standing record's hash moves.
  The one record whose content carried tracks is the released
  `widget.contract.reviewed` record in `fixture-theme-flat`, whose stored
  hash already matched neither hash of its content; it is left as is. The
  `Tracks` keys in stored claim snapshots were dropped (the store decodes
  them leniently and drops them on the next write, as `MigratedFrom` was in
  NIT-191). `lockedClaimHashNoOptionalFields` does not move.
- **Projection output for a track-less corpus changes only by removal.**
  `catalog.json` differs only in `fixture-theme-flat`, by the three removed
  `tracks` lists. The graph payload loses the `tracks` node key and the
  `groups.tracks` key, both `omitempty` and absent for every track-less
  corpus, so its bytes for such a corpus are unchanged. The regenerated
  viewers differ from the baseline by the removed track CSS and JS, the
  generation timestamp, and in `fixture-theme-flat` the removed track
  section and sidebar group.

## Complexity

One fewer per-claim list in the catalog and payload; no traversal, record,
witness or output-size term is added. The render budget loses the track
section projection entirely.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause or baseline changed; the
approval-integrity row it would exercise is covered directly above. This is
not release approval.

Commands: `go test -count=1 ./...` (all packages pass except three
read-only-directory tests in `internal/comments` and `internal/digest`,
which fail identically on the baseline as root and pass on the candidate as
an unprivileged user), the browser suite (`cd viewer-tests && go test
-count=1 ./...` with `DOSSIERX_TEST_BROWSER` set to Chromium 1194), where
every browser test passes and only the two goreleaser dry-run tests fail
because no goreleaser binary is installed here, `golangci-lint` v1.64.8 on
both modules (clean), and the end-to-end fold above.
