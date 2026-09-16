# Implementation lanes — viewer design revamp

Partition of the eleven screen specs into implementation lanes. **All eleven
specs are ACCEPTED as verified.** The residual verifier findings against 04, 06,
07 and 13 were node-citation and line-drift errors, not value errors: nothing
was parked because a measurement was wrong. There is no parked spec and no
unowned screen.

**Tree this partition was read from:**
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp`, HEAD `f576ecc`
(*docs(design): freeze the viewer design revamp specs, tokens, reference rules
and lanes*), whose parent `8890919`
(*feat(viewer): design-revamp token and font layer*) is the last commit that
touched `internal/render/viewer/template/style.css` (4870 lines).

## The partition rule

Screens overlap in code. Screens are therefore the **verification** unit and
code sections are the **implementation** unit. A lane owns sections; it verifies
the screens that live in them.

Where one screen's sections are split across lanes, the screen has exactly one
**primary** lane that signs it off end-to-end. Other lanes that paint part of
that screen list it as `(region only)` — they verify their own region on it and
report, but they do not sign the screen off.

No two lanes own the same `style.css` section, the same runtime function or the
same Go emitter. Four responsive containers, two `(pointer: coarse)` blocks and
the print block are physically shared; their ownership is resolved by **OD1**,
**OD2** and **OD3**, not by splitting a rule between lanes.

**Every line number in every screen spec is pinned to `3ac8844`, and every one
of them is stale.** The token/font layer that landed in `8890919` shifted
`style.css` by **+76 to +91 lines** below line ~230 (`.claim {` moved 227 → 303,
`.claim-readiness {` 810 → 898, `.comment-chip {` 1602 → 1690, `.claim-links {`
4020 → 4108), and `internal/render/geist_fonts.go` was replaced by
`engine_fonts.go`. The numbers in **this** file were re-derived against the live
worktree at `8890919`/`f576ecc` and are correct today — but a lane branches from
**integration HEAD**, which moves as lanes land. Every lane therefore carries a
**Section markers** row: the quoted marker comment or first selector for each
`style.css` range it owns. **A lane resolves an address by marker text at its
own HEAD, then by the live number in this file, and never by the spec's pinned
number.**

## L1 — satisfied by `8890919`, not a lane

The foundations work has shipped. Commit `8890919` landed, in
`internal/render/viewer/template/style.css` and `internal/render/`:

- the one unconditional light-first `:root` (`style.css:63-167`) — the 28
  allowlisted palette tokens plus the engine-owned `--font-serif`,
  `--radius-sm`, `--radius-pill`, `--blocked-surface`, `--blocked-hairline`, the
  eight `--text-*`, three `--leading-*`, two `--tracking-*` and seven
  `--spacing-*` tokens;
- the "two dark blocks are the same declarations twice" comment
  (`style.css:169-175`) and both dark bodies —
  `@media screen { html[data-theme="dark"] }` (`176-200`) and
  `@media screen and (prefers-color-scheme: dark) { :root }` (`202-226`) —
  byte-identical, including the new `--border-strong`, `--status-draft`,
  `--status-draft-bg`, `--blocked-surface` and `--blocked-hairline` twins;
- the five inlined faces — `internal/render/engine_fonts.go`,
  `engine_fonts_test.go`, `internal/render/viewer/template/fonts/*`
  (`inter-latin-wght.woff2`, `source-serif-4-latin-opsz-wght.woff2`, three
  IBM Plex Mono faces);
- `--font-serif` **and its one consumer**, `.claim-body { font-family:
  var(--font-serif) }` at `style.css:332-333`.

**What a lane must NOT re-do.** No lane may:

1. add, remove, rename or re-point any of the 28 palette tokens, or add a key to
   `internal/config/config.go` `ThemeTokenAllowlist` (closed, order
   load-bearing);
2. edit either dark block asymmetrically, or add a third dark block — a token
   re-pointed in one body and not the other is the exact failure the comment at
   `style.css:169-175` exists to prevent;
3. add a second `@media print` block, or move the one at `style.css:4809-4870`
   off the end of the file (`internal/render/theme_tokens_test.go
   TestStyleCSSModeAndPrintStructure`);
4. add, replace or re-inline a font face, or touch `engine_fonts.go` /
   `viewer/template/fonts/*`;
5. re-declare `--font-serif`, `--radius-sm`, `--radius-pill`,
   `--blocked-surface`, `--blocked-hairline`, any `--text-*`, `--leading-*`,
   `--tracking-*` or `--spacing-*` — **spend them, do not restate them**;
6. re-declare `--dxg-facet-other` (`style.css:76`, `:189`, `:215`). That
   declaration is landed, it wins over `graph.css:223`, and `tokens.md`
   Disagreement 3 is closed by it.

Three `tokens.md` Disagreements are now **closed** and their spec text is stale
wherever a spec still repeats them: **6** (no serif family) — closed;
**9** (no dark override for `--border-strong` / `--status-draft` /
`--status-draft-bg`) — closed, so a DRAFT chip on dark renders `#DDA94E`, **not**
`#976600`, and specs 06 § 8.4, 03 § 8.2 and 02 § 8.1 are wrong on that point;
**10** (nowhere to put the dark blocked surface) — closed as tokens, open as
consumers, see below.

**Residual L1 items that are real gaps, moved to the lane that owns the
consumer.** Every token below resolves today but is spent by nothing
(`grep -c 'var(--<name>' style.css` → `0`). None is in the allowlist, so no test
fails; `tokens.md` § 3 / § 4 / § 1 are simply not honoured until a lane spends
them. **Consuming a token is not owning it** — several lanes spend the same
token in their own sections, and that is not an overlap.

| Token(s) with no consumer | Lane that must land the first consumer | Where |
|---|---|---|
| `--text-body` `17px`, `--leading-body` `28px` | **L3** | claim prose — `.claim-body` base and the live `.claim-body, .sbody, …` rule (`tokens.md` § 3; 07a § 4, 05 § 4) |
| `--text-display`, `--text-h1`, `--tracking-display` | **L2** | `.system-record-head h2`, which is a `clamp(28px, 4vw, 42px)` literal today |
| `--radius-pill` | **L3** | the status-chip family (`.pill`, `999px` literal) |
| `--radius-sm` | **L2** | `#dxgOpen, .sec-tab { border-radius: 4px }` |
| `--blocked-surface`, `--blocked-hairline` | **L5** | the blocked readiness region's ground (06 § 4.x, 05 § 4.9) — the surface the token's own comment was written for. **L7** and **L8** also spend it (06a mismatch panel ground; 02 § 4.19 / R-H.1 inset card); that is consumption, not co-ownership |
| `--text-h2`, `--text-ui`, `--text-meta`, `--text-label`, `--leading-title`, `--leading-ui`, `--tracking-label`, `--spacing-1…12` | **every lane, in its own sections** | no single owner: each lane replaces the literals in the rules it owns and reports in step 7 which ones it could not reach |

**Caretaking that was L1's.** The palette blocks, the two dark bodies and the
print block now belong to **no lane**. A lane needing a palette change, a dark
twin or a print rule states it in its step-7 report and the **coordinator**
lands it. See OD2.

## Lane table

Paper PNGs are under
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/paper-png/`.
Fixture columns read
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/coverage-inventory.md`
(present, dated 2026-09-17 02:28). Its caveat is passed through to every lane:
**nothing in it was rendered in a browser.** A state marked "yes" means the data
exists, not that any card was seen to draw it.

---

### L3 · claim-card — card shell, head line, status chip family, prose typography, collapse  ★ PILOT

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:303-331` base `.claim {` + the `status-draft`/`status-locked` routing comment · `style.css:332-378` base `.claim-body` (the pre-System-Record `13px` rule, **and the one `var(--font-serif)` consumer**) · `style.css:1657-1676` `.claim-card--warn` / `.claim-banner` / `.claim-card--commented` left edge · `style.css:2380-2398` `.k`, `.k > .label`, `.k > .claim-comments-slot` (the "hard right" rule) · `style.css:2399-2430` the dead early `.pill` layer · `style.css:3930-3950` `.card, .claim-tree` + `.claim-card--warn, .claim-banner` · `style.css:3951-4039` the **live** `.k` layer (`.k` 3951, `.k > .label` 3966, `.claim-collapse-toggle` 3974, `.k > .claim-comments-slot:not([hidden])` 3998, `.claim-collapse-chevron` 4019, `.claim-collapse-content[hidden]` 4038) · `style.css:4040-4066` the **live** `.pill` layer (`.pill` 4040, `.pill.ps, .status-locked` 4050, `.pill.pv, .status-draft` 4056, `.pill.pw, .claim-review-pending` 4062) · `style.css:4068-4075` the live `.claim-body, .sbody, .claim-list-items, .claim-table-scroll td` rule · `style.css:4707-4724` the card half of the 860 container (`.card, .claim-tree` 4707, `.k` 4713, `.k > .claim-comments-slot:not([hidden])` 4715, `.pill` 4720, `.claim-body, .sbody, .claim-list-items` 4722) — see OD1. **Runtime:** `system-record.js` `enhanceClaimDisclosures`, `setClaimExpanded`, `cleanTitle`. **Go:** `internal/render/components/card.html:38-43` and the identical `.k` lines in `list.html`, `tree.html`, `steps.html`, `table.html`; `components/components.go` `pillClass`, `StatusLabel`. **New surface:** the per-claim `See in claims graph` affordance (07 § 4, 07a § 7.6) — new markup in `card.html` binding the existing `[data-dxg-open]` attribute contract; see **Overlap 3**. |
| **Section markers** | `.claim {` (preceded by `/* status-draft/status-locked map to the pill family below: locked -> accent,`) · `.claim-body {` · `.claim-card--warn {` · `/* The claim head line. v0.4.1 gives it two children instead of loose text:` → `.k {` · `.pill {` (early, dead layer) · `.card,` / `.claim-tree {` · `.k {` (live, inside the System Record layer) · `.pill {` (live) · `.claim-body,` / `.sbody,` (live) · inside `@media (max-width: 860px) {` — from `.card,` / `.claim-tree {` through `.claim-body, .sbody, .claim-list-items { font-size: 14px; }` |
| **Screens verified** | **07** (primary) · **07a** (primary) · 02 (card + chip region only) · 05 (card shell region only) · 06 (card region only) |
| **Specs to read** | `screens/07-claim-boundary-no-embodiment.md` (whole), `screens/07a-claim-draft-not-yet-approved.md` (whole), `screens/02-reading-view-default.md` § 4.11–4.13 and § 4.20, `screens/05-claim-one-expansion-at-a-time.md` § 4.1–4.4, `screens/06-claim-blocked-across-four-modules.md` § 4.1–4.4, `tokens.md` § 3 and § 5, `reference-rules.md` § R-H.2 (status chip above the title at 390), § R09.8 |
| **Paper PNGs** | `07-desktop-light.png`, `07-desktop-dark.png`, `07-mobile-light.png`, `07-mobile-dark.png`, `07a-desktop-light.png`, `07a-desktop-dark.png`, `07a-mobile-light.png`, `07a-mobile-dark.png`, `components-00.png` (chip specimens), `02-desktop-light.png` / `02-desktop-dark.png` (card at rest), `05-desktop-light.png` / `05-desktop-dark.png`, `06-desktop-light.png` / `06-desktop-dark.png` |
| **Depends on** | — |
| **States to render** | LOCKED chip · DRAFT chip · review-pending chip (`.pill.pw`) · warn/blocked card (`.claim-card--warn`) · commented card left edge (`.claim-card--commented`) · collapsed / expanded claim body · `:target` deep-link landing · **07's own state**: `build_role: out-of-scope` with no embodiment and `governed_by: none` — Cutainly supplies 27 out-of-scope claims (e.g. `permission-readiness.contract.what-this-module-does-not-own`), 805 claims with no embodiment, 303 literal `governed_by: none`; **no fixture needed** · **07a's own state**: 794 drafts, 391 of them review_pending; **no fixture needed**. **Fixtures for what Cutainly lacks:** `status: published` (legacy, 0 instances) → `testdata/fixture-coverage/lint/status-shape` (1 claim). `drifted` (0 of 395) → `internal/lock/lockedhash_test.go`, `internal/lock/downgrade_test.go`. `migrated` (0 of 828) → `internal/lock/ledger_test.go`. Layouts `table` / `list` / `banner` / `mockup` (0 each) → `testdata/fixture-basic`, `testdata/fixture-portability`, `testdata/fixture-theme-flat`, `testdata/fixture-coverage`, `internal/render/mockup_render_test.go`; **`layout: tree` has no YAML fixture anywhere** and is constructed in Go only (`internal/render/render_test.go`, `internal/lint/layout_shape_mismatch_test.go`) — render it from Go or report it as not rendered. Cutainly supplies `card` (777) and `steps` (51), draft (794), locked (34), review_pending (395). |

**Intent, in prose:** the chip is a state badge, not a decoration — it exists so a
reviewer can tell, without reading, whether the claim is a commitment or a
proposal. That is why the DRAFT form carries the **open** padlock (components board
section F: the shackle is closed when an approval is on record and open when
it is not — the chip never appears without its icon): reusing the closed glyph
would say the opposite of the truth. Corrected 04:30 after the pilot verifier's
pixel comparison; 07a's own board draws the open padlock. The
board's approval of a chip does not make its glyph right — check every glyph
against what the state means, and record a mismatch as a Paper defect in the
handoff. The `.k > .claim-comments-slot` rule is owned here because it is head
*layout*; the chip **component** is L6's. `system-record.js` deliberately skips
the comment slot when it rebuilds the head into a `<button>` (a control inside a
control is not a control) and strips `.pill, .claim-comments-slot` out of derived
titles — both behaviours must survive, and `viewer-tests/claim_collapse_test.go`
(`:98-112`, `:104`) pins the chip's position within `9px` of the head's right
edge, *after* the chip, so a head re-layout is a test renegotiation. 07's card is
the lane's hardest case because its footer reads `GOVERNED BY · NONE` and its
checks panel is the `declared_none` branch: both are **stated absences**, and
07 § 8.7 gives the rule the whole revamp turns on — *a count of zero keeps the
count wording; a declared absence gets the prose wording.*

---

### L2 · shell-chrome — sidebar, nav, facet TOC, module head, facet strip, measure, focus mode

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:263-269` `.sec-tab, .subtab, #dxgOpen` (the shared nav-row rule — see **Overlap 3**) · `style.css:2673-2920` the sidebar/tab section (`#nav` 2752, `.sidebar-footer` 2761, `.sidebar-footer-stamp` 2794, `.sec-tab` 2803, `.sec-tab.on` 2818, `.sub-nav` 2849, `.content-area` 2872, soft-mount host 2882) · `style.css:3138-3209` the System Record banner + `body.shell-body` + `.sidebar` (3164) · `style.css:3210-3275` `.system-panel-toggle*` and `body.system-sidebar-collapsed` · `style.css:3317-3456` `#nav`, `#dxgOpen, .sec-tab` (3401), `.sec-tab.on` (3420), `.sidebar-footer` (3450) · `style.css:3457-3482` `.content-area` + the two `min-width: 1181px` blocks · `style.css:3483-3548` `.system-record-head` and `.sub-nav` · `style.css:3756-3878` `.facet-toc` → `.facet-toc__item.on` incl. `body.system-toc-collapsed` · `style.css:4582-4620` the **whole** `@media (min-width:861px) and (max-width:1180px)` container (OD1) · `style.css:4622-4706` the chrome half of the 860 container, ending at `.subtab { flex: none; }` (OD1). **Shell:** `shell.html:53-54`, `:61-63`, `:66-73`, `:115-131`, `:133`, `:136-143`. **Runtime:** `system-record.js` `formatGeneratedTime`, `enhanceTimestamp`, `bindResizer`, `bindSidebarCollapse`, `renderToc`, `updateTocActive`, `addModuleHeaders`, `applyThemeChoice`, `bindThemeControl`; `viewer-runtime.js` `showModuleFacet`, `setDrawer` (see **Overlap 2**). **Go:** `internal/render/render.go` `Group` / `TabLabel` / `AllLocked` grouping and `GeneratedAt`. |
| **Section markers** | `.sec-tab,` / `.subtab,` / `#dxgOpen {` · `/* ---- sidebar + tab navigation ---------------------------------------` · `/* System Record visual language for the generated viewer. */` · `.system-panel-toggle {` · `#nav {` (System Record layer) · `.content-area {` (System Record layer) · `.system-record-head {` · `.facet-toc {` · `@media (min-width: 861px) and (max-width: 1180px) {` (first declaration `.content-area { padding: 0 34px 76px; }`) · `@media (max-width: 860px) {` (the container whose first declaration is `.content-area {`, up to and including `.subtab { flex: none; }`) |
| **Screens verified** | **02** (primary), **03** (primary) |
| **Specs to read** | `screens/02-reading-view-default.md`, `screens/03-reading-view-focus-mode.md`, `tokens.md` § 3, § 6, § 8, `reference-rules.md` § 11 (R11.1–R11.6), § 10 (R10.1–R10.5), § R09.7 |
| **Paper PNGs** | `02-desktop-light.png`, `02-desktop-dark.png`, `02-mobile-light.png`, `02-mobile-dark.png`, `02-mobile-nav-modules-light.png`, `02-mobile-nav-modules-dark.png`, `02-mobile-nav-facet-light.png`, `02-mobile-nav-facet-dark.png`, `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png`, `ref-11.png` |
| **Depends on** | — |
| **States to render** | focus off / focus on (desktop ≥1181) · focus state restored from `localStorage` after reload (R11.5) · sidebar drawer open / closed at 390 · facet-TOC popover open / closed at 390 and its `<select>` form at 861–1180 · module with 1 facet and with 3 facets · theme control System / Light / Dark · freshness line in all three bands (R10.2) and its `Live` form under `dossierx serve` (R10.5). **No fixture needed for any of these** — Cutainly supplies 26 modules, 3 facets, 828 cards, 24 populated tracks, 25 locked-and-stale build orders. **Fixtures for what Cutainly lacks:** no complete track exists (`track list` → complete=false for all 25), so focus mode's "done" state cannot be shown from the client → `testdata/fixture-graph-demo` (25 locked of 58) or `internal/render/track_view_test.go`. A **fresh** (non-stale) build order has 0 instances → `internal/render/build_order_render_test.go`, `viewer-tests/build_order_tab_test.go`. Body markdown is narrow in Cutainly (no tables, images, task lists, blockquotes, links) — those are **not** this lane's sections (OD6) and must not be styled here. The board's `Search 828 claims` field has **no engine surface at all** (02 § 8.16): report it as not implemented; do not invent one. |

**Intent, in prose:** R11.1 says focus is *one reversible state, not two independent
toggles*. The lane therefore deletes the two-toggle affordance
(`shell.html:61-63` + the `.system-panel-toggle--toc` button injected by
`renderToc`) and replaces it with one control, and it renegotiates
`viewer-tests/claim_collapse_test.go`
`TestDesktopNavigationPanelsCanCollapseAndExpand`, which today asserts exactly
the two toggles R11.1 forbids. R11.2 is the reason the reclaimed 282px right
gutter must **not** widen the prose measure: the freed width goes to the
evidence (R11.3). The `@media screen and (min-width: 1181px)` scope on the
reclaim is load-bearing for the reason the build-order block's own comment gives
at `style.css:3479-3482` — a print-time reclaim would fight the print block, and
focus must compose with that reclaim rather than double it (03 § 8.6). Focus
state lives on `<body>` or in `localStorage`, never inside the SSE-swapped
`<main class="content-area">` subtree.

---

### L4 · footer-strip — evidence & relationships strip, edges, sources, expansion exclusivity

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:877-893` the `.claim-links` structural doc comment + first-layer rule · `style.css:1272-1300` the `.claim-links-summary` doc-layer comment and rule · `style.css:1301-1346` deep-link auto-open (`.claim:target .claim-links…` 1331/1335) · `style.css:1347-1437` `.claim-edges` list, `li::before`, `.claim-ref*` · `style.css:1438-1643` the sources row (`.claim-source-list` 1447 → `.claim-source:target` 1630) · `style.css:3376-3392` `.claim-footer__chevron` + `.claim-links[open]` rotation, under the border-triangle/theme-parity comment at `3366-3375` · `style.css:4108-4151` the **live** footer (`.claim-links` 4108, `.claim-links-summary` 4116, `.claim-footer__identity` 4136, `__title` 4138, `__counts` 4141, `__chevron` 4151) · `style.css:4191-4222` the live `.claim-edges, .claim-source-list` overrides · `style.css:4726-4729` the footer half of the 860 container · `style.css:4735-4747` the **whole** `@media (max-width:560px)` container (OD1). **Runtime:** `system-record.js` `enhanceFooters`, `enhanceFieldLabels`. **Go:** `components/components.go` `EdgesHTMLWithLinks` and its doc block, the per-relation `<li>` emitters, the `<summary>` digest, `countSegment`, the two server-side auto-open signals; `components/sources.go` `writeSourcesRow`, `writeExternalSource`, `writeInternalSource`, `writeSourceNote`; `internal/render/depended_by_view.go` `buildDependedByLookup`, `buildTargetStatusLookup`, `attachEdgesOverride`. |
| **Section markers** | `/* .claim-links is the <details> that wraps the whole edges footer` · `/* The summary is the disclosure's only click target and reads as a mono` · `/* Deep-link auto-open (decision C9). Landing on #<claim-id> — from the` · `.claim-edges {` · `/* ---- the sources row (components/sources.go) -------------------------` · `/* Nav-group Lucide hosts stay 14px. The evidence-footer chevron is the CSS border triangle` → `.claim-footer__chevron {` · `.claim-links {` (live, System Record layer) · `.claim-edges,` / `.claim-source-list {` (live) · inside `@media (max-width: 860px) {` — `.claim-links-summary { align-items: flex-start; }` through `.claim-footer__counts > span:not(.claim-footer__chevron)` · `@media (max-width: 560px) {` |
| **Screens verified** | **05** (primary) · 07 (footer strip § 4.7/§ 4.8, region only) · 07a (footer region only) · 02 (footer strip region only) · 06 (footer chips region only) |
| **Specs to read** | `screens/05-claim-one-expansion-at-a-time.md` (whole), `screens/07-claim-boundary-no-embodiment.md` § 4.7, § 4.8, § 6, `screens/07a-claim-draft-not-yet-approved.md` § 6 and § 7.6, `screens/02-reading-view-default.md` § 6 (footer vocabulary), `screens/06-claim-blocked-across-four-modules.md` § 6, `reference-rules.md` § R-F.1–R-F.4, § R09.1, § R09.2, § R09.4, § R09.5, § R-I.2, § R-I.3 |
| **Paper PNGs** | `05-desktop-light.png`, `05-desktop-dark.png`, `05-mobile-light.png`, `05-mobile-dark.png`, `07-desktop-light.png`, `07-desktop-dark.png`, `07-mobile-light.png`, `07-mobile-dark.png`, `07a-desktop-light.png`, `07a-desktop-dark.png`, `07a-mobile-light.png`, `07a-mobile-dark.png`, `02-desktop-light.png` / `02-desktop-dark.png` (footer at rest), `06-desktop-light.png` / `06-desktop-dark.png` (footer chips at four-module scale), `components-00.png` |
| **Depends on** | **L3** |
| **States to render** | strip closed · strip open · blocked form (R-F.2) · each of the four chips at zero and at N · comment count hard right at zero and at N (**slot only** — see Overlap 1) · mobile two-row form at 390 with no pill splitting across rows (R-F.4) · exactly one expansion open at a time (R09.2) · `:target` auto-open · server-written auto-open (drifted file, locked + review_pending) · `GOVERNED BY · NONE` (07's stated-absence form — Cutainly has 303 literal `none`, **no fixture needed**) · code evidence switched off, where the checks chip is absent rather than zero (05 § 8.18). **Fixtures for what Cutainly lacks:** `drifted` has **no** Cutainly instance (`claim list --review-pending` → 0 of 395 drifted) → `internal/lock/lockedhash_test.go`, `internal/lock/downgrade_test.go`. `mirrors` has none (0 claims) → `testdata/fixture-coverage` (11 claim files) + `lint/mirror-reciprocal`, `lint/mirror-unanchored`, `lint/mirror-mismatch`; `testdata/fixture-graph-demo` (2). Internal source rows (path + sha256) have none (0 of 781 rows) → `testdata/fixture-coverage/lint/source-internal-drift`, `lint/source-shape`; `record_id`-narrowed internal source: same fixture. Cutainly supplies: 801 claims with `rests_on`, 525 real `governed_by` targets, 303 literal `governed_by: none`, 277 claims with sources (781 external rows), 1,562 notes of which 1,175 exceed the 3-line clamp, 202 one-`rests_on` claims for the one-expansion screen. |

**Intent, in prose:** R09.1 is the whole lane in one sentence — *four expansions of
one strip, never four panels*. The footer is a single strip that opens in place,
because four sibling panels would let a reviewer lose track of which claim they
were reading. R09.2 follows from it: two open expansions make the card taller
than the viewport and the claim's own text leaves the screen. Today the engine
disagrees with the board at every point: the summary is a mono count strip
(`"1 link - 2 files - 1 source - 1 drifted"`) rewritten in the browser into
`relationship` / `source` / `file` / `drifted` chips. **Board 05/07/07a's
vocabulary — Blocked / relationships / sources / checks, each a noun and a count,
comment count hard right — exists at no address in the engine.** This is new
surface, not restyling; size it accordingly. The live footer is a bordered,
radiused, tinted box (`style.css:4108-4115`); the board replaces it with an inset
hairline inside the claim card — no second border, no second radius. That is a
deletion, and it is the change the frozen theme-parity baselines
(`viewer-tests/testdata/theme-parity/baseline-flat.html`) will report; report the
parity failure, do not fix it. `viewer-tests/source_note_clamp_test.go` force-opens
`details.claim-links` in five places and breaks the moment the footer stops being
one `<details>` — that is a test renegotiation, not a bug.

---

### L6 · comments — chip, rail, bottom sheet, threads, composer

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:1677-1861` the comments section (`.comment-chip` 1690, `--open` 1705, `--empty` 1731, `-count` 1760, `.comments-panel` 1772, `.comment-thread` 1794, `.comments-resolved > summary` 1841, and the nested coarse-pointer block) · `style.css:1862-2099` the interactive comment UI (`.comments-overlay` 1881, `.comments-rail` 1896, `-head` 1917, `-title` 1927, `-close` 1940, `-body` 1952, `.comment-action` 1994, `.comment-thread-actions` 2025, `.comment-composer` 2040, `-input` 2051, `-submit` 2069, `.comments-toast` 2080) · `style.css:2298-2314` the second `@media (pointer: coarse)` block (**whole**, incl. the `.claim-collapse-toggle` and `.status-strip-head` entries in its selector list — L3 and L8 are dependents, OD3) · `style.css:2316-2332` the comments half of the 860 sheet container (`body.comments-open`, `.comments-overlay:not([hidden])`, `.comments-rail`) — OD1 · `style.css:4224-4263` the live `.comment-chip` / `--open` overrides · `style.css:4264-4274` the live `.comments-rail` / `-head` overrides. **Not** `style.css:4507-4510` — that four-selector `:focus-visible` rule is L9's (Overlap 4). The 860-container chip-slot rule at `4715-4718` is **L3's** — head layout, not the chip component. **Runtime:** `viewer-runtime.js` rail element handles, `chipsFor`, `setChipExpanded`, `updateChips`, `syncEmptyChips`, `recomputeChipsFromPanel`, `commentPanelOpen`, `openCommentPanel`, `closeCommentPanel`, `renderPanel`, `renderPanelReadOnly`, `renderPanelFromAPI`, `buildPanel`, `syncEmptyLine`, `buildThread`, `buildMessage`, `buildThreadActions`, `iconButton`, `growNow`, `autoGrow`, `buildComposer`, `buildReplyComposer`, `threadNode`, `doReply`, `doResolve`, `doReopen`, `doDelete`, `startEdit`, the chip click delegation, the scrim/close/Escape wiring and the SSE body classes. **Shell:** `shell.html:242-250`. **Go:** `components/components.go` `CommentChipHTML`, `commentsPanelTmpl`, `commentsPanelView`, `newCommentsPanelView` and the panel-append site; `components/comments.html`. |
| **Section markers** | `/* ---- comments (engine-managed review threads) --------------------------` · `/* ---- interactive comment UI (Phase 5, serve + file://) -----------------` · `/* Coarse pointers (touch) need >=44px hit targets on every comment control. */` → `@media (pointer: coarse) {` · inside `/* Mobile: the rail becomes a modal bottom sheet — the dimming backdrop and the` → `@media (max-width: 860px) {`, the declarations from `body.comments-open {` through the close of `.comments-rail { … border-radius: 14px 14px 0 0; }` · `.comment-chip {` (live, System Record layer) · `.comments-rail {` (live) |
| **Screens verified** | **14** (primary), **14a** (primary) |
| **Specs to read** | `screens/14-comments-rail-open-on-a-claim.md`, `screens/14a-comments-states-and-placement.md`, `tokens.md` § 2 (the `AGENT` colour is `--color-graph-facet-1` / `--color-dark-graph-facet-1`), `reference-rules.md` § R-J.1–R-J.7 (bottom-sheet policy), § R-F.3, § R10.1/R10.4 (elapsed time) |
| **Paper PNGs** | `14-desktop-light.png`, `14-desktop-dark.png`, `14-mobile-light.png`, `14-mobile-dark.png`, `14a-desktop-light.png`, `14a-desktop-dark.png`, `14a-mobile-light.png`, `14a-mobile-dark.png` |
| **Depends on** | **L3** |
| **States to render** | empty (state 1) · open thread (state 2, with reply) · resolved thread · read-only `file://` (state 3, no composer) · chip variants `--empty` / `--open` / `--resolved` at 0 and N · desktop non-modal rail (no backdrop, no scroll lock) · mobile modal sheet (backdrop + scroll lock) · composer pinned and never shrinking · edited message · loading and error body states · optimistic / pending / deleting messages. **Fixtures for what Cutainly lacks:** `resolved thread` has **no** Cutainly instance (all 10 threads are `status: open`) → `testdata/fixture-theme-flat/claims/widget-reviewed.yaml` or `viewer-tests/viewer_test.go TestUIResolveCollapsesThreadAndUpdatesChip`. `human-authored message` has none (10/10 authors are `agent`) → `testdata/fixture-coverage/lint/comments-unresolved/claims/commented.yaml`, `testdata/fixture-graph-demo/claims/engine-lint-runs-before-catalog.yaml`, `testdata/fixture-theme-flat/claims/widget-reviewed.yaml`. `edited message` has none (`edited: false` everywhere) → `viewer-tests/viewer_test.go TestUIEditMarksEditedAndPersists`. Cutainly supplies: 10 open threads on 9 claims, 10 replies, 819 zero-thread chips, and `audience-boundary.internals.invalidation-precedes-publication` with two threads on one claim. |

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
viewer-test files, and `system-record.js` carries two live dependencies on
`.claim-comments-slot` (`enhanceClaimDisclosures`, `cleanTitle`) that are **L3's**
functions — a class rename here is a cross-lane break.

---

### L8 · status-strip + issues view — the blocked banner on 02/03 **and** the promoted Issues view of 04

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:2100-2296` the whole status-strip section — the marker and its z-index-ledger comment (`2100-2141`), the "project health belongs to the reading flow" intent comment (`2143-2146`), then `.status-strip` 2147, `[hidden]` 2157, `--lint` 2161, `--integrity` 2166, `-head` 2172, `-summary` 2185, `-title` 2191, `-note` 2197, `-action` 2203, `-caret` 2211, `--open .status-strip-caret` 2218, `-body` 2224, `-filters` 2235, `.status-group + .status-group` 2242, `.status-group-head` 2246, `.status-finding-list` 2254, `.status-finding` 2260, `-rule` 2279, `-claim`, `-msg` 2292 · `style.css:2333-2362` the status-strip half of the 860 sheet container (`.status-strip, .claim-group, .claim` width reset 2333; `.status-strip-head, .claim-collapse-toggle { min-height: 44px }` 2340 — **L3 is a dependent on that selector list**, OD3; `-head` grid 2344, `-summary` 2349, `-action` 2353, `-caret` 2358) — OD1 · `style.css:4275-4279` the live `.status-strip` override · **the Issues view's own new section**, appended per OD1 and tagged `/* lane: L8 */`. **Runtime:** `viewer-runtime.js` `countLabel`, `findingsForActiveFacet`, `STATUS_SEVERITIES`, `stripSeverityFilter`, `addStatusGroup`, `collectStatusGroups`, `statusGroupClaimCount`, `uniqueClaimCount`, `countSeverity`, `statusChipLine`, `blockerHeadline`, `renderStatusGroup`, `positionStatusStrip`, `findingGroup`, `renderStatusStrip`, `setStripExpanded`, `refreshStatus`, and the offline fallback. **Shell:** `shell.html:272-281` (`#statusStrip`, and the literal action label `Show issues` at `:278`). **Go:** no emitter exists today; the Issues view's server side is **new** — a seventh `*_view.go` beside `build_order_view.go`, `graph_view.go`, `track_view.go`, `conformance_view.go`, `depended_by_view.go`, `implink_view.go` (04 § 7.4). It reads `internal/render/render.go` `HasReadinessMaps` and `in.cat.Readiness`, and `internal/serve/handlers.go` `LedgerFindings` for the live half. |
| **Section markers** | `/* ---- status strip (lock-ledger integrity + lint) -----------------------` (its comment block carries **THE Z-INDEX LEDGER**) · `/* Project health belongs to the reading flow, between the active module's` → `.status-strip {` · inside `@media (max-width: 860px) {` (the sheet container) — the declarations from `.status-strip,` / `.claim-group,` / `.claim {` through `.status-strip-caret { … margin-top: 4px; }` · `.status-strip {` (live, System Record layer, immediately before the `/* Claims graph: … */` marker) |
| **Screens verified** | **04** (primary, all four boards) · 02 (§ 4.10 and § 4.19 blocked banner, desktop and the 390 inset-card form — region only) · 03 (§ 4.5 blocked banner — region only) |
| **Specs to read** | `screens/04-issues-screen.md` (whole), `screens/02-reading-view-default.md` § 4.10, § 4.19 and § 6, `screens/03-reading-view-focus-mode.md` § 4.5 and § 7.5, `tokens.md` § 8, `reference-rules.md` § R08.1, § R08.2, § R09.6, § R-H.1 |
| **Paper PNGs** | `04-desktop-light.png`, `04-desktop-dark.png`, `04-mobile-light.png`, `04-mobile-dark.png`, `02-desktop-light.png`, `02-desktop-dark.png`, `02-mobile-light.png`, `02-mobile-dark.png`, `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png`, `ref-08.png` |
| **Depends on** | — (hand-off from **L2**, see Overlap 2) |
| **States to render** | strip hidden (nothing actionable) · lint variant (`--lint`) · integrity variant (`--integrity`) · collapsed · expanded (`Show issues` / `Hide issues`) · 390 inset-card form (R-H.1: inset card, **not** a full-bleed band) · offline fallback · Issues view grouped by module · a pressed severity chip (single-select, `stripSeverityFilter`) · a severity filter that matches nothing · scope = Facet / Module / Project · more than four modules in the ranking rail · empty state reached by direct navigation. **Fixtures for what Cutainly lacks:** `lint ERROR` findings have **no** Cutainly instance (`lint_error_count = 0`), so **the error arm of the two-hue severity vocabulary (R08.1) and the `Critical` chip cannot be shown from the client** → `testdata/fixture-coverage/lint/<rule>/` — **38 single-rule fixtures, one per lint rule**. `ledger findings` — the whole `APPROVAL RECORD` group of R08.2 and 04 § 8.3 — have none (`ledger_finding_count = 0`) and **no viewer fixture exists**: they live only as `internal/lock/audit_test.go`, `ledger_test.go`, `downgrade_test.go` unit assertions. `code-scan errors` have none (`scan_files_scanned = 0`) → `testdata/fixture-coverage/lint/code-orphan`, `internal/render/implink_view_test.go`. Severity chips are **partial**: warning only. Cutainly supplies: 11 lint warnings across 3 rules (comments-unresolved 9, body-edge-hint 1, track-empty 1), 28 `next_steps` entries, 10 open threads, 25 stale build orders, 2 conformance mismatches. |

**Intent, in prose:** R09.6 — *Issues is a screen, and the banner is the only way
in.* This lane builds both ends of that sentence, because two lanes cannot own
`.status-strip`: the strip's **collapsed** form is the blocked banner on 02 and
03, and its **detail list** — `.status-strip-body`, `.status-strip-filters`,
`.status-group-head`, `.status-finding*` — is promoted into the Issues view of
04 per 04 § 7 and § 8. R08.2 is the constraint that makes the two variants
non-cosmetic: a lint finding and a ledger finding must never read alike, because
one is an author's mistake and the other is evidence the lock ledger cannot be
trusted. R08.1 caps the vocabulary at two hues. The strip's 80/81 z-index
neighbours belong to the graph pane and are **L9's** — coordinate a renumber
through the coordinator rather than editing the ledger unilaterally; a promoted
Issues view is a *document* view and should take **no** z-index band at all.

**Residual risk this lane carries, stated by the 04 spec itself.** `grep -n
'issue'` over `style.css` returns nothing and `grep -rn 'issues'
internal/render/*.go` returns nothing: **the engine has no Issues *screen* at any
address.** What exists is the strip. L8 therefore *promotes* the strip's detail
list into that view, keeping the collapsed form as the 02/03 banner. Two further
constraints come with it. (1) `shell.html:265-271` states the strip exists **only
against a live serve**, because a baked integrity verdict is the "stale green
strip nobody checked". 04 § 8.11 resolves the split: readiness and lint groups
render statically from `in.cat.Readiness` (`render.go:879-884`), the
`APPROVAL RECORD` group renders only under serve, and its absence on `file://` is
**stated, not silently omitted**. (2) `viewer-tests/component_fit_test.go:67`
asserts `#statusStripBody .status-finding--group` is between 1 and 6 while
grouping by *severity*; the board groups by **module**, so that assertion changes
by construction — report it. `claim_collapse_test.go:212-241` pins the exact
headline string, and `theme_parity_test.go:828-829` pins two `24 surfaces` golden
rows that a new Issues surface shifts.

---

### L5 · readiness — the blockers expansion

| | |
|---|---|
| **Owned code sections** | **CSS:** `style.css:895-1246` the entire readiness surface (`.claim-readiness` 898, `-head` 906, `-id` 915, `-title` 919, `-state` 929, `--ready` 942, `-summary` 951, `-counts` 957, `-count` 963, `-label` 982, `-section-head` 993, `-routes` 1019, `-route` 1026, `-route-id` 1054, `-blockers`, `-blocker`, `-relation`, `-path`, `-trace`, `-map`, `-raw` 1224) · `style.css:1248-1262` the **whole** `@media (max-width: 860px)` readiness block · `style.css:1263-1270` the **whole** `@media (max-width: 520px)` readiness block. **Runtime:** `viewer-runtime.js` `renderClaimReadiness`, `readinessFactRow`, `readinessRoute`, `readinessHopLabel`, `readinessClaimLabel`, `readinessFactLabel`, `readinessPathDetails`, `readinessMermaidSource`, `readinessRawDiagnostics`, `offlineReadiness`, and the two `renderClaimReadiness` call sites; `build-order-ui.js` the Mermaid host wiring for `.claim-readiness-map`, which R09.9 makes dead weight. **Go:** `internal/render/render.go` readiness payload plumbing (`HasReadinessMaps` and the fields feeding it). |
| **Section markers** | `/* Claim readiness is a reviewer-facing work queue. The policy engine remains` → `.claim-readiness {` · `@media (max-width: 860px) {` whose first declaration is `.claim-readiness-head { display: block; }` · `@media (max-width: 520px) {` whose first declaration is `.claim-readiness-blocker { grid-template-columns: 1fr; }` |
| **Screens verified** | **06** (primary, all four boards) · 03 (§ 4.10–4.11 readiness expansion, region only) · 05 (§ 4.9 readiness blockers expansion, region only) |
| **Specs to read** | `screens/06-claim-blocked-across-four-modules.md` — **the § 4.x readiness sections and § 5–§ 9 in full** · `screens/03-reading-view-focus-mode.md` § 4.10–4.11 and § 7.4 · `screens/05-claim-one-expansion-at-a-time.md` § 4.9 · `tokens.md` § 1 (`--blocked-surface` / `--blocked-hairline`) · `reference-rules.md` § R09.3, § R09.8, § R09.9, § R-I.1 |
| **Paper PNGs** | `06-desktop-light.png`, `06-desktop-dark.png`, `06-mobile-light.png`, `06-mobile-dark.png`, `03-desktop-light.png`, `03-desktop-dark.png`, `03-mobile-light.png`, `03-mobile-dark.png`, `05-desktop-light.png`, `05-desktop-dark.png`, `05-mobile-light.png`, `05-mobile-dark.png`, `ref-09.png` |
| **Depends on** | **L4** |
| **States to render** | ready · locally approved but chain not ready · not approved but chain ready · not approved and chain not ready · zero blockers · auto-open when and only when blocked (R09.3) · **blocked across exactly four modules — 06's headline state**: Cutainly supplies 50 such claims, e.g. `sharing-observation.contract.the-command-surface` (36 conditions, 4 modules), and 19 claims blocked across ≥20 modules (max `voice.contract.what-this-module-does-not-own`, 226 conditions / 23 modules), so **no fixture is needed for 06's own subject** · one blocker in one module, for the singular degradation (`1 across 1 module`) · stacked blocker row at 390 with the dependency path stacked and never dropped (R-I.1). **Fixtures for what Cutainly lacks:** only **one** blocker kind exists in Cutainly — `dependency_unapproved`, 31,673 instances. The other five have no client instance: `missing_dependency` → `testdata/fixture-coverage/lint/dangling` + `internal/readiness/readiness_test.go`; `unreadable_dependency` → `internal/readiness/readiness_test.go`; `retired_dependency` → `testdata/fixture-coverage/lint/supersede`; `unknown_historical_baseline` → `internal/readiness/readiness_test.go`; `dependency_cycle` → `testdata/fixture-coverage/lint/cycle` (but see the L9 note: a cycle is error-severity and no corpus that renders can contain one — render it through `internal/readiness/readiness_test.go`, not through a viewer). Review causes `own_flag` → `viewer-tests/claim_readiness_test.go TestReadinessTreatsFlagDetailsAsText`; `direct_dependency_change`, `approval_content_drift` → `internal/readiness/readiness_test.go`, `internal/lock/lockedhash_test.go`. **`approval_missing`, `approval_released` and `approval_unknown` have no fixture anywhere** — `internal/lock` unit assertions only, never reaching a rendered viewer; do not attempt to render them, report them as not rendered. Cutainly supplies: 3 ready claims, 31 locally-approved/chain-not-ready, 38 not-approved/chain-ready, 756 not-approved/chain-not-ready, 41 zero-blocker claims, 202 one-`rests_on` claims. |

**Intent, in prose:** readiness is a *work queue*, not a diagnostic dump — the
reviewer should learn what to do next, not what the policy engine computed.
R09.9 removes the inline Mermaid dependency map because a picture of a graph
inside a card is a graph nobody can read; the breadcrumb and the graph pane carry
it instead. R09.8 demotes the raw JSON envelope and cuts the phrase
`shown via this dependency` for the same reason: eight demotions, and **nothing is
deleted from the data**. Removing the map moves the subject of
`viewer-tests/claim_readiness_test.go`
`TestReadinessMapCapNeverCapsTheAuthoritativeList` and the `Showing 12 of 14`
caption assertion — report those, do not paper over them. 06's module grouping is
**new construction**: the engine groups by *representative route*
(`readinessRoute`), not by *the module that owns the fix*, and there is no
`nearest N hops` label and no `Show N more` anywhere in `internal/render/`.

**Read the 06 spec's values, not its node ids.** § 4.8's dark-column node ids are
**wrong** — they name container frames rather than the painted nodes. The
**values** in that table are right and are what the lane implements. This is the
node-citation error that held 06 back, and it is not a value error.

---

### L7 · conformance — "How this was checked" disclosure

| | |
|---|---|
| **Owned code sections** | **Go (this panel's stylesheet is a Go string, not in `style.css`):** `internal/render/conformance_view.go` — the `conformanceCSS` constant in full, `viewerCSSWithConformance`, `conformanceStatusFetchGuardJS` · `internal/render/components/conformance.go` — `ConformanceHTML`, `writeConformanceCheck`, `conformanceStateLabel`, `conformancePill`, `writeConformanceValue`, `writeConformanceLine`, `writeConformanceMembers` · `internal/render/render.go` the conformance insertion point `insertEngineBlockBeforeClose` and the two `if` guards that append the CSS and the status-fetch guard. **Shell:** `shell.html:313-314` the guard injection point (order relative to `{{.ViewerRuntimeJS}}` is load-bearing). **Runtime:** `viewer-runtime.js` `conformanceNotReadyIDs` — the only runtime code that touches the panel. **CSS:** none. `grep -c conformance internal/render/viewer/template/style.css` returns **0**; if this lane needs a `style.css` rule it opens a new block appended per OD1, tagged `/* lane: L7 */`, and says so in its report. |
| **Section markers** | none in `style.css` — this lane owns no `style.css` range. Its stylesheet marker is the Go string's first line, `/* Added only when this viewer contains structured conformance results. */`, inside the `conformanceCSS` raw literal in `internal/render/conformance_view.go`. |
| **Screens verified** | **06a** (primary) · 07 (§ 4.12, the `declared_none` conformance branch — region only) |
| **Specs to read** | `screens/06a-how-this-was-checked-inline-disclosure.md` (whole), `screens/07-claim-boundary-no-embodiment.md` § 4.12 and § 6 (the closing note), `tokens.md` § 1 and § 3, `reference-rules.md` § 12 (R12.1–R12.4), § R09.3 |
| **Paper PNGs** | `06a-desktop-light.png`, `06a-desktop-dark.png`, `06a-mobile-light.png`, `06a-mobile-dark.png`, `07-desktop-light.png`, `07-desktop-dark.png`, `07-mobile-light.png`, `07-mobile-dark.png`, `ref-12.png` |
| **Depends on** | **L4** |
| **States to render** | disclosure closed · disclosure open · verdict `Matched` · `Mismatch` · `Uncheckable` · `Owed` · `mode: none` / `declared_none` (the `declaration: none` + `reason:` branch — **07's state**; Cutainly supplies 6 such claims, each with a prose reason, **no fixture needed**) · shape `scalar` · shape `set` with missing and extra members · empty `extra` as an em-dash row · `observation error` / `adapter message` rows · more than one check in one panel · `implementation_ready = false` auto-open (R09.3, already implemented at `conformance.go:28-30`) · ready panel renders **closed** and stays inside a collapsed claim. **Fixtures for what Cutainly lacks:** `check state: owed` has **no** Cutainly instance (summary.owed = 0) → `testdata/fixture-conformance-v1/claims/owed.yaml`. `check state: uncheckable` has none → `testdata/fixture-conformance-v1/claims/uncheckable.yaml` + `observations.json widget://uncheckable`. `conformance.blocking: true` has none (the client sets `blocking: false`, so **the refusing form of the disclosure cannot be shown from Cutainly**) → `internal/serve/conformance_blocking_test.go`, `internal/check/conformance_staged_test.go`. Cutainly supplies: 23 declared, 34 checks, 32 matched, 2 mismatch, 6 `mode: none`, 17 `mode: compare`, 31 set-shape, 3 scalar-shape, 2 `implementation_ready = false`. |

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
report says otherwise. On 07 the branch is `declared_none` and 06a § 8.4 is
explicit that the disclosure **must not render at all** there: an empty
"How this was checked" is worse than none. 07 § 6's closing note is a **rewrite**
of `conformance.go:38-43`'s scope sentence, not a reuse of it.

---

### L9 · claims-graph — the pane, its palette, its rail and its readings

| | |
|---|---|
| **Owned code sections** | **CSS:** `internal/render/viewer/template/graph.css` — **the whole file** (1186 lines): the dark-is-base `:root` ramp `147-231`, the `@media (prefers-color-scheme: light), print` override `236-286`, pane chrome `288-441`, controls `444-493`, `#dxgOpen` `495-537`, toggles `539-563`, notices `565-598`, canvas `600-638`, rail `640-700`, its `@media (max-width: 860px)` `704-729` and `@media (pointer: coarse)` `733-744`, legend `746-933`, rail interior `935-1109`, detail panel `1119-1186` · `style.css:4280-4506` the graph override block · `style.css:4507-4510` the shared `.dxg-btn, .dxg-select, .dxg-toggle, .comments-rail-close` `:focus-visible` rule (**L6 is a dependent** — Overlap 4) · `style.css:4512-4580` the graph's own `@media (max-width: 860px)` container. **Runtime:** `graph-core.js` (whole, 1713 lines) and `graph-ui.js` (whole, 3864 lines). **Shell:** `shell.html:284-299` the pane mount comment and `<section id="dxgPane" hidden>`, `:301-311` the `<script type="application/json" id="dossierx-graph">` payload block, `:316-325` the `{{.GraphCoreJS}}` / `{{.GraphUIJS}}` load order, and `:12-19` the comment fixing `graph.css` as the **first** `<style>` block. **Go:** `internal/render/graph_view.go` (`graphPayloadJSONWithBudget`, the single `graph.Build` call site), `internal/render/lazy_shell.go:77` `GraphPayload()`, and the graph plumbing in `internal/render/render.go` (the `//go:embed` line, `graphCoreFileName` / `graphUIFileName` / `graphCSSFileName`, `graphCSSTemplatePath`, the `GraphCSS` / `GraphPayload` / `GraphCoreJS` / `GraphUIJS` shell fields and their assignment sites). `internal/graph/*` is the payload's own package and is **read, not restyled** — a change there is a payload change and goes through the coordinator. |
| **Section markers** | `graph.css:1` `/* graph.css — chrome and categorical palette for the DossierX claims graph pane.` · `graph.css` `/* DARK IS THE BASE, light is the override.` · `/* ====== PANE CHROME ====== */` · `/* ---- the nav trigger ----` · `/* ---- notice strip: dangling edges, auto-collapse, refresh failure ----` · `/* ====== LEGEND STRIP — facet identity's second channel ====== */` · `/* ====== RAIL INTERIOR — the gaps list, then the detail footer ====== */` · `/* ---- detail panel ----` · in `style.css`: `/* Claims graph: a first-class System Record view, not a utility overlay. */` (the block's first selector is `#dxgPane {`), `/* Diagnostics are available elsewhere; this view reserves the rail for the` → `.dxg-gaps { display: none !important; }`, `.dxg-btn:focus-visible,` (the four-selector rule), and the `@media (max-width: 860px) {` container whose first declaration is `.dxg-head {` |
| **Screens verified** | **13** (primary, all four boards) |
| **Specs to read** | `screens/13-claims-graph-opened-from-a-claim.md` (whole), `tokens.md` § 2 (the two ramps and their opposite conventions) and `tokens.md` Disagreements 1–3, `reference-rules.md` § R08.1 |
| **Paper PNGs** | `13-desktop-light.png`, `13-desktop-dark.png`, `13-mobile-light.png`, `13-mobile-dark.png`, `ref-08.png` |
| **Depends on** | — (this lane touches no claim card; the per-claim `See in claims graph` link is **L3's** markup binding L9's `[data-dxg-open]` contract — Overlap 3) |
| **States to render** | facet hue identity across 3 facets · degree-scaled radius · collapsed module groups, all-locked and not-all-locked · draft fill + dashed status ring · ghost (out-of-scope) nodes · `governed_by` edges and the `governed` overlay (Cutainly: 525 real targets, 303 literal `none`) · track subgraphs and the owner's inner ruling (24 populated tracks) · review_pending halo vs open-thread halo · auto-collapse at corpus scale · nothing selected · empty scope intersection · empty overlay · a rule that found nothing · `+N more` · refresh failure · unresolved edges · no payload at all · served `Live` vs `file://` elapsed. **Fixtures for what Cutainly lacks** — the coverage inventory records that **three of the graph's overlays have nothing to draw**: **mirrors** (0 claims; `mirrors` appears in no client YAML) → `testdata/fixture-coverage` (11 claim files with mirrors) + `lint/mirror-reciprocal`, `lint/mirror-unanchored`, `lint/mirror-mismatch`, and `testdata/fixture-graph-demo` (2); the relation is asserted by `viewer-tests/graph_rail_test.go TestGraphLegendDescribesEveryRelationAndFollowsTheOverlay`. **Isolated claims** (0; every one of the 27 claims with no `rests_on` is depended on) → `testdata/fixture-coverage/lint/orphan`, and the rendered case is `viewer-tests/graph_parity_test.go TestGraphIsolatedMatchesOrphanLintUnscoped`, whose `parityClaims` fixture isolates `widget.contract.base` and `widget.contract.lonely`. **Cycles of every shape** — dependency, governance, mixed and self-edge (all 0) → the YAML fixtures `testdata/fixture-coverage/lint/cycle` (triangle-a/b/c + self), `lint/governed-cycle`, `lint/mixed-cycle` exist **but do not render**, because all four are error-severity lints and `dossierx check` returns above the render stage on the first error partition; the only way to draw one is payload injection, which is exactly what `viewer-tests/graph_pane_test.go TestGraphPaneRendersInjectedCycles` does (its `injectedCyclePayload` helper, and the file's own header comment at `graph_pane_test.go:18-27` states the reasoning). Use that injection route for the cycle ring, the `IN A CYCLE` / `Self-edge` rail values and the cycle rule block. A **complete track** has none → `testdata/fixture-graph-demo` (25 locked of 58), `internal/render/track_view_test.go`. `--dxg-cycle` / `--dxg-halo` / `--dxg-governed` resolution is pinned by `viewer-tests/graph_canvas_test.go:158`. |

**Intent, in prose:** board 13 makes the pane a **first-class System Record view**,
which is what `style.css:4280`'s own marker already says it is — and then the
shipped sheet contradicts it twice: `style.css:4433` hides the readings rail
outright (`.dxg-gaps { display: none !important }`) and `style.css:4441` deletes
the rail entirely when nothing is selected (`.dxg-rail:has(.dxg-detail-empty)`).
Board 13 reinstates the rail as `WHAT THE LAYOUT FOUND` and keeps it present with
nothing selected, because a rail that appears and disappears makes the canvas
width jump. `graph.css` keeps the **opposite** mode convention to `style.css` —
unconditional `:root` is dark, light is the override, and `internal/render/
theme_tokens_test.go TestGraphCSSModeStructure` fails if a dark query ever
appears in it or if the light query loses its `, print`. **The engine's graph
ramp wins over Paper's** (`tokens.md` Disagreements 1–2): five of Paper's nine
`--color-dark-graph-*` tokens are light hexes copied from their own light twins,
and the engine's values are the output of a maximin CIEDE2000 solve on frozen
slots 1–5 — changing one moves the ramp's minimum distance. `readPalette()`
(`graph-ui.js:1910`) is the **only** place graph colour is read; every new colour
must arrive through a `--dxg-*` custom property there, not as a literal in a draw
call. Two hard test constraints: `graph_scope_test.go` asserts
`.dxg-controls .dxg-ctl` is **exactly 6** (7 with tracks), and
`graph_redrive_test.go TestGraphPaneControlsMeetContrastAA` must survive every
colour change in § 4.2 and § 4.3. § 6's `last read 2h ago` wording change touches
`graph_pane_test.go:382-387`, which requires `[data-dxg-stamp]`'s text to start
with `payload generated` — report it.

---

## Recommended pilot: **L3 · claim-card**

**L3 is the pilot, and it verifies screens 07 and 07a end-to-end.** With L1
satisfied by `8890919`, L3 has no unmet dependency: it needs the palette, the
type and spacing tokens and the serif face, and all three have landed — it is the
only lane that can start against a finished foundation without waiting for
another lane's surface to exist. 07 is the brief's preferred pilot screen, and
its decisive surface is L3's: the card shell (12px radius against the engine's
8px, `style.css:303-331` / `3930-3950`), the head line (`.k`, both layers), the
status chip family (`.pill.ps` / `.pv` / `.pw`, the live layer at `4040-4066`,
whose DRAFT arm is now the first place the new `--status-draft` dark twin is
seen) and the prose typography (`.claim-body`, the one `--font-serif` consumer,
which the boards re-specify as serif 17/28 against the engine's 14.5px/1.68).
Running it first buys the coordinator four things the remaining eight lanes all
need: the card geometry every other surface is measured inside, the chip
vocabulary R-H.2 and every status state depend on, the first real cost of the
frozen theme-parity baselines against a card-level change, and the first proof
that a board value read from Paper survives an 828-claim corpus — and it buys
them on a screen whose own state (`build_role: out-of-scope`, no embodiment,
`governed_by: none`, 27 instances in Cutainly) needs no fixture at all.

## Wave plan

Lanes inside a wave share no `style.css` section, no runtime function and no Go
emitter. Every lane appears after every lane it depends on.

| Wave | Lanes | Why they are independent |
|---|---|---|
| **Pilot** | `L3` | Alone. Every other lane measures inside the card geometry, the head line and the chip family this lane sets, so nothing else may start against a card that is about to move. |
| **A** | `L2`, `L4`, `L6`, `L8` | **`style.css`:** L2 is the sidebar / nav / facet-TOC / module-head / content-area ranges plus the 861–1180 container and the chrome half of the 860 container; L4 is the footer, edges and sources ranges plus the footer half of the 860 container and the whole 560 container; L6 is the two comments ranges, the second coarse block and the comments half of the 860 sheet container; L8 is the status-strip range, the status-strip half of the same sheet container and the live `.status-strip` override. Four disjoint line sets, and the two shared containers are split declaration-by-declaration under OD1. **Runtime:** L2 holds `system-record.js`'s shell functions plus `showModuleFacet` / `setDrawer`; L4 holds `enhanceFooters` / `enhanceFieldLabels`; L6 holds the whole `viewer-runtime.js` comment surface; L8 holds the whole `viewer-runtime.js` status-strip surface. No function is named twice. **Go:** L2 is `render.go`'s grouping and `GeneratedAt`; L4 is `EdgesHTMLWithLinks`, `sources.go` and `depended_by_view.go`; L6 is `CommentChipHTML`, `comments.html` and the panel view; L8 emits a **new** `*_view.go` that exists in no other lane. Three inter-lane crossings are resolved as hand-offs, not dependencies: **Overlaps 1 and 2** below. |
| **B** | `L5`, `L7`, `L9` | **`style.css`:** L5 is `895-1270` (the readiness section and its two own media blocks); L7 owns **no** `style.css` range at all — its stylesheet is the `conformanceCSS` Go constant; L9 is `4280-4580` plus the whole of `graph.css`. Disjoint by construction. **Runtime:** L5 is the `viewer-runtime.js` readiness functions plus the dead `build-order-ui.js` Mermaid host; L7 is `conformanceNotReadyIDs`, one function; L9 is `graph-core.js` and `graph-ui.js`, two files no other lane opens. **Go:** L5 is `render.go`'s readiness plumbing; L7 is `conformance.go` + `conformance_view.go` + the conformance insertion guards; L9 is `graph_view.go`, `lazy_shell.go:77` and `render.go`'s graph plumbing. All three touch `internal/render/render.go` but never the same symbol — `render.go` is partitioned **by symbol, never by line range**, and a lane that needs a symbol it does not own says so in step 7. **Ordering:** L5 and L7 both need L4's strip to exist before they can be an expansion of it; L9 depends on nothing and is placed here because it is the largest single lane and the one whose tests are least entangled with the reading view. |

```
pilot:   L3★
wave A:  L2   L4   L6   L8
wave B:  L5   L7   L9
```

### Overlaps resolved

Four genuine overlaps exist. Each is resolved by giving the shared selector,
function or markup to **exactly one** lane; the other lane is a **dependent**
with a one-line hand-off rule. None of them creates a new wave.

**Overlap 1 — the footer's comment-count slot (L4 ↔ L6).** Boards 05, 07 and 07a
put a comment count hard right in the footer strip; R-F.3 gives it an accent
variant on mobile. **The slot is L4's** — its position in the fixed four-plus-one
order, the hard-right rule, the 390 two-row arrangement. **The chip is L6's** —
`CommentChipHTML`, `.comment-chip*`, the `--empty` / `--open` / `--resolved`
classes, the glyph and the count element. *Hand-off:* **L4 positions, L6 paints**
— L4 must not edit `CommentChipHTML` or any `.comment-chip*` rule, and L6 must
not set order, margin or flex behaviour inside `.claim-footer__counts`.

**Overlap 2 — the module-head area and the nav drawer (L2 ↔ L8, L2 ↔ L6).**
`positionStatusStrip` inserts `#statusStrip` between `.system-record-head` and
`.sub-nav`, both of which are L2 selectors; and `openCommentPanel` calls
`setDrawer(false)`, which is an L2 function. **`.system-record-head`, `.sub-nav`
and `setDrawer` are L2's; `positionStatusStrip` and every `.status-strip*`
selector are L8's; the nav↔comments mutual-exclusion call sites are L6's.**
*Hand-off:* **L2 keeps the head→strip→facet-strip sibling order and keeps
`setDrawer(false)` callable with its current signature**; if L2 changes either,
it says so in step 7 and L8 / L6 re-point in their own sections.

**Overlap 3 — the graph trigger and the per-claim graph link (L2 ↔ L9, L3 ↔ L9).**
`#dxgOpen` is styled in two files: `style.css:263-269` and `style.css:3401-3412`
shape it as a sidebar nav row beside `.sec-tab`, and `graph.css:495-537` gives it
its pane-open state. **The sidebar form is L2's** (both `style.css` rules), **the
pane-open form is L9's** (`graph.css`). Separately, the per-claim
`See in claims graph` affordance that 07 and 07a ask for exists nowhere — the
only trigger is `shell.html:73`. **The new link's markup and placement in the card
are L3's; the `[data-dxg-open]` attribute contract is L9's.** *Hand-off:* **L3
emits the attribute and nothing else; L9 keeps `graph-ui.js:68`'s
`OPEN_SELECTOR = '[data-dxg-open]'` delegation working for more than one match in
the document.**

**Overlap 4 — the shared `:focus-visible` rule (L6 ↔ L9).**
`style.css:4507-4510` is **one rule with four selectors**: `.dxg-btn`,
`.dxg-select`, `.dxg-toggle` and `.comments-rail-close`. **The whole rule is
L9's** — three of the four selectors are graph controls. *Hand-off:* **L6 must
not edit that selector list**; if the rail's close button needs a different focus
ring, L6 adds its own rule in an appended block per OD1, tagged `/* lane: L6 */`,
which wins on source order.

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

### How a lane is run

- **Lanes run in coordinator-created git worktrees branched from integration
  HEAD.** Never Agent-tool `isolation: "worktree"`, and never the shared main
  checkout. The coordinator creates the worktree under
  `.claude/worktrees/` (see the `safe-git-worktree` skill) and hands the lane its
  path and branch; the lane works only there.
- **Each lane reads its specs from its OWN worktree's `docs/design/`** — not from
  another worktree's copy and not from the vault. That is what makes step 1's
  "nothing else" enforceable and what keeps a lane's spec in sync with the tree
  it is editing.
- **The lane tools are documented at**
  `/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/tools/README.md`
  — `render-client.sh` (renders the Cutainly client with your engine tree, theme
  preset stripped, into your own out-dir; the real client is never touched) and
  `shot` (screenshots at a width, in a scheme, scrolling N pages, with
  `-click` for opening disclosures, claims, the comments rail and the graph
  pane). Step 3 is those two tools. Lane screenshots go to the vault under
  `screenshots/<lane>/`.
- **Line numbers are not addresses.** A lane resolves every `style.css` and
  `graph.css` range from its **Section markers** row by grepping the marker text
  at its own HEAD. The numbers in this file were correct at `8890919`; the
  numbers in the screen specs were correct at `3ac8844` and are not correct
  anywhere now.

## Open decisions

Recorded rather than asked. The coordinator may overrule any of them.

**OD1 — Shared responsive containers are owned line-by-line, and new mobile rules
go in a lane's own appended block.** Four containers are physically shared:
`@media (max-width: 860px)` at `style.css:2315-2363` (the comments-sheet /
status-strip / card container), `@media (min-width: 861px) and (max-width: 1180px)`
(`4582-4620`), `@media (max-width: 860px)` (`4621-4734`) and
`@media (max-width: 560px)` (`4735-4747`). Splitting a container between two lanes
invites a merge conflict on every line. **Decision:**

- `2315-2363` splits three ways: `2316-2332` **L6** (`body.comments-open`,
  `.comments-overlay`, `.comments-rail`); `2333-2362` **L8** (the
  `.status-strip, .claim-group, .claim` width reset and every `.status-strip*`
  rule, including the `.status-strip-head, .claim-collapse-toggle { min-height:
  44px }` pair at `2340-2343`, on which **L3 is a dependent** per OD3).
- `4582-4620` is **wholly L2's** — every declaration in it is facet-TOC,
  content-area or sidebar.
- `4621-4734` splits three ways: `4622-4706` **L2** (chrome, ending at
  `.subtab { flex: none; }`), `4707-4724` **L3** (card, `.k`, the chip slot,
  `.pill`, `.claim-body`), `4726-4729` **L4** (footer counts). `4730-4732`
  (`.track-head`, `.track-title`, `.track-cite`) belongs to the track sections
  and is **unowned** — no lane may edit it.
- `4512-4580` is a graph-only container and is **wholly L9's**.
- `4735-4747` is **wholly L4's**.

**Any lane adding a new mobile rule adds it in its own new `@media` block
appended after `style.css:4747` and before the print doc comment at `4748`,
tagged with a `/* lane: Lx */` comment** — never by editing a shared container in
place. This is also what makes the new rules win: equal specificity, later source
order.

**OD2 — The palette blocks, the two dark bodies and the single print block are
coordinator-held; no lane owns them.** This replaces the previous rule that made
L1 their caretaker. `style.css:63-167`, `169-175`, `176-200`, `202-226` and
`4748-4870` are edited by the coordinator only. The print block must stay last in
the file, because it beats screen layout rules at equal specificity on source
order alone, and six lanes editing it independently would break that invariant
and `internal/render/theme_tokens_test.go TestStyleCSSModeAndPrintStructure` with
it. **A lane needing a print rule, a palette change or a dark twin states it in
its step-7 report; the coordinator lands it.** The one crossing worth naming:
`style.css` re-declares `--dxg-facet-other` inside the palette blocks
(`:76`, `:189`, `:215`) and that declaration wins over `graph.css:223` — it is
coordinator-held, and **L9 changes it only by request**, saying so in its report.

**OD3 — The two `@media (pointer: coarse)` blocks go to L6, with L3 and L8 as
dependents.** `style.css:2298-2314` sets `min-height: 44px` on seven controls in
one selector list — five of them comments', plus `.claim-collapse-toggle` (L3)
and `.status-strip-head` (L8). **Decision:** L6 owns both coarse blocks intact;
L3 and L8 add new coarse rules in their own appended blocks per OD1 and must not
edit the selector list. The same rule governs the `2340-2343` pair inside the 860
sheet container, which L8 owns and L3 depends on. The 44px minimum is not a style
preference — it is the smallest target a thumb hits reliably, and it is the
existing precedent every mobile rule in the revamp extends.

**OD4 — A screen is signed off by exactly one lane; other lanes verify their
region only.** 02 is signed off by L2 even though L3, L4, L6 and L8 all paint on
it; 07 is signed off by L3 even though L4 paints its footer strip and L7 paints
its checks panel. The alternative — every lane that touches a screen signing it
off — produces four contradictory verdicts on the same board and no owner. The
primary lane is the one owning the sections that carry the screen's argument.

**OD5 — Every lane now has a signed-off screen, and the coverage is exactly
eleven.** L2 → 02, 03. L3 → 07, 07a. L4 → 05. L5 → 06. L6 → 14, 14a. L7 → 06a.
L8 → 04. L9 → 13. Eleven screens, eight lanes, no screen signed twice and none
signed by nobody. The former OD5 ("L5 and L8 ship without a signed-off screen,
and that is deliberate") is **withdrawn**: it was a consequence of the parked
list, and the parked list is gone.

**OD6 — No lane owns the markdown body constructs.** `style.css:379-780` (phase B
block constructs, phase C GFM tables, phase D images, inline citation markers) is
owned by no lane. No screen specifies it, and the coverage inventory records that
Cutainly's bodies contain no tables, images, task lists, blockquotes or links, so
nothing in that range can be verified against the client at all. L3 owns the
*prose* rules (`.claim-body` base at `332-378` and the live
`.claim-body, .sbody, …` rule at `4068-4075`) and nothing below 379.

**OD7 — `graph.css` and the `style.css` graph override block are L9's, not
frozen.** This replaces the previous freeze. `internal/render/viewer/template/
graph.css`, `graph-core.js`, `graph-ui.js`, `internal/render/graph_view.go`,
`internal/render/lazy_shell.go:77`, `render.go`'s graph plumbing,
`shell.html`'s graph pane markup and `style.css:4280-4580` (marker
`/* Claims graph: a first-class System Record view, not a utility overlay. */`)
are **L9's exclusive sections**. `internal/graph/*` is the payload package: L9
reads it and does not restyle it, and a payload change goes through the
coordinator. The z-index ledger's `80` / `81` rows for the graph pane are recorded
in `style.css:2128-2137` — inside **L8's** section — precisely because a ledger
split across two files is not a ledger; neither lane renumbers it alone.

**OD8 — The coverage inventory is present and is the fixture source of truth.**
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/coverage-inventory.md`
exists (2026-09-17 02:28) and every fixture named in the lane table is quoted
from it. Its own caveat stands and is passed through to every lane: **nothing in
it was rendered in a browser.** It is drawn from `catalog.json`, the CLI
envelopes and the claim YAML. A state marked "yes" means the data exists, not
that any card was seen to draw it. Step 3 of the lane contract is where that gets
established. Its provenance line is pinned to `3ac8844`; the counts are corpus
counts and do not move with the engine tree.

**OD9 — A spec's node ids are advisory; its measured values are binding.** The
residual verifier findings that held 04, 06, 07 and 13 back were node-citation
and line-drift errors — a table naming a container frame instead of the painted
node, an address off by the `8890919` shift. **A lane implements the values.** If
a node id in a spec does not resolve in Paper, the lane re-reads the value with
`get_computed_styles` / `get_jsx` against the board and records the corrected id
in its step-7 report. The known instance is **06 § 4.8's dark-column node ids**,
which name container frames; its values are right.
