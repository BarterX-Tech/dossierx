# 06 · Claim — blocked across four modules

Screen group 06 of the viewer design revamp. Source of record: Paper file
`01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`. Engine side read from the worktree
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp` at `3ac8844`.

Read this with `../tokens.md` (every token name and its light/dark value) and
`../reference-rules.md` (the rules cited by id below). Where this file names a
colour, the token is the one in `tokens.md`; a raw hex printed on a board is
evidence of what the designer meant, not permission to ship it.

Every number below was read with `get_computed_styles` or `get_jsx` on the node
named beside it. Nothing here is estimated from a screenshot.

---

## 1. Boards

| Node | Name | Size | What it shows |
|---|---|---|---|
| `1SY-0` | 06 · Claim — blocked across four modules | 1220 × 1868 | **Desktop light.** One full claim card: LOCKED status, blocked readiness (23 blockers across 4 modules), 13 relationships, 5 sources, 1 failing implementation check. All four metadata expansions drawn open. |
| `6CH-0` | 06 · … · DARK | 1220 × 1868 | **Desktop dark.** The same card, same states, same copy, in the dark token set. Structurally a node-for-node mirror of `1SY-0`. |
| `6JS-0` | 06 · … · MOBILE LIGHT | 390 × 2447 (fit-content) | **Mobile light.** Same claim, same four sections, same counts; the six mobile shape changes listed in the notes strip. No app bar, no device frame — a spec column, not a screen. |
| `72F-0` | 06 · … · MOBILE DARK | 390 × 2447 (fit-content) | **Mobile dark.** Node-for-node mirror of `6JS-0` in dark tokens. |
| `6BY-0` | Group 06 — band | 3420 × 27.5 | The group band: title eyebrow, the sub-label "four designs of one screen · light pair left, dark pair right", and a 2px `--color-accent` rule. |
| `7E6-0` | Group 06 — notes | 2828 × 241 | Three note columns (900px each, 64px gutter): WHAT THIS BOARD ANSWERS, ALL FOUR DOORS ARE OPEN AT ONCE — NO REAL SCREEN LOOKS LIKE THIS, WHAT MOBILE DOES DIFFERENTLY. |
| `O2-0` | 08 · Severity & integrity | 832 × 369 | **Reference board.** The severity vocabulary (five terms, two hues) and the lint-vs-ledger split. Applied here through R08.1 / R08.2. |
| `WI-0` | 09 · Placement map | 1180 × 1437 | **Reference board.** Where each component appears; the four-expansions-of-one-strip rule and the auto-open rule. Applied here through R09.1–R09.9. |

### Viewer states these boards depict

- Claim lifecycle: **locked** (`LOCKED` chip, padlock glyph). Not draft, not
  review_pending.
- Readiness: **blocked**, 23 blocker facts, grouped into **4 modules** —
  Capability support (11, nearest 1 hop), Verification (7, nearest 1 hop),
  Audience boundary (3, nearest 2 hops), Startup readiness (2, nearest 3 hops).
  The nearest module is expanded; the other three are one-line rows.
- Implementation checks: **1 check failing**, verdict word **Mismatch**, on a
  `test-coverage`-shaped check (13 declared steps, 12 exercised, step 1 never
  run).
- Relationships: 13 total across the three fixed directions — governed by (1),
  depends on (10, three shown + "Show 7 more"), depended on by (2, one shown).
- Sources: 5 total, three shown + "Show 2 more".
- Comments: **zero threads** — the count reads `0`.
- All four metadata expansions drawn **open simultaneously**. See §2; this is a
  board convention, not a reachable product state.

---

## 2. Design intent

### What the reviewer should perceive

The board's own answer, from the notes strip (`7E6-0`, column 1, verbatim
intent): *one question — 23 blockers is too many to read as a flat list, so
which module do I chase first.* Everything else on the screen is subordinate to
that. The blockers group by **the module that owns the fix**, which is the same
grouping the Issues screen uses, and each group leads with its **nearest hop
distance**, because *depth, not severity, is what tells the reviewer how far
away the fix is*. Only the nearest module is opened; the other three stay as
one-line rows carrying a count and a hop distance.

The card is a reading surface first. The claim's prose is serif at 17/28 on a
760px measure (`1W2-0`) and is the only thing on the card set in a reading face;
everything the interface says about the claim is sans, and everything the
machine emitted — the claim id, the dependency slugs, the section counts — is
mono. That three-way split is what lets a reviewer skim the card without reading
it.

The blocked state is said **once, loudly, in the right place**: the footer
strip's first chip reads `Blocked · 23 blockers` in `--color-blocked`
(`1VC-0`), and the readiness section directly beneath it takes a blocked-tinted
ground (`#FBF7F7` light / `--color-dark-blocked-surface` dark, `2YR-0` / `6DR-0`)
so the section itself is visibly the problem area. The checks section takes the
same tinted ground because it too carries a failure. Relationships and sources
sit on neutral grounds — they are context, not faults. A reviewer scanning the
card vertically sees two tinted bands and two quiet ones, and that is the whole
triage.

The blocker rows themselves never assert more than they know. Each row is a
title, a hop pill (`direct · 1 hop`), and a **dependency path of exactly two
slugs** — this claim's own slug, a chevron, and the unapproved target's slug in
blocked colour. No tree, no map, no chain.

### Rules that bind this screen

- **R09.1 — four expansions of one strip, never four panels.** Readiness,
  relationships, sources and checks are four doors on one metadata strip
  (`1VC-0`), which is one line at rest. This screen is the strip with the doors
  drawn open; it is not four stacked panels, and the section headers
  (`READINESS BLOCKERS`, `RELATIONSHIPS`, `SOURCES`, `IMPLEMENTATION CHECKS`)
  are expansion headers, not panel titles.
- **R09.2 — exactly one expansion open at a time.** See the defect note below:
  this board deliberately violates R09.2 as a drawing convention.
- **R09.3 — auto-open is reserved for readiness, and only when blocked.** This
  claim *is* blocked, so readiness is the one expansion the engine may open
  without being asked. Relationships, sources and checks must render closed on
  load. This is the single most load-bearing rule on the screen.
- **R09.4 — relationships is one section with three directions**, in the fixed
  order governed by → depends on → depended on by. The board renders exactly
  that order (`25R-0` → `260-0` → `26O-0`), with the count beside the direction
  label, not beside the section.
- **R09.5 — sources is its own expansion**, split out of relationships. The
  board gives it its own header and its own count (`270-0`).
- **R09.6 — issues is a screen, and the banner is the only way in.** This board
  carries **no blocked banner**: the blocked state is expressed by the footer
  chip and the tinted readiness ground. See §8 — a lane must not invent a banner
  here, and must not route to the Issues screen from the readiness section.
- **R09.7 — the facet dot is the only project-level health signal in the
  reading view.** Nothing on this card marks its own module as unhealthy; the
  four module rows inside readiness name *other* modules and are not module
  health markers.
- **R09.8 — eight demotions.** The checks section shows EXAMINED / COMPARED /
  FOUND only. Adapter, shape, target URI and raw diagnostics are absent by
  design; expected and observed are merged into the single marked step grid.
  "shown via this dependency" does not appear. Each blocker row leads with the
  **title**, not the claim id — the slug appears only inside the dependency
  path, where it is machine identity.
- **R09.9 — no inline dependency map.** The breadcrumb on each blocker row
  carries that row's path; the graph pane draws the real shape. This is why the
  readiness section ends with a `See in claims graph` link (the cluster
  `3B8-0` / `6FQ-0`, in the row `3B6-0` / `6FO-0`) rather than an inline diagram. A drawn tree would duplicate converging ancestors and
  thereby **invent blockers that do not exist**.
- **R08.1 — five severities, two hues, no third hue.** The screen introduces no
  colour outside the three status hues: blocked red for the blocked chip, the
  module counts, the target slugs, the step-1 chip and the Mismatch verdict;
  draft amber for the DRAFT relationship badges; lock green for the LOCKED chip
  and the LOCKED relationship badges. The navigation navy is used only for
  links and "Show N more".
- **R08.2 — a lint and a ledger finding must never read alike.** Not depicted
  here (no integrity findings on this claim), but it constrains §8's
  approval-record states.
- **R-F.1 — four chips in a fixed order, then the comment count hard right.**
  The footer strip reads `Blocked 23 blockers` · `13 relationships` ·
  `5 sources` · `1 check failing`, then a spacer (`1VH-0`, flex-grow 1), then
  the comment count. Every chip is **a noun and a count**; none is a score.
- **R-F.2 — chip states: closed, open, blocked; the blocked chip always leads
  row one.** Honoured on mobile row one. See defects for the two deviations.
- **R-F.3 — the comment count is one pill with one variant.** Desktop: passive,
  `--color-faint` bubble and label. This card's count is `0`, the zero-thread
  case.
- **R-F.4 — mobile footer: two rows, and a pill never splits.** Row one is the
  blocked chip left and the comment count right; row two is the three remaining
  chips as whole pills.
- **R-I.3 — the chip is the component; the footer's two rows are composition.**
  Binds everywhere this screen reasons about the footer. The detail chip
  (`377-0` section I3, `7N2-0`) is one component with three variants; the
  desktop strip's single line and the mobile strip's two rows are two
  *arrangements* of it. That split is why M5 (the two-row stack) and M6
  (plain text → pill chips) are two separate rules rather than one, and why P5
  and P6 are recorded as **chip-variant** defects — a wrong border or a wrong
  label weight is a fault in the component, and no change to the footer's row
  count can fix or excuse it. It is also the rule open decision 3 leans on: a
  screen may change the arrangement without redefining the chip.
- **R-H.2 — mobile claim header: status chip above the title**, order status →
  claim → id.
- **R-H.3 — the coverage strip never wraps.** 13 steps at 24 × 28 with a 3px gap
  inside a 358px gutter.
- **R-I.1 — the blocker row's dependency path stacks on mobile, never drops.**
- **R-I.2 — the relationship row becomes two lines with a right-ranged badge.**
- **R-I.0 / R-H.0 — same content, same vocabulary, same order at every width.**
  The notes strip says the same thing in its own words: *every width says the
  same four module names, the same four counts, and the same footer vocabulary
  in the same order.*
- **R00.0 — the components board is the source of truth.** Where this screen
  disagrees with `377-0`, the component board wins and the screen is flagged.
  Three such disagreements are recorded under Paper defects.

### Every note on the band and the notes strip, paraphrased with intent

**Band `6BY-0`.** Title `06 · CLAIM — BLOCKED ACROSS FOUR MODULES` in
`--color-accent`, 13/16 weight 600 tracking `+0.08em`; sub-label *"four designs
of one screen · light pair left, dark pair right"* in `--color-faint` 12/16.
Intent: the four artboards are one design at four renderings, not four designs.
A lane must not treat the mobile boards as a separate component set.

**Notes column 1 — WHAT THIS BOARD ANSWERS** (`7E6-0`, eyebrow
`--color-muted` 11/14 weight 600 tracking `+0.08em`; body `--color-muted` 13/20
sans). Paraphrase: the board exists to answer *which module do I chase first*.
Grouping is by the module that owns the fix, matching the Issues screen; each
group leads with its nearest hop distance because depth, not severity, measures
distance-to-fix. Only the nearest module opens. Every width repeats the same
four module names, the same four counts and the same footer vocabulary in the
same order.

**Notes column 2 — ALL FOUR DOORS ARE OPEN AT ONCE — NO REAL SCREEN LOOKS LIKE
THIS** (eyebrow in `--color-blocked`). Paraphrase, and it is the board
disowning itself: group 05 established that the footer strip has four doors and
that opening one closes the others. This board draws all four open on all four
widths **as a spec convenience**, so the four sections can be compared side by
side. *It is not a state the product can reach.* In the built viewer, opening
any one of the four chevrons collapses the other three. The board was
deliberately left un-redesigned and the contradiction recorded instead.

**Notes column 3 — WHAT MOBILE DOES DIFFERENTLY.** Paraphrase: nothing is
dropped; six things change shape.
1. The 13-step coverage strip stays whole and on one line — chips narrow 32 → 24
   and the gap 5 → 3 — so the gap at step 1 is still readable at a glance;
   wrapping to two rows would destroy that reading.
2. EXAMINED / COMPARED / FOUND lose their 92px label column and stack
   label-above-value, because 92px of a 358px gutter would leave the prose too
   narrow.
3. Relationship rows go from four columns to two lines: title on line one,
   `module · facet` and the status word on line two.
4. Source provenance (`cited once`, `third-party`) moves from a right-hand
   column onto the sub-line, right-aligned.
5. The footer strip becomes two rows of pills instead of one row of plain text —
   blocked chip and comment count on row one, the three neutral chips on row
   two — matching the mobile footer component used from group 02 onward.
6. A blocker row stacks: the title wraps above its hop pill, and the dependency
   path puts the source slug on its own line with the target slug below it
   behind a leading chevron, rather than running source › target across one
   line.
All three "Show N more" affordances survive unchanged — 9 in this module, 7
relationships, 2 sources — *since they are what keeps a 23-item list readable at
any width*. Both mobile boards are 390px-wide fit-content columns, not 390 × 844
device frames, and carry no app bar: this is a spec board, not a screen.

**Board header caption `1WG-0` / `6CI-0`.** Eyebrow `FULL CLAIM · LOCKED ·
BLOCKED ACROSS FOUR MODULES · ONE CHECK FAILING` (`--color-faint` /
`--color-dark-faint`, sans 11/14 weight 600 tracking `+0.09em`); intro in serif
16/25 on a 760px measure, `--color-muted` / `--color-dark-muted`: *"23 blockers
is too many to list flat, so they group by the module that owns the fix — the
same rule the Issues screen uses. Depth, not severity, is what the reviewer
needs here."* These two nodes are **board chrome, not screen content** — they
must not be implemented.

### Where the board contradicts its own notes — recorded as Paper defects

Approval is not proof. Full list in §9; summarised here because they are
intent-level:

- The board draws all four expansions open, which R09.2 forbids. The notes strip
  admits this in its own eyebrow. **A lane implements R09.2, not the picture.**
- The desktop footer strip (`1VC-0`) is plain text with chevrons, while the
  mobile strip (`6LV-0`) uses the pill chips of component I3. Both are on the
  same board. The desktop form is consistent with boards 05/07 and is not itself
  a defect, but the chip-state vocabulary of R-F.2 is only expressible on
  mobile, so a lane must not read R-F.2's "closed / open / blocked" onto the
  desktop strip.
- Two LOCKED relationship rows carry a **blocked-red** lifecycle dot against a
  green LOCKED badge (`26G-0`, `26U-0`), contradicting R-I.2's "dot in the
  lifecycle colour".
- The mobile LOCKED chip is 10/13 (`6K4-0`) where R-H.2 fixes it at 11/14.
- The mobile footer's three row-two chips are drawn with **up chevrons** (open)
  but **closed-variant** colour, and `1 check failing` carries a border where
  R-F.2's blocked variant has none.

---

## 3. Layout

### Desktop — 1220 board width, 1140 content width

| Property | Value | Node |
|---|---|---|
| Artboard width | `1220px`, `height: fit-content` | `1SY-0` (dark twin `6CH-0`) |
| Artboard padding | `40px` all round | `1SY-0` / `6CH-0` |
| Artboard gap (caption → card) | `20px` | `1SY-0` / `6CH-0` |
| Artboard ground | `#EFF1F4` = `--color-paper`; dark `--color-dark-paper` `#0D1117` | `1SY-0` / `6CH-0` |
| Content column width | `1140px` (1220 − 2 × 40) | `1WG-0`, `1SZ-0` |
| Card outer | `border-radius: 12px`, `border: 1px solid var(--color-border)`, `overflow: clip`, ground `--color-card` | `1SZ-0` |
| Card outer, dark | same geometry; ground `--color-dark-card`, border `--color-dark-border` | `6CL-0` |
| Card inner width | `1138px` (1140 − 2 × 1px border) | `1W1-0`, `2YR-0`, `25M-0`, `26Z-0`, `2S1-0` |
| Section inline padding | `32px` on every section | `1W1-0`, `2YR-0`, `25M-0`, `26Z-0`, `2S1-0` |
| Row width inside a section | `1074px` (1138 − 2 × 32) | `1W7-0`, `1VC-0`, `2YS-0`, `25N-0`, `270-0`, `2S2-0` |
| Head section padding | `30px` top / `22px` bottom, inline 32 | `1W1-0` / `6CM-0` |
| Head section gap | `16px` | `1W1-0` / `6CM-0` |
| **Reading measure** | `max-width: 760px` | `1W2-0` (prose block), `1WH-0` (board intro) |
| Footer strip | `margin-inline: 32px`, `padding-block: 16px`, `gap: 20px`, `border-top: 1px solid #E8ECF1` (dark `#212934`) | `1VC-0` / `6D1-0` |
| Readiness section | `18px` top / `22px` bottom padding, `gap: 12px`, inline 32 | `2YR-0` / `6DR-0` |
| Relationships section | `20px` top / `24px` bottom, `gap: 14px`, inline 32 | `25M-0` / `6FW-0` |
| Sources section | `20px` top / `24px` bottom, `gap: 13px`, inline 32 | `26Z-0` / `6H8-0` |
| Checks section | `18px` top / `24px` bottom, `gap: 16px`, inline 32 | `2S1-0` / `6I6-0` |
| Blocker-row list indent | `padding-left: 24px` | `2Z4-0` / `6E4-0` |
| Relationship-row indent | `padding-left: 19px` | rows under `25M-0` |
| Source ref-number column | `width: 26px`, gap `12px` to the body | `275-0`, `27D-0`, `27L-0` |
| Checks label column | `width: 92px`, gap `20px` to the value | `2SE-0`, `2SH-0`, `2SK-0` first children |
| Checks prose measure | `max-width: 700px` | `2SB-0` / `6IG-0` |
| Blocker hop-pill column | `width: 112px`, right-ranged (`justify-content: end`) | rows in `2Z4-0` |
| Relationship badge column | `width: 58px`, `text-align: right` | `25Z-0`, `269-0`, `26E-0`, `26J-0`, `26X-0` |
| Blocked-notice band | **absent** — this screen has none | — |

**Rails, TOC, sticky and scroll regions.** These boards draw the **claim card
only**. There is no shell, no left facet TOC, no right rail, no header, and
therefore no sticky region and no scroll region on this board. The rail
(`--container-rail` 268px) and facet TOC (`--container-toc` 244px) widths come
from the group-02 reading-view boards and are not re-measured here; a lane
placing this card inside the reading view takes those from the 02 spec. The
board's 1140px content column is the claim card at the narrow-desktop tier, not
a page width — see §5 for the breakpoint mapping.

### Mobile — 390

| Property | Value | Node |
|---|---|---|
| Artboard width | `390px`, `height: fit-content` (2447 tall) | `6JS-0` / `72F-0` |
| Artboard padding | none; the card is edge-to-edge | `6JS-0` / `72F-0` |
| Artboard ground | `--color-paper`; dark `--color-dark-paper` | `6JS-0` / `72F-0` |
| Card | `border-top` + `border-bottom` `1px solid var(--color-border)` only — **no side borders, no radius** | `6JX-0` / `72J-0` |
| Gutter | `16px` inline on every section → **358px content width** | `6JZ-0`, `6KA-0`, `6NV-0`, `6S8-0`, `6WB-0`, `6Z4-0` |
| Head padding | `20px` top, inline 16, gap `9px` | `6JZ-0` |
| Head inner gap (title → id) | `6px` | `6K5-0` |
| Prose block | `14px` top padding, `gap: 12px`, inline 16 | `6KA-0` / `72T-0` |
| Footer strip | `margin-inline: 16px`, `13px` top / `16px` bottom padding, `gap: 8px` between the two rows, `border-top: 1px solid #E8ECF1` (dark `#212934`) | `6LV-0` / `72Y-0` |
| Footer row 1 gap | `10px` (chip, flexible spacer `min-width: 4px`, comment count) | `6LW-0` |
| Footer row 2 gap | `8px` between the three pills | `6M8-0` |
| Section padding | `16px` uniform on all four sections | `6NV-0`, `6S8-0`, `6WB-0`, `6Z4-0` |
| Section gaps | readiness `12px`, relationships `12px`, sources `13px`, checks `14px` | `6NV-0`, `6S8-0`, `6WB-0`, `6Z4-0` |
| Blocker-row list indent | `padding-left: 22px` (desktop 24) | `6OO-0` |
| Relationship-row indent | `padding-left: 19px` (unchanged from desktop) | `6TE-0` |
| Source ref column | `width: 26px`, gap `10px` (desktop 12) | `6WW-0`, `6X9-0`, `6XG-0` |
| Checks rows | label column removed; `gap: 3px` label→value, `padding-block: 11px` | `6ZV-0`, `6ZY-0` |
| FOUND row | `12px` top / `10px` bottom padding, `gap: 9px` | `70F-0` |
| Coverage strip fit | 13 × 24px + 12 × 3px = 348px inside a 358px gutter → 10px spare | `70I-0`…`716-0` |

---

## 4. Components on this screen

Light value first, dark value second. Token names are from `tokens.md`; the hex
is printed beside each so a lane can verify without a second lookup. Node ids in
the right-hand column.

### 4.1 Claim head — title, id, status chip

| Property | Light | Dark | Node |
|---|---|---|---|
| Title font | sans (Inter) 20px / 26px, weight 600, tracking `-0.01em` | same | `1WD-0` head text / `6CM-0` |
| Title colour | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `1WD-0` / `6CM-0` |
| Title → id gap | `7px` | same | `1WD-0` |
| Claim id font | mono (IBM Plex Mono) 12px / 16px, weight 400 | same | `1WD-0` |
| Claim id colour | `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `1WD-0` / `6CM-0` |
| Head row gap (identity ↔ chip) | `16px`, `align-items: start` | same | `1W7-0` |
| **LOCKED chip** ground | `--color-locked-bg` `rgb(44 107 82 / 10%)` | `--color-dark-locked-bg` `rgb(99 190 154 / 13%)` | `1W8-0` / `6CM-0` |
| LOCKED chip radius | `999px` (`--radius-pill`) | same | `1W8-0` |
| LOCKED chip padding | `4px` block, `8px` left, `10px` right, gap `5px` | same | `1W8-0` |
| LOCKED chip label | sans 11px / 14px, weight 600, tracking `+0.05em` | same | `1W8-0` |
| LOCKED chip label colour | `--color-locked` `#2C6B52` (spelled as a raw hex on the light board) | `--color-dark-locked` `#63BE9A` | `1W8-0` / `6CM-0` |
| Padlock glyph | 12 × 12, stroke-width `2.4`, `stroke-linecap: round`, in the lock colour | same | `1W8-0` |

### 4.2 Claim prose and the truncation control

| Property | Light | Dark | Node |
|---|---|---|---|
| Body font | **serif** (Source Serif 4) 17px / 28px, weight 400 | same | `1W6-0` / `6CW-0` |
| Body colour | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `1W6-0` / `6CW-0` |
| Measure | `max-width: 760px` | same | `1W2-0` / `6CW-0` |
| Paragraph gap | `16px` | same | `1W2-0` |
| `…more` label | sans 14px / 28px, weight 500, `flex-shrink: 0` | same | `1W3-0` |
| `…more` colour | `--color-accent` `#1C4E8C` | `--color-dark-accent` `#6AA6E8` | `1W3-0` / `6CW-0` |
| `…more` gap from text | `7px`, `align-items: baseline` | same | `1W3-0` |

### 4.3 Footer metadata strip — desktop

One row, four doors, then the count hard right. `align-items: center`,
`gap: 20px`.

| Property | Light | Dark | Node |
|---|---|---|---|
| Top hairline | `1px solid #E8ECF1` (no token; nearest is `--color-border` `#DDE2E9`) | `1px solid #212934` (no token; nearest `--color-dark-border` `#242C38`) | `1VC-0` / `6D1-0` |
| Vertical padding | `16px` | same | `1VC-0` / `6D1-0` |
| Blocked dot | 7 × 7, `border-radius: 999px`, `--color-blocked` `#9E3B36` | `--color-dark-blocked` `#EC8A83` | `1W0-0` / `6D1-0` |
| `Blocked` label | sans 13px / 16px, weight 600, `--color-blocked` | `--color-dark-blocked` | `1VZ-0` / `6D1-0` |
| `23 blockers` count | sans 13px / 16px, weight 400, `--color-faint` | `--color-dark-faint` | `1VY-0` / `6D1-0` |
| Group gap (dot → label → count → chevron) | `7px` | same | `1VV-0` |
| Chevron | 12 × 12, stroke-width `2.4`, **up** (open), in the group's own colour | same | `1VW-0` |
| Divider rule | 1 × 12, `--color-border` | `--color-dark-border` | `1VU-0` / `6D1-0` |
| `13 relationships` / `5 sources` | sans 13px / 16px, weight 500, `--color-accent` | `--color-dark-accent` | `1VT-0`, `1VP-0` / `6D1-0` |
| Their chevron gap | `5px` | same | `1VQ-0`, `1VM-0` |
| `1 check failing` | 7px dot + sans 13/16 weight 600, both `--color-blocked`, gap `6px` | `--color-dark-blocked` | `2JR-0` / `6D1-0` |
| Spacer | `flex-grow: 1` (pushes the count hard right) | same | `1VH-0` / `6DM-0` |
| Comment icon | 14 × 14 speech bubble, stroke-width `2`, `--color-faint` | `--color-dark-faint` | `1VF-0` / `6D1-0` |
| Comment count `0` | sans 13px / 16px, weight 400, `--color-faint` | `--color-dark-faint` | `1VE-0` / `6D1-0` |
| Icon → count gap | `6px` | same | `1VD-0` |

**States visible.** All four doors in their **open** state (up chevron). The
closed state is not drawn on this board; take it from `377-0` section A. The
blocked door differs from the other three by carrying a leading dot and taking
weight 600; that is the only distinction the desktop strip makes.

### 4.4 Readiness expansion — section header, module rows, blocker rows

Node pairs below are read **light / dark** and are true twins: `1SY-0`'s
readiness section and `6CH-0`'s are node-for-node mirrors, child for child, so
every row names the node that actually carries the value on each board rather
than a container that happens to enclose it.

| Property | Light | Dark | Node (light / dark) |
|---|---|---|---|
| Section ground | `#FBF7F7` — a blocked tint over card; **no token** | `var(--color-dark-blocked-surface)` `#1D1618` | `2YR-0` / `6DR-0` |
| Section top hairline | `1px solid #E8ECF1` | `1px solid #212934` | `2YR-0` / `6DR-0` |
| Section padding | `18px` top / `22px` bottom, `32px` inline, gap `12px` | same | `2YR-0` / `6DR-0` |
| Header row | `display: flex`, align center, gap `12px` | same | `2YS-0` / `6DS-0` |
| Eyebrow `READINESS BLOCKERS` | sans 11px / 14px, weight 600, tracking `+0.08em`; drawn as the literal `#9E3B36`, which *is* `--color-blocked` (P10) | `var(--color-dark-blocked)` | `2YT-0` / `6DT-0` |
| Eyebrow rule | `height: 1px`, `flex-grow: 1`, `#EBD9D8` (no token) | `var(--color-dark-blocked-hairline)` `#3A2A2B` | `2YU-0` / `6DU-0` |
| `23 across 4 modules` | sans 12px / 16px, weight 400; literal `#6E7C8E`, which *is* `--color-faint` | `var(--color-dark-faint)` | `2YV-0` / `6DV-0` |
| **Module row (expanded)** — row box | align center, gap `12px`, `padding-block: 2px` | same | `2YW-0` / `6DW-0` |
| Module row chevron | 12 × 12 down-chevron, stroke `2.4`, `--color-muted`, `flex-shrink: 0` | `--color-dark-muted` | `2YX-0` / `6DX-0` |
| Module name | sans 14px / 18px, weight 600; literal `#101720`, which *is* `--color-ink` | `var(--color-dark-ink)` | `2YZ-0` / `6DZ-0` |
| `nearest N hop(s)` | sans 12px / 16px, weight 400; literal `#6E7C8E` = `--color-faint` | `var(--color-dark-faint)` | `2Z0-0` / `6E0-0` |
| Module row spacer | `flex-grow: 1` — the empty rectangle that ranges the count pill hard right | same | `2Z1-0` / `6E1-0` |
| Module count pill | ground `#9E3B361A` (= `--color-blocked` at 10 %), radius `999px` (`--radius-pill`), padding `3px` / `9px` | ground `#EC8A8329` (= `--color-dark-blocked` at ~16 %); **not** the light `#9E3B361A` | `2Z2-0` / `6E2-0` |
| Module count label | sans 11px / 14px, weight 600, `--color-blocked` | `var(--color-dark-blocked)` | the Text inside `2Z2-0` / `6E2-0` |
| **Module row (collapsed)** chevron | 12 × 12 **right**-chevron, `--color-muted` | `--color-dark-muted` | rows under `300-0` / `6EZ-0` |
| Collapsed-rows container | plain `display: flex; flex-direction: column` — carries no colour of its own | same | `300-0` / `6EZ-0` |
| Collapsed row divider + geometry | `1px solid #EBD9D8` (no token), `padding-block: 11px`, gap `12px`, align center | `1px solid #33262A` (no token) — the **dark** twin of `#EBD9D8`; a light hairline here would be a defect, see §9 decision 9 | `301-0`, `309-0`, `30H-0` / `6F0-0`, `6F8-0`, `6FG-0` |
| Blocker-rows container | `display: flex; flex-direction: column; padding-left: 24px` — the 24px indent under the open module, no colour of its own (restated from §3 so `6E4-0` is not mistaken for a blocker row) | same | `2Z4-0` / `6E4-0` |
| **Blocker row** divider | `1px solid #EFE6E5` (no token) | `1px solid #33262A` (no token) | `2Z5-0` / `6E5-0` |
| Blocker row padding / gap | `padding-block: 10px`, gap `14px` | same | `2Z5-0` / `6E5-0` |
| Blocker title | sans 14px / 20px, weight 600, `--color-ink` | `var(--color-dark-ink)` | `2Z5-0` / `6E5-0` |
| Title → path gap | `6px` (the copy column is `flex-grow: 1`, column, gap `6px`) | same | `2Z5-0` / `6E5-0` |
| Source slug chip | ground `--color-paper` `#EFF1F4`, radius `4px` (`--radius-sm`), padding `2px` / `7px`, mono 11px / 14px, `--color-muted` | ground `var(--color-dark-paper)`, text `var(--color-dark-muted)` | `2Z5-0` / `6E5-0` |
| Path chevron | 11 × 11, stroke `2.6`, `--color-faint`, `flex-shrink: 0` | `var(--color-dark-faint)` | `2Z5-0` / `6E5-0` |
| Target slug chip | ground `#9E3B361A`, radius `4px`, padding `2px` / `7px`, mono 11px / 14px, `--color-blocked` | ground `#EC8A8329`, text `var(--color-dark-blocked)` | `2Z5-0` / `6E5-0` |
| Slug row gap / wrap | align center, gap `6px`, `flex-wrap: wrap` | same | `2Z5-0` / `6E5-0` |
| Hop pill | ground `--color-paper`, radius `999px`, padding `3px` / `9px`, sans 11px / 14px weight 500, `--color-muted` | ground `var(--color-dark-paper)`, text `var(--color-dark-muted)` | `2Z5-0` / `6E5-0` |
| Hop pill column | `width: 112px`, `flex-shrink: 0`, `justify-content: end`, `align-items: start` | same | `2Z5-0` / `6E5-0` |
| `Show 9 more in this module` | sans 13px / 16px, weight 500, `--color-accent`; 12px down-chevron; align center, gap `7px`; `padding-top: 12px` above a `1px solid #EFE6E5` rule | `--color-dark-accent`; rule `1px solid #33262A` | `2ZV-0` / `6EV-0` |
| `See in claims graph` — **the row** | align center, `padding-top: 12px`, gap `10px`; a `flex-grow: 1` spacer (the empty serif node, P7) then the link cluster; **no border** | same geometry | `3B6-0` / `6FO-0` |
| `See in claims graph` — **the link cluster** | align center, `flex-shrink: 0`, gap `6px`; 13 × 13 branch glyph stroke `2`; label sans 13px / 16px weight 500. Drawn as the literal `#1C4E8C`, which *is* `--color-accent` (P10) | `var(--color-dark-accent)` on both glyph and label | `3B8-0` / `6FQ-0` |

Two rows above replace what earlier drafts printed as one. The row box
(`3B6-0` / `6FO-0`) owns `padding-top: 12px` and `gap: 10px`; the link cluster
(`3B8-0` / `6FQ-0`) owns `gap: 6px` and nothing else. Quoting `gap 6px` against
the row, or `padding-top 12px` against the cluster, pairs a child on one board
with a parent on the other and yields a geometry that exists on neither.

**States visible.** Expanded module (down chevron, rows revealed), collapsed
module ×3 (right chevron, count only), a truncation control with a count. No
hover, focus or disabled state is drawn on this board.

### 4.5 Relationships expansion

| Property | Light | Dark | Node |
|---|---|---|---|
| Section ground | `#F6F8FA` — **disagrees with** `--color-code-bg` `#F3F5F8` | `#1B212B` = `--color-dark-code-bg`, spelled as a literal | `25M-0` / `6FW-0` |
| Eyebrow `RELATIONSHIPS` | sans 11px / 14px, weight 600, tracking `+0.08em`, `--color-faint` | `--color-dark-faint` | `25O-0` / `6FX-0` |
| Eyebrow rule | 1px `#E3E8EE` (no token) | `#212934` | `25P-0` / `6FX-0` |
| Section count `13` | **mono** 11px / 14px, `--color-faint` | `--color-dark-faint` | `25Q-0` / `6FX-0` |
| Direction label (`GOVERNED BY`, `DEPENDS ON`, `DEPENDED ON BY`) | sans 11px / 14px, weight 600, tracking `+0.06em`, `--color-faint` | `--color-dark-faint` | `25U-0`, `263-0`, `26R-0` |
| Direction glyph | 12 × 12 arrow, stroke `2.4`, `stroke-linejoin: round`, `--color-faint`; up / down / right respectively | `--color-dark-faint` | `25S-0`, `261-0`, `26P-0` |
| Direction count | mono 11px / 14px, `--color-faint` | `--color-dark-faint` | `264-0`, `26S-0` |
| Direction row gap / top padding | gap `7px`, `padding-top: 4px` on the 2nd and 3rd directions | same | `260-0`, `26O-0` |
| Row lifecycle dot | 7 × 7, `border-radius: 999px`, in the lifecycle colour — `--color-draft` `#9A6A16` on DRAFT rows | `--color-dark-draft` `#DDA94E` | `25W-0`, `266-0`, `26B-0` |
| Row title | sans 14px / 18px, weight 400, `--color-accent` | `--color-dark-accent` | `25X-0`, `267-0`, `26C-0` |
| Row `module · facet` | sans 12px / 16px, weight 400, `--color-faint` | `--color-dark-faint` | `25Y-0`, `268-0` |
| Row badge | sans 11px / 14px, weight 600, tracking `+0.05em`, `width: 58px`, `text-align: right` | same | `25Z-0`, `26J-0` |
| Badge colour, DRAFT | `--color-draft` `#9A6A16` | `--color-dark-draft` `#DDA94E` | `25Z-0`, `269-0`, `26E-0` |
| Badge colour, LOCKED | `--color-locked` `#2C6B52` | `--color-dark-locked` `#63BE9A` | `26J-0`, `26X-0` |
| Row gap / indent | gap `10px`, `padding-left: 19px` | same | `25V-0`, `265-0` |
| `Show 7 more` | sans 13px / 16px, weight 500, `--color-accent`, 12px down-chevron, gap `7px`, indent `19px` | `--color-dark-accent` | `26K-0` / `6GU-0` |

**States visible.** Three directions, each with its own count; two lifecycle
states (DRAFT amber, LOCKED green) on rows; one truncation control.

### 4.6 Sources expansion

| Property | Light | Dark | Node |
|---|---|---|---|
| Section ground | none (inherits `--color-card`) | none (inherits `--color-dark-card`) | `26Z-0` / `6H8-0` |
| Eyebrow `SOURCES` | sans 11px / 14px, weight 600, tracking `+0.08em`, `--color-faint` | `--color-dark-faint` | `271-0` / `6H9-0` |
| Eyebrow rule | 1px `#E8ECF1` | `#212934` | `272-0` |
| Section count `5` | mono 11px / 14px, `--color-faint` | `--color-dark-faint` | `273-0` |
| Ref marker `[1]` | mono 12px / 16px, `--color-faint`, `width: 26px`, `padding-top: 2px` | `--color-dark-faint` | `275-0`, `27D-0`, `27L-0` |
| Source title | **serif** 15px / 23px, weight 400, `--color-ink` | `--color-dark-ink` | `277-0`, `27F-0`, `27N-0` |
| Source sub-line | sans 12px / 16px, `--color-faint` | `--color-dark-faint` | `277-0`, `27F-0`, `27N-0` |
| Title → sub-line gap | `3px` | same | `277-0` |
| Provenance word | sans 12px / 16px, `--color-faint`, `padding-top: 3px`, right column | `--color-dark-faint` | `27A-0`, `27I-0`, `27Q-0` |
| Row gap | `12px`; inter-row gap `13px` | same | `274-0`, `26Z-0` |
| `Show 2 more` | sans 13px / 16px, weight 500, `--color-accent`, 12px down-chevron, gap `7px`, after a 26px spacer | `--color-dark-accent` | `27S-0` / `6I1-0` |

**States visible.** Two provenance variants — `cited once` (internal doc) and
`third-party`. No hover state drawn.

### 4.7 Implementation-checks expansion

| Property | Light | Dark | Node |
|---|---|---|---|
| Section ground | `#FBF7F7` — blocked tint, **no token** | `--color-dark-blocked-surface` `#1D1618` | `2S1-0` / `6I6-0` |
| Eyebrow `IMPLEMENTATION CHECKS` | sans 11px / 14px, weight 600, tracking `+0.08em`, `--color-faint` | `--color-dark-faint` | `2S3-0` / `6I7-0` |
| Eyebrow rule | 1px `#EFE6E5` | `#212934` | `2S4-0` |
| Section count `1` | mono 11px / 14px, `--color-faint` | `--color-dark-faint` | `2S5-0` |
| Check title `Step coverage` | sans 16px / 20px, weight 600, `--color-ink` | `--color-dark-ink` | `2S7-0` / `6IB-0` |
| Verdict dot | 7 × 7, `--color-blocked` | `--color-dark-blocked` | `2S8-0` |
| Verdict word `Mismatch` | sans 13px / 16px, weight 600, `--color-blocked`, gap `6px` from dot | `--color-dark-blocked` | `2S8-0` |
| Title row gap | `14px`, `align-items: baseline` | same | `2S6-0` / `6IB-0` |
| Check prose | **serif** 15px / 23px, weight 400, `--color-muted`, `max-width: 700px`, `margin-top: -8px` | `--color-dark-muted` | `2SB-0` / `6IG-0` |
| Detail row label | sans 11px / 14px, weight 600, tracking `+0.06em`, `--color-faint`, `width: 92px` | `--color-dark-faint` | `2SE-0`, `2SH-0`, `2SK-0` |
| Detail row value | **serif** 14px / 22px, weight 400, `--color-muted` | `--color-dark-muted` | `2SE-0`, `2SH-0` |
| Detail row divider / padding | `1px solid #EFE6E5`, `padding-block: 10px`, gap `20px` | `#212934`-family twin | `2SE-0`, `2SH-0` |
| FOUND row padding | `12px` top / `10px` bottom | same | `2SK-0` |
| **Step chip, never-run** | `32 × 29`, radius `5px`, `1.5px` border `--color-blocked`, ground `#9E3B361F` (≈12 % blocked) | border `--color-dark-blocked`, ground `#EC8A832E` | `2SK-0` chip 1 / `6IS-0` |
| Never-run chip label | sans 12px / 16px, weight 600, `--color-blocked` | `--color-dark-blocked` | `2SK-0` / `6IS-0` |
| **Step chip, ordinary** | `32 × 29`, radius `5px`, `1px` border `--color-border`, ground `--color-card` | border `--color-dark-border`, ground `--color-dark-card` | `2SK-0` chips 2–13 / `6IU-0` |
| Ordinary chip label | sans 12px / 16px, weight 400, `--color-muted` | `--color-dark-muted` | `2SK-0` / `6IU-0` |
| Step strip gap | `5px`, `flex-wrap: wrap` | same | `2SK-0` |
| Strip → legend gap | `9px` | same | `2SK-0` |
| Legend `12 of 13 exercised` | **serif** 14px / 18px, `--color-faint` | `--color-dark-faint` | `2SK-0` |
| Legend swatch | `11 × 11`, radius `3px`, `1.5px` border `--color-blocked`, ground `#9E3B361F` | dark twin | `2SK-0` |
| Legend `never run` | sans 12px / 16px, `--color-faint`, gap `6px` from swatch, `14px` from the count | `--color-dark-faint` | `2SK-0` |
| `How this was checked` | 12px **right**-chevron stroke `2.6` + sans 13px / 16px weight 500, `--color-accent`, gap `7px`, `padding-top: 12px` | `--color-dark-accent` | `2TJ-0` / `6IH-0` |

**States visible.** One failing check (`Mismatch`), one never-run step against
twelve exercised ones, and a **collapsed** disclosure (`How this was checked`,
right chevron). The open form of that disclosure is board 06a's subject, not
this one's.

### 4.8 Mobile-only component forms

| Property | Light | Dark | Node |
|---|---|---|---|
| **Status chip (H2)** ground | `#2C6B521A` (= `--color-locked-bg` at ~10 %) | dark twin on `72K-0` | `6K0-0` |
| Status chip padding | `3px` block, `7px` left, `9px` right, gap `5px`, radius `999px` | same | `6K0-0` |
| Status chip padlock | `11 × 11`, stroke `2.4`, `--color-locked` | `--color-dark-locked` | `6K1-0` |
| Status chip label | sans **10px / 13px**, weight 600, tracking `+0.05em`, `--color-locked` | `--color-dark-locked` | `6K4-0` |
| Mobile title | sans 18px / 24px, weight 600, tracking `-0.01em`, `--color-ink` | `--color-dark-ink` | `6K6-0` |
| Mobile claim id | mono 11px / 15px, `--color-faint` | `--color-dark-faint` | `6K7-0` |
| Chip → title gap / title → id gap | `9px` / `6px` | same | `6JZ-0`, `6K5-0` |
| Mobile prose | **serif** 16px / 26px, `--color-ink` | `--color-dark-ink` | `6KB-0` |
| **Footer chip, blocked (row 1)** | `height: 30px`, radius `999px`, `padding-inline: 11px`, gap `7px`, ground `#9E3B3617` (~9 %), **no border** | ground `#EC8A8326` | `6LX-0` / `72Y-0` |
| Blocked chip dot / labels | 7 × 7 dot; `Blocked` sans 12/16 weight 600; `23 blockers` sans 12/16 weight 400; both `--color-blocked` | `--color-dark-blocked` | `6LX-0` |
| Blocked chip chevron | `11 × 11`, stroke `2.6`, `--color-blocked` | `--color-dark-blocked` | `6LX-0` |
| **Footer chip, neutral (row 2)** | `height: 30px`, radius `999px`, `padding-inline: 11px`, gap `6px`, ground `--color-card`, `1px` border `--color-border`, label sans 12/16 weight 500 `--color-muted`, chevron 11px `--color-faint` | ground `--color-dark-card`, border `--color-dark-border`, label `--color-dark-muted`, chevron `--color-dark-faint` | `6M9-0`, `6MD-0` / `72Y-0` |
| **Footer chip, check-failing (row 2)** | ground `#9E3B3617`, `1px` border `#9E3B3640`, label sans 12/16 weight 500 `--color-blocked` | ground `#EC8A8326`, border `#EC8A834D`, label `--color-dark-blocked` | `6MH-0` / `72Y-0` |
| Mobile comment count | 14px bubble stroke `2` + sans 13/16 `--color-faint`, gap `6px`, hard right | `--color-dark-faint` | `6M4-0` / `72Y-0` |
| **Mobile blocker row** | `border-top: 1px solid #EFE6E5`, `padding-block: 11px`, column gap `8px` | `border-top: 1px solid #33262A` — measured on the twin, not inferred | `6OP-0` / `742-0` (second row `74E-0`) |
| Mobile blocker title | sans 14px / 20px, weight 600, `--color-ink`; hop pill right-ranged, `flex-shrink: 0` | `--color-dark-ink` | `6OP-0` |
| Mobile dependency path | stacked: source slug chip, then a row of `11px` chevron (`margin-top: 3px`) + target slug chip; column gap `5px`, row gap `6px` | same | `6OP-0` |
| **Mobile relationship row** | dot 7 × 7 with `margin-top: 6px`; title sans **14px / 19px weight 400** `--color-accent`; line two `module · facet` sans 12/16 `--color-faint` left and badge sans 11/14 weight 600 tracking `+0.05em` right; row gap `10px`, line gap `3px`, line-two gap `8px` | dark twins | `6TE-0` |
| **Mobile source row** | ref `[n]` mono 11px / 20px `--color-faint` `width: 26px`; title serif 15px / 20px `--color-ink`; sub-line sans 12/16 `--color-faint`; provenance sans **11px** / 16px `--color-faint` right-ranged on the sub-line; gaps `10px` / `2px` / `10px` | dark twins | `6XA-0` |
| **Mobile check detail row** | label sans 11/14 weight 600 tracking `+0.06em` `--color-faint` **above** value serif 14px / 21px `--color-muted`; gap `3px`; `padding-block: 11px`; divider `1px #EFE6E5` | dark twins | `6ZV-0`, `6ZY-0` |
| **Mobile step chip, never-run** | `24 × 28`, radius `5px`, `1.5px` border `--color-blocked`, ground `#9E3B361F` | border `--color-dark-blocked` | `70I-0` |
| **Mobile step chip, ordinary** | `24 × 28`, radius `5px`, `1px` border `--color-border`, ground `--color-card` | dark twins | `70K-0` |
| Mobile check prose | serif 15px / 23px `--color-muted`, `margin-top: -6px` | `--color-dark-muted` | `6ZE-0` |
| Mobile `See in claims graph` | gap `6px`, `padding-top: 6px`, align center, no border; 13px glyph + sans 13/16 weight 500 `--color-accent`. One node, not two: the mobile form has no spacer, so the desktop row/cluster split of §4.4 does not apply | `var(--color-dark-accent)` on glyph and label | `6RN-0` / `75I-0` (label `75N-0`) |
| Mobile collapsed module row | `padding-top: 12px`, gap `10px`, align center, `border-top: 1px solid #EBD9D8` | `border-top: 1px solid #33262A` — the dark hairline, measured; **not** `#EBD9D8` | `6Q2-0` / `74U-0` (the other two: `752-0`, `75A-0`) |

### Count of measured values

**Desktop (boards `1SY-0` + `6CH-0`): 122 distinct measured values** across
§3 and §4.1–§4.7 (each cell in those tables is one measurement, read via
`get_computed_styles` or `get_jsx` on the node named beside it; §4.4's
blocker-rows-container row restates §3's and is not counted twice). That is
well above the 40-value floor. Mobile (`6JS-0` + `72F-0`) adds a further **61**
measured values in §3's mobile table and §4.8, for **183** in total.

---

## 5. Mobile rules

Every rule below is an **arrangement** rule and therefore lands in
`@media (max-width: 520px)`, except the two marked as pointer rules. **No 390px
breakpoint is added** (`tokens.md` §8). The 390 boards are canvas widths.

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | Card loses its side borders and its 12px radius; keeps top and bottom hairlines only, edge to edge (`6JX-0`) | `max-width: 520px` | Arrangement. Phone tier. |
| M2 | Gutter drops from 32px to **16px** on every section; content width 358px (`6NV-0`, `6S8-0`, `6WB-0`, `6Z4-0`) | `max-width: 520px` | Arrangement. |
| M3 | Status chip moves **above** the title (R-H.2); order status → claim → id (`6JZ-0`) | `max-width: 520px` | Arrangement. Board rule H2. |
| M4 | Title drops 20/26 → 18/24; claim id 12/16 mono → 11/15 mono; prose 17/28 → 16/26 (`6K6-0`, `6K7-0`, `6KB-0`) | `max-width: 520px` | Arrangement (type step, not a touch concern). |
| M5 | Footer strip becomes **two rows**: blocked chip + comment count on row one, the three neutral chips on row two; pills never split (R-F.4, R-I.3) (`6LV-0`) | `max-width: 520px`, written **after** `style.css:4735`'s `max-width: 560px` block | `.claim-footer__counts` is already touched at `style.css:4728` (860 tier) and `:4743` (560 tier); a 520 rule declared earlier in the file would lose. `tokens.md` §Disagreements 16. |
| M6 | Footer doors change from plain text to **pill chips** (component I3): 30px tall, `--radius-pill`, 11px inline padding (`6M9-0`) | `max-width: 520px` | Arrangement, and only arrangement — R-I.3. The chip's geometry and variants are fixed by `7N2-0`; the tier decides whether the doors are set as chips or as a text row, never what a chip is. |
| M7 | Blocker row **stacks**: title wraps above its hop pill; the dependency path puts the source slug on its own line with the target slug below behind a leading chevron (R-I.1) (`6OP-0`). The path is never dropped. | `max-width: 520px` | Arrangement. The engine already has `@media (max-width: 520px){.claim-readiness-blocker{grid-template-columns:1fr}}` at `style.css:1263–1270` — this is the rule to extend, not replace. |
| M8 | Relationship row: four columns → **two lines**, badge right-ranged with no fixed slot (R-I.2) (`6TE-0`) | `max-width: 520px` | Arrangement. |
| M9 | Source provenance moves from a right column onto the sub-line, right-aligned, and drops 12px → 11px (`6XA-0`) | `max-width: 520px` | Arrangement. |
| M10 | EXAMINED / COMPARED / FOUND lose the 92px label column and stack label-above-value (`6ZV-0`, `6ZY-0`) | `max-width: 520px` | Arrangement. 92px of a 358px gutter would starve the prose. |
| M11 | Coverage step chip goes 32 × 29 → **24 × 28**, gap 5 → **3px**; **the strip must not wrap** at any width in the tier (R-H.3) (`70I-0`, `70K-0`) | `max-width: 520px` | The 358px fit was measured at 400px; 520 is where the engine expresses it and the no-wrap invariant must hold across the whole tier. |
| M12 | Blocker-row list indent 24px → **22px** (`6OO-0`) | `max-width: 520px` | Arrangement. |
| M13 | Every interactive target on the card — the four footer chips, the module disclosure rows, each "Show N more", `How this was checked`, `See in claims graph` — needs a **≥44px hit area**. The chips are drawn 30px tall, so the extra height is padding, not a size change. | `@media (pointer: coarse)` | Target size is a pointer question, not a width question. The engine already sets `.comment-chip{min-height:44px}` in the first coarse block (`style.css:1848–1860`). |
| M14 | No hover affordance may be the only signal for any state on this card (the chevron direction and the chip's own colour must carry it). | `@media (pointer: coarse)` | Hover-less affordance is a pointer question. |

**Not hidden, not dropped.** Nothing on this screen is hidden at 390. All four
sections, all four module rows, all three "Show N more" controls, both
provenance words, the whole 13-step strip and the full dependency path survive.

**Bottom sheets.** This screen has **none**. Per R-J.1 there is one sheet shell
in the product and this card introduces no body for it. The comment count here
is `0` and, at 390, R-F.3 makes it the control that opens the comment sheet —
but the sheet itself is group 14's spec, not this one's. A lane must not build a
second shell for a blocker list, a module list or a check detail; if any of
those ever needs a sheet, it is a new **body** in the existing shell (R-J.1).

**Which 390-board rule maps to which engine tier, restated for the lane:**
arrangement → `max-width: 520px`; target size and hover-less affordance →
`(pointer: coarse)`. The 1220-wide desktop boards themselves narrow inside
`@media (min-width: 861px) and (max-width: 1180px)` (`style.css:4582`); the
widest tier is `@media (min-width: 1181px)` (`style.css:3466`).

---

## 6. Footer vocabulary

The claim's metadata strip is the only footer/meta row on this screen. There is
**no evidence footer, no freshness line and no elapsed-time wording on this
board** — those belong to the right rail, which these boards do not draw (R10.1
binds boards 02, 03, 13, 14).

### Desktop, left to right, quoted from `1VC-0` / `6D1-0`

1. `Blocked` — preceded by a 7px blocked dot.
2. `23 blockers`
3. *(1 × 12 divider rule)*
4. `13 relationships`
5. `5 sources`
6. `1 check failing` — preceded by a 7px blocked dot.
7. *(flex spacer)*
8. *(speech-bubble glyph)* `0`

### Mobile, quoted from `6LV-0` / `72Y-0`

- **Row one:** `Blocked` `23 blockers` (one pill, left) … *(spacer)* …
  *(speech-bubble glyph)* `0` (hard right).
- **Row two:** `13 relationships` · `5 sources` · `1 check failing`.

The order is identical at both widths; only the arrangement changes (R-H.0).

### Other counted strings on the screen, quoted

| String | Where | Node |
|---|---|---|
| `23 across 4 modules` | readiness section header, right | `2YV-0` / `6DV-0` |
| `nearest 1 hop` / `nearest 1 hop` / `nearest 2 hops` / `nearest 3 hops` | module rows, in board order Capability support → Verification → Audience boundary → Startup readiness | `2Z0-0`, then inside `301-0`, `309-0`, `30H-0` (dark: `6E0-0`, then inside `6F0-0`, `6F8-0`, `6FG-0`) |
| `11` / `7` / `3` / `2` | module count pills, same order | `2Z2-0`, then inside `301-0`, `309-0`, `30H-0` (dark: `6E2-0`, then inside `6F0-0`, `6F8-0`, `6FG-0`) |
| `direct · 1 hop` | blocker hop pill | `2Z5-0`, `2ZI-0` |
| `Show 9 more in this module` | readiness truncation | `2ZV-0` |
| `See in claims graph` | readiness footer, hard right | `3B8-0` / `6FQ-0` |
| `13` / `10` / `2` | relationships section count, then DEPENDS ON and DEPENDED ON BY direction counts | `25Q-0`, `264-0`, `26S-0` |
| `GOVERNED BY` / `DEPENDS ON` / `DEPENDED ON BY` | direction labels, fixed order | `25U-0`, `263-0`, `26R-0` |
| `DRAFT` / `LOCKED` | relationship lifecycle badges | `25Z-0`, `26J-0` |
| `Show 7 more` | relationships truncation | `26L-0` |
| `5` | sources section count | `273-0` |
| `[1]` `[2]` `[3]` | source ref markers | `275-0`, `27D-0`, `27L-0` |
| `cited once` / `third-party` | source provenance | `27A-0`, `27Q-0` |
| `Show 2 more` | sources truncation | `27U-0` |
| `1` | implementation-checks section count | `2S5-0` |
| `Step coverage` | check title | `2S7-0` |
| `Mismatch` | check verdict | `2S8-0` |
| `EXAMINED` / `COMPARED` / `FOUND` | check detail labels, fixed order | `2SE-0`, `2SH-0`, `2SK-0` |
| `12 of 13 exercised` | step-strip legend | `2SK-0` |
| `never run` | step-strip legend key | `2SK-0` |
| `How this was checked` | collapsed disclosure | `2TJ-0` |

Counting convention on this screen: a count is always **a numeral followed by a
noun**, singular or plural as the numeral requires (`5 sources`, `1 check
failing`), and a bare numeral only where a label already names the thing (the
`13` beside `RELATIONSHIPS`, the `11` in a module pill). Nothing on the screen
is a percentage, a ratio or a score, except `12 of 13 exercised`, which is a
coverage statement in the check's own legend and is deliberately spelled as a
sentence rather than `92%`.

---

## 7. Code address

All paths relative to the worktree root
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

### 7.0 A warning about line numbers in these two files

`style.css` and `graph.css` are **being edited concurrently** in this worktree
by the engine-fonts lane (`git status` shows both modified, together with
`internal/render/engine_fonts.go` and five new `.woff2` files; `style.css` has
grown from 4782 to 4870 lines during this spec's own measurement pass). Every
address below was re-derived against the working-tree state at
`md5 426425769517a4b28ddaaf5053cf98f4` (`style.css`, 4870 lines) and
`graph.css` at 1186 lines, **not** against `HEAD` (`3ac8844`).

**The marker comment is the durable address; the line number is a convenience.**
A lane that finds a line number off by a few dozen should grep the quoted marker
rather than assume this file is wrong.

The JS and HTML addresses in §7.3 and the test addresses in §7.5 are against
files this worktree has **not** modified, and are stable. §7.4 is *nearly* so,
with one exception a lane must know about: `internal/render/render.go` is also
modified in this worktree (` M` in `git status`, alongside `engine_fonts.go`,
the deleted `geist_fonts.go` and the five new `.woff2` files). Its two §7.4 rows
— `Render` at `:411` and `stripOverviewIDs` at `:1163–1208` — were re-derived
against the **working tree**, not `HEAD` (`3ac8844`), exactly as the two CSS
files were. Every other §7.4 address is against an unmodified file.

### 7.1 `internal/render/viewer/template/style.css`

| What | Marker comment / selector (quoted) | Lines |
|---|---|---|
| The palette; light-first `:root` | `/* THE palette. One unconditional :root, light-first: the values below are what` | `internal/render/viewer/template/style.css:49` |
| Dark re-point, explicit toggle | `@media screen {` → `html[data-theme="dark"]` | `internal/render/viewer/template/style.css:176` |
| Dark re-point, OS | `@media screen and (prefers-color-scheme: dark) {` | `internal/render/viewer/template/style.css:202` |
| Claim footer `<details>` wrapper (first declaration) | `/* .claim-links is the <details> that wraps the whole edges footer` … `.claim-links {` | `internal/render/viewer/template/style.css:877–894` (rule at `:889`) |
| **Readiness panel — the whole component** | `/* Claim readiness is a reviewer-facing work queue. The policy engine remains` … `.claim-readiness {` | `internal/render/viewer/template/style.css:895–1246` (rule at `:898`) |
| Readiness "ready" variant (§8.5's zero-blocker case) | `.claim-readiness--ready {` | `internal/render/viewer/template/style.css:942` |
| Readiness, tablet tier | `@media (max-width: 860px) {` | `internal/render/viewer/template/style.css:1248–1261` |
| **Readiness, phone tier — the blocker-row stack (M7 extends this)** | `@media (max-width: 520px) {` → `.claim-readiness-blocker { grid-template-columns: 1fr; }` | `internal/render/viewer/template/style.css:1263–1270` |
| Footer summary, first declaration | `/* The summary is the disclosure's only click target and reads as a mono` … `.claim-links-summary {` | `internal/render/viewer/template/style.css:1272–1300` (rule at `:1279`) |
| Deep-link auto-open (`:target`) | `/* Deep-link auto-open (decision C9). Landing on #<claim-id> — from the` | `internal/render/viewer/template/style.css:1301–1338` |
| Edge rows (`.claim-edges`) | `/* .claim-edges is a <ul> (see components.EdgesHTMLWithLinks): each` | `internal/render/viewer/template/style.css:1339–1437` |
| **Sources rows** | `/* ---- the sources row (components/sources.go) -------------------------` | `internal/render/viewer/template/style.css:1438–1675` |
| Source-note three-line clamp | `/* The three-line clamp on a source note, and the control that lifts it.` | `internal/render/viewer/template/style.css:1540–1580` |
| Comment chip, first declaration | `/* ---- comments (engine-managed review threads) --------------------------` … `.comment-chip {` | `internal/render/viewer/template/style.css:1677–1761` (rule at `:1690`, `--open` at `:1705`, `--empty` at `:1731`, `-count` at `:1760`) |
| Comment chip, touch target | `/* Coarse pointers (touch) need a >=44px hit target on the chip.` … `@media (pointer: coarse) {` | `internal/render/viewer/template/style.css:1848–1860` (query at `:1852`) |
| Second `(pointer: coarse)` block — comment controls | `@media (pointer: coarse) {` | `internal/render/viewer/template/style.css:2299` |
| Claim head `.k` + status pill, first declaration | `/* The claim head line. v0.4.1 gives it two children instead of loose text:` … `.k {` … `.pill {` | `internal/render/viewer/template/style.css:2365–2419` (`.k` at `:2380`, `.pill` at `:2399`, `.pill.ps` at `:2407`, `.pill.pw` at `:2416`) |
| Narrow tier | `@media (max-width: 640px) {` | `internal/render/viewer/template/style.css:2651` |
| System Record layer begins (everything below re-declares at equal specificity and WINS) | `/* System Record visual language for the generated viewer. */` | `internal/render/viewer/template/style.css:3138` |
| Above-the-fold tier | `@media (min-width: 861px) {` | `internal/render/viewer/template/style.css:3245` |
| Widest tier | `@media (min-width: 1181px) {` | `internal/render/viewer/template/style.css:3466` |
| **Claim head `.k`, the declaration that WINS** | `.k {` (System Record layer) | `internal/render/viewer/template/style.css:3951–3964` |
| **Status pill `.pill` / `.ps` / `.pv` / `.pw`, the declarations that WIN** | `.pill {` … `.pill.pw, .claim-review-pending {` | `internal/render/viewer/template/style.css:4040–4066` (`.ps` at `:4050`, `.pv` at `:4056`, `.pw` at `:4062`) |
| **Claim footer — the declarations that WIN** | `.claim-links {` … `.claim-links-summary {` … `.claim-footer__counts` | `internal/render/viewer/template/style.css:4108–4151` (`.claim-links-summary` at `:4116`, `.claim-footer__counts` at `:4141–4150`) |
| Comment chip, the declaration that WINS | `.comment-chip {` (System Record layer) | `internal/render/viewer/template/style.css:4224–4233` |
| **Narrow-desktop tier — the 1220 boards' tier** | `@media (min-width: 861px) and (max-width: 1180px) {` | `internal/render/viewer/template/style.css:4582–4620` |
| Tablet tier touching `.claim-footer__counts` | `@media (max-width: 860px) {` | `internal/render/viewer/template/style.css:4621–4733` (footer lines `4726–4729`) |
| **`max-width: 560px` tier touching `.claim-footer__counts` — M5 must be written after this** | `@media (max-width: 560px) {` | `internal/render/viewer/template/style.css:4735–4746` |
| Print — the one and only print block | `/* ---- print — THE @media print block ------------------------------------` … `@media print {` | `internal/render/viewer/template/style.css:4748–4870` (query at `:4809`) |

### 7.2 `internal/render/viewer/template/graph.css`

Touched by this screen only through the `See in claims graph` affordance, which
opens the graph pane.

| What | Marker | Lines |
|---|---|---|
| File header; dark-is-base convention | `/* graph.css — chrome and categorical palette for the DossierX claims graph pane.` | `internal/render/viewer/template/graph.css:1–146` |
| `DARK IS THE BASE, light is the override.` | same phrase, verbatim | `internal/render/viewer/template/graph.css:147` |
| Light override + print | ``/* `, print`: paper is light, whatever the OS says.`` | `internal/render/viewer/template/graph.css:233` |
| Pane surface | `.dxg-surface {` | `internal/render/viewer/template/graph.css:342` |
| **The nav trigger — the only graph entry point today** | `/* ---- the nav trigger --------------------------------------------------` … `#dxgOpen {` | `internal/render/viewer/template/graph.css:495–539` (rule at `:515`, open state at `:532`) |

### 7.3 Runtime JavaScript

| What | Address |
|---|---|
| **`renderClaimReadiness` — the whole readiness panel is built here, client-side** | `internal/render/viewer/template/viewer-runtime.js:1512–1614` |
| Readiness route group (the collapsible per-dependency group; nearest analogue of a module row) | `internal/render/viewer/template/viewer-runtime.js:1455–1476` |
| Readiness blocker row (`.claim-readiness-blocker`) | `internal/render/viewer/template/viewer-runtime.js:1444–1454` |
| Readiness hop label (`direct dependency` / `shown via this dependency`) | `internal/render/viewer/template/viewer-runtime.js:1375–1384`, called once outside its own body, at `:1451` (inside `readinessFactRow`, `:1444–1454`) |
| Readiness dependency path disclosure | `internal/render/viewer/template/viewer-runtime.js:1437–1443` |
| Readiness raw diagnostics disclosure (R09.8 demotes this) | `internal/render/viewer/template/viewer-runtime.js:1494–1510` |
| Readiness Mermaid map (R09.9 cuts this) | `internal/render/viewer/template/viewer-runtime.js:1385–1436`, mounted at `:1477–1493` |
| Offline readiness from the graph payload (the `file://` path) | `internal/render/viewer/template/viewer-runtime.js:1616–1631` |
| `renderStatusStrip`, which calls `renderClaimReadiness` on every poll | `internal/render/viewer/template/viewer-runtime.js:1633–1660` |
| Conformance panel auto-open on a failing check | `internal/render/viewer/template/viewer-runtime.js:1265` |
| Graph pane open selector `[data-dxg-open]` | `internal/render/viewer/template/graph-ui.js:68` |
| Graph pane `openPane()` | `internal/render/viewer/template/graph-ui.js:396` |
| Graph → reading-view deep link (`data-dxg-open-claim`) — the **reverse** direction of the board's `See in claims graph` | `internal/render/viewer/template/graph-ui.js:3585–3594` |
| The graph trigger's markup — one button in the sidebar, no per-claim trigger | `internal/render/viewer/template/shell.html:66–73` |

`system-record.js` and `build-order-ui.js` are not touched by this screen.

### 7.4 Go emitters — `internal/render`

| What | Address |
|---|---|
| Top-level render entry | `internal/render/render.go:411` (`func Render`) |
| Overview id-stripping (why at most one `:target` can match) | `internal/render/render.go:1163–1208` — doc comment `1163–1198`, `func stripOverviewIDs` `1199–1208`; the sole call site is `:1119` |
| Claim head + status pill markup | `internal/render/components/card.html:39` |
| Edges footer binding on the card | `internal/render/components/card.html:42` |
| `pillClass` — `ps` / `pv` / `pw` from Status × ReviewPending | `internal/render/components/components.go:243–258` |
| `StatusLabel` — `Locked` / `Draft` / `Review pending` | `internal/render/components/components.go:261–277` |
| `edgesHTML` — the default edges footer | `internal/render/components/components.go:281–345` |
| `EdgesHTMLWithLinks` — governed_by, depends-on, depended-on-by, review_pending | `internal/render/components/components.go:378–520` |
| `governed_by: none` row | `internal/render/components/components.go:407` |
| `depended on by:` row | `internal/render/components/components.go:451–455` |
| `review_pending` row | `internal/render/components/components.go:477–484` |
| `drifted` pill on a linked file | `internal/render/components/components.go:499` |
| The footer `<details>` + `<summary class="claim-links-summary">` | `internal/render/components/components.go:571–573` |
| `writeSourcesRow` — the sources list | `internal/render/components/sources.go:120–161` |
| `writeExternalSource` / `writeInternalSource` | `internal/render/components/sources.go:162–207` / `:208–268` |
| `writeSourceNote` — the clamped note + its toggle | `internal/render/components/sources.go:269–300` |
| `ConformanceHTML` — the whole implementation-checks panel | `internal/render/components/conformance.go:16–54` |
| `writeConformanceCheck` — one check article | `internal/render/components/conformance.go:55–95` |
| `conformanceStateLabel` / `conformancePill` — the verdict words and their pill classes | `internal/render/components/conformance.go:96–121` |
| `writeConformanceValue` / `writeConformanceLine` / `writeConformanceMembers` — the EXAMINED/COMPARED/FOUND analogues today | `internal/render/components/conformance.go:122–152` |
| **Conformance CSS, injected only when a project has results** | `internal/render/conformance_view.go:7–23` (`const conformanceCSS`) |
| Conformance status-freshness fetch guard | `internal/render/conformance_view.go:33–58` |
| `viewerCSSWithConformance` — how that CSS reaches the document | `internal/render/conformance_view.go:60` |
| Depended-on-by reverse index | `internal/render/depended_by_view.go:38` (`buildDependedByLookup`) |
| Target status lookup (the lifecycle colour on a relationship row) | `internal/render/depended_by_view.go:73` (`buildTargetStatusLookup`) |
| Edges override binding | `internal/render/depended_by_view.go:127` (`attachEdgesOverride`) |
| Theme-token consumer assertion (every allowlisted token must have a winning consumer) | `internal/render/theme_tokens_test.go:90` (`func TestEveryAllowlistedTokenHasAConsumer`); the cascade companion is `func TestConsumerReadWinsCascade` at `internal/render/theme_tokens_test.go:572`. The package-level `tokenConsumers` table those two read runs `:519–552`; `:531` is one `hover-bg` row inside it, not the assertion. |

### 7.5 Tests that assert on these selectors today

| Test | Address | What it pins |
|---|---|---|
| `TestReadinessBrowserScaleBudgets` | `viewer-tests/claim_readiness_test.go:98` | `.claim-readiness`, `.claim-readiness-blocker`, `.claim-readiness-route`, `.claim-readiness-map` counts |
| `TestReadinessMapCapNeverCapsTheAuthoritativeList` | `viewer-tests/claim_readiness_test.go:278` | `.claim-readiness-map-limit` never truncates the list |
| `TestStaticReadinessGroupsCompleteFactsAndRendersFocusedMapOnDemand` | `viewer-tests/claim_readiness_test.go:341` | static (`file://`) readiness grouping |
| `TestReadinessTreatsFlagDetailsAsText` | `viewer-tests/claim_readiness_test.go:409` | flag details are escaped, not markup |
| `TestLiveReadinessRefreshesAfterAnUpstreamApproval` | `viewer-tests/claim_readiness_test.go:431` | readiness re-render on poll |
| `TestConformanceStatusFreshnessGuardRejectsInvertedCompletion` | `viewer-tests/conformance_test.go:42` | the fetch guard in `conformance_view.go` |
| `TestConformancePanelVisibleStaticAndRefreshesWhenServed` | `viewer-tests/conformance_test.go:105` | `.claim-conformance-check[data-check-id][data-conformance-state]`, `.claim-conformance-line`, `data-implementation-ready` |
| `TestReadyConformanceStaysInsideCollapsedClaim` | `viewer-tests/component_fit_test.go:98` | `.claim .claim-conformance` nesting and default-closed state |
| `TestStatusStripGroupsBlockersAndStaysCollapsed` | `viewer-tests/component_fit_test.go:67` | the status strip's blocker grouping |
| `TestPhone390SoftMountSmoke` | `viewer-tests/component_fit_test.go:163` | the 390 phone render, `.claim-conformance` present |
| Source-note clamp suite | `viewer-tests/source_note_clamp_test.go:242, 304, 466, 482, 561` | opens `details.claim-links` to reach the sources rows |
| Theme parity | `viewer-tests/theme_parity_test.go:161, 220, 1275` | `.claim-links-summary` in both modes; suppresses `.claim-readiness` / `.claim-conformance` for the pixel baseline |
| Footer summary text | `internal/render/components/components_test.go:516, 549, 585, 771` | the exact `claim-links-summary` string (`"4 links - 2 files - 1 drifted"`) |
| Sources footer summary | `internal/render/components/sources_test.go:43, 59, 73` | `"0 links - 0 files - 2 sources"` |
| Comments absent-footer case | `internal/render/components/comments_test.go:272` | `claim-links`, `claim-links-summary`, `<ul class="claim-edges">` must be absent |
| Graph trigger attribute | `internal/render/graph_render_test.go:91` | `data-dxg-open` |

### 7.6 Gaps — addresses that do not exist yet

A lane must know these are **new construction**, not restyling:

- **Module grouping of blockers.** The engine groups by *representative route*
  (`viewer-runtime.js:1455`), not by *the module that owns the fix*. There is no
  `nearest N hops` label; `readinessHopLabel` (`viewer-runtime.js:1375`) emits
  `direct dependency` / `shown via this dependency`, and R09.8 **cuts** the
  latter phrase outright.
- **`Show N more`.** No such affordance exists anywhere in
  `internal/render/` — the readiness list, the edges list and the sources list
  are all rendered whole.
- **Per-claim `See in claims graph`.** The only graph trigger is
  `shell.html:73`; `[data-dxg-open]` (`graph-ui.js:68`) has exactly one match in
  the document and there is no API for "open the graph focused on claim X". The
  reverse direction exists (`graph-ui.js:3585–3594`).
- **The four-door footer strip.** `.claim-footer__counts`
  (`style.css:4141–4150`) is a set of small bordered count spans inside a single
  `<details>`, not four independent disclosures.
- **`EXAMINED` / `COMPARED` / `FOUND` and the step grid.**
  `writeConformanceLine` (`conformance.go:131`) emits label/value pairs, but
  there is no step-coverage grid component and no "N of M exercised" legend.
- **A serif face.** `--text-body`, the source titles, the check prose and the
  check detail values are all Source Serif 4 on these boards; `style.css`
  declares only `--font-sans` and `--font-mono` (`tokens.md` §7).

---

## 8. States not on the boards

For each, what this spec implies, derived from the rules — a lane must not be
left guessing.

1. **The default closed state of the four doors.** The boards draw all four
   open; R09.2 and R09.3 say the engine must render **readiness open (because
   this claim is blocked) and the other three closed**. The closed chip form is
   in `377-0` section A / R-F.2: `--color-card` ground, `--color-border` outline,
   `--color-muted` label at weight 500, **down** chevron in `--color-faint`.
   Opening any one closes the other three.

2. **A claim that is locked and *not* blocked.** Readiness must **not**
   auto-open (R09.3). The footer's first door loses its dot and its blocked
   colour and becomes a neutral closed chip; the readiness section, when opened,
   loses the `#FBF7F7` / `--color-dark-blocked-surface` tinted ground and takes
   the neutral card ground, because the tint means "this section is the
   problem", not "this section exists".

3. **`review_pending`, all three triggers.** The engine's pill class is `pw`
   (`components.go:251`) and its label is `Review pending`
   (the string literal at `components.go:264`, inside the
   `status == model.StatusLocked && reviewPending` guard that opens
   `StatusLabel` at `:262`), painted `--warn` / `--warn-bg` at
   `style.css:4062–4066` — i.e. the blocked hue. This spec implies the head chip
   swaps the green LOCKED chip for a blocked-hued `REVIEW PENDING` chip at the
   same geometry (`999px`, `4px`/`8px`/`10px` padding, 11/14 weight 600 tracking
   `+0.05em`), with the padlock replaced rather than kept — R-H.2's "padlock"
   is a property of the *locked* chip, not of the chip shape. The three triggers
   (an unlock of a dependency, a drifted linked file, an explicit flag) are one
   visual state, not three: the viewer never distinguishes them in the head, and
   the cause belongs in the readiness expansion where it is already explained.
   The `review_pending` edge row (`components.go:483`) is that explanation's
   current home.

4. **Draft status.** `--color-draft` / `--color-draft-bg` on the head chip, word
   `DRAFT`, **open** padlock (components board section F; corrected 04:30). Note the engine has **no dark override** for
   `--status-draft` or `--status-draft-bg` (`tokens.md` §Disagreements 9), so a
   draft chip on dark renders `#976600` today — that is a defect to fix in the
   engine, not a value to copy.

5. **Zero blockers / ready.** The readiness door reads a neutral `0 blockers`
   or is omitted; `claim-readiness--ready` already exists
   (`viewer-runtime.js:1526`, `style.css:942`). R-F.1 says the chip is a noun
   and a count, so a zero count is still a chip, not an absence — consistent
   with the zero-thread comment chip, which this screen already shows as `0`.

6. **Zero relationships / zero sources / zero checks.** Same rule: the chip
   stays and reads `0 relationships` / `0 sources` / `0 checks`. An expansion
   with a zero count must still open and must show a one-line empty statement in
   serif at the section's own detail size (14/22 desktop, 14/21 mobile), on the
   neutral ground, not a tinted one. It must **not** be hidden — a missing door
   would change the strip's fixed order, which R-F.1 forbids.

7. **The zero-thread comment chip.** Depicted here (`0`) in its desktop passive
   form. Its hover-reveal behaviour exists in the engine
   (`style.css:1731–1759`) and is a pointer-dependent affordance; at
   `(pointer: coarse)` it must be permanently visible with a ≥44px target
   (`style.css:1848–1860`), because there is no hover to reveal it.

8. **A single blocker, a single module.** The readiness header's
   `N across M modules` must degrade to correct singulars (`1 across 1 module`),
   and the single module group must still render as a group — the grouping is
   the rule (notes column 1), not an optimisation that switches off below a
   threshold.

9. **More than four blocking modules.** The board shows exactly four. Nothing in
   the rules caps the count. Only the nearest module opens (notes column 1); the
   remainder are one-line rows in ascending hop order, and the list is subject to
   the same `Show N more` treatment the within-module list gets. No inline map
   is permitted at any count (R09.9).

10. **Equal hop distances.** Two modules at `nearest 1 hop` occur on this very
    board (Capability support and Verification). Only the first opens. The
    implied tie-break, since the board opens the one with the larger count (11 vs
    7): **hop distance ascending, then blocker count descending, then module
    name**. Recorded as an open decision below.

11. **A cycle in the dependency chain.** `graph-ui.js:3582` already reports
    `in a cycle: yes / self-edge / no`. This spec implies a cycle does **not**
    get a new colour on the claim card (R08.1 forbids a third hue); it is a
    property the graph pane draws (`--color-graph-cycle`, engine value
    `#D1201A` / `#F5615C`), and the card's only obligation is that the
    dependency path on a blocker row remains **exactly two slugs** — the claim
    and its unapproved target — so a cycle cannot produce a runaway breadcrumb.

12. **Long titles and long slugs.** Every board node carries
    `overflow-wrap: anywhere`. A blocker title wraps above its hop pill on
    mobile (`6OP-0`) and beside it on desktop, where the pill's 112px column is
    fixed and right-ranged so the lane survives two hop-label lengths. A slug
    chip wraps as a unit inside `flex-wrap: wrap` (`2Z5-0`) and must never be
    truncated with an ellipsis — the slug is machine identity and a truncated id
    is a wrong id.

13. **Integrity / approval-record findings on this claim.** Not depicted. R08.2
    applies: they sort first, take a **red border** rather than an amber one,
    and group under `APPROVAL RECORD` with the four verdicts *missing, released,
    drifted, abandoned*. They belong on the Issues screen (board 04), reached
    only through a blocked banner (R09.6) — which this card does not carry, so
    on this screen they have no surface at all.

14. **The `Show N more` expanded state.** Not drawn. The implied behaviour is
    in-place expansion (the control is a count, and the count is what changes),
    not navigation and not a sheet. All three controls are independent; expanding
    one does not close another, because R09.2 governs the four **doors**, not
    the lists inside them.

15. **`How this was checked`, open.** Board 06a's subject. On this screen it is
    drawn closed (right chevron); the open form rotates the chevron down and is
    specified in 06a.

16. **Print.** `style.css` has exactly one `@media print` block
    (`style.css:4809`) and print always uses the light palette, so the blocked
    tint on the readiness and checks sections must be a colour that survives
    monochrome printing — i.e. the section must not depend on the tint alone to
    say "blocked"; the eyebrow's `--color-blocked` text and the footer chip's
    word carry it.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

1. **The board draws four doors open; the spec says one.** Decision:
   **implement R09.2 and R09.3.** Readiness auto-opens because this claim is
   blocked; relationships, sources and checks render closed. The board's own
   notes column 2 disowns the drawn state in its own eyebrow, so this is not a
   judgement call. The board is left unchanged (Paper is read-only for this
   lane).

2. **Module-ordering tie-break.** The board opens Capability support (11
   blockers) over Verification (7) when both are at `nearest 1 hop`. Decision:
   **hop distance ascending, then blocker count descending, then module name
   ascending**, and only the first module opens. Derived from the board's own
   ordering; nothing in the rules states it.

3. **The desktop footer strip is not the mobile chip component.** Decision:
   treat R-F.2's closed / open / blocked chip vocabulary as **mobile-only**
   (component I3, `377-0` → `7N2-0`), and the desktop strip as a plain-text row
   of four doors with chevrons plus the count hard right, per `1VC-0`. R-F.1's
   order and R-F.3's count placement bind both. This matches boards 05, 06a, 07
   and 07a. R-I.3 is what licenses the decision: the chip is the component and
   the rows are composition, so "the desktop strip is a different arrangement"
   is a legal thing to say, while "the desktop chip is a different chip" would
   not be.

4. **Section grounds have no tokens.** `#FBF7F7` (blocked tint over card),
   `#F6F8FA` (relationships), `#EBD9D8`, `#EFE6E5`, `#E8ECF1`, `#E3E8EE`,
   `#212934`, `#33262A` are all raw hexes with no Paper token. Decision: express
   each as a `color-mix` of an existing token rather than adding a hex —
   the readiness/checks ground as a blocked tint over `--card-bg`, the
   relationships ground as `--code-bg` (the engine already derives that as a
   `color-mix` of `--paper` into `--card-bg`), and every hairline from
   `--border` (or, on the blocked sections, `--warn` mixed into `--border`).
   The allowlist is closed (`tokens.md` §Open decisions), so no new token is
   proposed.

5. **`--color-dark-blocked-surface` / `--color-dark-blocked-hairline` have no
   engine home.** They are used on this screen (`6DR-0`, `6I6-0`, `6DS-0`).
   Decision: derive them at use as `color-mix(in srgb, var(--warn) X%,
   var(--card-bg))` and `color-mix(in srgb, var(--warn) X%, var(--border))`,
   matching the measured `#1D1618` / `#3A2A2B` on dark and `#FBF7F7` /
   `#EBD9D8` on light, rather than appending to the closed allowlist.

6. **The reading measure on this screen is 760px** (`1W2-0`, `6CW-0`),
   consistent with `tokens.md`'s frozen decision. `--container-reading: 720px`
   is treated as corrected to 760.

7. **`See in claims graph` needs a per-claim graph entry point that does not
   exist.** Decision: specify the affordance as drawn (`3B8-0` / `6FQ-0`, in
   the row `3B6-0` / `6FO-0`) and record the
   engine gap in §7.6. The link's behaviour — open the pane focused on this
   claim — is the mirror of `graph-ui.js:3585–3594` and needs a
   `data-dxg-open-claim`-style attribute on the *opening* side. Not designed
   here; flagged for the graph lane.

8. **M5 must be written after `style.css:4735`.** Decision recorded as an
   implementation constraint rather than left to discovery: a
   `@media (max-width: 520px)` block placed at `style.css:1263` (where the
   existing one lives) would lose to the 860 and 560 blocks at `:4728` and
   `:4743` that already declare `.claim-footer__counts`. The mobile footer rule
   goes in a **new** 520 block after `:4746`.

9. **The dark collapsed-module-row hairline is `#33262A`, and an earlier draft
   of §4.4 printed the light `#EBD9D8` in the Dark column.** Corrected here from
   a direct re-read. Measured: light `301-0` / `309-0` / `30H-0` all carry
   `border-top: 1px solid #EBD9D8`; dark `6F0-0` / `6F8-0` / `6FG-0` all carry
   `border-top: 1px solid #33262A`. A whole-subtree sweep of `6CH-0` returns
   **no** node bearing `#EBD9D8`, so the light hairline exists nowhere on either
   dark board — it was a transcription fault in this file, not a fault on the
   canvas. Decision: the Dark column of every hairline row names a value that a
   dark sweep of `6CH-0` / `72F-0` actually returns, and nothing else. A lane
   that implements a light hairline on a dark surface reproduces the defect
   group 09 shipped; `#33262A` is the value, and open decision 4's `color-mix`
   of `--warn` into `--border` is how it should be spelled in the engine, since
   neither hex has a token.

10. **`See in claims graph` is two nodes, not one, and the spec used to print
    one row for both.** The row (`3B6-0` / `6FO-0`) carries `padding-top: 12px`
    and `gap: 10px`; the link cluster (`3B8-0` / `6FQ-0`) carries `gap: 6px`.
    The earlier single row quoted the cluster on light and the row on dark, so
    it printed a `gap: 6px` / `padding-top: 12px` pair that exists on no node of
    either board. Decision: split the row in two and name both nodes on both
    boards, and — as a general rule for this file — a table row may pair a light
    node with a dark node **only when the two are twins at the same depth**.
    `1SY-0` and `6CH-0` are node-for-node mirrors, so a twin always exists;
    there is never a reason to reach for a parent on one side. §4.4 has been
    re-read end to end against that rule: `6DT-0`, `6DU-0`, `6DV-0`, `6DX-0`,
    `6DZ-0`, `6E0-0`, `6E1-0`, `6E2-0`, `6E5-0`, `6EV-0`, `6EZ-0`, `6F0-0`,
    `6F8-0`, `6FG-0` and `6FQ-0` now stand where `6DS-0`, `6DW-0` and `6E4-0`
    were previously doing duty for their own children.

### Paper defects

Found on these boards; recorded, not corrected (Paper is read-only for this
lane).

| # | Defect | Evidence |
|---|---|---|
| P1 | **All four expansions drawn open**, contradicting R09.2 and the board's own notes eyebrow. Present on all four artboards. | `1SY-0`, `6CH-0`, `6JS-0`, `72F-0`; admission at `7E6-0` column 2 |
| P2 | **Two LOCKED relationship rows carry a blocked-red lifecycle dot** (`#9E3B36`) against a green `LOCKED` badge, contradicting R-I.2's "dot in the lifecycle colour". | `26G-0` (The published observation record), `26U-0` (Observer substitution proof) |
| P3 | **Mobile LOCKED chip is 10/13**, where R-H.2 fixes it at 11/14 weight 600. The padlock is also 11 × 11 against R-H.2's 11px — that part agrees. | `6K4-0` |
| P4 | **Mobile relationship title is 14/19 weight 400**, where R-I.2 specifies 14/20 weight 500; its badge tracking is `+0.05em` where R-I.2 specifies `+0.06em`. | `6TE-0` |
| P5 | **Mobile row-two chips show up-chevrons (open) with closed-variant colour.** R-F.2's open variant requires an `--color-accent` border and an accent label at weight 600. | `6M9-0`, `6MD-0`, `6MH-0` |
| P6 | **The mobile blocked-variant chip `1 check failing` carries a 1px border** (`#9E3B3640` light, `#EC8A834D` dark) where R-F.2's blocked variant has **no border**. It also uses weight 500 where R-F.2 specifies weight 600 for the label. | `6MH-0` |
| P7 | **An empty text node ships in the desktop readiness footer.** `3B7-0` is a 923 × 22 serif 14/22 `--color-muted` Text node with no content, sitting left of `See in claims graph` purely as a spacer. | `3B6-0` → `3B7-0` |
| P8 | **The relationships section ground is `#F6F8FA` on light** where `--color-code-bg` is `#F3F5F8`; the dark twin is spelled as the literal `#1B212B` rather than `--color-dark-code-bg`. Two boards, one intended colour, two spellings. | `25M-0`, `6FW-0` |
| P9 | **Eight hairlines and two grounds are raw hexes with no token**: `#E8ECF1`, `#EBD9D8`, `#EFE6E5`, `#E3E8EE`, `#212934`, `#33262A`, `#FBF7F7`, `#9E3B361A`, `#9E3B361F`, `#9E3B3617`, `#9E3B3640`, `#EC8A8329`, `#EC8A832E`, `#EC8A8326`, `#EC8A834D`, `#2C6B521A`. Alpha is baked into the hex rather than expressed against a token. Note every dark hex in that list is a genuinely **dark** value — this defect is "untokenised", not "wrong hue"; see open decisions 4, 5 and 9. | Light: `1VC-0`, `2YR-0`, `2YU-0`, `2Z2-0`, `2Z5-0`, `2SK-0`, `6JZ-0`, `6LV-0`. Dark: `6D1-0`, `6DR-0`, `6E2-0`, `6E5-0`, `6F0-0`, `6IS-0`, `72Y-0`, `73O-0`. (`6DW-0` and `6E4-0` were cited here in an earlier draft and carry no colour at all — they are the module-row box and the 24px indent container.) |
| P10 | **The light desktop board spells colours as literals where the dark board spells tokens.** `1W8-0`'s LOCKED label and padlock are `#2C6B52`; the dark twin uses `var(--color-dark-locked)`. Same for `2JR-0` (`#9E3B36`) against `6DH-0`. The dark board is the better-tokenised of the pair. | `1W8-0`, `2JR-0`, `2YR-0` vs `6CM-0`, `6DH-0`, `6DR-0` |
| P11 | **Font families are spelled two ways across the four artboards.** Desktop uses `var(--font-sans)` / `var(--font-serif)` / `var(--font-mono)`; both mobile boards use the literals `"Inter", system-ui, sans-serif`, `"Source Serif 4", system-ui, sans-serif`, `"IBM Plex Mono", system-ui, sans-serif`. This is `tokens.md` §Disagreements 12 reproduced inside one screen group. | `1W1-0` vs `6JZ-0`, `6KB-0`, `6K7-0` |
| P12 | **The mobile source ref marker is mono 11/20**, an 11px glyph on a 20px leading, against the desktop 12/16. Neither size is on the type scale and the pair is not a deliberate step — 11/20 appears nowhere else in the token doc's off-scale list. | `6WW-0`, `6X9-0`, `6XG-0` |
| P13 | **`--color-faint` is used for two different jobs at two different sizes on the same row.** On `6XA-0` the sub-line is 12/16 faint and the provenance word is 11/16 faint; with no weight or colour difference, the two read as one run at two sizes rather than as two fields. | `6XA-0` |
