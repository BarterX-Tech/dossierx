# Graph-safety evidence: the Features entry and the feature page (NIT-201, v0.7.22)

Scope: viewer data. A brief in `briefs/features/` is a feature. This change
adds, all in `internal/render` and the embedded shell:

- the **Features** sidebar entry, one row per feature
  (`BriefsView.Features`, ordered by file name without `.md`, with the brief's title);
- the **feature page**: the brief page (NIT-197) with a `FeatureDetail`
  (`internal/render/feature_page.go`), whose Made of list groups the brief's
  `rests_on` by module and whose meta line counts how many of them are locked;
- Home's **Features tile** (`BriefsView.FeaturesTile`);
- a rewrite of every brief page's body links that name another brief's file
  to that page's id (`resolveBriefLinks`), for every brief, not only features;
- client code: `resolve()` maps a hash naming a brief's path to its page, the
  "On this page" rail gains the Made of row, and the status strip shows a
  brief's warnings (findings whose `claim_id` is its path) as rows of their
  own: `brief-dependency-drift` under Needs you, the duplicate under Check,
  naming and leading to the other brief.

The feature page is a consumer, never a producer. It reads the `briefs.Set`
NIT-204 discovers; NIT-205's `briefs.Evaluation`, already computed for the
payload, for each brief row's and feature's lock state, review_pending and open threads; and,
from the catalog, each claim's `ID`, `Module`, `Scope`, `Status` and
`ReviewPending` (the last two through the existing `buildTargetStatusLookup`,
for the row badge). It adds no claim edge, cause, condition, baseline or
traversal; it computes no readiness and no brief review; it writes nothing.

- Candidate: the head of
  `work/nit-201-viewer-features-entry-and-the-feature-page-b4`, rebased onto
  `e6d1d968` (the NIT-178 combo branch after NIT-196, NIT-204, NIT-197,
  NIT-202, NIT-205 and its follow-up #137, NIT-198 #138, and the
  skills-only NIT-193 #139 and NIT-190 #140). The benchmark
  figures were measured on the rebased head with the command shown.
- Environment: go1.26.5 darwin/arm64, Apple M4 Pro.

## What the feature page reads, and the judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Features rows, pages, tile | `briefs.Set`, folder `features` | every brief there, in brief-id order, which is file name without `.md` |
| Row mark (Features list and Briefs tree), tile state | `briefs.Evaluation` (NIT-205): `LockState`, `ReviewPending`, `OpenThreads` | one function, `briefMark` (`brief_page.go`, NIT-198's, now reading the evaluation): edited, then review, then thread, then draft or locked; unrecorded is drawn draft. With no evaluation (a render that passes none), the frontmatter `status` and the brief's own open threads |
| Page pill | the brief's frontmatter `status` | as NIT-197 draws it |
| Made of groups | the brief's `rests_on`, `cat.Claims[].Module`/`Scope` | one group per module id (the title-cased label is shown, never the key, so `a-b` and `a_b` stay apart; project claims under a sentinel key), in the order its first claim appears; rows in `rests_on` order; a project claim under "Project"; an id that names no claim last, under "Not a claim" |
| "all locked" / "M of N locked" | `cat.Claims[].Status` | N is `len(rests_on)`; M counts ids naming a claim whose `status` is `locked` |
| Row badge | `buildTargetStatusLookup(cat)` | the badge NIT-197's Rests on row draws |
| Body links | other briefs' `Path` and page id | a link naming `briefs/<folder>/<slug>.md`, relative to the brief or from the root, becomes `#<page id>` |
| Strip rows | `/api/status` `lint_warnings` | on a brief's page, each warning whose `claim_id` is its path: `brief-dependency-drift` is Needs you ("N claims this feature rests on have changed since approval"); `brief-rests-on-duplicate` is Check and names the other brief, read from the finding's message |

1. **"Locked" is the claim's `status`, not its readiness.** A locked claim
   that is `review_pending` still counts as locked in "M of N locked", as the
   relationship badge already draws it (`lifecycleModifier`). The meta line
   reports what the file records; readiness stays the claim card's and the
   strip's to show. Nothing here reads `cat.Readiness`.
2. **The Made of list is `rests_on` as authored, grouped, never reordered
   within a module.** It deduplicates nothing: a repeated id is a
   `brief-frontmatter` error, and where a page is still rendered beside that
   error (under `dossierx serve`) the id shows, and counts, as often as the
   file writes it.
3. **The page id of a linked brief is the payload's `anchor`.** Both are
   `renderedBrief.anchor`, set once by `briefAnchors`, which is what keeps a
   colliding spelling pointing at the right section. The link is rewritten at
   render, inside `<main>`, rather than resolved by reading the
   `dossierx-briefs` block in the browser: that block sits outside `<nav>`
   and `<main>`, so a live reload under `dossierx serve` does not swap it and
   a client lookup would go stale.
4. **No built, specified or conformance state.** No field of
   `catalog.Conformance`, implementation links or observations is read.

## Preserved invariants

- **A claim never learns about briefs.** No claim-side package changed:

      git diff --stat e6d1d968..HEAD -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/briefs        # empty

- **Readiness is read, not recomputed; nothing is written.**
  `buildFeatureDetail` holds a map from claim id to module label and a
  locked bit, built once per render from `cat.Claims`, and the lookup the
  claim footer already uses.
- **A project with no `features/` renders as before, outside the stylesheet
  and scripts.** Every feature element in `shell.html` is conditional on
  `.Briefs.Features` (the entry, the tile) or on `.Feature` (the page's
  kicker, Made of and Comment label), and each comment hugs its action so the
  conditional adds no newline. The rewrite of body links changes nothing for
  a body with no link to another brief. Measured against `e6d1d968`'s
  committed viewers with `<style>`/`<script>` bodies and the render stamps
  masked, and nothing else, the four fixtures without a `features/` folder
  (basic, conformance-v1, portability, theme-flat) are byte-identical.
  `TestRender_NoFeatureNoFeatureMarkup` pins the render-level half.
- **Every id on the page still names one element.** A feature's page id is
  a brief's, assigned by `briefAnchors` against every module and brief id;
  the entry, the Made of list and the tile add no id.
  `TestRender_BriefPageIDsAreUniqueOnThePage` still holds.

## Complexity and output size

Per render: one pass over the claims for the module/locked map (O(C)), taken
only when `features/` holds a brief; one pass over each feature's `rests_on`
(O(R) in total); one pass over every brief body for the link rewrite (O(total
brief bytes)), with one map lookup per link. No path, witness or pairwise
term.

Output grows with the Made of rows (one relationship row per `rests_on`
entry, bounded by the frontmatter the author wrote) and is otherwise the
brief page's. The tile adds one row per feature.

    go test ./internal/render -run '^$' -bench 'FeaturesView' -benchmem -benchtime=3x

| Case | time | bytes allocated | output |
| --- | --- | --- | --- |
| 12 features (the default folder cap), 200 `rests_on` each, 12 links each | 4.2 ms | 12.3 MB | 1.2 MB of bodies and Made of rows |
| 2,000 features (raised cap), 50 `rests_on` each, 200 links each | 330 ms | 684 MB | 66 MB of bodies and Made of rows |

At the default caps the feature pages add about a megabyte even at an
implausible 200 claims per feature. At a raised 2,000 features they pass the
64 MiB viewer bound, which refuses the render, but only after the view is
built in full: the refusal bounds the output, not the memory, and the case
above allocates about 684 MB on the way. Charging the feature view to the
render budget as it is built is backlog, not this change.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause, condition or baseline changed
and the page consumes the catalog as data. This is not release approval.
