# Working in this repository

## The release gate is never skipped

This is the core principle of the release pipeline, and it holds for every session, every
release, and every agent working in this repository.

The release gate is the only thing standing between a change and a published release that is
wrong about itself. It is not advisory, not a formality, and not something to work around when a
release is urgent — an urgent release is exactly the one that has not been read carefully.

What follows from it:

- **A skip is a failure, not a pass.** Any check that cannot execute — a missing browser, an
  unresolvable baseline, an exhausted budget, a tool that is not installed — is reported to the
  human in plain terms and the gate verdict is FAILED. There is no result that means "we did not
  check" and reads as "it is fine." A skipped check "is indistinguishable from a pass over zero
  assertions" (`viewer-tests/harness_test.go:47`), and the gate treats it as the failure it is.
- **Never narrow coverage silently.** If the gate cannot examine everything it is supposed to
  examine, it says so and fails. It does not sample, truncate, or quietly drop a surface to stay
  inside a budget.
- **The gate surfaces; it never fixes.** A gate run produces findings and a verdict against the
  tree it was pointed at. Fixes are made by other agents in the workflow, which produces a new
  tree, and the gate re-runs against that. It re-runs until it is green — a finding is never
  waved through.
- **Judgement is an agent's; action is not.** The verifying agents are read-only: they never
  edit, merge, tag, or push. The irreversible half of a release is carried out by a deterministic
  driver whose precondition is a green gate whose recorded tree still matches what is about to be
  released, and which runs only under a human's explicit authorization for that release. No model
  ever decides to publish. That precondition is about content, so it is joined by two questions
  content cannot answer: the release being tagged must be the release the tree itself declares
  (`CHANGELOG.md`'s newest heading and the site's newest `releases[]` entry), and the CI-run
  evidence record for that tree must exist — a release nobody ran `make ci-evidence` for is
  refused, not assumed.
- **Every finding reaches the human, and priority is computed, not asserted.** Nothing is
  filtered, deduplicated away or dropped on its way to the report. Every finding carries a closed
  `consequence` — `acts-wrongly`, `misled`, or `cosmetic` — and a mandatory one-sentence
  `failure_scenario` describing what actually goes wrong for whoever reaches the surface; free-text
  severity was retired for this, deliberately, in the v0.5.2 priority design, because that word was
  the reporting agent's own about its own work, nothing derived it from the finding's evidence, and
  a single gate run had produced nine dialects of it that nothing downstream could compare. Priority
  P0–P3 is computed from that consequence crossed with the surface's `reach_class` in
  `surfaces.yaml` (`client-shipped`, `consumer-docs`, `maintainer`, `process`) — a reviewed matrix,
  not a choice either the agent or the gate makes case by case. Priority is stamped onto a finding
  at the moment it is recorded; editing a surface's `reach_class` afterward re-ranks nothing already
  on the receipt, only what gets recorded from then on. A receipt carrying any P0 finding,
  or any P1 finding the human has not ruled on, evaluates to FAILED; P2 and P3 findings stay on the
  receipt, never block, and are written into THIS release's own `gate/deferred.json` ledger by the
  PROJECTOR, run once a round's answers are complete (and re-written the same way by the driver's D1
  at publish time) — no fan-out writes it; a fan-out only prints a notice, and prints it on every fan-out for
  as long as a ledger on disk names a different release. The next release's round one reads the ledger before that
  release's own first projector run overwrites it. A P1 finding the
  human has judged non-blocking is cleared by fixing the tree or by
  recording a ruling in `gate/overrides.json` — tracked, naming the release, the finding's digest,
  who ruled and why, refused when it is stale or inherited or unreasoned, and carried on the receipt
  beside the finding it clears, which stays. The matrix is a default, never a ceiling: the human's
  ruling can promote any finding (recorded as `promote_to` on the override entry), P0 included, when
  the matrix undersells it — but P0 itself admits
  no ruling the other way, because a client-shipped defect that leaves a follower acting wrongly is
  exactly what this gate exists to catch, and it is not waved through by a signature. Deleting a
  finding by hand is still possible and is still the thing not to do: it leaves an adjudicated
  finding indistinguishable from one nobody raised. The override record was built in v0.5.2. Why the
  evidence-derived classifier — one that would derive a finding's weight from a file:line and the
  contradicting prose span, rather than from a manifest field and a closed vocabulary — was NOT
  built, and what it would need first, is recorded in `cmd/dossierx/gate_stage3_test.go`'s residues
  note.
- **Verify the thing the user sees, not the thing you edited.** The site is read as rendered DOM
  from a real build, the binary is checked from the published archive, and the tag's tree is checked
  against the tree that was actually approved. This covers output; it does not cover invariants about
  how the source produces that output, which need their own checks — a hand-typed version string
  renders identically to a derived one on the day it is written.

The procedure lives in `docs/RELEASING.md`. That file is the only description of how this project
releases, and there is exactly one of them: if you find a second release procedure anywhere in this
repository, that is a defect to report, not a fallback to use.
