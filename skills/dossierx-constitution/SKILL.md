---
name: dossierx-constitution
description: >-
  Drafting and keeping the DossierX constitution — the one project-root
  constitution.yaml every module builds toward — and deciding when a
  project-wide fact is a constitution entry, a project claim (project.<slug>)
  or an ordinary module contract claim. Use this WHENEVER you draft, trim,
  show, lock or re-lock constitution.yaml, run dossierx constitution show or
  lock, meet CONSTITUTION_NOT_LOCKED, CONSTITUTION_OVER_CAP,
  constitution-near-cap or shared-context-budget, or author a project claim.
  Covers what belongs in the 800-word roof and what does not, the file shape,
  the human-gated lock and re-lock loop, the five gate states, and project
  claims. Load the DossierX router skill first.
---

# DossierX constitution — the roof, and project claims

Read **[`dossierx`](../dossierx/SKILL.md)** first for the envelope and the error codes, and
**[`dossierx-claims`](../dossierx-claims/SKILL.md)** for how to write a claim.

## Three places a fact can live

| the fact is… | it goes in | cited by `rests_on`? |
|---|---|---|
| law every module builds toward: a system invariant, a term every module uses, a decision whose reversal changes every module | `constitution.yaml` | **never** — every claim builds toward it by definition |
| project-wide, but not roof law, and something a claim may genuinely rest on | a project claim, `project.<slug>` | yes, from any module |
| what one module guarantees | that module's `contract` (or `internals`) claim | `contract` from anywhere; `internals` from its own module only |

Test for the roof: *if this changed, would every module have to be re-read?* Yes is a constitution
entry. If only the claims that depend on it would, it is a project claim — and it wants to be cited,
which the constitution never is. When in doubt, it is not constitution: the roof is the critical
brief, and every module's `manifest show --isolation` carries all of it.

## The file

One `constitution.yaml` beside `project.config.yaml` (the `constitution:` config key moves it; it
must sit outside `claims_dir`). It is not a module, not a claim, not a graph node:

```yaml
status: draft                 # only `dossierx constitution lock` writes locked
invariants:
  - slug: single-writer       # required, unique
    title: One writer per store   # optional
    body: Only the engine writes build/ledger; nothing else may.
glossary:
  - slug: claim
    body: One reviewable fact, in one file.
decisions:
  - slug: no-network
    body: The CLI never makes a network call.
```

Three sections only, plain text, unknown keys refused, one YAML document. **The cap is 800 words**
— titles and bodies, counted as letter/number runs; slugs do not count. `check` warns
`constitution-near-cap` from 720; over 800, `check` and `constitution lock` refuse
`CONSTITUTION_OVER_CAP`. The cap is fixed — there is no config value — so the only recovery is to
trim, and what to cut is the human's call: show them the entries you would move to project claims or
drop, and wait. Never belongs in it: per-module behavior, anything a claim needs to cite, rationale
essays, module lists, restated framework defaults.

## Lock and re-lock — the human's approval

`dossierx constitution show` prints the full text, the digest (words, cap, hash) and the lock
state. You draft and show; the **human** approves:

1. `dossierx constitution lock --dry-run` — show them the preview and the words.
2. On their yes: `dossierx constitution lock --reason "<their words>"`. It writes `status: locked`
   and records the content hash and their reason in `build/ledger/lock-store.json`. Commit both.

Until it is locked, `claim lock`, `claim reaudit --confirm` and plain `check` refuse
`CONSTITUTION_NOT_LOCKED` (`check --validate` / `--staged` report the `constitution-not-locked`
finding). `error.details.state` says why:

| state | meaning | recovery |
|---|---|---|
| `missing` / `unreadable` | no file, or it does not parse | draft or fix it, then the lock loop |
| `draft` | `status: draft` | the lock loop |
| `unrecorded` | `status: locked` with no record — flipped by hand | the lock loop; never flip `status` yourself |
| `edited` | locked, then edited: **the edited file is what every module now reads**, and nothing from the old version stays in force | show the human the diff (`git diff`), then the lock loop — or restore the file from git |

`claim new`, `claim show`, `claim list`, `serve`, `constitution show` and the bare `claim reaudit
<id>` preview keep working meanwhile, so drafting never stops. Do not loop on `claim lock`. A roof
that is locked **and** unchanged refuses `already_locked`: a lock signs a change, and there is none.
An edit to the constitution never touches a claim — no drift, no `review_pending`.

## Project claims

```
dossierx claim new project.<slug> --summary "..." --body "..." --rests-on project.other,widget.contract.retry-policy
```

It writes `project-claims/<slug>.yaml` (`project_claims_dir` in the config, never inside
`claims_dir`) with `scope: project` and no `module` or `facet`. It is an ordinary claim otherwise:
required `summary` and `rests_on` (or `--rests-on-none-reason`), linted, locked and reviewed like
any other. Its `rests_on` may name other `project.*` ids and any module's `*.contract.*`, never
`*.internals.*`. No count cap, no manifest, and the code-link gate does not apply to it.

Every module's `manifest show --isolation` carries the project claims index — each id with its
summary — together with the constitution text, in one fixed **10240-byte** budget. The project claim
whose index line crosses it gets `shared-context-budget` (project-wide when the constitution alone
is over). Recovery: shorten project claim summaries, retire a project claim, or trim the
constitution — there is no config value to raise.

Upgrading a corpus that still has a doctrine hub or `governed_by`: **[`dossierx-upgrading`](../dossierx-upgrading/SKILL.md)**.
