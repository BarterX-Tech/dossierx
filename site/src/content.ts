// Content spec for the DossierX static site.
//
// This is the single source of truth for on-page copy, typed so sections can
// import and render it directly.
//
// v0.3.0 RE-PREMISED THIS FILE. Every earlier draft read as if a person ran the
// pipeline: "run dossierx lint, then lock the claim, then render the viewer."
// That was always a half-truth and is now simply wrong. An AGENT operates
// DossierX — all nineteen commands, JSON by default — and a HUMAN reviews what
// it did, in a browser, with two gestures: comment and Resolve. Copy that blurs
// the two teaches the wrong mental model before a single command appears, so
// the roles are named in the hero, given their own section ("Who runs what")
// before any command is shown, and carried through every section after.
//
// Consequently: no command name on this page is a bare verb any more. It is
// `dossierx claim lock`, never `dossierx lock`; `claim list --review-pending`,
// never `dossierx stale`. The ten verbs v0.3.0 deleted, and the `migrate`
// command v0.4.0 removed, appear in the live copy in exactly one place — the
// migration table in the CLI section — and nowhere else. The `releases` array is
// release HISTORY and is exempt: the v0.3.0 entry names `migrate --adopt`
// because v0.3.0 shipped it.

import type { Release } from "./components/ReleaseTimeline";

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
  | "two-surface"
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

/**
 * The release ledger, oldest-first. ReleaseTimeline treats the LAST entry as
 * the current release, so a new release is appended, never prepended.
 *
 * Hoisted above `contentSpec` so the version can be derived from it rather
 * than hand-copied: the hero kicker, the hero badge list, the release-history
 * intro and the `dossierx version` example all read `latestRelease` below.
 * Every one of them had gone stale against this array at least once.
 *
 * NO ENTRY CARRIES A `commit`, and none may be given one. The field held the
 * tagged release's short sha and could not converge: writing the sha is itself a
 * commit, so the value was stale the moment it landed. v0.4.1 shipped naming
 * `5327923` while `refs/tags/v0.4.1` pointed at `206b4a4`, and v0.5.0 chased its
 * own sha through two commits before anything caught the loop. It also disagreed
 * with the binary by construction — GoReleaser stamps `main.commit` from
 * `{{.Commit}}`, the full forty characters, against seven here. Nothing on the
 * site reads it now, so re-adding one would be adding a claim with no reader and
 * no way to be right.
 */
const releases: Release[] = [
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
          date: "2026-07-26",
          title: "Comments on claims — review, discuss, resolve before locking",
          tag: "Previous release",
          highlights: [
            "Threaded, Google-Docs-style review comments attach to any claim: a new dossierx comment group with an --as human|agent actor, engine-minted thread and reply ids, and a pinned, greppable list format.",
            "An open comment thread is a third review_pending trigger alongside dependency drift and a flag: a claim cannot lock while it has an unresolved thread, and resolving the last open thread clears review_pending unless drift or a flag also stands. reaudit refuses a comment-only review_pending — there is no content diff to confirm.",
            "New dossierx serve: a localhost-only HTTP viewer with an interactive thread panel and composer, a same-origin admission layer (Host/Origin checks, no CORS), and live reload over server-sent events as claim files change on disk. It renders from memory and never writes viewer/index.html or .catalog.json on a page load.",
            "Adds NO new runtime dependency — the file watcher is a standard-library modification-time poll, so the runtime stays cobra + yaml.v3 only.",
            "A fourth embedded skill teaches agents when to comment vs. flag and the advisory-rights rule. The comments: field is omitempty and excluded from a claim's content hash, so commenting never rewrites an uncommented claim or flips its dependents.",
          ],
        },
        {
          version: "v0.3.0",
          date: "2026-07-28",
          title: "Agent-first restructure — two surfaces, one gate",
          tag: "Previous release",
          highlights: [
            "The premise, made explicit: the AGENT operates DossierX and the HUMAN reviews it. Each role gets its own surface — twenty CLI commands for the agent, the viewer's comment-and-Resolve for the human — and the docs, skills and site are rewritten around that split.",
            "MACHINE CONTRACT: every command emits exactly one JSON envelope ({ok, command, data, warnings, error, stopped_at}), JSON by default, with a shared snake_case error.code vocabulary that the viewer's JSON API and the CLI now literally share. --dry-run on every mutating verb; --reason required on claim lock, claim unlock, claim reaudit --confirm and build-order lock. Exit codes stayed additive: 1 generic, 2 not-found/wrong-state.",
            "SURFACE: 26 commands became 20 under eight nouns — check, claim (show/list/new/lock/unlock/flag/reaudit/link), comment (inbox/list/add/reply), build-order (propose/status/lock), migrate --adopt, serve, skills export, version. lint, catalog, render, deps, stale, coverage and the implink group are gone (stages and filters, not verbs); comment edit/delete/resolve/reopen are gone from the CLI and live only in the viewer, where the rights holder is. New: claim show (3–4 calls → 1), claim list with filters, claim new, and comment inbox (O(N) calls → 1).",
            "INTEGRITY: a lock ledger, separate from the content hash and built as a deny-list over every persisted field — so a swapped raw_html payload on a locked, allowlisted mockup can no longer pass as unchanged. Hand-flipping status, editing a locked body, re-locking onto a record an unlock already released, deleting a locked claim's file outright, deleting a thread from YAML, or hand-editing a locked build order is now reported as lock-ledger-missing, lock-content-drift, lock-ledger-orphan, lock-ledger-released, lock-ledger-abandoned, comment-ledger-drift, build-order-content-drift or build-order-ledger-missing. The gate rides git: a pre-commit hook running check --staged over the index, with CI as the authority, because merges and rebases never fire pre-commit.",
            "BREAKING, and it breaks on your first check: every project that locked a claim before v0.3.0 must run `dossierx migrate --adopt` once, then commit the rewritten .dossierx-lock-store.json. Pre-ledger projects used to be grandfathered in automatically on the first writing check; that is gone. Adoption is the one operation that manufactures approval out of nothing, so a gate that performs it on sight rewards deleting the ledger with a clean report over content nobody read — and no evidence inside a project distinguishes an honest v0.2.x store from a downgraded one, since locked_at shipped in v0.2.0. Adoption therefore fails closed everywhere, and only a human running the migration clears it.",
            "THE MODEL, stated plainly because the boundary matters: DossierX DETECTS and the forge ENFORCES. Every rule produces a named, recoverable finding — a stable rule string, the claim it is about, and the command that restores it — and branch protection with a required CI check is what makes anyone obey it. check --staged judges the GIT INDEX, what the commit will actually contain, writing nothing, which is what makes a pre-commit hook meaningful; it judges ONE TREE and reads no git history, so its verdict is identical in every clone however shallow.",
            "The boundary, stated as a principle rather than a list of cases: AN IN-REPO LEDGER CANNOT ATTEST ANYTHING AGAINST THE PERSON WHO CAN WRITE IT. The gate judges one tree, and within it a change to a locked claim's APPROVED CONTENT leaves a surviving artifact DISAGREEING with the one that moved, and is caught (review_pending is engine bookkeeping, outside the hash, so deleting it by hand clears a review flag silently) — an agent editing a locked claim, a careless hand-edit, a bad merge, a status flip, a deleted approval record, an erased comment thread all leave the untouched files disagreeing with the touched one, and that disagreement is a named finding. What has nothing to disagree with it is not: a record's reason, at and actor are testimony rather than signature, so rewriting those alone changes what the ledger says a human approved and is reported by nothing. What it cannot catch is a claim and its ledger record written TOGETHER, in one commit, by someone entitled to write both: nothing survives to disagree. The sharpest form involves no deletion at all — unlock a locked claim, rewrite its body, re-lock it (minting a fresh record whose hash correctly covers the new content), then hand-edit that record's reason, at and actor back to the original approval's values; check and check --staged both report ok:true over a ledger crediting a human who approved nothing. That is one illustration, not an inventory. No in-repo mechanism closes it, because any evidence the tool consults lives in the repository and the repository is writable by the person being gated — a build during this release cycle compared the staged tree against its parent commit and was withdrawn before shipping, since the parent commit is outside the COMMIT but not outside the COMMITTER (a rebase, --orphan or a second config file switches such a comparison off unremarkably) and it cost false refusals of ordinary git revert. It is caught where a control the committer cannot rewrite lives: branch protection, a required CI status check, and review of a diff in which such a change is loud. The direction that would actually move the boundary is signing the ledger with a key held outside the repository, of which git commit signing is the cheapest form.",

            "BUG FIX, and the reason the loop works at all: a claim with no comments rendered no 💬 chip, so the first comment on any card could never be opened from the viewer — the exact gesture the review loop starts with. Both gates had to move together, since the server emitted the chip only when threads existed and the client hid any chip reading zero, so an empty chip would have vanished the moment it was clicked. Every non-banner card now carries a chip, zero-state included; a static file:// export still hides them, because with no API to answer, read-only is the honest presentation.",
            "Also: self-edges rejected across every edge type and a second cycle pass over governed_by; edge labels in the viewer read as prose instead of raw ids (issue #11); five embedded skills, contract front-loaded, rewritten onto the new surface. Still cobra + yaml.v3 only.",
          ],
        },
        {
          version: "v0.3.1",
          date: "2026-07-30",
          title: "A documentation-grade renderer",
          tag: "Previous release",
          highlights: [
            "A claim body is now somewhere you can write real documentation. Numbered steps with commands under them, tables, diagrams, quotes and sub-headings all render as written — previously a fenced code block under a numbered step split the list in two and restarted the numbering at 1.",
            "Constructs: backslash escapes, bold, italic in both spellings under CommonMark left/right-flanking rules, strikethrough, double-backtick code spans, autolinks in both forms, blockquotes, horizontal rules, headings at ### and deeper, unbounded list nesting, task items, hard line breaks, loose lists, ordered-list start numbers, fence language classes, GFM pipe tables, and images. Emphasis is held to all 132 emphasis examples of CommonMark 0.31.2.",
            "Images render in claim bodies and never in comment bodies, fail-closed by construction rather than by a flag: markdown.Render emits none and cannot be told to, so the comment paths stay image-free by changing nothing. A src must resolve under its own claim's assets/ directory, and dossierx serve answers for them from an allowlist computed from the loaded claims rather than from filesystem traversal.",
            "SECURITY: parseLink's bracket rescan was quadratic — 1 MiB of bracket characters in a comment body cost 5.8 seconds of CPU, amplified because every comment is re-rendered on every GET /api/comments. This predates v0.3.1 and is live in the v0.3.0 binary.",
            "SECURITY: the mockup <img> gate recognised // as an authority prefix but not the three backslash spellings a browser normalises to it, so a reviewed, locked, allowlisted mockup claim could load a third-party image. The gate now lives in one shared internal/urlsafe instead of four divergent copies.",
            "Two new lints — markdown-sanity and asset-scope — with a deliberate severity split: error for security findings, warning for craft ones, because lock refuses on any error-severity finding corpus-wide.",
            "Upgrading: locked claim bodies may render differently with no edit, no content-hash change and no ledger event. dossierx claim unlock is the deliberate way to revisit them.",
          ],
        },
        {
          version: "v0.4.0",
          date: "2026-08-03",
          title: "Migration removed, and governed_by becomes a drift edge",
          tag: "Previous release",
          highlights: [
            "BREAKING: `dossierx migrate` is gone, and with it every automatic adoption path. A project whose lock store predates the lock ledger crosses by holding nothing locked: re-propose any locked build order, unlock every locked claim, then re-lock only what you still stand behind. That first lock in a project with nothing locked is what stamps the store onto the ledger schema — and it records a real approval, which is the whole difference from the adoption path this release deletes. `migrate` survives only as a hidden stub that names the new path, because README, the skills and the CI template spent a release telling agents to type it, and flag parsing runs before any unknown-command handler.",
            "BREAKING, on the wire: error.code `adoption_required` is renamed `pre_ledger_unadopted`; `already_migrated` is removed outright; the finding `lock-ledger-adoption-required` is renamed `lock-ledger-pre-ledger`. Nothing is grandfathered any more, but `claim show` keeps `ledger.grandfathered` — always false for new records.",
            "The pre-ledger finding is now CONDITIONAL: it is silent unless the project actually holds a locked claim or a locked build order. A pre-ledger project with nothing locked crosses silently and correctly on its next lock, and a finding on correct state is how gates get switched off.",
            "SURFACE: eight nouns and twenty leaves become SEVEN nouns and NINETEEN leaves — check, claim (show/list/new/lock/unlock/flag/reaudit/link), comment (inbox/list/add/reply), build-order (propose/status/lock), serve, skills export, version.",
            "SILENT: `governed_by` joins mirrors and rests_on as a DRIFT edge — a claim-valued governor whose content changes under a locked claim now flags that claim review_pending. It is NOT a gating edge; hub gating is unchanged. There is no backfill, so a claim locked before this release carries no governance baseline until its next lock or reaudit, and the first governor edit after upgrading does not flag it.",
            "SILENT: claim writes now merge only the top-level keys that changed onto the existing document, so adding one comment lands as a one-key diff instead of a whole-file rewrite and every untouched key keeps its authored quoting, key order and YAML comments. Block-scalar INDENT WIDTH is deliberately not preserved — the merged tree is re-emitted at two-space indent, which is what makes the bytes safe to hand to the round-trip guard — so the first write to a claim after upgrading may reindent its block scalars.",
            "Two comment-body shapes that used to be refused as unsafe now store cleanly: a space-indented first content line, and a two-space-indented multiline body. A TAB-led first content line is still unstorable at any indent width, so the refusal class survives — it is now tab-led only.",
            "A `layout: table` claim wider than its column now sits in a container that scrolls on its own axis, and long cell content wraps instead of forcing the track wide, so the page body never scrolls sideways. Column proportions shift by roughly 12% for ordinary content. Markdown tables inside a claim BODY still scroll rather than wrap — deliberately deferred.",
            "The markdown-sanity lint no longer fires on a closing delimiter run followed by punctuation: \"Only ~~strike~~, comma after.\" was a finding and is not one. The scanner now applies CommonMark's punctuation clause per rune rather than per byte, which is what multi-byte punctuation requires, and the whole testdata/markdown-cases corpus is a regression gate rather than a set of hand-picked strings.",
          ],
        },
        {
          version: "v0.4.1",
          date: "2026-08-04",
          title: "raw_html becomes an attachment, and the edges footer folds away",
          tag: "Previous release",
          highlights: [
            "`raw_html` is an ATTACHMENT legal on any layout, not a layout a claim has to adopt (closes issue #25). A claim that is genuinely a table, or a list of steps, can now carry a diagram or a small rendered mockup alongside its own content — all seven layouts render the attachment after the claim's own body/rows/steps and before the edges footer. `layout: mockup` remains a real layout with its own empty state; it simply stops being the toll gate you had to pass through to show anything rendered.",
            "That widens WHERE unescaped HTML may sit, and nothing else — not who may author it, not what reaches the viewer. Every other leg of the gate still fires on every raw_html-bearing claim whatever its layout: the mockup_modules allowlist, the tag/attribute/class markup allowlist, the raw_html_reviewed human-review flag, and the lock-lifecycle check. `dossierx claim flag`'s body-only rule moved with it — it now keys on whether a claim actually carries raw_html rather than on its layout, which was only ever a sound proxy while the two were the same question.",
            "The viewer's edges footer is now a disclosure you open, not a block you scroll past. rests_on, mirrors, governed_by, depended-on-by, migrated_from, implemented-in and review_pending fold into a native <details> whose summary is a digest — \"4 links - 2 files - 1 drifted\", with the drifted segment shown only when it is non-zero. It opens by itself, server-side, when a linked file has drifted or the claim is locked and review_pending; a deep link to the claim or a print forces it open in CSS, since neither is knowable at render time. `governed_by: none` no longer counts toward the link total because it states an absence rather than a followable edge, and a claim with no links and no linked files emits no disclosure at all. One toolbar toggle opens or closes every footer on the page.",
            "The 💬 chip left that footer and moved into each claim's head, which is where the review loop actually starts. A claim with no threads yet reveals its chip on card hover or keyboard focus rather than sitting inside a collapsed footer — never display:none, so it stays in the tab order. `layout: banner` is the one partial that renders neither a chip nor a footer, and neither half of this touches it.",
            "SILENT: an already-locked, byte-identical claim renders differently. Every layout that carries a chip or a footer emits different HTML from the same locked bytes — no edit, no content-hash move, no review_pending flip, nothing in the lock store and nothing for `dossierx check` to report. Same shape as v0.4.0's table fix and v0.3.1's renderer: re-run the render to pick it up, and `dossierx claim unlock` is the deliberate way to revisit a claim whose rendered output you now want to read again.",
            "SILENT: a locked claim that ALREADY carries raw_html re-hashes once against its dependents. The content hash — the rests_on/mirrors/governed_by drift baseline — now folds in raw_html when it is non-empty, so editing the attachment this release newly allows on a rule-bearing claim is no longer invisible to the claims resting on it. Gated, not additive: a claim with no raw_html, which is every claim before this release and most after it, feeds the hash byte-identical input and moves nothing. The one case that does move is a pre-existing allowlisted, reviewed mockup claim named by another locked claim's edges — that dependent flips review_pending once, with no edit and no ledger event.",
            "No migration, and a patch bump: no schema_version, on-disk store, config or CLI change. The raw_html widening is a relaxation of where a gate applies rather than a break in it, and a review_pending flip caused only by the hash widening clears the ordinary way — `dossierx claim reaudit --confirm`, or unlock, fix, lock.",
          ],
        },
        {
          version: "v0.5.0",
          date: "2026-08-07",
          title: "a claims graph in the viewer",
          tag: "Previous release",
          highlights: [
            "BREAKING: `dossierx check` now fails on a dependency loop that alternates `rests_on` and `governed_by`. The new `mixed-cycle` lint runs at ERROR severity, taking the registered rule count from 27 to 28. It walks the union of both graphs carrying the edge kind on every hop and reports a cycle whose hops include at least one of each — \"A rests_on B, B governed_by A\". Neither existing rule can see that shape: `cycle` walks `rests_on` alone and `governed-cycle` walks `governed_by` alone, so a mixed loop presents no back edge to either walk and passed the whole registry. A project carrying one passed before this release and exits 1 after it, with no edit on its side, no content-hash move and nothing in the lock store to explain it.",
            "There is deliberately no migration command and no migration document: a corpus containing this shape was always malformed, the engine simply could not see it. The recovery is to break the loop — the finding names every claim on it — and re-run `dossierx check`. Where those claims are locked that is unlock, edit, lock, the same as any other correction. `mirrors` is not part of the union graph and never trips the rule.",
            "The rendered viewer now carries a \"Claims graph\" pane: a canvas view of the corpus's `rests_on`, `governed_by` and `mirrors` edges, with selectable overlays (isolated & weakly linked, dependency cycles, governance, review pending, open comment threads, draft vs locked), a per-claim detail panel that includes an `in a cycle` row, granularity collapse to modules or facets, zoom, pan and drag. Above 300 claims the pane opens at module granularity rather than drawing every claim — the threshold is a legibility limit, not a performance one: layout stays under 5ms at 3000 nodes, but readable labels fall from 12 at 300 to 0 at 880.",
            "It is built by a new `internal/graph` package as a JSON payload inlined into the single self-contained `index.html`, alongside three new embedded client files. No external assets, so it works over `file://`. There is no new CLI noun and no new schema field. The pane is a third full-viewport overlay whose body scroll lock is additive with the sidebar drawer's and the comment panel's rather than mutually exclusive with them.",
            "`dossierx serve` gains `GET /api/graph`, backing a refresh button that rebuilds and re-stamps the payload from the current catalog at request time rather than reusing the render's. The button is absent over `file://`, where there is nothing to refresh from.",
            "`testdata/fixture-graph-demo` is a third committed sample viewer — a 58-claim fixture that is itself a ledger-covered dossierx project, so its comment digest is a tracked input alongside its lock store. `tests/fixture_staleness_test.go` now fails the build when a committed sample viewer no longer matches what the current renderer produces, instead of leaving a stale artifact to be noticed at release time.",
            "Not in this release, and stated so deliberately: any code-grounding signal. The graph audits claims, not code. There is no `has_code_link` field, no \"locked, ungrounded\" rule and no `implink` argument.",
          ],
        },
        {
          version: "v0.5.1",
          date: "2026-08-10",
          title: "The release pipeline itself, gated end to end",
          tag: "Previous release",
          highlights: [
            "SILENT: the embedded agent skills changed, and nothing on your side reports it — RE-RUN `dossierx skills export` after upgrading. Those bundles are written into a project as committed artifacts and nothing in `dossierx check` compares an exported copy against the binary's, so a project that skips the re-export keeps the previous release's guidance: an install line fetching the hook script from the v0.5.0 raw path, and a stale account of when `dossierx claim flag` is refused.",
            "Nothing a consumer runs behaves differently. No new or changed command, flag, error code, lint rule, schema field or rendered-viewer byte, and no engine behaviour moves — `internal/` is untouched end to end and there is nothing to re-render. Outside the release machinery what moved is the four install pins, now v0.5.1, one of them inside the exported skill itself; and seven wrong strings in what a consumer's own tooling prints or ships. Three came from the binary, each naming an invocation it rejects: the retired `implink` stub's replacement command, the missing-`--reason` refusal, and `claim show`'s next action for a drifted implementation link, which offered a bare `claim flag <id>` at a verb that requires `--claim-says`, `--now-does` and `--reason`, all three, before it does anything at all. Each was a shape the reader had to repair before it would run, and each now prints whole. Two are carried by the exported skill bundles: the cross-references between bundles were written as `[[wikilink]]`, which the two derived export forms rewrote into an anchor and the SKILL.md tree — the form Claude Code loads — shipped to a client's agent as the literal characters `[[` and `]]`; and the code-links bundle's `claim flag` rule still stated v0.4.0's layout test after v0.4.1 made the refusal key on content. Four non-test Go files move for those five, and none of them changes anything but the text a reader is handed. The last two are in the pre-commit hook: the installer's note attributed core.hooksPath to \"this repository\" when the value is read across every git config scope and is just as often global, and the hook body's \"remove the hook\" recovery named a path inside THIS repository, which the consumer fetching one file into their own project does not have. The note now says where to look up the setting's origin, and the recovery names the hook where git actually looks for it; the hook body goes to v7 with it.",
            "What ships otherwise is the machinery that publishes the NEXT release, because everything that has ever gone wrong with a DossierX release has been in the half a person performs by hand.",
            "`surfaces.yaml` declares all thirteen client-facing surfaces this project has, from the README to the compiled binary, beside seven declarations of what is deliberately out of scope and why. Every tracked file must be claimed by EXACTLY one entry — a file matching nothing fails the build, and an out-of-scope entry cannot quietly swallow a path a surface also claims, because both entries are named and the build goes red. The list of things to review used to live in a scope document, so a new client-facing file could appear with nothing to notice it and the only way to find the gap was an audit.",
            "At release each surface is read by its own agent over a bundle assembled for it, and the cache key is the digest of what that agent was ACTUALLY handed rather than the surface's name — change a byte of the evidence and the surface is re-read instead of carried forward. The tool grant is an exact set of two report-only tools, an allow-list and not a deny list: no file, shell, search, network or subagent tool, because \"the bundle is the whole evidence set\" is the property every key in the system rests on. What the repository cannot promise, and says so rather than implying otherwise, is that the harness outside it honoured the request.",
            "Publishing is a nine-step driver rather than a sequence of commands somebody types, authorized by the version typed twice — deliberately not a boolean, since a `=1` left in a shell profile or a CI secret authorizes every release forever, including the next one triggered by accident. Before it touches git it refuses a release already half-published, requires the TREE to declare the release being tagged (the changelog's newest heading and the site's last release entry must agree with each other and with what was typed), RECOMPUTES the gate in its own process rather than reading a verdict off a record, and demands the CI-run evidence for that exact commit. Then it merges, tags the named object, reads the tag back and re-checks the tree it points at, pushes the tag by value, verifies the published archives, and only then pushes main.",
            "The archives are read the way somebody downloading them reads them, between the two irreversible acts. It polls the forge until the assets exist — one second after a tag push the ordinary state is \"the tag is there and the assets are not\", and there is no second attempt once a tag is public — then refuses a release with no checksum file as UNCHECKABLE rather than reporting no mismatches, derives the expected archive names from the release config at the released commit instead of hard-coding six, compares every archive against its checksum line AND every line against an archive it actually read, and extracts and RUNS the host platform's binary. An archive can carry a correct name, a correct checksum and a stale binary while every metadata check passes over it.",
            "The forge now refuses an ungated tag. The publishing job runs behind a gate job that establishes two facts about the tagged TREE rather than about whoever pushed: the commit is reachable from origin/main, and the tree at that commit carries the release stamp for exactly this version. Every exit path that is not a pass is a refusal — there is no branch that reports \"could not check\" and exits 0. One residual is recorded rather than described as fixed: the workflow a tag runs is the one in the tagged tree, so only a forge-side tag protection rule can close that, and nothing inside a repository can be its own enforcement.",
            "A green badge is not the check and neither is a green check run: a conclusion reads `success` over zero tests, so a suite emptied by a run selector prints \"no tests to run\" for every package while the step, the job and the check run all conclude success over it. The release now adjudicates the per-test account the test binary itself emitted, per package and per matrix cell, for the merge commit being tagged. No conclusion is read as evidence anywhere; they are recorded for a human to look at and adjudicated by nothing. A release nobody gathered that evidence for is refused, not assumed.",
            "Two things a reader could see were wrong, and are fixed. The site's meta description had advertised a 20-command CLI since v0.3.0 — v0.4.0 cut the surface to seven nouns and nineteen leaves and the tag stayed wrong through two minor releases, because it is the one count on the page that nothing derives and the one string search engines quote. And the `dossierx version` transcript depicted the tag spelling where the published binary prints the stripped one; it now derives that string and is compared against a binary linked the way a release links one, read out of the RENDERED page. The hand-stamped `commit` field is deleted outright — data, reader, release step and its optional type declaration, which was the part that would have let the field come back silently.",
            "Not in this release, and stated so deliberately: nothing derives a finding's classification from the evidence behind it, and there is no override field on a receipt — a finding a human has judged non-blocking is cleared by fixing the tree or by deleting the finding by hand, and deleting it leaves an adjudicated finding indistinguishable from one nobody raised. Nor does anything here verify the deployed site, the workflow run or the CDN: those are the three checks the driver hands to a person at the end, and it says in those words that it examined none of them.",
          ],
        },
        {
          version: "v0.5.2",
          date: "2026-08-10",
          title: "The release that can release itself",
          tag: "Latest release",
          highlights: [
            "SILENT: the embedded agent skills changed, and nothing on your side reports it — RE-RUN `dossierx skills export` after upgrading. Those bundles are written into a project as committed artifacts and nothing in `dossierx check` compares an exported copy against the binary's, so a project that skips the re-export keeps v0.5.1's guidance: an install line fetching the hook script from the v0.5.1 raw path, and a build-order rule that has no answer for a declared contract.",
            "VISIBLE: `dossierx version` now prints `v0.5.2` from the published archive, where v0.5.1's archive printed `0.5.1`. The same release used to report two different version strings depending on how it was installed — the archive stamped GoReleaser's `{{.Version}}`, which strips the leading `v`, while `go install …@v0.5.1` applies no ldflags at all and falls back to the module proxy's copy of the tag, which keeps it. A scripted `dossierx version --format json | jq -r .data.version` compared against a `vX.Y.Z` tag therefore succeeded one way and failed the other. The stamp is now `{{.Tag}}` and both paths print the tag exactly as tagged, which is also what the git tag, the CHANGELOG heading and every string on this site already said. If your tooling compares that field against a bare `X.Y.Z`, this release is where it changes.",
            "The release pipeline could not complete unattended, and this is the release that fixes it. The driver pushes the tag first, verifies the six archives, and pushes main last — deliberately, so the site never announces a release whose archives do not exist. The forge's gate then required the tagged commit to be reachable from `origin/main`. Both guards are right alone and together they deadlock: the gate job fires on the TAG push, and asks there for a branch the driver pushes two steps later. It refuses, no archives build, the driver waits for archives that can never exist, main never moves. Nothing in that ring can move first, so no timeout resolves it. In v0.5.1 it polled twenty minutes and stopped with the tag public and nothing else done; a human finished that release by hand.",
            "SUPERSEDES the v0.5.1 entry below, which says the forge checks that the tagged commit is reachable from origin/main. It no longer does. The check is replaced by one the gate can make at tag-push time holding nothing but the tag: the tagged commit must be a MERGE. A real release always passes, because the driver merges with `--no-ff` and tags that merge by value. What it still refuses is the ordinary mistake the old check was written for — tagging the release branch instead of the merge. What it no longer refuses is a merge commit created locally and never pushed, and the release stamp does not cover that either, since a branch ready to merge already carries this release's stamp. That residual is written into the workflow's own header alongside why the gate receipt cannot close it: the receipt is never committed, on purpose.",
            "The gate stopped re-reporting a class of finding about itself. `surfaces.yaml` claims every tracked file for exactly one surface and each reviewing agent is handed that one surface's documents, so a sentence in CONTRIBUTING.md whose truth turns on `docs/RELEASING.md` was unanswerable by construction — the file was not handed, not withheld, and not present. A surface entry may now declare a `reads:` list of documents it does not own but needs, handed over as context and marked as belonging to another surface. Ownership is untouched: `reads:` takes no part in the exactly-one rule, and the bundle refuses any overlap with the surface's own documents.",
            "Every published release page now points at the CHANGELOG, which is the only place a breaking or silent change is written out with its recovery. The generated notes above it are grouped commit subjects, and a `feat` line reads the same whether or not it breaks your build — v0.5.0's breaking change appeared there as an ordinary Features bullet.",
            "`build_role: schema` is documented as covering anything that must exist before the things below it can conform to it, which includes a declared contract — a signature an implementation is written against — and not only a data shape. A module documented at two levels produces a declaration and an implementation per entry point, and roling the declaration `api` makes every one of those edges a forward reference, because `behavior` builds before `api`. In one audited module that single cause produced 35 of 70. No code changes, no corpus revalidates, and no locked build order can go stale as a result.",
            "Smaller, and none of it changes what a consumer runs: the release checklist's own `git push origin vX.Y.Z` is written fully qualified, because a release developed on a branch of that name makes it fail outright and makes `git rev-parse` resolve by search order rather than by intent; this site now describes the claims graph outside its own release entry and names the `GET /api/graph` that backs its refresh button; CONTRIBUTING says plainly that the CI hook matrix outranks a local pass and that a green against a Chromium fork is not a green against Chrome; and two test guards that advertised more than they checked are repaired.",
          ],
        },
];

/** The current release — the last entry, matching ReleaseTimeline's own rule. */
export const latestRelease: Release = releases[releases.length - 1];

/** The single source for every version string on the site. */
export const latestVersion: string = latestRelease.version;

/*
 * THERE IS NO SECOND VERSION STRING, AND THAT IS NEW IN v0.5.2.
 *
 * A `latestBinaryVersion` used to live here — `latestVersion` with the leading
 * `v` stripped — because the two install paths disagreed. `.goreleaser.yaml`
 * stamped `-X main.version={{.Version}}`, which is the tag with its `v` removed,
 * so the published archive printed `dossierx version 0.5.1`; but
 * `go install …@v0.5.1` applies no ldflags at all and falls back to
 * `debug.ReadBuildInfo`, which the module proxy fills with the tag verbatim, so
 * that binary printed `v0.5.1`. One release, two answers, decided by how you
 * installed it. The extra constant existed so this page could depict whichever
 * one the reader would actually see.
 *
 * Issue #38 fixed the cause instead: the stamp is now `{{.Tag}}`, both paths
 * print the tag as tagged, and the transcript below reads `latestVersion` like
 * everything else on the page. Do not reintroduce a stripped copy — a bare
 * `0.5.2` is now a string no install path produces, and
 * viewer-tests/site_source_test.go scans this file for exactly that literal.
 */

/**
 * Lowercases only the first character, so a release title reads as a clause
 * mid-sentence. Deliberately not `toLowerCase()`, which would flatten the
 * proper nouns and acronyms release titles carry (GFM, CommonMark, JSON).
 */
function lowerFirst(s: string): string {
  return s.charAt(0).toLowerCase() + s.slice(1);
}

export const contentSpec: ContentSpec = {
  siteTitle: "DossierX",
  tagline:
    "DossierX turns a directory of atomic YAML claims into documentation that cannot silently rot — and gives its two readers two different surfaces. Your coding agent operates a nineteen-command JSON CLI. You read the rendered viewer in a browser, comment on any card, and click Resolve. Nothing already locked changes without your approval on the record.",
  nav: [
    { id: "hero", label: "Overview" },
    { id: "roles", label: "Who runs what" },
    { id: "philosophy", label: "Why" },
    { id: "claims", label: "Claims" },
    { id: "lifecycle", label: "Lifecycle" },
    { id: "comments", label: "Review loop" },
    { id: "build-order", label: "Build Order" },
    { id: "code-links", label: "Code Links" },
    { id: "cli", label: "CLI contract" },
    { id: "versions", label: "Releases" },
    { id: "compare", label: "vs. Wiki/ADR" },
  ],
  sections: [
    {
      id: "hero",
      title: "Agents do the work. You hold the lock.",
      kind: "hero",
      contentMd:
        "**DossierX** is a config-driven Go CLI that turns a directory of atomic YAML **claims** — one reviewable fact each — into a linted, dependency-checked, human-reviewable HTML viewer, governed by a lock / review_pending / reaudit lifecycle.\n\nIt has two users and gives them two surfaces. Your **agent** is the operator: it authors draft claims, runs `dossierx check`, links code to the claims it implements, and reads the comment inbox. **You** are the reviewer: you run `dossierx serve`, read the viewer, comment on any card, and resolve the threads you opened. That Resolve click is the approval — and it is load-bearing, because a claim with an open thread cannot lock.\n\nThe gate is narrower than it sounds. Draft claims are the agent's workshop, unfrictioned by design. The invariant is only this: nothing already **locked** changes without your approval on the record.\n\n**DossierX detects; the forge enforces.** The lock ledger rides in git and turns a silent edit into a *named* finding — the rule, the claim, and the command that restores it. Branch protection with a required CI check is what makes anyone obey that finding; the pre-commit hook is fast feedback in front of it. That division is why a red check here means something specific enough to act on.\n\nCLI-first, no public API. The only project-specific input the engine reads is your `project.config.yaml` — point the same binary at any project's config and it becomes that project's documentation engine.",
      data: {
        pitchLine:
          "An agent-operated, human-approved documentation engine: atomic YAML claims, a nineteen-command JSON CLI for the agent, a browser viewer for the reviewer, and a lock ledger in git so nothing locked moves silently.",
        badges: [
          "Go 1.26",
          "cobra + yaml.v3 only",
          "19 commands, JSON by default",
          latestVersion,
          "github.com/BarterX-Tech/dossierx",
        ],
        // These are the five STAGES of one command, not five commands. In
        // v0.2.0 the first three were verbs you could type; v0.3.0 deleted them
        // because they were packaging artifacts of an engine with no exported Go
        // API. They survive here as the values `check` reports in the
        // envelope's `stopped_at`, which is exactly why they still matter to an
        // agent: "stopped at lint" and "stopped at scan" call for different
        // next moves.
        pipeline: ["lint", "catalog", "render", "ledger", "scan"],
        roles: [
          {
            id: "agent",
            who: "Your agent",
            surface: "the CLI · 19 commands · JSON by default",
          },
          {
            id: "human",
            who: "You",
            surface: "the viewer · dossierx serve · comment and Resolve",
          },
        ],
        ctas: [
          {
            label: "View on GitHub",
            href: "https://github.com/BarterX-Tech/dossierx",
          },
          { label: "See who runs what", href: "#roles" },
        ],
      },
    },
    {
      id: "roles",
      title: "Two surfaces, and a boundary between them.",
      kind: "two-surface",
      contentMd:
        "The most common misconception about a tool like this is that a person sits down and drives it. Nobody does. Your coding agent operates DossierX — it writes the claims, runs the checks, links the code, and reads the comment inbox. You review what it did.\n\nSo each role gets its own surface and is denied the other's. The agent's surface is the CLI: nineteen commands, one JSON envelope per run, stable error codes to branch on. Your surface is the rendered viewer that `dossierx serve` opens on localhost, where the only two things you do are **comment** and **Resolve**.\n\nNothing stops you typing the nineteen commands — you are the approver, and sniffing for a TTY to refuse you would be theatre. But you should never need to, and the strengths below are labelled honestly: some of this boundary is enforced in code, some is enforced by git, and one line of it is convention backed by a required `--reason`.",
      data: {
        surfaces: [
          {
            id: "human",
            role: "Reviewer",
            who: "Human",
            surface: "the viewer, in a browser · chat with the agent",
            command: "dossierx serve",
            commandNote: "the one DossierX command you run",
            can: [
              "Read every claim, facet, build order and code link",
              "Open the claims graph — every rests_on, governed_by and mirrors edge as one canvas, with overlays for cycles, governance, isolated claims and open threads",
              "Comment on any card — including a card with no thread yet",
              "Reply; resolve and reopen any thread",
              "Edit and delete your own messages",
              "Instruct the agent in chat: lock, unlock, flag, reaudit, fix",
            ],
            cannot: [
              "Lock, unlock, flag or reaudit from the viewer — there are no such controls",
              "Create or edit a claim",
              "Propose or lock a build order, or link code",
              "Comment on a static file:// export — read-only by design",
            ],
            footnote:
              "Not stopped from typing the nineteen commands anyway. You are the approver; nothing pretends otherwise.",
          },
          {
            id: "agent",
            role: "Operator",
            who: "Agent",
            surface: "the CLI, all 19 commands · JSON by default",
            command: "dossierx check",
            commandNote: "the whole pipeline, as often as it likes",
            can: [
              "Create, edit, restructure and delete DRAFT claims — this is the work",
              "Open threads and reply to any thread",
              "Run check, claim show, claim list, comment inbox freely",
              "Link finished code to the claim it implements",
            ],
            cannot: [
              "Change a locked claim without an approval record on the ledger",
              "Lock, unlock, flag or reaudit without your yes — every one needs --reason",
              "Resolve or reopen a thread a human opened — advisory rights, already enforced",
              "Edit or delete any comment — that surface is the viewer's",
            ],
            footnote:
              "Every mutating verb takes --dry-run, so the agent can show you the transition, the preconditions and the blast radius before it writes anything.",
          },
        ],
        boundary: {
          label: "the approval boundary",
          crossings: [
            {
              from: "human",
              text: "“I left comments.”",
              note: "in chat — the human's half of the handoff",
            },
            {
              from: "agent",
              text: "“Here is the diff. May I lock it?”",
              note: "a --dry-run preview, then it waits",
            },
          ],
        },
        gate: "The rule is not “the agent may not touch claims”. The agent does all the work, and draft authoring stays completely unfrictioned. The invariant is narrower and therefore keepable: nothing already locked changes without your approval on the record. Draft claims are the agent's workshop. Locked claims are yours.",
        enforcement: [
          {
            strength: "Hard",
            title: "In the code itself",
            body: "The viewer has no lock controls, so a browser cannot lock. Comment edit and delete are gone from the CLI. Advisory rights are enforced against the CLI actor: an agent passing --as agent can reply to your thread and cannot close it. The viewer's own write API is the exception, and it is labelled under Convention below.",
          },
          {
            strength: "Blocked",
            title: "By git, not by a harness",
            body: "A commit carrying an unapproved change to a locked claim is refused by the pre-commit hook, whatever tool made it — because the gate is dossierx check --staged over the index, and every tool commits through git. It is one hook, not one plugin per harness. It is also local and skippable, which is why the row below exists.",
          },
          {
            strength: "Detected",
            title: "By check, and by CI",
            body: "Where no hook is installed, dossierx check fails on the same condition. Merges, rebases, cherry-picks and reverts never fire pre-commit at all: the hook is fast feedback, CI is the authority.",
          },
          {
            strength: "Convention",
            title: "Named as such",
            body: "“Ask before locking” lives in the skills, not in a syscall. The required --reason cannot make an unprompted lock impossible — it makes it loud, attributable and reviewable in the ledger. “You reply, the human resolves” is the same kind of line where the viewer's write API is concerned: it is localhost-bound and same-origin-checked but not authenticated, and a request that omits its actor is treated as human, so any local caller has your rights there. Deliberate — anything that can curl it can already edit the claim YAML — but stated rather than implied.",
          },
        ],
      },
    },
    {
      id: "philosophy",
      title: "A surface an agent can trust, and facts a human can.",
      kind: "narrative",
      contentMd:
        "An agent cannot operate a tool whose answers are English sentences. It needs one response shape, one vocabulary of error codes, and a promise that neither will move under it — so v0.3.0 made the machine contract the product's spine: `--format json` by default, one envelope per invocation, a stable `error.code` to branch on, and `--dry-run` on everything that writes. The surface got *smaller* — twenty-six commands to nineteen, across two releases — while getting more capable, because every verb that survived is a verb an agent actually needs.\n\nUnderneath it, the original thesis is unchanged. Markdown folders, ADRs and wikis fail the same way: prose can become wrong without producing a machine-readable signal. DossierX replaces page-level trust with atomic facts that can be linted, reviewed, locked, and flagged when their dependencies or implementing code move.\n\nIt began as internal tooling inside a private, multi-module production app that had been burned by silent documentation drift. The public tool keeps the proven claim schema, the check pipeline, the lifecycle, build ordering and code linking, and takes all project-specific structure from `project.config.yaml`. The embedded skills teach an agent the whole loop: author claims, derive a build order, ground finished code in the claim it implements, and review with threaded comments — never touching a locked claim without asking you first.",
      data: {
        principles: [
          {
            title: "One surface per role",
            body: "The agent gets nineteen commands and a JSON envelope; the human gets a browser and two gestures. Neither gets the other's, which is what makes the roles legible instead of a convention someone has to remember.",
          },
          {
            title: "A narrow gate, kept",
            body: "Draft claims are free — create, rewrite, delete, no friction. Only locked claims are gated, and only by one rule: no change without an approval record. A gate this narrow is one you can actually enforce.",
          },
          {
            title: "Determinism is first-class",
            body: "Catalog output is always alphabetical-by-id and never ranges over Go maps directly — two builds from identical input are byte-for-byte identical, so .catalog.json, the lock ledger and lint diffs are all reviewable in version control.",
          },
          {
            title: "Enforcement rides git",
            body: "The pre-write gate is a pre-commit hook running check over the index, not a per-harness plugin. Claude Code, Codex, Pi, Kimi, your editor, a shell script — all commit through git, all hit the same gate, and there is no adapter to maintain.",
          },
        ],
      },
    },
    {
      id: "claims",
      title: "The claim — the atomic unit of everything.",
      kind: "model-diagram",
      contentMd:
        "A **claim** is one reviewable, YAML-authored fact: prose, a table, a sequence, or a mockup. Its `module.facet.slug` id, content, status, and typed relationships give the engine enough structure to validate what ordinary prose cannot.\n\nThe agent authors them with `dossierx claim new` and inspects them with `dossierx claim show` — hand-editing YAML is the thing being gated, so there is a sanctioned way to write one. Claims form a dependency graph through `mirrors`, `rests_on`, and `governed_by`, and the viewer draws that graph: a **Claims graph** pane renders every edge as one canvas, with overlays for dependency cycles, governance, isolated and weakly linked claims, draft versus locked, and open comment threads, plus a per-claim detail panel and collapse to modules or facets. Separately, `build_role` places a fact in the implementation sequence, while `kind` distinguishes facts from reading guidance. The schema, examples, and real on-disk layout below show the full contract.",
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
          code: 'id: logger.internals.event-envelope-fields\nfacet: internals\nmodule: logger\nstatus: locked          # set by claim lock --reason, never hand-edited\nlayout: table          # inferred from `rows` if omitted\nbuild_role: schema     # REQUIRED now that status is locked\nsection: "1 - event envelope"\norder: 2               # viewer-only display hint\nrows:\n  - field: event_name\n    type: string\n    notes: stable machine name, carries no dynamic values\n  - field: severity\n    type: enum\n    notes: one shared severity vocabulary\nrests_on:\n  - logger.contract.event-envelope-overview   # target must exist\ngoverned_by:\n  type: platform.doctrine.stable-machine-names-carry-no-dynamic-values',
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
            desc: "draft | locked — moved only by claim lock / claim unlock, each requiring --reason. Hand-flipping it is what the lock ledger catches.",
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
            desc: "orientation | schema | behavior | api | verification | out-of-scope. Optional while draft, REQUIRED once locked. `schema` covers anything that must exist before the things below it can conform to it — data shapes, and also a declared contract such as a signature an implementation is written against. `api` is for entry points that wrap the behavior they call into.",
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
            desc: "provenance note — what claim list --migrated counts",
          },
          {
            name: "raw_html + raw_html_reviewed",
            desc: "review gate for unescaped HTML in raw_html claims (any layout, including layout: mockup) — and the reason the lock ledger hashes every persisted field, not just the eleven a content hash covers",
          },
        ],
        engineManagedFields: [
          {
            name: "review_pending",
            desc: "set on a locked claim by any of three triggers — a rests_on/mirrors/governed_by target's content hash drifts, an agent runs claim flag, or a comment thread is open — and cleared by claim reaudit --confirm, by an unlock, or by resolving the last open thread once no other trigger stands",
          },
          {
            name: "comments",
            desc: "the review threads themselves, excluded from the claim's content hash so commenting never rewrites a dependent claim — and digested into their own store so a live reviewer's browser is never a writer to the lock store",
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
              "Names the doctrine claim backing this claim's authority — or type: none with a required reason. It is NOT a gating edge: with doctrine_facet set, hub-gating walks mirrors and rests_on only, so a doctrine claim named solely through governed_by does not hold up the lock, and to have hub-gating cover it you name it in rests_on as well. Since v0.4.0 it is also a DRIFT edge alongside mirrors and rests_on: a claim-valued governor whose content changes under a locked claim flags that claim review_pending. Self-edges are rejected and the governance graph is cycle-checked in its own pass.",
          },
        ],
      },
    },
    {
      id: "lifecycle",
      title: "Who mandates the transition, and who executes it.",
      kind: "lifecycle-diagram",
      contentMd:
        "A locked claim is a **trust assertion**: you reviewed it, lint passed, and other claims may safely depend on it. The lifecycle has not changed since v0.3.0 — what changed there is that every edge now names two parties. You **mandate** a transition; the agent **executes** it, and records your words in `--reason`.\n\nOnly two edges have no human on the mandating side, and both are the engine noticing something rather than deciding anything: `DetectStale` flags a locked claim whose dependency drifted, and opening a comment thread flags one under discussion. Everything else — lock, unlock, reaudit, and the flag an agent raises when code and claim disagree — needs a yes it can quote.\n\nThe agent's move before any of them is `--dry-run`: the transition it would make, the preconditions it would have to satisfy, what else the change touches, and what is still missing. You read that, then say yes.",
      data: {
        states: [
          {
            id: "draft",
            label: "draft",
            desc: "The agent's workshop. Freely editable, not yet trusted, nothing depends on it.",
          },
          {
            id: "locked",
            label: "locked",
            desc: "Your reviewed source of truth. Lint-gated, ledger-recorded, safe to depend on.",
          },
          {
            id: "review_pending",
            label: "locked + review_pending",
            desc: "Still locked, still trusted — but visibly flagged: a dependency drifted, code no longer matches, or a thread is open.",
          },
        ],
        transitions: [
          {
            from: "draft",
            to: "locked",
            trigger: 'dossierx claim lock <id> --reason "…"',
            mandate: "Human — in chat",
            execute: "Agent",
            note: "Refused on any lint error, on hub-gating (a doctrine-facet claim named in mirrors or rests_on still draft), and on any open comment thread. --reason is required, so an unprompted lock is loud and attributable.",
          },
          {
            from: "locked",
            to: "review_pending",
            trigger: "DetectStale (automatic)",
            mandate: "Nobody — the engine noticed",
            execute: "Engine, inside check",
            note: "A mirrors/rests_on/governed_by dependency's content hash changed. Persisted back to the claim file by dossierx check. A claim locked before v0.4.0 carries no governance baseline until its next lock or reaudit, so its first governor edit after upgrading does not flag it.",
          },
          {
            from: "locked",
            to: "review_pending",
            trigger: "dossierx claim flag <id>",
            mandate: "Agent — reporting, not deciding",
            execute: "Agent",
            note: "The agent asserts that code drifted from the claim. Requires --claim-says --now-does --reason, all non-empty. Locked claims only; refused (structured_layout) on any claim carrying rows, steps or raw_html — the test is on content, not on the layout name, so a card bearing raw_html is refused too.",
          },
          {
            from: "locked",
            to: "review_pending",
            trigger: "a comment thread is opened",
            mandate: "Human — usually from the viewer",
            execute: "Engine",
            note: "An open thread on a locked claim is a legal, long-lived state. A draft claim with an open thread cannot lock in the first place.",
          },
          {
            from: "review_pending",
            to: "locked",
            trigger: "dossierx claim reaudit <id> --confirm --reason …",
            mandate: "Human — after reading the diff",
            execute: "Agent",
            note: "The DRIFT tool, not the general edit tool: it rewrites body only, refuses a claim that is not already review_pending, and refuses a comment-only review_pending (there is no content change to diff). Bare reaudit previews; --confirm applies and appends audit_notes.",
          },
          {
            from: "review_pending",
            to: "locked",
            trigger: "Resolve, clicked in the viewer",
            mandate: "Human",
            execute: "Human — the one write they make",
            note: "Resolving the last open thread clears review_pending, unless drift or a flag also stands, in which case the flag is retained with a printed reason. An agent operating the CLI cannot do this for you: advisory rights are enforced against --as, and an agent asserting --as agent is refused.",
          },
          {
            from: "locked",
            to: "draft",
            trigger: 'dossierx claim unlock <id> --reason "…"',
            mandate: "Human — in chat",
            execute: "Agent",
            note: "The approval path for changing anything locked is unlock → fix → lock, not reaudit. No lint gate: you may need to unlock precisely to fix what lint complains about. The ledger record survives the unlock so drift detection still works across the window where it matters.",
          },
        ],
        invariant:
          "A locked claim's status never reverts to draft on its own, and never moves at all without a record. review_pending has three triggers (dependency drift, claim flag, an open thread) and three clearers (reaudit --confirm, resolving the last open thread, unlock). Every state change to a locked claim leaves a ledger entry carrying the hash, the timestamp, the actor and your reason — so a hand-edit that skips the CLI is not prevented, it is simply no longer silent.",
      },
    },
    {
      id: "comments",
      title: "The review loop — your two gestures, and where they land.",
      kind: "comments-workflow",
      contentMd:
        "This is the loop the whole release is built around, and it starts with you disagreeing with something on screen.\n\nYou open `dossierx serve`, read a card, and click 💬 — on **any** card, including one nobody has commented on yet. Your comment is written into that claim's YAML; the 500 ms watcher re-renders and every open viewer updates. You tell your agent “I left comments.” It runs `dossierx comment inbox` — one call, every open thread in the project, each one carrying `agent_can_resolve` so it never wastes a call trying to close your thread. It fixes the claim and replies.\n\nThen you click **Resolve**. That click is the approval: `dossierx claim lock` refuses any claim with an open thread, so resolving is literally what unblocks locking. It is the cheapest honest approval signal in the design, and it already existed — v0.3.0's job was to stop the loop dead-ending, by making every card commentable and giving the agent one call to find what you wrote.",
      data: {
        roles: [
          { id: "human", label: "human" },
          { id: "agent", label: "agent" },
          { id: "engine", label: "engine" },
        ],
        // The stepped loop. `surface` names WHERE the step happens, which is the
        // fact the old copy left out: a reader who does not know that step 1 is
        // a browser and step 4 is a terminal cannot picture the loop at all.
        loop: [
          {
            role: "human",
            surface: "viewer",
            title: "Comment on the card",
            body: "You read a claim in dossierx serve, disagree, and click 💬: “This row contradicts the API facet — which is right?” Any card, including one with no thread yet.",
          },
          {
            role: "engine",
            surface: "claims/…yaml + serve",
            title: "It lands in the claim",
            body: "The thread is written into the claim's own YAML under comments:, excluded from its content hash so no dependent claim is disturbed. serve's 500 ms watcher re-renders and every open viewer updates over server-sent events.",
          },
          {
            role: "human",
            surface: "chat",
            title: "“I left comments.”",
            body: "The whole handoff. You do not name the claim, quote an id, or run a command — the next step finds all of it.",
          },
          {
            role: "agent",
            surface: "terminal",
            title: "dossierx comment inbox",
            body: "Every open thread in the project, oldest activity first, in one call — with claim id, module, facet, the body, and agent_can_resolve per thread. --since <cursor> makes the next poll incremental without ever missing a thread.",
          },
          {
            role: "agent",
            surface: "terminal + claim",
            title: "Fix, then reply — never resolve",
            body: "It edits the claim and replies on your thread: “Fixed the rows; the API facet was stale.” It does not resolve: on the CLI advisory rights refuse it outright, and its skills forbid reaching around them, so your thread stays yours.",
          },
          {
            role: "human",
            surface: "viewer",
            title: "You click Resolve",
            body: "This is the approval. claim lock refuses any claim with an open thread, so this one click is what unblocks locking — no second tool, no second vocabulary, no command to remember.",
            emphasis: true,
          },
          {
            role: "human",
            surface: "chat",
            title: "“Good, lock it.”",
            body: "Said in words, about a card, not an id: “the severity row in the internals facet.” Resolving that to a claim id is the agent's job, and it confirms the id back to you before acting.",
          },
          {
            role: "agent",
            surface: "terminal",
            title: "Preview, then lock with your words",
            body: "claim lock --dry-run first: the transition, the preconditions, what else it affects, what is missing. Then dossierx claim lock <id> --reason \"<your words>\" — which writes the ledger entry that makes the change non-silent forever.",
          },
        ],
        // Visual (d): the same claim seen from both surfaces at once. The
        // terminal half is a real `comment inbox` envelope (trimmed to the
        // fields that matter for the story) and the viewer half is the card and
        // thread that same claim renders — same claim id, same thread id, one
        // YAML file underneath.
        surfacePair: {
          claimID: "logger.internals.event-envelope-fields",
          terminal: {
            title: "the agent's terminal",
            url: "dossierx comment inbox",
            code: '$ dossierx comment inbox\n{\n  "ok": true,\n  "command": "comment inbox",\n  "data": {\n    "cursor": "2026-07-26T10:12:00Z",\n    "count": 1,\n    "claims": 1,\n    "threads": [\n      {\n        "claim_id": "logger.internals.event-envelope-fields",\n        "claim_title": "Event Envelope Fields",\n        "module": "logger",\n        "facet": "internals",\n        "claim_status": "draft",\n        "thread_id": "c-8f3a2b",\n        "author": "human",\n        "created": "2026-07-26T10:12:00Z",\n        "body": "This row contradicts the API facet — which is right?",\n        "replies": 1,\n        "last_activity": "2026-07-26T10:40:00Z",\n        "last_author": "agent",\n        "agent_can_resolve": false,\n        "agent_has_replied": true\n      }\n    ]\n  }\n}',
            note: "agent_can_resolve is false, and the agent knows it before it spends a call. The thread is the human's; replying is the whole move.",
          },
          viewer: {
            title: "the reviewer's browser",
            url: "http://127.0.0.1:52431/#logger.internals.event-envelope-fields",
            note: "Two controls, and neither of them locks anything: the composer, and Resolve.",
          },
          card: {
            id: "logger.internals.event-envelope-fields",
            module: "logger",
            facet: "internals",
            status: "draft",
            statusNote: "blocked from locking · 1 open thread",
            panelTitle: "Comments",
            rows: [
              { field: "event_name", type: "string" },
              { field: "severity", type: "enum", flagged: true },
            ],
            thread: {
              id: "c-8f3a2b",
              role: "human",
              status: "open",
              created: "2026-07-26 · 10:12",
              body: "This row contradicts the API facet — which is right?",
              replies: [
                {
                  id: "r-4c9e11",
                  role: "agent",
                  created: "2026-07-26 · 10:40",
                  body: "Fixed the rows; the API facet was stale.",
                },
              ],
            },
            resolvedCount: 1,
          },
        },
        gateNote:
          'Both halves read and write ONE file. The browser and the CLI share the same claims lock, so a live reviewer and a working agent never clobber each other, and no page load ever rewrites viewer/index.html or .catalog.json — serve renders from memory.',
      },
    },
    {
      id: "build-order",
      title: "Build Order — the sequence to actually implement in.",
      kind: "build-order-diagram",
      contentMd:
        "Once a module is fully locked, DossierX turns `build_role` and `rests_on` into a dependency-safe implementation sequence — the order the agent should write code in, which is not the human reading order the viewer displays. One optimizes comprehension; the other tells an implementer what must exist first.\n\nFive phases stay fixed, while behavior and API claims are topologically sorted inside their phase. Proposal fails on drafts, on unresolved threads, or on a same-phase cycle, reports excluded work, and becomes a drift-checked artifact only once you approve the lock — which, like every other lock in v0.3.0, takes a `--reason` and lands in the ledger.",
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
            desc: "Derive the sequence and write .build-order.<module>.json (locked:false). Refused unless every claim in the module is locked and thread-free; fails on a same-phase cycle. --dry-run reports the order without writing it.",
          },
          {
            cmd: "dossierx build-order status --module <name>",
            desc: "Reports proposed / locked / stale plus coverage (N of M covered, K excluded). Staleness is structural: it re-derives what a fresh propose would compute and flags any divergence.",
          },
          {
            cmd: "dossierx build-order lock --module <name> --reason …",
            desc: "Freeze the proposed sequence, snapshotting a content-hash baseline. A locked build order is a locked artifact, so it sits inside the same gate as a locked claim — your yes, in --reason, on the record, and check reports a hand-edited one as build-order-content-drift. Refused on a stale order (build_order_stale) rather than refreshed: re-propose, show the new sequence, lock again.",
          },
        ],
      },
    },
    {
      id: "code-links",
      title: "Code links — closing the loop between docs and code.",
      kind: "narrative",
      contentMd:
        "DossierX keeps a drift-checked map from every locked claim to the source that implements it. A `dossierx-claim: <id>` marker lets `dossierx check` link code automatically; unknown or unlocked ids fail the check, and later file changes surface as drift in `claim show` and `claim list --drifted`.\n\nThe division of labour is the same one the rest of the release runs on. Grounding correct code in the claim it implements is fully autonomous — the agent tags the file, check reconciles it, nobody is asked anything. Deciding that a locked claim is now *wrong* is not: `dossierx claim flag` records what the claim says, what the code now does, and why, and sends the claim through the visible `review_pending → reaudit` path instead of silently rewriting either side.",
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
          code: "dossierx claim link --module widget \\\n  --claim widget.internals.queue-saturation-policy \\\n  --file src/widget/queue.py --symbol _drop_for_saturation\n\n# read-only: one call for edges, links, drift, discussion and next actions\ndossierx claim show widget.internals.queue-saturation-policy\ndossierx claim list --module widget --drifted",
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
            owner: "Human-mandated: claim flag → reaudit, or unlock → fix → lock",
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
      title: "The machine contract — what your agent uses.",
      kind: "cli-explorer",
      contentMd:
        "This is not a tutorial, because nobody is going to type these. It is the contract your agent codes against: nineteen commands, one JSON envelope per invocation, a stable `error.code` to branch on, `--dry-run` on everything that writes, and `--reason` on everything that changes a locked thing.\n\nOne binary serves any project through `project.config.yaml`, discovered by walking up from the working directory the way git finds `.git`. `check` is the CI entry point — it detects drift, lints, catalogs, renders the viewer, verifies the lock ledger and reconciles code links, and reports which of those stages it stopped at. `--validate` runs it read-only; `--staged` judges the git index instead of the working tree, which is what the pre-commit hook runs.",
      data: {
        // The three contract facts an agent needs before any individual
        // command matters: the shape of every answer, the vocabulary of every
        // failure, and the promise that nothing writes until it has been
        // previewed. Rendered above the explorer for that reason.
        contract: [
          {
            title: "One envelope, every run",
            body: "Exactly one JSON document per invocation, on stdout, on success and on failure. data is per-command and documented with it; warnings appear on successful runs too, which is when they are easiest to miss; stopped_at names the stage a fail-fast pipeline reached. --format text returns the old human prose, byte-for-byte.",
            code: '{\n  "ok": false,\n  "command": "claim lock",\n  "error": {\n    "code": "unresolved_comments",\n    "message": "claim has 1 unresolved comment thread(s) [c-8f3a2b]",\n    "hint": "the human resolves it in the viewer; that click is the approval",\n    "details": { "open_thread_ids": ["c-8f3a2b"] }\n  },\n  "stopped_at": ""\n}',
          },
          {
            title: "Codes, not sentences",
            body: "error.code is the only field promised to be stable — branch on it, never regex message or hint. The vocabulary is shared with the viewer's JSON API, so an agent and a browser get literally the same constants for the same condition. Exit codes stayed additive: 1 is a generic failure, 2 is not-found or wrong-state.",
          },
          {
            title: "Nothing writes unpreviewed",
            body: "Every mutating verb takes --dry-run and reports the transition, the preconditions, the side effects and what is missing. claim lock / unlock / reaudit --confirm and build-order lock additionally require --reason: your approval, in your words, written to the ledger. Comment verbs require --as human|agent — never defaulted, so a human's terminal action can never file as the agent.",
          },
        ],
        errorCodes: [
          {
            code: "unresolved_comments",
            recovery:
              "The claim has an open thread. Reply on it; the human resolves it in the viewer, and that click is the approval the lock gate wants.",
          },
          {
            code: "integrity_failed",
            recovery:
              "A locked claim moved with no matching ledger record. Restore the file, or take the honest path: unlock → fix → lock.",
          },
          {
            code: "pre_ledger_unadopted",
            recovery:
              "This project's lock store predates the lock ledger and it still holds locked artifacts, so no approval can be recorded here. There is no migration command: re-propose any locked build order, unlock every locked claim, then lock only what the human still stands behind — the first lock in a project holding nothing locked crosses the store onto the ledger.",
          },
          {
            code: "lint_failed",
            recovery:
              "Run check --validate (read-only) and fix the findings before trying to lock again.",
          },
          {
            code: "rights_denied",
            recovery:
              "You tried to act on a message you did not author. Reply instead — comment inbox told you agent_can_resolve was false.",
          },
          {
            code: "dependency_not_locked",
            recovery:
              "Hub-gating: a doctrine-facet claim this one names in mirrors or rests_on is still draft. Lock that one first, with its own approval.",
          },
          {
            code: "not_review_pending",
            recovery:
              "reaudit only accepts a claim already flagged review_pending. To change a settled locked claim, unlock → fix → lock.",
          },
          {
            code: "claim_not_found",
            recovery:
              "The human described a card, not an id. Resolve it with claim list --match \"<their words>\" and confirm the id back to them.",
          },
          {
            code: "write_conflict",
            recovery:
              "Contention on a write sentinel — another dossierx process, usually serve, holds it. Retry. If the retry stalls the same ~10s and fails identically, a process died holding it: delete the .lock file the message names. (A claim that changed under you is claim_file_changed, which you re-read rather than retry.)",
          },
        ],
        groups: [
          {
            group: "check",
            commands: [
              {
                name: "check",
                usage: "dossierx check [--validate] [--staged]",
                summary:
                  "The whole pipeline in one shot — the CI entry point, and the agent's routine command.",
                detail:
                  "1) DetectStale flips drifted locked claims to review_pending and persists the flag. 2) lint → catalog → render, stopping at the first failure. 3) The lock-ledger gate: every locked claim's hash must match a ledger record. 4) Scans source_dirs for dossierx-claim tags (an unknown or unlocked tag is a HARD failure). 5) Prints orientation-note counts, impl-link drift, and a next-steps block. --validate lints in memory and writes NOTHING — no claim files, no lock store, no .catalog.json, no viewer — which is what makes the per-claim authoring loop safe. --staged judges the GIT INDEX rather than the working tree, reading content from git show, and is what the pre-commit hook runs.",
                example:
                  '$ dossierx check --format text\nimpl-links: scanned 214 file(s), found 37 tag(s), reconciled 37 link(s) (0 error(s))\ncheck: OK\nnext steps:\n  4 claim(s) still draft -> ask the human, then lock them one at a time\n  module "logger" is fully locked with no build order yet -> dossierx build-order propose --module logger',
              },
            ],
          },
          {
            group: "claim",
            commands: [
              {
                name: "claim show",
                usage: "dossierx claim show <id>",
                summary:
                  "One claim's entire state in a single call — the command an agent starts from.",
                detail:
                  "Absorbs the retired deps and implink status. Reports lifecycle (status, locked_at, review_pending and WHICH of the three triggers set it), both edge directions including the incoming half no other command could show, implemented_in with a per-file drift verdict, the comment roll-up with the open thread ids in full (they are a lock gate, so counting them is not enough), and next_actions — computed in the binary from the same gates the write paths enforce, so the advice can never disagree with what a command would actually do.",
                example:
                  '$ dossierx claim show logger.internals.dispatch\n{\n  "ok": true,\n  "command": "claim show",\n  "data": {\n    "claim_id": "logger.internals.dispatch",\n    "status": "locked",\n    "review_pending": true,\n    "review_pending_trigger": "comments",\n    "comments": { "threads": 2, "open": 1, "open_thread_ids": ["c-8f3a2b"] },\n    "next_actions": [\n      "review_pending because of open discussion -> reply on the thread; only the human can resolve it"\n    ]\n  }\n}',
              },
              {
                name: "claim list",
                usage:
                  "dossierx claim list [--review-pending] [--migrated] [--drifted] [--facet …] [--module …] [--match …]",
                summary:
                  "The filtered inventory — and how an agent turns “that severity card” into an id.",
                detail:
                  "Absorbs the retired stale (--review-pending) and coverage (--migrated) verbs, because those were filters wearing a verb's clothes. --drifted selects claims whose linked files changed. --match fuzzy-matches the id and derived title and ranks by relevance, which is the sanctioned way to resolve a human's description of a card; the agent then confirms the id back before acting on it.",
                example:
                  "$ dossierx claim list --review-pending --format text\nlocked logger.internals.dispatch review_pending drifted\nlocked widget.api.enqueue review_pending open_threads=1\nclaim list: 2 of 186 claim(s) (1.1%)",
              },
              {
                name: "claim new",
                usage:
                  'dossierx claim new <id> --body "…" [--layout …] [--build-role …] [--rests-on …] [--dry-run]',
                summary:
                  "The sanctioned way to author a claim, since hand-writing claim YAML is the thing being gated.",
                detail:
                  "Writes one DRAFT claim to <claims_dir>/<id>.yaml (or --file), validating the id's three segments against the configured modules and facets. --governed-by defaults to none and then requires --governed-reason, so an ungoverned claim is a stated choice rather than an omission. Authors card/list/tree/banner layouts; table, steps and mockup claims need rows/steps/raw_html, which this command deliberately does not write.",
                example:
                  '$ dossierx claim new logger.contract.overview \\\n    --body "The logger module owns event emission for the whole platform." \\\n    --governed-by none --governed-reason "no doctrine facet configured yet"\n{\n  "ok": true,\n  "command": "claim new",\n  "data": {\n    "claim_id": "logger.contract.overview",\n    "path": "claims/logger.contract.overview.yaml",\n    "status": "draft",\n    "layout": "card",\n    "lint_error_count": 0,\n    "lint_warning_count": 1\n  }\n}',
              },
              {
                name: "claim lock",
                usage: 'dossierx claim lock <id> --reason "…" [--dry-run]',
                summary: "Promote a draft claim to locked — with your yes on it.",
                detail:
                  "Refused on any lint error, on hub-gating (mirrors or rests_on names a doctrine-facet claim that is still draft — governed_by does not gate), on any unresolved comment thread, and on a claim that is already locked (already_locked) — re-locking would sign a fresh approval over whatever the file now says and clear review_pending with no diff, so the path stays unlock → fix → lock. Takes the claims file lock, saves the claim, snapshots the per-dependent content-hash baseline, and writes the ledger record — {hash, at, actor, reason} — that makes any later out-of-band edit detectable. --reason is required: it is the human approval this command executes, in their words.",
                example:
                  '$ dossierx claim lock logger.contract.api-surface --reason "reviewed in the viewer, resolved c-8f3a2b" --format text\nlock: logger.contract.api-surface is now locked',
              },
              {
                name: "claim unlock",
                usage: 'dossierx claim unlock <id> --reason "…" [--dry-run]',
                summary:
                  "Return a locked claim to draft — the first half of the approval path.",
                detail:
                  "unlock → fix → lock is how anything locked changes; reaudit is not that tool. No lint gate, because you may need to unlock precisely to fix what lint complains about. Clears review_pending and any pending flag, and keeps the ledger record alive so comment-drift detection survives the window where it matters.",
                example:
                  '$ dossierx claim unlock logger.contract.api-surface --reason "the retry note is wrong, rewriting" --format text\nunlock: logger.contract.api-surface is now draft',
              },
              {
                name: "claim flag",
                usage:
                  "dossierx claim flag <id> --claim-says … --now-does … --reason … [--dry-run]",
                summary:
                  "The agent's one report-a-problem verb: code and claim now disagree.",
                detail:
                  "All three flags required and non-empty; only a LOCKED claim can be flagged, and a claim whose content is not just `body` is refused with structured_layout — rows or steps present, layout: mockup, or raw_html on ANY layout. The test is on CONTENT, not on the layout name: raw_html is an attachment legal on every layout, so a card, banner, list or tree claim carrying one is refused too, and no layout is flaggable by name. A flag-sourced reaudit rewrites body and nothing else, so accepting one of those would clear review_pending while the content a reader actually sees stayed stale. Writes a one-shot pending flag and sets review_pending, which routes the claim through the visible reaudit path rather than letting an agent quietly rewrite a locked fact.",
                example:
                  '$ dossierx claim flag logger.internals.dispatch \\\n  --claim-says "dispatch is synchronous" \\\n  --now-does   "dispatch runs on a worker pool" \\\n  --reason     "concurrency added in PR #42"',
              },
              {
                name: "claim reaudit",
                usage:
                  'dossierx claim reaudit <id> [--confirm --reason "…"] [--dry-run]',
                summary:
                  "The drift tool: propose, then apply, a diff for a review_pending claim.",
                detail:
                  "Deliberately narrow. It refuses a claim that is not already review_pending, it rewrites body only, and it refuses a comment-only review_pending because a comment carries no content change to diff — it points at the viewer instead. Bare reaudit prints the proposal and stops; --confirm --reason applies it, appends audit_notes, re-baselines the dependency hashes and clears the flag (leaving review_pending set if an open thread still stands). --dry-run always wins over --confirm.",
                example:
                  '$ dossierx claim reaudit logger.internals.dispatch --confirm --reason "confirmed against dispatch.go" --format text\nreaudit: logger.internals.dispatch applied, review_pending cleared',
              },
              {
                name: "claim link",
                usage:
                  "dossierx claim link --module … --claim … --file … [--symbol …] [--dry-run]",
                summary:
                  "Manually ground a locked claim in the file that implements it.",
                detail:
                  "The manual counterpart to check's automatic dossierx-claim scan, for the cases scanning cannot reach (no source_dirs, generated files). Validates: claim exists → belongs to --module → is locked → file resolves project-relative (absolute paths and .. escapes refused). Snapshots the file's sha256 as the drift baseline. Fully agent-autonomous: grounding correct code needs no approval.",
                example:
                  "$ dossierx claim link --module logger \\\n  --claim logger.internals.dispatch \\\n  --file internal/logger/dispatch.go --symbol Dispatcher.Run",
              },
            ],
          },
          {
            group: "comment",
            commands: [
              {
                name: "comment inbox",
                usage: "dossierx comment inbox [--since <RFC3339>]",
                summary:
                  "Every open thread in the project, in one call — what the human left for you.",
                detail:
                  "The agent's half of the review loop, and the reason the loop no longer dead-ends: “I left comments” used to cost one call per claim. Oldest activity first, because an inbox is a queue. Each thread carries agent_can_resolve (almost always false, by design — the human's Resolve click is the approval) and agent_has_replied, so the agent never spends a call earning a rights_denied. The echoed cursor advances over every open thread, including ones --since filtered out, so polling with it cannot miss a comment written while the previous call was running.",
                example:
                  "$ dossierx comment inbox --format text\nlogger.internals.event-envelope-fields c-8f3a2b human 2026-07-26T10:12:00Z replies=0 agent_can_resolve=false: This row contradicts the API facet — which is right?\ncomment inbox: 1 open thread(s) across 1 claim(s)\ncomment inbox: cursor 2026-07-26T10:12:00Z",
              },
              {
                name: "comment list",
                usage: "dossierx comment list <claim-id> [--open]",
                summary: "One claim's threads, one greppable line each.",
                detail:
                  "Pinned one-line-per-thread format under --format text: <thread-id> <status> <author> <created> replies=<N>: <first-line-of-body> — a stable contract the skills, this site and a golden test all reproduce. --open filters to unresolved threads. Under the default JSON it returns the full thread objects, replies included.",
                example:
                  "$ dossierx comment list logger.internals.event-envelope-fields --open --format text\nc-8f3a2b open human 2026-07-26T10:12:00Z replies=1: This row contradicts the API facet — which is right?",
              },
              {
                name: "comment add",
                usage:
                  'dossierx comment add <claim-id> --as human|agent --body "…" [--dry-run]',
                summary: "Open a new thread on a claim.",
                detail:
                  "Mints an engine-generated thread id and echoes it so the next verb can chain. --as records the author role, not an identity, and is required on every mutating comment verb — never defaulted, because a default would let a human's terminal action file as the agent and quietly acquire agent rights. A draft claim with an open thread cannot lock; opening one on a locked claim sets review_pending. Refused on banner-layout claims, which render no footer to hold a thread.",
                example:
                  '$ dossierx comment add logger.internals.event-envelope-fields --as agent --body "Rows now match the API facet — please confirm." --format text\ncomment: c-91ba7d added on logger.internals.event-envelope-fields',
              },
              {
                name: "comment reply",
                usage:
                  'dossierx comment reply <claim-id> <thread-id> --as human|agent --body "…" [--dry-run]',
                summary:
                  "Reply to an open thread — the agent's move on a human's comment.",
                detail:
                  "Addressed by the engine-generated reply id it echoes, never by ordinal position. Replying to an already-resolved thread is refused. This is the whole of an agent's authority on your thread: it fixes the claim, it replies, and it waits for your Resolve.",
                example:
                  '$ dossierx comment reply logger.internals.event-envelope-fields c-8f3a2b --as agent --body "Fixed the rows; the API facet was stale." --format text\ncomment: reply r-4c9e11 added to thread c-8f3a2b on logger.internals.event-envelope-fields',
              },
            ],
          },
          {
            group: "build-order",
            commands: [
              {
                name: "build-order propose",
                usage:
                  "dossierx build-order propose --module <name> [--dry-run]",
                summary: "Derive the phased implementation sequence.",
                detail:
                  "Writes .build-order.<module>.json (locked:false) split into orientation/schema/behavior/api/verification plus excluded. Refused unless every claim in the module is locked and free of open threads; fails on a same-phase rests_on cycle.",
                example:
                  "$ dossierx build-order propose --module logger --format text\n  schema  5   behavior 11   api 4   verification 6   excluded 2\n  locked: false",
              },
              {
                name: "build-order status",
                usage: "dossierx build-order status --module <name>",
                summary: "Report proposed / locked / stale plus coverage.",
                detail:
                  "Staleness is structural, not a timestamp: status re-derives the order a fresh propose would compute and flags any divergence — reordered, re-roled, promoted, renamed, added or deleted claims. Coverage reads N of M claims covered, K excluded as out-of-scope.",
                example:
                  "$ dossierx build-order status --module logger --format text\n  proposed: true  locked: false  stale: false\n  coverage: 29 of 31 covered (2 excluded)",
              },
              {
                name: "build-order lock",
                usage:
                  'dossierx build-order lock --module <name> --reason "…" [--dry-run]',
                summary: "Freeze the proposed sequence, with your yes on it.",
                detail:
                  "Snapshots a content-hash baseline and stamps locked_at. Refuses if nothing is proposed, or if it is already locked and not stale. --reason is required for the same reason claim lock requires it: a locked build order is a locked artifact, and a second class of locked thing outside the ledger would make “nothing already locked changes” an overclaim.",
                example:
                  '$ dossierx build-order lock --module logger --reason "sequence reviewed 2026-07-26" --format text\nbuild-order lock: logger locked at 2026-07-26T09:14:03Z',
              },
            ],
          },
          {
            group: "the three singles",
            commands: [
              {
                name: "serve",
                usage: "dossierx serve [--port <n>]",
                summary:
                  "The human's one command: the viewer, with a localhost-only comment write-back API.",
                detail:
                  "Binds 127.0.0.1 on a random high port (override with --port), renders the viewer from memory, and exposes the comment operations over a same-origin JSON API — so a reviewer opens, replies to and resolves threads in the browser while the agent works from the CLI. It also serves GET /api/graph, which backs the claims-graph pane's refresh button by rebuilding and re-stamping the graph payload from the current catalog at request time rather than reusing the one baked into the render; over file:// there is nothing to refresh from, so the button is absent. Every request passes Host + Origin admission checks (DNS-rebinding and CSRF defence) and no CORS header is ever sent; the page live-reloads over server-sent events as claim files change. It renders to memory only — never writing viewer/index.html or .catalog.json on a page load — and every claim write goes through the one claims-locked path. It is also the single permanently text-only command: a long-running process cannot be one envelope, and its consumer is the human anyway.",
                example:
                  "$ dossierx serve\nserving: http://127.0.0.1:52431/",
              },
              {
                name: "skills export",
                usage: "dossierx skills export [dir]",
                summary:
                  "Write the binary's embedded agent skills in every form this repo reads — plain markdown, any harness.",
                detail:
                  "One embedded source, written in three forms, because no two harnesses read the same file: a SKILL.md tree (into [dir], else .claude/skills when .claude already exists), an idempotent marker-delimited section spliced into an existing AGENTS.md, and a self-contained agent guide that needs no loader or plugin. Detection, not creation: the tree and the AGENTS.md section only go where the layout already exists, the guide is always written. Where the guide lands follows the project root: in a DossierX project — anywhere project.config.yaml is found — it is docs/dossierx-agent-guide.md beside that config, which is exactly the path the AGENTS.md section's links point at, and the AGENTS.md section is written only in that same case. Run in a directory with no project yet, which is what a bootstrap agent does first, and there is no root to hang docs/ off: the guide is written beside the bundles instead and no AGENTS.md is touched. The AGENTS.md section deliberately carries the ROUTER ONLY — that text is resident on every turn, so inlining all five bundles would bill four skills of context to work that has nothing to do with DossierX. The bundle is the router (the nouns, the envelope, the exit codes, the error-code recovery table, the rules that never bend) plus one companion per workflow: claims, build order, code links, comments. Re-running it is how a project picks up a new release's guidance, so the derived forms are committed artifacts like the ledger.",
                example:
                  "$ dossierx skills export .claude/skills --format text\nskills export: wrote .claude/skills/dossierx/SKILL.md\nskills export: wrote .claude/skills/dossierx-build-order/SKILL.md\nskills export: wrote .claude/skills/dossierx-claims/SKILL.md\nskills export: wrote .claude/skills/dossierx-code-links/SKILL.md\nskills export: wrote .claude/skills/dossierx-comments/SKILL.md\nskills export: wrote ~/myproject/AGENTS.md\nskills export: wrote ~/myproject/docs/dossierx-agent-guide.md\nskills export: skill-tree (claude-code) -> .claude/skills [written]\nskills export: agents-md-section (codex) -> ~/myproject/AGENTS.md [updated]\nskills export: generic-guide (any) -> ~/myproject/docs/dossierx-agent-guide.md [written]\nskills export: wrote 7 file(s)",
              },
              {
                name: "version",
                usage: "dossierx version",
                summary: "Print the binary's version, commit and build date.",
                detail:
                  "Describes the binary itself, so unlike every other command it never loads a project config and runs from anywhere — which is exactly why a bootstrapping agent calls it first. The root command also exposes the equivalent built-in --version flag. Values are ldflag-stamped at release, with a debug.ReadBuildInfo fallback for plain go install builds.",
                // The transcript shows the ONE line this page can derive. The
                // binary prints three — a version line, a "commit:" line and a
                // "date:" line — and the site has a truthful source for only
                // the first.
                //
                // `commit` used to come from a hand-stamped field on the release
                // entry: a value that could not converge (writing the sha is
                // itself a commit, so it was stale the moment it landed), that
                // named the wrong sha for two releases, and that the binary
                // spells as a full 40-character sha where the site carried
                // seven. `date` was no better — the entry's `date` is a calendar
                // day, where the binary prints the RFC 3339 timestamp
                // GoReleaser stamps into `main.date`, so depicting it as the
                // "date:" line asserted output no build produces.
                //
                // Both are dropped rather than approximated: an abridged
                // transcript states less than the truth, where an approximated
                // one states something else. What remains is literal — including
                // the word "version", which this line lost for several releases.
                //
                // It reads `latestVersion`, like every other version string on
                // this page. Until v0.5.2 it read a separate `latestBinaryVersion`
                // with the leading `v` stripped, because the release build stamped
                // `{{.Version}}` and the archive genuinely printed `0.5.1` while
                // `go install` printed `v0.5.1`. The stamp is `{{.Tag}}` now, both
                // paths print the tag as tagged, and there is one spelling again.
                //
                // This is held against a binary linked the way the release links
                // one — but not from here. viewer-tests/site_dom_test.go reads
                // the session out of the RENDERED page and compares it, because
                // four attempts to read this line as SOURCE were defeated
                // without an assertion going red: a prose comment satisfied the
                // search, a commented-out entry satisfied it, and a second
                // declaration after the good one satisfied it while the bundler
                // evaluated the second. A comment renders nothing.
                example:
                  `$ dossierx version --format text\ndossierx version ${latestVersion}`,
              },
            ],
          },
        ],
        // The one place on this page the retired verbs appear. An agent that
        // learned v0.2.0 will reach for them, and a migration table is cheaper
        // than an error message.
        migration: {
          title: "26 → 19: what was cut, and where it went",
          rows: [
            {
              cut: "lint · catalog · render",
              now: "check, and check --validate for a read-only run",
              why: "Stages of check wearing verbs. They existed because the extracted engine had no exported Go API, so every internal package needed a command — a packaging artifact, not a surface.",
            },
            {
              cut: "deps · implink status",
              now: "claim show",
              why: "Both answered part of “what is the state of this claim?”, which took three or four calls to assemble. One call now returns all of it, plus next_actions.",
            },
            {
              cut: "implink set",
              now: "claim link",
              why: "The implink noun disappeared: linking code is something you do to a claim, so it belongs under claim.",
            },
            {
              cut: "stale · coverage",
              now: "claim list --review-pending / --migrated",
              why: "Filters, not verbs.",
            },
            {
              cut: "comment edit · comment delete",
              now: "the viewer only",
              why: "A review history an agent can rewrite is not a review history.",
            },
            {
              cut: "comment resolve · comment reopen",
              now: "the viewer only",
              why: "Advisory rights already refused a CLI agent acting on a human's thread, and every viewer thread is human-authored — so on the CLI these verbs could only ever have acted on the agent's own. Vestigial. They stay on the surface the rights holder actually uses; the localhost API behind it is not authenticated, so this is a smaller surface rather than a closed one.",
            },
            {
              cut: "migrate --adopt",
              now: "no replacement — unlock everything, then re-lock",
              why: "Removed outright in v0.4.0. Adoption is the one operation that manufactures approval out of nothing, and no evidence inside a repository distinguishes an honest pre-ledger store from a downgraded one. A pre-ledger project crosses by holding nothing locked: re-propose any locked build order, unlock every locked claim, then lock only what you still stand behind — and that first lock records a real approval instead of a grandfathered one.",
            },
            {
              cut: "— added —",
              now: "claim show · claim list · claim new · comment inbox",
              why: "Four commands that collapse many calls into one, plus claim new: if hand-editing YAML is gated, an agent needs a sanctioned way to author a claim at all.",
            },
          ],
        },
        // Visual (c): harness independence. Two claims, both true today: the
        // skills are plain markdown that any agent can load, and the ENFORCEMENT
        // is a git hook plus CI rather than anything harness-shaped.
        compat: {
          title: "Any agent. One gate.",
          lede: "DossierX has no harness adapter, and no plan for one. What an agent needs to know is markdown; what stops an unapproved change is git.",
          source: {
            label: "one embedded markdown bundle",
            detail:
              "router skill + four companions, compiled into the binary — no download, no version skew with the CLI it documents",
          },
          targets: [
            {
              harness: "Claude Code",
              path: ".claude/skills/<name>/SKILL.md",
              note: "dossierx skills export .claude/skills — the layout it already reads",
            },
            {
              harness: "Codex",
              path: "AGENTS.md",
              note: "a marker-delimited router section spliced in for you, idempotently — into an AGENTS.md you already have, never one it creates",
            },
            {
              harness: "Pi · Kimi · your own harness",
              path: "docs/dossierx-agent-guide.md",
              note: "always written, all five bundles inline, self-contained — no loader, no plugin, no sibling files",
            },
          ],
          harnesses: [
            "Claude Code",
            "Codex",
            "Pi",
            "Kimi",
            "your editor",
            "a shell script",
          ],
          gate: {
            label: "git commit",
            hook: {
              title: "pre-commit — fast feedback",
              body: "dossierx check --staged over the index, reading content from git show so it never dirties the tree it is judging. Refuses a commit that moves a locked claim without an approval record.",
            },
            ci: {
              title: "CI — the authority",
              body: "Clean merges, rebases, cherry-picks, reverts and --no-verify never fire pre-commit at all. The same check runs in CI against the committed ledger, which is why the ledger is a tracked, committed artifact.",
            },
          },
        },
        globalFlags: [
          {
            name: "--format json|text",
            desc: "json is the DEFAULT — one envelope per invocation. text returns the human prose, byte-for-byte identical to v0.2.0's output, which is what the golden parity fixtures pin.",
          },
          {
            name: "--config string",
            desc: "Path to project.config.yaml. Empty (default) walks upward from cwd like git finds .git; not found → exit 2 with code config_not_found.",
          },
        ],
      },
    },
    {
      id: "versions",
      title: "Release history.",
      kind: "timeline",
      contentMd:
        // Derived, deliberately: an elaboration written by hand here would be a
        // second copy of the newest entry's highlights, which sit directly below.
        `Every change ships as a tagged release with its own changelog entry. The current release is **${latestRelease.version}** — ${lowerFirst(latestRelease.title)}, tagged ${latestRelease.date}. The release ledger reads oldest first, from the engine's public extraction forward, so the current release is the LAST entry on it and the one marked "latest".`,
      data: {
        releases,
      },
    },
    {
      id: "compare",
      title: "Why DossierX over a wiki, ADRs, or a docs/ folder.",
      kind: "comparison",
      contentMd:
        "The tradeoff is deliberate: YAML structure, a required approval record, and confirm-before-write add friction, so DossierX is overkill for notes or narrative writing. It is built for durable contracts, schemas, behaviors and invariants where silently wrong documentation is expensive — and for the case where the thing writing the documentation is a coding agent.\n\n**A wiki optimizes for writing documentation. DossierX optimizes for trusting it.**",
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
            property: "Safe to hand to a coding agent",
            wiki: "scrape HTML, hope",
            adr: "prose, no contract",
            markdown: "prose, no contract",
            dossierx: "19 commands, one JSON envelope, stable error codes",
          },
          {
            property: "Who can change a reviewed statement",
            wiki: "anyone, silently",
            adr: "anyone, silently",
            markdown: "anyone, silently",
            dossierx: "nobody without an approval record in the ledger",
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
            dossierx: "derivable, lockable build-order artifact",
          },
          {
            property: "Enforced in CI",
            wiki: "no",
            adr: "no",
            markdown: "no",
            dossierx: "dossierx check gates the build; a git hook gates the commit",
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
        "Open source, CLI-only, and built in Go. Install it as a pinned tool: `tool github.com/BarterX-Tech/dossierx/cmd/dossierx`. Then run `dossierx skills export`, hand the skills to your agent, and run `dossierx serve` when it asks you to review.",
      data: {
        links: [
          {
            label: "GitHub — BarterX-Tech/dossierx",
            href: "https://github.com/BarterX-Tech/dossierx",
          },
          { label: "Overview", href: "#hero" },
          { label: "Who runs what", href: "#roles" },
          { label: "Review loop", href: "#comments" },
          { label: "CLI contract", href: "#cli" },
        ],
        note: "Page content is grounded in the repository's README, CHANGELOG, FORMAT.md, ROADMAP, embedded skills, and implementation packages.",
      },
    },
  ],
};

/**
 * Leaf commands in the CLI explorer, counted from the explorer's own data.
 *
 * The hero used to hard-code this. It said 20 from v0.3.0 until v0.4.0's
 * surface change made it false, and nothing caught it — it disagreed with the
 * hero badge, the hero role card and the release entry on the same page. This
 * is the same failure the version strings had before they were derived, so it
 * gets the same treatment.
 */
export const commandCount: number = (() => {
  const cli = contentSpec.sections.find((s) => s.kind === "cli-explorer");
  const groups = (cli?.data as { groups?: { commands?: unknown[] }[] } | undefined)?.groups ?? [];
  return groups.reduce((n, g) => n + (g.commands?.length ?? 0), 0);
})();
