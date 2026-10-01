# Graph-safety evidence: a brief whose rests_on claim moved (NIT-200, v0.7.22)

Scope: viewer data. The brief page (`render.BriefsView`) now draws
`briefs.Review.ReviewPending` and `ChangedClaims` that NIT-205 already
computes and that the `dossierx-briefs` payload already carries: one amber
banner per changed claim (`internal/render/brief_review.go`), the REVIEW
PENDING pill (`BriefLockPillHTML`), the amber sidebar mark (`briefMark`,
the one mark NIT-201 introduced and this ticket owns), "changed <date>" on
that claim's Rests on / Made of row, and `BriefRow.ReviewPending` on a claim
card's BRIEFS group. "Confirm in a thread" is the page-foot Comment button,
so NIT-198's delegated handler opens the rail; this ticket adds no click
handler. The Threads block's copy for an edited or pending brief is
state-specific (`system-record.js` `briefThreadState`). Each claim redline
and each edited view still goes through `resolveBriefLinks`.

The page is a consumer, never a producer. Whether a brief is review-pending,
which claims moved, their wording at the baseline and now, and when the
current wording was approved, are `briefs.Evaluate`'s answers. Nothing here
computes a lock state, a review verdict or a baseline, and nothing writes.

- Candidate: the head of `cursor/nit-200-brief-review-pending-84ad`, cut from
  `3967ab5f` (the NIT-178 combo branch after NIT-199). The figures below were
  measured on the working tree of that branch before this note was committed,
  with the commands shown; the comparison base is `3967ab5f` throughout.
- Environment: the Cloud Agent VM (linux, go as installed).

## What the page reads, and the judgements it makes

| Element | Source | Rule |
| --- | --- | --- |
| Pending? | `Review.ReviewPending` | Evaluate's answer; a draft is never pending |
| Banners | `Review.ChangedClaims` | one per moved or gone baseline, in Evaluate's id order |
| Redline | `ChangedClaim.Baseline` / `.Current` | `approvaledit.Passages` + `markdown.Render`, wrapped as `BriefDiff.ChangesHTML` |
| Unavailable | `Baseline == nil` | `briefs.UnavailableWording`, and a link to the claim |
| Rests on note | `ChangedClaim.ChangedAt` | "changed 18 Sep" from the standing claim record's time, or "changed" |
| Pill | `reviewPillState` | `review` when locked and pending and not edited; edited still wins |
| Sidebar mark | `briefMark(Review)` | edited, then review, then thread, then draft/locked — unchanged order |
| Claim-card row | `BriefRow.ReviewPending` | `briefReviewOf(evaluation, brief).ReviewPending` |

1. **The page never invents a wording.** A missing baseline is named, not
   diffed against an empty string (which would mark every passage as added).
2. **Edited still outranks review** on the mark and the pill. A brief that is
   both draws B3's banners and B2's views, with the agent-update caption.
3. **Claim locking is not blocked**, and the banner says so. Nothing here
   enters a claim's readiness, lock or review.

## Preserved invariants

- **A claim never learns about briefs; no claim-side package changed:**

      git diff --stat 3967ab5f -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/textdiff internal/briefs \
        internal/approvaledit

  `internal/briefs` and `internal/approvaledit` are unread by this ticket
  except as data and as the passage helper NIT-199 already exported.
- **Nothing is written and nothing is recomputed.** `buildBriefsView` and
  `buildBriefRelationsLookup` read the `*briefs.Evaluation` the render
  already held for the payload; the payload's shape is unchanged.
- **A project with no brief renders the same page outside the stylesheet and
  scripts.** The new template (`briefReview`) is a `{{define}}` joined to its
  neighbour with no text between them. Measured against `3967ab5f`'s
  committed viewers with `<style>`/`<script>` bodies and timestamps masked,
  the four no-brief fixtures must stay byte-identical.
- **Author text reaches the page as text.** Claim wording crosses
  `markdown.Render` (the claim escaping boundary); each banner's claim id is
  html/template-escaped; the Rests on note is `html.EscapeString`ed
  (`TestRender_BriefReviewPendingEscapesAuthorMarkup`).

## Complexity and output size

Per render, for each review-pending brief only: one `approvaledit.Passages`
per changed claim — `textdiff.Blocks`' LCS over passages, bounded by
`textdiff.MaxBlocks`, and `MarkWords` over each replaced pair, bounded by
`textdiff.MaxMarkWords` — then one or two claim-mode renders of each
passage. No path and no pair of briefs: the cost is per changed claim on
pending briefs. `resolveBriefLinks` is one pass over each new view's bytes.

A brief that is not pending costs what it cost on the base (one review
lookup the mark already made, plus a boolean on the claim-card row).

    go test ./internal/render -run '^$' -bench 'RenderBriefDiff|BriefsView' -benchmem -benchtime=3x

The claim redline is the same bounded LCS NIT-199 already measured. At the
default cap of 60 briefs, each pending on one claim at the claim-body cap,
the extra work is 60 bounded diffs of claim prose, not brief prose.

## Verdict

PASS for this change: every applicable obligation above ran on the
candidate. Excluded, with reason: the readiness shape matrix (chains,
diamonds, cycles, dense DAGs), because no traversal, edge, cause, condition
or baseline changed and the page consumes the evaluation as data. This is
not release approval.
