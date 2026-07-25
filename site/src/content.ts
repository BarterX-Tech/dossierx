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
  | "comments-workflow"
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
    "Turn system facts into reviewable YAML claims. DossierX checks them in CI, links them to code, opens threaded review comments on any claim, and flags drift before stale documentation becomes trusted truth.",
  nav: [
    { id: "hero", label: "Overview" },
    { id: "philosophy", label: "Why" },
    { id: "claims", label: "Claims" },
    { id: "lifecycle", label: "Lifecycle" },
    { id: "comments", label: "Comments" },
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
        "**DossierX** is a config-driven Go CLI that turns a directory of atomic YAML **claims** — one reviewable fact each — into a linted, validated, human-reviewable HTML documentation site, governed by a lock / review_pending / reaudit lifecycle.\n\nIt treats docs like source-controlled assertions, not free-form prose. Every statement is atomic, validated by a linter, reviewed and locked by a human, and protected by an audit trail so it can never silently drift out of truth.\n\nReview happens on the claims themselves: threaded comments attach to any claim, a human and an agent talk it out, and an unresolved thread blocks the claim from locking. Open `dossierx serve` for a live, localhost-only viewer where a reviewer resolves threads in the browser while the agent works from the CLI.\n\nCLI-first by design. No public API. The only project-specific input the engine ever reads is your `project.config.yaml` — point the same binary at any project's config and it becomes that project's documentation engine.",
      data: {
        pitchLine:
          "A config-driven CLI that turns YAML 'claims' into a linted, validated, human-reviewable HTML documentation site, with threaded review comments and an audit trail via a lock / review_pending / reaudit lifecycle.",
        badges: [
          "Go 1.26",
          "cobra + yaml.v3 only",
          "CLI-first, no public API",
          "v0.2.0",
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
        "Markdown folders, ADRs, and wikis fail in the same way: prose can become wrong without producing a machine-readable signal. DossierX replaces page-level trust with atomic facts that can be linted, reviewed, locked, and flagged when their dependencies or implementing code move.\n\nIt began as internal tooling inside a private, multi-module production app that had been burned by silent documentation drift. The public tool keeps the proven claim schema, `lint → catalog → render → check` pipeline, lifecycle, build ordering, and code linking while taking all project-specific structure from `project.config.yaml`.\n\nThe same claim boundary serves two readers: humans get a coherent reviewable site; coding agents can load only the locked facts relevant to the work at hand. Four embedded skills teach agents how to author claims, derive build order, link finished code back to its specification, and review claims with threaded comments.",
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
            desc: "set on a locked claim by any of three triggers — a rests_on/mirrors target's content hash drifts, an agent runs dossierx flag, or an open comment thread — and cleared by a confirmed reaudit --confirm, an unlock, or resolving the last open thread once no other trigger stands",
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
        "A locked claim is a **trust assertion**: a human reviewed it, lint passed, and other claims may safely depend on it. Three things can flag it `review_pending` without unlocking it: a dependency's content hash drifts, an agent reports that code changed meaning via `dossierx flag`, or someone opens a comment thread on it.\n\nEach trigger has its own clearer. Drift and flags clear through `reaudit --confirm`, which presents the proposed change before writing, records the audit, and refreshes the baseline. A comment trigger clears when the last open thread is resolved — unless drift or a flag also stands. And `unlock` clears any of them by returning the claim to draft. Drift and open discussion become loud; re-approval stays deliberate.",
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
            from: "locked",
            to: "review_pending",
            trigger: "dossierx comment add <id>",
            note: "A new open comment thread flags a locked claim for review — a legal, long-lived state. (A claim cannot lock in the first place while it has an unresolved thread.)",
          },
          {
            from: "review_pending",
            to: "locked",
            trigger: "dossierx reaudit <id> --confirm",
            note: "Clears a drift- or flag-triggered review. Prints the diff first; writes only on --confirm; appends audit_notes. Refuses a comment-only review_pending (nothing to diff).",
          },
          {
            from: "review_pending",
            to: "locked",
            trigger: "dossierx comment resolve <id> <tid>",
            note: "Resolving (or deleting) the last open thread clears review_pending — but only when no dependency drift or flag also stands, else the flag is retained with a printed reason.",
          },
          {
            from: "locked",
            to: "draft",
            trigger: "dossierx unlock <id>",
            note: "Manual escape hatch, and a third clearer: returning a claim to draft drops review_pending. No lint gate — you may need to unlock precisely to fix what lint complains about.",
          },
        ],
        invariant:
          "A locked claim's Status never reverts to draft on its own. review_pending has three triggers (dependency drift, dossierx flag, an open comment thread) and three clearers (reaudit --confirm, resolving the last open thread, unlock); a locked claim is one-directional back to trusted until one of those clearers runs.",
      },
    },
    {
      id: "comments",
      title: "Review with comments — resolve every thread before locking.",
      kind: "comments-workflow",
      contentMd:
        "Comments are engine-managed review discussion attached to a claim — the same idea as `audit_notes`, but a conversation. A reviewer opens a thread on a claim's card; a human and an agent talk it through; the thread is resolved. Until it is, the claim **cannot lock**, and an open thread on an already-locked claim flips it to `review_pending`.\n\nThe write path is `dossierx serve`: a localhost-only viewer with a live thread panel and composer. Agents mutate through the CLI, so the shared claims lock keeps a live reviewer and a working agent from clobbering each other. Bodies live in the claim YAML, excluded from its content hash, rendered through the same safe markdown the viewer already uses.",
      data: {
        roles: [
          { id: "human", label: "human" },
          { id: "agent", label: "agent" },
          { id: "engine", label: "engine" },
        ],
        workflow: [
          {
            role: "human",
            title: "Open a thread",
            body: "A reviewer reads the claim in dossierx serve and opens a comment on its card: “This row contradicts the API facet — which is right?”",
          },
          {
            role: "agent",
            title: "Reply and fix",
            body: "The agent investigates, edits the claim, and replies “Fixed the rows; API facet was stale.” It never resolves a human’s thread — it addresses and asks for confirmation.",
          },
          {
            role: "human",
            title: "Resolve",
            body: "Satisfied, the reviewer resolves the thread. Advisory rights: a human-opened thread is resolved only by a human; an agent-opened one by either.",
          },
          {
            role: "engine",
            title: "Lock is gated",
            body: "dossierx lock refuses any claim that still has an open thread and names the ids — the gate that makes “resolve before locking” a real rule, not a convention.",
          },
          {
            role: "engine",
            title: "review_pending on a locked claim",
            body: "Open a thread on an already-locked claim and it flips to review_pending — a legal, long-lived state, surfaced by dossierx stale and dossierx check next-steps.",
          },
          {
            role: "engine",
            title: "Resolve to clear",
            body: "Resolving (or deleting) the last open thread clears review_pending — unless dependency drift or a flag also stands, in which case it prints why the flag is retained.",
          },
        ],
        card: {
          id: "logger.internals.event-envelope-fields",
          module: "logger",
          facet: "internals",
          status: "draft",
          panelTitle: "Comments",
          thread: {
            id: "c-8f3a2b",
            role: "human",
            status: "open",
            created: "2026-07-24 · 10:12",
            body: "This row contradicts the API facet — which is right?",
            replies: [
              {
                id: "r-4c9e11",
                role: "agent",
                created: "2026-07-24 · 10:40",
                body: "Fixed the rows; API facet was stale.",
              },
            ],
          },
          resolvedCount: 1,
        },
        terminal: {
          lang: "bash",
          code: '$ dossierx serve\nserving: http://127.0.0.1:52431/\n\n# in another shell: the open threads an agent must clear before it can lock\n$ dossierx comment list logger.internals.event-envelope-fields --open\nc-8f3a2b open human 2026-07-24T10:12:00Z replies=1: This row contradicts the API facet — which is right?\n\n$ dossierx lock logger.internals.event-envelope-fields\nlock: refused, claim "logger.internals.event-envelope-fields" has 1 unresolved comment thread(s) [c-8f3a2b] — resolve them first, e.g. "dossierx comment resolve logger.internals.event-envelope-fields c-8f3a2b"',
        },
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
      title: "The CLI — 26 commands, zero hardcoded project.",
      kind: "cli-explorer",
      contentMd:
        "One binary serves any project through `project.config.yaml`, discovered from the working tree or supplied with `--config`. Use the explorer below for the full command surface; `check` is the CI entry point that detects drift, validates claims, renders the viewer, and verifies code links, while the `comment` group and `serve` add threaded review on top of it.",
      data: {
        groups: [
          {
            group: "Render pipeline",
            commands: [
              {
                name: "lint",
                usage: "dossierx lint [--json]",
                summary: "Run all 22 lints in isolation and across the set.",
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
                  "The confirm-before-write gate for a drift- or flag-triggered review_pending claim.",
                detail:
                  "Valid on a locked claim whose review_pending came from dependency drift or a flag (else exit 2). A review_pending that is only from an open comment thread is refused — a comment carries no proposed content change to diff, so reaudit points you at dossierx comment resolve instead. Prints the proposed diff and stops unless --confirm. Two sources converge: a flagged claim gets a real before/after diff (ProposeFlagDiff); a drifted one gets a dependency-drift stub (ProposeDiff). On confirm: applies, appends audit_notes, re-baselines hashes, clears the flag — but leaves review_pending set if an open thread still stands.",
                example:
                  '$ dossierx reaudit logger.internals.dispatch --confirm\nreaudit: applied, review_pending cleared\n$ dossierx reaudit logger.contract.api-surface\nreaudit: claim "logger.contract.api-surface" is review_pending only because of 1 open comment thread(s); resolve them with "dossierx comment resolve ..." — nothing to reaudit',
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
            group: "comments & serve",
            commands: [
              {
                name: "serve",
                usage: "dossierx serve [--port <n>]",
                summary:
                  "Serve the claims viewer with a localhost-only comment write-back API.",
                detail:
                  "Binds 127.0.0.1 on a random high port (override with --port), renders the viewer from memory, and exposes the same comment operations as the CLI over a same-origin JSON API — so a reviewer opens, replies to, and resolves threads in the browser while an agent works from the CLI. Every request passes Host + Origin admission checks (DNS-rebinding and CSRF defense) and no CORS header is ever sent; the page live-reloads over server-sent events as claim files change on disk. It renders to memory only — never writing viewer/index.html or .catalog.json on a page load — and every claim write goes through the one claims-locked code path, so browser and CLI never clobber each other.",
                example:
                  "$ dossierx serve\nserving: http://127.0.0.1:52431/",
              },
              {
                name: "comment add",
                usage:
                  'dossierx comment add <claim-id> --as human|agent --body "…"',
                summary: "Open a new comment thread on a claim.",
                detail:
                  "Mints an engine-generated thread id and echoes it so you can chain the next verb. --as records the author role (human or agent), not an identity. A draft claim with an open thread cannot lock; opening a thread on a locked claim sets review_pending. Refused on banner-layout claims (they render no viewer footer to hold a thread).",
                example:
                  '$ dossierx comment add logger.internals.event-envelope-fields --as human --body "This row contradicts the API facet"\ncomment: c-8f3a2b added on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment reply",
                usage:
                  'dossierx comment reply <claim-id> <thread-id> --as human|agent --body "…"',
                summary: "Reply to an open thread.",
                detail:
                  "Addressed by the engine-generated reply id it echoes, never by ordinal position. Replying to an already-resolved thread is refused.",
                example:
                  '$ dossierx comment reply logger.internals.event-envelope-fields c-8f3a2b --as agent --body "Fixed the rows; API facet was stale."\ncomment: reply r-4c9e11 added to thread c-8f3a2b on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment resolve",
                usage:
                  "dossierx comment resolve <claim-id> <thread-id> --as human|agent",
                summary: "Mark a thread resolved (advisory rights apply).",
                detail:
                  'A human-opened thread can be resolved only with --as human; an agent-opened one by either. Resolving the last open thread clears review_pending — but only if no dependency drift or flag also stands, otherwise it prints why the flag is retained. An agent should reply "addressed, please confirm" rather than resolve a human thread.',
                example:
                  '$ dossierx comment resolve logger.internals.event-envelope-fields c-8f3a2b --as human\ncomment: thread c-8f3a2b resolved on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment reopen",
                usage:
                  "dossierx comment reopen <claim-id> <thread-id> --as human|agent",
                summary: "Reopen a resolved thread.",
                detail:
                  "Re-sets review_pending on a locked claim. Same advisory rights as resolve — an agent may not reopen a human-resolved human thread.",
                example:
                  '$ dossierx comment reopen logger.internals.event-envelope-fields c-8f3a2b --as human\ncomment: thread c-8f3a2b reopened on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment edit",
                usage:
                  'dossierx comment edit <claim-id> <thread-id> [--reply <reply-id>] --as … --body "…"',
                summary: "Edit a thread root's body, or a reply's with --reply.",
                detail:
                  'Rights key off the author of the edited message (own-role only). Sets edited: true, which the viewer shows as an "(edited)" marker. --reply targets one reply by id; without it, the thread root.',
                example:
                  '$ dossierx comment edit logger.internals.event-envelope-fields c-8f3a2b --as human --body "This row contradicts the API facet — which wins?"\ncomment: thread c-8f3a2b edited on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment delete",
                usage:
                  "dossierx comment delete <claim-id> <thread-id> [--reply <reply-id>] --as …",
                summary: "Delete a whole thread, or one reply with --reply.",
                detail:
                  "Own-role only. Deleting the last open thread clears review_pending under the same no-other-trigger rule as resolve.",
                example:
                  '$ dossierx comment delete logger.internals.event-envelope-fields c-8f3a2b --as human\ncomment: thread c-8f3a2b deleted on logger.internals.event-envelope-fields; run "dossierx check" or "dossierx serve" to view',
              },
              {
                name: "comment list",
                usage: "dossierx comment list <claim-id> [--open] [--json]",
                summary: "List a claim's threads, one greppable line each.",
                detail:
                  "Pinned one-line-per-thread format: <thread-id> <status> <author> <created> replies=<N>: <first-line-of-body> — a stable contract the skill, this site, and a golden test all reproduce. --open filters to unresolved threads; --json emits a top-level array (stdout stays pure JSON, the human hint goes to stderr).",
                example:
                  "$ dossierx comment list logger.internals.event-envelope-fields --open\nc-8f3a2b open human 2026-07-24T10:12:00Z replies=1: This row contradicts the API facet",
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
              {
                name: "version",
                usage: "dossierx version",
                summary: "Print the binary's version, commit, and build date.",
                detail:
                  "Describes the binary itself, so unlike every other command it never loads a project config and runs from anywhere. The root command also exposes the equivalent built-in --version flag. Values are ldflag-stamped at release, with a debug.ReadBuildInfo fallback for plain go install builds.",
                example:
                  "$ dossierx version\ndossierx v0.2.0\n  commit: e5c8ab1\n  date:   2026-07-25",
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
                  "Write the binary's four embedded Claude Code skills to <dir>.",
                detail:
                  "Walks the embedded skills/ FS (dossierx-claims, dossierx-build-order, dossierx-code-links, dossierx-comments), preserves layout, creates parent dirs, overwrites existing files. Projects that exported the old three bundles must re-export to pick up dossierx-comments.",
                example:
                  "$ dossierx skills export ./.claude/skills\nskills export: wrote 4 file(s) to ./.claude/skills",
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
        "Every change ships as a tagged release with its own changelog entry. The current release is **v0.2.0**, which adds threaded review comments and `dossierx serve` — see the full history below for what's shipped since the engine's public extraction.",
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
            tag: "Previous release",
            highlights: [
              "Every generic 'docs' placeholder — CLI-invocation examples, the docs-claim: source tag (including the real Go regex), docs-v1 naming in skill docs, the default viewer title, and the on-disk store filenames — is renamed to the tool's actual name, dossierx.",
              "BREAKING: .docs-lock-store.json and .docs-flag-store.json are renamed to .dossierx-lock-store.json and .dossierx-flag-store.json, with no migration. An existing project's lock/flag store will not be found after upgrading past this release.",
              "Minor version bump (v0.0.3 → v0.1.0), not a patch, to signal the breaking on-disk change under pre-1.0 semver.",
            ],
          },
          {
            version: "v0.1.1",
            date: "2026-07-24",
            title: "Steps number/text alignment fix",
            tag: "Previous release",
            highlights: [
              "Fixes a rendering bug in the default viewer's layout: steps claims: because each step's text is routed through the shared markdown renderer, it was wrapped in a <p> whose default browser top margin pushed the first line down relative to the step's fixed-size number circle, leaving the number visibly misaligned above the text.",
              "The viewer stylesheet now resets the step body's <p> margins — a 0.5rem inter-block rhythm with the first block's top and last block's bottom margin zeroed — so the number circle sits flush with the first line of step text.",
              "Purely a default-viewer CSS fix: no schema_version, on-disk store, API, or config change, so it lands as a patch bump (v0.1.0 → v0.1.1) under pre-1.0 semver.",
            ],
          },
          {
            version: "v0.1.2",
            date: "2026-07-25",
            title: "Consolidated audit-fix release",
            tag: "Previous release",
            highlights: [
              "A deep audit against a real 202-claim consumer project found 25 confirmed defects, fixed together as one patch rather than a stream of point releases.",
              "Markdown [text](url) links now render as anchors in claim bodies AND structured table cells (scheme-allowlisted; javascript:/data: neutralized); backtick code spans render in cells too.",
              "Lifecycle data integrity: dependency-hash baselines are keyed per-dependent (with an automatic, re-arming lock-store migration), unlock clears pending flags, and flag is refused on structured layouts.",
              "Security: the raw_html mockup gate is now default-deny across every attribute quote form (control-char and entity evasions closed) and enforced by render and catalog, not just lock.",
              "Build-order staleness is now structural — status re-derives the order a fresh propose would compute and flags any divergence (reordered, re-roled, promoted, renamed, added, or deleted claims); the Build Order viewer section is visible; new dossierx version command; lint --json emits valid arrays.",
              "Patch bump despite new capabilities: internal/ is not importable, no breaking CLI changes, and the lock-store migrates automatically.",
            ],
          },
          {
            version: "v0.2.0",
            date: "2026-07-25",
            title: "Comments on claims — review, discuss, resolve before locking",
            tag: "Latest release",
            highlights: [
              "Threaded, Google-Docs-style review comments attach to any claim: a new dossierx comment group (add / reply / resolve / reopen / edit / delete / list) with an --as human|agent actor, engine-minted thread and reply ids, and a pinned, greppable list format.",
              "An open comment thread is a third review_pending trigger alongside dependency drift and dossierx flag: a claim cannot lock while it has an unresolved thread (dossierx build-order propose refuses such a module too), and resolving the last open thread clears review_pending unless drift or a flag also stands. reaudit refuses a comment-only review_pending — there is no content diff to confirm.",
              "New dossierx serve: a localhost-only HTTP viewer with an interactive thread panel and composer, a same-origin admission layer (Host/Origin checks, no CORS), and live reload over server-sent events as claim files change on disk. It renders from memory and never writes viewer/index.html or .catalog.json on a page load.",
              "Adds NO new runtime dependency — the file watcher is a standard-library modification-time poll, so the runtime stays cobra + yaml.v3 only.",
              "A fourth embedded skill, dossierx-comments, teaches agents when to comment vs. flag and the advisory-rights rule. The comments: field is omitempty and excluded from a claim's content hash, so commenting never rewrites an uncommented claim or flips its dependents. Minor, backward-compatible bump — dossierx skills export now writes four bundles, so re-export to pick up the new skill.",
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
            property: "Review comments that gate trust",
            wiki: "comments exist, but never block anything",
            adr: "review is out-of-band; nothing enforces it",
            markdown: "PR comments on the diff, detached from the doc",
            dossierx: "threaded per claim; an unresolved thread blocks the lock",
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
