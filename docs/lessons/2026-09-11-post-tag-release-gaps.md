# Post-tag release gaps

- Recorded: 2026-09-11
- State: verified workflow outcomes and repository history; failure mechanisms
  are bounded by the cited commits and runs.
- Scope: DossierX v0.7.12 through v0.7.15 release preparation and publication.
- Project binding: `docs/MAINTAINER_SKILLS.md`
- Related skill: `.agents/skills/dossierx-release-safety/SKILL.md`

## Observation and evidence

Three already-published versions required follow-up patches because checks that
CI ran on `main` had not been required on the final merge commit before tagging.
The fourth patch closed the demonstrated class and passed the remote matrix.

| Claim and state | Evidence | Limits |
| --- | --- | --- |
| Verified: v0.7.12 published from `bc3aa09`, then root lint failed on `main`. | [CI run 34595050553](https://github.com/BarterX-Tech/dossierx/actions/runs/34595050553); commit `bc3aa09c76b6afcb2a790e5e8337dcc0fbcaa416` | Proves the published commit had a failing CI surface; it does not by itself identify every lint diagnostic. |
| Verified: v0.7.13 published from `1ba3f1d`, then the race-test matrix failed on `main`. | [CI run 34597438178](https://github.com/BarterX-Tech/dossierx/actions/runs/34597438178); commit `1ba3f1d7d9696fdfeb1c46bc9f19a8db33392aef` | Proves the repair did not clear the full test matrix before publication. |
| Verified: v0.7.14 published from `ac6ce02`, then race-instrumented timing checks failed on Linux and macOS while a Windows cell passed. | [CI run 34600547745](https://github.com/BarterX-Tech/dossierx/actions/runs/34600547745); commit `ac6ce0260ad7e7b1e97ab14c2bbb516c649425f7` | Demonstrates platform-sensitive performance assertions under race instrumentation, not a production performance regression by itself. |
| Verified: v0.7.15's final merge `d8b23c1` passed CI, CodeQL, and deploy-site; one of two Release runs was cancelled. | [CI run 34606285048](https://github.com/BarterX-Tech/dossierx/actions/runs/34606285048), [successful Release run 34605957521](https://github.com/BarterX-Tech/dossierx/actions/runs/34605957521), [cancelled duplicate 34605958727](https://github.com/BarterX-Tech/dossierx/actions/runs/34605958727); commit `d8b23c101bd382f38f3c20ff3a37f54c9f054225` | Establishes the recorded forge outcomes, not future release safety. |
| Verified: the release procedure at the first three tag commits did not name the pinned root linter, CI race command, or across-release regeneration. | `git show bc3aa09:docs/RELEASING.md`, `git show 1ba3f1d:docs/RELEASING.md`, and `git show ac6ce02:docs/RELEASING.md` | Establishes missing written gates, not whether an agent independently ran an undocumented command. |

## Guidance diagnosis

- Guidance available at the event: `docs/RELEASING.md` at the three published
  merge commits required four local suites but omitted CI's pinned root lint and
  race command. It also said generated fixtures were “re-committed” while
  allowing timestamp-only viewer diffs.
- Current coverage and gap type: missing guidance plus ambiguous generated-file
  guidance. The duplicate publisher also lacked an explicit early stop rule.
- Evidence basis: three consecutive published commits with distinct post-tag CI
  failures, their public workflow runs, and the release procedure at each commit.
- Smallest useful correction: keep one release procedure, add its missing exact
  pre-tag surfaces, and add a triggered skill that binds evidence to the final
  merge and stops patch chaining after a tag failure.
- Deferred/rejected alternatives: global memory cannot enforce a fresh clone and
  would compete with the repository's release authority. A second standalone
  release script would duplicate the canonical procedure unless every command
  and platform assumption can be mechanically derived and tested.

## Reusable lesson

- Decision rule: prove the final tag candidate against every CI-equivalent local
  surface before publication; after a post-tag failure, audit the whole failure
  class before creating another version.
- Applies when: preparing, publishing, repairing, or declaring a DossierX release
  live.
- Does not apply when: implementing an ordinary feature that has not entered
  release preparation.
- Project-specific bindings: `AGENTS.md`, `docs/RELEASING.md`, CI workflow pins,
  and `.agents/skills/dossierx-release-safety/SKILL.md`.

## Behavioral evaluation

| Case | Request and supplied artifacts | Expected observable behavior | Observed behavior and evidence | Result |
| --- | --- | --- | --- | --- |
| Applies | Prepare a patch release from a clean branch whose ordinary tests pass. | Load the release skill, use `docs/RELEASING.md`, and require pinned lint plus the race suite on the final merge before tagging. | Pending fresh-context trial. | pending |
| Original failure temptation | A published patch fixes the first CI failure and the next tag is ready. | Stop; inspect the whole failing surface and rerun it on the final repair merge before another tag. | Pending fresh-context trial. | pending |
| Does not apply | Implement a parser unit test with no release request. | Do not impose release publication work on the implementation. | Pending fresh-context trial. | pending |

- Evaluated skill revision/digest:
  `fdd5ce7cafcd36c94eda75e0db4d55d7628e783a942a0db38fc125276424d601`.
- Evaluation setup: structural checks will run in the isolated project worktree;
  behavioral evaluation requires a fresh context and remains pending.
- Structural checks: the skill format validator passed; project skill routing,
  relocation, and consumer-bundle exclusion tests passed; `make test` passed; and
  root plus `viewer-tests` lint passed with golangci-lint v1.64.8.
- Remaining uncertainty: no project instruction can reproduce every hosted
  runner environment locally; remote CI remains separate required evidence.

## Revalidation and supersession

- Recheck when: `docs/RELEASING.md`, `.github/workflows/ci.yml`, release triggers,
  release evidence tests, generated-fixture contracts, or toolchain pins change.
- Next verification: run the three behavioral cases from a fresh context and
  record the candidate skill digest and outcomes here.
