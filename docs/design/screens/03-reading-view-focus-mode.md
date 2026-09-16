# 03 · Reading view — focus mode

Screen group 03 of the viewer design revamp. Source of record: Paper file
`01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`. Engine side read from the worktree
`feat/viewer-design-revamp` at `3ac8844`.

Read this with `../tokens.md` (every value you are allowed to spell) and
`../reference-rules.md` (the rules this screen is bound by). Where this file
names a colour, the token name is `tokens.md`'s; the hex beside it is what the
board actually painted, recorded so a reviewer can see whether the board and the
token agree. **Where they disagree the token wins** — every disagreement found on
this screen is listed under "Paper defects".

Every number below was read with `get_computed_styles` or `get_jsx`. Nothing here
was measured off a screenshot.

---

## 1. Boards

| Node | Name | Size | What it shows | Viewer states depicted |
|---|---|---|---|---|
| `13J-0` | 03 · Reading view — focus mode | 1440 × 1024 | Desktop light, focus **on**. Both rails gone; one 1140px card column holding a blocked banner and two claim cards. | Module `06 / 26` "Permission readiness"; readiness 31 of 31 locked; facet tabs Contract (active, 26) / Internals (5); facet-level blocked banner; claim 1 LOCKED + blocked, readiness expansion **open**, comment count 0; claim 2 LOCKED + blocked, all expansions closed, `No checks declared`, comment count 2. Focus control in its **active** state. |
| `594-0` | 03 · Reading view — focus mode · DARK | 1440 × 1024 | The same composition on dark. | Identical states to `13J-0`. Only the palette changes; no layout value differs. |
| `5DS-0` | 03 · Focus mode · MOBILE LIGHT | 390 × 844 | Phone light. App bar, module head, facet tabs plus an `On this facet` trigger, one claim with the readiness expansion open. | LOCKED chip **above** the title; two-row footer (blocked chip + comment count, then three detail pills); readiness blockers open with the hop pill right-ranged and the path wrapped. **No Focus control. No blocked banner.** |
| `5H1-0` | 03 · Focus mode · MOBILE DARK | 390 × 844 | The dark twin of `5DS-0`. | Identical states to `5DS-0`. |
| `4VO-0` | Group 03 — band | 3860 × 31 | The group's title band: `03 · READING VIEW — FOCUS MODE` in `--color-accent`, caption `four designs of one screen · light pair left, dark pair right` in `--color-faint`, over a 2px `--color-accent` rule. | — (chrome for the canvas, not the product) |
| `5KP-0` | Group 03 — notes | 3860 × 61 | Three note columns, 900px each, 64px apart: `WHAT FOCUS MODE DOES`, `WHY MOBILE HAS NO FOCUS CONTROL`, `THE BLOCKER ROW ON MOBILE`. | — |
| `1E3-0` | 11 · Focus mode — notes / One control, not two toggles | 560 × fit | The reference board that governs this screen: four numbered rules and one `DECIDED` panel. | — |

Caption frames read but not measured: `5K9-0`, `5KD-0`, `5KH-0`, `5KL-0`.

Supporting nodes read for measurement (all descendants of the above).

*Desktop light `13J-0`:* `159-0`, `176-0`, `17E-0`, `17R-0`, `17S-0`, `17T-0`,
`17F-0`, `17G-0`, `17M-0`, `17H-0`, `17K-0`, `17J-0`, `17I-0`, `17N-0`, `17P-0`,
`17Q-0`, `177-0`, `178-0`, `179-0`, `17A-0`, `17B-0`, `17C-0`, `17D-0`, `15A-0`,
`15B-0`, `170-0`, `163-0`, `16R-0`, `16N-0`, `16Q-0`, `1OS-0`, `1OT-0`, `1OU-0`,
`34R-0`, `31D-0`, `15C-0`, `35H-0`.

*Desktop dark `594-0`:* `595-0`, `596-0`, `597-0`, `59B-0`, `59F-0`, `59G-0`,
`59H-0`, `59I-0`, `59K-0`, `59L-0`, `59U-0`, `59V-0`, `59W-0`, `5A3-0`, `5A4-0`,
`5AD-0`, `5AE-0`, `5AF-0`, `5AG-0`, `5AI-0`, `5B8-0`, `5CQ-0`.

*Mobile light `5DS-0`:* `5DT-0`, `5DU-0`, `5E4-0`, `5EB-0`, `5EJ-0`, `5EO-0`,
`5EP-0`, `5MI-0`, `5FL-0`.

*Mobile dark `5H1-0`:* `5H2-0`, `5H3-0`, `5HC-0`, `5HJ-0`, `5HR-0`, `5HW-0`,
`5HX-0`, `5HY-0`, `5I6-0`, `5IT-0`, `5N9-0`.

**A node id in this file names the box that carries the value, not the group the
value renders inside.** Two attributions in the first draft named a parent — the
prose wrapper `16N-0` for the 760px measure, and the Focus *group* `17G-0` /
`59F-0` for the Focus *button*'s geometry — and both are corrected throughout.
Where a group and its child both matter, both are listed.

---

## 2. Design intent

### What the reviewer should perceive

Focus mode is not a layout. It is **one reversible state a reader enters when
they want the surroundings to stop talking**, and the whole design of this screen
follows from that single idea.

When a reviewer hits `F`, the left module navigation and the right facet TOC both
leave. Nothing announces this. The page does not shift the paragraph they were
reading: the claim prose holds its 760px measure before and after, and the
sentence under the cursor is at the same x, at the same width, in the same face.
What *does* change is everything that was cramped beside it — the blocked banner,
the readiness blockers, the dependency breadcrumbs, the hop pills — because the
card grows from its rail-flanked width to 1140px and the evidence spreads into
the room the rails gave up. The reader's experience is therefore "the noise
stopped and the proof got bigger", never "the page rearranged itself".

The only thing that says the mode is on is the Focus control itself, which stays
in the header lit in `--color-accent` over a 9 % accent tint. It is the
affordance, the state indicator and the exit in one 28px button. There is no
banner, no hint strip, no toast.

On a phone the mode does not exist, and this is not an omission. Focus removes
the rails; a phone never had them; so there is nothing to remove and no control
for it. The mobile boards in this group are therefore not "focus on a phone" —
they are what focus mode is *about*: one claim with its readiness blockers open,
reading at full width.

### The rules that bind this screen

**From `reference-rules.md`, board 11 (`1E3-0`) — this screen's governing board:**

- **R11.1 — one reversible state, not two independent toggles.** Focus replaces
  per-rail show/hide. Binds `13J-0`, `594-0`, `5DS-0`, `5H1-0`, and the header on
  boards 02, 04, 13, 14. The engine today violates this: it ships two independent
  toggles (§7).
- **R11.2 — the measure never changes.** Prose stays at 760px in both modes.
  Confirmed by measurement here — but the cap is **on the prose runs, not on any
  wrapper around them**. On light, `16N-0` is only the wrapper (`display: flex; flex-direction: column;
  gap: 16px`, width 1074px, no `max-width`, no font); the 760px cap and the
  Source Serif 4 17/28 `#101720` run sit on its two children, the paragraph
  `16Q-0` (`max-width: 760px`) and the `…more` line `1OS-0` (`max-width: 760px`,
  `align-items: baseline`, `gap: 7px`). On dark there is no wrapper at all: the
  paragraph `5AD-0` and the `…more` line `5AE-0` are direct children of the claim
  block `5A3-0` and each carries `max-width: 760px` itself. Same measure, two
  different structures — recorded as Paper defect 13.
- **R11.3 — the freed width goes to the evidence.** The page grows to 1140px.
  Confirmed: `15B-0` and `59V-0` are both `max-width: 1140px`, and the readiness
  expansion (`31D-0`, `5B8-0`) is the only child that goes full-bleed to 1138px
  by negative side margins.
- **R11.4 — the control is its own reminder.** No banner, no hint strip. Exits
  are `F`, a second click, or pushing the pointer to an edge to *peek* a rail
  without leaving the mode.
- **R11.5 — focus survives a reload and lives in `localStorage`.** Per-reader
  state, like the theme choice; never baked into the generated HTML.
- **R11.6 — DECIDED: focus does not collapse the claim list.** Hiding the rails
  and giving their width to the evidence is the whole of it. Reducing the page to
  one claim would break scroll position, the facet TOC's meaning and every deep
  link into the facet.

**From the rest of `reference-rules.md`:**

- **R09.1 / R-F.1 — four expansions of one strip, in a fixed order, never four
  panels.** The footer strip on `34R-0` and `35H-0` is one line at rest carrying
  readiness, relationships, sources, checks, then the comment count hard right.
- **R09.2 — exactly one expansion open at a time.** `13J-0` shows readiness open
  on claim 1 and every expansion closed on claim 2; no board in this group shows
  two open.
- **R09.3 — auto-open is reserved for readiness, and only when blocked.** Claim 1
  is blocked and its readiness is open; this is the one auto-open the rules
  permit. Relationships never opens by default.
- **R09.6 — Issues is a screen and the banner is the only way in.** The desktop
  banner (`170-0`, `59W-0`) with its `Show issues` action is that entry point.
  The mobile boards omit it — flagged below.
- **R09.7 — the facet dot is the only project-level health signal in the reading
  view.** Focus hides the TOC, so in focus mode that signal is off screen and
  **nothing replaces it**. This is consistent with R11.4 (no substitute chrome)
  and is recorded so no lane invents one.
- **R09.9 — no inline dependency map.** The readiness expansion states each
  blocker's path as a breadcrumb of slugs; the graph pane draws the real shape
  via `See in claims graph`. Focus mode's extra width must not be spent on a map.
- **R10.1 / R10.5 — elapsed, not stamped; "Live" under `serve`.** These live in
  the right-rail footer, which focus hides. See §8.
- **R-F.2 / R-F.3 / R-F.4 — chip states and the mobile two-row footer.** These
  bind the mobile boards; the desktop footer is a bare label-and-chevron strip,
  which the components board does not specify (see Open decisions).
- **R-H.2 — status chip above the title on mobile.** Confirmed: `5EP-0` /
  `5HX-0` order the chip, then the title, then the id.
- **R-I.1 — the blocker row's dependency path stacks and is never dropped.** The
  mobile boards keep the path but arrange it differently from component I1 —
  flagged below.
- **R00.0 — the components board is the source of truth; where a screen and a
  reference board disagree, the reference board wins and the screen is flagged.**

### Every note on the band and the notes strip

**Band `4VO-0`** — two texts, both canvas chrome:

- `03 · READING VIEW — FOCUS MODE` (Inter 13/16, weight 600, `+0.08em`,
  `--color-accent`).
- `four designs of one screen · light pair left, dark pair right` (Inter 12/16,
  `--color-faint`). *Intent:* the four boards are four renderings of **one**
  screen, not four variants to choose between. A lane agent implements one thing.

**Notes strip `5KP-0`** — three columns, each 900px wide, 64px apart, eyebrow
Inter 11/14 weight 600 `+0.08em`, body Inter 13/20 `--color-muted`:

1. **`WHAT FOCUS MODE DOES`** (eyebrow `--color-muted`) — "Both rails go and the
   measure widens from 760 to 1140, so a blocker chain fits on one line. The
   Focus control stays lit to say the view is in that state, and F returns you."
   *Intent, paraphrased:* the justification for the mode is a concrete reading
   failure — a blocker chain wrapping — not a preference for minimalism. The lit
   control is load-bearing state, not decoration.
   **Note the wording collision:** "the measure widens from 760 to 1140" reads as
   though the *prose measure* grows, which R11.2 forbids and which the board's
   own `max-width: 760px` on the prose runs `16Q-0` / `1OS-0` (dark `5AD-0` /
   `5AE-0`) contradicts. What widens is the **page**.
   Board 11 states this correctly ("Claim prose stays at 760px… The page grows to
   1140px"); the notes strip is loose. Recorded as a Paper defect.
2. **`WHY MOBILE HAS NO FOCUS CONTROL`** (eyebrow `--color-blocked` — the strip's
   one coloured eyebrow, marking the note most likely to be read as an omission)
   — "Focus mode exists to take the rails away. Mobile never shows them, so there
   is nothing for it to remove and no control for it. The mobile boards here show
   what 03 is really about — one claim with its readiness blockers open."
   *Intent:* the absent control is a decision with a reason, not an unfinished
   board. A lane agent must not add a phone Focus button.
3. **`THE BLOCKER ROW ON MOBILE`** — "The hop distance moves up beside the
   blocker name and the dependency chain wraps below it instead of sitting in a
   fixed right column. Same three facts, stacked rather than ranged."
   *Intent:* the three facts (title, hop distance, path) are invariant; only
   their arrangement changes with width. **But the arrangement this note
   describes is not the arrangement component I1 specifies** (R-I.1 puts the hop
   pill after the title and stacks the path one slug per line behind a leading
   chevron). The board contradicts the component board and its own note endorses
   the contradiction — approval is not proof. Recorded as a Paper defect; R00.0
   resolves it in the component board's favour.

**Board 11 `1E3-0`, read in full** (its four rules and the DECIDED panel are
quoted verbatim in `reference-rules.md` R11.1–R11.6). Its own measured chrome:
eyebrow `BOARD 11 — FOCUS MODE` Inter 11/14 weight 600 `+0.09em`
`--color-faint`; heading `One control, not two toggles` Inter 26/32 weight 600
`-0.02em` `--color-ink`; intro Source Serif 4 16/26 `--color-muted`; rules panel
`--color-border` ground with a 1px `--color-border` border, 10px radius and a
1px gap between rows so the rows read as hairline-divided; row padding 15px
block / 18px inline, gap 14px; marker column mono 12/16 `--color-faint` at a
fixed 18px width; row title Inter 14/18 weight 600 `--color-ink`; row body Source
Serif 4 14/22 `--color-muted`. DECIDED panel: `#A9CBBB` 1px border (no token —
see `tokens.md` Disagreement 12), 10px radius, 16px block / 18px inline padding,
label `DECIDED` Inter 11/14 weight 600 `+0.08em` in `#2C6B52`
(`--color-locked`).

### Things on the board that contradict the board's own rules

Collected here in prose; itemised under "Paper defects" in §9.

- The light desktop board writes every colour as a **raw hex**; its dark twin
  writes **tokens**. The same value is therefore authoritative in one board and
  editorial in the other.
- The blocked tint takes **six different alphas** across this one screen (6 %,
  9 %, 10 % on light; 10 %, 13 %, 16 % on dark) where `--color-blocked-bg` /
  `--color-dark-blocked-bg` define exactly one per mode.
- Six hairline and ground colours on this screen have **no token at all**
  (`#E8ECF1`, `#FBF7F7`, `#EBD9D8`, `#EFE6E5` on light; `#212934`, `#33262A` on
  dark), while the dark blocked ground correctly uses
  `--color-dark-blocked-surface`.
- The mobile boards drop the blocked banner, which R09.6 makes the **only** way
  into the Issues screen and R-H.1 gives a specific mobile form.

---

## 3. Layout

### Desktop, 1440 (`13J-0` light, `594-0` dark — identical geometry)

| What | Value | Node |
|---|---|---|
| Artboard | `1440 × 1024`, `display: flex`, `overflow: clip` | `13J-0` / `594-0` |
| Page ground | `#EFF1F4` light / `var(--color-dark-paper)` dark | `13J-0` / `594-0`, `159-0` / `595-0` |
| Shell column | `flex: 1 1 0`, `height: 1024px`, `flex-direction: column`, `overflow: clip` | `159-0` / `595-0` |
| Header block | `padding-top: 36px`, `padding-inline: 48px`, `gap: 20px`, `align-items: center`, `flex-shrink: 0` | `176-0` / `596-0` |
| Header inner width | `1140px` (fixed), `justify-content: space-between`, `align-items: end` | `17E-0` / `597-0` |
| Identity column gap | `6px` | `17R-0` / `598-0` |
| Metric cluster gap | `8px`, `align-items: center` | `17F-0` / `59B-0` |
| Focus group (divider + button) | `gap: 10px`, `margin-left: 6px` | `17G-0` / `59F-0`, inside `17F-0` / `59B-0` |
| Tab strip width | `1140px`, `gap: 26px`, `border-bottom: 1px` | `177-0` / `59M-0` |
| **Scroll region** | `flex: 1 1 0`, `min-height: 0`, `justify-content: center`, `padding-top: 24px`, `padding-inline: 48px`, `overflow: clip` | `15A-0` / `59U-0` |
| **Card column** | `width: 100%`, **`max-width: 1140px`** | `15B-0` / `59V-0` |
| Card radius | `12px 12px 0 0` (bottom corners square — the column runs off the fold) | `15B-0` / `59V-0` |
| Card ground / border | `#FFFFFF` / `var(--color-dark-card)`, `1px solid #DDE2E9` / `var(--color-dark-border)` | `15B-0` / `59V-0` |
| Banner padding | `13px` block, `32px` inline, `gap: 12px` | `170-0` / `59W-0` |
| Claim 1 padding | `34px` top, `28px` bottom, `32px` inline, `gap: 16px` | `163-0` / `5A3-0` |
| Claim 2 padding | `30px` top, `28px` bottom, `32px` inline, `gap: 16px`, `border-top: 1px` | `15C-0` / `5CQ-0` |
| Claim content width | `1074px` (`1140 − 2 − 32 − 32`) | `16R-0`, `16N-0`, `34R-0` |
| Prose wrapper (light only) | `display: flex`, `flex-direction: column`, `gap: 16px`, width `1074px` — **carries no `max-width` and no font** | `16N-0` (dark has no counterpart) |
| **Prose measure** | **`max-width: 760px`**, set per run, not on a wrapper | light `16Q-0` (paragraph) + `1OS-0` (`…more` line); dark `5AD-0` (paragraph) + `5AE-0` (`…more` line) |
| Readiness expansion bleed | `margin: 8px -32px -28px -32px` → renders `1138px`, full card width | `31D-0` / `5B8-0` |
| Readiness expansion padding | `18px` top, `24px` bottom, `32px` inline, `gap: 12px` | `31D-0` / `5B8-0` |
| Blocker rows indent | `padding-left: 24px` | inside `31D-0` / `5B8-0` |
| Hop-pill column | fixed `112px`, `justify-content: end` | rows inside `31D-0` / `5B8-0` |

**Sticky regions:** none are drawn on these boards. The header block `176-0` is a
`flex-shrink: 0` sibling of the scroll region `15A-0`, i.e. the module head and
the facet tabs are **outside** the scroller and the claim column scrolls beneath
them. The engine expresses this differently today (`.sub-nav` is
`position: sticky; top: 0` — style.css:3435-3443); see §7.

**Rails:** there are none on any board in this group. `--container-rail` (268px)
and `--container-toc` (244px) are unused on this screen by definition. The
1140px measure is what those two widths become when they are given back.

### Mobile, 390 (`5DS-0` light, `5H1-0` dark — identical geometry)

| What | Value | Node |
|---|---|---|
| Artboard | `390 × 844`, `flex-direction: column`, `overflow: clip` | `5DS-0` / `5H1-0` |
| Page ground | `var(--color-paper)` / `var(--color-dark-paper)` | `5DT-0` / `5H2-0` |
| Header block | `border-bottom: 1px`, `flex-shrink: 0` | `5DT-0` / `5H2-0` |
| App bar | `height: 52px`, `padding-inline: 16px`, `gap: 12px` | `5DU-0` / `5H3-0` |
| Module head | `padding: 14px 16px`, `gap: 5px` | `5E4-0` / `5HC-0` |
| Metric row | `gap: 8px`, `padding-top: 6px` | `5E7-0` / `5HF-0` |
| Tab strip | `padding-inline: 16px`, `gap: 22px` | `5EB-0` / `5HJ-0` |
| Tab-strip spacer | `flex: 1`, `min-width: 8px` | inside `5EB-0` |
| **Scroll region** | `flex: 1 1 0`, `min-height: 0`, `overflow: clip`, ground `var(--color-card)` / `var(--color-dark-card)` | `5EO-0` / `5HW-0` |
| Claim padding | `20px` top, `16px` bottom, `16px` inline, `gap: 11px` | `5EP-0` / `5HX-0` |
| Claim content width | `358px` (`390 − 16 − 16`) | `5EV-0`, `5EY-0`, `5MI-0` |
| **Prose measure** | none — the prose fills the 358px gutter (no `max-width`) | `5EY-0` / `5I6-0` |
| Footer strip | `margin-top: 4px`, `padding-top: 13px`, `gap: 8px`, `flex-direction: column` | `5MI-0` / `5N9-0` |
| Footer row 1 gap | `10px`; spacer `flex: 1`, `min-width: 4px` | inside `5MI-0` / `5N9-0` |
| Footer row 2 gap | `8px` | inside `5MI-0` / `5N9-0` |
| Readiness expansion | `padding: 16px` (uniform), `gap: 12px`, full-bleed to 390 | `5FL-0` / `5IT-0` |
| Blocker rows indent | `padding-left: 22px` (desktop is 24px) | inside `5FL-0` / `5IT-0` |
| Blocker row | `padding-block: 11px`, `gap: 8px`, `flex-direction: column` | inside `5FL-0` / `5IT-0` |
| Path line | `flex-wrap: wrap`, `gap: 6px` | inside `5FL-0` / `5IT-0` |

**No card wrapper on mobile.** The desktop's 1140px rounded card
(`15B-0`) has no mobile counterpart: the claim sits directly on
`var(--color-card)` filling the viewport, and the boundary between claim and
expansion is a hairline, not a card edge.

---

## 4. Components on this screen

Every value below was read from Paper. Node ids are given beside each group.
Light values come from `13J-0` / `5DS-0`, dark from `594-0` / `5H1-0`.

> **Measured-value count for the desktop light board `13J-0`: 142.** Counted as
> the value-bearing rows (header rows excluded) of the desktop table in §3 and of
> §§4.1, 4.2, 4.3, 4.5, 4.6, 4.7, 4.8 and 4.10 — every one read with
> `get_computed_styles` or `get_jsx` on a node of `13J-0`. It is a floor, not a
> total: several rows carry more than one number (a padding row carries four).
> The dark twin `594-0` supplies the dark column of each of those rows, and the
> mobile-only tables (§3 mobile, §§4.4, 4.9, 4.11, 4.12) add **62** more rows
> read from `5DS-0` / `5H1-0`.
>
> The count rose from 137 after verification: re-reading §3's prose rows and
> §4.2 against Paper split two rows whose values had been attributed to a parent
> into the parent row plus the child rows that actually carry the values
> (prose wrapper vs. prose runs; Focus group vs. divider, button, icon, label and
> key hint). No number changed — only which node each is recorded against.

### 4.1 Module head — eyebrow, title, readiness metric

Nodes: `176-0` `17E-0` `17R-0` `17T-0` `17S-0` `17F-0` `17Q-0` `17P-0` `17N-0`
(dark: `596-0` `597-0` `598-0` `599-0` `59A-0` `59B-0` `59C-0` `59D-0` `59E-0`;
mobile: `5E4-0` `5E5-0` `5E6-0` `5E7-0` `5E8-0` `5E9-0` `5EA-0`).

| Element | Property | Desktop value | Node | Dark token (hex) | Mobile value | Node |
|---|---|---|---|---|---|---|
| Eyebrow `MODULE 06 / 26` | family / size / leading | `--font-mono` (IBM Plex Mono) 11 / 14 | `17T-0` | — | 11 / 14 | `5E5-0` |
| | tracking | `+0.08em` (`--tracking-label`) | `17T-0` | — | `+0.08em` | `5E5-0` |
| | colour | `--color-faint` `#6E7C8E` | `17T-0` | `--color-dark-faint` `#8494A8` | `--color-faint` | `5E5-0` |
| Title `Permission readiness` | family | `--font-sans` (Inter) | `17S-0` | — | Inter | `5E6-0` |
| | size / leading | `28 / 34` | `17S-0` | — | **`24 / 30`** | `5E6-0` |
| | weight / tracking | `600` / `-0.02em` | `17S-0` | — | `600` / `-0.02em` | `5E6-0` |
| | colour | `--color-ink` `#101720` | `17S-0` | `--color-dark-ink` `#E6EAF0` | `--color-ink` | `5E6-0` |
| Metric numerator `31` | size / leading / weight | Inter `13 / 16` / `600` | `17Q-0` | — | `13 / 16` / `600` | `5E8-0` |
| | colour | `--color-ink` `#101720` | `17Q-0` | `--color-dark-ink` | `--color-ink` | `5E8-0` |
| Metric phrase `of 31 locked` | size / leading / weight | Inter `13 / 16` / `400` | `17P-0` | — | `13 / 16` | `5E9-0` |
| | colour | `--color-muted` `#54606F` | `17P-0` | `--color-dark-muted` `#9AA7B8` | `--color-muted` | `5E9-0` |
| Progress track | size | `80 × 6`, radius `999px` (`--radius-pill`), `overflow: clip` | `17N-0` | — | **`72 × 6`** | `5EA-0` |
| | track colour | `--color-border` `#DDE2E9` | `17N-0` | *(not drawn — dark board draws only the fill)* | *(not drawn)* | `5EA-0` |
| | fill colour / width | `--color-locked` `#2C6B52`, `80px` (100 %) | inside `17N-0` | `--color-dark-locked` `#63BE9A` | `--color-locked`, `72px` | `5EA-0` |

*Intent:* the metric is a **sentence with one number pulled out**, not a gauge
with a label. The bar is a confirmation of a phrase already read, which is why it
is 6px tall and carries no text of its own. `--color-locked` is the only status
colour in the header, because "locked" is the only status the header states.

### 4.2 Focus control — the one control (desktop only)

Nodes: the **group** is `17G-0` (light) / `59F-0` (dark) and holds exactly two
children — a leading divider `17M-0` / `59G-0` and the button box `17H-0` /
`59H-0`. **Every geometry and fill value below sits on the button box, not on the
group**; the group carries only `display: flex`, `align-items: center`,
`gap: 10px`, `margin-left: 6px`. States visible: **active** on both boards.
Rest / hover / disabled are not drawn — see §8.

| Property | Light value | Dark value | Node |
|---|---|---|---|
| Group | `display: flex`, `align-items: center`, `gap: 10px`, `margin-left: 6px` | same | `17G-0` / `59F-0` |
| Leading divider | `1 × 16`, `flex-shrink: 0`, `--color-border` `#DDE2E9` | `var(--color-dark-border)` | `17M-0` / `59G-0` |
| Height | `28px` | `28px` | `17H-0` / `59H-0` |
| Inline padding | `11px` | `11px` | `17H-0` / `59H-0` |
| Gap | `7px` | `7px` | `17H-0` / `59H-0` |
| Radius | `7px` (**off-scale** — not `--radius-sm` 4, not `--radius-md` 8) | `7px` | `17H-0` / `59H-0` |
| Ground | `#1C4E8C17` = `--color-accent` at 9 % (equals `--color-accent-bg`) ✓ | `#6AA6E824` = `--color-dark-accent` at 14 % (equals `--color-dark-accent-bg`) ✓ | `17H-0` / `59H-0` |
| Border | `1px solid #1C4E8C38` = `--color-accent` at 22 % | `1px solid #6AA6E866` = `--color-dark-accent` at 40 % | `17H-0` / `59H-0` |
| Icon | 13 × 13, `viewBox 0 0 24 24`, `stroke-width: 2`, `stroke-linecap: round`, `stroke-linejoin: round`, `flex-shrink: 0`, four-corner "expand" glyph, path `M8 3H5a2 2 0 0 0-2 2v3M16 3h3a2 2 0 0 1 2 2v3M8 21H5a2 2 0 0 1-2-2v-3M16 21h3a2 2 0 0 0 2-2v-3` | same glyph | `17K-0` / `59I-0` |
| Icon colour | inline style `rgb(28, 78, 140)` = `--color-accent` (**but the `<path>` attribute says `stroke="#54606F"`** — defect 5) | `var(--color-dark-accent)` on the `<path>`, no conflicting element style | `17K-0` / `59I-0` |
| Label `Focus` | Inter `13 / 16`, weight `500` | same | `17J-0` / `59K-0` |
| Label colour | `--color-accent` `#1C4E8C` | `--color-dark-accent` `#6AA6E8` | `17J-0` / `59K-0` |
| Key hint `F` | `--font-mono` `11 / 14`, weight `400` | same | `17I-0` / `59L-0` |
| Key hint colour | `--color-accent` `#1C4E8C` | `--color-dark-accent` `#6AA6E8` | `17I-0` / `59L-0` |

Both tint alphas land on their token exactly: `0x17` = 23/255 = 9.0 % =
`--color-accent-bg`; `0x24` = 36/255 = 14.1 % = `--color-dark-accent-bg`. The
Focus control is one of the few places on this screen where the board hex and the
token agree in **both** modes — the others are the LOCKED chip (§4.6) and, in one
mode each, the light count pill and target slug (§4.10) and the dark mobile
blocked chip (§4.9). Each such agreement is marked ✓ where it appears.

*Intent:* the control carries its own keyboard shortcut in mono, beside the
label, because the shortcut **is** the primary exit (R11.4) and a tooltip would
only be discovered by a reader who already hovered. The tint-plus-border
treatment, rather than a filled button, is the same "this is on" language the
active facet tab uses — `--color-accent` at low alpha with a full-strength
label. Nothing else in the header is accent-coloured, so the lit control is
unambiguous.

### 4.3 Facet tabs

Nodes: `177-0` `17B-0` `17D-0` `17C-0` `178-0` `17A-0` `179-0` (dark `59M-0`
`59N-0` `59O-0` `59P-0` `59Q-0` `59R-0` `59S-0`; mobile `5EB-0` `5EC-0` `5ED-0`
`5EE-0` `5EF-0` `5EG-0` `5EH-0`).

| Element | Property | Desktop | Node | Dark | Mobile | Node |
|---|---|---|---|---|---|---|
| Strip | gap | `26px` | `177-0` | — | **`22px`** | `5EB-0` |
| | bottom rule | `1px solid --color-border #DDE2E9` | `177-0` | `var(--color-dark-border)` | *(none on the strip; the block below carries it)* | `5EB-0` |
| Selected tab | underline | `2px solid --color-accent #1C4E8C`, `margin-bottom: -1px` | `17B-0` | `var(--color-dark-accent)` | same | `5EC-0` |
| | bottom padding | `11px` | `17B-0` | — | **`10px`** | `5EC-0` |
| | inner gap | `7px` | `17B-0` | — | `7px` | `5EC-0` |
| | label | Inter `14 / 18`, weight `600`, `--color-accent` | `17D-0` | `--color-dark-accent` `#6AA6E8` | same | `5ED-0` |
| | count | mono `11 / 14`, weight `400`, `--color-accent` | `17C-0` | `--color-dark-accent` | same | `5EE-0` |
| Unselected tab | bottom padding | `11px`, no underline | `178-0` | — | `10px` | `5EF-0` |
| | label | Inter `14 / 18`, weight `400`, `--color-muted` `#54606F` | `17A-0` | `--color-dark-muted` `#9AA7B8` | same | `5EG-0` |
| | count | mono `11 / 14`, `--color-faint` `#6E7C8E` | `179-0` | `--color-dark-faint` `#8494A8` | same | `5EH-0` |

*Intent:* the count is mono because it is a machine fact about the facet; the
label is sans because it is what the interface calls it. The selected tab moves
**three** things at once (colour, weight, underline) so it survives a
desaturated display; colour alone would not.

### 4.4 `On this facet` trigger (mobile only)

Nodes: `5EJ-0` `5EK-0` `5EM-0` (dark `5HR-0`). This is the mobile substitute for
the facet TOC rail — the trigger that opens the facet-index bottom sheet
(R-J.6's "known bodies today: the facet index and the comment thread").

| Property | Light value | Dark value | Node |
|---|---|---|---|
| Height | `26px` | `26px` | `5EJ-0` / `5HR-0` |
| Inline padding | `9px` | `9px` | `5EJ-0` / `5HR-0` |
| Gap | `6px` | `6px` | `5EJ-0` / `5HR-0` |
| Radius | `7px` (off-scale, same as the Focus button) | `7px` | `5EJ-0` / `5HR-0` |
| Ground | `--color-card` `#FFFFFF` | `--color-dark-card` `#161B22` | `5EJ-0` / `5HR-0` |
| Border | `1px solid --color-border #DDE2E9` | `1px solid --color-dark-border #242C38` | `5EJ-0` / `5HR-0` |
| Bottom margin | `6px` (lifts it clear of the tab underline) | `6px` | `5EJ-0` / `5HR-0` |
| Icon | 12 × 12 list glyph, `stroke-width: 2`, `--color-muted` | `--color-dark-muted` | `5EK-0` |
| Label | Inter `12 / 16`, weight `500`, `--color-muted` | `--color-dark-muted` | `5EM-0` |

**Touch-target note:** at 26px this is below the 44px minimum. That minimum is
**not** in `reference-rules.md` — R-J.1–R-J.7 carry no target-size rule and the
string `44` does not occur in that file. It is set by `tokens.md` §8, row
*"J · 44px minimum tap target; grabber as a drag affordance → `(pointer: coarse)`"*,
which §5 cites correctly. See §5 for how the trigger meets it without being
redrawn.

### 4.5 Blocked banner (desktop only)

Nodes: `170-0` `173-0` `172-0` `171-0` (dark `59W-0` `59X-0` `5A0-0` `5A1-0`).

| Property | Light value | Dark value | Node |
|---|---|---|---|
| Ground | `#9E3B360F` = `--color-blocked` at **6 %** (token `--color-blocked-bg` is 10 %) | `#EC8A831A` = `--color-dark-blocked` at **10 %** (token is 13 %) | `170-0` / `59W-0` |
| Bottom border | `1px solid --color-border #DDE2E9` | `1px solid var(--color-dark-border)` | `170-0` / `59W-0` |
| Padding | `13px` block, `32px` inline | same | `170-0` / `59W-0` |
| Gap | `12px` | `12px` | `170-0` / `59W-0` |
| Icon | 15 × 15 warning triangle, `stroke-width: 2`, `stroke-linecap: round`, `--color-blocked` `#9E3B36` | `--color-dark-blocked` `#EC8A83` | `173-0` / `59X-0` |
| Body text | Inter `14 / 18`, weight `400`, `--color-ink` `#101720`, `flex: 1 1 0` | `--color-dark-ink` `#E6EAF0` | `172-0` / `5A0-0` |
| Action `Show issues` | Inter `13 / 16`, weight `500`, `--color-blocked` `#9E3B36` | `--color-dark-blocked` `#EC8A83` | `171-0` / `5A1-0` |

*Intent:* the body is ink, not red. Only the icon and the action take the blocked
colour — the sentence is a fact, the action is the alarm. The banner is the top
edge of the card, inside its 12px radius, so it reads as belonging to the claim
column rather than floating above it (R09.6: "it appears exactly where the issues
are").

### 4.6 Claim head — title, id, status chip

Nodes: `16R-0` (light claim 1), `15C-0`'s head (claim 2), `5A4-0` (dark),
`5EP-0` / `5HX-0` (mobile).

| Element | Property | Desktop | Node | Dark | Mobile | Node |
|---|---|---|---|---|---|---|
| Head row | gap / align | `16px` / `align-items: start` | `16R-0` | same | *(column — chip above)* | `5EP-0` |
| Identity column | gap | `7px` | `16R-0` | same | **`6px`**, chip separated by `11px` | `5EP-0` |
| Title | family / size / leading | Inter `20 / 26` (`--text-h2` + `--leading-title`) | `16R-0` | same | **`18 / 24`** | `5EW-0` |
| | weight / tracking | `600` / `-0.01em` (**off-scale tracking, no token**) | `16R-0` | same | `600` / `-0.01em` | `5EW-0` |
| | colour | `--color-ink` `#101720` | `16R-0` | `--color-dark-ink` `#E6EAF0` | `--color-ink` | `5EW-0` |
| Claim id | family / size / leading | mono `12 / 16` | `16R-0` | same | **mono `11 / 15`** | `5EX-0` |
| | colour | `--color-faint` `#6E7C8E` | `16R-0` | `--color-dark-faint` `#8494A8` | `--color-faint` | `5EX-0` |
| LOCKED chip | ground | `#2C6B521A` = `--color-locked-bg` (10 %) ✓ | `16R-0` | `#63BE9A21` = `--color-dark-locked-bg` (13 %) ✓ | `#2C6B521A` | `5EP-0` / `5HY-0` |
| | radius | `999px` (`--radius-pill`) | `16R-0` | same | same | `5EP-0` |
| | padding | `4px` block, `8px` left, `10px` right | `16R-0` | same | **`3px` block, `7px` left, `9px` right** | `5EP-0` |
| | gap | `5px` | `16R-0` | same | `5px` | `5EP-0` |
| | padlock icon | `12 × 12`, `stroke-width: 2.4`, `--color-locked` `#2C6B52` | `16R-0` | `--color-dark-locked` `#63BE9A` | **`11 × 11`** | `5ER-0` |
| | label | Inter `11 / 14`, weight `600`, tracking `+0.05em` (**off-scale, no token**) | `16R-0` | same | same | `5EU-0` |
| | label colour | `--color-locked` `#2C6B52` | `16R-0` | `--color-dark-locked` `#63BE9A` | `--color-locked` | `5EU-0` |
| | `flex-shrink` | `0` — the chip never compresses; the title wraps instead | `16R-0` | same | `align-self: start` | `5EP-0` |

### 4.7 Claim prose and the `…more` control

Nodes: light — wrapper `16N-0`, paragraph 1 `16Q-0`, `…more` line `1OS-0`
(paragraph 2 `1OT-0` + link `1OU-0`). Dark — paragraph 1 `5AD-0`, `…more` line
`5AE-0` (paragraph 2 `5AF-0` + link `5AG-0`), **with no wrapper frame**. Mobile —
`5EY-0` / `5I6-0`.

| Element | Property | Desktop | Node | Dark | Node (dark) | Mobile | Node |
|---|---|---|---|---|---|---|---|
| Prose | family | `--font-serif` (Source Serif 4) | `16Q-0`, `1OT-0` | `"Source Serif 4", system-ui, sans-serif` (literal — defect 2) | `5AD-0`, `5AF-0` | same | `5EY-0` |
| | size / leading | `17 / 28` (`--text-body` + `--leading-body`) | `16Q-0`, `1OT-0` | `17 / 28` | `5AD-0`, `5AF-0` | **`16 / 26`** | `5EY-0` / `5I6-0` |
| | weight | `400` | `16Q-0`, `1OT-0` | `400` | `5AD-0`, `5AF-0` | `400` | `5I6-0` |
| | colour | `--color-ink` `#101720` | `16Q-0`, `1OT-0` | `--color-dark-ink` | `5AD-0`, `5AF-0` | `--color-ink` | `5EY-0` |
| | **measure** | **`max-width: 760px`**, set on each run | `16Q-0`, `1OS-0` | **`max-width: 760px`** | `5AD-0`, `5AE-0` | *(none — fills 358px)* | `5EY-0` |
| | paragraph gap | `16px`, from the wrapper's `flex-direction: column` | `16N-0` | `16px`, from the **claim block's** own gap — there is no wrapper | `5A3-0` | — | — |
| `…more` | size / leading / weight | Inter `14 / 28` / `500` | `1OU-0` | same, family literal `"Inter", system-ui, sans-serif` | `5AG-0` | *(not drawn)* | — |
| | colour | `--color-accent` `#1C4E8C` | `1OU-0` | `--color-dark-accent` | `5AG-0` | — | — |
| | placement | `align-items: baseline`, `gap: 7px` after the last word, `flex-shrink: 0`, `white-space: pre` | `1OS-0` / `1OU-0` | same | `5AE-0` / `5AG-0` | — | — |

*Intent:* `…more` sits on the prose baseline at a **sans** 14px inside a serif
17px line, so it reads as an interface control interrupting the text rather than
as text. The `28px` line-height keeps it on the prose's own baseline grid.

### 4.8 Footer strip — desktop form (one line at rest)

Nodes: `34R-0` (claim 1, readiness **open**), `35H-0` (claim 2, all **closed**);
dark `5AI-0`.

| Element | Property | Light value | Node | Dark |
|---|---|---|---|---|
| Strip | top rule | `1px solid #E8ECF1` (**no token**) | `34R-0` | `1px solid #212934` (**no token**) |
| | margin-top / padding-top | `8px` / `16px` | `34R-0` | same |
| | gap / align | `20px` / `center` | `34R-0` | same |
| Readiness group | inner gap | `6px` | `34R-0` | same |
| | status dot | `7 × 7`, radius `999px`, `--color-blocked` `#9E3B36` | `34R-0` | `--color-dark-blocked` `#EC8A83` |
| | status word `Blocked` | Inter `13 / 16`, weight `600`, `--color-blocked` | `34R-0` | `--color-dark-blocked` |
| | count `2 blockers` | Inter `13 / 16`, weight `400`, `--color-muted` `#54606F` | `34R-0` | `--color-dark-muted` |
| | chevron, **open** | `12 × 12`, `stroke-width: 2.4`, path `m18 15-6-6-6 6` (**up**), `--color-blocked` | `34R-0` | `--color-dark-blocked` |
| | chevron, **closed** | `12 × 12`, `stroke-width: 2.4`, path `m6 9 6 6 6-6` (**down**), `--color-faint` `#6E7C8E` | `35H-0` | `--color-dark-faint` |
| Divider | size / colour | `1 × 12`, `--color-border` `#DDE2E9` | `34R-0` | `var(--color-dark-border)` |
| Detail chip (closed) | inner gap | `5px` | `34R-0` | same |
| | label | Inter `13 / 16`, weight `400`, `--color-muted` `#54606F` | `34R-0` | `--color-dark-muted` |
| | chevron | `12 × 12`, `stroke-width: 2.4`, down, `--color-faint` `#6E7C8E` | `34R-0` | `--color-dark-faint` |
| Detail chip (**zero**) | label | `No checks declared`, Inter `13 / 16`, **`--color-faint` `#6E7C8E`** — one step quieter than a chip with a count | `35H-0` | `--color-dark-faint` |
| Spacer | | `flex: 1 1 0` — pushes the comment count hard right (R-F.1) | `34R-0` | same |
| Comment count | gap | `6px` | `34R-0` | same |
| | icon | `14 × 14` speech bubble, `stroke-width: 2`, `stroke-linecap: round`, `--color-faint` `#6E7C8E` | `34R-0` | `--color-dark-faint` |
| | number | Inter `13 / 16`, weight `400`, `--color-faint` | `34R-0` (`0`), `35H-0` (`2`) | `--color-dark-faint` |

**No pill.** The desktop chips are bare label-and-chevron. The 30px outlined pill
described in R-F.2 is the **mobile** form (components board section I3, whose
stated reason is "at 12px on a phone, colour alone is too quiet"). See Open
decisions.

**Order, confirmed on both claims:** readiness (with its status word) →
divider → relationships → sources → checks → spacer → comment count. That is
R-F.1's fixed order, and the divider marks the readiness chip as the one that
carries a status rather than only a count.

### 4.9 Footer strip — mobile form (two rows)

Nodes: `5MI-0` (light), `5N9-0` (dark).

| Element | Property | Light value | Dark value |
|---|---|---|---|
| Strip | top rule / margin / padding | `1px solid #E8ECF1` (no token) / `4px` / `13px` | `1px solid #212934` (no token) |
| | direction / gap | `column` / `8px` | same |
| **Row 1** | gap | `10px`; spacer `flex: 1`, `min-width: 4px` | same |
| Blocked chip | height / radius | `30px` / `999px` | same |
| | ground | `#9E3B3617` = `--color-blocked` at **9 %** | `#EC8A8321` = `--color-dark-blocked` at **13 %** ✓ |
| | inline padding / gap | `11px` / `7px` | same |
| | dot | `7 × 7`, `--color-blocked` | `--color-dark-blocked` |
| | status word | Inter `12 / 16`, weight `600`, `--color-blocked` | `--color-dark-blocked` |
| | count | Inter `12 / 16`, weight `400`, **`--color-blocked`** (desktop uses `--color-muted` here) | `--color-dark-blocked` |
| | chevron (open) | `11 × 11`, `stroke-width: 2.6`, up, `--color-blocked` | `--color-dark-blocked` |
| Comment count | icon / number | `14 × 14` bubble `stroke-width: 2` `--color-faint`; Inter `13 / 16` `--color-faint` | `--color-dark-faint` |
| **Row 2** | gap | `8px` | same |
| Detail pill | height / radius | `30px` / `999px` | same |
| | ground / border | `--color-card` / `1px solid --color-border` | `--color-dark-card` / `1px solid --color-dark-border` |
| | inline padding / gap | `11px` / `6px` | same |
| | label | Inter `12 / 16`, weight `500`, `--color-muted`, `width: max-content` | `--color-dark-muted` |
| | chevron | `11 × 11`, `stroke-width: 2.6`, down, `--color-faint` | `--color-dark-faint` |

`width: max-content` on every pill label is what enforces R-F.4's "each one fits
whole — nothing spans the width or splits across lines".

**R-I.3 governs how to read this table.** *"The detail chip is the component. The
footer's two-row arrangement is composition, not part of the chip."* (Evidence:
components board section I3, `7N2-0`.) So the pill geometry above — 30px,
`--radius-pill`, `11px` inline padding, `6px` gap, `width: max-content` label —
is the **chip**, and belongs to the chip wherever it appears; the two rows, their
`8px` gap and the `flex: 1; min-width: 4px` spacer are the **footer**. A lane
agent changing one must not treat it as licence to change the other, and the
desktop/mobile split in §4.8 vs §4.9 is a difference of composition on the same
component.

**The comment count on mobile is drawn as the desktop passive count**
(`--color-faint`, no tint, no pill). R-F.3 requires the mobile variant to take
`--color-accent-bg` with an `--color-accent` icon and label, because on a phone
it is the control that opens the sheet. Flagged in §9.

### 4.10 Readiness expansion — desktop

Nodes: `31D-0` (light), `5B8-0` (dark).

| Element | Property | Light value | Dark value |
|---|---|---|---|
| Panel | ground | `#FBF7F7` (**no token** — a 3 %-ish blocked wash on card) | `var(--color-dark-blocked-surface)` `#1D1618` ✓ |
| | top rule | `1px solid #E8ECF1` (no token) | `1px solid #212934` (no token) |
| | bleed | `margin: 8px -32px -28px -32px` | same |
| | padding | `18px` top, `24px` bottom, `32px` inline | same |
| | gap | `12px` | same |
| Section eyebrow | text / face | `READINESS BLOCKERS`, Inter `11 / 14`, weight `600`, `+0.08em` | same |
| | colour | `--color-blocked` `#9E3B36` | `--color-dark-blocked` `#EC8A83` |
| Eyebrow rule | height / colour | `1px`, `#EBD9D8` (**no token**) | `var(--color-dark-blocked-hairline)` `#3A2A2B` ✓ |
| Scope count | text / face / colour | `2 in 1 module`, Inter `12 / 16`, `--color-faint` | `--color-dark-faint` |
| Group row | gap / padding | `12px` / `2px` block | same |
| | chevron | `12 × 12`, `stroke-width: 2.4`, down, `--color-muted` | `--color-dark-muted` |
| | group name | Inter `14 / 18`, weight `600`, `--color-ink` | `--color-dark-ink` |
| | nearest-hop note | Inter `12 / 16`, `--color-faint` | `--color-dark-faint` |
| | count pill | radius `999px`, padding `3px / 9px`, ground `#9E3B361A` (blocked at 10 % ✓) | `#EC8A8329` (blocked at **16 %** — off token) |
| | count pill label | Inter `11 / 14`, weight `600`, `--color-blocked` | `--color-dark-blocked` |
| Rows container | indent | `padding-left: 24px` | same |
| Blocker row | divider / gap / padding | `1px solid #EFE6E5` (no token) / `14px` / `10px` block | `1px solid #33262A` (no token) |
| | title | Inter `14 / 20`, weight `600`, `--color-ink` | `--color-dark-ink` |
| | path gap | `6px` | same |
| | source slug | ground `--color-paper` `#EFF1F4`, radius `4px` (`--radius-sm`), padding `2px / 7px`, mono `11 / 14`, `--color-muted` | ground `--color-dark-paper`, `--color-dark-muted` |
| | path chevron | `11 × 11`, `stroke-width: 2.6`, right, `--color-faint` | `--color-dark-faint` |
| | target slug | ground `#9E3B361A` (blocked 10 % ✓), radius `4px`, padding `2px / 7px`, mono `11 / 14`, `--color-blocked` | ground `#EC8A8329` (16 %), `--color-dark-blocked` |
| | hop column | fixed `112px`, `justify-content: end`, `align-items: start` | same |
| | hop pill | ground `--color-paper`, radius `999px`, padding `3px / 9px`, Inter `11 / 14`, weight `500`, `--color-muted`, `width: max-content` | `--color-dark-paper` / `--color-dark-muted` |
| Resolution note | padding-top / gap | `12px` / `10px` | same |
| | text | Source Serif 4 `14 / 22`, `--color-muted`, `flex: 1 1 0` | `--color-dark-muted` |
| Graph link | icon | `13 × 13` node-and-arc glyph, `stroke-width: 2`, `--color-accent` `#1C4E8C` | `--color-dark-accent` `#6AA6E8` |
| | label | `See in claims graph`, Inter `13 / 16`, weight `500`, `--color-accent` | `--color-dark-accent` |
| | gap | `6px`, `flex-shrink: 0` | same |

*Intent:* the path is **two slugs and a chevron**, not a map (R09.9). The target
slug takes the blocked tint and the source slug takes the neutral paper ground,
so the eye lands on the id that is not approved. The 112px fixed hop column is
what gives the two rows a shared right lane without a table.

The resolution note is the only **serif** run in this panel: it is a sentence a
human wrote about what to do, sitting among machine output, and the family says
so (`tokens.md` §7).

### 4.11 Readiness expansion — mobile

Nodes: `5FL-0` (light), `5IT-0` (dark). Differences from desktop only:

| Element | Property | Light value | Dark value |
|---|---|---|---|
| Panel | padding | `16px` uniform (desktop `18 / 24 / 32`) | same |
| | top rule | `1px solid #EFE6E5` (light; desktop uses `#E8ECF1`) | `1px solid #33262A` |
| | ground | `#FBF7F7` | `var(--color-dark-blocked-surface)` |
| Eyebrow row | gap | `10px` (desktop `12px`) | same |
| Rows container | indent | `padding-left: 22px` (desktop `24px`) | same |
| Blocker row | direction / gap / padding | `column` / `8px` / `11px` block | same |
| Row line 1 | gap / align | `10px` / `align-items: start` | same |
| | title | Inter `14 / 20`, weight `600`, `--color-ink`, `flex: 1 1 0` | `--color-dark-ink` |
| | hop pill | `flex-shrink: 0`, right-ranged, text **`1 hop` / `2 hops`** (desktop: `direct · 1 hop` / `upstream · 2 hops`) | same |
| Row line 2 | wrapping | `flex-wrap: wrap`, `gap: 6px` — the path wraps within the gutter | same |
| Resolution note | direction / gap | `column` / `10px` — the graph link drops below the sentence | same |

### 4.12 Mobile app bar

Nodes: `5DU-0` `5DV-0` `5DX-0` `5DY-0` `5E1-0` (dark `5H3-0`…).

| Element | Value | Node |
|---|---|---|
| Bar height / inline padding / gap | `52px` / `16px` / `12px` | `5DU-0` / `5H3-0` |
| Hamburger | `20 × 20`, `stroke-width: 2`, `--color-muted` | `5DV-0` |
| Project name `Curtainly` | Inter `14 / 18`, weight `500`, `--color-muted`, `flex: 1 1 0` | `5DX-0` |
| Search glyph | `19 × 19`, `stroke-width: 2`, `--color-muted` | `5DY-0` |
| Theme glyph | `19 × 19`, `stroke-width: 2`, `--color-muted` | `5E1-0` |

All four bar items are `--color-muted` / `--color-dark-muted`: the app bar is
identity and utilities, and nothing in it competes with the claim.

---

## 5. Mobile rules

The 390px boards are drawn at 390 because that is the device worth drawing.
**No 390px breakpoint is added.** Every rule below names the existing engine
query it lands in (`tokens.md` §8).

| Rule, as the boards state it | Engine query | Why that query |
|---|---|---|
| **The Focus control is not rendered.** Mobile has no rails, so there is nothing to hide. (`5DS-0`, `5H1-0`; notes strip `5KP-0` column 2.) | `@media (max-width: 860px)` | The rails themselves already collapse at 860, in **two** blocks: the TOC rail inside `style.css:4533` (`.facet-toc` re-anchored as a fixed 248px popover at `:4555-4566`, `.facet-toc__list` hidden at `:4568-4569`, `.system-panel-toggle--sidebar { display: none }` at `:4552`), and the left rail turned into a drawer inside `style.css:2932` (`.layout`, `.content-area`, `.sidebar`, `.nav-toggle`, `.nav-overlay`). *(Not `style.css:4424` — that 860 block is the claims-graph pane's `.dxg-*` rules and touches no rail.)* The control must disappear on the same boundary as the thing it controls, not later — a lit Focus button on a tablet with no rails would be a lie. |
| **The `F` shortcut is inert below 861px.** | `@media (min-width: 861px)` gating the binding, not a CSS rule | Same reason; a keyboard is possible on a tablet, the rails are not. |
| Module head drops from `28 / 34` to `24 / 30`; the progress bar from `80px` to `72px`. | `@media (max-width: 520px)` | Phone tier. Arrangement/size by width. |
| Claim title drops from `20 / 26` to `18 / 24`; claim id from mono `12 / 16` to `11 / 15`; prose from `17 / 28` to `16 / 26` and loses its `max-width`. | `@media (max-width: 520px)` | Phone tier. R11.2's "measure never changes" is a **focus-mode** invariant, not a cross-breakpoint one — at 390 the gutter *is* the measure. |
| **Status chip moves above the title** (R-H.2). Order: status, claim, id. Gaps `11px` then `6px`. | `@media (max-width: 520px)` | Phone tier, per `tokens.md` §8 row H2. |
| Claim padding drops from `34 / 28 / 32` to `20 / 16 / 16`; the 1140px rounded card wrapper is not rendered at all. | `@media (max-width: 520px)` | Phone tier. |
| **Footer becomes two rows** (R-F.4): row 1 = blocked/status chip left, comment count right; row 2 = detail pills. Bare labels become 30px `--radius-pill` pills. | `@media (max-width: 520px)`, **written to win over the two existing responsive rules on `.claim-footer__counts`** — the `860px` pair at style.css:4640-4641 (block opened at 4533) and the `560px` pair at style.css:4655-4656 (block 4647-4658) | `tokens.md` §8 row H3 and Disagreement 16. A 520 rule at equal specificity must come **after** the 560 block in source order, or the narrower tier silently loses. *(There is no 640px rule here: the file's only `@media (max-width: 640px)`, style.css:2563, is `.gcp-*` console rules and declares nothing on `.claim-footer__counts` — the correction `tokens.md` §8 row H3 already carries, from screen 14 OD14.11.)* |
| Detail pills never split: each label is `width: max-content`. | `@media (max-width: 520px)` | R-F.4. |
| **Blocker row stacks**: hop pill right-ranged on the title line, path wrapping below (`flex-wrap: wrap`). | `@media (max-width: 520px)` | R-I.1 / `tokens.md` §8 row I1. **But see §9 defect 7** — the component board's I1 form differs and wins. |
| Hop-pill text shortens from `direct · 1 hop` to `1 hop`. | `@media (max-width: 520px)` | Content change, not a layout change; it belongs with the arrangement rule because it exists only to fit. |
| Readiness expansion padding becomes `16px` uniform; row indent `24px → 22px`; resolution note stacks above its graph link. | `@media (max-width: 520px)` | Phone tier. |
| `On this facet` trigger appears in the tab strip, right-ranged behind a `flex: 1; min-width: 8px` spacer. | `@media (max-width: 860px)` for its existence; `@media (max-width: 520px)` for the in-strip placement | It exists wherever the TOC rail does not (860); its position inside the tab strip is a phone arrangement. |
| **Touch targets: every tappable thing is ≥ 44px.** The boards draw the `On this facet` trigger at `26px` and the footer pills at `30px`. | `@media (pointer: coarse)` | `tokens.md` §8: *"target size and hover-less affordances are a pointer question"*. The engine already sets `.comment-chip { min-height: 44px }` here (style.css:1765-1772) and `.subtab { min-height: 44px }` unconditionally (style.css:3446-3451). **Do not put a 44px minimum behind a width query.** Meet it by growing the hit area (padding / `::before` inflation), not by redrawing the pill at 44px tall — the boards' 30px visual height is the design. |
| Hover affordances (the `…more` link's hover, chip hover washes) have no touch equivalent; affected controls must read as tappable at rest. | `@media (pointer: coarse)` | Same rule. |

### Bottom sheets on this screen

Per **R-J.1**, *there is one bottom sheet in this product, not several* — one
fixed shell, varying bodies. Screen 03 touches it in exactly one place: the
`On this facet` trigger (`5EJ-0` / `5HR-0`) opens the **facet-index body** of
that shell. No board in this group draws the sheet itself; the shell is specified
on components board `AYG-0` / `AYO-0` and restated in R-J.2 – R-J.5:

- scrim `--scrim` (`rgba(0,0,0,.22)` light / `rgba(0,0,0,.42)` dark);
- `16px` top corners; `36 × 4` grabber in `--color-border-strong`, `--radius-pill`,
  under `8px` top padding, centred; header row with a `1px --color-border`
  hairline beneath it; an extra `1px` top hairline on dark;
- `min-height: 240px`, `max-height: 660px`, `height: fit-content`, scrolling
  internally — **660 is a ceiling, not a height**;
- ground `--color-card`, width = viewport, `overflow: clip`.

**A lane agent must not design a second shell for the facet index.** Layout goes
in `@media (max-width: 520px)`; the grabber's drag affordance and the 44px
targets inside it go in `@media (pointer: coarse)`.

There is **no comment sheet on this screen's boards** — the comment count is
drawn as a passive count on both mobile boards. Per R-F.3 it should be the
accent-tinted control that opens the comment body of the same shell; see §9.

---

## 6. Footer vocabulary

The words on this screen, quoted from the boards, in the order they appear.

### Module head (`17T-0`, `17S-0`, `17Q-0`, `17P-0`)

> `MODULE 06 / 26`
> `Permission readiness`
> `31` `of 31 locked`

The eyebrow is `MODULE` + a two-digit ordinal + ` / ` + the module's claim count.
The metric is **numerator, then the phrase** — `31` is a separate element at
weight 600 so the number is the thing that reads; `of 31 locked` is one string.
Not "100 %", not "31/31".

### Facet tabs (`17D-0`, `17C-0`, `17A-0`, `179-0`)

> `Contract` `26`  ·  `Internals` `5`

Label then bare count. No parentheses, no "claims".

### Focus control (`17J-0`, `17I-0`)

> `Focus` `F`

Label then the bare key, no brackets, no "press".

### Blocked banner (`172-0`, `171-0`)

> `All 26 contract claims are blocked by unapproved dependencies outside this module`
> `Show issues`

Quantifier first (`All 26`), then the facet name, then the state, then **where
the cause is** (`outside this module`). The action is two words, imperative.
`Show issues` is already the engine's wording (viewer-runtime.js:1725).

### Claim footer strip — desktop, claim 1 (`34R-0`)

> `Blocked`  `2 blockers`  |  `4 relationships`  `2 sources`  `1 check`  …  `0`

### Claim footer strip — desktop, claim 2 (`35H-0`)

> `Blocked`  `2 blockers`  |  `3 relationships`  `1 source`  `No checks declared`  …  `2`

Each chip is **a noun and a count**, never a score (R-F.1). Singular and plural
both appear and both are correct: `1 check`, `1 source`, `2 sources`,
`4 relationships`. The zero state is **not** `0 checks` — it is the sentence
`No checks declared`, in `--color-faint`, because "zero checks" and "no checks
were declared" are different facts and the viewer only knows the second. The
comment count is a bare integer beside a bubble glyph; `0` is rendered, not
suppressed.

### Claim footer strip — mobile (`5MI-0`, `5N9-0`)

> row 1 — `Blocked` `2 blockers` … `0`
> row 2 — `4 relationships` `2 sources` `1 check`

Same words, same order, different arrangement (R-H.0, R-I.0).

### Readiness expansion (`31D-0`, `5B8-0`, `5FL-0`, `5IT-0`)

> `READINESS BLOCKERS`  ————  `2 in 1 module`
> `Capability support`  `nearest 1 hop`  `2`
> `Capabilities own their dimensions is not locally approved`
>   `mechanism-scoped-observation-authority` › `capabilities-own-their-dimensions`   `direct · 1 hop`
> `The five verdicts is not locally approved`
>   `mechanism-scoped-observation-authority` › `capabilities-own-their-dimensions` › `the-five-verdicts`   `upstream · 2 hops`
> `Both sit in capability-support. One approval there clears this claim.`   `See in claims graph`

Vocabulary notes:

- **Scope count**: `N in M module(s)` — blockers first, modules second.
- **Group meta**: `nearest 1 hop` — "nearest", never "min" or "closest distance".
- **Blocker title**: the claim's **title**, then ` is not locally approved`.
  Never the slug (R09.8: "The reviewer recognises the title. The slug is for the
  agent."). The slug appears only inside the path, in mono.
- **Hop label**: `direct · 1 hop` / `upstream · 2 hops` on desktop; `1 hop` /
  `2 hops` on mobile. The relation word and the distance are joined by ` · `.
- **Resolution note**: a two-sentence human statement — where the blockers sit,
  and what single action clears the claim. It is the only serif line in the
  panel.
- **Graph action**: `See in claims graph` — "see", not "open" or "view".

### Freshness / elapsed time

**Not on this screen.** R10.1 ("Updated 23 hours ago", never a timestamp),
R10.3 (one unit) and R10.5 (`Live` under `dossierx serve`) bind the **right-rail
footer**, and focus mode's whole purpose is that the rails are gone. Per R11.4
no substitute line is added. On the mobile boards the rail does not exist at any
state, so the freshness line has no home there either; it belongs to the nav
sheet body, which is board 02's business, not 03's. See §8.

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

> **Line numbers are pinned to commit `3ac8844`**, not to the working tree.
> This worktree is shared: while this spec was being written, another lane grew
> `style.css` from 4782 to 4870 lines (an engine-font change replacing
> `geist_fonts.go` with `engine_fonts.go` and adding Inter / IBM Plex Mono /
> Source Serif 4 faces), so every line number below drifted mid-session. Each
> address here was re-verified against `git show 3ac8844:<path>`. Recover a
> drifted address by grepping the quoted selector or identifier, not by trusting
> the number.

### 7.1 Focus mode does not exist today — what it replaces

There is no `focus`, `focus-mode` or `data-focus` anywhere in
`internal/render/viewer/template/`. What exists instead is **exactly the two
independent toggles R11.1 abolishes**:

| Today | Address |
|---|---|
| Left-rail toggle button (markup) | `internal/render/viewer/template/shell.html:61-63` — `<button id="sidebarCollapseToggle" class="system-panel-toggle system-panel-toggle--sidebar" … aria-label="Hide navigation">` |
| Left-rail toggle binding | `internal/render/viewer/template/system-record.js:82-94` (`bindSidebarCollapse`) |
| Left-rail collapsed state | `internal/render/viewer/template/style.css:3161-3186` (`body.system-sidebar-collapsed …`, inside `@media (min-width: 861px)` opened at `:3157`) |
| Right-rail toggle button (markup, injected) | `internal/render/viewer/template/system-record.js:356` — the `.system-panel-toggle--toc` button inside `renderToc`'s `toc.innerHTML` |
| Right-rail toggle binding | `internal/render/viewer/template/system-record.js:361-367` — the `.system-panel-toggle--toc` click handler that toggles `body.system-toc-collapsed` (`:357-360` is the `.facet-toc__select` change handler, a different control) |
| Right-rail collapsed state | `internal/render/viewer/template/style.css:3717-3736` (`body.system-toc-collapsed .facet-toc { width: 44px; … }`) |
| Content-area re-padding when the right rail collapses | `internal/render/viewer/template/style.css:3378-3380` — `@media (min-width: 1181px) { body.system-toc-collapsed .content-area { padding-right: 86px; } }` |
| Shared toggle chrome | `internal/render/viewer/template/style.css:3122-3156` (`.system-panel-toggle`, `:hover`, `__chevron`, `--sidebar`) |
| The test that pins **both** toggles as independent | `viewer-tests/claim_collapse_test.go:168-202` — `TestDesktopNavigationPanelsCanCollapseAndExpand`; asserts each rail collapses to exactly `44px` and flips `aria-expanded` / `aria-label` |

That test is the contract a focus-mode implementation has to renegotiate: it
asserts two separately-clickable panel toggles, which R11.1 forbids.

### 7.2 Layout — the widths focus mode changes

| What | Address |
|---|---|
| `.content-area` reading gutter (System Record layer) | `style.css:3369-3376` — `padding: 0 282px 80px 42px; transition: padding 180ms ease-out;`. **The 282px right gutter is the reserved TOC rail**; focus must reclaim it. |
| `.content-area` (docs layer, lower specificity) | `style.css:2784-2801` |
| `.content-area` narrow-desktop | `style.css:4494-4496` — `@media (min-width: 861px) and (max-width: 1180px) { .content-area { padding: 0 34px 76px; } }` |
| `.content-area` tablet/phone | `style.css:4533-4540` — inside `@media (max-width: 860px)` |
| Width transition | `style.css:3157-3160` — `@media (min-width: 861px) { .content-area { transition: width 180ms ease-out, padding 180ms ease-out; } }` |
| Precedent for reclaiming the rail's padding | `style.css:3382-3394` — the Build-order tab's `@media screen and (min-width: 1181px) { body:has(.build-order-section:not([hidden])) .content-area { padding-right: 42px; } }`, with a 12-line comment explaining why the `screen and` scope and the `1181px` floor are load-bearing. **A focus rule is the same shape and must not fight it.** |
| Right rail (facet TOC) | `style.css:3668-3790` — `.facet-toc` through `.facet-toc__item.on` |
| Left rail | `style.css:2585-2832` (`/* ---- sidebar + tab navigation ---- */`), sidebar collapse at `:3161-3186` |
| Module head (title / summary / metric) | `style.css:3395-3433` (`.system-record-head`, `h2`, `__summary`, `__metric`, `__metric strong`); print override `style.css:4748`; narrow `style.css:4593-4606` |
| Module head construction | `system-record.js:403-430` (`addModuleHeaders`) — builds `.system-record-head` with `'<strong>' + locked + ' of ' + total + '</strong> claims locked'` |
| Facet tab strip | `style.css:3435-3459` (`.sub-nav` sticky at `top: 0`, `.subtab`, `.subtab:hover`, `.subtab.on`); docs layer `style.css:2756-2782`; phone `style.css:4607-4617` |
| Facet tab markup | `shell.html:137-143` — `<div class="sub-nav">` / `<button class="subtab" data-target="#{{.ID}}">` |
| Module section markup + counts | `shell.html:136` — `data-claim-count` / `data-locked-count` / `data-facet-count` |

### 7.3 The claim card and its footer

| What | Address |
|---|---|
| Claim card template | `internal/render/components/card.html:38-43` — `<section class="claim claim-card card…">`, `.k` head line with the inline status pill, `.claim-body`, `{{edges .}}` |
| Status pill class mapping | `internal/render/components/components.go:251-261` (`pillClass`) |
| Status label text | `internal/render/components/components.go:262-300` (`StatusLabel`) |
| Footer `<details>` emission | `internal/render/components/components.go:569-573` — `<details class="claim-links"><summary class="claim-links-summary">…</summary><ul class="claim-edges">` |
| Footer count phrasing (server side) | `internal/render/components/components.go:608-615` (`countSegment`) |
| Comment chip emission | `internal/render/components/components.go:659-696` (`CommentChipHTML`), slot at `:662`/`:667` |
| Footer chrome | `style.css:4020-4064` — `.claim-links`, `.claim-links-summary` (`min-height: 56px; padding: 11px 13px; font: 650 10px/1.35 var(--font-mono)`), `.claim-footer__identity`, `.claim-footer__title`, `.claim-footer__counts`, `.claim-footer__chevron` |
| Footer hover wash (deliberately **not** `--hover-bg`) | `style.css:4041-4047` |
| Footer disclosure chevron rotate | `style.css:3302` — `.claim-links[open] .claim-footer__chevron` |
| Footer docs-layer copy | `style.css:1184-1212` |
| Footer re-labelling at runtime | `system-record.js:122-155` (`enhanceFooters`) — rewrites the summary into `.claim-footer__identity` + `.claim-footer__counts`, emitting `relationship(s)`, `source(s)`, `file(s)`, `drifted` |
| Footer at ≤860 | `style.css:4638-4645` |
| Footer at ≤560 | `style.css:4647-4658` — **the rules a 520px two-row footer must be ordered after** |
| Deep-link auto-open of the footer | `style.css:1213-1250` (`.claim:target .claim-links …`) |

### 7.4 Readiness / blockers expansion

| What | Address |
|---|---|
| Section marker + rationale | `style.css:807-809` — *"Claim readiness is a reviewer-facing work queue. The policy engine remains…"* |
| Readiness chrome | `style.css:810-1159` — `.claim-readiness`, `-head`, `-id`, `-title`, `-state`, `--ready`, `-summary`, `-counts`, `-count`, `-label`, `-section-head`, `-local`, `-routes`, `-route`, `-route-id`, `-route-via`, `-route-count`, `-relation` |
| Readiness at ≤860 / ≤520 | `style.css:1160-1174` / `style.css:1175-1183` |
| Readiness DOM construction | `viewer-runtime.js:1516-1620` (`renderClaimReadiness`) — builds `<section class="claim-readiness">` at `:1524-1526`, head at `:1533-1551`, counts at `:1552-1567`, local block at `:1568-1586`, route header at `:1587` |
| Blocker row | `viewer-runtime.js:1444-1454` (`readinessFactRow`) — `.claim-readiness-blocker` / `-blocker-copy` / `-relation` |
| Route group | `viewer-runtime.js:1455-1492` (`readinessRoute`) — `-route`, `-route-id`, `-route-via` (`'local review'` / `'direct dependency'` / `'shown via this dependency'`), `-route-count` (`'blocker'`/`'blockers'`), `-chevron`, `-route-body`, `-blockers` |
| Hop label | `viewer-runtime.js:1375-1384` (`readinessHopLabel`) |
| Blocker title from the claim's label | `viewer-runtime.js:1332-1344` (`readinessClaimLabel`), `:1345-1368` (`readinessFactLabel`) |
| Dependency-path `<details>` | `viewer-runtime.js:1437-1443` (`readinessPathDetails`) |
| Inline map (**R09.9 says this must go**) | `viewer-runtime.js:1477-1492` — `.claim-readiness-trace` / `-map` / `-map-caption` (`'Representative routes · scroll horizontally'`) / `-map-scroll` / `-map-limit` |
| Raw diagnostics (**R09.8 demotes this**) | `viewer-runtime.js:1493-1515` (`readinessRawDiagnostics`, `.claim-readiness-raw`) |
| Offline fallback | `viewer-runtime.js:1621-1637` (`offlineReadiness`) |
| Tests asserting these selectors | `viewer-tests/claim_readiness_test.go:77-92` (node/row/route/map census), `:117` (`#widget\.contract\.l000-n00 .claim-readiness`), `:122` (`.claim-readiness-blocker` count), `:154-167` (`.claim-readiness-route`, `-trace`, `-map svg`), `:179-185` (`-map-limit`, processed maps) |

### 7.5 Blocked banner / Issues entry point

| What | Address |
|---|---|
| Section marker + z-index ledger | `style.css:2012-2054` |
| Strip chrome | `style.css:2055-2210` — `.status-strip` (`:2059`), `[hidden]` (`:2069`), `--lint` (`:2073`), `--integrity` (`:2078`), `-head` (`:2084`), `-summary` (`:2097`), `-title` (`:2103`), `-note` (`:2109`), `-action` (`:2115`), `-caret` (`:2123`), `--open .status-strip-caret` (`:2130`), `-body` (`:2136`), `-filters` (`:2147`) |
| Strip at coarse pointer / ≤860 | `style.css:2211-2226` / `style.css:2227-2276` |
| Strip in the System Record layer | `style.css:4187-4191` |
| Strip hidden in print | `style.css:4754` |
| The headline sentence | `viewer-runtime.js:1245-1262` (`blockerHeadline`) — emits `'<n> claims blocked by unapproved dependencies'` for `dependency_unapproved`, the board's sentence minus `outside this module` |
| The action label | `viewer-runtime.js:1725` — `stripAction.textContent = stripExpanded ? 'Hide issues' : 'Show issues'` |
| Strip render / expand | `viewer-runtime.js:1638-1720` (`renderStatusStrip`), `:1721-1733` (`setStripExpanded`), `:1734-1802` (`refreshStatus`) |
| Severity vocabulary | `viewer-runtime.js:1148-1238` (`addStatusGroup`, `collectStatusGroups`, `statusGroupClaimCount`, `uniqueClaimCount`, `countSeverity`), `:1239-1244` (`statusChipLine`) |
| Test | `viewer-tests/claim_collapse_test.go:204` — `TestStatusStripShowsOnlyActiveFacetIssues` |

### 7.6 Comment count

| What | Address |
|---|---|
| Section marker | `style.css:1589-1601` |
| Chip chrome | `style.css:1602-1616` (`.comment-chip`), `:1617-1642` (`--open`), `:1643-1648` (`--empty`), `:1649-1671` (reveal-on-hover, with the "must stay AFTER" ordering note at `:1661`), `:1672-1675` (`-count`) |
| Coarse-pointer 44px minimum | `style.css:1760-1773` — `@media (pointer: coarse) { .comment-chip { min-height: 44px } … }` |
| Mobile sheet / overlay | `style.css:1774-1804` (`/* ---- interactive comment UI ---- */`), `:1789-1804` (`body.comments-open` scroll lock), `:1805` (`.comments-overlay` `display: none` above 860) |
| Mobile rail-as-bottom-sheet | `style.css:2223-2276` |
| Chip runtime | `viewer-runtime.js:462-533` (`chipsFor`, `setChipExpanded`, `updateChips`, `syncEmptyChips`, `recomputeChipsFromPanel`) |
| Panel open/close | `viewer-runtime.js:534-569` |
| Test | `viewer-tests/empty_chip_test.go` |

### 7.7 Theme / dark mode — where the tokens live

| What | Address |
|---|---|
| Light-first contract (read before touching anything) | `style.css:1-47` |
| Unconditional `:root` (light + print values) | `style.css:49-106` |
| Explicit dark toggle | `style.css:107-126` — `@media screen { html[data-theme="dark"] … }` |
| OS dark | `style.css:127-160` — `@media screen and (prefers-color-scheme: dark) { :root … }` |
| `--dxg-facet-other` re-declaration that wins over `graph.css` | `style.css:76`, `:120`, `:140` |
| Graph ramp (dark-first, opposite convention) | `internal/render/viewer/template/graph.css:1-30` (header), `:107-160` (the `--dxg-facet-other` / maximin CIEDE2000 rationale), `:161-200` (`:root` dark ramp, `--dxg-facet-1: #7C8CE8` at `:164`) |
| Theme emission | `internal/render/render.go:963-1008` (`themeOverrideCSS`), `:1020-1052` (`writeBlock`) |
| Allowlist | `internal/config/config.go:159` (`ThemeTokenAllowlist`) — **28 keys, order load-bearing, closed** |
| Token-reaches-a-consumer test | `internal/render/theme_tokens_test.go` |
| Theme choice in `localStorage` — **the precedent R11.5 names** | `shell.html:24-32` (boot read, key `dossierx-theme`), `system-record.js:432-440` (`applyThemeChoice`), `:441-454` (`bindThemeControl`), `:435` (`localStorage.setItem('dossierx-theme', choice)`) |
| Theme control markup | `shell.html:124-128` |
| Tests | `viewer-tests/theme_modes_test.go:375` (`TestEveryThemeTokenReachesTheReader`), `:501` (`TestThemeModesTrackTheColourScheme`), `:593` (`TestDarkOnlyTokenDoesNotApplyToPrint`) |

### 7.8 Breakpoints, print, and the tests that guard the layout

| What | Address |
|---|---|
| `@media (max-width: 860px)` | `style.css:1160`, `:2227`, `:2932`, `:4424`, `:4533` |
| `@media (max-width: 640px)` | `style.css:2563` |
| `@media (max-width: 560px)` | `style.css:4647` |
| `@media (max-width: 520px)` | `style.css:1175` |
| `@media (pointer: coarse)` | `style.css:1764`, `:2211` |
| `@media (min-width: 861px)` | `style.css:3157` |
| `@media (min-width: 861px) and (max-width: 1180px)` | `style.css:4494` |
| `@media (min-width: 1181px)` | `style.css:3378`, `:3391` |
| The one `@media print` block, last in the file | `style.css:4660-4720` (the header explaining why), `:4721-4782` (the block); `.content-area { padding: 0 }` at `:4755`, `.status-strip { display: none !important }` at `:4754`, `.system-record-head { padding-top: 0 }` at `:4748` |
| Claim body must use the card's full content width | `viewer-tests/system_record_layout_test.go:14-40` — `TestClaimBodyUsesAvailableCardWidth`, at `1600 × 900` and `600 × 800` |
| Rail collapse behaviour | `viewer-tests/claim_collapse_test.go:168-202` |
| Per-claim and per-facet collapse | `viewer-tests/claim_collapse_test.go:79`, `:125`, `:140`; runtime `system-record.js:185-240` |
| Theme parity census (reads `.facet-toc`, `.facet-toc__select`, `.facet-toc__item`, `.sec-tab`, `.claim-links-summary`, `.claim-banner`) | `viewer-tests/theme_parity_test.go:149-165`, `:211-224`, `:332`, `:744-770`, `:1272` |
| Reading-view browser scale budget | `viewer-tests/reading_view_scale_test.go:142` — `TestReadingViewBrowserScaleBudgets` |
| Soft-mount / surface template | `shell.html:145-152`; `viewer-runtime.js:170-215` (`softMountEnabled`, `mountSurface`, `mountAllSurfaces`) |

### 7.9 Where a focus-mode implementation lands

Not code, an address list. A lane agent implementing this screen touches:

- **`style.css`** — one new state block. The rail-hiding rules belong beside
  `style.css:3161-3186` and `:3717-3736` (the two collapse blocks they replace);
  the width reclaim belongs beside `style.css:3378-3394` and must be
  `@media screen and (min-width: 1181px)` for the reason that block's own comment
  gives.
- **`shell.html`** — the Focus button. The left toggle at `shell.html:61-63` is
  the markup it replaces; the right toggle is injected by `system-record.js:356`
  and is removed there.
- **`system-record.js`** — one binding, modelled on `bindThemeControl`
  (`:441-454`) because R11.5 makes focus the same kind of per-reader state;
  `bindSidebarCollapse` (`:82-94`) and the `renderToc` toggle binding
  (`:361-367`) are the two it collapses into one.
- **`viewer-tests/claim_collapse_test.go:168-202`** — the test that currently
  asserts the two-toggle behaviour.

---

## 8. States not on the boards

The boards draw one module, two locked-and-blocked claims, one open expansion and
one banner. Everything below is a state the engine can render on this screen that
no board in group 03 depicts. For each, what the rules imply — so no lane is left
guessing.

### 8.1 Focus control states

Only the **active** state is drawn (`17G-0`, `59F-0`).

- **Rest (focus off).** Implied: the same 28px / 7px-radius / 11px-padding shell
  with no accent tint and no accent border — a neutral control that reads as the
  other header utilities do. It must still carry the label and the `F` hint,
  because discoverability is the only reason the hint exists.
- **Hover.** Not drawn. The screen introduces no hover language of its own; use
  the engine's existing `--hover-bg` wash (`style.css:3137`
  `.system-panel-toggle:hover`), not a new one.
- **Focus-visible.** Not drawn. Required — the control is keyboard-primary by
  design (R11.4). Use the engine's existing focus-visible treatment; do not
  invent a ring for this one button.
- **Disabled / absent.** Below 861px the control is **not rendered** (§5). It is
  never rendered disabled: a greyed Focus button would advertise a feature the
  device cannot have.
- **Edge-peek (R11.4).** Not drawn on any board. Implied: a temporary reveal of
  one rail while the pointer is at that edge, which **does not change the
  persisted state** and does not relight or dim the control. It has no touch
  analogue; see Open decisions.

### 8.2 Claim status other than LOCKED

Both claims are `LOCKED`. The engine renders three more:

- **DRAFT.** `--color-draft` `#9A6A16` / `--color-dark-draft` `#DDA94E`, tint
  `--color-draft-bg` / `--color-dark-draft-bg`, at the same chip geometry
  (`4px` block / `8px` left / `10px` right, `999px`, label 11/14 weight 600
  `+0.05em`). **`tokens.md` Disagreement 9 applies**: the engine's
  `--status-draft` has no dark override today, so a draft chip on dark renders
  `#976600` until that is fixed. Focus mode changes nothing about it.
- **BLOCKED as a status** (distinct from the readiness chip in the footer):
  `--color-blocked` / `--color-dark-blocked` with `--color-blocked-bg`.
- **review_pending, all three triggers.** The engine emits
  `<li class="claim-review-pending">review_pending</li>`
  (`components.go:483`) and the `pill pw` class (`style.css:1547-1555`,
  `:2324-2330`). No board in this group draws it. Implied: it is a **status chip
  variant**, not a fourth chip in the footer strip — the head row holds exactly
  one status chip, and review_pending replaces the word inside it rather than
  sitting beside it. Which of the three triggers fired is a claim-detail fact and
  belongs behind the provenance disclosure (R09.8), not in the head.

### 8.3 Claim not blocked

- **Ready.** `claim-readiness--ready` exists (`style.css:854-862`). Implied for
  the footer strip: the leading item loses its `7px` `--color-blocked` dot and
  its blocked-red word, and becomes a plain noun-and-count chip in
  `--color-muted` like the other three. The divider after it (`1 × 12`
  `--color-border`) exists to separate a *status* from *counts*; with no status
  to separate, **drop the divider**.
- **Auto-open.** R09.3: readiness may auto-open **only when the claim is
  blocked**. A ready claim opens nothing. The 1140px width is still taken —
  focus is about the page, not about whether a particular claim has evidence.
- **Banner absent.** When no claim in the facet is blocked the banner is not
  rendered, and the card's `12px 12px 0 0` radius (`15B-0`) belongs to the first
  claim instead. The first claim then needs the banner's top padding, not its
  own `34px`, or the column will start with a visibly deeper gap than it has on
  the drawn board.

### 8.4 Zero and empty states

- **Zero comments.** Drawn — `0` renders plainly on `34R-0`. But the engine hides
  an empty chip until the card is hovered (`style.css:1643-1670`,
  `.comment-chip--empty` + the `.card:hover` reveal rule). The board contradicts
  that. Implied and decided: the **footer-strip count** always renders, including
  `0`; the hover-reveal rule belongs to the card-level chip, not to the count in
  the strip. On `(pointer: coarse)` nothing may depend on hover at all.
- **Zero checks.** Drawn as `No checks declared` (`35H-0`). Implied for the other
  three: the same sentence form — `No sources cited`, `No relationships`,
  and for readiness, the ready-chip form above. Never `0 sources`.
- **Empty facet.** `.claims-empty` (`shell.html:157` module, `:196` track; styled at `style.css:210`). Implied: focus
  still applies — the page is 1140px wide with an empty column. Focus is a
  property of the reader, not of the content (R11.6: same claims, same order,
  more room; here, no claims, more room).
- **Single-facet module.** The `.sub-nav` strip is rendered only for a module
  with more than one facet (`style.css:2756-2760` states this). Implied: with one
  facet the tab strip is absent but the header's `1px --color-border` bottom rule
  must still be drawn, or the module head will float without a base.

### 8.5 Content the boards do not stress

- **Long claim titles.** The head row is `flex` with the chip at
  `flex-shrink: 0` (`16R-0`), so the title wraps to a second line and the chip
  holds its lane. On mobile the chip is above the title precisely so the title
  gets the whole 358px (R-H.2). Implied: no truncation, no ellipsis — a claim
  title is what the reviewer recognises the claim by (R09.8).
- **Long claim ids.** Mono `12 / 16` with `overflow-wrap: anywhere` set on the
  board's root. Implied: ids wrap; they are never truncated, because an id is
  what a reader types into `dossierx claim lock`.
- **Deep dependency paths.** The drawn paths are 2 and 3 hops. Implied: desktop
  keeps the single line (this is what focus mode's 1140px buys — the notes strip
  says so in as many words), and mobile wraps within the gutter
  (`flex-wrap: wrap`). If a path exceeds the desktop line at 1140px it wraps
  there too; it is never truncated and never becomes a map (R09.9).
- **Cycles.** `--color-graph-cycle` is a **graph** state. In the reading view a
  cycle shows up as a blocker path whose slug sequence repeats an id. R09.9
  forbids drawing the shape inline: the row states the path, and
  `See in claims graph` is where the cycle is seen. No new colour is introduced
  in the reading view for it.
- **More than one blocker group.** The board draws one group
  (`Capability support`). Implied: groups stack with the same `12px` panel gap;
  each keeps its own chevron, count pill and `nearest N hop` note. The
  `2 in 1 module` scope count in the eyebrow becomes `N in M modules`.
- **Three-digit counts.** Tab counts and chip counts are mono and unpadded. The
  desktop tab strip is fixed at 1140px with a `26px` gap and has room; the mobile
  strip has a `flex: 1; min-width: 8px` spacer before `On this facet`, which is
  what absorbs a wider count. Implied: the mobile strip must not wrap — the
  spacer collapses to `8px` first, then `On this facet` may shorten, and only
  then does anything else give.

### 8.6 Modes and surfaces focus has to survive

- **Print.** Focus is a screen state. The single `@media print` block
  (`style.css:4721-4782`) already drops chrome and closes gutters
  (`.content-area { padding: 0 }` at `:4755`). Implied: focus adds **no**
  print-only branch, and the printed page is identical whether or not focus was
  on when Print was hit.
- **Build order, tracks and the graph pane.** These are siblings of a module
  section (`shell.html:160-230`) and share `.content-area`. Implied: focus is one
  body-level class and applies to all of them. The Build-order tab already
  reclaims the right rail's padding at the same `min-width: 1181px`
  (`style.css:3392`); focus and that rule must compose, not fight — a build-order
  tab in focus mode must not end up with a double-reclaimed gutter.
- **`dossierx serve`.** R10.5's `Live` line lives in the rail footer, which focus
  hides, so in focus mode there is no freshness statement on screen at all.
  Implied and accepted: per R11.4 nothing replaces it. A reader who wants to know
  the build's age leaves focus.
- **Deep links.** `.claim:target` auto-opens a claim's footer
  (`style.css:1213-1250`) and a source row (`:1517-1546`). R11.6 makes focus
  reversible and lossless, so a deep link must land identically with focus on or
  off — same claim, same scroll position, same auto-open. Focus must not be
  entered or exited by a hash change.
- **SSE fragment swap under `serve`.** The runtime replaces
  `<main class="content-area">` wholesale (`shell.html:230`, `:292`;
  `viewer-runtime.js:2075-2148`). Implied: focus state lives on `<body>` or in
  `localStorage`, never inside the swapped subtree, or a live rebuild silently
  drops the reader out of the mode — which is exactly the failure R11.5 is
  written against.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

1. **Reading measure: 760px prose inside an 1140px page.** Confirmed by direct
   measurement on this screen — the cap sits on each prose run, not on a
   container (`16Q-0` / `1OS-0` on light, `5AD-0` / `5AE-0` on dark, all four
   `max-width: 760px`; the light wrapper `16N-0` has none), while
   `15B-0` / `59V-0` = `max-width: 1140px`. That agrees with board 11 and with
   `tokens.md` § Open decisions. `--container-reading: 720px` is **not** used on
   this screen and `--container-rail` / `--container-toc` are unused by
   definition.

2. **Mobile prose has no measure.** At 390 the prose fills the 358px gutter
   (`5EY-0` / `5I6-0` carry no `max-width`). R11.2's "the measure never changes"
   is a focus-mode invariant at one width, not a promise across breakpoints.

3. **The desktop footer strip is a bare label-and-chevron line; the 30px pill is
   the ≤520 form only.** The components board specifies the pill under section I3
   ("expansion forms", whose own note reasons about 12px on a phone) and never
   specifies a desktop chip. **R-I.3** settles that this is legitimate rather
   than a conflict: *"The detail chip is the component; the footer's two rows are
   composition"* (Evidence: section I3, `7N2-0`) — so the pill's geometry is the
   chip and the desktop line is a second composition of the same chip, not a
   second component. Decided: the desktop form measured here
   (`34R-0` / `35H-0`) is the desktop form, and this spec is its source of truth
   until the components board gains a desktop section. Recorded as a gap, not a
   contradiction.

4. **One blocked-tint alpha per mode.** The boards use six. Decided: every
   blocked ground on this screen takes `--color-blocked-bg` (10 %) on light and
   `--color-dark-blocked-bg` (13 %) on dark — banner, footer chip, count pill and
   target slug alike. Implement the token, never the board hex.

5. **Untokenised hairlines and grounds become token-derived mixes, not
   literals.** The footer-strip rule (`#E8ECF1` / `#212934`) is a lighter tier
   than `--color-border`; express it as a mix of `--border` into `--card-bg`, in
   the same idiom `style.css:4024` already uses for `.claim-links`' ground
   (`background: color-mix(in srgb, var(--paper) 55%, var(--card-bg))`). The
   light readiness ground (`#FBF7F7`) is a mix of `--warn` into `--card-bg`; the
   light hairlines (`#EBD9D8`, `#EFE6E5`) are heavier mixes of the same pair. On
   dark the ground is `--color-dark-blocked-surface` and the hairline is
   `--color-dark-blocked-hairline` — **both of which have no engine token**
   (`tokens.md` Disagreement 10) and must be derived the same way. No new
   allowlist key: the allowlist is closed and its order is load-bearing.

6. **The mobile blocked banner is implemented even though the boards omit it.**
   R09.6 makes the banner the only entry point to the Issues screen and R-H.1
   gives it a specific mobile form (inset card, 16px side margins, 10px radius,
   1px tinted-red border, a divided 38px action row). A phone with no way into
   Issues is a correctness failure, not a simplification. The group-03 mobile
   boards are treated as showing the banner-less case (no blocked claims in the
   facet), not as deleting the component.

7. **The mobile comment count takes the accent treatment.** R-F.3 is explicit:
   on mobile it is the control that opens the sheet, so it takes
   `--color-accent-bg` with an `--color-accent` icon and label. The boards draw
   it as the desktop passive count. The reference rule wins (R00.0). The tint is
   `color-mix(in srgb, var(--link) 9%, transparent)` per `tokens.md` § Open
   decisions, since `--color-accent-bg` has no engine home.

8. **The mobile blocker row follows component I1, not this screen's boards.**
   R-I.1 puts the hop pill after the title and stacks the path one slug per line
   behind a leading chevron; boards `5DS-0` / `5H1-0` right-range the hop pill on
   the title line and wrap the path. The notes strip endorses the board's form,
   but reference-rules' own open decision settles it: *"Where a reference board
   and a screen board disagree, the reference board wins, and the screen is
   flagged."* The hop-pill text also reverts to the full `direct · 1 hop` form,
   since the shortened `1 hop` existed only to fit the right-ranged layout.

9. **Focus replaces both toggles; it does not join them.**
   `#sidebarCollapseToggle` and `.system-panel-toggle--toc` are removed, not
   kept alongside a third control. R11.1's whole point is that two toggles
   produce four layouts. `viewer-tests/claim_collapse_test.go:168-202` is
   rewritten as part of the change, not worked around.

10. **`F` is inert while a text field has focus.** Not stated on any board.
    Decided: the binding no-ops when `document.activeElement` is an `input`,
    `textarea` or `contenteditable` — the comment composer
    (`viewer-runtime.js:826-846`) is one keystroke away from the reading view,
    and a reader typing "Focus mode is wrong here" must not be thrown into it.

11. **Edge-peek is `(pointer: fine)` only.** R11.4's "push the pointer to either
    edge" has no touch analogue, and mobile has no rails to peek. Decided: the
    peek affordance is bound only where a fine pointer exists, and its absence on
    touch is not compensated for.

12. **The persisted key is `dossierx-focus`, beside `dossierx-theme`.** R11.5
    names `localStorage` but not a key. Decided by symmetry with
    `system-record.js:435` / `shell.html:28`. Like the theme, it is read at boot
    before first paint so the page does not flash the rails in.

13. **The freshness line is simply absent in focus mode.** No substitute, no
    relocation into the header. R11.4 forbids a hint strip, and R10's rules bind
    the rail footer, which is gone. A reader who needs the build's age exits
    focus.

14. **The serif gap is closing under this screen, and this spec assumes it.**
    `tokens.md` Disagreement 6 records that the engine has no serif family, which
    would make every `--text-body` run on this screen (claim prose at `17 / 28`,
    the resolution note at `14 / 22`) unimplementable. While this spec was being
    written another lane began landing engine-owned faces — the working tree
    shows `internal/render/engine_fonts.go` added, `geist_fonts.go` deleted, and
    `inter-latin-wght.woff2`, `ibm-plex-mono-latin-{400,500,600}.woff2` and
    `source-serif-4-latin-opsz-wght.woff2` added under
    `internal/render/viewer/template/fonts/`. Decided: this screen is specified
    on the assumption that an engine-owned `--font-serif` exists by the time it
    is implemented, per `tokens.md`'s "treat `--font-serif` as a new
    engine-owned token, not a theme token". A lane agent that finds it still
    missing stops and says so rather than substituting the sans stack — the
    serif/sans/mono division on this screen is semantic (`tokens.md` §7), not
    decorative.

15. **Off-scale values on this screen stay as per-component literals.** The
    `7px` radius on the Focus control and the `On this facet` trigger, the
    `-0.01em` claim-title tracking, the `+0.05em` chip-label tracking, the `112px`
    hop column and the `12px` card radius are all measured, deliberate, and
    outside the token scales. Per `tokens.md` § Open decisions they are not
    promoted into the scales.

### Paper defects

Found on the group-03 boards. Each is a thing the board does that the board's own
rules, or `tokens.md`, forbid. **None is to be copied.**

1. **The light desktop board writes raw hexes; its dark twin writes tokens.**
   `13J-0` and every descendant spell colour as `#101720`, `#54606F`, `#6E7C8E`,
   `#DDE2E9`, `#1C4E8C`, `#2C6B52`, `#9E3B36`, `#EFF1F4`, `#FFFFFF`; `594-0`
   spells `var(--color-dark-ink)`, `var(--color-dark-muted)` and so on. The same
   value is authoritative in one board and editorial in the other.
   (`tokens.md` Disagreement 12, instanced here.)

2. **Font families are spelled two ways within one group.** `13J-0` uses
   `var(--font-sans)` / `var(--font-mono)` / `var(--font-serif)`; `594-0`,
   `5DS-0` and `5H1-0` use the literals `"Inter", system-ui, sans-serif`,
   `"IBM Plex Mono", system-ui, sans-serif`, `"Source Serif 4", system-ui,
   sans-serif`. The mono literal's fallback is **`sans-serif`, not
   `monospace`** — a machine without IBM Plex Mono renders claim ids in a
   proportional face, which is the one thing mono exists here to prevent.

3. **The blocked tint takes six alphas on one screen.** Light: `#9E3B360F` (6 %,
   banner `170-0`), `#9E3B3617` (9 %, mobile blocked chip `5MI-0`), `#9E3B361A`
   (10 %, count pill and target slug `31D-0` / `5FL-0`). Dark: `#EC8A831A`
   (10 %, banner `59W-0`), `#EC8A8321` (13 %, mobile blocked chip `5N9-0`),
   `#EC8A8329` (16 %, count pill and target slug `5B8-0` / `5IT-0`).
   `--color-blocked-bg` is 10 % and `--color-dark-blocked-bg` is 13 %. Only two
   of the six land on a token.

4. **Six hairline and ground colours have no token.** Light `#E8ECF1`
   (footer-strip rule, `34R-0` / `35H-0` / `5MI-0`), `#FBF7F7` (readiness ground,
   `31D-0` / `5FL-0`), `#EBD9D8` (eyebrow rule), `#EFE6E5` (row divider). Dark
   `#212934` (footer-strip rule and readiness top rule, `5AI-0` / `5B8-0` /
   `5N9-0`), `#33262A` (row divider, `5B8-0` / `5IT-0`). Note the **asymmetry**:
   the dark readiness *ground* correctly uses `--color-dark-blocked-surface` and
   its eyebrow rule correctly uses `--color-dark-blocked-hairline`, while the
   light board's equivalents are bare hexes with no light twin token at all.

5. **The Focus control's icon carries two conflicting stroke declarations.** On
   `17K-0` (the icon inside the Focus button `17H-0`) the SVG element's
   inline style sets `stroke: rgb(28, 78, 140)`
   (`--color-accent`) while the `<path>` attribute sets `stroke="#54606F"`
   (`--color-muted`). The style wins in Paper, so the board *looks* right, but
   markup transcribed literally would ship a muted glyph inside an accent
   button. The dark twin `59I-0` (inside the button `59H-0`) puts
   `var(--color-dark-accent)` on the `<path>` alone, with no competing element
   style, and is correct.

6. **The notes strip contradicts board 11 on what widens.** `5KP-0` column 1:
   *"the measure widens from 760 to 1140"*. Board 11 rule 1 (`1ED-0` / `1EE-0`):
   *"Claim prose stays at 760px in both modes."* Board 11 rule 2 (`1EI-0` /
   `1EJ-0`): *"The page grows to 1140px."* The board's own geometry
   (`16Q-0` = 760, `15B-0` = 1140) proves board 11 right. The notes strip's
   sentence, read literally, instructs a lane agent to violate R11.2.

7. **The mobile blocker row is a fourth layout for a component that already has
   one.** `5DS-0` / `5H1-0` right-range the hop pill on the title line and wrap
   the path with `flex-wrap: wrap`; component I1 (`7N2-0`, R-I.1) puts the hop
   pill after the title and stacks the path one slug per line behind a leading
   chevron. The hop text also differs (`1 hop` vs `direct · 1 hop`). This is
   precisely the drift R00.0 was written about — *"the blockers panel had drifted
   into two designs… with nothing in the file saying which was right"* — and the
   screen's own note endorses the drift, which is why approval is not proof.

8. **The mobile comment count is drawn as the desktop passive count.**
   `5MI-0` / `5N9-0` render it in `--color-faint` with no tint and no pill.
   R-F.3 requires the mobile variant to be the accent-tinted control that opens
   the sheet. As drawn, the phone boards give the reader no visible way to reach
   a comment thread.

9. **The mobile boards drop the blocked banner.** No `H1` component appears on
   `5DS-0` or `5H1-0`, although both claims are blocked and the desktop twin
   carries the banner for the same facet. R09.6 makes it the only entry point to
   Issues and R-H.1 gives it a mobile form. Nothing on the boards or in the notes
   strip explains the omission — unlike the missing Focus control, which is
   explained.

10. **The desktop progress track is drawn twice as differently as it should be.**
    Light `17N-0` draws a `--color-border` track with a `--color-locked` fill
    inside it; dark `59E-0` draws only a `--color-dark-locked` rectangle with no
    track. At 31/31 the two look identical, which is exactly why the difference
    survived review — at 20/31 the dark board would have no track to show the
    remainder against.

11. **Board 11's DECIDED panel border `#A9CBBB` has no token.** A lightened lock
    green invented for one panel. (`tokens.md` Disagreement 12; restated because
    board 11 governs this screen and a lane agent will open it.)

12. **The light readiness panel's top rule and the light footer strip's top rule
    are the same hex `#E8ECF1`, but the mobile readiness panel's top rule is
    `#EFE6E5`.** Three rules that should be two tiers are drawn as three values.

13. **The claim prose is structured two different ways on the light and dark
    desktop boards.** On `13J-0` the two paragraphs sit inside a wrapper frame
    `16N-0` (`flex-direction: column`, `gap: 16px`, width 1074px, no `max-width`,
    no font), and the 760px cap is repeated on each child (`16Q-0`, `1OS-0`). On
    `594-0` there is no wrapper: `5AD-0` and `5AE-0` are direct children of the
    claim block `5A3-0`, each carrying its own `max-width: 760px`, and the 16px
    paragraph gap comes from the claim block's own gap instead. The rendered
    result is identical, so nothing looked wrong at review — but a reader
    comparing the two boards node-for-node finds no counterpart for `16N-0`, and
    a lane agent transcribing either board gets a different DOM. Harmless as
    drawn; a trap for anyone citing node ids. Both structures agree on the value
    that matters (R11.2's 760px), so the rule is not at risk — only the
    attribution was, and §2, §3, §4.7 and §9 OD1 now name the runs rather than
    the containers.
