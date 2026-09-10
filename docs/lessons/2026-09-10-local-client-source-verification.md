# Local source must be explicit in client verification

- Recorded: 2026-09-10
- State: verified mechanism; fresh-context skill selection pending
- Scope: DossierX Go 1.26 tool clients tested against a local DossierX checkout before release
- Project binding: `docs/MAINTAINER_SKILLS.md`
- Related skill: `.agents/skills/dossierx-local-client/SKILL.md`

## Observation and evidence

A temporary Go workspace can replace the client's released DossierX module with
an exact local checkout while preserving the client's dependency files. The
same route can run both the read-only check and the live viewer.

| Claim and state | Evidence | Limits |
| --- | --- | --- |
| Verified: the local runner resolved the named checkout and ran the real Curtainly documentation corpus with 0 lint errors and 0 ledger findings. | From the candidate tree based on `4fe85dc68c4c34bf67969df8fc21f8d14f172eea`, set `CLIENT` to the Curtainly documentation module and run `scripts/run-local-client.sh "$CLIENT" check --validate --format json`. | The corpus had 11 existing warnings. The candidate was uncommitted when observed, so re-run after commit. |
| Verified: the same runner started the real client viewer and its HTTP root responded successfully. | Run `scripts/run-local-client.sh "$CLIENT" serve`, then request the printed loopback URL. | This established server reachability, not visual correctness for an arbitrary feature. |
| Verified: the client dependency files and working-tree status were unchanged after the read-only run. | Compare hashes of the client `go.mod` and `go.sum`, Git status, and absence of `go.work` before and after the command. | A write-mode DossierX command can still change its normal generated or ledger outputs. |

## Guidance diagnosis

- Guidance available at the event: no skill covered a client agent receiving a local DossierX source path.
- Current coverage and gap type: missing guidance plus discovery failure.
- Evidence basis: one reproducible successful mechanism and the user's explicit request for path-only agent delegation.
- Smallest useful correction: one project-owned skill, one deterministic runner, and one thin machine-level discovery adapter.
- Deferred/rejected alternatives: a persistent client `go.work` hides which source ordinary Go commands use; `go install` does not control `go tool dossierx`; a copied global procedure would drift from the project skill.

## Reusable lesson

- Decision rule: route a supplied local source path through a temporary workspace and report the resolved source identity.
- Applies when: a client with a DossierX Go tool declaration must exercise unreleased source on the same machine.
- Does not apply when: testing a published release artifact, publishing a release, or changing the client's long-term dependency pin.
- Project-specific bindings: `.agents/skills/dossierx-local-client/SKILL.md`, its runner, and `CONTRIBUTING.md`.

## Behavioral evaluation

| Case | Request and supplied artifacts | Expected observable behavior | Observed behavior and evidence | Result |
| --- | --- | --- | --- | --- |
| Applies | “Use DossierX at `<worktree>` to verify this client.” | Resolve both paths, run the temporary override, and report source identity plus client results. | Fresh-context agent trial not run. | pending |
| Original failure temptation | The client is pinned to an older release and a newer global binary is installed. | Do not edit the pin or use the global binary; use the named source through the runner. | Fresh-context agent trial not run. | pending |
| Does not apply | “Verify the published DossierX release archive.” | Do not invoke the local-client skill; follow release verification instead. | Fresh-context agent trial not run. | pending |

- Evaluated skill revision/digest: SHA-256 `deb18ae59aa441a22f8a7832cf9f16dc0be1e104887d1ca18c27443b481e50e9`
- Evaluation setup: structural and real-run checks in the authoring session; no independent agent was requested.
- Structural checks: both canonical skill and discovery adapter passed `quick_validate.py`; `go test ./tests -run '^TestProject.*Skill' -count=1 -v` passed; both project and explicit-source runner forms executed the real client tool.
- Remaining uncertainty: automatic selection by a fresh client coding agent has not yet been observed.

## Revalidation and supersession

- Recheck when: the client stops using Go tool directives, the helper interface changes, or a host changes skill discovery behavior.
- Next verification: start a fresh client task with only the DossierX source path and confirm that the discovery adapter loads the canonical skill and preserves the client pin.
