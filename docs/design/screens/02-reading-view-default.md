# 02 · Reading view — default

Screen group 02. Source of record: Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page
`1-0`. Every number below was read with `get_computed_styles` or `get_jsx`
(inline-styles format); nothing is measured off a screenshot. Screenshots were
used only to establish what the screen is trying to say and to decide what to
measure.

Read with `../tokens.md` (token names and the engine mapping) and
`../reference-rules.md` (the rules, cited below by rule id). Where a board and a
reference rule disagree, the reference board wins and the screen board is
flagged as a Paper defect — `reference-rules.md` § Open decisions, generalising
R00.0.

---

## 1. Boards

| Node | Name | What it shows | Viewer states depicted |
|---|---|---|---|
| `3B-0` | 02 · Reading view — default | 1440 × 1024 desktop, light. Three columns: modules sidebar, claim column, `ON THIS FACET` panel. | Module `Permission readiness`, facet `Contract` active; **all 31 claims LOCKED**; **every claim blocked** by dependencies outside the module; facet-level blocked banner present; three claim cards visible, comment counts `0` and `2`; freshness = Fresh (`Updated 23 hours ago`); Focus control at rest. |
| `4FO-0` | 02 · Reading view — default · DARK | The same screen, dark. | Identical states; theme toggle shows `Dark` selected. |
| `4O0-0` | 02 · Reading view — default · MOBILE LIGHT | 390 × 844, light. App bar, module head, facet tabs + `On this facet` control, inset blocked card, claim stack. | Identical states, mobile arrangement: status chip above title, two-row footer. |
| `4SX-0` | 02 · Reading view — default · MOBILE DARK | The same, dark. | Identical states. |
| `4EY-0` | Group 02 band (root child, name `Frame`, 5560 × 27.5) | The group's title band: eyebrow `02 · READING VIEW — DEFAULT` + caption. | n/a — annotation. |
| `4VD-0` | Group 02 notes strip (root child, name `Frame`, 5560 × 61, worldY 14200) | Three note columns: `WHAT CHANGES IN DARK`, `WHAT CHANGES ON MOBILE`, `HOW MOBILE NAVIGATES`. | n/a — annotation. **The strip is not named `Group 02 — notes`; it is an unnamed `Frame`.** Identified by position (directly under the four 02 boards, above `Group 03 — band` at worldY 14900) and by content. |
| `4Y3-0` | 02 · Mobile nav — modules (light) | The modules rail as a **left drawer** over a scrim. | Drawer open; `Permission readiness` active with padlock; `TRACKS 25` collapsed; theme toggle in the drawer footer. |
| `50U-0` | 02 · Mobile nav — on this facet (light) | The facet index as a **bottom sheet**. | Sheet open at its 660px ceiling; 13 rows; first row active; twelve rows carry blocked-coloured counts. |
| `52B-0` | 02 · Mobile nav — modules (dark) | Dark twin of `4Y3-0`. | Same. |
| `552-0` | 02 · Mobile nav — on this facet (dark) | Dark twin of `50U-0`. | Same, plus the dark-only 1px top hairline on the sheet. |

Captions `4F5-0` / `4FA-0` / `4FF-0` / `4FK-0` and `56H-0` / `56K-0` / `56N-0` /
`56Q-0` label the boards; they carry no product UI.

---

## 2. Design intent

### What the reviewer should perceive

This is the screen a reviewer lives on. Its whole job is to make **one claim's
prose the largest, quietest, most central thing on the page**, and to put every
machine fact about that claim within one glance but below it in the hierarchy.
The measure is fixed (`6O-0` `max-width: 760px`) and the two rails are given
fixed widths that do not compete with it: the modules rail is 268px (`3C-0`) and
the facet panel 244px (`8P-0`), so at 1440 the reading column owns 928px of
which only 760 carries text. The rails are painted on the page ground
(`--color-paper`) and the claim column is the only raised surface
(`--color-card`) — the card is literally the figure and the rails are literally
the ground.

Serif marks what a human wrote and must judge; sans marks what the interface
says about it; mono marks what the machine emitted. On `6W-0` the claim prose is
`var(--font-serif)` 17/28 and the claim id directly under the title is
`var(--font-mono)` 12/16 — same claim, two families, for exactly that reason
(`tokens.md` § 7).

The screen's second job is to make **blockedness unmissable without making it
loud**. Every claim on this board is LOCKED *and* blocked, which is the
combination a reviewer finds hardest: the status chip says approved, the footer
says it cannot ship. The board resolves it by weight, not by shouting — the
LOCKED chip is a 10 %-tinted pill in `--color-locked`, the `Blocked` word is
13/16 weight 600 in `--color-blocked` preceded by a 7px dot, and the facet-wide
statement lives once, at the top of the card stack, in a 10 %-tinted band.

### Rules that bind this screen

- **R09.1 / R-F.1 — one line at rest, four expansions.** The footer strip on
  `6W-0` is one row: readiness (`Blocked 2 blockers`), relationships
  (`4 relationships`), sources (`2 sources`), checks (`1 check`), then the
  comment count hard right. No expansion is open on any 02 board. Each entry is a
  noun and a count; none is a score.
- **R09.2 — exactly one expansion open at a time.** Not exercised on 02 (all
  closed), but the strip must be built so it can only ever have one open.
- **R09.3 — auto-open is reserved for readiness, and only when blocked.** Every
  claim here *is* blocked, and readiness is nevertheless **closed** on all four
  boards. The rule says readiness *may* auto-open, not must; 02 is the evidence
  that "blocked" alone is not a trigger at facet scale, because a facet where
  every claim is blocked would auto-open 26 panels and destroy the reading view.
  **Decision recorded below.**
- **R09.6 — Issues is a screen and the banner is the only way in.** The banner
  (`6P-0` desktop, `56T-0` mobile) carries the only `Show issues` control on the
  screen; nothing else links to 04.
- **R09.7 — the facet marker is the only project-level health signal in the
  reading view.** On 02 that signal is carried by the *count* colour in
  `ON THIS FACET`, not by a dot (see Paper defects).
- **R10.1 / R10.2 / R10.3 / R10.4 / R10.5 — freshness.** `1F4-0` reads
  `Updated 23 hours ago`: elapsed, one unit, no timestamp. It is the **Fresh**
  band (< 24 h), and Fresh takes no colour — measured label `#54606F`
  (`--color-muted`) weight 500, clock icon stroke `#6E7C8E` (`--color-faint`),
  exactly as R10.2 requires.
- **R11.1 / R11.4 — focus is one control.** `13D-0` (inside wrapper `13B-0`) is
  the only focus affordance: a 28px outlined button reading `Focus` with a mono
  `F` hint. There is no per-rail show/hide control anywhere on the four boards,
  and no banner.
- **R-F.2 / R-F.3 / R-F.4 — footer chip vocabulary and the mobile two-row
  form.** The mobile footer (`5L0-0`, `5LR-0`) is the two-row form: blocked chip
  + comment count on row one, three neutral chips on row two.
- **R-H.0 / R-H.1 / R-H.2 — only three components change shape on mobile.** All
  three appear on 02: blocked notice → inset card; claim header → chip above
  title; footer strip → two rows. Nothing else changes shape.
- **R-J.1 … R-J.5 — bottom-sheet policy.** The facet index (`50U-0`, `552-0`) is
  a **body in the one shell**. Measured against R-J.2 it is correct on every
  point: 16px top corners, 36 × 4 grabber in `--color-border-strong` under 8px of
  top padding, header hairline, dark-only 1px top hairline.
- **R00.0 — the components board is the source of truth.** Where 02 disagrees
  with it (the mobile comment control), 02 is wrong.

### The notes strip (`4VD-0`), paraphrased with intent

1. **WHAT CHANGES IN DARK** — "Nothing but the palette. Same widths, same type
   scale, same order. Status colours lighten so they stay legible on a dark
   ground — locked, draft and blocked keep their hue, not their value."
   *Intent:* dark is a re-pointing of tokens, never a re-layout. A lane agent who
   finds itself writing a second layout rule under a dark query has misread this.
   **Verified against the boards and true for every measured value except the
   claim prose family — see Paper defects.**
2. **WHAT CHANGES ON MOBILE** — "The footer strip splits: status and the comment
   count stay on one line, and the three detail chips become a divided bar so
   nothing wraps raggedly. Wording and order are unchanged."
   *Intent:* the split is a wrapping-avoidance measure, not a re-prioritisation;
   nothing is demoted and nothing is renamed. Matches R-F.4.
3. **HOW MOBILE NAVIGATES** — "Both rails become overlays. Modules open as a left
   drawer from the menu; the facet index opens as a bottom sheet from the control
   beside the facet tabs. Same content, same counts, same order — reached rather
   than always present."
   *Intent:* the rails are not deleted on a phone, they are deferred. Two
   different overlay shapes are used deliberately — a list you *browse* (modules)
   comes from the edge it lives on; a list you *jump within* (this facet) comes
   from the thumb.

### The band (`4EY-0`)

Eyebrow `02 · READING VIEW — DEFAULT` (Inter 13/16 weight 600, tracking
`0.08em`, `--color-accent`) and caption `four designs of one screen · light pair
left, dark pair right` (Inter 12/16, `--color-faint`), over a 2px
`--color-accent` rule. Annotation only; nothing here is product UI.

### Board contradictions (recorded in full under § 9)

The notes strip says dark changes "nothing but the palette", yet the dark desktop
board drops Source Serif 4 from the claim prose. The components board says the
mobile comment count takes the accent, yet both mobile 02 boards paint it faint.
Both are Paper defects, not values to copy.

---

## 3. Layout

### Desktop, 1440 (board `3B-0`)

| Value | Measured | Node |
|---|---|---|
| Page frame | `1440 × 1024`, `display: flex`, `overflow: clip`, ground `#EFF1F4` (`--color-paper`) | `3B-0` |
| Modules rail width | `268px`, `flex-shrink: 0`, full `1024px` height | `3C-0` |
| Modules rail edge | `border-right: 1px solid #DDE2E9` (`--color-border`) | `3C-0` |
| Claim column | `flex: 1 1 0`, `min-width: 0`, `1024px` tall, ground `--color-paper` | `60-0` |
| Facet panel width | `244px`, `flex-shrink: 0`, `1024px` tall, ground `--color-paper` | `8P-0` |
| Claim-column head block | `padding: 36px 48px 0`, `gap: 20px`, column flex | `61-0` |
| Head row width | `760px` | `62-0` |
| Facet tab strip width | `760px`, `gap: 26px`, `border-bottom: 1px solid #DDE2E9` | `6C-0` |
| Scroll region | `flex: 1 1 0`, `min-height: 0`, `justify-content: center`, `padding: 24px 48px 0`, `overflow: clip` | `6N-0` |
| Card stack | `width: 100%`, `max-width: 760px`, radius `12px 12px 0 0`, `overflow: clip`, ground `#FFFFFF`, `border: 1px solid #DDE2E9` | `6O-0` |
| Sidebar header | `padding: 24px 20px 20px`, `gap: 3px` | `3D-0` |
| Sidebar search block | `padding: 0 20px 16px` | `3H-0` |
| Sidebar scroll region | `flex: 1 1 0`, `min-height: 0`, `padding-inline: 12px`, `gap: 4px`, `overflow: clip` | `3O-0` |
| Module row list gap | `1px` (hairline-divided rows) | `3X-0` |
| Sidebar footer | `border-top: 1px solid #DDE2E9`, `padding: 16px 20px 20px`, `gap: 14px` | `5A-0` |
| Facet panel padding | `36px` top, `4px` left, `20px` right; `gap: 3px` | `8P-0` |
| Facet list gap | `2px` | `1B3-0` |
| Freshness block | `margin-top: auto`, `border-top: 1px solid #DDE2E9`, `padding: 14px 8px 20px`, `gap: 5px` | `1EZ-0` |

**Sticky / scroll regions.** Two independent scroll regions plus one fixed
footer each side: the sidebar's `3O-0` (`overflow: clip`, `flex: 1`) scrolls the
module list while `3D-0`/`3H-0` above and `5A-0` below stay put; the claim
column's `6N-0` scrolls the card stack while `61-0` (head + tabs) stays put; the
facet panel's list scrolls while `1EZ-0` is pinned by `margin-top: auto`. The
card stack's `12px` radius is **top-only** (`border-bottom-*-radius: 0px` on
`6O-0`) — the stack runs off the bottom of the viewport by design, so the page
reads as continuing rather than ending.

1440 = 268 (`3C-0`) + 928 (`60-0`) + 244 (`8P-0`), exactly.

### Mobile, 390 (board `4O0-0`)

| Value | Measured | Node |
|---|---|---|
| Frame | `390 × 844`, column flex, ground `--color-paper` | `4O0-0` |
| Head block | column flex, `border-bottom: 1px solid var(--color-border)`, ground `--color-paper` | `4QI-0` |
| App bar | `height: 52px`, `padding-inline: 16px`, `gap: 12px` | `4QJ-0` |
| Module head | `padding: 14px 16px`, `gap: 5px`, `width: 390px` | `4QT-0` |
| Lock metric row | `padding-top: 6px`, `gap: 8px` | `4QW-0` |
| Facet tab strip | `padding-inline: 16px`, `gap: 22px` | `4R0-0` |
| Tab/control spacer | `flex: 1 1 0`, `min-width: 8px` | `4XP-0` |
| Scroll region | `flex: 1 1 0`, `min-height: 0`, `overflow: clip`, ground `--color-card` | `4R8-0` |
| Blocked card inset | `margin: 14px 16px 0` | `56T-0` |
| First claim | `padding: 22px 16px 18px`, `gap: 11px` | `4RH-0` |
| Later claims | `padding: 20px 16px 18px`, `gap: 11px`, `border-top: 1px solid var(--color-border)` | `4SL-0` |

**The gutter is 16px and the card has no side margin on mobile.** The whole
scroll region `4R8-0` takes `--color-card`, so the card becomes the viewport:
no 12px radius, no `--color-paper` visible beside it. Content width is
390 − 32 = **358px**, which is the figure R-H.3 measures its no-wrap coverage
strip against.

Mobile dark (`4SX-0`) is identical geometry — `4SY-0`, `4TN-0`, `4TW-0`
(`22/16/18`, gap `11px`), `4V0-0` (`20/16/18`) — with `--color-dark-*` in place
of the light tokens.

---

## 4. Components on this screen

**Count recorded for the desktop light board: 127 measured property rows** — 18
in § 3's desktop layout table and 109 across §§ 4.1–4.17 — every one of them
read from `get_computed_styles` or `get_jsx` on a named desktop node, and most
carrying more than one value (a `padding / gap` row is two). Dark, mobile,
drawer and sheet values are recorded alongside them and are **not** counted
toward that figure.

Colour convention below: Paper light token → Paper dark token, with the hex the
board actually painted beside it. Where a board painted a raw hex that equals a
token, the token is named and the raw hex is given as evidence — implement the
token (`tokens.md` Disagreement 12).

**Node convention.** The Node column names the node that *carries* the measured
property, not the nearest component-shaped ancestor. Where a component is a
wrapper plus a child — a divider beside a button, an icon slot beside a label —
the wrapper's own properties (its outer margin, the gap it puts *between* its
children) get their own row and the child's properties are addressed to the
child. A section heading may still name the wrapper, because that is what the
component is called; the rows underneath it are addressed node by node. Two rows
in the same section may therefore report two different `gap` values without
contradiction: they belong to two different nodes. § 4.8 is the clearest case.

### 4.1 Sidebar — product identity (`3D-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Title | Inter 19/24, weight 600, tracking `-0.01em` | same | `3E-0` / `4FR-0` |
| Title colour | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `3E-0` / `4FR-0` |
| Eyebrow | Inter 12/16, weight 400 | same | `3F-0` |
| Eyebrow colour | `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `3F-0` |
| Block | `padding: 24px 20px 20px`, `gap: 3px` | same | `3D-0` |

### 4.2 Sidebar — search field (`3I-0`)

| Property | Value | Node |
|---|---|---|
| Height | `34px` | `3I-0` |
| Ground / border / radius | `--color-card` `#FFFFFF` / `--color-border` `#DDE2E9` 1px / `8px` (`--radius-md`) | `3I-0` |
| Padding / gap | `padding-inline: 10px`, `gap: 8px` | `3I-0` |
| Icon | 15px, 2px stroke, **`#8592A3`** | `3J-0` |
| Placeholder | Inter 14/18, `--color-faint` `#6E7C8E` | `3M-0` |

`#8592A3` is not a token; it is the pre-token faint residue named in
`tokens.md` Disagreement 4. Ship `--color-faint`.

### 4.3 Sidebar — section header, `MODULES` / `TRACKS` (`3P-0`, `4X-0`)

| Property | Value | Node |
|---|---|---|
| Label | Inter 11/14, weight 600, tracking **`0.09em`**, `--color-faint` | `3Q-0` |
| Count | IBM Plex Mono 11/14, weight 400, `--color-faint` | `3R-0` |
| Padding | `10px 8px 6px`, `gap: 8px` | `3P-0` |
| Disclosure glyph | 12px, 2.6px stroke, `--color-faint`; chevron **down** = expanded (`MODULES`), chevron **right** = collapsed (`TRACKS`) | `10U-0` / `10X-0` |
| `TRACKS` block padding-top | `18px` (not 10) — a deliberate extra separation from the module list | `4X-0` |

Dark twin: label and count `--color-dark-faint`, glyph `--color-dark-faint`
(`4G1-0`).

### 4.4 Sidebar — module row (`3T-0` rest, `4A-0` active)

| State | Property | Light | Dark | Node |
|---|---|---|---|---|
| Rest | Height / padding / radius | `32px` / `padding-inline: 10px` / `6px` | same | `3T-0` |
| Rest | Label | Inter 14/18 weight 400, `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `3U-0` |
| Rest | Count | IBM Plex Mono 11/14, `--color-faint`, `width: 16px`, right-ranged | `--color-dark-faint` | `3V-0` |
| **Active** | Ground | `#1C4E8C17` = `--color-accent-bg` (accent @ 9 %) | `#6AA6E824` = `--color-dark-accent-bg` (14 %) | `4A-0` / `4MB-0` pattern |
| **Active** | Label | Inter 14/18 **weight 600**, `--color-accent` `#1C4E8C` | `--color-dark-accent` `#6AA6E8` | `4B-0` |
| **Active** | Count | mono 11/14, `--color-accent` | `--color-dark-accent` | `4F-0` |
| **Active** | Gap | `6px` (opens room for the padlock) | same | `4A-0` |
| Both | All-locked marker | 13px padlock, 2.2px stroke, `--color-locked` `#2C6B52` | `--color-dark-locked` `#63BE9A` | `4C-0` |

The padlock appears on `Permission readiness` (active) and `Emission boundary`
(at rest, `21`) — so the marker is a property of the module, not of selection.
Rows are separated by a `1px` flex gap (`3X-0`), which is what makes a bordered
stack read as hairline-divided rows without drawing hairlines.

### 4.5 Sidebar — footer nav rows (`5C-0`, `5I-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Row height / gap | `30px` / `9px` | same | `5C-0` |
| Icon | 15px, 2px stroke, `--color-muted` | `--color-dark-muted` | `5D-0` / `5J-0` |
| Label | Inter 14/18 weight 400, `--color-muted` | `--color-dark-muted` | `5H-0`, `5N-0` |
| Words, in order | `Claims graph`, then `Build order` | same | `5H-0`, `5N-0` |
| List gap | `2px` | same | `5B-0` |

### 4.6 Sidebar — theme segmented control (`5O-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Track ground | **`#E3E7ED`** (no token) | **`#080B10`** (no token) | `5O-0` / `4ID-0` |
| Track radius / padding | `8px` / `3px` | same | `5O-0` |
| Segment height / radius / gap | `28px` / `6px` / `6px` | same | `5P-0` |
| Selected segment ground | `--color-card` `#FFFFFF` | `--color-dark-border` `#242C38` | `5P-0` / dark `Dark` segment |
| Selected label | Inter 13/16 **weight 500**, `--color-ink` | `--color-dark-ink` | `5T-0` |
| Unselected label | Inter 13/16 weight 400, `--color-muted` | `--color-dark-muted` | `5X-0` |
| Icons | 13px, 2px stroke; selected takes ink, unselected muted | same | `5Q-0` / `5V-0` |
| Words, in order | `Light`, `Dark` | same | `5T-0`, `5X-0` |

Both track grounds are untokened literals. Decision recorded in § 9.

### 4.7 Module head (`62-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Eyebrow | IBM Plex Mono 11/14, tracking `0.08em`, `--color-faint`; text `MODULE 06 / 26` | `--color-dark-faint` | `64-0` |
| Title | Inter **28/34**, weight 600, tracking `-0.02em`, `--color-ink` | `--color-dark-ink` | `65-0` |
| Title/eyebrow gap | `6px` | same | `63-0` |
| Row alignment | `align-items: end`, `justify-content: space-between`, width `760px` | same | `62-0` |
| Metric numeral | Inter 13/16 **weight 600**, `--color-ink`; text `31` | `--color-dark-ink` | `67-0` |
| Metric phrase | Inter 13/16 weight 400, `--color-muted`; text `of 31 locked` | `--color-dark-muted` | `68-0` |
| Progress track | `80 × 6`, radius `999px` (`--radius-pill`), `--color-border` | `--color-dark-border` | `69-0` |
| Progress fill | `80 × 6`, `--color-locked` `#2C6B52` | `--color-dark-locked` `#63BE9A` | inside `69-0` |
| Metric gap | `8px` | same | `66-0` |

28/34 is **off the type scale** (`--text-h1` is 28/36). Recorded, not widened.

### 4.8 Focus control (`13B-0` wrapper, `13D-0` button)

The intent is that focus is *offered*, not advertised: a quiet outlined button
sitting after a hairline divider at the end of the header's right cluster, low
enough in contrast that it reads as a tool rather than a call to action, but
given a key hint so that the keyboard user never needs to find it twice.

`13B-0` is not the button. It is a two-child flex wrapper — divider `13C-0`
plus button `13D-0` — and it carries **only** the separation between those two
(`gap: 10px`) and its own offset from the control before it (`margin-left: 6px`).
All of the button's own geometry, ground, border and internal `gap: 7px` live on
`13D-0`. The two gaps are different numbers because they do different work: 10px
is the gap *around* the divider, 7px is the rhythm *inside* the button.

| Property | Light | Dark | Node |
|---|---|---|---|
| Wrapper | `display: flex`, `align-items: center`, `margin-left: 6px`, `gap: 10px`; no ground, no border, no radius of its own | same (`4IZ-0`) | `13B-0` |
| Divider before it | `1 × 16`, `flex-shrink: 0`, `--color-border` | `--color-dark-border` (`4J0-0`) | `13C-0`, in `13B-0` |
| Height / radius / padding | `28px` / **`7px`** / `padding-inline: 11px` | same | `13D-0` |
| Ground / border | `--color-card` / 1px `--color-border` | `--color-dark-card` / `--color-dark-border` (`4J1-0`) | `13D-0` |
| Icon | 13 × 13 SVG on a 24-unit viewBox, `stroke-width: 2`, round caps and joins, `--color-muted`; `flex-shrink: 0` | `--color-dark-muted` (`4J2-0`) | `13E-0` (path `13F-0`), in `13D-0` |
| Label | Inter 13/16 **weight 500**, `--color-muted`; text `Focus` | `--color-dark-muted` (`4J4-0`) | `13G-0`, in `13D-0` |
| Key hint | IBM Plex Mono 11/14 weight 400, `--color-faint`; text `F` | `--color-dark-faint` (`4J5-0`) | `13H-0`, in `13D-0` |
| Gap | `7px`, between icon, label and key hint | same | `13D-0` |

The dark twin is structurally identical — wrapper `4IZ-0`, divider `4J0-0`,
button `4J1-0`, icon `4J2-0`, label `4J4-0`, hint `4J5-0` — and every dark value
is a `--color-dark-*` token reference, not a re-typed hex. That is the correct
form and is worth naming, because `tokens.md` Disagreement 12 records boards
elsewhere in this file that did the opposite. Note the inversion against the
light board: `3B-0` spells the colours as literals and the families as
`var(--font-*)`, while `4FO-0` spells the colours as tokens and the families as
the literal `"Inter", system-ui, sans-serif` (§ 9.10).

**Only the rest state is on any 02 board.** Its active state is board 03's; per
R11.4 the active state is the mode's only reminder and its only exit.

`7px` is an off-scale radius (the scale has 4 / 8 / 999). Recorded per
`tokens.md` § Open decisions.

### 4.9 Facet tab strip (`6C-0`)

| State | Property | Light | Dark | Node |
|---|---|---|---|---|
| Strip | `gap: 26px`, `border-bottom: 1px solid --color-border`, width `760px` | | | `6C-0` |
| **Selected** | Underline | `2px solid --color-accent`, `margin-bottom: -1px` (sits on the strip rule) | `--color-dark-accent` | `6D-0` |
| **Selected** | Label | Inter 14/18 **weight 600**, `--color-accent`; text `Contract` | `--color-dark-accent` | `6E-0` |
| **Selected** | Count | mono 11/14, `--color-accent`; text `26` | `--color-dark-accent` | `6F-0` |
| Rest | Label | Inter 14/18 weight 400, `--color-muted`; text `Internals` | `--color-dark-muted` | `6H-0` |
| Rest | Count | mono 11/14, `--color-faint`; text `5` | `--color-dark-faint` | `6I-0` |
| Both | `padding-bottom: 11px`, `gap: 7px` | | | `6D-0` / `6G-0` |

### 4.10 Blocked banner — desktop (`6P-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Ground | `#9E3B360F` — `--color-blocked` @ ~6 % | `#EC8A831A` — `--color-dark-blocked` @ ~10 % | `6P-0` / `4JG-0` |
| Bottom edge | `1px solid --color-border` | `--color-dark-border` | `6P-0` |
| Padding / gap | `13px` block, `32px` inline / `12px` | same | `6P-0` |
| Icon | 15px warning triangle, 2px stroke, `--color-blocked` | `--color-dark-blocked` | `6Q-0` |
| Body | Inter 14/18 weight 400, `--color-ink` | `--color-dark-ink` | `6T-0` |
| Action | Inter 13/16 **weight 500**, `--color-blocked` | `--color-dark-blocked` | `6U-0` |
| Radius | none — it is the top slab of the card, clipped by `6O-0`'s `12px` | same | `6P-0` |

It is a **full-bleed band inside the card**, flush to both card edges, above the
first claim. There is no separate banner card.

### 4.11 Claim card — head (`6W-0` head row)

| Property | Light | Dark | Node |
|---|---|---|---|
| Card padding | first claim `34px` top / `28px` bottom / `32px` inline, `gap: 16px` | same | `6W-0` / `4JN-0` |
| Later claims | `30px` top / `28px` bottom / `32px` inline, `border-top: 1px solid --color-border` | `--color-dark-border` | `7W-0`, `34F-0` |
| Head row | `align-items: start`, `gap: 16px` | same | inside `6W-0` |
| Title | Inter **20/26**, weight 600, tracking `-0.01em`, `--color-ink` | `--color-dark-ink` | inside `6W-0` |
| Id | IBM Plex Mono **12/16**, weight 400, `--color-faint` | `--color-dark-faint` | inside `6W-0` |
| Title→id gap | `7px` | same | inside `6W-0` |

Order is **title, then id, then status chip right-ranged** on desktop.

### 4.12 Status chip — LOCKED (desktop, inside `6W-0`)

| Property | Light | Dark |
|---|---|---|
| Ground | `#2C6B521A` — `--color-locked-bg` (10 %) | `#63BE9A21` — `--color-dark-locked-bg` (13 %) |
| Radius | `999px` (`--radius-pill`) | same |
| Padding | `4px` block, `8px` left, `10px` right | same |
| Gap | `5px` | same |
| Padlock | 12px, 2.4px stroke, `--color-locked` | `--color-dark-locked` |
| Label | Inter 11/14, weight 600, tracking **`+0.05em`**, `--color-locked`; text `LOCKED` | `--color-dark-locked` |

Nodes: light chip inside `6W-0`; dark chip inside `4JN-0`. This is the same chip
the mobile boards use (`4RI-0`) at one step smaller padding — see § 4.18.

### 4.13 Claim prose (inside `6W-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Family | `var(--font-serif)` = Source Serif 4 | **`system-ui, sans-serif`** — defect | `6W-0` / `4JN-0` |
| Size / leading / weight | 17 / 28 / 400 | 17 / 28 / 400 | |
| Colour | `--color-ink` | `--color-dark-ink` | |
| Paragraph gap | `16px` | `16px` | |
| Truncation control | Inter 14/**28** weight 500, `--color-accent`; text `…more`, baseline-aligned, `gap: 7px` | `--color-dark-accent` | |

The `…more` control sits **inline, on the last line's baseline**, not on its own
row — it reads as a continuation of the sentence, which is what keeps the clamp
from looking like a button.

### 4.14 Claim footer strip — desktop (`6W-0` footer row / `33P-0`)

**The desktop footer is a plain inline control row, not pills.** Only the mobile
form takes pill grounds (R-H.0: the footer strip is one of the three components
that change shape below the phone breakpoint; § 4.19).

| Property | Light | Dark | Node |
|---|---|---|---|
| Top rule | `1px solid` **`#E8ECF1`** (no token) | **`#212934`** (no token) | `6W-0` / `4JN-0` |
| Spacing | `margin-top: 8px`, `padding-top: 16px`, `gap: 20px` | same | `33P-0` |
| Readiness dot | `7 × 7`, radius `999px`, `--color-blocked` | `--color-dark-blocked` | `33P-0` |
| Readiness word | Inter 13/16 **weight 600**, `--color-blocked`; text `Blocked` | `--color-dark-blocked` | `33P-0` |
| Readiness count | Inter 13/16 weight 400, `--color-muted`; text `2 blockers` | `--color-dark-muted` | `33P-0` |
| Readiness group gap | `6px` | same | `33P-0` |
| Divider after readiness | `1 × 12`, `--color-border` | `--color-dark-border` | `33P-0` |
| Detail entries | Inter 13/16 weight 400, `--color-muted`, `gap: 5px` | `--color-dark-muted` | `33P-0` |
| Chevrons | 12px, 2.4px stroke, `--color-faint` | `--color-dark-faint` | `33P-0` |
| Zero-state entry | Inter 13/16, **`--color-faint`** (one step quieter than a counted entry); text `No checks declared` | `--color-dark-faint` | `33P-0` |
| Comment control | 14px speech-bubble, 2px stroke, `--color-faint`; count Inter 13/16, `--color-faint`, `gap: 6px`, hard right via a `flex: 1` spacer | `--color-dark-faint` | `33P-0` |

**Only the readiness entry carries a dot and a coloured word; the other three are
plain.** That is how "one of these four is a status and three are counts" is
said without four colours.

### 4.15 Facet panel header (`8Q-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Label | Inter 11/14, weight 600, tracking `0.09em`, `--color-faint`; text `ON THIS FACET` | `--color-dark-faint` | `8R-0` / `4M9-0` |
| Padding | `0 8px 10px` | same | `8Q-0` |

### 4.16 Facet panel row (`1B4-0` active, `1B8-0` rest)

| State | Property | Light | Dark | Node |
|---|---|---|---|---|
| **Active** | Ground / radius | `#1C4E8C17` = `--color-accent-bg` / `6px` | `#6AA6E824` = `--color-dark-accent-bg` / `6px` | `1B4-0` / `4MB-0` |
| **Active** | Title | Inter 13/19 **weight 500**, `--color-accent` | `--color-dark-accent` | `1B5-0` / inside `4MB-0` |
| **Active** | Count | mono 12/16 **weight 500**, `--color-accent` | `--color-dark-accent` | `1B7-0` |
| Rest | Title | Inter 13/19 weight 400, `--color-muted` | `--color-dark-muted` | `1B9-0` |
| Rest (blocked) | Count | mono 12/16 weight 400, **`--color-blocked`** | `--color-dark-blocked` | `1BB-0` and eleven siblings |
| Both | Padding / gap / count slot | `7px` block, `8px` inline / `8px` / `width: 22px`, right-ranged, `padding-top: 1px` | same | `1B4-0` |
| Both | List gap | `2px` | same | `1B3-0` |

Twelve of the thirteen rows paint the count `--color-blocked`; the active row
paints it `--color-accent` even though that claim is blocked too. See § 9.

### 4.17 Freshness footer (`1EZ-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Block | `margin-top: auto`, `border-top: 1px solid --color-border`, `padding: 14px 8px 20px`, `gap: 5px` | `--color-dark-border` | `1EZ-0` / `4NS-0` |
| Clock icon | 12px, 2px stroke, `--color-faint` | `--color-dark-faint` | `1F1-0` |
| Phrase | Inter 12/16 **weight 500**, `--color-muted`; text `Updated 23 hours ago` | `--color-dark-muted` | `1F4-0` |
| Icon/phrase gap | `7px` | same | `1F0-0` |
| Caption | Inter 11/15 weight 400, `--color-faint`; text `Claims changed since then are not in this view` | `--color-dark-faint` | `1F5-0` |

This is the **Fresh** presentation of R10.2 and it is the only one on any 02
board. Ageing is identical; Stale re-paints the phrase and the icon
`--color-draft` at weight 600.

### 4.18 Mobile — app bar, module head, tabs (`4QI-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Bar height / padding / gap | `52px` / `padding-inline: 16px` / `12px` | same | `4QJ-0` / `4SZ-0` |
| Bar ground / edge | `--color-paper` / bottom `1px --color-border` | `--color-dark-paper` / `--color-dark-border` | `4QI-0` / `4SY-0` |
| Menu glyph | 20px, 2px stroke, `--color-muted` | `--color-dark-muted` | `4QK-0` |
| Project label | Inter 14/18 **weight 500**, `--color-muted`; text `Curtainly` | `--color-dark-muted` | `4QM-0` |
| Search / theme glyphs | 19px, 2px stroke, `--color-muted` | `--color-dark-muted` | `4QN-0`, `4QQ-0` |
| Module eyebrow | mono 11/14, tracking `0.08em`, `--color-faint` | `--color-dark-faint` | `4QU-0` |
| Module title | Inter **24/30**, weight 600, tracking `-0.02em`, `--color-ink` | `--color-dark-ink` | `4QV-0` |
| Metric | `31` Inter 13/16 w600 ink + `of 31 locked` 13/16 w400 muted | dark twins | `4QX-0`, `4QY-0` |
| Progress bar | `72 × 6`, radius `999px`, `--color-locked` (no track drawn at 100 %) | `--color-dark-locked` | `4QZ-0` |
| Tab strip | `padding-inline: 16px`, `gap: 22px`, `padding-bottom: 10px` | same | `4R0-0` |
| Facet-sheet trigger | `26px` tall, radius **`7px`**, `padding-inline: 9px`, `gap: 6px`, `margin-bottom: 6px`, ground `--color-card`, 1px `--color-border`; 12px list glyph + Inter 12/16 w500 `--color-muted`, text `On this facet` | `--color-dark-card` / `--color-dark-border` / `--color-dark-muted` | `4XR-0` / `4XY-0` |

Desktop title 28/34 → mobile 24/30; claim title 20/26 → 18/24; prose 17/28 →
16/26. The scale steps down; the vocabulary does not.

### 4.19 Mobile — blocked notice, inset card (`56T-0` / `574-0`)

| Property | Light | Dark |
|---|---|---|
| Inset | `margin: 14px 16px 0` | same |
| Ground | `#9E3B360F` (blocked @ ~6 %) | `#EC8A8317` (dark blocked @ ~9 %) |
| Border / radius | 1px `#9E3B3633` (blocked @ 20 %) / **`10px`** | 1px `#EC8A8342` (26 %) / `10px` |
| Body block | `padding: 12px 14px`, `gap: 10px` | same |
| Icon | 15px triangle, 2px stroke, `--color-blocked`, `margin-top: 2px` | `--color-dark-blocked` |
| Body text | Inter **13/19**, weight 400, `--color-ink` | `--color-dark-ink` |
| Action row | `height: 38px`, `padding-inline: 14px`, top `1px #9E3B362E` (18 %) | top `1px #EC8A8338` (22 %) |
| Action label | Inter 13/16 **weight 600**, `--color-blocked`; text `Show issues` | `--color-dark-blocked` |
| Trailing chevron | 13px, 2.6px stroke, `--color-blocked` | `--color-dark-blocked` |

Exactly R-H.1, including the divided action row that makes the tap target the
card's full width.

### 4.20 Mobile — claim header (`4RI-0` / `4TX-0`)

| Property | Light | Dark |
|---|---|---|
| Order | status chip, then title, then id — chip **above** the title | same |
| Chip ground / radius / padding / gap | `#2C6B521A` (`--color-locked-bg`) / `999px` / `3px` block, `7px` left, `9px` right / `5px` | `--color-dark-locked-bg` |
| Padlock | 11px, 2.4px stroke, `--color-locked` | `--color-dark-locked` |
| Chip label | Inter 11/14, w600, tracking `+0.05em`, `--color-locked`; `LOCKED` | `--color-dark-locked` |
| Title | Inter **18/24**, w600, tracking `-0.01em`, `--color-ink` | `--color-dark-ink` |
| Id | IBM Plex Mono **11/15**, w400, `--color-faint` | `--color-dark-faint` |
| Gaps | chip→title `9px`, title→id `6px` | same |

Exactly R-H.2.

### 4.21 Mobile — footer strip, two rows (`5L0-0` / `5LR-0`)

| Property | Light | Dark |
|---|---|---|
| Top rule / spacing | `1px #E8ECF1` / `margin-top: 6px`, `padding-top: 13px`, row gap `8px` | `1px #212934` |
| **Blocked chip** ground | `#9E3B3617` (blocked @ ~9 %) | `#EC8A8321` (13 %) |
| Blocked chip geometry | `height: 30px`, radius `999px`, `padding-inline: 11px`, `gap: 7px` | same |
| Blocked chip dot | `7 × 7`, `--color-blocked` | `--color-dark-blocked` |
| Blocked chip label / count | Inter **12/16** w600 / 12/16 w400, both `--color-blocked` | `--color-dark-blocked` |
| Blocked chip chevron | 11px, 2.6px stroke, `--color-blocked` | `--color-dark-blocked` |
| Row-one right | comment control: 14px bubble + Inter 13/16 count, both **`--color-faint`**, `gap: 6px` | `--color-dark-faint` |
| Row-one spacer | `flex: 1`, `min-width: 4px` | same |
| **Closed detail chip** | `height: 30px`, radius `999px`, `padding-inline: 11px`, `gap: 6px`, ground `--color-card`, 1px `--color-border` | `--color-dark-card` / `--color-dark-border` |
| Closed chip label | Inter 12/16 **weight 500**, `--color-muted` | `--color-dark-muted` |
| Closed chip chevron | 11px, 2.6px stroke, `--color-faint` | `--color-dark-faint` |
| Row-two gap | `8px` | same |
| Order | row one: `Blocked 2 blockers` … comment count. row two: `4 relationships`, `2 sources`, `1 check` | same |

Matches R-F.2 (closed variant) and R-F.4 exactly. **The comment control does
not** — see § 9.

Read the table the way R-I.3 asks: the chip rows (geometry, ground, label,
chevron) are the *component*, and the top rule, row gap, spacer and order rows
are *composition*. A lane agent changing the two-row arrangement is not touching
the chip, and restyling the chip does not license moving the rows.

The **open** and **blocked-chip** variants of R-F.2 are not on any 02 board;
their values are the components board's (section I3, `7N2-0` → `7NA-0`) and a
lane agent takes them from there, not from here.

---

## 5. Mobile rules

Every rule below is expressed in an **existing engine query**. No 390px
breakpoint is added (`tokens.md` § 8).

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | Both rails leave the flow: the modules rail becomes a left drawer, the facet panel becomes a bottom sheet. Neither is deleted. | `@media (max-width: 860px)` | Arrangement, and 860 is where the engine already collapses rails (`style.css:1160, 2932, 4424, 4533`). |
| M2 | A 52px app bar appears carrying the drawer trigger, project label, search and theme. | `@media (max-width: 860px)` | It is the drawer's affordance; it exists wherever the drawer does. |
| M3 | The card loses its side margins and its radius: the scroll region takes `--color-card` edge-to-edge, gutter 16px, content 358px. | `@media (max-width: 520px)` | Phone tier; a tablet at 700px still wants a card. |
| M4 | Blocked notice becomes an inset card (16px side margins, 10px radius, 1px tinted-red border) with a divided 38px action row. | `@media (max-width: 520px)` | R-H.1; phone tier. |
| M5 | Status chip moves above the claim title; order status → title → id. | `@media (max-width: 520px)` | R-H.2; phone tier. |
| M6 | Footer strip becomes two rows: status + comment count, then the three detail chips as pills. Written to win over the existing `max-width: 640px` (`style.css:2563`) and `max-width: 560px` (`style.css:4647`) rules on `.claim-footer__counts`. | `@media (max-width: 520px)` | R-F.4; phone tier. **The 560px rule is real and already owns this selector.** |
| M7 | Type steps down: module title 28/34 → 24/30, claim title 20/26 → 18/24, prose 17/28 → 16/26, claim id mono 12/16 → 11/15. | `@media (max-width: 520px)` | Arrangement/measure, not pointer. |
| M8 | The facet-sheet trigger appears at the right end of the facet tab strip. | `@media (max-width: 860px)` | It exists exactly when the facet panel is not in the flow (M1). |
| M9 | Every icon-only control in the app bar, the drawer and the sheet header gets a **44 × 44** hit area; sheet rows get `min-height: 44px`. | `@media (pointer: coarse)` | Target size is a pointer question, never a width one. The engine already sets `.comment-chip { min-height: 44px }` here (`style.css:1764`). |
| M10 | The grabber is a drag affordance, not decoration; it is present whenever the sheet is. | `@media (pointer: coarse)` for the drag behaviour; `max-width: 520px` for the sheet's own layout | R-J.2 / `tokens.md` § 8. |
| M11 | Desktop hover states (`--hover-bg` washes on module rows, facet rows, the footer summary) must not be the only way to discover a control. | `@media (pointer: coarse)` | There is no hover on a phone. |

### Mobile nav sheets

Boards `4Y3-0` / `52B-0` (modules, light/dark) and `50U-0` / `552-0` (on this
facet, light/dark). Both are overlay states of the 02 mobile screen, not
separate screens: the underlying page in each is `4O0-0` / `4SX-0` unchanged.
They are reached from the two controls measured in § 4.18 — the 20px menu glyph
in the app bar (`4QK-0`) opens the modules drawer; the `On this facet` trigger at
the right end of the facet tab strip (`4XR-0`) opens the sheet.

Per **R-J.1** there is one sheet shell and the facet index is a **body** in it.
Measured on `50U-0` / `552-0` and conforming to **R-J.2** on every point:

| Property | Light | Dark | Node |
|---|---|---|---|
| Sheet size | `390 × 660`, `justify-content: end` on the frame | same | `50V-0` / `553-0` |
| Top corners | `16px` / `16px`, bottom `0` | same | `50V-0` |
| Ground | `--color-card` | `--color-dark-card` | `50V-0` / `553-0` |
| Dark-only top hairline | — | `1px solid --color-dark-border` | `553-0` |
| Grabber | `36 × 4`, radius `999px`, `--color-border-strong` `#C2CAD5`, under `8px` top padding, centred | `--color-dark-border-strong` `#38424F` | `50X-0` / `554-0` |
| Header | `padding: 14px 16px 12px`, `gap: 10px`, bottom `1px --color-border` | `--color-dark-border` | `50Y-0` / `556-0` |
| Header title | Inter **16/22**, w600, tracking `-0.01em`, `--color-ink`; text `On this facet` | `--color-dark-ink` | `50Z-0` |
| Header count | IBM Plex Mono 12/16, `--color-faint`; text `13` | `--color-dark-faint` | `510-0` |
| Close glyph | 20px, 2px stroke, `--color-muted`, hard right | `--color-dark-muted` | `511-0` |
| Body | `flex: 1 1 0`, `min-height: 0`, `padding: 6px 8px`, `overflow: clip` | same | `514-0` / `55C-0` |
| Row | `padding: 11px 12px`, `gap: 10px`; measured height `42px` | same | `515-0`, `518-0` |
| Active row | ground `#1C4E8C17` (`--color-accent-bg`), radius `8px`, title Inter 14/20 **w600** `--color-accent`, count mono 12/20 w500 `--color-accent` | dark twins | `515-0`, `516-0`, `517-0` |
| Rest row | title Inter 14/20 w400 `--color-muted`; blocked count mono 12/20 w400 `--color-blocked` | dark twins | `519-0`, `51A-0` |
| Scrim | `#10172057` — `--color-ink` @ 34 % | `#00000085` — black @ 52 % | `50U-0` / `552-0` |

The sheet is drawn **at** the 660px ceiling because thirteen rows overflow it —
which is R-J.3's "660 is a ceiling, not a height" shown from the clipping side.
The header carries both optional slots (second line absent, count present),
which is R-J.4's "slots, not forks".

**The modules drawer is not a sheet and must not be built from the sheet shell.**
Measured on `4Y3-0` / `52B-0`:

| Property | Light | Dark | Node |
|---|---|---|---|
| Panel | `328 × 844`, flush left, no radius | same | `4Y4-0` / `52C-0` |
| Ground / edge | `--color-paper` / right `1px --color-border` | `--color-dark-paper` / `--color-dark-border` | `4Y4-0` / `52C-0` |
| Scrim | `#10172057` | `#00000085` | `4Y3-0` / `52B-0` |
| Header | `padding: 22px 20px 16px`, `gap: 12px`; title Inter 19/24 w600 `-0.01em` ink; eyebrow Inter 12/16 faint; 20px close glyph 2px stroke muted, `margin-top: 2px` | dark twins | `4Y5-0` |
| Search block | `padding: 0 20px 14px` | same | `4YB-0` |
| List region | `flex: 1 1 0`, `padding-inline: 12px`, `overflow: clip` | same | `4YI-0` |
| Section header | `padding: 8px 8px 6px`, `gap: 8px`; `MODULES` Inter 11/14 w600 `0.09em` faint; count mono 11/14 faint; 12px chevron 2.6px faint | dark twins | `4YJ-0` |
| `TRACKS` header | `padding: 16px 8px 6px` | same | `4ZZ-0` |
| Row | **`height: 44px`**, `padding-inline: 10px` | same | `4YO-0` |
| Active row | ground `#1C4E8C17`, radius `8px`, `gap: 7px`; label Inter **15/20** w600 `--color-accent`; 14px padlock 2.2px `--color-locked`; count mono 12/16 `--color-accent` in a `20px` right-ranged slot | dark twins | `4Z8-0` |
| Footer | `padding: 14px 20px 22px`, `gap: 14px`, top `1px --color-border` | `--color-dark-border` | `505-0` |

Same content, same counts, same order as the desktop rail — the drawer is the
rail reached rather than always present, per note 3 on `4VD-0`.

---

## 6. Footer vocabulary

Quoted from the boards. Order is fixed and is not a screen's to change
(R-F.1 / R09.1). The four-entry order below is what R09.5 produces: sources is
split out of relationships and gets its own entry rather than being folded into
a single "links" count.

### Claim footer strip — desktop, claim with everything (`6W-0`)

> `Blocked` · `2 blockers` · ⌄ | `4 relationships` ⌄ · `2 sources` ⌄ ·
> `1 check` ⌄ · … · 💬 `0`

Order, left to right: **readiness, relationships, sources, checks**, then the
comment count hard right. The `|` is the measured `1 × 12` divider that follows
readiness only.

### Claim footer strip — desktop, claim with a zero (`33P-0`)

> `Blocked` · `2 blockers` ⌄ | `3 relationships` ⌄ · `1 source` ⌄ ·
> `No checks declared` ⌄ · … · 💬 `2`

- Counted nouns are **singularised**: `1 source`, not `1 sources`.
- A zero is a **sentence, not a `0`**: `No checks declared`, painted
  `--color-faint` rather than `--color-muted`.

### Claim footer strip — mobile (`5L0-0`)

Row one: `Blocked` `2 blockers` ⌄ … 💬 `0`
Row two: `4 relationships` ⌄ · `2 sources` ⌄ · `1 check` ⌄

Wording and order identical to desktop. Nothing is abbreviated for width.

### Facet-level banner

Desktop (`6T-0` / `6U-0`) and mobile (`56Y-0` / `570-0`), identical words:

> `All 26 contract claims are blocked by unapproved dependencies outside this module` — `Show issues`

The action label is exactly `Show issues` on both. Mobile adds a trailing
chevron; desktop does not.

### Module head metric (`67-0` / `68-0`)

> **`31`** `of 31 locked`

Two spans: the numeral at weight 600 in `--color-ink`, the phrase at weight 400
in `--color-muted`. The word `claims` does **not** appear.

### Module eyebrow (`64-0`)

> `MODULE 06 / 26`

Mono, tracking `0.08em`, uppercase as authored — position in the module list,
not a count of anything on the page.

### Facet panel (`8R-0`) and sheet header (`50Z-0`)

> `ON THIS FACET` (desktop panel, caps) / `On this facet` (mobile sheet header
> and mobile tab-strip trigger, sentence case)

The caps form is a section label; the sentence-case form is a title and a button
label. Both are the same words in the same order.

### Freshness footer (`1F4-0` / `1F5-0`)

> 🕐 `Updated 23 hours ago`
> `Claims changed since then are not in this view`

Elapsed, one unit, no timestamp (R10.1, R10.3). The caption states what the
elapsed figure *means for the reader*, which is why the rule is not satisfied by
the phrase alone. Under `dossierx serve` the phrase becomes `Live` (R10.5); the
caption then has nothing to qualify and is dropped.

### Sidebar (`3Q-0`, `3R-0`, `4Y-0`, `4Z-0`, `3M-0`, `5H-0`, `5N-0`, `5T-0`, `5X-0`)

> `MODULES` `26` · `TRACKS` `25` · `Search 828 claims` · `Claims graph` ·
> `Build order` · `Light` / `Dark`

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp` at `3ac8844`.

> **Every line number below is pinned to commit `3ac8844` and was verified
> against `git show 3ac8844:<path>`, not against the working tree.** The
> worktree is shared and was being edited by another lane while this spec was
> written: between the first and second read of `style.css` it grew from 4782 to
> 4870 lines, and `internal/render/geist_fonts.go` was replaced by
> `internal/render/engine_fonts.go`. Resolve any address by its quoted marker
> comment or selector first, and by the line number second.

### `internal/render/viewer/template/style.css` (4782 lines)

| Region | Marker comment (quoted) | Lines | What it owns on this screen |
|---|---|---|---|
| Palette | `/* THE palette. One unconditional :root, light-first: the values below are what` | `style.css:49`–`style.css:159` | The 28 tokens; `@media screen { html[data-theme="dark"] }` at `style.css:107`, `@media screen and (prefers-color-scheme: dark)` at `style.css:127` |
| Base claim card | `.claim {` | `style.css:227`–`style.css:233` | Pre-System-Record card: `border 1px var(--border)`, `radius 8px`, `padding 1rem 1.25rem`, `margin-bottom 1rem` |
| Claim body | `.claim-body {` | `style.css:245`–`style.css:249` | `font-size: 13px`, `white-space: pre-wrap` — the prose the design re-specs as serif 17/28 |
| Edges footer | `/* .claim-links is the <details> that wraps the whole edges footer` | `style.css:789`–`style.css:806` | The footer disclosure's contract |
| Comments | `/* ---- comments (engine-managed review threads) --------------------------` | `style.css:1589`–`style.css:1773` | `.comment-chip` `style.css:1602`, `--open` `style.css:1617`, `--empty` `style.css:1643`, reveal-on-hover `style.css:1649`–`style.css:1661`, count `style.css:1672` |
| Touch targets | `/* Coarse pointers (touch) need a >=44px hit target on the chip.` | `style.css:1760`–`style.css:1772` | `@media (pointer: coarse) { .comment-chip { min-height: 44px } }` — the existing precedent M9 extends |
| Status strip | `/* ---- status strip (lock-ledger integrity + lint) -----------------------` | `style.css:2012`–`style.css:2209` | `.status-strip` `style.css:2059`, `--integrity` `style.css:2078`, head `style.css:2084`, `-action` `style.css:2115`, body `style.css:2136` — the engine's realisation of the blocked banner |
| Comment touch | `/* Coarse pointers (touch) need >=44px hit targets on every comment control. */` | `style.css:2210`–`style.css:2222` | |
| Mobile sheet | `/* Mobile: the rail becomes a modal bottom sheet — the dimming backdrop and the` | `style.css:2223`–`style.css:2275` | `@media (max-width: 860px)`: `.comments-rail` becomes a sheet with `border-radius: 14px 14px 0 0`, `max-height: 70vh`; `.status-strip-head { min-height: 44px }` |
| Claim head | `/* The claim head line. v0.4.1 gives it two children instead of loose text:` | `style.css:2277`–`style.css:2310` | The `.k` flex row that carries the label, the status pill and the chip slot |
| Pills | `.pill {` | `style.css:2311`–`style.css:2331` and `style.css:3952`–`style.css:3978` | The LOCKED/DRAFT/review-pending pill family |
| Sidebar + tabs | `/* ---- sidebar + tab navigation ---------------------------------------` | `style.css:2585`–`style.css:2755` | `.sidebar` `style.css:2601` (210px, `--card-bg`), `.sidebar-header` `style.css:2614`, `#nav` `style.css:2664`, `.sidebar-footer` `style.css:2673`, `.sidebar-footer-stamp` `style.css:2706`, `.sec-tab` `style.css:2715`, `.sec-tab.on` `style.css:2730` |
| Facet tab strip | `/* Secondary horizontal facet tab strip inside a module's content area,` | `style.css:2756`–`style.css:2783` | `.sub-nav` `style.css:2761`, `.subtab` `style.css:2767`, `.subtab.on` `style.css:2779` — the engine's Contract/Internals strip |
| Content area | `.content-area {` | `style.css:2784`–`style.css:2788` | `flex: 1`, `min-width: 0`, `margin: 0 auto` |
| Soft-mount host | `/* Soft-mount host: claim cards are cloned into this wrapper.` | `style.css:2794`–`style.css:2801` | Must survive any card-geometry change |
| System Record layer | `/* System Record visual language for the generated viewer. */` | `style.css:3050`–`style.css:4417` | The layer that actually paints this screen today |
| — body | `body.shell-body {` | `style.css:3054`–`style.css:3063` | `font-size: 15px`, `line-height: 1.62`, `letter-spacing: -.004em` |
| — sidebar | `.sidebar {` | `style.css:3076`–`style.css:3116` | `width: var(--system-record-sidebar-width, 270px)`, `min-width 220px`, `max-width 420px` |
| — collapse | `@media (min-width: 861px) {` | `style.css:3157`–`style.css:3186` | `body.system-sidebar-collapsed .sidebar { width: 44px }` |
| — nav | `#nav {` | `style.css:3229`–`style.css:3233` | `padding: 12px 10px 24px` |
| — nav rows | `.sec-tab {` | `style.css:3314`–`style.css:3337` | `min-height 35px`, `padding 8px 10px`, `radius 4px`, `font-size 12px`; `.sec-tab.on` `style.css:3332` takes `var(--accent-bg)` + `inset 2px 0 0 var(--accent)` |
| — rail reserve | `@media (min-width: 1181px) {` | `style.css:3378`–`style.css:3380` | `body.system-toc-collapsed .content-area { padding-right: 86px }` |
| — build-order reserve | `/* The Build order tab reclaims the 282px right rail .content-area reserves` | `style.css:3382`–`style.css:3393` | `@media screen and (min-width: 1181px)` |
| — module head | `.system-record-head {` | `style.css:3395`–`style.css:3433` | Grid `minmax(0,1fr) auto`, `gap 22px`, `padding 38px 0 24px`, `border-bottom 1px var(--border-strong)`; `h2` `clamp(28px, 4vw, 42px)` weight 560 tracking `-.04em`; `__metric` `11px/1.4` |
| — facet strip | `.sub-nav {` | `style.css:3435`–`style.css:3459` | `position: sticky`, `top: 0`, `z-index: 8`, `gap: 23px`; `.subtab.on` at `style.css:3455` |
| — claim card | `.card,` / `.claim-tree {` | `style.css:3842`–`style.css:3854` | `margin 0 0 14px`, `padding 20px 21px`, `border 1px var(--border)`, `radius var(--radius)`, `background var(--card-bg)`; hover at `style.css:3851` |
| — warn card | `.claim-card--warn,` / `.claim-banner {` | `style.css:3856`–`style.css:3861` | `border-color color-mix(--warn 45% --border)`, `background color-mix(--warn-bg 50% --card-bg)`, `inset 0 2px 0` |
| — scroll margin | `.claim { scroll-margin-top: 72px; }` | `style.css:3829` | Deep-link landing offset |
| — facet TOC | `.facet-toc {` | `style.css:3668`–`style.css:3790` | **`position: fixed; top: 28px; right: 18px; width: 236px; border-left: 1px solid var(--border); background: var(--paper)`** — the engine's right-hand facet panel; `__head` `style.css:3697`, `__list` `style.css:3764`, `__item` `style.css:3773`, `__item.on` `style.css:3789` |
| — footer | `.claim-links {` | `style.css:4020`–`style.css:4063` | `margin-top 17px`, `border 1px`, `radius 8px`, `background color-mix(--paper 55% --card-bg)`; `.claim-links-summary` `style.css:4028` (`min-height 56px`, `font: 650 10px/1.35 var(--font-mono)`); `.claim-footer__counts` `style.css:4053`; count chips `style.css:4054`–`style.css:4061` |
| — chip | `.comment-chip {` | `style.css:4136`–`style.css:4145` | System Record override of the base chip |
| Narrow desktop | `@media (min-width: 861px) and (max-width: 1180px) {` | `style.css:4494`–`style.css:4531` | `.content-area { padding: 0 34px 76px }`, `.facet-toc { top: 10px; right: 12px }` and a `min(260px, …)` width; the rest of the block already collapses the facet TOC to a head-only `<select>` — `__disclosure`/`__list` `display: none` at `style.css:4507`, `__head` to a 40px absolute corner at `style.css:4509`, `__select` at `style.css:4520` |
| Tablet & below | `@media (max-width: 860px) {` | `style.css:4533`–`style.css:4591` | `.content-area { padding: 54px 16px 58px }`, `.sidebar { width: min(310px, 100vw - 42px) }`, `.facet-toc` becomes a fixed 248px popover with a `<select>` |
| Narrow tier | `@media (max-width: 560px) {` | `style.css:4647`–`style.css:4658` | **Already owns `.claim-footer__counts`** — M6 must be written to win over it |
| Phone tier | `@media (max-width: 520px) {` | `style.css:1175` | Where M3–M7 land |
| Print | `/* ---- print — THE @media print block ------------------------------------` | `style.css:4660`–`style.css:4782` | Last block in the file; `.facet-toc` hidden at `style.css:4751`, `.status-strip { display: none !important }` at `style.css:4754` |

### `internal/render/viewer/template/graph.css` (1145 lines)

**No selector in `graph.css` paints anything on screen 02.** Every selector is
prefixed `#dxg` / `.dxg-` by its own contract (`graph.css:11`–`graph.css:13`),
and this screen renders no graph pane. The one cross-over is that `style.css`
re-declares `--dxg-facet-other` at `style.css:76` (light `#0d55b5`) and
`style.css:120` / `style.css:140` (dark `#77a9e8`), overriding `graph.css`
because `style.css` is emitted second; its only consumer is
`.dxg-legend-module` at `style.css:4415`, which is off this screen.

### Runtime JavaScript

| Function | Address | What it does for this screen |
|---|---|---|
| `addModuleHeaders` | `internal/render/viewer/template/system-record.js:403`–`system-record.js:430` | Builds the `.system-record-head` from `data-claim-count` / `data-locked-count` / `data-facet-count`. Emits `<strong>N of M</strong> claims locked` at `system-record.js:425` and the summary `N claims across M record sections.` at `system-record.js:423`. |
| `renderToc` | `system-record.js:342`–`system-record.js:401` | Builds `#systemFacetToc` (`system-record.js:356`) and every `.facet-toc__item` (`system-record.js:386`). Each item is `<span>` two-digit index + `<strong>` title — **an index, not a count**. |
| `updateTocActive` | called at `system-record.js:400` | Applies `.facet-toc__item.on`. |
| `enhanceTimestamp` | `system-record.js:37`–`system-record.js:44` | Rewrites `.sidebar-footer-stamp` to `Generated <ordinal> <month>, <year> <time>` (`formatGeneratedTime`, `system-record.js:29`–`system-record.js:35`), and sets a `title`. **This is the function R10.1/R10.3/R10.4 replace.** |
| `bindResizer` | `system-record.js:46`–`system-record.js:62` | Drives `--system-record-sidebar-width`, clamped 220–420. |
| `showModuleFacet` | `internal/render/viewer/template/viewer-runtime.js:216` | Facet switching behind the `.subtab` strip; materialises the soft-mounted surface before strip filtering (`viewer-runtime.js:239`–`viewer-runtime.js:246`). |
| `setDrawer` | `viewer-runtime.js:322`–`viewer-runtime.js:341` | Owns `body.nav-open`, `#navToggle` `aria-expanded` and `#navOverlay` — **the existing mobile modules drawer**. Bound at `viewer-runtime.js:2231`–`viewer-runtime.js:2238`. |
| `renderStatusStrip` | `viewer-runtime.js:1638`–`viewer-runtime.js:1740` | Paints `#statusStrip` from `/api/status`; hides it when no group is actionable (`viewer-runtime.js:1656`–`viewer-runtime.js:1660`); toggles `status-strip--integrity` / `--lint` at `viewer-runtime.js:1713`–`viewer-runtime.js:1714`. Offline fallback at `viewer-runtime.js:2274`. |
| `graph-ui.js` | `internal/render/viewer/template/graph-ui.js:345`, `graph-ui.js:1178` | Owns the `Claims graph` surface the sidebar footer row opens. |
| `build-order-ui.js` | `internal/render/viewer/template/build-order-ui.js:4`–`build-order-ui.js:8` | Lazy Mermaid renderer behind the `Build order` nav entry. |

### `internal/render/viewer/template/shell.html` (333 lines)

| Address | What it is |
|---|---|
| `shell.html:53` | `#navToggle` — the mobile drawer trigger |
| `shell.html:54` | `#navOverlay` — the drawer scrim |
| `shell.html:73` | `#dxgOpen` — `Claims graph` nav row |
| `shell.html:115`–`shell.html:117` | `Build order` nav group and its `.sec-tab` |
| `shell.html:123`–`shell.html:131` | `.sidebar-footer` — `.theme-control` (`System` / `Light` / `Dark`) and `.sidebar-footer-stamp` |
| `shell.html:133` | `<main class="content-area">` |
| `shell.html:136` | `<section class="module-section">` carrying `data-claim-count` / `data-locked-count` / `data-facet-count` |
| `shell.html:138`–`shell.html:141` | `.sub-nav` / `.subtab` — the facet tab strip |
| `shell.html:145` | `<section class="claim-group">` + `data-dossierx-surface` |
| `shell.html:272`–`shell.html:281` | `#statusStrip`, with `Show issues` as the literal action label at `shell.html:278` |

### Go emitters, `internal/render`

| Address | What it is |
|---|---|
| `internal/render/render.go:139`–`render.go:144` | `GeneratedAt` field |
| `internal/render/render.go:906` | `GeneratedAt: in.generatedAt.Format("2006-01-02 15:04 UTC")` — the stamp R10.4 must turn into a machine-readable instant |
| `internal/render/render.go:259`–`render.go:283` | `Group` / `TabLabel` / `AllLocked` — the facet tab strip's data |
| `internal/render/render.go:298`–`render.go:338` | Module → facet grouping and module-level `AllLocked` |
| `internal/render/render.go:1401`–`render.go:1405` | Where `AllLocked` and `TabLabel` are populated |
| `internal/render/components/card.html:38` | `<section class="claim claim-card card…" data-status>` |
| `internal/render/components/card.html:39` | `<div class="k">` — label, `{{pillClass}}` status pill, `{{commentChip .}}` |
| `internal/render/components/card.html:40` | `<div class="claim-body">` |
| `internal/render/components/card.html:42` | `{{edges .}}` — the footer |
| `internal/render/components/components.go:378` (`EdgesHTMLWithLinks`), doc block from `components.go:357`; emission at `components.go:569`–`components.go:575` | Emits `<details class="claim-links">`, `<summary class="claim-links-summary">`, `<ul class="claim-edges">` |
| `internal/render/components/components.go:530`–`components.go:539` | The two server-side auto-open signals (drifted link at `components.go:531`–`components.go:536`, review_pending at `components.go:537`–`components.go:539`) |
| `internal/render/components/components.go:554`–`components.go:562`, with `countSegment` at `components.go:608` | The summary digest and its pluralisation |
| `internal/render/components/components.go:659` (`CommentChipHTML`), doc block from `components.go:615`; variants at `components.go:662`–`components.go:672` | The three chip variants (`--empty`, `--resolved`, `--open`) and the `hidden` slot |
| `internal/render/components/components.go:67` | `"commentChip": CommentChipHTML` in the funcMap |
| `internal/render/geist_fonts.go` (at `3ac8844`) | The precedent for an engine-owned inlined face — the mechanism a serif family would use. **Already superseded in the shared worktree:** an uncommitted change deletes this file in favour of `internal/render/engine_fonts.go` and adds `inter-latin-wght.woff2`, `source-serif-4-latin-opsz-wght.woff2` and three IBM Plex Mono faces under `viewer/template/fonts/`. A lane agent must re-read that path before citing it. |

### Tests that assert on these selectors today

| Address | What it pins |
|---|---|
| `viewer-tests/claim_collapse_test.go:192`–`claim_collapse_test.go:199` | `#systemFacetToc` collapsed width is exactly `44`, expanded `>= 220`; the toggle's `aria-expanded` / `aria-label` strings |
| `viewer-tests/claim_collapse_test.go:98`–`claim_collapse_test.go:112` | `.comment-chip` stays present and clickable inside a collapsed claim head |
| `viewer-tests/component_fit_test.go:81`, `component_fit_test.go:185` | `#statusStrip` `status-strip--open` state |
| `viewer-tests/theme_parity_test.go:149`–`theme_parity_test.go:160` | Surface groups 22–24: `.facet-toc` box-shadow, `.facet-toc__select` colours, and `.sidebar` / `.card` / `.claim-banner` / `.sec-tab.on` / `.facet-toc__item.on` backgrounds |
| `viewer-tests/theme_parity_test.go:828`–`theme_parity_test.go:829` | `.status-strip` background-color and border-color parity |
| `viewer-tests/theme_parity_test.go:1275` | The suppression sheet that hides `.facet-toc`, `#statusStrip`, `.comment-chip` for screenshot parity — any renamed selector must be updated here or parity silently stops covering it |
| `viewer-tests/reveal_test.go:98`–`reveal_test.go:99` | `.claim:target .claim-links::details-content` and the print rule — the deep-link auto-open contract |
| `viewer-tests/reveal_test.go:149`, `reveal_test.go:191`, `reveal_test.go:260` | `details.claim-links` must exist and be visible |
| `viewer-tests/empty_chip_test.go:39` | The zero-thread chip's resting state |
| `viewer-tests/system_record_layout_test.go:14` | `TestClaimBodyUsesAvailableCardWidth` — the measure |
| `viewer-tests/reading_view_scale_test.go:19`–`reading_view_scale_test.go:32` | The reading-view scale budgets this screen must not regress |
| `internal/render/theme_tokens_test.go:90` | `TestEveryAllowlistedTokenHasAConsumer` — a new token without a consumer fails |
| `internal/render/theme_tokens_test.go:175` | `TestStyleCSSModeAndPrintStructure` — the light-first / one-print-block invariant |
| `internal/render/theme_tokens_test.go:572` | `TestConsumerReadWinsCascade` — a `var()` must live in the declaration that wins |

---

## 8. States not on the boards

Each entry says what the spec implies, derived from the rules. A lane must not
be left guessing.

1. **DRAFT claim.** 02 shows only LOCKED. The chip keeps the § 4.12 geometry
   exactly and swaps the colour pair: ground `--color-draft-bg`, padlock replaced
   by no glyph (a draft is not locked), label `DRAFT` in `--color-draft`. Dark
   uses `--color-dark-draft` / `--color-dark-draft-bg` — and note
   `tokens.md` Disagreement 9: the engine has **no dark override** for
   `--status-draft`, so this needs one.
2. **`review_pending`, all three triggers.** The engine writes the footer ` open`
   server-side when a claim is locked + review_pending
   (`components.go:537`–`components.go:539`). R09.3 permits auto-open only for
   readiness when blocked; a review_pending footer that opens *relationships*
   violates it. Spec: review_pending opens **readiness only**, and the status pill
   takes the `--color-draft` pair (not-settled), never `--color-blocked`
   (a dependency problem) — R08.1's two-hue discipline decides this.
3. **Every claim not blocked.** The facet banner disappears entirely (the engine
   already hides `#statusStrip` when nothing is actionable,
   `viewer-runtime.js:1656`). The claim footer's first entry becomes the
   non-blocked readiness word with no dot and `--color-muted`, keeping position
   one; it never disappears, because a fixed four-entry order is the rule
   (R-F.1).
4. **Mixed facet — some blocked, some not.** The banner's sentence is count-led
   (`All 26 contract claims are blocked…`); with a partial count it must state
   the count, not "some". The facet panel then paints only the blocked rows'
   counts `--color-blocked` and leaves the rest `--color-faint`, which is what
   makes R09.7's single signal useful rather than uniform.
5. **Zero comment threads.** Desktop keeps the passive faint count (`0` is drawn
   on `6W-0`). Mobile must still render the control, because R-J.6 makes "empty"
   a state of the sheet rather than an absence of it. The engine's third chip
   variant (`comment-chip--empty`, `components.go:663`) is the right hook, and
   its `opacity: 0` reveal-on-hover behaviour (`style.css:1643`) must not survive
   on `pointer: coarse` — `style.css:1769` already pins it back to `.75` there.
6. **A facet with one claim.** The facet panel and the sheet still render; the
   sheet's `min-height: 240px` (R-J.3) means a one-row body is a short sheet, not
   a stretched one. `renderToc` already singularises (`system-record.js:375`).
7. **A facet with no claims.** `renderToc` hides the TOC entirely
   (`system-record.js:370`–`system-record.js:371`). Spec: the mobile `On this facet` trigger hides with
   it — a control that opens an empty sheet is a dead control, the same reasoning
   `components.go` gives for the hidden zero-state chip slot.
8. **Long claim titles.** Desktop title is 20/26 in a 694px column and wraps
   freely; the status chip is `flex-shrink: 0` and right-ranged, so it never
   compresses. Mobile puts the chip on its own line above (R-H.2) precisely so
   the title runs the full 358px. Facet-panel rows already wrap to three lines
   (`1CC-0`, `1CG-0` are 71px tall) — the count slot stays `22px` and
   `align-items: start`, so the count aligns to the title's first line, not its
   centre.
9. **Long module names in the rail.** The label is `flex: 1 1 0` and the count
   slot is a fixed `16px` right-ranged lane (`3V-0`), so the count never moves.
   Truncation is the label's problem, never the count's.
10. **A module with no all-locked padlock.** The 13px padlock is simply absent and
    the `6px` gap collapses (compare `3T-0`, which has no gap, with `4A-0`, which
    does). The row height stays `32px`.
11. **Dependency cycles.** Not depicted anywhere on 02. A cycle is a graph state;
    the reading view's only obligation is that the readiness entry still reads as
    a noun and a count (`N blockers`), and that the banner still routes to 04.
    `--color-graph-cycle` belongs to the graph pane and must not leak into the
    footer — R08.1 forbids a third hue in this vocabulary.
12. **`dossierx serve` vs. a built file.** R10.5: the freshness phrase becomes
    `Live`. The caption `Claims changed since then are not in this view` is
    dropped with it, since it qualifies an elapsed figure that no longer exists.
    Nothing else on the screen changes.
13. **Stale build (> 7 d).** R10.2: the phrase goes to weight 600 in
    `--color-draft` and the clock icon to `--color-draft`. Geometry unchanged.
    On dark this needs `--color-dark-draft`, which the engine does not yet have.
14. **Focus mode active.** Board 03's state, not 02's. From
    R11.1/R11.2/R11.3/R11.4:
    both rails leave, the page grows to 1140px, the claim measure stays at 760px
    (pixel-identical prose), and the `Focus` button `13D-0` takes its active
    state in place.
15. **Print.** Always the light palette (`tokens.md` § 1). `.facet-toc` and
    `.status-strip` are already hidden (`style.css:4751`, `style.css:4754`); the
    blocked banner's information therefore has to survive in the claim footer,
    which it does, because the footer's readiness entry is per-claim.
16. **Search.** The board draws a `Search 828 claims` field (`3I-0`, `4YB-0`).
    **The engine has no search of any kind** — no `search` identifier exists in
    `shell.html` or `system-record.js`. See § 9.

---

## 9. Open decisions

Recorded, not asked, per the freeze protocol.

1. **Readiness does not auto-open on this screen, even though every claim is
   blocked.** R09.3 says readiness *may* auto-open when a claim is blocked, and
   all four 02 boards show it closed on a facet where all 26 claims are blocked.
   Decision: **auto-open is per-claim and is suppressed when the facet-level
   banner is showing.** The banner already states the fact at facet scale;
   opening 26 panels underneath it would restate it 26 times and destroy the
   reading view. The trigger therefore reads: readiness auto-opens when *this
   claim* is blocked **and** no facet-level blocked banner is present.
2. **The desktop footer is a plain inline control row, not pills.** Measured on
   `6W-0` and `33P-0`: no ground, no border, no radius, 13/16 type, a single
   `1 × 12` divider after readiness. R-F.2's 30px pill geometry is the **mobile**
   form (its own evidence is components section I3, which argues from "at 12px on
   a phone"). R-H.0 names the footer strip as one of exactly three components
   that change shape below the phone breakpoint, which is what licenses this.
   Decision: pills below 520px, plain row above.
3. **`--container-rail` and `--container-toc` are swapped relative to this
   screen.** `tokens.md` § 6 labels `--container-rail: 268px` "Right rail" and
   `--container-toc: 244px` "Left facet TOC". Measured, `3C-0` (268px) is the
   **left** modules rail and `8P-0` (244px) is the **right** facet panel.
   Decision: the widths are right, the descriptions are inverted; implement
   268 left / 244 right. `reference-rules.md` R09.7's phrase "the left TOC on
   every desktop board" is wrong for board 02 for the same reason.
4. **The modules drawer is a second overlay shell and that is intended.** R-J.1
   says there is one *bottom sheet*. `4Y3-0` is a 328px left drawer, not a sheet,
   and the notes strip (`4VD-0`, note 3) states the two shapes as a deliberate
   pair. Decision: the drawer does not fall under R-J.1; it reuses the engine's
   existing `body.nav-open` / `#navOverlay` mechanism
   (`viewer-runtime.js:322`–`viewer-runtime.js:341`). Any *third* overlay that is
   neither of these two is out of order.
5. **Scrim: use the engine's `--scrim`, not the boards' values.** Boards paint
   `#10172057` (ink @ 34 %) light and `#00000085` (black @ 52 %) dark; the engine
   token is `rgba(0,0,0,.22)` / `rgba(0,0,0,.42)`. The token is in the closed
   allowlist and is themeable; the board values are not tokens. Decision: ship
   `var(--scrim)` for both the sheet and the drawer, on both boards' behalf.
6. **Sheet rows go to 44px under `pointer: coarse`.** Measured `42px`
   (`515-0`, `518-0`) against the drawer's `44px` (`4YO-0`) for the same kind of
   row. Decision: 44 wins; the 2px comes out of the sheet's list padding, not out
   of the row's 11px block padding, so the type does not move.
7. **The mobile comment control takes the accent pill, against what 02 draws.**
   R-F.3 and components section J2 (`B0C-0`) specify a 30px `--radius-pill` pill
   on `--color-accent-bg` with a 14px bubble and a 13/16 weight-600 accent label.
   Both 02 mobile boards paint it faint and bare. Decision: **implement the
   component board** (R00.0, and `reference-rules.md` § Open decisions: the
   reference board wins and the screen is flagged). It applies at count `0` too,
   because R-J.6 makes "empty" a state of the sheet rather than a reason to omit
   its control.
8. **The active facet row keeps the accent count, losing its blocked signal, and
   that is accepted.** Measured: `1B7-0` paints `2` in `--color-accent` while its
   twelve siblings paint theirs `--color-blocked`, and the claim in question is
   blocked. Decision: accepted as drawn. "Where you are" is a stronger, rarer
   signal than "this one is blocked", the reader is already looking at that
   claim's own footer, and two colours in one 22px slot would be unreadable.
9. **The facet panel's numeral is a count, not an index — the engine's is an
   index.** `renderToc` writes a zero-padded position (`system-record.js:382`, `system-record.js:389`);
   the board writes a per-claim count coloured by blockedness
   (`1BB-0` and eleven siblings). Decision: the board is the specification. The
   number to show is the claim's **blocker count when blocked**, which is what
   makes R09.7's single signal informative; an index number carries no health
   information and R09.7 says this slot is where the health signal lives.
10. **Untokened literals measured on 02, and what to do with each.**
    - `#8592A3` (search icon, `3J-0`) → ship `--color-faint`
      (`tokens.md` Disagreement 4).
    - `#E8ECF1` / `#212934` (the in-card footer hairline, `6W-0` / `4JN-0`) → a
      *lighter* hairline than `--color-border`, used inside a card rather than
      between cards. Decision: ship
      `color-mix(in srgb, var(--border) 55%, var(--card-bg))`; do not add a
      token, the allowlist is closed.
    - `#E3E7ED` / `#080B10` (theme-toggle track, `5O-0` / `4ID-0`) → ship
      `color-mix(in srgb, var(--paper) 88%, var(--ink))` light and
      `color-mix(in srgb, var(--paper) 88%, #000)` dark. No token.
    - Raw alpha-baked blocked/locked hexes (`#9E3B360F`, `#9E3B3617`,
      `#9E3B3633`, `#9E3B362E`, `#2C6B521A`, `#63BE9A21`, `#EC8A831A`,
      `#EC8A8317`, `#EC8A8321`, `#EC8A8342`, `#EC8A8338`, `#1C4E8C17`,
      `#6AA6E824`) → ship the `*-bg` tokens, or `color-mix` off the base token
      where no `-bg` token exists (`tokens.md` Disagreement 12).
11. **The navigation tint must not be the engine's `--accent-bg`.** The engine
    paints `.sec-tab.on` and `.facet-toc__item.on` with `var(--accent-bg)`
    (`style.css:3332`, `style.css:3789`), which is the **green lock** tint; the
    boards paint navigation with the navy accent at 9 % / 14 %. Decision: follow
    `tokens.md` § Open decisions — `color-mix(in srgb, var(--link) 9%,
    transparent)` light, 14 % dark. This changes two live selectors and both are
    covered by `theme_parity_test.go:152`.
12. **`Search 828 claims` is drawn but has no engine.** Decision: **do not ship a
    non-functional search field.** Until a lane implements search, the sidebar
    header block (`3D-0`) runs straight into the module list and the drawer's
    `4YB-0` block is omitted; the 16px / 14px of bottom padding it carried goes
    with it. A field that does nothing is worse than no field, and a dead control
    in the most-used chrome on the screen is the exact thing R09.6's "the banner
    is the only way in" reasoning is protecting against.
13. **The module-head metric wording changes.** Engine emits
    `<strong>31 of 31</strong> claims locked` (`system-record.js:425`); the board
    reads `**31** of 31 locked` with the bold on the numeral alone. Decision: the
    board wins — drop `claims`, move the weight to the first numeral only.
14. **Freshness replaces the generated stamp.** `enhanceTimestamp`
    (`system-record.js:37`) currently produces an absolute local date. Decision:
    it becomes the elapsed phrase (R10.1, one unit per R10.3), computed in the
    browser from a machine-readable stamp (R10.4), with the absolute time kept on
    the `title` attribute — which `enhanceTimestamp` already sets
    (`system-record.js:41`), so the mechanism exists. `render.go:906` must emit an
    instant a browser can parse.

### Paper defects

1. **`4FO-0` (desktop dark) loses the serif on claim prose.** Both prose nodes
   inside `4JN-0` compute `fontFamily: "system-ui, sans-serif"` where the light
   twin `6W-0` computes `var(--font-serif)`. The mobile dark board `4SX-0`
   (`4U6-0`) keeps `"Source Serif 4", system-ui, sans-serif`, so this is isolated
   to one board. It also directly contradicts that board group's own note
   (`4VD-0`: dark changes "nothing but the palette"). **Ship Source Serif 4 on
   dark.**
2. **Both mobile 02 boards contradict the components board on the comment
   control.** `5L0-0` / `5LR-0` paint a bare faint bubble + count; R-F.3 and
   `B0C-0` specify an accent pill. Approval is not proof: the component board
   wins (§ 9.7).
3. **The `ON THIS FACET` panel has no status dot.** R09.7 names "the status dot
   in `ON THIS FACET`" as the reading view's only project-level health signal;
   no node in `1B3-0` or `4MA-0` is a dot. The signal is carried by count
   colour instead. Either the rule's wording or the board is wrong; the board's
   mechanism is the one that works at 13 rows, so the rule's wording is the
   defect.
4. **The dark blocked banner uses a tint where a surface token exists.**
   `4JG-0` paints `#EC8A831A` (dark blocked @ ~10 %) over `--color-dark-card`.
   `--color-dark-blocked-surface` `#1D1618` and
   `--color-dark-blocked-hairline` `#3A2A2B` exist in the token set precisely
   because "on dark, a 10 % red tint is invisible" (`tokens.md` § 1). The board
   did the thing the tokens were created to prevent. Flagged; the resolution is
   `tokens.md` Disagreement 10's — neither token has an engine home, so the tint
   ships for now and the surface tokens stay recorded as the correction owed.
5. **The notes strip is not named.** The root child at worldY 14200 is an unnamed
   `Frame` (`4VD-0`) where every other group's equivalent is named
   `Group NN — notes`. Likewise the band at worldY 12980 (`4EY-0`) against
   `Group 03 — band` (`4VO-0`). Cosmetic, but it defeats name-based lookup and
   cost this agent a positional inference.
6. **Untokened literals on a board that has tokens for them.** `#8592A3`,
   `#E8ECF1`, `#212934`, `#E3E7ED`, `#080B10`, and the thirteen alpha-baked
   status hexes listed in § 9.10. Board `3B-0` spells families as
   `var(--font-sans)` / `var(--font-serif)` / `var(--font-mono)` while
   `4FO-0`, `4O0-0`, `4SX-0`, `4Y3-0`, `50U-0`, `52B-0` and `552-0` spell them as
   the literal `"Inter", system-ui, sans-serif` — the same split
   `tokens.md` Disagreement 12 records for the components board.
7. **Scrim values are literals, and the two modes disagree by more than value.**
   Light `#10172057` is an ink-tinted scrim; dark `#00000085` is a neutral black
   one. A scrim that changes *hue* between modes is not a re-pointed token.
   Resolved by § 9.5.
8. **Sheet rows (42px) and drawer rows (44px) disagree on the same gesture.**
   `515-0` / `518-0` against `4YO-0`. Resolved by § 9.6.
9. **The desktop `Focus` control has radius `7px` and the mobile facet trigger
   has radius `7px`, neither of which is in the radius scale** (4 / 8 / 999).
   Recorded per `tokens.md` § Open decisions ("off-scale radii stay as
   per-component literals") rather than corrected — but two components choosing
   the same off-scale value suggests the scale is missing a step, and that is a
   foundations question, not a screen one.
