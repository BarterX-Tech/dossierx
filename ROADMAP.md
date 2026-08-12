# Roadmap

This file tracks known-unverified or in-progress items — things that are implemented and
tested but not yet proven against a real, external, end-to-end use case. It is distinct from
[CHANGELOG.md](CHANGELOG.md), which tracks what has actually shipped.

## Unverified: Code Links end-to-end

The `dossierx-claim: <id>` source-scanning mechanism (the `source_dirs` config field, drift
detection via `dossierx claim flag` / `dossierx claim reaudit`, and the viewer's "implemented in"
line) is covered
by synthetic fixtures but has not yet been exercised end-to-end against a real project that
completed a full claim-author → lock → implement → link cycle.

Update this entry once that cycle completes in a real consuming project.

## Deferred: Tarjan SCC cycle reporting

The `cycle`, `governed-cycle`, and related graph-invariant lints currently report a cycle by
walking an explicit depth-first search stack and naming the claims on it. A proper Tarjan
strongly-connected-components pass would let the engine report every disjoint cycle in one run
instead of the first one the walk happens to hit, and would give a tighter, more structural
diagnostic when a project's `rests_on` or `governed_by` graph has more than one broken loop at
once. This is a formal deferral, not a scheduled item: no release currently commits to it.

## Deferred: a cycle signal on the reading view itself

Most of this shipped in v0.5.0. The viewer carries a permanent "Claims graph" pane with a
`dependency cycles` overlay that rings cycle-member claims red and an `in a cycle` row in the
detail panel; the payload is rebuilt on every render — under `dossierx serve` that is every
reload — and `GET /api/graph` refreshes it live. A human working entirely inside `serve` between
checks now does have a visual signal short of running `dossierx check` themselves.

What is still deferred is narrower: that signal only exists once the reader opens the pane.
Nothing on the reading view itself indicates that the currently-rendered claim set contains an
unresolved cycle somewhere in the graph. This is a formal deferral, not a scheduled item: no
release currently commits to it.

## Deferred: markdown-sanity's silence on a MATCHED underscore run

The `markdown-sanity` lint reports an *unmatched* emphasis/strikethrough delimiter run — one
`*`, `_`, or `~` sequence with no partner on the same block — so an author sees a warning when a
stray delimiter was clearly a typo. It says nothing about a run that pairs perfectly and
therefore does render as emphasis. `__init__`, written as a whole word with underscores flanking
it on both sides, renders `<strong>init</strong>`; a stray `_` earlier in a paragraph that later
finds an unintended partner emphasises everything in between. Neither case is unbalanced, so
nothing fires, and an author who did not intend either result has no lint telling them so.

This is the residual exposure the whole "intraword underscores don't italicize" argument was won
on: the rule that makes ordinary identifier text (`governed_by`, `rests_on`) safe from accidental
emphasis is a flanking rule, not a rule against pairing at all, and a full-word token like
`__init__` is exactly the shape the flanking rule was designed to still let through. Deferred by
human decision for this release rather than by a missing capability — the workaround, for an
author who needs a literal double-underscore token to stay literal, is a backslash escape on one
delimiter (`\_\_init\_\_`) or a code span (`` `__init__` ``). Neither workaround is written down
anywhere a claim author will see it. This is a formal deferral, not a scheduled item: no release
currently commits to it.

## Deferred: `markdown.Diagnose` was never built, so the lint mirrors the parser

`internal/render/markdown` exports exactly `Render` and `RenderInline`, both returning finished
`template.HTML`. There is no diagnostics entry point — nothing in that package can tell a caller
"this fence never closed" or "this `src` was refused" — and the renderer's whole contract is that
malformed input degrades silently to literal text, which is the silence the `markdown-sanity` and
`asset-scope` lints exist to break. So those two lints share a second, independent scanner:
`internal/lint/markdown_scan.go`, 1,048 lines at release (1,132 before `internal/urlsafe`
absorbed the URL rules), every recognizer of which names the renderer rule it mirrors.

The consequence is stated in that file's own doc comment (lines 16–20) and is not hidden: **the
mirror can drift.** Any change to a recognition rule in `markdown.go` — the escapable set, the
fence opener's shape, the list indentation rule, the scheme allowlist — has to be made twice, and
nothing in the build fails if only one copy moves. The comment names the fix ("the right
long-term fix is a diagnostics entry point on the markdown package") and points at a release
change list rather than at anything a reader of this repository would find, which is why it is
written down here. One half of the mirror is already immune: the URL rules now call the shared
`internal/urlsafe` leaf that the renderer calls, so that half cannot drift by construction. The
rest can. This is a formal deferral, not a scheduled item: no release currently commits to it.

## Deferred: a short table row renders short, which contradicts spec amendment A9

The v0.3.1 plan's section 10 is authoritative, and amendment A9 says a body row whose cell count
differs from the header's "is padded with empty `<td>` or truncated — never emitted ragged".
Half of that shipped: a longer row does have its extra cells dropped. A **short** row does not
get padded — it emits exactly the cells it has, leaving a ragged right edge where GFM would
square the table off.

This was a deliberate reversal taken mid-release, not an oversight. Padding was the amplification
vector the phase-C bound existed to stop: a header of N columns and M one-cell rows emitted N×M
cells from roughly N+2M source bytes. Two attempts to tune a refusal threshold both failed the
same way — A9 also says a ragged table *remains a table*, so any refusal rule contradicts the
spec by construction and merely relocates a cliff real authors fall off (measured on the previous
tree, a four-column centre-aligned table with three ragged rows silently rendered as a
paragraph). Emitting only cells that exist in the source closes the vector at its origin instead
of capping it, and let the refusal path and the three-valued table verdict be deleted outright.
The trade accepted is the ragged right edge; column alignment is unaffected, because widths are
shared table-wide. FORMAT.md documents the shipped behaviour as the rule. Nothing here reopens
A9's padding clause: this is a formal deferral, not a scheduled item, and no release currently
commits to restoring it.

## Deferred: four remembered verbs get a bare "unknown command" instead of a stub

`cmd/dossierx/retired.go` gives every verb v0.3.0 folded into a noun a stub that names its
replacement — `dossierx lint` answers with the `check` recovery, and so on. Four are missing:
`lock`, `unlock`, `flag` and `reaudit`. Until v0.5.2 the file recorded a reason for that which was
false: it claimed the root already answers them with a hint listing the seven nouns. It does not.
Cobra's `legacyArgs` rejects an unknown command during `Execute`, so the root's `RunE` — the only
hint-bearing branch — is never reached, and the caller gets:

    dossierx lock some.claim.id
    {"command":"","error":{"code":"usage","message":"unknown command \"lock\" for \"dossierx\""}}

No hint, no replacement named, and an empty `command` field: exactly the answer `retired.go` exists
to remove, on the four verbs most likely to be typed from pre-v0.3.0 memory, since each of them
still exists at `dossierx claim <verb>`.

Not fixed in v0.5.2 because it is a CLI-surface change, not a comment fix: four new commands enter
`surface.json`, and every skill and document that states the noun and leaf counts would move with
them. That is a minor release's work, and this is a patch. The false justification has been replaced
with the measurement, so the next reader meets the gap rather than a reason not to look. This is a
formal deferral, not a scheduled item: no release currently commits to it.
