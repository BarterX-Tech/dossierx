# Graph-safety evidence: briefs in a claim's relationships (NIT-202, v0.7.22)

Scope: a new projection in the claim card's relationships panel, the derived
BRIEFS group.

- `internal/render/depended_by_view.go`: `buildBriefsExplainingLookup` (the
  reverse of every brief's `rests_on`, beside `buildDependedByLookup`),
  `buildBriefsCitedLookup` (a claim's `internal` sources whose path is a brief
  file, with the `source-internal-drift` verdict for each), their join
  `buildBriefRelationsLookup`, and one more argument to `attachEdgesOverride`.
- `internal/render/render.go`: `renderBoundedAt` passes `Extras.Briefs` to that
  lookup, which takes each row's link from `briefAnchors` — the brief pages'
  own id map (NIT-197), computed from the same set, catalog and config.
- `internal/render/components/components.go`: `BriefRow`, `BriefRelations`,
  `writeBriefsGroup`; `EdgesHTMLWithCodeLinks` gains the group and counts its
  rows in the relationships chip.
- `internal/render/viewer/template/style.css`: the rows' desktop and phone
  layout.

Viewer data is a graph-safety trigger (AGENTS.md), which is why this note
exists. The projection adds no claim edge, cause, condition, baseline or
traversal. It reads two things the engine already has — each brief's
`rests_on` (NIT-204) and each claim's `sources` — and writes nothing: no claim
file, no store, no ledger, no sentinel.

- Baseline: `cb06088f` (NIT-197's brief page and Briefs sidebar tree, on the
  NIT-178 combo tip `7f0264c3` with NIT-196 Home and NIT-204 briefs read side).
  Candidate: the head of
  `work/nit-202-viewer-briefs-in-a-claims-relationships-derived-b5`, rebased
  onto that baseline and carrying this note; the figures below were measured
  on its final tree.
- Environment: go1.26.5 darwin/arm64 (Apple M4 Pro); browser suite on Chrome
  for Testing (Playwright chromium-1234).

## Preserved invariants

- **Claim files are never written.** Nothing in the change opens a file for
  writing; the one read it adds is `source-internal-drift`'s own read of a
  cited brief. End to end, every claim file and `constitution.yaml` of five
  fixture corpora hashes identically before and after `check`, `check
  --validate`, `claim list` and `manifest show` under both binaries (below).
  The browser suite serves a project whose claim cites two briefs and is
  explained by a third, renders it at two widths in two themes, and asserts
  the claim file is byte-identical afterwards
  (`TestBriefRowsLayOutAsTheBoardDrawsThem`, `viewer-tests/brief_relations_test.go`).
- **Claim hashes and the claim graph are unchanged.** The claim-side packages
  are untouched, and `internal/briefs` too:

      git diff --stat cb06088f -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/briefs          # empty

  `LockedClaimHash` and `ContentHash` live in `internal/lock` and read
  `model.Claim`, neither of which changed; no brief byte can reach them.
  `rests_on` remains the only claim-to-claim edge: an explaining brief is a
  row, never an edge, and nothing traverses from it. The graph payload inside
  `index.html` and every `catalog.json` are byte-identical to the baseline on
  all five corpora.
- **Readiness is unaffected.** `check` and `check --validate` envelopes
  (readiness, review causes, `lint_findings`, exit code), `claim list`, and
  `manifest show --isolation` / `--integration` for every module are
  byte-identical between the binaries on all five corpora, timestamps and the
  build version normalised. The lock store (`build/ledger/lock-store.json`) is
  byte-identical too.
- **Drift is the lint's verdict, not a second copy of it.** "pin out of date"
  is `len(sourceInternalDrift.Check(one claim, one source)) > 0`, the
  registered lint run on that one source; the hashing rule is not
  re-implemented. The verdict is memoised within one render on (path,
  record_id, sha256), the only inputs it depends on. Mutating the call to
  `false` fails `TestRenderWith_BriefsInAClaimsRelationships`.
- **No brief, no brief-derived byte.** A project with no brief gets no lookup
  entry (both builders return nil on an empty set) and `attachEdgesOverride`
  keeps its early return; the same catalog rendered with an empty set shows
  no group even for a source whose path looks like a brief's (asserted in the
  render test; relaxing the empty-set guard fails it). The engine stylesheet
  grows by the new rules for every project, as any viewer CSS change does.

## The differential, as run

`dx-base` from `git archive cb06088f`, `dx-cand` from the candidate, each run
over copies of the candidate's five fixtures (committed `build/viewer/index.html`
removed first, so each binary writes its own), with the loop and `norm` of
`docs/graph-safety/nit-204-briefs.md` plus a `shasum -a 256` of every file
under `claims/` and of `constitution.yaml` before and after:

| Corpus | Artifacts compared | Identical | Different | Claim files |
| --- | --- | --- | --- | --- |
| fixture-basic | 9 | 8 | `index.html` (stylesheet only) | identical |
| fixture-conformance-v1 | 11 | 10 | `index.html` (stylesheet only) | identical |
| fixture-portability | 9 | 8 | `index.html` (stylesheet only) | identical |
| fixture-theme-flat | 9 | 8 | `index.html` (stylesheet only) | identical |
| fixture-graph-demo | 11 | 10 | `index.html` (stylesheet, two cards) | identical |

With every `<style>` block masked, the four corpora without briefs have zero
differing lines in `index.html`; graph-demo differs in exactly two lines, the
footers of `engine.contract.build-is-pure` and `viewer.contract.render-is-pure`,
each gaining one "Explained by" row for `graph.reading-the-graph` (its
`rests_on` names both) and the chip counting it. Every `check` exited 0 on
both binaries.

## Complexity and output size

C claims, N briefs, R the longest `rests_on`, S the claims' internal sources
in total, P the longest brief path, T the longest title, B the bytes of the
distinct cited files.

| Pass | Bound |
| --- | --- |
| Explained-by lookup | a claim-id set, O(C); one lookup per `rests_on` entry, O(N·R); one sort per claim's rows, O(N·R log N) in total. Ids that name no claim are dropped, so the map holds at most C keys and N·R rows |
| Cited lookup | one path map of the briefs, O(N); one pass over the sources, O(S); one sort per claim's rows, O(S log S) |
| Drift verdicts | one lint read and hash per distinct (path, record_id, sha256), so O(B) bytes hashed — each cited brief at most once per distinct pin, not once per citing claim |
| Output | one row per explaining brief per claim it names plus one per distinct cited brief per claim: at most N·R + S rows, each O(P + T) bytes. No row enumerates a path; nothing is recursive |

The rows ride in the claim cards, so they are charged wherever the cards
already are: to the output budget on the bounded render (`check`, `serve` with
a conformance report) and, as before this change, to nothing on `serve`'s
unbounded path, whose claims have never been charged (see the NIT-204 note).

Measured with `go test ./internal/render -run '^$' -bench
BriefRelationsAtScale -benchmem` (`internal/render/brief_relations_bench_test.go`):
2,000 claims; 60 briefs (the default `max_briefs`) of ~11 KB each, each resting
on 200 claims; every claim citing three briefs with a holding pin.

| Rows derived | Card bytes added | Lookup time | Allocations | Bytes allocated |
| --- | --- | --- | --- | --- |
| 18,000 | 12,900,983 | 7.1 ms | 26,912 | 9.3 MB |

Before the verdict was memoised the same run took 261 ms and 113 MB (6,000
reads and hashes of the same 60 files); the memo is why the drift term is
O(B) rather than O(S·file).

## Exclusions

- **The readiness shape matrix** (chains, diamonds, cycles, dense DAGs) and
  witness-path bounds: no traversal, edge, cause, condition or baseline
  changed, and readiness does not read briefs or the new lookup.
- **Lock and write-path mutation proofs**: nothing here writes, and the lock
  evaluator's code and inputs are unchanged.
- **Review-pending on a brief row**: NIT-205 computes it; `BriefRow.ReviewPending`
  is never set on this candidate, and the row's rendering of it is pinned at
  the component (`TestBriefRowDrawsAPendingReviewAmberWithTheDraftDot`).

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
This is not release approval.
