---
name: dossierx-release-safety
description: Prepare, tag, publish, repair, or verify a DossierX release without treating stale branch checks, partial local suites, or a published tag as proof that the release is good. Use for release readiness, patch releases, post-tag failures, and claims that a release is live; do not use for ordinary feature implementation that is not entering release work.
---

# DossierX release safety

Read and follow [AGENTS.md](../../../AGENTS.md), then use
[docs/RELEASING.md](../../../docs/RELEASING.md) as the only release procedure.
This skill owns decision rules and stop conditions, not a second copy of the
commands.

## Bind evidence to the intended tag

Name the candidate version, immediate previous stable release, release branch
head, final merge commit, and intended tag commit. Evidence belongs to the exact
revision and environment that produced it. Merging, resolving conflicts, moving
the previous-release baseline, changing generated inputs, or changing a tool pin
invalidates the affected evidence.

Before a tag exists, complete every pre-tag item in the release procedure on the
final merge commit. In particular, do not substitute the ordinary root tests for
CI's race suite, an unpinned local linter for CI's pinned linter, or a report
against an older release for the current across-release comparison. A skipped or
unavailable check is not green.

Treat generated timestamps as review noise, not release content. Regenerate all
required fixtures, inspect them, restore stamp-only diffs, and commit only
semantic changes that the release intends to ship.

## Stop before creating another patch

A published tag is irreversible. If a post-tag check fails, state what users can
observe, preserve the failed run, and stop publication work. Diagnose the whole
failure class against the exact CI surface before preparing another version. Do
not fix only the first reported assertion and immediately tag again.

The repair candidate must pass the original failing command and its neighboring
cases on the final merge commit. For timing failures, audit every wall-clock
assertion reachable under the same instrumentation and platforms; distinguish
correctness limits from performance-only thresholds rather than increasing one
number until a runner passes.

## Verify publication as a separate phase

After the single authorized tag push, verify that exactly one publisher owns the
tag, the Release workflow succeeded, every expected archive and checksum exists,
the downloaded binary carries the intended stamps, the module proxy resolves the
tag, main points at the tagged merge, CI and CodeQL pass there, and the deployed
pages show the intended version. A tag, a release page, a successful build, and a
live site are separate facts.

Do not declare the release live while any required fact is pending, failed,
skipped, or verified only from source. Record exact revisions, run URLs, artifact
checks, and remaining gaps in the release evidence.

For why these stop conditions exist, read
[`docs/lessons/2026-09-11-post-tag-release-gaps.md`](../../../docs/lessons/2026-09-11-post-tag-release-gaps.md).
