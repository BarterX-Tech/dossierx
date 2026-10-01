# Graph-safety evidence: the Briefs index behind “All briefs” (NIT-203, v0.7.22)

Scope: viewer data. The existing `render.BriefsView` projection
(`internal/render/brief_page.go`) now carries a B6 **All briefs** index
(`BriefsIndex`, `internal/render/brief_index.go`) and Home's **Briefs** tile
(`BriefsView.BriefsTile`). Home's Edited after approval and To re-read cards
gain brief halves from the same `briefMark(briefs.Review)` the sidebar
already reads. `#_briefs` is unchanged (NIT-197 reserved it).

The index is a consumer, never a producer. It reads `briefs.Set`, the
effective `BriefCapLimits`, and each page's already-computed `Mark` /
`OpenThreads` / `Pill`. It adds no claim edge, cause, condition, baseline or
traversal; it computes no readiness and no brief review; it writes nothing.

- Candidate: the head of
  `work/nit-203-viewer-briefs-index-behind-all-briefs-b6`, cut from
  `36b1ed01` (the NIT-178 combo after NIT-200 #143). The figures below were
  measured on this working tree with the command shown.
- Environment: go1.26.5 linux/amd64, Cloud Agent VM.

## What the index reads, and the judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Groups | `BriefsView.Folders` | every folder except `features/`, file-system (name) order |
| Folder label | `components.DisplayCase` | the same title-case the sidebar already uses |
| Folder pair | `len(pages)` / `set.Caps.PerFolder` | "N of 12"; over → red + `brief-folder-cap` |
| Inclusive total | `len(set.Briefs)` / `set.Caps.Total` | "11 of 60, 5 are features"; over → `brief-total-cap` |
| State totals | each non-feature page's `Mark` and `OpenThreads` | locked / review / edited from `briefMark`; threads are the open-thread sum |
| Row pill | `BriefPageView.Pill` | the one lock/review pill `briefMark` already chose |
| Collapse | page marks | open iff any page is edited, review or thread |
| Home Briefs tile | `BriefsIndex` | hidden when no non-feature folder; links to `#_briefs` |
| Home edited / review cards | `homeBriefWaitingOf` | `briefMark` on every brief, features included |

1. **`briefMark` is still the one mark function.** The index, the tile, the
   waiting-card halves, the Features list and the Briefs tree all call it.
   Nothing here re-implements lock or review.
2. **Features are excluded from the groups, not from the 60.** The tile's
   count is non-feature; its cap line is inclusive. Waiting cards count
   feature briefs: they are briefs in those states.
3. **Caps are the effective config values**, not the defaults. A human-raised
   `max_briefs` / `max_briefs_per_folder` is what the meter prints.

## Preserved invariants

- **A claim never learns about briefs.** No claim-side package changed:

      git diff --stat 36b1ed01 -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/briefs        # empty

- **Readiness is unread; nothing is written.** `buildBriefsIndex` walks the
  folders `buildBriefsView` already assembled. `homeBriefWaitingOf` is one
  pass over `set.Briefs`.
- **A project with no brief, or only `features/`, renders as before outside
  the stylesheet and scripts.** The index and the Briefs tile stay inside
  `{{if .Folders}}` / `BriefsTile.Count`. `TestRenderWith_BriefsPayload` and
  `TestRender_FeaturesOnlyProjectMakesNoBriefsPromise` still hold.
- **Every id on the page still names one element.** The index reuses
  `#_briefs` and each brief's existing page id. `TestRender_BriefPageIDsAreUniqueOnThePage`
  still holds.
- **Author text is escaped.** Titles and summaries go through `html/template`.
  `TestRender_BriefsIndexEscapesAuthorMarkup` pins it.

## Complexity and output size

Per render: one pass over the folders already built (O(N) briefs), plus one
pass over the set for Home's waiting halves. No path, witness or pairwise
term.

    go test ./internal/render -run '^$' -bench BenchmarkBriefsIndex -benchmem -benchtime=3x

| Case | time | bytes allocated | structure |
| --- | --- | --- | --- |
| 60 briefs (default total cap), 12 per folder, 1/5 features | 0.44 ms | 289 KB | 5 index folders, 12 features |
| 2,000 briefs (raised cap) | 12.8 ms | 9.3 MB | 167 folders, 400 features |

At the default caps the index is a few hundred kilobytes of allocation on
the way to the page `buildBriefsView` already built. The 2,000-feature /
2,000-brief view-built-before-64-MiB-bound item stays backlog.

## Verdict

PASS for this change: every applicable obligation above ran on the
candidate. Excluded, with reason: the readiness shape matrix (chains,
diamonds, cycles, dense DAGs), because no traversal, edge, cause, condition
or baseline changed. This is not release approval.
