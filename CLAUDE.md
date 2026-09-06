# Working in this repository

## A check that did not run is a failure, not a pass

DossierX is alpha and its releases are cut by a maintainer who reads the change and runs the
suites. There is no gate pipeline any more. What survives it is the principle the pipeline was
built on, because that part was never about the pipeline:

- **A skip is a failure.** Any check that cannot execute, whether the browser is missing, the
  tool is not installed, or the selector matched nothing, is reported in plain terms and the
  answer is FAILED. There is no result that means "we did not check" and reads as "it is fine."
  A skipped check is indistinguishable from a pass over zero assertions
  (`viewer-tests/harness_test.go:47`), and it is treated as the failure it is. `go test` exits 0
  for a skip and exits 0 for a `-run` selector that matches nothing, so an exit status alone is
  never the evidence.
- **Never narrow coverage silently.** If something cannot examine everything it is supposed to
  examine, it says so and fails. It does not sample, truncate, or quietly drop a case to stay
  inside a budget. If you do bound coverage on purpose, say what was left out where the reader
  will see it.
- **Verify the thing the user sees, not the thing you edited.** The site is read as served, the
  binary is checked from the published archive. Confirming a string in a source file proves you
  made an edit and nothing else. This covers output; it does not cover invariants about how the
  source produces that output, which need their own checks, because a hand-typed version string
  renders identically to a derived one on the day it is written.
- **State the harm, not an adjective.** When you report a problem, say who does what and what
  goes wrong for them. A severity word is an opinion nobody can check; a stated harm can be
  refuted on the evidence, which is the property that matters when the report is wrong.
- **Judgement is an agent's; publishing is not.** No model decides to release. Tagging and
  pushing are a maintainer's, under `docs/RELEASING.md`.

The procedure lives in `docs/RELEASING.md`. That file is the only description of how this project
releases, and there is exactly one of them: if you find a second release procedure anywhere in
this repository, that is a defect to report, not a fallback to use.

## Required project skill

Before planning, implementing, reviewing, or releasing a change to dependency readiness, claim
locking, review propagation, build-order graphs, catalog or viewer graph projections, or any other
graph traversal, read and follow `.claude/skills/dossierx-graph-safety/SKILL.md`.

This is a maintainer-only overlay. It is not part of the consumer skill bundle under `skills/` and
must not be added to `skills/embed.go`. It defines graph proof obligations, not a second release
procedure; release execution remains exclusively in `docs/RELEASING.md`.
