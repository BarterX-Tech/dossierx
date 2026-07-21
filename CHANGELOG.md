# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.1] - 2026-07-21

DossierX is a config-driven CLI that turns YAML "claims" — atomic, reviewable facts about a
system — into a linted, validated, human-reviewable HTML documentation site, with a built-in
audit trail via a lock/lint/reaudit lifecycle: a claim is freely editable while in `draft`,
gets promoted to `locked` only once it passes lint, and any subsequent drift (a changed
dependency, code that no longer matches) is surfaced as `review_pending` and resolved through
an explicit, confirm-before-write reaudit rather than a silent auto-update.

The engine originated as an internal documentation tool built and hardened against real,
multi-module projects — proving out the claim schema, the lint → catalog → render → check
pipeline, the lock lifecycle, per-module build ordering, and claim-to-code linking against
production use before anything here was written with an external audience in mind. This
release extracts that engine into its own repository and genericizes it: every project-specific
name, facet, and module that had leaked into the original code has been removed, so the only
project-specific input the CLI now reads is a project's own `project.config.yaml`.

This is DossierX's first public release. It ships the `dossierx` CLI (`lint`, `catalog`,
`render`, `check`, `deps`, `coverage`, `stale`, `lock`/`unlock`, `reaudit`, `build-order`,
`flag`, `implink`), documented in [README.md](README.md), along with three Claude Code skills
in `skills/` for projects that consume DossierX to author claims, derive build order, and link
code back to claims from within an agentic workflow.

[Unreleased]: https://github.com/BarterX-Tech/dossierx/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/BarterX-Tech/dossierx/releases/tag/v0.0.1
