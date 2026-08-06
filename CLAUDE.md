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
  ever decides to publish.
- **Every finding reaches the human.** Findings are classified by the evidence they carry, but
  none is suppressed on its way to the report. The human confirms what blocks a release; an agent
  never makes that call alone, and an override is recorded with its rationale.
- **Verify the thing the user sees, not the thing you edited.** The site is read as rendered DOM
  from a real build, the binary is checked by installing it, and the tag's tree is checked against
  the tree that was actually approved.

The full design lives in `docs/RELEASING.md` and, while the redesign is in flight, in
`scratchpad/pre-merge-gate-scope.md` on the release-pipeline branch.
