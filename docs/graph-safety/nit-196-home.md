# Graph-safety evidence: the viewer Home page (NIT-196, v0.7.22)

Scope: a new viewer projection, `render.HomeView` (`internal/render/home_view.go`),
rendered by the embedded shell as the Home page, and the sidebar and routing
changes around it (`shell.html`, `viewer-runtime.js`, `system-record.js`).
Home is a consumer of readiness, never a producer: it reads the live
`readiness.Assessment` already on the catalog, the approved-edit projection,
each claim's comments and the module groups the shell already builds. It adds
no edge, cause, condition, baseline or traversal, and writes nothing.

- Candidate: the head of `claude/project-thread-u0i976`, based on
  `release/v0.7.22` at `a17942e`.
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

- **Readiness is read, not recomputed.** `buildHomeView` takes the catalog by
  value and reads `Readiness`, `ApprovedEdits` and `Claims`; it calls no
  readiness, lock or reaudit function and holds no writer. The lock store,
  flags, receipts and claim files are untouched by construction.
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

One pass over the claims plus one over each claim's comments (O(V + T)), one
sort of the claims by id (O(V log V)) and one pass over the module groups. No
path, witness or pairwise term. Output is bounded independently of corpus
size: a card names at most three claims and the draft card at most three
modules, each followed by a count of the rest.

Measured on a synthetic corpus in which every claim carries an open thread and
an `upstream_dependency_review` cause and half are drafts (the worst case for
Home: every card populated by every claim):

| Claims | `buildHomeView` | Home section bytes |
| --- | --- | --- |
| 10 | 17 µs | 3,624 |
| 1,000 | 0.62 ms | 3,648 |
| 10,000 | 6.0 ms | 3,660 |

The byte growth is the digits of the counts.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause, condition or baseline changed
and Home consumes the evaluator's output as data. This is not release approval.
