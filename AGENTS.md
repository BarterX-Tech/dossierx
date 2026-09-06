# Working in this repository

Read and follow `CLAUDE.md`. Its repository rules apply to every agent, not only
Claude Code. `docs/RELEASING.md` is the only release procedure.

## Required project skill

Before planning, implementing, reviewing, or releasing a change to dependency
readiness, claim locking, review propagation, build-order graphs, catalog or
viewer graph projections, or any other graph traversal, read and follow:

`.agents/skills/dossierx-graph-safety/SKILL.md`

The graph-safety result is a separate proof obligation. Do not treat a normal
unit-test pass, a prior release result, or a proposed bounded algorithm as proof
that a graph change is non-degrading.

## Required audit-loop skill

Before designing or running an iterative auditor-and-fixer workflow, read and
follow the [canonical audit-loop skill](.agents/skills/audit-loop/SKILL.md).

Use it only for an actual audit loop. It does not replace ordinary review or the
separate graph-safety gate.
