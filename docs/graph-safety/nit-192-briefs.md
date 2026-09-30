# Graph-safety evidence: briefs lock, review and comments (NIT-205, v0.7.22)

Scope: the write side of briefs (NIT-192's second half) and every consumer
that reads it.

- `internal/lock`: the `briefs` map in `build/ledger/lock-store.json`
  (`briefs.go`: `BriefRecord`, `RecordBriefApproval`, `RecordBriefReaudit`,
  `ReleaseBriefApproval`, `RetainedClaimContent`), the strict decoder's key and
  version whitelist, and the version-4 schema.
- `internal/briefs`: `LockHash`, the frontmatter's `comments` key, the
  frontmatter rewrites (`write.go`), and the lifecycle read (`lockstate.go`:
  `Evaluate`, `Baselines`, the four findings).
- `internal/digest`: the `briefs` map of the comment digest store.
- `internal/comments`: the brief methods on `Deps` (`briefs.go`), and the six
  thread operations factored out of the claim methods so both share them.
- `internal/check`: brief findings in `lint_findings` (`FindingsWith`) and in
  the ledger gate (`briefLedgerFindings`), the review state handed to the
  render, `OpenBriefComments`.
- `internal/render`: the review fields of the `dossierx-briefs` payload.
- `internal/serve`: the review state read against the same store load as
  readiness.
- `cmd/dossierx`: `brief lock`, `brief unlock`, `brief reaudit`; review state in
  `brief list` / `brief show`; brief paths in `comment add|reply|list|inbox`.

This change touches lock state and a review-propagation edge, which is why the
note exists (AGENTS.md). The edge is new and one-directional: a locked brief
records, per `rests_on` claim, that claim's `ContentHash` as a baseline, and a
claim whose hash moved makes the BRIEF review-pending. Nothing points the other
way.

- Baseline: `7f0264c3` (release/v0.7.22 after NIT-204, PR #133). Candidate: the
  head of `work/nit-205-engine-192b-briefs-lock-review-propagation-and-comment`
  carrying this note; the figures below were measured on the working tree of
  that branch before this note was committed, with the commands shown.
- Environment: go1.26.5 darwin/arm64 (Apple M4 Pro).

## Graph facts, stated before the proofs

- **The facts that determine a brief's state**: its `status` line, its lock
  hash, and the standing record under `briefs[<id>]` (hash, baselines). Lock
  state is a function of those three alone; review-pending additionally of the
  claims' current `ContentHash`.
- **Independent obstacles**: content drift (the brief moved) and dependency
  drift (a claim moved) are independent. Clearing one does not clear the
  other: `brief reaudit --confirm` refreshes baselines and refuses a brief
  edited since approval (`integrity_failed`), and `brief lock` re-signs the
  brief's text and re-records the baselines in one approval.
- **Identity**: a record is keyed by brief id (`<folder>.<slug>`); a baseline
  by claim id within its record, exactly as `Store.Hashes[dependent][dep]`. A
  changed claim is reported once per (brief, claim).
- **No traversal**: a brief's baselines are one hop. No claim-to-claim path is
  followed from a brief; the claim graph's own propagation is untouched.

## Preserved invariants

- **A claim never learns about briefs.** The claim-side packages still do not
  import `internal/briefs`, and none of them changed except `internal/lock`'s
  store plumbing:

      go list -deps ./internal/lock ./internal/readiness ./internal/catalog \
        ./internal/model ./internal/lint ./internal/manifest ./internal/loader \
        ./internal/reaudit | grep -c internal/briefs              # 0
      git diff --stat 7f0264c3 -- internal/readiness internal/catalog \
        internal/model internal/lint internal/manifest internal/loader \
        internal/reaudit internal/lock/lockedhash.go internal/lock/audit.go \
        internal/lock/ledger.go internal/lock/policy.go            # empty

  In `internal/lock`, the only non-test reader of `Store.Briefs` is
  `briefs.go`; `lock.go` gains the field, its strict-decode key and version,
  and the decode line. `ContentHash`, `LockedClaimHash`, `Audit`,
  `DetectStale`, the lock policy and readiness read `Hashes`, `Receipts`,
  `LockedAt`, `Ledger` and `Constitution` only, so no brief byte enters any
  claim hash, baseline, cause or finding.
  `TestBriefRecordsNeverReachAClaimRule` (`internal/lock/briefs_test.go`)
  holds a standing and a released brief record naming a claim and asserts the
  claim's two hashes, the store's claim maps and `Audit`'s output unchanged.
- **Nothing flows back.** `TestBriefStateNeverGatesAClaim`
  (`cmd/dossierx/brief_lock_cli_test.go`) raises `brief-content-drift` and
  `brief-unrecorded` in one project and asserts `claim lock --dry-run` is not
  blocked and names no `brief-` rule, that the lock succeeds, and that the
  claim file carries no `review_pending`. End to end, on
  `testdata/fixture-graph-demo` (`brief-state.sh` below): a locked brief edited
  since approval AND resting on a claim that moved, against the same corpus
  with no briefs at all, gives byte-identical `claim list`, `manifest show
  --isolation` / `--integration` for every module, `catalog.json` (readiness
  included), viewer `index.html` outside the `dossierx-briefs` line, and the
  locked claim files after a writing `check` (so no `review_pending` was set).
  The claim findings are identical; the brief corpus adds exactly
  `brief-content-drift` (ledger) and `brief-dependency-drift` (lint warning).
- **A draft is never review-pending, and readiness writes nothing.**
  `TestEvaluate_TheLockStatesAndTheirFindings` (`internal/briefs`) — eight
  rows: draft with and without a record whose claims moved, locked and
  unchanged, unrecorded, released-then-typed-locked, edited, a moved claim, a
  gone claim — asserts the lock state, the review verdict and exactly the
  findings each raises. `Evaluate` takes values and returns values; the only
  writers of a brief record are the three verbs, under the claims and
  lock-store sentinels.
- **No brief, no change.** On five corpora the baseline and candidate binaries
  give byte-identical `check` and `check --validate` envelopes, `claim list`,
  both manifest views for every module, `catalog.json`, the three ledger
  stores and the viewer (timestamps and version normalised). With its one
  draft brief, graph-demo differs in one line of `index.html`, the
  `dossierx-briefs` payload, which gained the review fields:

  | Corpus | Artifacts compared | Identical |
  | --- | --- | --- |
  | fixture-basic | 9 | 9 |
  | fixture-conformance-v1 | 10 | 10 |
  | fixture-portability | 9 | 9 |
  | fixture-theme-flat | 9 | 9 |
  | fixture-graph-demo, `briefs/` removed | 10 | 10 |
  | fixture-graph-demo, with its brief | 10 | 9 (`index.html`: the payload line) |

- **The store's claim side is untouched by the schema bump.** Version 4 is
  earned, not stamped: `storeSchemaVersion` stays 3 for every claim write and
  every fresh store, and only the first brief record raises a store to 4. A
  v0.7.21 (version 3) store decodes strictly and leniently, saves back at
  version 3 with no `briefs` key, and after a brief record its claim maps are
  identical (`TestAV0721StoreLoadsAndEarnsTheBriefsSchemaOnlyWhenABriefIsRecorded`).
  The strict decoder refuses a `briefs` map below version 4
  (`TestDecodeStoreRefusesABriefsMapBelowVersionFour`). No claim-side test or
  golden changed: `internal/lock`, `internal/readiness`, `internal/catalog`
  and every claim test pass unedited.
- **Both check modes judge the same tree.** `--staged` reads the briefs, the
  lock store and the comment digest from the index.
  `TestBriefLockFindingsFollowTheTreeEachModeJudges` (`internal/check`) runs
  five rows — content drift, unrecorded, dependency drift, a gone claim, a
  hand-edited brief comment block — each unstaged (reported by `--validate`,
  not by `--staged`) and then staged (both agree).
  `TestAStagedBriefLockTravelsWithItsRecord` stages a brief's `status:
  locked` without the store (`brief-unrecorded` at commit) and then with it
  (clean).
- **Writes are guarded.** A brief file is rewritten only in its status token
  and its `comments` block, and every rewrite is re-parsed and refused if the
  lock hash moved (`TestSetStatus_RewritesOnlyTheStatusToken`,
  `TestSetComments_RoundTripsAndTouchesNothingElse`). A brief that is a
  symlink is refused by discovery and so unreachable by every verb; the comment
  ops refuse a brief that changed between their read and their write, and do
  not write through a link (`TestBriefThreads_NeverWriteThroughALinkOrOverAnEdit`).
  `brief lock` and `brief unlock` make the same two checks before their write
  (`rewriteBriefFile`); that path has no test of its own. Every `--dry-run`
  leaves the project byte-identical (`TestBriefWriteDryRunsWriteNothing`).

## Complexity and output size

N briefs, R the longest `rests_on`, C claims, W the bytes of one claim's
wording (summary, body, steps), B the briefs' total bytes, S the store's
retained claim snapshots.

| Pass | Bound |
| --- | --- |
| `Evaluate` | a claim-id index O(C); per brief one record lookup and one `ContentHash` per baseline, O(N·R) hashes of claims, each O(W) |
| Baseline wording | per changed claim one receipt lookup; only a receipt that is missing or does not match falls back to `RetainedClaimContent`, one pass over S map entries hashing only same-id snapshots |
| `FindingsWith` | the brief rules as before plus the lifecycle findings, O(N·R) findings of bounded length |
| Ledger gate (briefs) | `Evaluate` plus one digest per brief with threads, O(N·R + comment bytes) |
| Payload | the NIT-204 payload plus, per brief, the approved markdown and its rendered HTML, O(B), and per changed claim two wordings, O(N·R·W); charged to the same render budget |
| `brief lock` / `reaudit --confirm` | one discovery, one claim load, R hashes and receipts, one store save: O(F log F + C + R·W + store bytes) |

Nothing enumerates a path or a pair: every term is linear in briefs, baselines
or bytes.

Measured, every baseline drifted and every wording retained (the worst case
for the review payload), claims of about 1,800 bytes:

| Briefs × rests_on | `Evaluate` | allocations | bytes allocated | changed claims |
| --- | --- | --- | --- | --- |
| 60 × 10 | 1.8 ms | 12,803 | 1.1 MB | 600 |
| 600 × 10 | 17.5 ms | 127,308 | 11.8 MB | 6,000 |
| 2,000 × 10 | 55 ms | 424,141 | 40.6 MB | 20,000 |

| Briefs (word cap, 10 moved claims each) | `briefsPayloadJSON` | allocations | bytes allocated | payload bytes |
| --- | --- | --- | --- | --- |
| 60 | 10.9 ms | 1,237 | 21 MB | 4,208,230 |
| 2,000 | 252 ms | 40,035 | 641 MB | 140,269,333 |

The 2,000-brief payload is over the 64 MiB render budget, so that render is
refused at the payload (`conformance_capacity_exceeded`), exactly as an
oversized NIT-204 payload is; the bound is the budget, not the caps. Under the
default caps (60 briefs) a fully drifted project's payload is about 4 MB.

    go test ./internal/briefs -run '^$' -bench EvaluateAtScale -benchmem
    go test ./internal/render -run '^$' -bench BriefsPayload -benchmem

## The differentials, as run

The no-brief differential is NIT-204's script
(`docs/graph-safety/nit-204-briefs.md`, "The differential, as run") with
`dx-base` built from `git archive 7f0264c3`. Its only output is `graph-demo
build/viewer/index.html DIFF`, two lines in the normalised diff: the old and
the new payload line.

The brief-state differential (`brief-state.sh R W`, with `$R/dx-cand` the
candidate binary and `DOSSIERX_ACTOR=proof`):

```bash
P=$R/p
prep() {
  rm -rf $P; cp -R $W/testdata/fixture-graph-demo $P; cd $P
  $R/dx-cand brief lock briefs/graph/reading-the-graph.md --reason proof --format text >/dev/null
  sed -i '' 's/nothing it infers\./nothing it infers, edited./' briefs/graph/reading-the-graph.md
  f=$(grep -l "^id: engine.contract.build-is-pure" -r claims)
  sed -i '' 's/^summary: \(.*\)$/summary: \1 (moved)/' $f
}
out() {
  cd $P
  $R/dx-cand check > $R/$1.check.json 2>&1
  $R/dx-cand check --validate > $R/$1.validate.json 2>&1
  $R/dx-cand claim list > $R/$1.list.json
  for m in $(ls claims | grep -v '\.'); do
    $R/dx-cand manifest show $m --isolation; $R/dx-cand manifest show $m --integration
  done > $R/$1.manifest.json
  cp build/catalog/catalog.json $R/$1.catalog.json
  grep -v 'id="dossierx-briefs"' build/viewer/index.html \
    | sed -E 's/20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z/TS/g' > $R/$1.viewer.html
  for c in $(grep -rl "^status: locked" claims); do cat $c; done > $R/$1.claimfiles.txt
}
prep; out A
prep; rm -rf briefs   # and the store's briefs map, version back to 3
out B
for k in list.json manifest.json catalog.json viewer.html claimfiles.txt; do
  cmp -s $R/A.$k $R/B.$k && echo "$k identical" || echo "$k DIFF"
done
```

All five are identical. Both runs of `check` and `check --validate` exit
`integrity_failed` on `lock-content-drift` for the edited claim; A adds
`brief-content-drift` (ledger) and `brief-dependency-drift` (lint), nothing
else.

## Compatibility with v0.7.21, measured

With a v0.7.21 binary built from the tag, on a project whose store the
candidate moved to version 4 by `brief lock`:

- v0.7.21 `check --staged` passes the commit (exit 0): it reads the store
  leniently and knows no briefs. It does not refuse; it judges no brief.
- v0.7.21 `claim lock` and `constitution lock` rewrite the store without the
  `briefs` map, keeping `version: 4`; the candidate then reports every locked
  brief as `brief-unrecorded` (fail-closed, loud).
- v0.7.21 `comment add` rewrites the digest store without its `briefs` map;
  the candidate then reports the brief's threads as
  `comment-digest-unrecorded`.

CHANGELOG "Upgrading" says so, rather than that an old hook refuses.

## Exclusions

- **The claim readiness shape matrix** (chains, diamonds, cycles, dense DAGs)
  and witness bounds: no claim traversal, edge, cause, condition or baseline
  changed; `internal/readiness` is byte-identical to the baseline and does not
  import `internal/briefs`. The brief edge is one hop with no traversal, and
  its only shape rows are covered directly: single edge (a moved claim), many
  briefs on one claim and one brief on many claims (the benchmarks, every
  baseline drifted), a missing node (a gone claim, `brief-rests-on-missing`).
- **Cycles**: a brief is not a claim and no claim can rest on a brief, so a
  cycle through a brief cannot be written.
- **Catalog and graph-payload projections**: neither carries briefs; they are
  byte-identical on every corpus above, including the brief-state pair.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
This is not release approval.
