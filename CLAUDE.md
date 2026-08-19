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
- **Every finding reaches the human, and a reader who acts wrongly always stops the release.**
  Nothing is filtered, deduplicated away or dropped on its way to the report. What a finding does
  NOT carry is a severity adjective: an adjective is an opinion nobody can check, and the one thing
  a non-overridable rule must leave room for is being shown to be wrong. Instead every finding
  states a `consequence` from a closed set, a `failure_scenario` describing the concrete harm — who
  does what, and what goes wrong for them — and the reporting agent's own `blocking` judgement.
  A scenario that is empty, a single word, or made only of severity words is refused when the
  answer is recorded, not puzzled over later.
  The verdict is a pure function of those recorded fields, and it is two rules. A finding whose
  consequence is `acts-wrongly` — a reader following the document DOES the wrong thing — blocks
  unconditionally, at every reach class, with no override, no deferral and no signature: the only
  ways past it are to fix the tree or to demonstrate that there was never a defect. Every other
  finding blocks exactly when the agent that raised it judged it `blocks`; the agents are trusted to
  weigh their own findings, and no table anywhere re-grades them. A finding judged `deferrable`
  stops nothing and is not dropped for it — it rides the record to the human whole, so the honest
  move is always to report it and judge it, never to omit it.
  One consequence to know before you meet it: there is no override field on the receipt, so a
  blocking finding that should never have been raised is cleared only by fixing the tree or by
  deleting the finding by hand, and deleting it leaves an adjudicated finding indistinguishable from
  one nobody raised. That is the cost the `failure_scenario` exists to keep small: a stated harm can
  be refuted on the evidence, and an adjective cannot. Why the evidence-derived classifier was not
  built, and what it would need first, is recorded at `cmd/dossierx/gate_stage3_test.go:42-57`.
- **Verify the thing the user sees, not the thing you edited.** The site is read as rendered DOM
  from a real build, the binary is checked from the published archive, and the tag's tree is checked
  against the tree that was actually approved. This covers output; it does not cover invariants about
  how the source produces that output, which need their own checks — a hand-typed version string
  renders identically to a derived one on the day it is written.

The procedure lives in `docs/RELEASING.md`. That file is the only description of how this project
releases, and there is exactly one of them: if you find a second release procedure anywhere in this
repository, that is a defect to report, not a fallback to use.
