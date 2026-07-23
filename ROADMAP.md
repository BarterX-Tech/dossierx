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
