# Working in this repository

## Core Principles
- **A skip is a failure**: Any check that cannot execute (missing browser/tool, unmatched selector) is FAILED. An exit status of 0 alone is never evidence; a skipped check is indistinguishable from passing zero assertions (`viewer-tests/harness_test.go:47`).
- **Never narrow coverage silently**: If a check cannot examine everything required, report the failure. If bounding coverage intentionally, explicitly state what was omitted where readers will see it; never sample, truncate, or drop cases silently.
- **Verify what the user sees, not what was edited**: Validate rendered output and published archive binaries as served/distributed. Source string edits do not verify output or runtime invariants.
- **State the harm, not an adjective**: Report specifically who does what and what goes wrong; evaluate on verifiable evidence rather than subjective severity words.
- **Judgement is an agent's; publishing is a maintainer's**: No model decides to release. Tagging and pushing belong exclusively to maintainers under [docs/RELEASING.md](docs/RELEASING.md).
- **Single release authority**: [docs/RELEASING.md](docs/RELEASING.md) is the sole release procedure; any second release procedure in the repository is a defect.

## Required Skills & Workflows
- **Graph Safety ([canonical graph-safety skill](.agents/skills/dossierx-graph-safety/SKILL.md))**:
  - Mandatory before planning, implementing, reviewing, or releasing changes to graph semantics, traversal, state transitions, or projection output (dependency readiness, claim locking, review propagation, build-order graphs, catalog, viewer data).
  - Prose-only spelling or formatting edits that leave behavior unchanged do not require a graph-safety proof.
  - Separate proof obligation: normal unit-test passes, prior release results, or proposed bounded algorithms do not prove non-degradation.
  - Maintainer-only overlay: do not add to consumer skill bundle under `skills/` or `skills/embed.go`.
  - If your agent does not discover skills automatically, open the canonical path explicitly relative to the repository root.
- **Audit Loops ([canonical audit-loop skill](.agents/skills/audit-loop/SKILL.md))**:
  - Mandatory when designing or running an iterative auditor-and-fixer workflow.
  - Use only for actual audit loops; does not replace ordinary review or the separate graph-safety gate.
- **Lesson Capture ([learning-to-skill](.agents/skills/learning-to-skill/SKILL.md))**:
  - Mandatory when asked to capture lessons, turn experience into skills, or refine reusable instructions.
  - Consult [the maintainer skills guide](docs/MAINTAINER_SKILLS.md) for canonical locations, portability rules, and project-specific bindings.
- **Skill Gaps ([skill-gap-review](.agents/skills/skill-gap-review/SKILL.md))**:
  - Use when asked which skills are missing or what a collection of past work should teach.
  - Identify and prioritize evidence-backed improvements before creating more skills.
