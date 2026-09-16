# Viewer design tokens — the frozen set

Source of record: Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, board **01 · Foundations**
(node `1-0`) and board **00 · Components — source of truth** (node `377-0`), read
through `get_tokens`, `get_computed_styles` and `get_jsx`. Engine side read from
the worktree `feat/viewer-design-revamp` at `3ac8844`.

## How to read this

If you are a lane agent implementing one screen, the whole of what you read is:

1. **this file** — every value you are allowed to spell, and what it maps onto in
   the engine;
2. **`reference-rules.md`** — the rules the five reference boards and the
   components board bind you to, stated as rules rather than as pictures;
3. **your screen's own spec**;
4. **your screen's PNGs**, under
   `/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/paper-png/`.

You do not need to open Paper, and you must not re-derive a value from a PNG. A
PNG here tells you *intent* — what the screen is trying to say, what reads first,
what is deliberately quiet. Every *number* comes from this file. If a number you
need is not in this file, it was not measured; say so rather than estimating it
from a screenshot.

Two standing rules that outrank any board:

- **Approval is not proof.** These boards shipped with known defects, listed
  under "Disagreements" below. A board that paints a dark surface with a light
  hex is wrong even though it was approved. Implement the token, not the pixel.
- **Dark is expressed as dark tokens.** Every dark value below is a
  `--color-dark-*` twin. If a board shows a light hex on a dark surface, that is
  a defect to flag, not a value to copy.

---

## 1. Colour — surfaces and ink

Paper names the light token and its dark twin as two separate tokens. The engine
names one token and re-points it inside a dark query. The right-hand column is
the engine token a lane agent actually writes.

| Paper token (light) | Light value | Paper token (dark) | Dark value | What it paints | Engine token |
|---|---|---|---|---|---|
| `--color-paper` | `#EFF1F4` | `--color-dark-paper` | `#0D1117` | Page ground behind cards; the "sea fog" / "night water" field | `--paper` (engine light `#f6f8fc`, dark `#0a1220`) |
| `--color-card` | `#FFFFFF` | `--color-dark-card` | `#161B22` | Claim card, rail panel, sheet body — every raised surface | `--card-bg` (engine `#ffffff` / `#0f1b2e`) |
| `--color-ink` | `#101720` | `--color-dark-ink` | `#E6EAF0` | Claim titles, claim prose, headings, primary body text | `--ink` (engine `#091426` / `#e8eef8`) |
| `--color-muted` | `#54606F` | `--color-dark-muted` | `#9AA7B8` | Secondary interface text, chip labels, note prose | `--muted` (engine `#536179` / `#a9b5c8`) |
| `--color-faint` | `#6E7C8E` | `--color-dark-faint` | `#8494A8` | Captions, ids, counts at rest, placeholder text | `--faint` (engine `#7d899a` / `#75839a`) |
| `--color-border` | `#DDE2E9` | `--color-dark-border` | `#242C38` | Hairlines: chip outlines, row dividers, panel edges | `--border` (engine `#d8deea` / `#263754`) |
| `--color-border-strong` | `#C2CAD5` | `--color-dark-border-strong` | `#38424F` | Section rules on the components board, sheet grabber, resolve-pill outline | `--border-strong` (engine `#aab5c7`; **no dark override in the engine** — see Disagreements) |
| `--color-accent` | `#1C4E8C` | `--color-dark-accent` | `#6AA6E8` | Navigation only — links, open-chip label and border, active controls, "Reply", the mobile comment trigger. Board 01, node `2C-0`: "Navigation only. Never overlaps a status meaning." | `--link` (engine `#205b78` / `#8ab7ff`) — **not** `--accent` |
| `--color-accent-bg` | `rgb(28 78 140 / 9%)` | `--color-dark-accent-bg` | `rgb(106 166 232 / 14%)` | Tint behind the accent: the mobile comment-count pill, active nav | **no engine token** — the engine's `--accent-bg` is tied to `--accent` (green), not to `--link` |
| `--color-locked` | `#2C6B52` | `--color-dark-locked` | `#63BE9A` | LOCKED status: padlock, chip label, status dot, resolve tick. Board 01, node `1X-0`: "Approved and frozen. Changes only via unlock → fix → lock." | `--accent` (engine `#287052` / `#70c99c`) |
| `--color-locked-bg` | `rgb(44 107 82 / 10%)` | `--color-dark-locked-bg` | `rgb(99 190 154 / 13%)` | Tint behind the LOCKED chip | `--accent-bg` (engine `rgba(40,112,82,.12)` / `rgba(112,201,156,.12)`) |
| `--color-draft` | `#9A6A16` | `--color-dark-draft` | `#DDA94E` | DRAFT status, and the single stale-freshness state. Board 01, node `22-0`: "Free to edit. Carries no approval and gates nothing." | `--status-draft` (engine `#976600`; **no dark override in the engine**) |
| `--color-draft-bg` | `rgb(154 106 22 / 11%)` | `--color-dark-draft-bg` | `rgb(221 169 78 / 13%)` | Tint behind the DRAFT chip and the Blocker severity pill | `--status-draft-bg` (engine `rgba(151,102,0,.12)`; no dark override) |
| `--color-blocked` | `#9E3B36` | `--color-dark-blocked` | `#EC8A83` | BLOCKED: notice icon and text, blocked chip, Critical / Needs-you severity, integrity findings. Board 01, node `27-0`: "A required dependency is unapproved." | `--warn` (engine `#a2433d` / `#ff8b94`) |
| `--color-blocked-bg` | `rgb(158 59 54 / 10%)` | `--color-dark-blocked-bg` | `rgb(236 138 131 / 13%)` | Tint behind a blocked chip or notice | `--warn-bg` (engine `rgba(162,67,61,.10)` / `rgba(255,139,148,.10)`) |
| `--color-code-bg` | `#F3F5F8` | `--color-dark-code-bg` | `#1B212B` | Fenced blocks, inline code, the evidence panel on board 12 | `--code-bg`, and `--code-inline-bg` for inline spans (engine derives both from a `color-mix` of `--paper` into `--card-bg`) |
| — | — | `--color-dark-blocked-surface` | `#1D1618` | The dark-mode ground of a blocked notice card. Intent: on dark, a 10 % red tint over `#0D1117` is invisible, so the notice gets a *surface* colour rather than a tint | **no engine token** — see Disagreements |
| — | — | `--color-dark-blocked-hairline` | `#3A2A2B` | The 1px edge of that dark blocked notice | **no engine token** |

### Engine-only tokens the design does not name

These are in the 28-token allowlist (`ThemeTokenAllowlist`,
`internal/config/config.go:159`) but have no Paper token. Leave them at their
engine defaults unless a screen spec says otherwise; a lane agent must not invent
a value for them.

| Engine token | Light | Dark | Notes |
|---|---|---|---|
| `--table-head-bg` | `rgba(127,127,127,.10)` | same | Mode-invariant by design — a neutral overlay tint tracks whatever it sits on |
| `--image-bg` | `rgba(127,127,127,.06)` | same | Mode-invariant |
| `--hover-bg` | `rgba(125,137,154,.08)` | same | Mode-invariant |
| `--shadow` | `rgba(0,0,0,.08)` | `rgba(0,0,0,.28)` | One of the three that genuinely differ per mode |
| `--shadow-strong` | `rgba(0,0,0,.14)` | `rgba(0,0,0,.34)` | Differs per mode |
| `--shadow-cast` | `rgba(9,20,38,.12)` | same | Mode-invariant |
| `--scrim` | `rgba(0,0,0,.22)` | `rgba(0,0,0,.42)` | Differs per mode. This is the bottom-sheet scrim (board J) |
| `--selection-bg` | `rgba(40,112,82,.20)` | same | Text selection |
| `--mockup-bg` | `#fff` | same | Deliberately mode-invariant |
| `--radius` | `6px` | — | The engine's only radius token |
| `--font-sans` | `"Avenir Next", -apple-system, BlinkMacSystemFont, "Inter", "Segoe UI", sans-serif` | — | |
| `--font-mono` | `ui-monospace, "SFMono-Regular", "IBM Plex Mono", Menlo, monospace` | — | |

That is 28 keys total: the original 14 (`accent`, `accent-bg`, `ink`, `muted`,
`faint`, `paper`, `card-bg`, `border`, `link`, `warn`, `warn-bg`, `font-sans`,
`font-mono`, `radius`) plus the 14 added for the custom-theme work
(`code-inline-bg`, `code-bg`, `table-head-bg`, `image-bg`, `hover-bg`,
`border-strong`, `shadow`, `shadow-strong`, `shadow-cast`, `scrim`,
`selection-bg`, `status-draft`, `status-draft-bg`, `mockup-bg`). The list order
is load-bearing — `internal/render` emits declarations in it — so nothing is
inserted, only appended.

### How the engine's light/dark mechanism works, and what that means for you

`internal/render/viewer/template/style.css` is **light-first**: one unconditional
`:root` carries the light values, which are also the printed values, and two
blocks re-point the handful that invert — `@media screen { html[data-theme="dark"] }`
for the explicit toggle and `@media screen and (prefers-color-scheme: dark) { :root }`
for the OS. The `screen and` prefix is load-bearing: without it, a reader on a
dark OS who hits Print gets near-black paper. There is exactly one `@media print`
block and it is the last block in the file.

Consequences a lane agent must respect:

- Spell a consumer as `var(--token, <light literal>)`. The **fallback is always
  the light value**, because the sheet is light-first and the fallback only ever
  fires for a `template_overrides` project that forked the file and dropped the
  `:root`.
- A token has to be read by the declaration that **wins**, not merely by a
  declaration. `code` and the fenced-block list are re-declared later at equal
  specificity, so their `var()` must live in the later copy.
  `internal/render/theme_tokens_test.go` asserts this per token and fails if an
  allowlisted token has no consumer in `style.css` or `graph.css`.
- Print always uses the light palette. A `dark:` value never reaches paper, even
  for a token declared only under `dark:` (`docs/theming.md`).

---

## 2. Graph colour — the two ramps and their opposite conventions

`graph.css` keeps the **opposite** convention to `style.css` and says so in its
own header: its unconditional `:root` holds the **dark** values, and
`@media (prefers-color-scheme: light), print` overrides to light. It is injected
as the **first** `<style>` block, before `style.css` and before the generated
theme block, so both cascade over it.

Categorical colour is deliberately outside the theme allowlist: the allowlist
carries one accent, and the facet ramp needs twenty mutually distinguishable
hues. Slots 1–5 are the requirements prototype's frozen values and are not
negotiable. Slots 6–20 are solved as a maximin CIEDE2000 problem, not a hue
rotation, and hold a minimum pairwise distance of 10.8 (light) / 11.8 (dark).

Paper carries only the nine that the viewer chrome touches. Slots 6–20 have no
Paper token and must be read from `graph.css`.

| Paper light token | Light value | Paper dark token | Paper's dark value | Engine `graph.css` dark | Agrees? | What it paints |
|---|---|---|---|---|---|---|
| `--color-graph-facet-1` | `#4257C4` | `--color-dark-graph-facet-1` | `#8E9BF0` | `#7C8CE8` | **no** | Facet slot 1; also the AGENT authorship label in a comment thread |
| `--color-graph-facet-2` | `#12897F` | `--color-dark-graph-facet-2` | `#12897F` | `#3FB3A6` | **no — light hex on a dark board** | Facet slot 2 |
| `--color-graph-facet-3` | `#B65A34` | `--color-dark-graph-facet-3` | `#B65A34` | `#DE8A62` | **no — light hex on a dark board** | Facet slot 3 |
| `--color-graph-facet-4` | `#7050A8` | `--color-dark-graph-facet-4` | `#A98FD8` | `#A98CD8` | **no — off by 3 in green** | Facet slot 4 |
| `--color-graph-facet-5` | `#67717E` | `--color-dark-graph-facet-5` | `#67717E` | `#8E9AA8` | **no — light hex on a dark board** | Facet slot 5 |
| `--color-graph-other` | `#7D8C85` | `--color-dark-graph-other` | `#7D8C85` | `#6C7F75` | **no — light hex on a dark board, and both are dead** | Reserved slot for a claim with no facet. **`style.css` re-declares `--dxg-facet-other` and, being emitted after `graph.css`, wins: the live value is `#0d55b5` light / `#77a9e8` dark.** |
| `--color-graph-cycle` | `#D1201A` | `--color-dark-graph-cycle` | `#FF6A62` | `#F5615C` | **no** | Cycle state on an edge — a state colour, not an identity colour |
| `--color-graph-halo` | `#C07E0C` | `--color-dark-graph-halo` | `#C07E0C` | `#EFB44D` | **no — light hex on a dark board** | Halo state on a node |
| `--color-graph-governed` | `#C11F5B` | `--color-dark-graph-governed` | `#E87F9B` | `#F06A9C` | **no** | Governance edge. Deliberately outside the ramp |

**All nine light values match `graph.css` exactly. None of the nine dark values
does.** Five of them (`facet-2`, `facet-3`, `facet-5`, `other`, `halo`) are the
light hex copied verbatim onto a dark board. Treat `graph.css` as authoritative
for every graph colour, light and dark, and treat Paper's `--color-dark-graph-*`
set as unusable until it is corrected.

---

## 3. Type scale

Read off board 01 node `2F-0` with `get_jsx`. `--text-meta` is defined in the
token set but has no row on the specimen board; it is nevertheless the most-used
size on the screen boards.

| Paper token | Size | Leading | Weight | Tracking | Family | What it sets |
|---|---|---|---|---|---|---|
| `--text-display` | `40px` | `46px` (**no token**) | `600` (`--font-weight-semibold`) | `-0.02em` (`--tracking-display`) | sans | Document title |
| `--text-h1` | `28px` | `36px` (**no token**) | `600` | `-0.02em` | sans | Facet heading |
| `--text-h2` | `20px` | `26px` (`--leading-title`) | `600` | `0em` (`--tracking-normal`) | sans | Claim title (desktop); components-board section headings |
| `--text-body` | `17px` | `28px` (`--leading-body`) | `400` (`--font-weight-regular`) | `0em` | **serif** | Claim prose. Measured at `max-width: 680px` on the specimen |
| `--text-ui` | `15px` | `20px` (`--leading-ui`) | `400`–`500` (`--font-weight-medium`) | `0em` | sans | Navigation, tabs, buttons, counts |
| — (**no token**) | `13px` | `20px` | `400` | `0em` | **mono** | Claim ids, code. The specimen's own sixth row; the token set has no 13px entry |
| `--text-meta` | `14px` | `20px` / `22px` per context | `400`–`600` | `0em` | sans (chrome) or serif (note prose) | Comment bodies, mobile row titles, reference-board note prose |
| `--text-label` | `12px` | `16px` (**no token**) | `600` | `+0.08em` (`--tracking-label`) | sans | Caps section labels, uppercase eyebrows |

Weights: `--font-weight-regular 400`, `--font-weight-medium 500`,
`--font-weight-semibold 600`. There is no 700 in the set and none on the boards.

Tracking: `--tracking-display -0.02em`, `--tracking-normal 0em`,
`--tracking-label 0.08em`. Boards also use `-0.01em` (mobile claim title 18px,
sheet header 16px), `+0.09em` (reference-board eyebrows), `+0.06em` (small caps
labels in code evidence and mobile relationship badges), `+0.05em` (mobile
LOCKED chip) and `+0.04em` (comment authorship labels). **None of those five has
a token.**

Leading: `--leading-body 28px`, `--leading-title 26px`, `--leading-ui 20px`. The
display (46px), h1 (36px) and label (16px) leadings have no token.

**Engine reality:** `style.css` carries no type-scale custom properties at all —
every size, weight, leading and tracking is a literal in a rule. The scale above
is therefore a *specification for what those literals must become*, not a set of
variables the engine already reads. There is no `--text-*`, `--leading-*`,
`--tracking-*` or `--font-weight-*` in the engine, and none of them is in the 28-
token allowlist, so a project cannot theme type. That is deliberate.

### Sizes observed on the boards that are off the scale

Measured, not estimated. Each one is either a deliberate context variant or
drift; a lane agent must not introduce a new one.

`26/32` (reference-board headings, boards 10, 11, 12) · `18/24` (mobile claim
title, component H2) · `16/22` (bottom-sheet header, component J1) · `16/20`
(code-evidence panel title, board 12) · `16/25` and `16/26` (reference-board
intro prose, serif) · `15/24` (components-board section note, serif, section H
only) · `14/22` (comment body; reference-board row prose) · `14/18` (specimen
annotation column; reference-board row titles) · `13/19` (mobile blocked-notice
body) · `13/20` (components-board section notes in sections I and J, sans) ·
`13/16` (mobile comment count, primary button label) · `12/18` (components-board
per-component notes) · `11/14` and `11/15` (mobile chip labels, ids, authorship
labels) · `10/12` (freshness band labels).

---

## 4. Spacing scale

| Token | Value |
|---|---|
| `--spacing-1` | `4px` |
| `--spacing-2` | `8px` |
| `--spacing-3` | `12px` |
| `--spacing-4` | `16px` |
| `--spacing-6` | `24px` |
| `--spacing-8` | `32px` |
| `--spacing-12` | `48px` |

The scale is a 4px grid with the 20px, 28px, 36px, 40px and 44px steps
deliberately absent. Intent: five steps is enough to build a reading surface, and
each omitted step is a step a designer would otherwise reach for to avoid making
a decision.

**Off-scale values measured on the boards** (every one is a real, intentional
micro-adjustment inside a component, not a layout step): `1px` (the gap that
makes a bordered stack read as hairline-divided rows), `2px`, `3px`, `5px`, `6px`,
`7px`, `9px`, `10px`, `11px`, `13px`, `14px`, `18px`, `19px`, `20px`, `22px`,
`26px`, `28px`, `36px`, `64px`. Use the scale for layout — the distance between
components — and treat inside-a-component padding as measured per component in
the screen spec.

---

## 5. Radii

| Token | Value | What it rounds |
|---|---|---|
| `--radius-sm` | `4px` | Inline code spans, slug chips inside a blocker row |
| `--radius-md` | `8px` | The comment composer field |
| `--radius-pill` | `999px` | Every chip, pill, status badge, severity token, the sheet grabber |

**Off-scale radii measured on the boards:** `5px` (mobile coverage step chip),
`6px` (primary button in the sheet composer), `10px` (blocked notice card,
reference-board panels, the code-evidence panel), `16px` (bottom-sheet top
corners — component J1 fixes this at 16px explicitly).

The engine has exactly one radius token, `--radius: 6px`, and it is themeable.
The design needs four distinct radii and the engine cannot express them as
tokens; they become literals.

---

## 6. Containers

| Token | Value | Intent |
|---|---|---|
| `--container-reading` | `720px` | The reading measure |
| `--container-rail` | `268px` | Right rail |
| `--container-toc` | `244px` | Left facet TOC |

**Conflict:** board 11 (`1E3-0`) states, in prose, "Claim prose stays at **760px**
in both modes. The paragraph you are reading is pixel-identical before and
after," and "The page grows to **1140px**." The token says 720. Board 01's prose
specimen is capped at `680px`. Three numbers for one measure. See Disagreements.

---

## 7. Font families and their roles

| Token | Family | Role, read off the boards |
|---|---|---|
| `--font-sans` | **Inter** | All chrome. Document title, facet heading, claim title, navigation, tabs, buttons, counts, every chip and pill label, status words, caps eyebrows, table headers, the placement-map table, mobile row titles, comment bodies |
| `--font-serif` | **Source Serif 4** | Running prose the reviewer reads for meaning. Claim prose at 17/28; every explanatory note on the reference boards (16/25, 16/26, 14/22); the components board's own section notes (15/24) and "how to use this board" text (16/26) |
| `--font-mono` | **IBM Plex Mono** | Machine identity and machine output. Claim ids and slugs, dependency-path slugs, counts inside pills, severity counts, code lines and line numbers, the freshness band labels (`< 24 h`, `1 – 7 d`, `> 7 d`), comment authorship labels (`HUMAN`, `AGENT`) and their elapsed times, the numbered markers on reference-board rule lists |

The division is not decorative. Serif marks *what a human wrote and a human must
judge*; sans marks *what the interface says about it*; mono marks *what the
machine emitted*. A claim's prose is serif and the same claim's id is mono for
exactly that reason.

### The engine has no serif

`style.css` declares `--font-sans` and `--font-mono` only. The three matches for
"serif" in the file are all the `sans-serif` tail of a stack. There is **no
`--font-serif` token, no serif face, and no serif in the 28-token allowlist**, so
a project cannot theme one either. Shipping `--text-body` in Source Serif 4
requires a new engine-owned family — this is the single largest engine gap the
foundations work exposes.

### The Geist mechanism, and why it is the precedent

`internal/render/geist_fonts.go` inlines two engine-owned faces as base64
`data:` URLs so a viewer that names them loads from the single HTML file:

```
@font-face{font-family:Geist;src:url(data:font/woff2;base64,…) format("woff2");
           font-weight:100 900;font-style:normal;font-display:swap}
@font-face{font-family:"Geist Mono";src:url(data:font/woff2;base64,…) format("woff2");
           font-weight:100 900;font-style:normal;font-display:swap}
```

Sources: `viewer/template/fonts/geist-latin-wght.woff2` and
`geist-mono-latin-wght.woff2`. Both are variable faces declared `100 900`. They
exist for the `claude` preset's stacks. A project stylesheet override replaces
`style.css` wholesale and does **not** receive these faces.

This is the pattern a serif face would follow: an engine-owned variable woff2,
inlined as a `data:` URL, named by an engine-owned token.

The *project-supplied* font path is different and is capped:
`viewer.theme.fonts` inlines a project's own files as base64, every declared
`family` must appear in the merged `font-sans`/`font-mono` value or the theme is
refused, and **total raw font bytes across every face are capped at 2 MiB** —
roughly four variable faces (`docs/theming.md`). `check` reports
`data.theme_font_count` and `data.theme_font_bytes`, which are *absent* from
`data`, not zero, when a theme declares no fonts.

---

## 8. Breakpoints — the mapping rule

Paper carries three breakpoint tokens:

| Token | Value |
|---|---|
| `--breakpoint-mobile` | `390px` |
| `--breakpoint-tablet` | `834px` |
| `--breakpoint-desktop` | `1440px` |

**These are canvas widths, not CSS breakpoints. Do not emit any of them.** The
designs are drawn at 1440 (desktop) and 390 (mobile) because those are the two
devices worth drawing, not because the stylesheet should change at those widths.

The engine's breakpoints, as they exist in `style.css` today:

| Engine query | Line(s) | Tier |
|---|---|---|
| `@media (min-width: 1181px)` | 3378, 3391 | Widest desktop; the rail-reserving layout |
| `@media (min-width: 861px) and (max-width: 1180px)` | 4494 | Narrow desktop |
| `@media (min-width: 861px)` | 3157 | Anything above the tablet fold |
| `@media (max-width: 860px)` | 1160, 2227, 2932, 4424, 4533 | Tablet and below — rails collapse here |
| `@media (max-width: 640px)` | 2563 | Narrow tier |
| `@media (max-width: 560px)` | 4647 | A fifth, narrower tier the brief did not list; today it owns `.claim-links-summary` and `.claim-footer__counts` |
| `@media (max-width: 520px)` | 1175 | Phone tier |
| `@media (pointer: coarse)` | 1764, 2211 | Touch, independent of width |

**A 390px breakpoint is not to be added.** Every rule a 390 board implies maps
onto an existing query:

| 390 board rule | Engine query it lands in | Why |
|---|---|---|
| H1 · blocked notice becomes an inset card with a divided action row | `max-width: 520px` | Phone tier; 390 < 520 |
| H2 · status chip moves above the claim title | `max-width: 520px` | Phone tier |
| H3 · footer strip splits into two rows (status + comment count, then detail pills) | `max-width: 520px` | Phone tier. Two existing responsive rules already touch `.claim-footer__counts` — `style.css:4640–4641` inside `max-width: 860px` (opened at 4533) and `style.css:4655–4656` inside `max-width: 560px` (opened at 4647). The new two-row form goes in 520 so it wins over both; because the queries are equal-specificity it must be a **new** 520 block placed after 4658, not the existing one at `style.css:1175`. (Corrected: the `max-width: 640px` block at `style.css:2563` is `.gcp-*` console rules and carries no `.claim-footer__counts` declaration — see screen 14, OD14.11.) |
| I1 · blocker row stacks the dependency path | `max-width: 520px` | Phone tier |
| I2 · relationship row becomes two lines with a right-ranged badge | `max-width: 520px` | Phone tier |
| I3 · detail chip gains an accent border when open | `max-width: 520px` | Phone tier |
| I4 · coverage step chip is 24 × 28 with a 3px gap "below 400px" | `max-width: 520px` | Phone tier. 400 is where the board measured the fit; 520 is where the engine expresses it, and the strip still must not wrap at 520 |
| J · bottom sheet: scrim, 16px top corners, 36 × 4 grabber, header hairline, 240–660px height band | `max-width: 520px` for the layout | The sheet only exists on phones |
| J · 44px minimum tap target; grabber as a drag affordance | `(pointer: coarse)` | Touch, not width. The engine already sets `.comment-chip { min-height: 44px }` here |
| J2 · comment count takes the accent instead of faint | `max-width: 520px` | It stops being a passive count and becomes the control that opens the sheet |
| Mobile nav sheets (modules, on-this-facet) | `max-width: 520px` + `(pointer: coarse)` for tap sizing | Same split: arrangement by width, target size by pointer |
| 1440 desktop boards | `min-width: 1181px` | The widest tier |
| 1220-wide claim boards (05, 06, 06a, 07, 07a) | `min-width: 861px and max-width: 1180px` is where they narrow | The claim column at 1220 sits inside the narrow-desktop tier |

Rule of thumb for a lane agent: **arrangement is a width question and belongs in
`max-width: 520px`; target size and hover-less affordances are a pointer question
and belong in `(pointer: coarse)`.** Do not put a 44px minimum behind a width
query, and do not put a stacking change behind `pointer: coarse`.

---

## Disagreements

Every item is a measured conflict between Paper's token set, the boards, and the
engine. Ordered by how much damage it does if implemented as drawn.

1. **Five of the nine `--color-dark-graph-*` tokens are light hexes.**
   `--color-dark-graph-facet-2` `#12897F`, `-facet-3` `#B65A34`, `-facet-5`
   `#67717E`, `-graph-other` `#7D8C85` and `-graph-halo` `#C07E0C` are byte-for-byte
   their own light twins. The engine's dark ramp has `#3FB3A6`, `#DE8A62`,
   `#8E9AA8`, `#6C7F75` and `#EFB44D`. A light hex on a dark board is a defect.
   *Resolution: `graph.css` wins; Paper's dark graph set is not usable.*

2. **The other four dark graph tokens also disagree with the engine.**
   `facet-1` `#8E9BF0` vs `#7C8CE8`; `facet-4` `#A98FD8` vs `#A98CD8` (three
   units of green apart, which reads as a transcription slip); `cycle` `#FF6A62`
   vs `#F5615C`; `governed` `#E87F9B` vs `#F06A9C`. The engine's values are the
   output of a maximin CIEDE2000 solve seeded on frozen slots 1–5; changing one
   moves the ramp's minimum distance. *Resolution: `graph.css` wins.*

3. **`--dxg-facet-other` is dead in `graph.css`.** `style.css` re-declares it
   (`#0d55b5` light, `#77a9e8` dark) and is emitted after `graph.css`, so it
   wins. `graph.css`'s own `#7D8C85` / `#6C7F75` never paints, and Paper's
   `--color-graph-other` / `--color-dark-graph-other` copy the values that never
   paint. Three sources, one live value. *Resolution: the live value is
   `#0d55b5` / `#77a9e8`; the Paper token is documentation of a dead branch.*

4. **`--color-faint` has two values.** The token is `#6E7C8E`. Board 01's own
   light swatch is labelled `#8592A3 · faint` (node `T-0`), and five nodes across
   boards 08 and 12 are painted `#8592A3` literally. *Resolution: the token
   `#6E7C8E` wins; `#8592A3` is pre-token residue.*

5. **The reading measure has three values.** `--container-reading` is `720px`;
   board 11 states in prose that claim prose is `760px` in both modes and that
   focus grows the page to `1140px`; board 01's prose specimen is capped at
   `680px`. *Resolution: see Open decisions.*

6. **The engine has no serif family and no way to add one through a theme.**
   `--text-body` is specified as Source Serif 4 and there is no `--font-serif`
   token in `style.css` and none in the 28-token allowlist. Every serif on every
   board is currently unimplementable.

7. **`--color-accent` is not the engine's `--accent`.** Paper's accent is
   navigation navy and maps onto the engine's `--link`; Paper's `--color-locked`
   is the approval green and maps onto the engine's `--accent`. Reading the two
   names as cognates inverts every link and every lock in the viewer.

8. **`--color-accent-bg` has no engine home.** The engine's `--accent-bg` is
   bound to `--accent` (green). The tint behind a navigation accent — the mobile
   comment trigger, the active nav item — has no allowlisted token, and the
   allowlist is closed.

9. **Three dark twins the engine does not have.** `--color-dark-border-strong`
   `#38424F`, `--color-dark-draft` `#DDA94E` and `--color-dark-draft-bg` are real
   Paper tokens, but `--border-strong`, `--status-draft` and `--status-draft-bg`
   have **no dark override** in `style.css` — they sit in the unconditional
   `:root` only. On dark today, `--border-strong` renders `#aab5c7` and
   `--status-draft` renders `#976600`. The boards assume otherwise.

10. **`--color-dark-blocked-surface` and `--color-dark-blocked-hairline` have no
    light twin and no engine token.** They encode a real rule — on dark, a 10 %
    red tint over `#0D1117` is invisible, so a blocked notice needs a surface
    rather than a tint — and there is nowhere to put it.

11. **Radii and type cannot be themed and only partly exist.** The engine has one
    radius token against the design's three (plus four off-scale literals), and
    no type tokens at all against the design's eight sizes, three weights, three
    trackings and three leadings.

12. **The boards do not use the tokens consistently.** Boards 01, 08, 10, 11 and
    12 spell families as `var(--font-sans)`; the components board (377-0) and
    every mobile board spell them as the literal `"Inter", system-ui, sans-serif`.
    Colour is worse: the components board's mobile sections use raw hexes with
    baked alpha — `#9E3B360F`, `#9E3B3633`, `#9E3B3617`, `#9E3B361A`,
    `#9E3B361F`, `#9E3B362E`, `#2C6B521A`, `#2C6B5214`, `#E8ECF1`, `#EFE6E5` —
    where `--color-blocked-bg`, `--color-locked-bg` and `--color-border` exist.
    Board 12's evidence panel is `#F6F8FA` where `--color-code-bg` is `#F3F5F8`,
    and its line numbers are `#A9B3C1` with no token at all. Board 11's DECIDED
    panel is bordered `#A9CBBB`, a lightened lock green with no token.
    *Resolution: implement the token, never the literal. A raw hex on a board is
    evidence of what the designer meant, not permission to ship it.*

13. **The components board's own section notes are not one family.** Section H's
    note is Source Serif 4 15/24; sections I and J are Inter 13/20. Within the
    board that calls itself the source of truth.

14. **Board 01's two swatch rows are asymmetric.** The light row shows paper,
    card, ink, muted, faint, accent; the dark row shows paper, card, ink, muted,
    **border**, accent. Neither row shows `--color-border-strong`, the status
    four, `--color-code-bg`, or either blocked-surface token. The board is a
    specimen, not the set; the set is `get_tokens`.

15. **`--text-meta` (14px) has no row on the specimen, and the specimen's 13px
    mono row has no token.** The two most-used sizes on the screen boards are the
    two the type specimen handles worst.

16. **The engine has a `max-width: 560px` breakpoint the brief did not list.** It
    is real (`style.css:4647`) and it already owns `.claim-footer__counts`, which
    the mobile footer rule changes. A 520px rule for the footer must therefore be
    written to win over it.

## Open decisions

Recorded rather than asked, per the freeze protocol.

- **Reading measure: use 760px.** Board 11 is the only place that states the
  measure as a *rule* with a reason attached ("the paragraph you are reading is
  pixel-identical before and after" — focus must not reflow prose), and a rule
  with a reason beats a token with none. `--container-reading: 720px` is
  corrected to `760px`; board 01's `680px` is a specimen cap, not the measure.
  Focus mode's page width is `1140px`.

- **Dark graph colour: `graph.css` is authoritative, full stop.** Paper's
  `--color-dark-graph-*` set is not implemented and is not corrected in Paper by
  this freeze (Paper is read-only for this lane). Lane agents read graph colour
  from `internal/render/viewer/template/graph.css`, and `--dxg-facet-other` from
  `style.css` because that is the declaration that wins.

- **Serif: treat `--font-serif` as a new engine-owned token, not a theme token.**
  The allowlist is closed and its order is load-bearing; appending a serif key
  would let a project set a serif and still leaves the engine with no face to
  fall back to. The Geist mechanism in `internal/render/geist_fonts.go` is the
  precedent: an engine-owned variable woff2 inlined as a `data:` URL, named by an
  engine-owned token that is *not* themeable. Sizing this face against the 2 MiB
  project-font cap is out of scope for foundations.

- **Navigation accent tint: use `color-mix(in srgb, var(--link) 9%, transparent)`
  rather than adding a `link-bg` token.** The allowlist is closed, and the tint
  is derivable from the token that is already there.

- **Off-scale spacing, radii and type sizes stay as per-component literals.** The
  scales govern layout; a component's internal padding is measured in that
  component's own spec. Do not widen the scales to swallow `5px`, `7px`, `11px`
  or `26/32` — that would turn a deliberate micro-adjustment into a sanctioned
  step and the scale would stop meaning anything.

- **390px breakpoint: not added, as instructed.** Every mobile rule maps onto
  `max-width: 520px`, with touch-target rules on `(pointer: coarse)`, per the
  table in section 8.

## Export failures

None. All 55 boards exported to PNG at 1x on the first attempt and are present
and verified as PNG under
`/Users/nitinkhanna/Desktop/artifacts/2026-09-17-dossierx-design-revamp-implementation/paper-png/`.
