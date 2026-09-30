# Graph-safety evidence: the viewer Home page (NIT-196, v0.7.22)

Scope: a new viewer projection, `render.HomeView` (`internal/render/home_view.go`),
rendered by the embedded shell as the Home page, and the sidebar and routing
changes around it (`shell.html`, `viewer-runtime.js`, `system-record.js`),
plus one field on `dossierx serve`'s `/api/fragment` answer (`generated_at`,
the render stamp read off the same render's sidebar, so a live reload moves
Home's "last check").
Home is a consumer of readiness, never a producer: it reads the live
`readiness.Assessment` already on the catalog, the approved-edit projection,
each claim's comments and the module groups the shell already builds. For the
Constitution tile it also reads two files the renderer did not read before:
`constitution.yaml` (a second read; the Constitution section already reads
it) and the lock store's constitution record, which it passes to
`constitution.Evaluate`, the roof gate's own evaluator. It adds no edge,
cause, condition, baseline or traversal, and writes nothing.

- Candidate: the head of `claude/project-thread-u0i976` (PR #131), based on
  `release/v0.7.22` at `a17942e`. The figures below were measured on the
  commit that adds this revision of the note, with the commands shown.
- Environment: go1.26.5 darwin/arm64.

## What Home reads, and the one judgement it makes

| Card | Source | Rule |
| --- | --- | --- |
| Edited after approval | `cat.ApprovedEdits` | a claim with a released approval whose text moved |
| To re-read | `cat.Readiness[id].ReviewCauses` | a **locked** claim with a `direct_dependency_change` or `upstream_dependency_review` cause |
| Open threads | `claim.OpenThreadIDs()` | every open thread, counted per thread |
| Draft claims to lock | `claim.Status` | every draft, grouped by module |

"To re-read" deliberately does not use the assessment's `review_pending`
boolean. That boolean is the OR of every independent cause, including
`own_thread` and `unapproved_edit`, which Home counts on their own cards, and
the ledger integrity causes (`approval_*`), which the status strip owns. Using
it would count one claim on two cards and label an integrity finding "something
it rests on changed". Home reads the causes the evaluator emitted and never
re-derives them; the saved `review_pending` field on the claim file is not read.

## Preserved invariants

- **Readiness is read, not recomputed.** `buildHomeView` takes a
  `*catalog.Catalog` and only reads `Readiness`, `ApprovedEdits` and `Claims`;
  it calls no readiness or reaudit function and holds no writer. The one lock
  call is `lock.LoadStore`, a read; the store, flags, receipts and claim files
  are untouched by construction.
- **The roof's state is the gate's.** The tile, the sidebar lock icon and
  the Constitution page's word meter show `constitution.Evaluate`'s verdict
  against the store's record, the same
  evaluator `check` refuses on, so a hand-flipped `status: locked` with no
  record reads "Not locked" and a locked file edited since reads "Edited since
  lock" (`TestRender_HomeEmptyStates`).
- **Independent boundaries stay independent.** A claim appears on the re-read
  card only through its own dependency causes; an open thread or an unapproved
  edit on the same claim is counted where it belongs and does not clear or
  hide the other (`TestRender_HomeWaitingOnYou` pins a claim that is
  review_pending only through `own_thread` and asserts it is not counted as
  "to re-read").
- **Other projections are unchanged.** `catalog.json` and the graph payload do
  not change; the regenerated fixture viewers differ from the baseline only by
  the Home section, the sidebar markup, and the CSS and JS that draw them.

## Complexity and output size

Two passes over the claims (the cards, and the project-claim count for the
tile) plus one over each claim's comments (O(V + T)), one sort of the claims
by id (O(V log V)), one pass over the module groups, and two small file reads
(`constitution.yaml` and the lock store). No path, witness or pairwise term.
Home is built once per render: eagerly for the embedded shell, and memoised
(`sync.Once`) on the lazy path a project shell override uses, however many
times the template references `.Home`. Output is bounded independently of
corpus size: a card names at most three claims and the draft card at most
three modules, each followed by a count of the rest.

Measured on a synthetic corpus in which every claim carries an open thread and
an `upstream_dependency_review` cause and half are drafts (the worst case for
Home: every card populated by every claim):

| Claims | `buildHomeView` time | allocations | bytes allocated |
| --- | --- | --- | --- |
| 10 | 6.5 µs | 98 | 32 KB |
| 1,000 | 0.58 ms | 2,602 | 2.6 MB |
| 10,000 | 6.0 ms | 25,125 | 40 MB |

Time and allocations come from `BenchmarkBuildHomeView` (in
`internal/render/home_view_test.go`; it builds without a config, so the two
file reads are not in the figures). The rendered Home section is 3,660 bytes
at 10 claims and 3,695 bytes at 2,000 (five modules, every claim on every
card), as `TestRender_HomeSizeIsIndependentOfCorpusSize` logs; the growth is
the digits of the counts, and the test fails if it exceeds 64 bytes or the
draft card names more than three modules. That the swapped subtrees carry no
render stamp is pinned by `TestRender_SwappedSubtreesCarryNoRenderStamp` and
`TestFragment_ReturnsBothSubtrees`.

    go test ./internal/render -run '^$' -bench BuildHomeView -benchmem
    go test ./internal/render -run 'TestRender_Home|TestRender_Swapped' -count=1 -v   # logs the section bytes

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause, condition or baseline changed
and Home consumes the evaluator's output as data. This is not release approval.
