# Portable maintainer skills

This repository keeps reusable instructions and their supporting lessons in Git.
A fresh session needs the repository, not a previous agent's private memory.

## Canonical files

- `AGENTS.md` owns shared repository rules and routes tasks to relevant skills.
- `.agents/skills/<name>/SKILL.md` owns each maintainer skill. Keep supporting
  references and portable scripts inside that skill directory.
- `docs/lessons/` records demonstrated failures, evidence, and the limits of each
  lesson. These records are context, not another instruction hierarchy.
- `CLAUDE.md` and files under `.claude/skills/` are thin discovery adapters. They
  point to canonical files and do not repeat their procedures.
- `skills/` remains the separate consumer bundle shipped by DossierX. Maintainer
  skills must not enter that bundle.

Use [the Agent Skills format](https://agentskills.io/specification): a skill
folder with YAML `name` and `description` followed by Markdown instructions.
Use [AGENTS.md](https://agents.md/) for shared repository guidance. The formats
provide portable content; automatic discovery still depends on the host agent.

## Use on another agent or machine

Clone the repository at the required revision. Open `AGENTS.md`, then the
canonical `SKILL.md` for the task. If the agent does not discover either file,
ask it explicitly: "Read AGENTS.md and use the relevant skill under
.agents/skills before doing this task." No slash command, plugin, global memory,
absolute home path, or symlink is required for this reading path.

Paths in repository rules are relative to the repository root. Links inside a
skill are relative to the containing Markdown file. Resolve executable paths
from the skill location and pass the repository root explicitly when needed;
do not assume the starting working directory. Declare required tools and
versions, input/output paths, resource limits, and side effects before adding
scripts. A missing prerequisite is missing evidence, never a successful check.

Optional host adapters may add discovery metadata. Keep decisions and procedures
in the canonical skill. Test any claimed native discovery on the actual host;
being able to read a Markdown file does not prove automatic invocation there.

## Turn a lesson into a skill

When asked to capture or improve a reusable lesson, use
[learning-to-skill](../.agents/skills/learning-to-skill/SKILL.md). Record the
failure once, then add only the decision rule that would have prevented it.
A one-off environment problem may need a fix or a lesson record without a new
skill. Update an existing skill when its trigger and scope already fit.

For a review of many incidents or a request to identify important missing skills,
start with [skill-gap-review](../.agents/skills/skill-gap-review/SKILL.md). It finds
and prioritizes gaps; `learning-to-skill` implements a selected learning. Both work
from tracked evidence without a transcript collector.

The project bindings for these workflows are:

- shared rules: `AGENTS.md`;
- selected-learning workflow: `.agents/skills/learning-to-skill/SKILL.md`;
- skill library: `.agents/skills/`;
- evidence records: `docs/lessons/`, using `YYYY-MM-DD-<topic>.md`;
- release authority: `docs/RELEASING.md` (the only release procedure);
- structural validation: `go test ./tests -run '^TestProject.*Skill' -count=1 -v`;
- broader repository validation: `make test`;
- behavior evaluation: fresh-context task trials recorded with skill revision,
  inputs, expected decisions, actual decisions, and missing coverage.

The `audit-loop` skill is also reusable as a whole directory: its workflow lives
in its own `references/`. For DossierX, bind it to the commands above and include
`dossierx-graph-safety` whenever graph behavior changes. Use `docs/RELEASING.md`
for release authority. The path under `.agents/workflows/` is only a forwarding
pointer for existing callers. On another project, provide that project's gates
and commands in the run contract.

Project-specific facts belong in a reference or lesson, not in a generic rule.
To reuse a generic skill elsewhere, copy its whole directory at a named revision
and supply that project's bindings. Track the upstream revision when vendoring;
updates require review. Do not silently turn a vendored copy into a second
canonical source for this repository. If a shared library becomes necessary,
version it separately and pin consumers instead of copying untracked home files.

A lesson's verified revision is historical evidence. Recheck the current contract
and implementation before applying it. When evidence or product decisions
invalidate a rule, mark the lesson superseded, link its replacement, and update
all affected routing. Do not keep obsolete rules active as accumulating memory.

## What validation establishes

Repository tests check file reachability, relocatable adapters, valid skill
metadata, and exclusion from the consumer bundle. Behavioral trials check how
an agent uses the instructions on realistic tasks, including tasks where they
should not apply. Neither proves the graph algorithm is safe: that requires the
semantic and scale evidence defined by the graph-safety skill.

The first recorded lesson is
[graph path growth](lessons/2026-09-06-graph-path-growth.md).
