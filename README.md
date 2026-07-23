# DossierX

A config-driven CLI that turns YAML "claims" into a linted, validated, human-reviewable HTML documentation site, with an audit trail via a lock/lint/reaudit lifecycle.

[![CI](https://img.shields.io/github/actions/workflow/status/BarterX-Tech/dossierx/ci.yml?branch=main&label=CI)](https://github.com/BarterX-Tech/dossierx/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/BarterX-Tech/dossierx)](https://github.com/BarterX-Tech/dossierx/releases)

DossierX is a CLI only — there is no public Go package API. Everything below is invoked through the `dossierx` binary; `internal/` is not importable by other modules and its structure is free to change between releases without that counting as a breaking change.

## Install

```sh
go install github.com/BarterX-Tech/dossierx/cmd/dossierx@latest
```

This requires a working Go 1.26+ toolchain. Prebuilt binaries for common platforms are also attached to each [GitHub release](https://github.com/BarterX-Tech/dossierx/releases).

## Quick start

A DossierX project is a `project.config.yaml` file plus a directory of claim YAML files.

**`project.config.yaml`**

```yaml
schema_version: 1
facets:
  - contract
  - internals
modules:
  - widget
claims_dir: claims
```

**`claims/widget-overview.yaml`**

```yaml
id: widget.contract.overview
facet: contract
module: widget
status: draft
layout: card
body: |
  A widget is the smallest unit this project documents.
governed_by:
  type: none
  reason: no doctrine facet configured yet
```

Then, from the directory containing `project.config.yaml`:

```sh
dossierx lint      # validate the claim(s) above
dossierx catalog    # build .catalog.json from claims
dossierx render      # generate viewer/index.html from the catalog
```

`dossierx check` runs all three of the above in one shot and stops at the first failure — this is the command most CI setups and pre-commit hooks should call. Open the generated `viewer/index.html` in a browser to see the rendered site.

## CLI reference

Every subcommand accepts the global `--config` flag: a path to `project.config.yaml`. When omitted, DossierX searches upward from the current directory for one, the same way `git` finds `.git`.

| Command | Description |
|---|---|
| `dossierx lint [--json]` | Run every lint against `claims_dir`; `--json` prints findings as JSON instead of text. |
| `dossierx catalog` | Build `.catalog.json` from the claims directory. |
| `dossierx render` | Generate `viewer/index.html` from the catalog. |
| `dossierx check` | Run lint, catalog, and render in one shot, stopping at the first failure. |
| `dossierx deps <id>` | Print a claim's edge graph in both directions (what it rests on, what rests on it). |
| `dossierx coverage` | Report the percentage of claims carrying a `migrated_from` note. |
| `dossierx stale` | List locked claims currently flagged `review_pending`. |
| `dossierx lock <id>` | Lock a draft claim; refused if lint fails. |
| `dossierx unlock <id>` | Unlock a locked claim back to `draft`. |
| `dossierx reaudit <id> [--confirm]` | Propose a diff for a `review_pending` claim; `--confirm` applies it. Without `--confirm`, only prints the proposed diff. |
| `dossierx build-order propose --module <name>` | Write a build-order artifact for a module; refused unless every claim in it is locked. |
| `dossierx build-order status --module <name>` | Show a module's build-order artifact state: proposed/locked/stale, plus coverage. |
| `dossierx build-order lock --module <name>` | Lock a module's proposed build-order artifact, snapshotting a content-hash baseline. |
| `dossierx flag <id> --claim-says --now-does --reason` | Flag a locked claim whose stated behavior no longer matches reality, marking it `review_pending`. |
| `dossierx implink set --module --claim --file [--symbol]` | Manually record that a claim is implemented in a source file (for links scanning can't reach). |
| `dossierx implink status --module <name>` | Report which claims in a module are linked to code and which are drifted or unlinked. |

### Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Failure — a lint error, a validation failure, a malformed claim, etc. |
| `2` | Not found / not in the right state — e.g. no `project.config.yaml` found, a claim id that doesn't exist, or a claim that isn't `review_pending` when a command requires it to be. |

## Concepts

**Claims.** A claim is one atomic, YAML-authored fact about a system: a field table, a sequence of steps, a paragraph of prose, a piece of hand-authored mockup HTML. Each claim has an `id`, a `facet` (which tab it renders under, e.g. `contract` or `internals`), a `module`, a `status` (`draft` or `locked`), a `layout` (how it's rendered — `card`, `table`, `list`, `steps`, `tree`, `banner`, `mockup`), and a `governed_by` block naming what backs its truth (a doctrine claim, or `none`). Claims can name other claims they `rests_on`, forming a dependency graph the CLI can walk and validate.

**The pipeline: lint → catalog → render → check.** `lint` validates every claim in isolation and across the whole set (id shape, facet/module membership, dependency cycles, and more). `catalog` compiles the validated claims into a single `.catalog.json`. `render` turns that catalog into a static `viewer/index.html` site. `check` chains all three and stops at the first failure — the command to wire into CI.

**The lock/review_pending/reaudit lifecycle.** A new claim starts as `draft` and is freely editable. `dossierx lock <id>` promotes it to `locked` — refused if lint doesn't pass — signaling it has been reviewed and is now a source of truth other claims and code can depend on. A locked claim never silently changes: if a dependency it `rests_on` changes, or code that implements it changes in a way that no longer matches, the claim is flagged `review_pending` (via `dossierx flag`) rather than auto-updated. `dossierx reaudit <id>` proposes a diff to resolve the pending state and only writes it with an explicit `--confirm`, so every change to a locked claim goes through a human-reviewed, confirm-before-write step — never an automatic overwrite.

## Config schema (`project.config.yaml`)

| Field | Type | Required | Description |
|---|---|---|---|
| `schema_version` | int | yes | Must equal the version this DossierX build understands (currently `1`). |
| `facets` | []string | yes | The non-empty, deduplicated list of facet names (tabs) this project uses, e.g. `[contract, internals]`. |
| `modules` | []string | yes | The non-empty, deduplicated list of module names this project documents. |
| `claims_dir` | string | yes | Directory of claim YAML files, resolved relative to `project.config.yaml`'s own directory (never the process's current working directory). |
| `title` | string | no | The viewer's display name — used in `<title>`, the header, and the sidebar heading. Defaults to a generic fallback when unset. |
| `eyebrow` | string | no | A one-line subtitle rendered under the title in the sidebar header. No line is rendered when unset. |
| `doctrine_facet` | string | no | Names one of `facets` as the project's doctrine facet, enabling hub-gating. Must be a facet the project actually declares. |
| `source_dirs` | []string | no | Directories (relative to the config file) scanned for `dossierx-claim: <id>` source comments — the code side of claim-to-code linking. Unset means "do not scan." |
| `mockup_modules` | []string | no | The allowlist of modules permitted to author `layout: mockup` claims. Every entry must also appear in `modules`. Unset/empty means no module may. |
| `viewer.template_overrides` | string | no | A directory of partial-template overrides, resolved relative to the config file. Missing individual partials fall back to engine defaults; a configured-but-missing directory is a hard error. |
| `viewer.theme` | map[string]string | no | CSS custom-property overrides. Keys must be drawn from the fixed allowlist below (without the leading `--`); values are validated defensively before being injected into a generated stylesheet. |

`viewer.theme`'s 14 allowed keys: `accent`, `accent-bg`, `ink`, `muted`, `faint`, `paper`, `card-bg`, `border`, `link`, `warn`, `warn-bg`, `font-sans`, `font-mono`, `radius`.

Config loading is strict: an unknown top-level or `viewer.theme` field is a hard error, not silently ignored.

## Using DossierX's skills in your project

DossierX ships three [Claude Code](https://claude.com/claude-code) skills — `dossierx-claims`, `dossierx-build-order`, and `dossierx-code-links` — that teach an agent working in a *consuming* project how to author claims, derive a module's build order, and link finished code back to the claims it implements. See [`skills/`](skills/) for what each one covers.

To use them in a project that depends on DossierX:

1. Install the pinned CLI: `go install github.com/BarterX-Tech/dossierx/cmd/dossierx@<version>`.
2. Export the skills into your project: `dossierx skills export .claude/skills/`.
3. Write your project's `project.config.yaml` (see the schema above).
4. Optionally, add a project-specific overlay skill alongside the exported ones for anything local to your repo (house style, module-specific conventions) that the generic skills can't know about.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, package boundary rules, and how to run tests and lint locally.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
