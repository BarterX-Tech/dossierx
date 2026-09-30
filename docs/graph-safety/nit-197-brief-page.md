# Graph-safety evidence: the brief page and the Briefs tree (NIT-197, v0.7.22)

Scope: viewer data. A new projection, `render.BriefsView`
(`internal/render/brief_page.go`), rendered by the embedded shell as the Briefs
sidebar group, the "All briefs" index and one page per brief; two fields on the
`dossierx-briefs` payload (`anchor`, and `body_html` now carrying `<img>` tags
on the relative `brief-assets/<folder>/<name>` path); the images a static build
copies beside the viewer (`internal/check/brief_assets.go`) and the serve route
that answers the same path (`internal/serve/brief_assets.go`); and the client
code that routes to, lists and searches the pages (`viewer-runtime.js`,
`system-record.js`). The audit round added one more reader: each module's
sidebar row carries a `data-search` index of its claims' titles, summaries
and ids (`ModuleGroup.Search`), so "Search claims and briefs" finds claims.

The brief page is a consumer, never a producer. It reads the `briefs.Set`
NIT-204 discovers, and from the catalog only each claim's `ID`, `Status`,
`ReviewPending` (the lifecycle badge, through the existing
`buildTargetStatusLookup`), `Sources` (the cited-by index), and `Summary`
(the module rows' search index). It adds no
claim edge, cause, condition, baseline or traversal; it computes no readiness;
and it writes nothing but the copied images, which are build output.

- Candidate: the head of
  `work/nit-197-viewer-brief-page-and-the-briefs-sidebar-tree-b1`, cut from
  `7f0264c3` (the NIT-178 combo branch after NIT-196 and NIT-204). The figures
  below were measured on the commit that adds this note, with the commands
  shown.
- Environment: go1.26.5 darwin/arm64.

## What the page reads, and the two judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Tree, pages, counts against caps | `briefs.Set` | every brief; `features/` left out of the tree and the index, its pages still rendered |
| Pill and sidebar mark | the brief's frontmatter `status` | `locked` or `draft`, nothing else |
| Rests on | the brief's frontmatter `rests_on` | as authored; an id that names no claim renders as a plain link (check reports it) |
| Cited by | `cat.Claims[].Sources` | a claim whose `internal` source `path`, cleaned, equals the brief's path; each claim once, sorted by id |
| Row badge | `buildTargetStatusLookup(cat)` | the badge the claim footer already draws |
| Strip findings | `/api/status` `lint_errors` / `lint_warnings` | a finding whose `claim_id` is the brief's path, its folder's or the tree's |

1. **Cited by is derived, not stored.** It is the inverse of an edge a claim
   already declares (an internal source), computed per render. No brief field
   and no claim field records it, so it cannot drift from its source. It is
   not a claim-to-claim edge: `rests_on` stays the only one, and nothing
   traverses cited-by.
2. **Brief findings are scoped by path, not by rule id.** The strip keeps a
   finding on a brief's page when its `claim_id` is one of three paths. That
   is the rule internal/briefs already writes (`claim_id` is the path a reader
   opens), so rules NIT-205 adds (drift, review, dependency) appear without a
   client change, and no brief finding can land on a claim's facet (a path
   holds a slash; a claim id never does).

## Preserved invariants

- **A claim never learns about briefs.** No claim-side package changed:

      git diff --stat 7f0264c3..HEAD -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/briefs        # empty

  The new code lives in `internal/render`, `internal/render/components`
  (`brief.go`: the claim pill and relationship-row writers, exported for the
  brief page, with no change to what they emit for a claim), `internal/check`
  (the copy) and `internal/serve` (the route).
- **Readiness is read, not recomputed; nothing is written but build output.**
  `buildBriefsView` holds a `*catalog.Catalog` and reads four fields of each
  claim. `writeBriefAssets` writes only under `build/viewer/brief-assets/`,
  which it rebuilds whole.
- **A project with no brief gains only the claim search index outside the
  stylesheet and scripts.** Every brief element in `shell.html` is
  conditional on the view (on the tree having a folder, for the search
  promise, the index and the group), and each comment hugs its action so the
  conditional adds no newline. Measured against `7f0264c3`'s committed
  viewers with `<style>`/`<script>` bodies, the render stamp and the module
  rows' `data-search` attributes masked, the four no-brief fixtures (basic,
  conformance-v1, portability, theme-flat) are byte-identical.
  `TestRenderWith_BriefsPayload` pins the render-level half (an empty set
  renders exactly what no `Extras` renders).
- **Every id on the page names one element.** `brief-<folder>-<slug>` can be
  spelled by two briefs (`a-b/c`, `a/b-c`) or by a brief and a module's facet
  (`brief-q-contract`); the later claimant takes `-2`, `-3`, in brief-id order,
  and the payload's `anchor` is the id the section carries. A page's title
  heading is `<id>_title`: the underscore is outside every brief and module
  id's alphabet, so a brief named `x-title` beside one named `x` no longer
  shares an id with the other's heading (audit F3; the first round's
  `<id>-title` made the claim above false for that pair).
  `TestRender_BriefPageIDsAreUniqueOnThePage` holds all three shapes;
  mutation: dropping the collision loop, or returning to `-title`, fails it.
- **What is copied is what was counted.** The static viewer's 64 MiB bound
  (`conformance.MaxOutputBytes`, `RenderBoundedWith`) now charges the page and
  the images the page references, from one list (`render.BriefAssets`), before
  anything is rendered; `writeBriefAssets` copies that same list and refuses a
  source that is no longer the plain file of the size discovery read
  (`TestRenderBoundedWith_ChargesBriefImagesToTheBound`: exact fit passes, one
  byte short is `conformance_capacity_exceeded`;
  `TestWriteBriefAssets_RefusesAnImageThatChangedUnderIt`). The default caps
  admit about 180 MiB of images against the 64 MiB bound, so a lint-clean
  project can reach it; the refusal then names the largest images, their
  total and the bound, and the CLI hint says to shrink or remove them
  (`render.ErrBriefImagesOverBound`, `TestCheck_BriefImagesOverTheViewerBoundAreNamed`).
  Both write paths, plain and conformance-enabled, copy the images
  (`TestRunConformanceWritesAgreementThenRemovesStaleStatus` holds the second).
- **The serve route is an allowlist.** `GET /brief-assets/{folder}/{name}`
  answers only a path `render.BriefAssets` lists for the current tree
  (`TestBriefAsset_*`). What those tests exercise for a symlinked image is
  discovery: `briefs.Load` reads the link as a non-regular entry, so it never
  reaches the allowlist. The route's own `EvalSymlinks` re-check, which
  refuses a resolved path that is not itself inside `briefs_dir`, is defence
  in depth for a link that appears between discovery and the request; no test
  opens that window, and removing the re-check passes the route tests.

## Complexity and output size

Per render: one pass over the briefs to render each body once (shared by the
page and the payload, O(total brief bytes)); one pass over every claim's
sources for the cited-by index (O(C + S)); one pass over the configured
modules and claim modules for the reserved ids (O(M)); one sort of folder
names. The id assignment probes `-2`, `-3`… only on a collision, so its worst
case is O(B²) probes for B briefs all spelling one id, which the id grammar
makes impossible beyond pairs in practice and the total cap bounds (60 by
default). No path, witness or pairwise claim term.

Output grows with the briefs' own bytes and with the cited-by rows, which are
bounded by the number of internal sources in the corpus: each source adds at
most one row to one brief. A brief body appears twice in the page (its section
and the payload's `body_html`); both are charged to the viewer's bound.

The module rows' search index is one pass over the claims already grouped
(O(C)) and adds each claim's title, summary and id to the page once: 178
bytes on fixture-basic, 5,877 on the 58-claim graph-demo, and at most about
C × (summary cap + id + title) bytes in general, charged to the same bound.

    go test ./internal/render -run '^$' -bench 'BriefsView|BriefsPayload' -benchmem -benchtime=3x

| Case | time | bytes allocated | output |
| --- | --- | --- | --- |
| payload, 60 briefs at the word cap | 2.2 ms | 3.2 MB | 619 KB payload |
| payload, 2,000 briefs (raised cap) | 75 ms | 133 MB | 20.6 MB payload |
| view, 60 briefs, 1,000 claims each citing one | 4.3 ms | 5.4 MB | 1.1 MB of sections |
| view, 2,000 briefs, 10,000 claims each citing one | 128 ms | 116 MB | 25.1 MB of sections |

At the default caps the brief sections and payload together are under 2 MB,
against a 64 MiB bound. At a raised 2,000 briefs they approach 46 MB, which is
where the bound, not memory, refuses first.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause, condition or baseline changed
and the page consumes the catalog as data. This is not release approval.
