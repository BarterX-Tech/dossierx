// Content spec for the DossierX static site.
//
// This is the single source of truth for on-page copy, typed so sections can
// import and render it directly.

export interface NavItem {
  id: string;
  label: string;
}

export interface CodeExample {
  title: string;
  lang: string;
  code: string;
}

export type SectionKind =
  | "hero"
  | "narrative"
  | "model-diagram"
  | "lifecycle-diagram"
  | "build-order-diagram"
  | "cli-explorer"
  | "timeline"
  | "comparison"
  | "footer";

export interface Section {
  id: string;
  title: string;
  kind: SectionKind;
  contentMd: string;
  codeExamples?: CodeExample[];
  // `data` shape varies per section kind; typed loosely on purpose.
  data?: Record<string, unknown>;
}

export interface ContentSpec {
  siteTitle: string;
  tagline: string;
  nav: NavItem[];
  sections: Section[];
}

export const contentSpec: ContentSpec = {
  siteTitle: "DossierX",
  tagline:
    "Turn system facts into reviewable YAML claims. DossierX checks them in CI, links them to code, and flags drift before stale documentation becomes trusted truth.",
  nav: [
    { id: "hero", label: "Overview" },
    { id: "philosophy", label: "Why" },
    { id: "claims", label: "Claims" },
    { id: "lifecycle", label: "Lifecycle" },
    { id: "build-order", label: "Build Order" },
    { id: "code-links", label: "Code Links" },
    { id: "cli", label: "CLI" },
    { id: "versions", label: "Releases" },
    { id: "compare", label: "vs. Wiki/ADR" },
  ],
  sections: [
    {
      id: "hero",
      title: "Documentation that makes drift impossible to miss.",
      kind: "hero",
      contentMd:
        "**DossierX** is a config-driven Go CLI that turns a directory of atomic YAML **claims** — one reviewable fact each — into a linted, validated, human-reviewable HTML documentation site, governed by a lock / review_pending / reaudit lifecycle.\n\nIt treats docs like source-controlled assertions, not free-form prose. Every statement is atomic, validated by a linter, reviewed and locked by a human, and protected by an audit trail so it can never silently drift out of truth.\n\nCLI-only by design. No exported Go API. The only project-specific input the engine ever reads is your `project.config.yaml` — point the same binary at any project's config and it becomes that project's documentation engine.",
      data: {
        pitchLine:
          "A config-driven CLI that turns YAML 'claims' into a linted, validated, human-reviewable HTML documentation site, with an audit trail via a lock/lint/reaudit lifecycle.",
        badges: [
          "Go 1.26",
          "cobra + yaml.v3 only",
          "CLI-only, no public API",
          "v0.1.0",
          "github.com/BarterX-Tech/dossierx",
        ],
        pipeline: ["lint", "catalog", "render", "check"],
        ctas: [
          {
            label: "View on GitHub",
            href: "https://github.com/BarterX-Tech/dossierx",
          },
          { label: "Read the model", href: "#claims" },
        ],
      },
    },
    {
      id: "philosophy",
      title: "Ordinary docs rot silently. This exists to stop that.",
      kind: "narrative",
      contentMd:
        "Markdown folders, ADRs, and wikis fail in the same way: prose can become wrong without producing a machine-readable signal. DossierX replaces page-level trust with atomic facts that can be linted, reviewed, locked, and flagged when their dependencies or implementing code move.\n\nIt began as internal tooling inside a private, multi-module production app that had been burned by silent documentation drift. The public tool keeps the proven claim schema, `lint → catalog → render → check` pipeline, lifecycle, build ordering, and code linking while taking all project-specific structure from `project.config.yaml`.\n\nThe same claim boundary serves two readers: humans get a coherent reviewable site; coding agents can load only the locked facts relevant to the work at hand. Three embedded skills teach agents how to author claims, derive build order, and link finished code back to its specification.",
      data: {
        principles: [
          {
            title: "Atomic units",
            body: "One YAML file, one fact — true or false on its own. Review, locking, and dependency edges operate at fact granularity, never at wall-of-prose granularity.",
          },
          {
            title: "Determinism is first-class",
            body: "Catalog output is always alphabetical-by-id and never ranges over Go maps directly — two builds from identical input are byte-for-byte identical, so .catalog.json and lint diffs are reviewable in version control.",
          },
          {
            title: "Confirm-before-write",
            body: "A locked truth never changes without a human reviewing a printed diff and passing an explicit --confirm. Never an automatic overwrite.",
          },
          {
            title: "Structure lives in fields, not paths",
            body: "Directory layout inside claims_dir carries zero meaning — module and facet come only from a claim's own YAML fields, so a project can reorganize its claims tree freely without breaking anything.",
          },
        ],
      },
    },
    {
      id: "claims",
      title: "The claim — the atomic unit of everything.",
      kind: "model-diagram",
      contentMd:
        "A **claim** is one reviewable, YAML-authored fact: prose, a table, a sequence, or a mockup. Its `module.facet.slug` id, content, status, and typed relationships give the engine enough structure to validate what ordinary prose cannot.\n\nClaims form a dependency graph through `mirrors`, `rests_on`, and `governed_by`. Separately, `build_role` places a fact in the implementation sequence, while `kind` distinguishes facts from reading guidance—the schema, examples, and real on-disk layout below show the full contract.",
      codeExamples: [
        {
          title: "A minimal claim",
          lang: "yaml",
          code: "id: widget.contract.overview\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  A widget is the smallest unit this project documents.\ngoverned_by:\n  type: none\n  reason: no doctrine facet configured yet",
        },
        {
          title:
            "A locked schema claim: rests_on a contract claim, backed by doctrine",
          lang: "yaml",
          code: 'id: logger.internals.event-envelope-fields\nfacet: internals\nmodule: logger\nstatus: locked\nlayout: table          # inferred from `rows` if omitted\nbuild_role: schema     # REQUIRED now that status is locked\nsection: "1 - event envelope"\norder: 2               # viewer-only display hint\nrows:\n  - field: event_name\n    type: string\n    notes: stable machine name, carries no dynamic values\n  - field: severity\n    type: enum\n    notes: one shared severity vocabulary\nrests_on:\n  - logger.contract.event-envelope-overview   # target must exist\ngoverned_by:\n  type: platform.doctrine.stable-machine-names-carry-no-dynamic-values',
        },
      ],
      data: {
        requiredFields: [
          {
            name: "id",
            desc: "module.facet.slug — three segments; module/facet must be configured, slug unique within the pair",
          },
          {
            name: "facet",
            desc: "which tab it renders under (e.g. contract vs internals)",
          },
          { name: "module", desc: "the module it belongs to" },
          {
            name: "status",
            desc: "draft | locked — changed only via lock/unlock, never hand-edited",
          },
          {
            name: "content",
            desc: "at least one of body (markdown), rows (table, column-checked), or steps (ordered)",
          },
        ],
        optionalFields: [
          {
            name: "layout",
            desc: "card | table | list | steps | tree | banner | mockup. Inferred: rows→table, steps→steps, else card. list/tree/banner/mockup are never inferred.",
          },
          {
            name: "build_role",
            desc: "orientation | schema | behavior | api | verification | out-of-scope. Optional while draft, REQUIRED once locked.",
          },
          {
            name: "kind",
            desc: "fact (default) | orientation-note. A different axis from build_role.",
          },
          { name: "section", desc: "free-form in-content heading label" },
          {
            name: "order",
            desc: "viewer-only ascending sort hint, unrelated to catalog's alphabetical-by-id ordering",
          },
          { name: "emphasis", desc: "renders as a warn / hard-boundary card" },
          {
            name: "migrated_from",
            desc: "provenance note (what the coverage command counts)",
          },
          {
            name: "raw_html + raw_html_reviewed",
            desc: "review gate for unescaped HTML in layout: mockup claims",
          },
        ],
        engineManagedFields: [
          {
            name: "review_pending",
            desc: "set true by internal/lock when a rests_on target's content hash drifts under a locked claim; cleared only by a confirmed reaudit --confirm",
          },
          {
            name: "audit_notes",
            desc: "durable provenance trail appended by each confirmed reaudit",
          },
          {
            name: "SourcePath",
            desc: 'Go-only (yaml:"-"); the file the claim loaded from',
          },
        ],
        edgeTypes: [
          {
            name: "mirrors",
            semantics:
              "Deterministic value-equality. Two claims must say the same thing; divergence is a hard lint failure (mirror-mismatch), not staleness. Both sides must declare it (reciprocal).",
          },
          {
            name: "rests_on",
            semantics:
              "Semantic-consequence dependency. Target must exist. A changed target under a locked claim flags it review_pending — it is not invalidated outright. Also topologically orders behavior claims in build order.",
          },
          {
            name: "governed_by",
            semantics:
              "Names the doctrine claim backing this claim's authority — or type: none with a required reason. With doctrine_facet set, a claim can't lock until its named doctrine claim is itself locked (hub-gating).",
          },
        ],
      },
    },
    {
      id: "lifecycle",
      title: "Lock, drift, reaudit — the trust mechanism.",
      kind: "lifecycle-diagram",
      contentMd:
        "A locked claim is a **trust assertion**: a human reviewed it, lint passed, and other claims may safely depend on it. If a dependency hash changes—or an agent reports that code changed meaning—the claim stays locked but becomes visibly `review_pending`.\n\nDetection can only raise the flag. Clearing it requires `reaudit --confirm`, which presents the proposed change before writing, records the audit, refreshes the baseline, and restores the locked state. Drift becomes loud; re-approval stays deliberate.",
      data: {
        states: [
          {
            id: "draft",
            label: "draft",
            desc: "Freely editable, not yet trusted.",
          },
          {
            id: "locked",
            label: "locked",
            desc: "Reviewed source of truth. Lint-gated. Others may depend on it.",
          },
          {
            id: "review_pending",
            label: "locked + review_pending",
            desc: "Still locked, but visibly flagged: a dependency drifted or code no longer matches.",
          },
        ],
        transitions: [
          {
            from: "draft",
            to: "locked",
            trigger: "dossierx lock <id>",
            note: "Human-initiated. Refused on any lint error. Hub-gating enforced.",
          },
          {
            from: "locked",
            to: "review_pending",
            trigger: "DetectStale (auto)",
            note: "A mirrors/rests_on dependency's content hash changed. Persisted back to the claim file by dossierx check.",
          },
          {
            from: "locked",
            to: "review_pending",
            trigger: "dossierx flag <id>",
            note: "Agent asserts code drifted. Requires --claim-says --now-does --reason, all non-empty. Locked-only.",
          },
          {
            from: "review_pending",
            to: "locked",
            trigger: "dossierx reaudit <id> --confirm",
            note: "The ONLY way to clear the flag. Prints diff first; writes only on --confirm. Appends audit_notes.",
          },
          {
            from: "locked",
            to: "draft",
            trigger: "dossierx unlock <id>",
            note: "Manual escape hatch. No lint gate — you may need to unlock precisely to fix what lint complains about.",
          },
        ],
        invariant:
          "A locked claim's Status never reverts to draft on its own; review_pending is the only automatic transition, and it is one-directional until a human confirms a reaudit.",
      },
    },
    {
      id: "build-order",
      title: "Build Order — the sequence to actually implement in.",
      kind: "build-order-diagram",
      contentMd:
        "Once a module is fully locked, DossierX turns `build_role` and `rests_on` into a dependency-safe implementation sequence. Human reading order remains independent: one optimizes comprehension; the other tells an implementer what must exist first.\n\nFive phases stay fixed, while behavior and API claims are topologically sorted inside their phase. Proposal fails on drafts or cycles, reports excluded work, and becomes a drift-checked artifact only after a human locks it.",
      data: {
        phases: [
          {
            phase: "orientation",
            role: "Context / process claims. Read for background; you never build code directly from these.",
            ordering: "stable display order",
          },
          {
            phase: "schema",
            role: "Data shapes and types. Built first — everything below assumes these types exist.",
            ordering: "stable display order",
          },
          {
            phase: "behavior",
            role: "Workflow / logic. The bulk of the real work.",
            ordering: "topological sort over rests_on",
          },
          {
            phase: "api",
            role: "Public entry points, built over existing behavior.",
            ordering: "topological sort over rests_on",
          },
          {
            phase: "verification",
            role: "Test / acceptance checklists, read last — tests written against everything already built.",
            ordering: "stable display order",
          },
          {
            phase: "out-of-scope",
            role: "Deferred / future scope. Excluded from the sequence but reported so nothing silently disappears.",
            ordering: "excluded (reported)",
          },
        ],
        subcommands: [
          {
            cmd: "dossierx build-order propose --module <name>",
            desc: "Derive the sequence, write .build-order.<module>.json (locked:false). Refused unless every claim in the module is locked; fails on same-phase cycle.",
          },
          {
            cmd: "dossierx build-order status --module <name>",
            desc: "Reports proposed / locked / stale plus coverage (N of M covered, K excluded).",
          },
          {
            cmd: "dossierx build-order lock --module <name>",
            desc: "Freeze the proposed sequence, snapshotting a content-hash baseline. Human confirms first.",
          },
        ],
      },
    },
    {
      id: "code-links",
      title: "Code links — closing the loop between docs and code.",
      kind: "narrative",
      contentMd:
        "DossierX keeps a drift-checked map from every locked claim to the source that implements it. A `dossierx-claim: <id>` marker lets `dossierx check` link code automatically; unknown or unlocked ids fail the check, and later file changes surface as drift.\n\nThe human gate stays separate: re-tag code when meaning is unchanged; use `dossierx flag` when implementation and claim now disagree. That sends the claim through the same visible `review_pending → reaudit` path instead of silently rewriting either side.",
      codeExamples: [
        {
          title: "Channel B — the everyday case (any comment syntax works)",
          lang: "python",
          code: "# dossierx-claim: widget.internals.queue-saturation-policy\ndef _drop_for_saturation(self):\n    ...\n# Run `dossierx check`: Scan finds this marker in source_dirs, looks up the\n# claim's module, and auto-links the file. Only the literal marker matters.",
        },
        {
          title:
            "Manual fallback when scanning can't reach (no source_dirs, generated files)",
          lang: "bash",
          code: "dossierx implink set --module widget --claim widget.internals.queue-saturation-policy \\\n  --file src/widget/queue.py --symbol _drop_for_saturation\n\ndocs implink status --module widget   # read-only: drift + coverage",
        },
        {
          title: "The impl-links status block emitted inside `dossierx check`",
          lang: "text",
          code: "impl-links: scanned 214 file(s), found 37 tag(s), reconciled 37 link(s) (0 error(s))\ncheck: OK\nimpl-links: 35 linked, 1 drifted, 2 unlinked-in-schema/behavior/api/verification-phases\n  drifted: widget.behavior.retry src/widget/retry.py: file changed since linked at 2026-05-01T...\n  unlinked: widget.api.enqueue",
        },
      ],
      data: {
        channels: [
          {
            channel: "A — spec is wrong",
            about: "The locked claim itself needs revisiting",
            owner: "Human, via dossierx flag → dossierx reaudit",
            trigger: "Meaning changed vs. what's locked",
          },
          {
            channel: "B — grounding correct code",
            about: "Where the (still-correct) claim lives in code",
            owner: "Fully agent-autonomous, no human gate",
            trigger: "Code finished, or a linked file still matches",
          },
        ],
        unlinkedNote:
          "The unlinked count only covers locked claims in the four code-producing build roles (schema/behavior/api/verification). orientation and out-of-scope claims are expected to have no code link.",
      },
    },
    {
      id: "cli",
      title: "The CLI — 17 commands, zero hardcoded project.",
      kind: "cli-explorer",
      contentMd:
        "One binary serves any project through `project.config.yaml`, discovered from the working tree or supplied with `--config`. Use the explorer below for the full command surface; `check` is the CI entry point that detects drift, validates claims, renders the viewer, and verifies code links.",
      data: {
        groups: [
          {
            group: "Render pipeline",
            commands: [
              {
                name: "lint",
                usage: "dossierx lint [--json]",
                summary: "Run all 21 lints in isolation and across the set.",
                detail:
                  "Warnings (e.g. orphan) print but don't fail; any error-severity finding returns exit 1. --json emits findings as indented JSON.",
                example:
                  "$ dossierx lint\n[warning] orphan: logger.overview.router: claim has no edges\nlint: 12 finding(s), 0 error(s)",
              },
              {
                name: "catalog",
                usage: "dossierx catalog",
                summary:
                  "Compile validated claims to a deterministic .catalog.json.",
                detail:
                  "Always id-sorted, never ranges over a Go map — two builds from identical input are byte-identical.",
                example:
                  "$ dossierx catalog\ncatalog: wrote /path/dossierx/.catalog.json (186 claim(s))",
              },
              {
                name: "render",
                usage: "dossierx render",
                summary: "Build the self-contained viewer/index.html.",
                detail:
                  "Builds the catalog in memory (does NOT read .catalog.json), renders HTML, writes viewer/index.html stamped with a generated-at header.",
                example:
                  "$ dossierx render\nrender: wrote /path/dossierx/viewer/index.html",
              },
              {
                name: "check",
                usage: "dossierx check",
                summary:
                  "The routine CI/pre-commit command — does more than lint+catalog+render.",
                detail:
                  "1) DetectStale flips drifted locked claims to review_pending and persists the flag to each file. 2) lint → catalog → render, stopping at first failure. 3) Scans source_dirs for dossierx-claim tags (unknown/unlocked tag = HARD failure). 4) Prints non-blocking orientation-note counts, impl-link drift, and a next-steps block.",
                example:
                  '$ dossierx check\ncheck: OK\nnext steps:\n  4 claim(s) still draft -> dossierx lock <id>\n  module "logger" fully locked, no build order -> dossierx build-order propose --module logger',
              },
            ],
          },
          {
            group: "Lock lifecycle",
            commands: [
              {
                name: "lock",
                usage: "dossierx lock <id>",
                summary: "Promote a draft claim to locked.",
                detail:
                  "Refused if lint fails or hub-gating blocks (rests_on an unlocked doctrine claim). Takes a file lock, saves the claim, snapshots a content-hash baseline.",
                example:
                  "$ dossierx lock logger.contract.api-surface\nlock: logger.contract.api-surface is now locked",
              },
              {
                name: "unlock",
                usage: "dossierx unlock <id>",
                summary: "Return a locked claim to draft.",
                detail:
                  "Always human-initiated, no lint gate — you may need to unlock precisely to fix what lint complains about. Clears review_pending.",
                example:
                  "$ dossierx unlock logger.contract.api-surface\nunlock: logger.contract.api-surface is now draft",
              },
              {
                name: "stale",
                usage: "dossierx stale",
                summary: "List locked claims currently flagged review_pending.",
                detail: "Distinguishes 'nothing locked' from '0 flagged'.",
                example:
                  "$ dossierx stale\nstale: 0 claim(s) flagged review_pending",
              },
              {
                name: "reaudit",
                usage: "dossierx reaudit <id> [--confirm]",
                summary:
                  "The single confirm-before-write gate for a review_pending claim.",
                detail:
                  "Only valid on locked+review_pending (else exit 2). Prints the proposed diff and stops unless --confirm. Two sources converge: a flagged claim gets a real before/after diff (ProposeFlagDiff); any other gets a dependency-drift stub (ProposeDiff). On confirm: applies, appends audit_notes, re-baselines hashes, clears the flag.",
                example:
                  "$ dossierx reaudit logger.internals.dispatch\nreaudit: not applied (pass --confirm to apply)\n$ dossierx reaudit logger.internals.dispatch --confirm\nreaudit: applied, review_pending cleared",
              },
              {
                name: "flag",
                usage:
                  "dossierx flag <id> --claim-says … --now-does … --reason …",
                summary:
                  "Agent-initiated reaudit trigger for code that drifted from a claim.",
                detail:
                  "All three flags required and non-empty. Only a LOCKED claim can be flagged. Writes a one-shot PendingFlag to .dossierx-flag-store.json and sets review_pending=true.",
                example:
                  '$ dossierx flag logger.internals.dispatch \\\n  --claim-says "dispatch is synchronous" \\\n  --now-does   "dispatch runs on a worker pool" \\\n  --reason     "concurrency added in PR #42"',
              },
            ],
          },
          {
            group: "Inspection",
            commands: [
              {
                name: "deps",
                usage: "dossierx deps <id>",
                summary: "Print a claim's edges in both directions.",
                detail:
                  "Outgoing mirrors, outgoing rests_on, governed_by (type + reason, or (unset)), plus computed incoming mirrors / incoming rests_on. Exit 2 if the id isn't found.",
                example:
                  "$ dossierx deps logger.contract.api-surface\n  outgoing rests_on:  [logger.doctrine.single-writer]\n  incoming mirrors:   [platform.contract.telemetry-facade]",
              },
              {
                name: "coverage",
                usage: "dossierx coverage",
                summary: "Percentage of claims carrying a migrated_from note.",
                detail:
                  "Migration provenance — NOT the code-link report. That surfaces via the impl-links status block.",
                example:
                  "$ dossierx coverage\ncoverage: 0/186 claim(s) carry migrated_from (0.0%)",
              },
            ],
          },
          {
            group: "build-order (subcommands, each requires --module)",
            commands: [
              {
                name: "build-order propose",
                usage: "dossierx build-order propose --module <name>",
                summary: "Derive the phased implementation sequence.",
                detail:
                  "Writes .build-order.<module>.json (locked:false) split into orientation/schema/behavior/api/verification + excluded. Refused unless every claim in the module is locked; fails on a same-phase rests_on cycle.",
                example:
                  "$ dossierx build-order propose --module logger\n  schema  5   behavior 11   api 4   verification 6   excluded 2\n  locked: false",
              },
              {
                name: "build-order status",
                usage: "dossierx build-order status --module <name>",
                summary: "Report proposed / locked / stale + coverage.",
                detail:
                  "Coverage: N of M claims covered, K excluded as out-of-scope. Prints a 'not proposed yet' hint when no artifact exists.",
                example:
                  "$ dossierx build-order status --module logger\n  proposed: true  locked: false  stale: false\n  coverage: 29 of 31 covered (2 excluded)",
              },
              {
                name: "build-order lock",
                usage: "dossierx build-order lock --module <name>",
                summary: "Freeze the proposed sequence.",
                detail:
                  "Snapshots a content-hash baseline and stamps locked_at. Refuses if nothing proposed, or already locked and not stale.",
                example:
                  "$ dossierx build-order lock --module logger\nbuild-order lock: logger locked at 2026-07-22T09:14:03Z",
              },
            ],
          },
          {
            group: "implink (subcommands, each requires --module)",
            commands: [
              {
                name: "implink set",
                usage:
                  "dossierx implink set --module <name> --claim <id> --file <path> [--symbol <name>]",
                summary:
                  "Manually link code to a claim — immediate, no confirm step.",
                detail:
                  "The manual counterpart to check's automatic dossierx-claim scan. Validates: claim exists → belongs to module → is locked → file resolves project-relative (absolute paths and .. escapes refused). Snapshots the file's sha256 as drift baseline.",
                example:
                  "$ dossierx implink set --module logger \\\n  --claim logger.internals.dispatch \\\n  --file internal/logger/dispatch.go --symbol Dispatcher.Run",
              },
              {
                name: "implink status",
                usage: "dossierx implink status --module <name>",
                summary: "Report coverage plus drifted and unlinked claim ids.",
                detail: "Read-only companion to check's impl-links block.",
                example:
                  '$ dossierx implink status --module logger\nimplink: module "logger": 7 of 31 claim(s) linked\n  drifted: logger.internals.dispatch ...\n  unlinked: logger.contract.api-surface',
              },
            ],
          },
          {
            group: "skills",
            commands: [
              {
                name: "skills export",
                usage: "dossierx skills export <dir>",
                summary:
                  "Write the binary's three embedded Claude Code skills to <dir>.",
                detail:
                  "Walks the embedded skills/ FS (dossierx-claims, dossierx-build-order, dossierx-code-links), preserves layout, creates parent dirs, overwrites existing files.",
                example:
                  "$ dossierx skills export ./.claude/skills\nskills export: wrote 3 file(s) to ./.claude/skills",
              },
            ],
          },
        ],
        globalFlag: {
          name: "--config string",
          desc: "Path to project.config.yaml. Empty (default) walks upward from cwd like git finds .git; not found → exit 2.",
        },
      },
    },
    {
      id: "versions",
      title: "Release history.",
      kind: "timeline",
      contentMd:
        "Four tagged releases trace the engine's public extraction, cross-platform hardening, a viewer freshness cue, and the docs→dossierx naming rebrand. The current release is **v0.1.0**.",
      data: {
        releases: [
          {
            version: "v0.0.1",
            date: "2026-07-21",
            title: "Initial public release",
            tag: "First commit 3561a2d — 'feat: initial release — claims-to-HTML documentation engine'",
            highlights: [
              "Extracts the proven internal engine into its own repository and removes project-specific names, facets, and modules; project.config.yaml becomes the only project-specific input.",
              "Ships the full CLI: lint, catalog, render, check, deps, coverage, stale, lock/unlock, reaudit, build-order, flag, implink.",
              "21 lint rules, each with an isolated test fixture plus a coverage meta-gate.",
              "Three vendorable Claude Code skills embedded in the binary: dossierx-claims, dossierx-build-order, dossierx-code-links.",
              "The lock / review_pending / reaudit lifecycle as the core value — confirm-before-write on every locked change.",
            ],
          },
          {
            version: "v0.0.2",
            date: "2026-07-22",
            title: "Cross-platform hardening (one real bug)",
            tag: "First real CI run — Linux/Windows/macOS matrix with -race, gofmt, golangci-lint",
            highlights: [
              "HEADLINE BUG FIX: many concurrent `dossierx lock` runs against the same claims_dir could fail on Windows with a transient 'being used by another process' error (mandatory file locking colliding a concurrent atomic rename with a concurrent read). Both read and write paths in internal/loader now retry with short backoff, Windows-only.",
              "User-facing upshot: DossierX now runs reliably on Windows.",
              "Corrected minor gofmt drift in two files.",
              "CLI-integration test harness appends .exe so os/exec can launch the test binary on Windows.",
              "Two POSIX-permission tests skipped on Windows (don't fit its ACL model); an inconclusive concurrency negative-control assertion skipped on windows-latest.",
              "Tightened golangci-lint config/version pinning to match the module's go 1.26 floor.",
            ],
          },
          {
            version: "v0.0.3",
            date: "2026-07-22",
            title: "Viewer freshness cue",
            tag: "Previous release",
            highlights: [
              "The rendered viewer's sidebar now shows a 'Generated <timestamp>' footer, surfacing the same render-time timestamp already embedded in the leading generated-by HTML comment.",
              "A reviewer can tell at a glance how fresh or stale a documentation page is without viewing page source.",
              "Small, purely additive usability improvement.",
            ],
          },
          {
            version: "v0.1.0",
            date: "2026-07-23",
            title: "docs → dossierx naming rebrand",
            tag: "Latest release · PR #7",
            highlights: [
              "Every generic 'docs' placeholder — CLI-invocation examples, the docs-claim: source tag (including the real Go regex), docs-v1 naming in skill docs, the default viewer title, and the on-disk store filenames — is renamed to the tool's actual name, dossierx.",
              "BREAKING: .docs-lock-store.json and .docs-flag-store.json are renamed to .dossierx-lock-store.json and .dossierx-flag-store.json, with no migration. An existing project's lock/flag store will not be found after upgrading past this release.",
              "Minor version bump (v0.0.3 → v0.1.0), not a patch, to signal the breaking on-disk change under pre-1.0 semver.",
            ],
          },
        ],
      },
    },
    {
      id: "compare",
      title: "Why DossierX over a wiki, ADRs, or a docs/ folder.",
      kind: "comparison",
      contentMd:
        "The tradeoff is deliberate: YAML structure and confirm-before-write add friction, so DossierX is overkill for notes or narrative writing. It is built for durable contracts, schemas, behaviors, and invariants where silently wrong documentation is expensive.\n\n**A wiki optimizes for writing documentation. DossierX optimizes for trusting it.**",
      data: {
        columns: [
          "Property a skeptical lead cares about",
          "Wiki / Notion",
          "ADRs",
          "Plain docs/ markdown",
          "DossierX",
        ],
        rows: [
          {
            property: "Unit of trust",
            wiki: "a whole page",
            adr: "a whole decision",
            markdown: "a whole file",
            dossierx: "one atomic fact",
          },
          {
            property: '"Is this reviewed & safe to depend on?"',
            wiki: "no signal",
            adr: "accepted ≠ still-true",
            markdown: "no signal",
            dossierx: "per-fact locked state, lint-gated",
          },
          {
            property: "Dependencies between facts",
            wiki: "in your head",
            adr: "prose references",
            markdown: "prose references",
            dossierx: "validated rests_on graph, cycle-checked",
          },
          {
            property: "Notices when code drifts",
            wiki: "never",
            adr: "never",
            markdown: "never",
            dossierx: "flags review_pending, confirm-before-write",
          },
          {
            property: "Tells you what to build first",
            wiki: "no",
            adr: "no",
            markdown: "no",
            dossierx: "derivable build-order artifact",
          },
          {
            property: "Enforced in CI",
            wiki: "no",
            adr: "no",
            markdown: "no",
            dossierx: "dossierx check gates the build",
          },
          {
            property: "Every claim names what backs its truth",
            wiki: "no",
            adr: "partial",
            markdown: "no",
            dossierx:
              "required governed_by (doctrine or explicit none + reason)",
          },
        ],
      },
    },
    {
      id: "footer",
      title: "DossierX",
      kind: "footer",
      contentMd:
        "Open source, CLI-only, and built in Go. Install it as a pinned tool: `tool github.com/BarterX-Tech/dossierx/cmd/dossierx`.",
      data: {
        links: [
          {
            label: "GitHub — BarterX-Tech/dossierx",
            href: "https://github.com/BarterX-Tech/dossierx",
          },
          { label: "Overview", href: "#hero" },
          { label: "Claim model", href: "#claims" },
          { label: "Lifecycle", href: "#lifecycle" },
          { label: "CLI reference", href: "#cli" },
        ],
        note: "Page content is grounded in the repository's README, CHANGELOG, FORMAT.md, ROADMAP, and implementation packages.",
      },
    },
  ],
};
