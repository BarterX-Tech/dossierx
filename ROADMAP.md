# Roadmap

This file tracks known-unverified or in-progress items — things that are implemented and
tested but not yet proven against a real, external, end-to-end use case. It is distinct from
[CHANGELOG.md](CHANGELOG.md), which tracks what has actually shipped.

## Unverified: Code Links end-to-end

The `dossierx-claim: <id>` source-scanning mechanism (the `source_dirs` config field, drift
detection via `dossierx flag` / `dossierx reaudit`, and the viewer's "implemented in" line) is covered
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

## Deferred: an on-page cycle signal in `dossierx serve`

`dossierx check` and the lint suite already refuse a cycle at lock time, but `dossierx serve` —
the human's live-reload viewer — has no on-page indicator that the currently-rendered claim set
contains an unresolved cycle somewhere in the graph. A human working entirely inside `serve`
between checks has no visual signal short of running `dossierx check` themselves. This is a
formal deferral, not a scheduled item: no release currently commits to it.
