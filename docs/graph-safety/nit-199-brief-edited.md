# Graph-safety evidence: a brief edited since its approval (NIT-199, v0.7.22)

Scope: viewer data. The brief page (`render.BriefsView`, NIT-197) now reads
each brief's lock and review state (`briefs.Review`, NIT-205), which the
render already carried for the `dossierx-briefs` payload, and draws what it
says: the sidebar mark by the legend's priority (`briefMark`), the header pill
for an edited and an unrecorded brief (`components.BriefLockPillHTML`), the
approval's date and reason in the meta line (`briefApproval`), and, for a
locked brief whose file moved since its approval, the banner and the Changes /
Approved / Current views (`render.RenderBriefDiff`, `briefEditView`,
`internal/render/brief_edit.go`). The views are rendered into the page's own
section, like the body, so a live reload's fragment swap carries them. The
client code only picks the shown view (`viewer-runtime.js` `syncBriefEdits`)
and lists the shown view's headings in "On this page" (`system-record.js`
`briefHeadings`).

One shared helper moved: `internal/approvaledit`'s passage diff, which
`Compute` ran inline for a claim, is now `approvaledit.Passages(before, after,
render)`, and its word-marking renderer `RenderPassageWith(hunk, render)`.
`Compute` calls them with the claim renderer it always used, so a claim's
`approved_edit` is computed by the same code path as before; the brief page
calls them with the brief renderer (document mode, images on
`brief-assets/<folder>/`).

The page is a consumer, never a producer. Whether a brief is edited since its
approval is `briefs.Evaluate`'s `LockEdited` — the predicate that raises
`brief-content-drift` — and the approved text is the record's own
`Review.Approved`. Nothing here computes a lock state, a review verdict or a
baseline, and nothing writes.

- Candidate: the head of
  `work/nit-199-viewer-brief-edited-since-approval-with-changes-approved`, rebased
  onto `41f603be` (the NIT-178 combo branch after NIT-196, NIT-204, NIT-197,
  NIT-202, NIT-205 and its #137 follow-up). The figures below were measured on the working tree
  of that branch before this note was committed, with the commands shown.
- Environment: go1.26.5 darwin/arm64 (Apple M4 Pro).

## What the page reads, and the judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Pill | `Review.LockState`, else the frontmatter `status` | `edited` → EDITED SINCE APPROVAL; `unrecorded` → LOCK NOT RECORDED; otherwise NIT-197's Locked / Draft |
| Sidebar mark | `Review.LockState`, `ReviewPending`, `OpenThreads`, `status` | the first that holds of edited, review, thread, draft, locked; unrecorded takes draft |
| Meta line | `Review.LockedAt`, `LockReason` | only for `locked` and `edited`, the states with a standing approval |
| Banner and views | `Review.LockState == edited`, `Review.Approved` | the diff of `Approved.Markdown` against the brief's body; no approved markdown (a record `brief lock` did not write) shows the recover note |
| Images that moved | `Review.ApprovedImages` against `Brief.ImageDigests()` | changed, added, removed by digest, the comparison `ContentMoved` makes for the finding |
| Whitespace only | `briefs.LockHash` of the approved text against `Brief.LockHash` | the text's hash moved while no passage, summary or `rests_on` id reads differently |

1. **An unrecorded brief is not drawn as locked.** Its file says `locked`,
   but `brief-unrecorded` means no approval stands; the pill says LOCK NOT
   RECORDED and the mark is the draft mark. This is the honest reading of the
   state NIT-205 defines, and it changes only what the page draws.
2. **The mark follows the legend's priority.** The review and open-thread
   marks are drawn from fields NIT-205 already puts in the review; their
   pages (NIT-200, NIT-198) are not built here.
3. **Trailing blank lines do not make a passage differ.** `textdiff` keeps a
   passage's trailing blank lines on it, so appending a section to a brief
   would redline the unchanged last passage above it. The brief diff ends both
   bodies with exactly one blank line before diffing (`endPassage`); a
   markdown body renders the same whatever trails its last line. The claim
   diff is unchanged.
   Because the lock still signs those bytes, an edit that changes nothing
   else is named for what it is ("only whitespace changed"), decided from
   the hash, never guessed as an image.
4. **A passage and its replacement count once.** A run of removed passages
   and the added run that replaces it count as the longer of the two.
   `approvaledit.Passages`' own count, which the claim viewer shows, pairs a
   removal only with the addition directly after it and so reads nine for
   five paragraphs reworded in a row; it is left as it is here.

## Preserved invariants

- **A claim never learns about briefs; no claim-side package changed** but
  the `approvaledit` refactor:

      git diff --stat 41f603be -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/textdiff               # empty

  `internal/briefs` gains one field, `Review.ApprovedImages`: a copy of the
  standing record's image digests, `json:"-"` so the payload is unchanged,
  which the page compares with the brief's own to name the images that
  moved. `Evaluate` fills it from the record it already reads; no state,
  finding or verdict depends on it.

  `internal/approvaledit`'s tests pass unedited, and its only non-test
  callers (`check`, `serve`) still call `Compute` with the same arguments.
- **Nothing is written and nothing is recomputed.** `buildBriefsView` reads
  the `*briefs.Evaluation` the render already held for the payload; the
  payload's shape is unchanged.
- **A project with no brief renders the same page outside the stylesheet and
  scripts.** The new template (`briefEdited`) is a `{{define}}` joined to its
  neighbour with no text between them, so it adds no byte to the page. Measured against `41f603be`'s committed viewers
  with `<style>`/`<script>` bodies and timestamps masked, the four no-brief
  fixtures (basic, conformance-v1, portability, theme-flat) are
  byte-identical; graph-demo, with its one draft brief, differs in one line:
  the brief section's two new attributes, `data-brief-id` and
  `data-lock-state="draft"`.
- **Author text reaches the page as text.** Every view is
  `markdown.RenderDocument` output (the escaping boundary every brief body
  crosses); the word marks go in through `approvaledit`'s checked sentinel
  substitution, which falls back to an unmarked passage rather than emit
  unverified markup; each redline block's `aria-label` is
  `template.HTMLEscapeString`ed (`TestRender_BriefEditedEscapesAuthorMarkup`).

## Complexity and output size

Per render, for each edited brief only (every other brief is unchanged from
NIT-197): one `approvaledit.Passages` over the approved and current bodies —
`textdiff.Blocks`' LCS over passages, bounded by `textdiff.MaxBlocks` (400 a
side, else one whole-body replacement), and `MarkWords` over each replaced
pair, bounded by `textdiff.MaxMarkWords` (1,500 words a side, else
unmarked) — then one or two renders of each passage and one render of the
approved body. No path, pair of briefs, or claim term: the cost is per edited
brief and linear in their count.

Output: each edited brief's section carries its body up to three times (the
redline holds the approved and the current wording of each changed run, the
Approved view the approved body, the Current view the current body), so at
most about 4× the brief's rendered bytes, charged to the viewer's bound like
the rest of the page.

    go test ./internal/render -run '^$' -bench RenderBriefDiff -benchmem -benchtime=3x

| Case (one brief at the 2,000-word cap) | time | bytes allocated | Changes + Current HTML |
| --- | --- | --- | --- |
| 100 paragraphs, every other reworded (50 marked pairs) | 0.70 ms | 1.2 MB | 55 KB |
| one 2,000-word paragraph, one word changed (past the mark cap) | 0.17 ms | 0.56 MB | 30 KB |
| one 1,500-word paragraph, one word changed (at the mark cap) | 0.12 ms | 0.38 MB | 23 KB |
| every paragraph rewritten | 0.24 ms | 0.53 MB | 32 KB |

At the default cap of 60 briefs, all edited in the worst shape, that is
about 42 ms and 3.3 MB of sections per render.

## Verdict

PASS for this change: every applicable obligation above ran on the
candidate. Excluded, with reason: the readiness shape matrix (chains,
diamonds, cycles, dense DAGs), because no traversal, edge, cause, condition
or baseline changed and the page consumes the evaluation as data. This is
not release approval.
