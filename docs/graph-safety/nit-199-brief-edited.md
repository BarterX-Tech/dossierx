# Graph-safety evidence: a brief edited since its approval (NIT-199, v0.7.22)

Scope: viewer data. The brief page (`render.BriefsView`, NIT-197) now reads
each brief's lock and review state (`briefs.Review`, NIT-205), which the
render already carried for the `dossierx-briefs` payload, and draws what it
says: the header pill
for an edited and an unrecorded brief (`components.BriefLockPillHTML`), the
approval's date and reason in the meta line (`briefApproval`), and, for a
locked brief whose file moved since its approval, the banner and the Changes /
Approved / Current views (`render.RenderBriefDiff`, `briefEditView`,
`internal/render/brief_edit.go`). The views are rendered into the page's own
section, like the body, so a live reload's fragment swap carries them, and
each view goes through NIT-201's `resolveBriefLinks`, as the body does, so a
link to another brief opens its page. A feature brief (NIT-201's feature
page) gets the same banner and views. The sidebar mark is NIT-201's one
`briefMark` (`internal/render/brief_page.go`), which this page reads and no
longer duplicates. "Approve or restore in a thread" is the page-foot Comment
button, so NIT-198's delegated handler opens the rail; this ticket adds no
click handler for it. The client code only picks the shown view (`viewer-runtime.js` `syncBriefEdits`)
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
  onto `ccbbb72c` (the NIT-178 combo branch after NIT-196, NIT-204, NIT-197,
  NIT-202, NIT-205 and its #137 follow-up, then NIT-198 (#138), NIT-193
  (#139), NIT-190 (#140) and NIT-201 (#141)). The figures below were
  re-measured on the working tree of that branch, rebased, before this note
  was committed, with the commands shown; the comparison base is `ccbbb72c`
  throughout.
- Environment: go1.26.5 darwin/arm64 (Apple M4 Pro).

## What the page reads, and the judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Pill | `Review.LockState`, else the frontmatter `status` | `edited` → EDITED SINCE APPROVAL; `unrecorded` → LOCK NOT RECORDED; otherwise NIT-197's Locked / Draft |
| Sidebar mark | NIT-201's `briefMark(Review)` | unchanged by this ticket; its "Not locked: no approval on record" is also the unrecorded meta line's words |
| Meta line | `Review.LockedAt`, `LockReason` | only for `locked` and `edited`, the states with a standing approval, and not on a feature page, whose meta line stays NIT-201's count of what it rests on |
| Links in the views | `resolveBriefLinks` over each view's HTML | the body's own rewrite: a link naming a brief becomes `#<anchor>`, any other is left as written |
| Banner and views | `Review.LockState == edited`, `Review.Approved` | the diff of `Approved.Markdown` against the brief's body; no approved markdown (a record `brief lock` did not write) shows the recover note |
| Images that moved | `Review.ApprovedImages` against `Brief.ImageDigests()` | changed, added, removed by digest, the comparison `ContentMoved` makes for the finding |
| Whitespace only | `briefs.LockHash` of the approved text against `Brief.LockHash` | the text's hash moved while no passage, summary or `rests_on` id reads differently |

1. **An unrecorded brief is not drawn as locked.** Its file says `locked`,
   but `brief-unrecorded` means no approval stands; the pill says LOCK NOT
   RECORDED and the mark is the draft mark. This is the honest reading of the
   state NIT-205 defines, and it changes only what the page draws.
2. **Trailing blank lines do not make a passage differ.** `textdiff` keeps a
   passage's trailing blank lines on it, so appending a section to a brief
   would redline the unchanged last passage above it. The brief diff ends both
   bodies with exactly one blank line before diffing (`endPassage`); a
   markdown body renders the same whatever trails its last line. The claim
   diff is unchanged.
   Because the lock still signs those bytes, an edit that changes nothing
   else is named for what it is ("only whitespace changed"), decided from
   the hash, never guessed as an image.
3. **A passage and its replacement count once.** A run of removed passages
   and the added run that replaces it count as the longer of the two.
   `approvaledit.Passages`' own count, which the claim viewer shows, pairs a
   removal only with the addition directly after it and so reads nine for
   five paragraphs reworded in a row; it is left as it is here.

## Preserved invariants

- **A claim never learns about briefs; no claim-side package changed** but
  the `approvaledit` refactor:

      git diff --stat ccbbb72c -- internal/lock internal/readiness \
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
  neighbour with no text between them, so it adds no byte to the page. Measured against `ccbbb72c`'s committed viewers
  with `<style>`/`<script>` bodies and timestamps masked, the four no-brief
  fixtures (basic, conformance-v1, portability, theme-flat) are
  byte-identical; graph-demo, with one draft brief and two features (one
  locked, one draft), differs in three lines: each brief section's one new
  attribute, `data-lock-state`.
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
brief and linear in their count. `resolveBriefLinks` is one pass over each
view's bytes (O(view bytes), NIT-201's bound), three passes per edited brief.

Output: each edited brief's section carries its body up to three times (the
redline holds the approved and the current wording of each changed run, the
Approved view the approved body, the Current view the current body), so at
most about 4× the brief's rendered bytes, charged to the viewer's bound like
the rest of the page.

    go test ./internal/render -run '^$' -bench RenderBriefDiff -benchmem -benchtime=3x

| Case (one brief at the 2,000-word cap) | time | bytes allocated | Changes + Current HTML |
| --- | --- | --- | --- |
| 100 paragraphs, every other reworded (50 marked pairs) | 0.63 ms | 1.2 MB | 55 KB |
| one 2,000-word paragraph, one word changed (past the mark cap) | 0.14 ms | 0.56 MB | 30 KB |
| one 1,500-word paragraph, one word changed (at the mark cap) | 0.11 ms | 0.38 MB | 23 KB |
| every paragraph rewritten | 0.21 ms | 0.53 MB | 32 KB |

At the default cap of 60 briefs, all edited in the worst shape, that is
about 38 ms and 3.3 MB of sections per render.

Briefs that are not edited cost what they cost on the base. NIT-197's and
NIT-201's own benchmarks (no edited brief), run the same way on this branch
and on an export of `ccbbb72c`:

    go test ./internal/render -run '^$' -bench 'BriefsView|FeaturesView' -benchmem -benchtime=3x

| Case | `ccbbb72c` | this branch | page bytes (both) |
| --- | --- | --- | --- |
| 60 briefs, 1,000 claims | 6.6 ms, 6.04 MB, 17,069 allocs | 6.7 ms, 6.11 MB, 17,319 allocs | 1,106,080 |
| 2,000 briefs, 10,000 claims | 153 ms, 137 MB, 251,694 allocs | 153 ms, 138 MB, 259,701 allocs | 25,086,000 |
| 12 features, 200 rests_on each | 4.0 ms, 12.4 MB, 71,492 allocs | 3.8 ms, 12.4 MB, 71,524 allocs | 1,192,812 |
| 2,000 features, 50 rests_on each | 327 ms, 684 MB, 6,047,172 allocs | 324 ms, 685 MB, 6,055,089 allocs | 66,068,000 |

The extra allocations are about four per brief (8,007 over 2,000 briefs:
the pill, the approval and the edit check read the review the mark already
read); the benchmark's page-bytes metric (bodies and lists) is identical.

## Verdict

PASS for this change: every applicable obligation above ran on the
candidate. Excluded, with reason: the readiness shape matrix (chains,
diamonds, cycles, dense DAGs), because no traversal, edge, cause, condition
or baseline changed and the page consumes the evaluation as data. This is
not release approval.
