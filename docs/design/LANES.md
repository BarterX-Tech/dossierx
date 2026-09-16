# Implementation lanes — viewer design revamp

Partition of the eleven screen specs into implementation lanes.

**Tree this partition was read from:**
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp`, HEAD `8890919`
(*feat(viewer): design-revamp token and font layer*).

**The specs pin their line numbers to `3ac8844`. This file does not.** Every
`style.css` address below was re-derived against the **live worktree** file
(4870 lines) by grepping the selector or marker text, because the token/font
layer that landed in `8890919` shifted the whole file by **+76 to +91 lines**
below line ~230 (`.claim {` moved 227 → 303, `.claim-readiness {` 810 → 898,
`.comment-chip {` 1602 → 1690, `.claim-links {` 4020 → 4108). Where a lane's
spec quotes a number, the lane resolves it **by marker text, then by the live
number in this file, and never by the spec's pinned number.**

## The partition rule

Screens overlap in code. Screens are therefore the **verification** unit and
code sections are the **implementation** unit. A lane owns sections; it verifies
the screens that live in them.

Where one screen's sections are split across lanes, the screen has exactly one
**primary** lane that signs it off end-to-end. Other lanes that paint part of
that screen list it as `(region only)` — they verify their own region on it and
report, but they do not sign the screen off.

No two lanes own the same `style.css` section, the same runtime function or the
same Go emitter. Three responsive containers and the print block are physically
shared; their ownership is resolved by the rules in **Open decisions 1 and 2**,
not by splitting the container between lanes.

## Parked — spec not verified

These specs **failed verification** and get **no lane**. Their exclusive code
sections stay unowned; no lane may edit them. The coordinator re-runs the specs
and re-partitions.

| Spec | Screen | Exclusive sections that stay unowned |
|---|---|---|
| `docs/design/screens/04-issues-screen.md` | 04 · Issues screen | **parked — spec not verified.** The Issues screen does not exist in the engine at any address; nothing is to be built for it. The status strip that is its entry point is owned by **L8** for the *blocked banner* only (screens 02 and 03), not as an Issues screen. |
| `docs/design/screens/06-claim-blocked-across-four-modules.md` | 06 · Claim blocked across four modules | **parked — spec not verified.** Its sections are the readiness expansion (**L5**) and the claim card (**L3**). L5 may not cite 06 as a source; it works from 03 § 4.10–4.11 and 05 § 4.9 only. |
| `docs/design/screens/07-claim-boundary-no-embodiment.md` | 07 · Claim — boundary, no embodiment | **parked — spec not verified.** Its sections are the claim card (**L3**), the footer strip (**L4**) and the `declared_none` conformance branch (**L7**). Those lanes work from 07a, 05 and 06a respectively. |
| `docs/design/screens/13-claims-graph-opened-from-a-claim.md` | 13 · Claims graph | **parked — spec not verified.** `internal/render/viewer/template/graph.css` (1186 lines), `graph-core.js`, `graph-ui.js`, `internal/render/graph_view.go`, `internal/graph/*`, `shell.html:284-323` and the `style.css` graph override block (`style.css:4280-4508`, marker `/* Claims graph: a first-class System Record view… */`) are owned by **no lane** and must not be touched. |

## Lane table

Paper PNGs are under
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/paper-png/`.
Fixture column reads
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/coverage-inventory.md`
(present, dated 2026-09-17 02:28 — **not** pending).

---

### L1 · foundations — tokens, palette, dark blocks, fonts

| | |
|---|---|
| **Owned code sections** | `style.css:1-62` file-header colour-mechanism contract · `style.css:63-167` the one unconditional `:root` (28 palette tokens + `--spacing-*` + type tokens) · `style.css:169-175` the "two dark blocks are the same declarations twice" comment · `style.css:176-200` `@media screen { html[data-theme="dark"] }` · `style.css:202-226` `@media screen and (prefers-color-scheme: dark) { :root }` · `style.css:4748-4808` the print-block doc comment and `style.css:4809-4870` `@media print` (**caretaker**, see Open decision 2) · `internal/render/engine_fonts.go`, `engine_fonts_test.go`, `internal/render/viewer/template/fonts/*` · `internal/config/config.go:154-159` `ThemeTokenAllowlist` · `internal/render/render.go` `themeOverrideCSS` / `writeBlock` (theme emission) · `internal/render/theme_tokens_test.go`, `internal/render/theme_emit_test.go` |
| **Screens verified** | none — this lane paints no screen. It is verified against `docs/design/tokens.md` § 1–§ 7 and by `viewer-tests/theme_modes_test.go` + `internal/render/theme_tokens_test.go`. |
| **Specs to read** | `docs/design/tokens.md` (whole), `docs/design/reference-rules.md` § R08.1 (two hues, no third) |
| **Paper PNGs** | `foundations-01.png`, `components-00.png` |
| **Depends on** | — |
| **States to render** | light · explicit dark (`html[data-theme="dark"]`) · OS dark (`prefers-color-scheme`) · print. The two dark bodies must stay byte-identical. **Every dark value is a `--color-dark-*` token; a light hex appearing in either dark block is a defect to flag in the handoff report, not to copy.** Fixture: `testdata/fixture-theme-flat` (per-token/flat keys — Cutainly sets only `preset: claude`, so flat-key override has **no** client coverage) and `testdata/fixture-theme-preset/fonts` (project-supplied fonts — Cutainly ships none). |

**Intent, in prose:** the palette is light-first because the print block pins
`color-scheme: light` and print must never inherit dark ink. The two dark blocks
exist so that a reader's explicit Dark choice and their OS setting can never
disagree. `--blocked-surface` / `--blocked-hairline` already landed in
`8890919`; the serif family (`tokens.md` Disagreement 6) landed as
`source-serif-4-latin-opsz-wght.woff2`. Confirm both before any lane cites a
serif value as shippable.

---

### L2 · shell-chrome — sidebar, nav, facet TOC, module head, facet strip, measure, focus mode

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:2673-2920` marker `/* ---- sidebar + tab navigation ---- */` (`#nav` 2752, `.sidebar-footer` 2761, `.sidebar-footer-stamp` 2794, `.sec-tab` 2803, `.sec-tab.on` 2818, `.sub-nav` 2849, `.content-area` 2872, soft-mount host 2882) · `style.css:3138-3209` marker `/* System Record visual language… */` + `body.shell-body` + `.sidebar` (3164) · `style.css:3210-3275` `.system-panel-toggle*` and `body.system-sidebar-collapsed` · `style.css:3317-3456` `#nav`, `.sec-tab` (3402), `.sec-tab.on` (3420), `.sidebar-footer` (3450) · `style.css:3457-3482` `.content-area` + the two `min-width: 1181px` blocks · `style.css:3483-3548` `.system-record-head` and `.sub-nav` · `style.css:3756-3878` `.facet-toc` → `.facet-toc__item.on` incl. `body.system-toc-collapsed` · `style.css:4582-4620` `@media (min-width:861px) and (max-width:1180px)` (**the whole block** — every declaration in it is facet-TOC, content-area or sidebar; see OD1) · `style.css:4622-4706` chrome declarations inside `@media (max-width:860px)`, ending at `.subtab { flex: none; }` (see OD1). **Shell:** `shell.html:53-54`, `:61-63`, `:66-73`, `:115-131`, `:133`, `:136-143`. **Runtime:** `system-record.js` `formatGeneratedTime`, `enhanceTimestamp`, `bindResizer`, `bindSidebarCollapse`, `renderToc`, `updateTocActive`, `addModuleHeaders`, `applyThemeChoice`, `bindThemeControl`; `viewer-runtime.js` `showModuleFacet`, `setDrawer`. **Go:** `internal/render/render.go` `Group`/`TabLabel`/`AllLocked` grouping and `GeneratedAt`. |
| **Screens verified** | **02** (primary), **03** (primary) |
| **Specs to read** | `screens/02-reading-view-default.md`, `screens/03-reading-view-focus-mode.md`, `reference-rules.md` § 11 (R11.1–R11.6), § 10 (R10.1–R10.5), § R09.7 |
| **Paper PNGs** | `02-desktop-light.png`, `02-desktop-dark.png`, `02-mobile-light.png`, `02-mobile-dark.png`, `02-mobile-nav-modules-light.png`, `02-mobile-nav-modules-dark.png`, `02-mobile-nav-facet-light.png`, `02-mobile-nav-facet-dark.png`, `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png` |
| **Depends on** | **L1** |
| **States to render** | focus off / focus on (desktop ≥1181) · focus state restored from `localStorage` after reload (R11.5) · sidebar drawer open / closed at 390 · facet-TOC popover open / closed at 390 and its `<select>` form at 861–1180 · module with 1 facet and with 3 facets · theme control System / Light / Dark · freshness line in all three bands (R10.2) and its `Live` form under `dossierx serve` (R10.5). **No fixture needed for any of these** — Cutainly supplies 26 modules, 3 facets, 828 cards, 24 populated tracks. **Gap:** no complete track exists (`track list` → complete=false for all 25), so focus mode's "done" state cannot be shown from the client; fixture `testdata/fixture-graph-demo` (25 locked of 58) or `internal/render/track_view_test.go`. Body markdown is narrow in Cutainly (no tables, images, task lists, blockquotes, links) — those are **not** this lane's sections and must not be styled here. |

**Intent, in prose:** R11.1 says focus is *one reversible state, not two independent
toggles*. The lane therefore deletes the two-toggle affordance
(`shell.html:61-63` + the `.system-panel-toggle--toc` button injected by
`renderToc`) and replaces it with one control, and it renegotiates
`viewer-tests/claim_collapse_test.go` `TestDesktopNavigationPanelsCanCollapseAndExpand`,
which today asserts exactly the two toggles R11.1 forbids. R11.2 is the reason
the reclaimed 282px right gutter must **not** widen the prose measure: the freed
width goes to the evidence (R11.3). The `@media screen and (min-width: 1181px)`
scope on the reclaim is load-bearing for the reason the build-order block's own
comment gives at `style.css:3479-3482` — a print-time reclaim would fight the
print block.

---

### L3 · claim-card — card shell, head line, status chip family, prose typography, collapse

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:303-331` base `.claim {` + the `status-draft`/`status-locked` routing comment · `style.css:332-378` base `.claim-body` (the pre-System-Record `13px` rule) · `style.css:2380-2398` `.k`, `.k > .label`, `.k > .claim-comments-slot` (the "hard right" rule) · `style.css:2399-2430` the dead early `.pill` layer · `style.css:3930-3950` `.card, .claim-tree` + `.claim-card--warn, .claim-banner` · `style.css:3951-4039` the **live** `.k` layer: `.k` 3951, `.k > .label` 3966, `.claim-collapse-toggle` 3974, `.k > .claim-comments-slot:not([hidden])` 3998, `.claim-collapse-chevron` 4019, `.claim-collapse-content[hidden]` 4038 · `style.css:4040-4066` the **live** `.pill` layer: `.pill` 4040, `.pill.ps, .status-locked` 4050, `.pill.pv, .status-draft` 4056, `.pill.pw, .claim-review-pending` 4062 · `style.css:4068-4075` the live `.claim-body, .sbody, .claim-list-items, .claim-table-scroll td` rule · `style.css:1657-1676` `.claim-card--warn` / `.claim-banner` / `.claim-card--commented` left edge · `style.css:4707-4724` the card block inside `@media (max-width:860px)`: `.card, .claim-tree` 4707, `.k` 4713, `.k > .claim-comments-slot:not([hidden])` 4715, `.pill` 4720, `.claim-body, .sbody, .claim-list-items` 4722 (see OD1). **Runtime:** `system-record.js` `enhanceClaimDisclosures`, `cleanTitle`. **Go:** `internal/render/components/card.html:38-43` and the identical `.k` lines in `list.html`, `tree.html`, `steps.html`, `table.html`; `internal/render/components/components.go` `pillClass`, `StatusLabel`. |
| **Screens verified** | **07a** (primary) · 02 (card + chip region only) · 05 (card shell region only) |
| **Specs to read** | `screens/07a-claim-draft-not-yet-approved.md`, `screens/02-reading-view-default.md` § 4.11–4.13 and § 4.20, `reference-rules.md` § R-H.2 (status chip above the title at 390) |
| **Paper PNGs** | `07a-desktop-light.png`, `07a-desktop-dark.png`, `07a-mobile-light.png`, `07a-mobile-dark.png`, `components-00.png` (chip specimens) |
| **Depends on** | **L1** |
| **States to render** | LOCKED chip · DRAFT chip · review-pending chip (`.pill.pw`) · warn/blocked card (`.claim-card--warn`) · commented card left edge (`.claim-card--commented`) · collapsed / expanded claim body · `:target` deep-link landing. **Fixtures:** `status: published` (legacy) has **no** Cutainly instance → `testdata/fixture-coverage/lint/status-shape` (1 claim). `drifted` and `migrated` have none → `internal/lock/lockedhash_test.go`, `internal/lock/downgrade_test.go`, `internal/lock/ledger_test.go`. Layouts `table` / `list` / `tree` / `banner` / `mockup` have none → `testdata/fixture-basic`, `testdata/fixture-portability`, `testdata/fixture-theme-flat`, `testdata/fixture-coverage`; `layout: tree` has **no YAML fixture anywhere** and is constructed in Go only (`internal/render/render_test.go`). Cutainly supplies `card` (777) and `steps` (51), draft (794), locked (34), review_pending (395). |

**Intent, in prose:** the chip is a state badge, not a decoration — it exists so a
reviewer can tell, without reading, whether the claim is a commitment or a
proposal. That is why the DRAFT form carries **no padlock**: a draft is not
locked, and reusing the locked glyph would say the opposite of the truth. The
board's approval of a chip does not make its glyph right — check every glyph
against what the state means, and record a mismatch as a Paper defect in the
handoff. The `.k > .claim-comments-slot` rule is owned here because it is head
*layout*; the chip **component** is L6's. `system-record.js` deliberately skips
the comment slot when it rebuilds the head into a `<button>` (a control inside a
control is not a control) and strips `.pill, .claim-comments-slot` out of derived
titles — both behaviours must survive.

---

### L4 · footer-strip — evidence & relationships strip, edges, sources, expansion exclusivity  ★ PILOT

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:877-893` the `.claim-links` structural doc comment + first-layer rule · `style.css:1272-1300` the `.claim-links-summary` doc-layer comment and rule · `style.css:1301-1346` deep-link auto-open (`.claim:target .claim-links…` 1331/1335) · `style.css:1347-1437` `.claim-edges` list, `li::before`, `.claim-ref*` · `style.css:1438-1643` marker `/* ---- the sources row (components/sources.go) ---- */` (`.claim-source-list` 1447 → `.claim-source:target` 1630) · `style.css:3376-3392` `.claim-footer__chevron` + `.claim-links[open]` rotation · `style.css:4108-4151` the **live** footer: `.claim-links` 4108, `.claim-links-summary` 4116, `.claim-footer__identity` 4136, `__title` 4138, `__counts` 4141, `__chevron` 4151 · `style.css:4192-4223` the live `.claim-source-list` overrides · `style.css:4726-4729` footer declarations inside the 860 block and `style.css:4735-4747` the **whole** `@media (max-width:560px)` block (see OD1). **Runtime:** `system-record.js` `enhanceFooters`, `enhanceFieldLabels`. **Go:** `internal/render/components/components.go` `EdgesHTMLWithLinks` and its doc block, the per-relation `<li>` emitters, the `<summary>` digest, `countSegment`, the two server-side auto-open signals; `internal/render/components/sources.go` `writeSourcesRow`, `writeExternalSource`, `writeInternalSource`; `internal/render/depended_by_view.go` `buildDependedByLookup`, `buildTargetStatusLookup`, `attachEdgesOverride`. |
| **Screens verified** | **05** (primary) · 07a (footer region only) · 02 (footer strip region only) |
| **Specs to read** | `screens/05-claim-one-expansion-at-a-time.md`, `screens/02-reading-view-default.md` § 6 (footer vocabulary), `screens/07a` § 6 and § 7.6, `reference-rules.md` § R-F.1–R-F.4, § R09.1, § R09.2, § R09.4, § R09.5, § R-I.2, § R-I.3 |
| **Paper PNGs** | `05-desktop-light.png`, `05-desktop-dark.png`, `05-mobile-light.png`, `05-mobile-dark.png`, `02-desktop-light.png` / `02-desktop-dark.png` (footer at rest), `07a-desktop-light.png` / `07a-desktop-dark.png` (bare-text footer form), `components-00.png` |
| **Depends on** | **L1**, **L3** |
| **States to render** | strip closed · strip open · blocked form (R-F.2) · each of the four chips at zero and at N · comment count hard right at zero and at N · mobile two-row form at 390, with no pill splitting across rows (R-F.4) · exactly one expansion open at a time (R09.2) · `:target` auto-open · server-written auto-open (drifted file, locked + review_pending). **Fixtures:** `drifted` has **no** Cutainly instance (`claim list --review-pending` → 0 of 395 drifted) → `internal/lock/lockedhash_test.go`, `internal/lock/downgrade_test.go`. `mirrors` has none (0 claims) → `testdata/fixture-coverage` (11 claim files) + `lint/mirror-reciprocal`, `lint/mirror-unanchored`, `lint/mirror-mismatch`. Internal source rows (path + sha256) have none (0 of 781 rows) → `testdata/fixture-coverage/lint/source-internal-drift`, `lint/source-shape`. `record_id`-narrowed internal source: same fixture. Cutainly supplies: 801 claims with `rests_on`, 525 real `governed_by` targets, 303 literal `governed_by: none`, 277 claims with sources (781 external rows), 1,562 notes of which 1,175 exceed the 3-line clamp, 202 one-`rests_on` claims for the one-expansion screen. |

**Intent, in prose:** R09.1 is the whole lane in one sentence — *four expansions of
one strip, never four panels*. The footer is a single strip that opens in place,
because four sibling panels would let a reviewer lose track of which claim they
were reading. R09.2 follows from it: two open expansions make the card taller
than the viewport and the claim's own text leaves the screen. Today the engine
disagrees with the board at every point: the summary is a mono count strip
(`"1 link - 2 files - 1 source - 1 drifted"`) rewritten in the browser into
`relationship` / `source` / `file` / `drifted` chips. **Board 05/07a's vocabulary
— Blocked / relationships / sources / checks, each a noun and a count, comment
count hard right — exists at no address in the engine.** This is new surface, not
restyling; size it accordingly. The live footer is a bordered, radiused, tinted
box (`style.css:4108-4115`); the board replaces it with an inset hairline inside
the claim card — no second border, no second radius. That is a deletion, and it
is the change the frozen theme-parity baselines
(`viewer-tests/testdata/theme-parity/baseline-flat.html`) will report; report the
parity failure, do not fix it.

---

### L5 · readiness — the blockers expansion

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:895-1246` marker `/* Claim readiness is a reviewer-facing work queue… */` — the entire readiness surface (`.claim-readiness` 898 through `-raw`) · `style.css:1248-1262` `@media (max-width: 860px)` readiness block (**whole block**) · `style.css:1263-1270` `@media (max-width: 520px)` readiness block (**whole block**). **Runtime:** `viewer-runtime.js` `renderClaimReadiness`, `readinessFactRow`, `readinessRoute`, `readinessHopLabel`, `readinessClaimLabel`, `readinessFactLabel`, `readinessPathDetails`, `readinessMermaidSource`, `readinessRawDiagnostics`, `offlineReadiness`, and the two `renderClaimReadiness` call sites. **Go:** `internal/render/render.go` readiness payload plumbing (`HasReadinessMaps` and the fields feeding it). |
| **Screens verified** | **none signed off** — 06 and 07 are **parked — spec not verified**. Region-only: 03 § 4.10–4.11 (readiness expansion, verified spec), 05 § 4.9 (readiness blockers expansion, verified spec). |
| **Specs to read** | `screens/03-reading-view-focus-mode.md` § 4.10–4.11 and § 7.4, `screens/05-claim-one-expansion-at-a-time.md` § 4.9, `reference-rules.md` § R09.3, § R09.8, § R09.9, § R-I.1. **Do not read 06 or 07.** |
| **Paper PNGs** | `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png`, `05-desktop-light.png`, `05-desktop-dark.png`, `05-mobile-light.png`, `05-mobile-dark.png`. **Not** `06-*.png` — parked. |
| **Depends on** | **L1**, **L4** |
| **States to render** | ready · locally approved but chain not ready · not approved but chain ready · not approved and chain not ready · zero blockers · auto-open when and only when blocked (R09.3) · stacked blocker row at 390 with the dependency path stacked and never dropped (R-I.1). **Fixtures:** only **one** blocker kind exists in Cutainly — `dependency_unapproved`, 31,673 instances. The other five conditions have no client instance: `missing_dependency` → `testdata/fixture-coverage/lint/dangling` + `internal/readiness/readiness_test.go`; `unreadable_dependency` → `internal/readiness/readiness_test.go`; `retired_dependency` → `testdata/fixture-coverage/lint/supersede`; `unknown_historical_baseline` → `internal/readiness/readiness_test.go`; `dependency_cycle` → `testdata/fixture-coverage/lint/cycle`. Review causes `own_flag`, `direct_dependency_change`, `approval_content_drift` → `internal/readiness/readiness_test.go` / `internal/lock/lockedhash_test.go`. **`approval_missing`, `approval_released` and `approval_unknown` have no fixture anywhere** — they exist only as `internal/lock` unit assertions that never reach a rendered viewer; do not attempt to render them, report them as not rendered. Cutainly supplies: 3 ready claims, 31 locally-approved/chain-not-ready, 38 not-approved/chain-ready, 756 not-approved/chain-not-ready, 41 zero-blocker claims, 202 one-`rests_on` claims. |

**Intent, in prose:** readiness is a *work queue*, not a diagnostic dump — the
reviewer should learn what to do next, not what the policy engine computed.
R09.9 removes the inline Mermaid dependency map because a picture of a graph
inside a card is a graph nobody can read; the breadcrumb and the graph pane carry
it instead. R09.8 demotes the raw JSON envelope and cuts the phrase
`shown via this dependency` for the same reason: eight demotions, and **nothing is
deleted from the data**. Removing the map moves the subject of
`viewer-tests/claim_readiness_test.go` `TestReadinessMapCapNeverCapsTheAuthoritativeList`
and the `Showing 12 of 14` caption assertion — report those, do not paper over
them.

---

### L6 · comments — chip, rail, bottom sheet, threads, composer

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:1677-1861` marker `/* ---- comments (engine-managed review threads) ---- */` (`.comment-chip` 1690, `--open` 1705, `--empty` 1731, `-count` 1760, `.comments-panel` 1772, `.comment-thread` 1794, `.comments-resolved > summary` 1841, and the nested coarse-pointer block) · `style.css:1862-2099` marker `/* ---- interactive comment UI (Phase 5, serve + file://) ---- */` (`.comments-overlay` 1881, `.comments-rail` 1896, `-head` 1917, `-title` 1927, `-close` 1940, `-body` 1952, `.comment-action` 1994, `.comment-thread-actions` 2025, `.comment-composer` 2040, `-input` 2051, `-submit` 2069, `.comments-toast` 2080) · `style.css:2298-2314` `@media (pointer: coarse)` (**whole block**, incl. the `.claim-collapse-toggle` and `.status-strip-head` entries in its selector list — L3 and L8 are dependents, see OD3) · `style.css:2315-2332` `@media (max-width: 860px)` sheet block (**whole block**) · `style.css:4224-4263` the live `.comment-chip` / `--open` overrides · `style.css:4264-4274` the live `.comments-rail` / `-head` overrides · `style.css:4510` `.comments-rail-close:focus-visible`. The 860-container chip-slot rule at `style.css:4715-4718` is **L3's**, not this lane's — it is head layout, not the chip component. **Runtime:** `viewer-runtime.js` rail element handles, `chipsFor`, `setChipExpanded`, `updateChips`, `syncEmptyChips`, `recomputeChipsFromPanel`, `commentPanelOpen`, `openCommentPanel`, `closeCommentPanel`, `renderPanel`, `renderPanelReadOnly`, `renderPanelFromAPI`, `buildPanel`, `syncEmptyLine`, `buildThread`, `buildMessage`, `buildThreadActions`, `iconButton`, `growNow`, `autoGrow`, `buildComposer`, `buildReplyComposer`, `threadNode`, `doReply`, `doResolve`, `doReopen`, `doDelete`, `startEdit`, the chip click delegation, the scrim/close/Escape wiring and the SSE body classes. **Shell:** `shell.html:242-250`. **Go:** `internal/render/components/components.go` `CommentChipHTML`, `commentsPanelTmpl`, `commentsPanelView`, `newCommentsPanelView` and the panel-append site; `internal/render/components/comments.html`. |
| **Screens verified** | **14** (primary), **14a** (primary) |
| **Specs to read** | `screens/14-comments-rail-open-on-a-claim.md`, `screens/14a-comments-states-and-placement.md`, `reference-rules.md` § R-J.1–R-J.7 (bottom-sheet policy), § R-F.3, § R10.1/R10.4 (elapsed time) |
| **Paper PNGs** | `14-desktop-light.png`, `14-desktop-dark.png`, `14-mobile-light.png`, `14-mobile-dark.png`, `14a-desktop-light.png`, `14a-desktop-dark.png`, `14a-mobile-light.png`, `14a-mobile-dark.png` |
| **Depends on** | **L1**, **L3** |
| **States to render** | empty (state 1) · open thread (state 2, with reply) · resolved thread · read-only `file://` (state 3, no composer) · chip variants `--empty` / `--open` / `--resolved` at 0 and N · desktop non-modal rail (no backdrop, no scroll lock) · mobile modal sheet (backdrop + scroll lock) · composer pinned and never shrinking · edited message. **Fixtures:** `resolved thread` has **no** Cutainly instance (all 10 threads are `status: open`) → `testdata/fixture-theme-flat/claims/widget-reviewed.yaml` or `viewer-tests/viewer_test.go TestUIResolveCollapsesThreadAndUpdatesChip`. `human-authored message` has none (10/10 authors are `agent`) → `testdata/fixture-coverage/lint/comments-unresolved/claims/commented.yaml`, `testdata/fixture-graph-demo/claims/engine-lint-runs-before-catalog.yaml`. `edited message` has none (`edited: false` everywhere) → `viewer-tests/viewer_test.go TestUIEditMarksEditedAndPersists`. Cutainly supplies: 10 open threads on 9 claims, 10 replies, 819 zero-thread chips, and `audience-boundary.internals.invalidation-precedes-publication` with two threads on one claim. |

**Intent, in prose:** R-J.1 — *there is one bottom sheet in this product, not
several.* The rail and the sheet are one shell in two shapes, which is why the
`matchMedia('(max-width: 860px)')` role/`aria-modal` swap in the runtime must stay
aligned with the CSS breakpoint: a modal that the CSS thinks is non-modal is a
trap for a screen-reader user. R09.8 governs the header: *the reviewer recognises
the title; the slug is for the agent* — today the header is one line carrying the
slug, and the boards want two lines carrying `Comments` and `on <claim title>`.
`{{.Created}}` is printed raw into `comments.html`, so the absolute stamp still
leaks into the HTML; R10.4 requires the elapsed phrase to be computed in the
browser from the `datetime` attribute. **Restyle these selectors; do not rename
them** — `.comments-rail*`, `.comment-chip*`, `.claim-comments-slot`,
`.comment-thread`, `.comment-composer*`, `.comments-resolved`, `.comments-panel`
and the body classes `comments-open` / `comments-live` are asserted across
`internal/render/components/comments_test.go`, `comments_render_test.go` and six
viewer-test files.

---

### L7 · conformance — "How this was checked" disclosure

| | |
|---|---|
| **Owned code sections** | **Go (this panel's stylesheet is a Go string, not in `style.css`):** `internal/render/conformance_view.go` — the `conformanceCSS` constant in full, `viewerCSSWithConformance`, `conformanceStatusFetchGuardJS` · `internal/render/components/conformance.go` — `ConformanceHTML`, `writeConformanceCheck`, `conformanceStateLabel`, `conformancePill`, `writeConformanceValue`, `writeConformanceLine`, `writeConformanceMembers` · `internal/render/render.go` the conformance insertion point `insertEngineBlockBeforeClose` and the two `if` guards that append the CSS and the status-fetch guard. **Shell:** `shell.html:313-314` the guard injection point (order relative to `{{.ViewerRuntimeJS}}` is load-bearing). **Runtime:** `viewer-runtime.js` `conformanceNotReadyIDs` — the only runtime code that touches the panel. **CSS:** none. `grep -c conformance internal/render/viewer/template/style.css` returns **0**; if this lane needs a `style.css` rule it opens a new block appended per OD1 and says so in its report. |
| **Screens verified** | **06a** (primary) |
| **Specs to read** | `screens/06a-how-this-was-checked-inline-disclosure.md`, `reference-rules.md` § 12 (R12.1–R12.4), § R09.3 |
| **Paper PNGs** | `06a-desktop-light.png`, `06a-desktop-dark.png`, `06a-mobile-light.png`, `06a-mobile-dark.png`, `ref-12.png` |
| **Depends on** | **L1**, **L4** |
| **States to render** | disclosure closed · disclosure open · verdict `Matched` · `Mismatch` · `Uncheckable` · `Owed` · `mode: none` / `declared_none` (the `declaration: none` + `reason:` branch) · shape `scalar` · shape `set` with missing and extra members · `implementation_ready = false` auto-open (R09.3, already implemented at `conformance.go` lines 28-30) · ready panel renders **closed** and stays inside a collapsed claim. **Fixtures:** `check state: owed` has **no** Cutainly instance (summary.owed = 0) → `testdata/fixture-conformance-v1/claims/owed.yaml`. `check state: uncheckable` has none → `testdata/fixture-conformance-v1/claims/uncheckable.yaml` + `observations.json widget://uncheckable`. `conformance.blocking: true` has none (the client sets `blocking: false`, so **the refusing form of the disclosure cannot be shown from Cutainly**) → `internal/serve/conformance_blocking_test.go`, `internal/check/conformance_staged_test.go`. Cutainly supplies: 23 declared, 34 checks, 32 matched, 2 mismatch, 6 `mode: none`, 17 `mode: compare`, 31 set-shape, 3 scalar-shape, 2 `implementation_ready = false`. |

**Intent, in prose:** R12.1 — *a snippet lives behind "How this was checked", never
inline.* The point of the disclosure is that the machine layer is available on
demand and never in the reading path: a reviewer reading a claim should not have
to scroll past nine `key: <code>value</code>` rows to reach the next claim. None
of the board's structure exists today: there is no nested disclosure, no
`EXAMINED` / `COMPARED` / `FOUND` layer, no coverage step strip, no snapshot hash
and no `Copy as JSON`. `writeConformanceLine` — one function emitting
`<p class="claim-conformance-line"><span>LABEL:</span> <code>VALUE</code></p>` — is
the whole machine layer today. **Two traps.** (1) `viewer-tests/conformance_test.go`
reads every `.claim-conformance-line`'s trimmed `textContent`; any change to the
row markup breaks it by construction — report it. (2) `theme_parity_test.go`
suppresses `.claim-conformance` with `display: none !important`, so **parity will
not catch a colour regression in this panel**; verify light and dark by eye
against the PNGs, and express every dark value as a `--color-dark-*` token. The
class name `claim-conformance` is asserted as a literal string across **four
packages** (`internal/render`, `internal/serve`, `internal/check`, `viewer-tests`)
— renaming it is a four-package change and out of this lane's budget unless the
report says otherwise.

---

### L8 · blocked-banner — the status strip as the reading view's blocked notice

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:2100-2297` marker `/* ---- status strip (lock-ledger integrity + lint) ---- */` incl. the z-index ledger comment, `.status-strip` 2147, `[hidden]` 2157, `--lint` 2161, `--integrity` 2166, `-head` 2172, `-summary` 2185, `-title` 2191, `-note` 2197, `-action` 2203, `-caret` 2211, `--open .status-strip-caret` 2218, `-body` 2224, `-filters` 2235 · `style.css:4275-4279` the live `.status-strip` override. **Runtime:** `viewer-runtime.js` `blockerHeadline`, `addStatusGroup`, `collectStatusGroups`, `statusGroupClaimCount`, `uniqueClaimCount`, `countSeverity`, `statusChipLine`, `renderStatusStrip`, `setStripExpanded`, `refreshStatus`, and the offline fallback. **Shell:** `shell.html:272-281` (`#statusStrip`, and the literal action label `Show issues`). |
| **Screens verified** | **none signed off** — 04 is **parked — spec not verified**. Region-only: 02 § 4.10 and § 4.19 (blocked banner, desktop and the 390 inset-card form), 03 § 4.5 (blocked banner). |
| **Specs to read** | `screens/02-reading-view-default.md` § 4.10, § 4.19 and § 6 (facet-level banner), `screens/03-reading-view-focus-mode.md` § 4.5 and § 7.5, `reference-rules.md` § R08.1, § R08.2, § R09.6, § R-H.1. **Do not read 04.** |
| **Paper PNGs** | `02-desktop-light.png`, `02-desktop-dark.png`, `02-mobile-light.png`, `02-mobile-dark.png`, `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png`. **Not** `04-*.png` — parked. |
| **Depends on** | **L1**, **L2** |
| **States to render** | strip hidden (nothing actionable) · lint variant (`--lint`) · integrity variant (`--integrity`) · collapsed · expanded (`Show issues` / `Hide issues`) · 390 inset-card form (R-H.1: inset card, **not** a full-bleed band) · offline fallback. **Fixtures:** `lint ERROR` findings have **no** Cutainly instance (`lint_error_count = 0`), so **the error arm of the two-hue severity vocabulary (R08.1) cannot be shown from the client** → `testdata/fixture-coverage/lint/<rule>/` (38 single-rule fixtures). `ledger findings` have none → `internal/lock/audit_test.go`, `ledger_test.go`, `downgrade_test.go`. `code-scan errors` have none (`scan_files_scanned = 0`) → `testdata/fixture-coverage/lint/code-orphan`, `internal/render/implink_view_test.go`. Severity chips are **partial**: warning only. Cutainly supplies: 11 lint warnings across 3 rules (comments-unresolved 9, body-edge-hint 1, track-empty 1), 28 `next_steps` entries, 2 conformance mismatches. |

**Intent, in prose:** R09.6 — *Issues is a screen, and the banner is the only way
in.* This lane builds **only the banner**, because the screen it leads to (04) is
parked. R08.2 is the constraint that makes the two variants non-cosmetic: a lint
finding and a ledger finding must never read alike, because one is an author's
mistake and the other is evidence the lock ledger cannot be trusted — a reviewer
who confuses them draws the wrong conclusion about the whole project. R08.1 caps
the vocabulary at two hues; the client can only exercise one of them, so the
error arm must come from a fixture or be reported as not rendered. The strip's
80/81 z-index neighbours belong to the graph pane and are **parked** — do not
renumber the ledger.

---

## Recommended pilot: **L4 · footer-strip**

The brief asks for the lane covering screen 07 or 13. Both of those specs failed
verification: 13's sections (`graph.css`, `graph-ui.js`, `graph-core.js`, the
`style.css` graph override block) are owned by no lane at all, and 07's sections
are split across L3, L4 and L7. **L4 is the lane that covers 07's decisive
surface.** Screen 07's § 4.7/§ 4.8 footer strip — four chips in a fixed order with
the comment count hard right, in a desktop bare-text form and a mobile two-row
form — is the same component 05 (verified) and 07a (verified) draw, so the pilot
can be verified against a spec that passed while still being the thing 07 is
mostly made of. It is also the right pilot on its own merits: it is the largest
genuinely **new** surface in the revamp (07a § 7.6 records that the board's
footer vocabulary "does not exist in the engine at any address"), it is the
shared chrome that R-F.1–R-F.4 and R09.1/R09.2 all bind, three other lanes
(L5, L7, and L6's mobile row-2 composition) hang off its strip, and it is the
first place the team will find out how expensive the frozen theme-parity
baselines make a visual change — which is exactly the number the remaining waves
need before they commit. Running it first buys the coordinator the footer
vocabulary, the expansion-exclusivity mechanism and a real parity cost estimate
in one lane.

## Wave plan

Waves of at most 5. Lanes inside a wave share no section, no runtime function and
no Go emitter. Every lane appears after every lane it depends on.

| Wave | Lanes | Why they are independent |
|---|---|---|
| **1** | `L1` | Everything reads the palette. Nothing else may start: a lane that measures against a stale token measures nothing. |
| **2** | `L2`, `L3`, `L8` | L2 owns the sidebar/TOC/measure sections, L3 the card/head/pill sections, L8 the status-strip section. Disjoint in `style.css`, in the runtime and in the Go emitters. They meet only inside the 860 media container, resolved by OD1. |
| **3** | `L4` ★ pilot, `L6` | Both need L3 (L4 lives inside the card, L6's chip rides the `.k` head). Disjoint from each other: L4 owns the footer's *composition*, L6 owns the chip *component* (R-I.3). |
| **4** | `L5`, `L7` | Both need L4's strip to exist before they can be an expansion of it. L5 is `style.css:895-1270` + the readiness runtime; L7 is entirely Go (`conformance.go`, `conformance_view.go`) with no `style.css` section at all. |

```
wave 1:  L1
wave 2:  L2   L3   L8
wave 3:  L4★  L6
wave 4:  L5   L7
```

## Lane contract

Every lane follows these seven steps, in order.

1. **Read `LEARNINGS.md`, your spec file(s), `tokens.md`, your Paper PNGs.
   Nothing else.** Not other lanes' specs, not the parked specs, not the other
   lanes' code sections.
2. **Implement.** Keep every non-parity test green. **Parity failures are
   reported, not fixed** — a regenerated baseline hides the change the reviewer
   is being asked to approve.
3. **Serve the Cutainly project copy via the local-client runner; screenshot
   1440 and 390, both themes, scrolling to capture below the fold.**
4. **Re-read `LEARNINGS.md` before handoff** — it will have moved while you
   worked.
5. **A verifier re-renders independently and compares against the spec AND the
   PNG.** Not against your screenshots.
6. **Learnings → vault `learnings/inbox/<lane>.md`; dead-code candidates → vault
   `learnings/deadcode/<lane>.md`.**
7. **Report: matched / changed / could not do and why / states not rendered.**

Two standing obligations that sit across all seven steps:

- **A value you did not read from Paper (`get_computed_styles` / `get_jsx` /
  `get_tokens`) is not a value.** Do not estimate from a screenshot. Screenshots
  are for intent and for spotting what to measure.
- **Record design intent in prose next to every measured value.** A board that
  shipped a wrong glyph is still wrong; approval is not proof. Dark boards are
  expressed as `--color-dark-*` tokens, and a light hex on a dark board is a
  defect to flag in step 7, not a value to copy.

## Open decisions

Recorded rather than asked. The coordinator may overrule any of them.

**OD1 — Shared responsive containers are owned line-by-line, and new mobile rules
go in a lane's own appended block.** Three containers are physically shared:
`@media (min-width: 861px) and (max-width: 1180px)` (`style.css:4582-4620`),
`@media (max-width: 860px)` (`style.css:4621-4734`) and `@media (max-width: 560px)`
(`style.css:4735-4747`). Splitting a container between two lanes invites a merge
conflict on every line. **Decision:** the 861–1180 container
(`4582-4620`) is **wholly L2's** — every declaration in it is facet-TOC,
content-area or sidebar. The 860 container splits three ways: `4622-4706` L2
(chrome, ending at `.subtab { flex: none; }`), `4707-4724` L3 (card, `.k`, the
chip slot, `.pill`, `.claim-body`), `4726-4729` L4 (footer counts). `4730-4732`
(`.track-head`, `.track-title`, `.track-cite`) belongs to the track sections and
is **unowned** — no lane may edit it. The 560 container (`4735-4747`) is wholly
L4's. **Any lane
adding a new mobile rule adds it in its own new `@media` block appended after
`style.css:4747` and before the print doc comment at `4748`, tagged with a
`/* lane: Lx */` comment** — never by editing a shared container in place. This
is also what makes the new rules win: equal specificity, later source order.

**OD2 — L1 is the caretaker of the single print block; other lanes request print
rules through it.** `style.css:4809-4870` is the one `@media print` block and it
must stay last in the file, because it beats screen layout rules at equal
specificity on source order alone. Six lanes editing it independently would break
that invariant and `internal/render/theme_tokens_test.go
TestStyleCSSModeAndPrintStructure` with it. **Decision:** L1 owns the block. A
lane needing a print rule states it in its step-7 report; L1 lands it.

**OD3 — The two `@media (pointer: coarse)` blocks go to L6, with L3 and L8 as
dependents.** `style.css:2298-2314` sets `min-height: 44px` on seven controls in
one selector list — five of them comments', plus `.claim-collapse-toggle` (L3)
and `.status-strip-head` (L8). **Decision:** L6 owns both coarse blocks intact;
L3 and L8 add new coarse rules in their own appended blocks per OD1 and must not
edit the selector list. The 44px minimum is not a style preference — it is the
smallest target a thumb hits reliably, and it is the existing precedent every
mobile rule in the revamp extends.

**OD4 — A screen is signed off by exactly one lane; other lanes verify their
region only.** 02 is signed off by L2 even though L3, L4, L6 and L8 all paint on
it. The alternative — every lane that touches a screen signing it off — produces
four contradictory verdicts on the same board and no owner. The primary lane is
the one owning the sections that carry the screen's argument.

**OD5 — L5 and L8 ship without a signed-off screen, and that is deliberate.**
Both lanes' home screens (06/07 and 04) are parked. Both own sections that
verified screens (03, 05, 02) depend on, so they cannot simply wait. **Decision:**
they implement against the verified screens' region specs only, and their step-7
report states explicitly which board they could **not** verify against and why.
When the coordinator re-runs 04/06/07, those screens attach to these existing
lanes rather than spawning new ones — the sections are already owned.

**OD6 — No lane owns the markdown body constructs.** `style.css:379-780` (phase B
block constructs, phase C GFM tables, phase D images, inline citation markers) is
owned by no lane. No verified screen specifies it, and the coverage inventory
records that Cutainly's bodies contain no tables, images, task lists, blockquotes
or links, so nothing in that range can be verified against the client at all.
L3 owns the *prose* rules (`.claim-body` base at `332-378` and the live
`.claim-body, .sbody, …` rule at `4068-4075`) and nothing below 379.

**OD7 — `graph.css` and the `style.css` graph override block are frozen.** Screen
13 is parked, so `internal/render/viewer/template/graph.css`,
`graph-core.js`, `graph-ui.js`, `internal/render/graph_view.go`,
`internal/graph/*` and `style.css:4280-4508` (marker `/* Claims graph: a first-class System Record view, not a utility overlay. */` at `4280`) are owned by no lane and are
off-limits. The one crossing is that `style.css` re-declares `--dxg-facet-other`
inside the palette blocks L1 owns (`style.css:76`, `:189`, `:215` region) — that
declaration is L1's and wins over `graph.css`; L1 changes it only if `tokens.md`
§ 2 requires it, and says so in its report.

**OD8 — The coverage inventory is present and is the fixture source of truth.**
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/coverage-inventory.md`
exists (2026-09-17 02:28) and every fixture named in the lane table is quoted
from it. Its own caveat stands and is passed through to every lane: **nothing in
it was rendered in a browser.** It is drawn from `catalog.json`, the CLI
envelopes and the claim YAML. A state marked "yes" means the data exists, not
that any card was seen to draw it. Step 3 of the lane contract is where that gets
established.
