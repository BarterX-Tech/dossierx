# 06a · How this was checked — inline disclosure

Screen spec for group **06a**. Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`.
Engine side read from the worktree `feat/viewer-design-revamp` at `3ac8844`
(`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`).

Read with `docs/design/tokens.md` (token names and the engine mapping) and
`docs/design/reference-rules.md` (the rules cited by id below). Every number here
was read with `get_computed_styles` or `get_jsx`. Nothing is estimated from a
screenshot; screenshots were used only to decide what to measure and to read
intent.

---

## 1. Boards

| Node | Name | Size | What it shows |
|---|---|---|---|
| `2UK-0` | 06a · How this was checked — inline disclosure | 1220 × 1717 | Desktop light. One subject in two states plus an argument: a page header, state **1 CLOSED**, state **2 OPEN**, and a three-row rationale table. |
| `7QD-0` | … · DARK | 1220 × 1717 | Desktop dark. Same structure, node-for-node; expressed almost entirely in `--color-dark-*` tokens (see defects for the two exceptions). |
| `7VW-0` | … · MOBILE LIGHT | 390 × 2451 | Phone light. Same document at phone width — a fit-content column, **no app bar and no status bar** (it is a spec board, not a device screen). |
| `8AY-0` | … · MOBILE DARK | 390 × 2451 | Phone dark, node-for-node with `7VW-0`. |
| `7PU-0` | Group 06a — band | 3420 × 27.5 | The group band: eyebrow `06A · HOW THIS WAS CHECKED — INLINE DISCLOSURE` in `--color-accent`, caption "four designs of one screen · light pair left, dark pair right" in `--color-faint`, over a 2px `--color-accent` rule. |
| `8TO-0` | Group 06a — notes | 2828 × 215 | Three note columns — WHAT THIS BOARD IS, WHAT MOBILE DOES DIFFERENTLY, ONE THING MOBILE ADDS. Read in full under §2. |
| `2ML-0` | 12 · Code evidence (proposal) | 860 × 781.5 | **Reference board.** Binds this screen via R12.1 but is labelled `PROPOSAL — NOT AVAILABLE TODAY` (`2MN-0`). Nothing from it is in scope. |

### Viewer states depicted

All four boards depict **one and only one** viewer state: a claim whose single
implementation check is `step-coverage` on adapter `test-coverage/v1`, shape
`set`, verdict **Mismatch**, with 12 of 13 declared steps exercised and `s01`
missing. The check panel is therefore in its **not-ready / auto-open** form
throughout.

Two *disclosure* states are drawn, and they are the subject of the board:

- **State 1 — CLOSED** (`2WW-0` light / `7QJ-0` dark / `7W7-0`+`80G-0` mobile
  light / `8B4-0`+`8BO-0` mobile dark): the machine layer is hidden; the panel
  ends with a collapsed `> How this was checked` trigger.
- **State 2 — OPEN** (`2UQ-0` / `7SH-0` / `83D-0` / `8DJ-0`): the same panel, the
  trigger flipped to `^`, a right-ranged `snapshot b00905f98ca6`, and nine
  machine-layer rows plus a Copy row pushed in below it.

**Not depicted anywhere on these boards:** claim status (LOCKED / DRAFT /
BLOCKED), readiness, the blocked banner, comments, the four-chip metadata strip,
the claim title, the claim id, the right rail, the facet TOC, focus mode, any
other check verdict (`Matched`, `Uncheckable`, `Owed`), any other check shape
(`scalar`), multi-check panels, or `declared_none`. See §8.

---

## 2. Design intent

### What the reviewer should perceive

The board's own header states the thesis (`2UO-0`, verbatim):

> "Everything above this line is written for the reviewer. Everything below is
> verbatim engine data for the agent operating the tool — so here the identifiers
> stay exactly as the engine wrote them. Closed by default; nobody is made to
> read it."

That is the whole screen. There is a **line of authorship** running through the
check panel. Above it — the verdict word, the sentence explaining the mismatch,
`EXAMINED` / `COMPARED` / `FOUND`, the thirteen coverage chips — is prose and
signal written *for a human who must judge*. Below it — `check`, `adapter`,
`shape`, `target`, `expected`, `observed`, `missing`, `extra`, `detail` — is the
engine's own envelope, unedited, *for the agent operating the tool*. The
typography is the argument: everything above the line is sans and serif; every
key and every value below it is mono (`2V4-0` — every row's label and value is
`var(--font-mono)`). Serif marks what a human wrote, sans marks what the
interface says about it, mono marks what the machine emitted (`tokens.md` §7).
A reviewer should be able to tell, without reading a word, which half is which.

The second perception is that opening the machine layer **costs nothing and
loses nothing**. The rows above push down; the claim does not move; the panel
does not become a new surface. The reviewer never leaves the sentence they were
reading.

### Rules from `reference-rules.md` that bind this screen

- **R09.1 — four expansions of one strip, never four panels.** This disclosure is
  *inside* the checks expansion, which is one of the four. The board's `IT IS`
  row says so in its own words: "It is nested inside the checks expansion, so it
  is a second level of disclosure — **not a fifth chip in the strip**. Closing
  the checks expansion closes it too." (`2WH-0` / `2WI-0`.)
- **R09.2 — exactly one expansion open at a time.** Unchanged by this screen; the
  nested disclosure is not a fifth thing that can be open.
- **R09.3 — auto-open is reserved for readiness, and only when blocked.** The
  check panel on these boards is drawn in its not-ready form; the *nested* "How
  this was checked" disclosure is nevertheless **closed by default** (`2UO-0`:
  "Closed by default; nobody is made to read it"). Auto-open does not cascade
  into the second level.
- **R09.8 — eight demotions, and nothing is deleted from the data.** This screen
  *is* the destination of four of those eight. `Adapter · Shape`, `Target URI`
  and the raw diagnostics envelope are the demoted fields, and they land here,
  behind one disclosure, where an agent can still reach them (`2V6-0`…`2VY-0`).
  R09.8's `Expected + Observed → Merged` verdict is honoured by the **`FOUND`
  chip strip** above the line — one marked set, not two lists — while the two raw
  sets survive verbatim below it. Both halves of R09.8 are visible in one panel.
- **R09.9 — no inline dependency map.** Honoured by absence; nothing on this
  screen draws a graph.
- **R12.3 — tinting marks the compared members, and set order carries no
  meaning.** Written for board 12, but it is the rule that licenses §4.5's
  coverage strip. The thirteen chips are a *marked set*, not a diff: the one
  tinted chip says "this member was not exercised", and the left-to-right order
  is the step ids' own order, carrying no claim of its own. A lane agent must
  not read the strip as a sequence, sort it by verdict, or infer that step 1
  failing means the run stopped there. The verbatim `expected` / `observed` sets
  below the line are the authority; the strip is their marked projection.
- **R12.1 — a snippet lives behind "How this was checked", never inline.** This
  board *is* the disclosure R12.1 names. Board 12 is a proposal and is not
  implemented, so 06a ships the disclosure **without** a code snippet. If board
  12 ever ships, its panel goes inside this disclosure and nowhere else, and
  R12.4's WATCH caveat (opt-in per project, never on by default) rides with it.
- **R08.1 — no third hue.** The verdict dot and label, the missing-step chip, the
  `missing` value and the legend swatch are all `--color-blocked`. The rationale
  table's three labels re-use the three status colours for a fourth job — `IT IS`
  in `--color-locked`, `NOT` in `--color-blocked`, `WHY NOT` in `--color-faint`
  (`895-0`, and the dark twin `7V8-0` → `--color-dark-locked`). No new hue.
- **R00.0 — the components board is the source of truth; a screen is an
  instance.** Where this board and the components board disagree, the components
  board wins and this board is flagged. See Paper defects.
- **R-H.3 — the coverage strip never wraps.** Measured here as thirteen 24px
  chips with a 3px gap = **348px** inside a 358px gutter, 10px to spare
  (`817-0`, thirteen children `818-0`…`81W-0` at a 27px stride).
- **R-J.1 — there is one bottom sheet in this product.** This screen must not
  introduce a second. The board says it itself (see `NOT` below).

R10 (freshness) does **not** bind this screen: there is no right-rail footer here
and no elapsed phrase. The `snapshot b00905f98ca6` line is a provenance hash, not
a freshness statement, and must not be turned into one.

### The board's own three-row argument (`2WC-0`), verbatim and paraphrased

- **IT IS — "An inline disclosure, opening in place"** (`2WH-0`). Body `2WI-0`:
  the rows above push down, the claim stays where it is, it is a second level of
  disclosure nested in the checks expansion, and closing the checks expansion
  closes it too. *Intent: the reviewer's scroll position is the thing being
  protected. Nothing that opens here may move the claim.*
- **NOT — "A screen, a sheet or a side panel"** (`2WN-0`). Body `2WO-0`: "Eight
  rows of terminal detail do not earn a surface of their own. A screen would lose
  the claim you opened it from; a sheet would introduce a third kind of overlay
  beside the comments rail and the graph pane, for the least-used content in the
  viewer." *Intent: this is a standing prohibition, not a preference. It is also
  the reason R-J.1 is not violated on mobile — see §5.*
- **WHY NOT — "Compare with the Issues screen"** (`2WT-0`). Body `2WU-0`: "Issues
  earned a screen because it spans many claims and needs scope and sort. This
  spans one check, has no controls, and is read once then closed. The test is
  whether you would ever want to stay in it — here you would not." *Intent: the
  board hands a reusable test for any future disclosure — does the reader want to
  stay in it? If not, it is inline.*

### Notes strip (`8TO-0`), all three columns

**WHAT THIS BOARD IS.** "A spec board, not a device screen: one subject — the
inline 'How this was checked' disclosure — shown closed, then open, then argued
for. So the two mobile boards are 390px-wide fit-content columns with no app bar
and no status bar, the same document read at phone width. The explainer is the
reason the check can be trusted, so it survives whole at 390; what changes is how
its label/value rows are laid out." And: "Nothing is dropped. Every row, every
value, all thirteen coverage chips, the snapshot hash, the Copy as JSON row and
its caption, and all three rationale rows are present at 390. No wording differs
from desktop at any width." *Intent: completeness at 390 is a hard invariant, not
a goal. A lane agent may not drop a row to make the phone fit.*

**WHAT MOBILE DOES DIFFERENTLY.** Three sets of ranged rows become stacked lines
— `EXAMINED` / `COMPARED` / `FOUND` lose their 92px label column; the nine
machine-layer rows lose their 84px mono label column; the rationale table loses
its 92px `IT IS` / `NOT` / `WHY NOT` column. Both check panels go **edge to edge**
instead of sitting inset, and panel padding drops from 32px to the 16px gutter —
"That is what buys the coverage strip its width: at 24 × 28 with a 3px gap
(desktop is 32 × 29 / 5px) thirteen chips need 348px, and only the full 358px of
gutter content holds them in one row. The strip never wraps and no chip is
dropped." Type runs one step heavier where it got quiet (closed coverage chips
500 against desktop's 400; machine-layer keys 500 against 400) and sizes soften
(page title 26 → 24, header prose 16/26 → 15/24, rationale row padding 18/15 →
14/14, board margin 36px → 16px side and 24px top); the 700px prose measure is
removed as wider than the column. *Intent: the phone does not get a different
component. It gets the same component with its columns unwound, and the width
that buys back is spent entirely on the one element that must not wrap.*

**ONE THING MOBILE ADDS.** "The open panel gains a 1px top hairline it does not
have on desktop, and loses its 10px corner radius. On desktop the open state is a
rounded card floating inside the board, which is how it reads as a distinct
surface. Full-bleed there is no radius to keep, so the hairline is doing the job
the radius did — marking where the panel starts. It is an addition, not a
carry-over, and it is the only one on these boards." And the consequence the
board volunteers against itself: "the closed and open panels are framed
identically at 390, where on desktop they are visibly different objects. The
numbered CLOSED / OPEN labels above them are the only thing distinguishing the
two states. If that is too weak a cue, the fix is a stronger label, not a
narrower panel — narrowing the panel breaks the coverage strip." *Intent: a
declared, accepted weakness with its remedy pre-authorised. A lane agent that
finds the two mobile states hard to tell apart must strengthen the label, never
re-inset the panel.*

### Band (`7PU-0`)

Eyebrow `06A · HOW THIS WAS CHECKED — INLINE DISCLOSURE`, Inter 13/16 weight 600
tracking `+0.08em` in `--color-accent`; caption "four designs of one screen ·
light pair left, dark pair right", Inter 12/16 in `--color-faint`; 12px baseline
gap; 2px `--color-accent` rule, full 3420px width; column gap 9px. This is canvas
scaffolding and ships nothing.

### Where the board contradicts its own notes

Recorded here because approval is not proof; the full list is in §9.

1. The notes say the open panel's 1px top hairline is "an addition, not a
   carry-over" and that desktop "does not have" it. Measured, the desktop open
   panel's check body **does** carry `border-top: 1px solid #E8ECF1` (`3LZ-0`,
   and `7SI-0` dark with `#212934`). The claim is true only of the rounded
   *wrapper* (`2UQ-0` / `7SH-0` carry no border). The note overstates.
2. The notes list the mobile panel change as "panel padding drops from 32px to
   the 16px gutter". Measured, the **block** padding also drops: desktop
   `18px / 24px` (`3KD-0`) against mobile `16px / 20px` (`80G-0`). Only the
   inline change is documented.

---

## 3. Layout

### Desktop — 1220px board, 1440 tier

The board is drawn at **1220**, not 1440. Per `tokens.md` §8 the 1220-wide claim
boards (05, 06, **06a**, 07, 07a) are where the claim column narrows, and they
land in `@media (min-width: 861px) and (max-width: 1180px)`; the widest tier
(`min-width: 1181px`) is the 1440 boards. This screen therefore has **no 1440
board of its own** — it is a component spec that is placed inside whatever claim
column the reading view gives it.

| Region | Node | Measured |
|---|---|---|
| Board | `2UK-0` | `width: 1220px`, `height: fit-content`, `padding: 36px`, `gap: 20px`, `background: #EFF1F4` (`--color-paper`), `overflow: clip` |
| Content measure | derived | 1220 − 2 × 36 = **1148px** — every top-level child is 1148 wide |
| Header block | `2UL-0` | 1148 × 114, `flex-direction: column`, `gap: 8px` |
| State-1 group | `2WW-0` | 1148 × 382, column, `gap: 10px` |
| State-1 label row | `2WX-0` | 1148 × 16, row, `align-items: center`, `gap: 10px`; trailing rule `2X1-0` 881 × 1 in `--color-border` |
| Closed panel | `3KD-0` | 1148 wide, `padding: 18px 32px 24px`, `gap: 16px`, `background: #FBF7F7`, `border-top: 1px solid #E8ECF1`, **no radius, full-bleed to the 1148 measure** |
| Closed panel inner measure | `3KE-0` etc. | 1148 − 2 × 32 = **1084px** |
| State-2 label row | `2XE-0` | 1148 × 16, row, `gap: 10px`; trailing rule `2XI-0` 805 × 1 |
| Open panel wrapper | `2UQ-0` | 1148 × 761, column, `border-radius: 10px`, `padding-inline: 24px`, `background: #FBF7F7`, **no block padding, no border** |
| Open panel inner measure | `3LZ-0` etc. | 1148 − 2 × 24 = **1100px** |
| Open check body | `3LZ-0` | 1100 wide, `padding: 18px 32px 24px`, `gap: 16px`, `background: #FBF7F7`, `border-top: 1px solid #E8ECF1` → inner measure 1100 − 64 = **1036px** |
| Open disclosure strip | `2UX-0` | 1100 × 41, row, `align-items: center`, `gap: 7px`, `padding: 10px 32px 14px`, `border-top: 1px solid #EFE6E5` |
| Machine layer | `2V4-0` | 1100 × 392, column, `padding: 16px 32px 22px`, `background: #F1F3F6`, `border-top: 1px solid #E3E8EE` → inner measure **1036px** |
| Rationale table | `2WC-0` | 1148 × 292, column, `gap: 1px`, `border-radius: 10px`, `border: 1px solid var(--color-border)`, `background: var(--color-border)`, `overflow: clip` — the 1px gap over a border-coloured ground is what draws the two hairline dividers |
| Rationale row | `2WD-0` | 1146 × 96, row, `padding: 15px 18px`, `gap: 14px`, `background: #FFFFFF` |
| Prose measure inside the panel | `3KN-0` / `3M9-0` | `max-width: 700px`, `margin-top: -8px` |

**Sticky / scroll regions: none.** Nothing on this screen is sticky and nothing
scrolls internally. The whole board is `height: fit-content`; the machine layer
and the rationale table grow the page rather than scroll. This is deliberate —
R09.1's "the rows above push down" cannot be true of a region that scrolls under
a fixed header.

Dark board `7QD-0` is geometrically identical: `padding: 36px`, `gap: 20px`,
1220 wide, `height: fit-content`, and every child frame matches its light twin's
node dimensions node-for-node (`7QE-0` 1148 × 114, `7QJ-0` 1148 × 382, `7SH-0`
1148 × 761, `7V6-0` 1148 × 292). Only colour differs.

### Mobile — 390px board

| Region | Node | Measured |
|---|---|---|
| Board | `7VW-0` | `width: 390px`, `height: fit-content`, `padding-block: 24px` (**no inline padding — children own the gutter**), `gap: 20px`, `background: var(--color-paper)`, `overflow: clip` |
| Header | `7VX-0` | `padding-inline: 16px`, column, `gap: 8px` → content width **358px** |
| State-1 label | `7W7-0` → `7W8-0` | column, `gap: 10px`; the label row itself is 390 wide; trailing rule `7WC-0` is 91 × 1 |
| State-2 label | `831-0` | `padding-inline: 16px`, row, `gap: 10px`; trailing rule `835-0` is 15 × 1 |
| Closed panel | `80G-0` | **390 wide, edge to edge**, `padding: 16px 16px 20px`, `gap: 14px`, `background: #FBF7F7`, `border-top: 1px solid #E8ECF1`, no radius → inner measure **358px** |
| Open panel wrapper | `83D-0` | 390 × 1054, `background: #FBF7F7`, `border-top: 1px solid #E8ECF1`, **no radius** |
| Open check body | `83E-0` | 390 wide, `padding: 16px` (all four sides), `gap: 14px` |
| Open disclosure strip | `85N-0` | 390 × 39, `padding: 10px 16px 12px`, `gap: 7px`, `border-top: 1px solid #EFE6E5` |
| Machine layer | `86T-0` | 390 × 579, `padding: 12px 16px 18px`, `background: #F1F3F6`, `border-top: 1px solid #E3E8EE` |
| Rationale table | `893-0` | 358 wide, `margin-left: 16px`, `margin-right: 16px`, `border-radius: 10px`, `border: 1px solid var(--color-border)`, `gap: 1px` — the **only** element on the mobile board that keeps an inset and a radius |
| Rationale row | `894-0` | 356 wide, `padding: 14px`, `gap: 5px`, `background: #FFFFFF` |
| Coverage strip | `817-0` | `gap: 3px`; thirteen children at a 27px stride (`818-0` x = 0 … `81W-0` x = 324) → **348px of 358px, 10px spare** |

Mobile dark `8AY-0` is geometrically identical: `padding-block: 24px`, `gap:
20px`, header `8B0-0` `padding-inline: 16px` `gap: 8px`, closed panel `8BO-0`
`padding: 16px 16px 20px` `gap: 14px`, machine layer `8F6-0` `padding: 12px 16px
18px`, rationale `8GK-0` `margin-left/right: 16px` `border-radius: 10px` `gap:
1px`, rationale row `8GL-0` `padding: 14px` `gap: 5px`.

---

## 4. Components on this screen

Colour tokens are given as the `tokens.md` name with the hex beside. Where the
board painted a raw hex, that is stated and flagged — implement the token, never
the literal (`tokens.md` Disagreement 12).

**Measured-value count for the desktop light board `2UK-0`: 186 values, each
carrying its node id below.** (Dark and mobile values are additional.)

### 4.1 Page header (`2UL-0` light / `7QE-0` dark / `7VX-0` mobile)

| Element | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Container | display / direction / gap | `flex` / `column` / `8px` (`2UL-0`) | same (`7QE-0`) | same, plus `padding-inline: 16px` (`7VX-0`) |
| Eyebrow "EXPANDED FROM BOARD 06 · THE MACHINE LAYER" | family | `var(--font-sans)` = Inter (`2UM-0`) | same (`7QF-0`) | same (`7VY-0`) |
| | size / leading | `11px / 14px` (`2UM-0`) | `11px / 14px` (`7QF-0`) | `11px / 14px` (`7VY-0`) |
| | weight | `600` (`2UM-0`) | `600` (`7QF-0`) | `600` (`7VY-0`) |
| | tracking | `0.09em` (`2UM-0`) — **off-scale, no token** | `0.09em` (`7QF-0`) | `0.09em` (`7VY-0`) |
| | colour | `--color-faint` `#6E7C8E` (`2UM-0`) | `--color-dark-faint` `#8494A8` (`7QF-0`) | `--color-faint` (`7VY-0`) |
| Title "How this was checked" | size / leading | `26px / 32px` (`2UN-0`) | `26px / 32px` (`7QG-0`) | **`24px / 30px`** (`7VZ-0`) |
| | weight / tracking | `600` / `-0.02em` (`2UN-0`) | same (`7QG-0`) | same (`7VZ-0`, `8B2-0`) |
| | colour | `--color-ink` `#101720` (`2UN-0`) | `--color-dark-ink` `#E6EAF0` (`7QG-0`) | `--color-ink` (`7VZ-0`) / `--color-dark-ink` (`8B2-0`) |
| Header prose | family | `var(--font-serif)` = Source Serif 4 (`2UO-0`) | same (`7QH-0`) | same (`7W0-0`, `8B3-0`) |
| | size / leading | `16px / 26px` (`2UO-0`) | `16px / 26px` (`7QH-0`) | **`15px / 24px`** (`7W0-0`, `8B3-0`) |
| | weight / colour | `400` / `--color-muted` `#54606F` (`2UO-0`) | `400` / `--color-dark-muted` `#9AA7B8` (`7QH-0`) | `400` / `--color-muted` (`7W0-0`), `--color-dark-muted` (`8B3-0`) |

### 4.2 State label ("1 CLOSED — …", "2 OPEN — …") — `2WX-0` / `2XE-0`

| Part | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Row | layout | `flex`, `align-items: center`, `gap: 10px` (`2WX-0`, `2XE-0`) | same (`7QK-0`, `7SB-0`) | same (`7W8-0`, `831-0`; `831-0` adds `padding-inline: 16px`) |
| Number badge | padding / radius | `2px 8px` / `4px` (`2WY-0`) | `2px 8px` / `4px` (`7QL-0`, `7SC-0`) | `2px 8px` / `4px` (`7W9-0`, `832-0`) |
| | ground | `#54606F` — raw hex, = `--color-muted` (`2WY-0`) | `--color-dark-muted` `#9AA7B8` (`7QL-0`) | `--color-muted` (`7W9-0`) |
| | label | Inter `10px / 12px`, **weight `700`**, `#FFFFFF` (`2WZ-0`) | same shape (`7QL-0` via `7QL-0`) | same (`7W9-0` via `7W9-0`) |
| Label text | type | Inter `12px / 16px`, weight `600`, tracking `0.08em` (`2X0-0`, `2XH-0`) | same (`7QN-0`, `7SE-0`) | same (`7WB-0`, `834-0`) |
| | colour | `--color-faint` (`2X0-0`) | `--color-dark-faint` (`7QN-0`, `7SE-0`) | `--color-faint` (`7WB-0`, `834-0`) |
| Trailing rule | size / colour | 881 × 1 (`2X1-0`), 805 × 1 (`2XI-0`), `--color-border` `#DDE2E9` | `--color-dark-border` `#242C38` (`7QO-0`, `7SF-0`) | 91 × 1 (`7WC-0`), 15 × 1 (`835-0`) |

This label is board scaffolding: it exists to name the two states side by side
and **does not ship**. It is measured only so a lane agent recognises it as
scaffolding rather than chrome.

### 4.3 Check panel — closed (`3KD-0` light / `7QP-0` dark / `80G-0` mobile)

| Element | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Panel | padding | `18px 32px 24px` (`3KD-0`) | `18px 32px 24px` (`7QP-0`) | `16px 16px 20px` (`80G-0`, `8BO-0`) |
| | gap | `16px` (`3KD-0`) | `16px` (`7QP-0`) | `14px` (`80G-0`, `8BO-0`) |
| | ground | `#FBF7F7` — **raw hex, no token** (`3KD-0`) | `--color-dark-blocked-surface` `#1D1618` (`7QP-0`) | `#FBF7F7` (`80G-0`), `--color-dark-blocked-surface` (`8BO-0`) |
| | top border | `1px solid #E8ECF1` — **raw hex, no token** (`3KD-0`) | `1px solid #212934` — **raw hex, no token** (`7QP-0`) | same (`80G-0`, `8BO-0`) |
| | radius | none (`3KD-0`) | none (`7QP-0`) | none (`80G-0`) |
| Section eyebrow "IMPLEMENTATION CHECKS" | type | Inter `11px / 14px`, weight `600`, tracking `0.08em` (`3KF-0`) | same (`7QR-0`) | same (`80I-0`) |
| | colour | `#6E7C8E` raw = `--color-faint` (`3KF-0`) | `--color-dark-faint` (`7QR-0`) | `--color-faint` (`80I-0`) |
| | row gap | `12px` (`3KE-0`) | `12px` (`7QQ-0` — the eyebrow row frame) | measured on `80H-0` |
| Eyebrow rule | height / colour | `1px`, `#EFE6E5` — **raw hex, no token** (`3KG-0`) | `--color-dark-blocked-hairline` `#3A2A2B` (`7QS-0`) | `1px`, `#EFE6E5` (`80J-0`) |
| Eyebrow count "1" | type / colour | `var(--font-mono)` `11px / 14px`, weight `400`, `#6E7C8E` (`3KH-0`) | mono `11/14` w400, `--color-dark-faint` (`7QT-0`) | mono `11/14`, `--color-faint` (`80K-0`) |
| Check title "Step coverage" | type | Inter `16px / 20px`, weight `600` (`3KJ-0`) | Inter `16/20` weight `600` (`7QV-0`) | Inter `16/20` weight `600` (`80M-0`) |
| | colour | `#101720` raw = `--color-ink` (`3KJ-0`) | `--color-dark-ink` (`7QV-0`) | `--color-ink` (`80M-0`) |
| | row | `align-items: baseline`, `gap: 14px` (`3KI-0`) | same (`7QU-0`) | same (`80L-0`) |
| Verdict dot | size / radius / colour | `7 × 7`, `999px`, `#9E3B36` = `--color-blocked` (`3KL-0`) | `7 × 7`, `999px`, `--color-dark-blocked` `#EC8A83` (`7QX-0`) | `7 × 7`, `999px`, `#9E3B36` (`80N-0`) |
| Verdict label "Mismatch" | type / colour | Inter `13px / 16px`, weight `600`, `#9E3B36` (`3KM-0`) | Inter `13/16` w600, `--color-dark-blocked` (`7QY-0`) | Inter `13/16` w600, `#9E3B36` (`80N-0`) |
| | dot↔label gap | `6px` (`3KK-0`) | `6px` (`7QW-0` — the dot+label group) | `6px` (`80N-0`) |
| Explanation sentence | type | serif `15px / 23px`, weight `400` (`3KN-0`) | serif `15/23` (`7QZ-0`) | serif `15/23` (`80Q-0`) |
| | colour | `#54606F` = `--color-muted` (`3KN-0`) | `--color-dark-muted` (`7QZ-0`) | `--color-muted` (`80Q-0`) |
| | measure / pull-up | `max-width: 700px`, `margin-top: -8px` (`3KN-0`) | same (`7QZ-0`) | **no max-width**, `margin-top: -6px` (`80Q-0`) |

### 4.4 EXAMINED / COMPARED rows (`3KP-0`, `3KS-0` / dark `7R1-0`, `7R4-0` / mobile `80T-0`, `80W-0`)

| Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|
| Row layout | `flex`, `align-items: baseline`, `gap: 20px`, `padding-block: 10px` (`3KP-0`) | same (`7R1-0`) | **`flex-direction: column`**, `gap: 3px`, `padding-block: 10px` (`80T-0`) |
| Row hairline | `border-top: 1px solid #EFE6E5` (`3KP-0`, `3KS-0`) | `border-top: 1px solid var(--color-dark-blocked-hairline)` (`7R1-0`, `7R4-0`) | `border-top: 1px solid #EFE6E5` (`80T-0`) |
| Label column | `width: 92px`, `flex-shrink: 0` (`3KQ-0`) | `width: 92px`, `flex-shrink: 0` (`7R2-0`) | **no column — label is its own line** (`80T-0` child) |
| Label type | Inter `11px / 14px`, weight `600`, tracking `0.06em` (`3KQ-0`) | same (`7R2-0` EXAMINED, `7R5-0` COMPARED) | same (`80T-0`) |
| Label colour | `#6E7C8E` = `--color-faint` (`3KQ-0`) | `--color-dark-faint` (`7R2-0`, `7R5-0`) | `--color-faint` (`80T-0`) |
| Value type | serif `14px / 22px`, weight `400` (`3KR-0`) | serif `14/22` w400 (`7R3-0`, `7R6-0`) | serif `14/22` (`80T-0`) |
| Value colour | `#54606F` = `--color-muted` (`3KR-0`) | `--color-dark-muted` (`7R3-0`, `7R6-0`) | `--color-muted` (`80T-0`) |

### 4.5 FOUND row and the coverage strip (`3KV-0` / `7R7-0` / `815-0`)

| Element | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Row | padding | `12px` top / `10px` bottom (`3KV-0`) | same (`7R7-0`) | `12px` top / `10px` bottom (`815-0`) |
| | gap / hairline | `20px`, `1px solid #EFE6E5` (`3KV-0`) | `20px`, `1px solid var(--color-dark-blocked-hairline)` (`7R7-0`) | column, `gap: 8px`, `1px solid #EFE6E5` (`815-0`) |
| | label cell | `width: 92px`, `flex-shrink: 0` (`3KW-0`) | `width: 92px`, `flex-shrink: 0` (`7R8-0`) | no cell — label is line one (`816-0`) |
| | value stack gap | `9px` (`3KX-0`) | `9px` (`7R9-0`) | n/a — stack is the row (`815-0`) |
| Strip | layout | `flex`, `flex-wrap: wrap`, `gap: 5px` (`3KY-0`) | same (`7RA-0`) | `flex`, **`gap: 3px`**, no wrap (`817-0`) |
| Step chip — blocked (step 1) | size | `32 × 29` (`3KZ-0`) | `32 × 29` (`7RB-0`) | **`24 × 28`** (`818-0`) |
| | radius | `5px` (`3KZ-0`) | `5px` (`7RB-0`) | `5px` (`818-0`) |
| | border | `1.5px solid #9E3B36` = `--color-blocked` (`3KZ-0`) | `1.5px solid var(--color-dark-blocked)` (`7RB-0`) | `1.5px solid #9E3B36` (`818-0`) |
| | ground | `#9E3B361F` (blocked @ ~12 %) — **baked-alpha hex** (`3KZ-0`) | `#EC8A8329` (blocked @ ~16 %) — **baked-alpha hex** (`7RB-0`) | `#9E3B361F` (`818-0`) |
| | label | Inter `12px / 16px`, weight `600`, `#9E3B36` (`3L0-0`) | Inter `12/16` w600, `--color-dark-blocked` (`7RC-0`) | Inter `12px / 16px`, weight `600`, `#9E3B36` (`819-0`) |
| Step chip — ordinary | size / radius | `32 × 29`, `5px` (`3L1-0`) | `32 × 29`, `5px` (`7RD-0`) | **`24 × 28`**, `5px` (`81A-0`) |
| | ground / border | `#FFFFFF` = `--color-card`, `1px solid #DDE2E9` = `--color-border` (`3L1-0`) | `var(--color-dark-card)` `#161B22`, `1px solid var(--color-dark-border)` (`7RD-0`) | `#FFFFFF`, `1px solid #DDE2E9` (`81A-0`) |
| | label | Inter `12px / 16px`, **weight `400`**, `#54606F` (`3L2-0`) | Inter `12/16` w400, `--color-dark-muted` (`7RE-0`) | Inter `12/16`, **weight `500`**, `var(--color-muted)` (`81B-0`) |
| | chip count | 13 (`3KZ-0`, `3L1-0` … 13 children of `3KY-0`) | 13 (`7RB-0` … `7RZ-0`, 13 children of `7RA-0`) | 13 (`818-0` … `81W-0`) |
| Legend row | gap | `14px` (`3LP-0`) | `14px` (`7S1-0`) | `12px` (`81Y-0`) |
| Legend count "12 of 13 exercised" | type / colour | serif `14px / 18px`, `#6E7C8E` = `--color-faint` (`3LQ-0`) | serif `14/18` w400, `--color-dark-faint` (`7S2-0`) | serif `14/18` w400, `var(--color-faint)` (`81Z-0`) |
| Legend swatch | size / radius / border / ground | `11 × 11`, `3px`, `1.5px solid #9E3B36`, `#9E3B361F` (`3LS-0`) | `11 × 11`, `3px`, `1.5px solid var(--color-dark-blocked)`, `#EC8A8329` (`7S4-0`) | `11 × 11`, `3px`, `1.5px solid #9E3B36`, `#9E3B361F` (`821-0`) |
| Legend label "never run" | type / colour | Inter `12px / 16px`, weight `400`, `--color-faint` (`3LT-0`) | Inter `12/16` w400, `--color-dark-faint` (`7S5-0`) | Inter `12/16` w400, `var(--color-faint)` (`822-0`) |
| | swatch↔label gap | `6px` (`3LR-0`) | `6px` (`7S3-0`) | `6px` (`820-0`) |

**The mobile blocked chip's label is now measured** (`819-0`: Inter `12px / 16px`,
weight `600`, `#9E3B36`) — it matches the desktop value exactly. The earlier
"not measured" gap in this table is closed. What the measurement exposed instead
is a disagreement with the components board; see Paper defect 12.

### 4.6 Disclosure trigger — closed state (`3LU-0` / `7S6-0` / `82F-0`)

| Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|
| Row | `flex`, `align-items: center`, `gap: 7px`, `padding-top: 12px`, **no border** (`3LU-0`) | same (`7S6-0`) | same (`82F-0`) |
| Chevron | inline SVG `12 × 12`, `viewBox 0 0 24 24`, path `m9 18 6-6-6-6` (right/closed), `stroke-width: 2.6`, `stroke-linecap: round` (`2UY-0`-shape, here `3LV-0`) | same, stroke `var(--color-dark-accent)` (SVG `7S7-0`, path `7S8-0`) | same (`82F-0`) |
| Chevron stroke | `#1C4E8C` = `--color-accent` (`3LV-0`) | `--color-dark-accent` `#6AA6E8` (`7S8-0`) | `#1C4E8C` (`82F-0`) |
| Label "How this was checked" | Inter `13px / 16px`, weight `500`, `#1C4E8C` (`3LX-0`) | Inter `13/16` w500, `--color-dark-accent` (`7S9-0`) | Inter `13/16` w500, `#1C4E8C` (`82F-0`) |

**State meaning.** In the closed state the trigger is *inside* the check body,
carries no hairline, and the panel's last visible thing is the accent label. The
chevron points **right**.

### 4.7 Disclosure trigger — open state (`2UX-0` / `7TZ-0` / `85N-0`)

| Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|
| Strip | `flex`, `align-items: center`, `gap: 7px`, `padding: 10px 32px 14px` (`2UX-0`) | same (`7TZ-0`) | `padding: 10px 16px 12px` (`85N-0`) |
| Strip hairline | `border-top: 1px solid #EFE6E5` (`2UX-0`) | `border-top: 1px solid var(--color-dark-blocked-hairline)` (`7TZ-0`) | `1px solid #EFE6E5` (`85N-0`, `8F0-0` dark uses the token) |
| Chevron | `12 × 12`, path `m18 15-6-6-6 6` (**up/open**), `stroke-width: 2.6` (`2UY-0` / `2UZ-0`) | same, `var(--color-dark-accent)` (`7U0-0`) | same (`85O-0`) |
| Label | Inter `13px / 16px`, weight `500`, `--color-accent` (`2V0-0`) | `--color-dark-accent` (`7U2-0`) | `--color-accent` (`85Q-0`) |
| Spacer | `flex-grow: 1`, zero height (`2V1-0`) | same (`7U3-0`) | same (`85R-0`) |
| Snapshot hash "snapshot b00905f98ca6" | mono `11px / 14px`, weight `400`, `--color-faint` (`2V2-0`) | mono `11/14`, `--color-dark-faint` (`7U4-0`) | mono `11/14`, `--color-faint` (`85S-0`) |

**State meaning.** Opening does four things and only four: the chevron flips to
up; the trigger gains a top hairline and becomes a full-width divided strip
rather than a line inside the body; the snapshot hash appears hard right; and the
machine layer is inserted below. Nothing above the trigger changes.

### 4.8 Machine layer (`2V4-0` / `7U5-0` / `86T-0`)

| Element | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Block | padding | `16px 32px 22px` (`2V4-0`) | same (`7U5-0`) | `12px 16px 18px` (`86T-0`, `8F6-0`) |
| | ground | `#F1F3F6` — **raw hex; `--color-code-bg` is `#F3F5F8`** (`2V4-0`) | `--color-dark-code-bg` `#1B212B` (`7U5-0`) | `#F1F3F6` (`86T-0`), `--color-dark-code-bg` (`8F6-0`) |
| | top border | `1px solid #E3E8EE` — **raw hex** (`2V4-0`) | `1px solid var(--color-dark-border)` (`7U5-0`) | `1px solid #E3E8EE` (`86T-0`) |
| Row | layout | `flex`, `align-items: baseline`, `gap: 18px`, `padding-block: 9px` (`2V5-0`) | same (`7U6-0`, `7U9-0`) | **`column`**, `gap: 2px`, `padding-block: 8px` (`86U-0`) |
| | hairline | `1px solid #E3E8EE` on every row after the first (`2V8-0`, `2VB-0`, `2VE-0`, `2VJ-0`, `2VM-0`, `2VP-0`, `2VS-0`, `2VV-0`) | `1px solid var(--color-dark-border)` on every row after the first (`7U9-0`, `7UC-0`, `7UF-0`, `7UJ-0`, `7UM-0`, `7UP-0`, `7US-0`, `7UV-0`) | same pattern |
| Key | column width | `84px`, `flex-shrink: 0` (`2V6-0`) | `84px`, `flex-shrink: 0` (`7U7-0`) | **none — key is its own line** (`86U-0`) |
| | type | mono `11px / 14px`, weight `400` (`2V6-0`) | mono `11/14` w400 (`7U7-0`) | mono `11/14`, **weight `500`** (`86U-0`) |
| | colour | `--color-faint` (`2V6-0`) | `--color-dark-faint` (`7U7-0`) | `--color-faint` (`86U-0`) |
| Value — short | type / colour | mono `12px / 16px`, `--color-ink` (`2V7-0`) | mono `12/16` w400, `--color-dark-ink` (`7U8-0`) | mono `12px / 18px`, `--color-ink` (`86U-0`) |
| Value — wrapping (`target`, `expected`, `observed`) | type | mono `12px / 19px`, `overflow-wrap: anywhere` (`2VG-0`, `2VL-0`, `2VO-0`) | mono `12/19`, `overflow-wrap: anywhere` (`7UH-0`, `7UL-0`, `7UO-0`) | mono `12 / 18`, `overflow-wrap: anywhere` |
| Value — `missing` | type / weight / colour | mono `12px / 16px`, weight `500`, `--color-blocked` (`2VR-0`) | mono `12/16` w500, `--color-dark-blocked` (`7UR-0`) | same |
| Value — `extra` (em dash) | colour | `--color-faint` (`2VU-0`) | `--color-dark-faint` (`7UU-0`) | same |
| Key order | 9 rows, fixed | `check`, `adapter`, `shape`, `target`, `expected`, `observed`, `missing`, `extra`, `detail` (`2V6-0`, `2V9-0`, `2VC-0`, `2VF-0`, `2VJ-0`, `2VM-0`, `2VP-0`, `2VS-0`, `2VV-0`) | same (`7U7-0`, `7UA-0`, `7UD-0`, `7UG-0`, `7UK-0`, `7UN-0`, `7UQ-0`, `7UT-0`, `7UW-0`) | same (`86U-0` … `886-0`) |
| Copy row | layout | `flex`, `align-items: center`, `gap: 14px`, `padding-top: 14px`, **no hairline** (`2VY-0`) | same (`7UY-0`) | `gap: 12px`, `padding-top: 14px` (`88B-0`) |
| Copy icon | SVG `12 × 12`, two paths (`rect 14×14 x8 y8 rx2` + `M4 16V6a2 2 0 0 1 2-2h10`), `stroke-width: 2` | stroke `#54606F` = `--color-muted` (`2W0-0`) | stroke `var(--color-dark-muted)` (`7V0-0`) | stroke `#54606F` (`88B-0`) |
| Copy label "Copy as JSON" | Inter `12px / 16px`, weight `500`, `--color-muted` (`2W3-0`) | Inter `12/16` w500, `--color-dark-muted` (`7V3-0`) | `--color-muted` (`88B-0`) |
| Copy caption | Inter `12px / 16px`, weight `400`, `--color-faint` (`2W4-0`) | Inter `12/16` w400, `--color-dark-faint` (`7V4-0`) | `--color-faint` (`88B-0`) |
| | icon↔label gap | `6px` (`2VZ-0`) | `6px` (`7UZ-0`) | `6px` (`88B-0`) |

### 4.9 Rationale table (`2WC-0` / `7V6-0` / `893-0`)

| Element | Property | Desktop light | Desktop dark | Mobile |
|---|---|---|---|---|
| Table | radius / border / ground / gap | `10px`, `1px solid var(--color-border)`, ground `var(--color-border)`, `gap: 1px`, `overflow: clip` (`2WC-0`) | `10px`, `1px solid var(--color-dark-border)`, ground `var(--color-dark-border)`, `gap: 1px` (`7V6-0`) | same, plus `margin-left/right: 16px` (`893-0`, `8GK-0`) |
| Row | padding / gap / ground | `15px 18px`, `gap: 14px`, `#FFFFFF` = `--color-card` (`2WD-0`) | `15px 18px`, `gap: 14px`, `var(--color-dark-card)` (`7V7-0`) | **`14px` all round**, `gap: 5px`, `#FFFFFF` (`894-0`) / `var(--color-dark-card)` (`8GL-0`) |
| Row frames, in order | — | `2WD-0` (`IT IS`), `2WJ-0` (`NOT`), `2WP-0` (`WHY NOT`) | `7V7-0`, `7VD-0`, `7VJ-0` | `894-0`, `898-0`, `89C-0` |
| Label column | width / offset | `92px`, `flex-shrink: 0`, `padding-top: 2px` (`2WE-0`, `2WK-0`, `2WQ-0`) | `92px`, `flex-shrink: 0`, `padding-top: 2px` (`7V8-0`, `7VE-0`, `7VK-0` — the three label-cell wrappers) | **no column — label is line one** (`895-0`) |
| Label type | Inter `11px / 14px`, weight `600`, tracking `0.06em` | `2WF-0`, `2WL-0`, `2WR-0` | `7V9-0`, `7VF-0`, `7VL-0` (the label text nodes inside those wrappers) | `895-0`, `899-0`, `89D-0` |
| Label colour — `IT IS` | `#2C6B52` = `--color-locked` (`2WF-0`) | `--color-dark-locked` `#63BE9A` (`7V9-0`) | `#2C6B52` (`895-0`) |
| Label colour — `NOT` | `#9E3B36` = `--color-blocked` (`2WL-0`) | `--color-dark-blocked` `#EC8A83` (`7VF-0`) | `#9E3B36` (`899-0`) |
| Label colour — `WHY NOT` | `--color-faint` (`2WR-0`) | `--color-dark-faint` `#8494A8` (`7VL-0`) | `--color-faint` (`89D-0`) |
| Heading | type / colour | Inter `14px / 18px`, weight `600`, `--color-ink` (`2WH-0`) | Inter `14/18` w600, `--color-dark-ink` (`7VB-0`, `7VH-0`, `7VN-0`) | Inter `14px / **19px**`, weight `600`, `--color-ink` (`896-0`) |
| Body | type / colour | serif `14px / 22px`, weight `400`, `--color-muted` (`2WI-0`) | serif `14/22` w400, `--color-dark-muted` (`7VC-0`, `7VI-0`, `7VO-0`) | serif `14/22`, `--color-muted` (`897-0`) |
| | heading↔body gap | `4px` (`2WG-0`) | `4px` (`7VA-0`, `7VG-0`, `7VM-0`) | `5px` via row gap (`894-0`) |

### States visible on the boards, and states that are not

Visible and measured: **closed** and **open** (§4.6, §4.7); **blocked/mismatch**
verdict (§4.3); **blocked** and **ordinary** coverage step (§4.5); the three
rationale label colours (§4.9). **Hover, active, focus, selected and disabled are
drawn nowhere on these four boards** — a Paper file has no hover. §8 says what
the spec implies for them.

---

## 5. Mobile rules

Per `tokens.md` §8 the rule is: **arrangement is a width question and belongs in
`@media (max-width: 520px)`; target size and hover-less affordance is a pointer
question and belongs in `@media (pointer: coarse)`.** No 390px breakpoint is
added.

| # | Rule at 390 | Engine query | Evidence |
|---|---|---|---|
| M1 | Board gutter drops 36px → **16px side**, 24px top/bottom, and the page's inline padding moves onto its children | `max-width: 520px` | `2UK-0` `padding: 36px` → `7VW-0` `padding-block: 24px` + `7VX-0` `padding-inline: 16px` |
| M2 | Both check panels go **edge to edge**: the open panel loses its 24px inset and its 10px radius | `max-width: 520px` | `2UQ-0` (`radius 10px`, `padding-inline 24px`) → `83D-0` (no radius, no inset) |
| M3 | The open panel **gains a 1px top hairline** to replace the radius as the "panel starts here" mark | `max-width: 520px` | `83D-0` `border-top: 1px solid #E8ECF1`; dark `8DJ-0` `1px solid #212934` |
| M4 | Panel padding `18/32/24` → `16/16/20`; check-body gap `16px` → `14px` | `max-width: 520px` | `3KD-0` → `80G-0` |
| M5 | `EXAMINED` / `COMPARED` / `FOUND` lose the **92px label column**; label stacks above value with a 3px gap | `max-width: 520px` | `3KP-0` (row, `gap 20px`, `3KQ-0` `width 92px`) → `80T-0` (`column`, `gap 3px`) |
| M6 | The nine machine-layer rows lose the **84px mono key column** the same way, gap `2px`, row padding `9px` → `8px` | `max-width: 520px` | `2V5-0` / `2V6-0` → `86U-0` |
| M7 | The rationale table loses its **92px `IT IS` / `NOT` / `WHY NOT` column**; row padding `15/18` → `14`, gap `14px` → `5px` | `max-width: 520px` | `2WD-0` / `2WE-0` → `894-0` |
| M8 | Coverage step chip `32 × 29` / `5px` gap → **`24 × 28` / `3px` gap**. Thirteen chips = 348px inside a 358px gutter. **The strip must never wrap and no chip may be dropped** (R-H.3) | `max-width: 520px` | `3KZ-0` / `3KY-0` → `818-0` / `817-0`; 13 children at a 27px stride |
| M9 | Type softens: title `26/32` → `24/30`; header prose `16/26` → `15/24`; machine value leading `16` → `18` | `max-width: 520px` | `2UN-0` → `7VZ-0`; `2UO-0` → `7W0-0`; `2V7-0` → `86U-0` |
| M10 | Type runs **one step heavier** where it got quiet: ordinary coverage-chip label `400` → `500`; machine-layer key `400` → `500` | `max-width: 520px` | `3L2-0` → `81B-0`; `2V6-0` → `86U-0`. **The chip half of this rule is flagged** — the components board (R-H.3, `7P6-0`) draws the ordinary chip label at weight `400` at this width, so per R00.0 it wins until amended. See Paper defect 12; the machine-layer half is unaffected. |
| M11 | The `max-width: 700px` prose measure on the explanation sentence is **removed** (it is wider than the column); the `-8px` pull-up becomes `-6px` | `max-width: 520px` | `3KN-0` → `80Q-0` |
| M12 | The rationale table keeps its 16px inset and 10px radius — it is the **only** element that does | `max-width: 520px` | `893-0` `margin-left/right: 16px`, `border-radius: 10px` |
| M13 | The disclosure trigger and the Copy-as-JSON row are the screen's only controls; each needs a **≥ 44px hit area**, achieved by padding the hit target, not by growing the 13px/12px label | `(pointer: coarse)` | Drawn heights are 28px (`82F-0`) and 30px (`88B-0`) — below 44, so the pointer query must supply the rest, exactly as the engine already does for `.comment-chip` (marker `style.css:1848-1851`, rule `1852-1860`) |
| M14 | Chevron rotation is the open/closed affordance; there is no hover state to lean on | `(pointer: coarse)` | `3LV-0` (`m9 18 6-6-6-6`) vs `2UZ-0` (`m18 15-6-6-6 6`) |

### Sheet policy on this screen

**There is no bottom sheet on this screen, at any width, and adding one is
prohibited.** R-J.1 says there is one sheet shell in this product, and the board's
own `NOT` row (`2WN-0` / `2WO-0`) rules this content out of it by name: "a sheet
would introduce a third kind of overlay beside the comments rail and the graph
pane, for the least-used content in the viewer." The machine layer opens **in
place** at 390 exactly as it does at 1220. A lane agent that reaches for
`AYG-0`'s shell here has misread both rules.

### What does not change at 390

Every row, every value, all thirteen coverage chips, the snapshot hash, the Copy
as JSON row and its caption, and all three rationale rows are present
(`8TO-0`). **No wording differs from desktop at any width.** Nothing is hidden,
truncated or replaced with a "show more".

---

## 6. Footer vocabulary

This screen has **no claim footer strip** and **no evidence/freshness row** —
those belong to boards 05/06/07 and to the right rail. R10 does not bind here.
The meta vocabulary this screen does own, quoted from the board text in the order
it appears:

**Panel eyebrow row** (`3KF-0` → `3KH-0`), left to right:

> `IMPLEMENTATION CHECKS` … `1`

The trailing `1` is the check count, mono, right-ranged across a hairline. It is
a count and a noun in the R-F.1 sense, with the noun carried by the eyebrow.

**Verdict** (`3KM-0`): `Mismatch` — preceded by a 7px dot, hard right of the
check title. The engine's four verdict words are `Matched`, `Mismatch`,
`Uncheckable`, `Owed` (`internal/render/components/conformance.go:96-109`); only
`Mismatch` is drawn.

**Label column, in fixed order** (`3KQ-0`, `3KT-0`, `3KW-0` parents):

> `EXAMINED` · `COMPARED` · `FOUND`

**Under FOUND**, left to right (`3LQ-0`, `3LT-0`):

> `12 of 13 exercised`   ▫ `never run`

`12 of 13 exercised` is a count sentence, not a percentage and not a score. The
swatch before `never run` is the same treatment as the step-1 chip, at 11 × 11 —
the legend *is* the chip, shrunk.

**Disclosure trigger** (`3LX-0` closed, `2V0-0` open), identical in both states:

> `How this was checked`

**Open-state right end** (`2V2-0`):

> `snapshot b00905f98ca6`

Mono, faint, right-ranged. Twelve hex characters after the word `snapshot`. It is
provenance — which observation pass produced these values — and is the reason
R12.4's WORKS caveat can claim a snippet would be "exactly as fresh as the value
beside it". It is **not** an elapsed time and must not be rendered as one.

**Machine-layer keys, in fixed order** (`2V6-0` … `2VV-0`), lowercase mono:

> `check` · `adapter` · `shape` · `target` · `expected` · `observed` · `missing`
> · `extra` · `detail`

**Copy row** (`2W3-0`, `2W4-0`), left to right:

> ⧉ `Copy as JSON`   `the same envelope dossierx check returns`

**Board scaffolding** (does not ship): `EXPANDED FROM BOARD 06 · THE MACHINE
LAYER` (`2UM-0`); `1` `CLOSED — HOW IT USUALLY SITS` (`2X0-0`); `2` `OPEN — SAME
PLACE, ROWS PUSHED DOWN` (`2XH-0`); `IT IS` / `NOT` / `WHY NOT` (`2WF-0`,
`2WL-0`, `2WR-0`).

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

### 7.1 The Go emitter — this is where the panel is built today

- `internal/render/components/conformance.go:16-53` — `ConformanceHTML(result)`.
  Emits `<details class="claim-conformance" data-conformance-mode … data-claim-id
  … data-implementation-ready …>` and, at lines 28-30, **`open` when the claim is
  not implementation-ready** — this is R09.3's auto-open, already implemented.
  The summary is `internal/render/components/conformance.go:31` —
  `<summary class="claim-conformance-head"><strong>Implementation checks</strong>`
  plus a `.pill ps`/`.pill pw` reading `Ready` / `Not ready`. The
  `declared_none` branch is lines 39-45 (`data-conformance-state="declared_none"`
  is written earlier, at 24-26).
- `internal/render/components/conformance.go:55-94` — `writeConformanceCheck`.
  Emits `<article class="claim-conformance-check" data-check-id … data-conformance-state
  … data-shape … data-implementation-ready …>` whose
  `claim-conformance-check-head` markup is lines 65-71, then the flat rows —
  lines 73-92 — in the order `Shape` (73), `Adapter` (74), `Target` (75),
  `Expected` (76), `Observed` (78), `missing` (81), `extra` (84),
  `observation error` (87), `adapter message` (88), `detail` (91).
- `internal/render/components/conformance.go:96-109` — `conformanceStateLabel`:
  the four verdict words `Matched` (99), `Mismatch` (101), `Uncheckable` (103),
  `Owed` (105), each returned from the `case` line immediately above it.
- `internal/render/components/conformance.go:111-120` — `conformancePill`: state →
  `ps` (114) / `pw` (116) / `pv` (118, the `default`).
- `internal/render/components/conformance.go:122-129` — `writeConformanceValue`,
  the two-way dispatcher `Expected` / `Observed` go through: a `string` falls to
  `writeConformanceLine` (125), a `[]string` to `writeConformanceMembers` (127).
- `internal/render/components/conformance.go:131-137` — `writeConformanceLine`:
  `<p class="claim-conformance-line"><span>LABEL:</span> <code>VALUE</code></p>`.
  **This one function is the whole machine layer today** — nine rows, no
  key column, no disclosure around them.
- `internal/render/components/conformance.go:139-152` — `writeConformanceMembers`:
  the set form, one `<code>` per member joined by `", "` (the join is the
  `if i > 0` at 144-146). This is what §4.8's `expected` / `observed` rows are,
  and what §4.5's thirteen chips would have to be built from.
- `internal/render/render.go:819` — the insertion point:
  `rendered := insertEngineBlockBeforeClose(buf.String(), string(components.ConformanceHTML(result)))`.
  The comment above it (`render.go:814-817`) states the rule: "Conformance is
  engine-owned generated evidence, not a replaceable presentation partial.
  Inserting it inside the claim root keeps the projection visible when a project
  overrides that entire partial and keeps it inside claim collapse."
- `internal/render/render.go:1413-1420` — `insertEngineBlockBeforeClose`, which
  finds the last `</section>` / `</article>`.
- `internal/render/render.go:482` and `:488` — the CSS and the status-fetch guard
  are appended only when the catalogue actually carries conformance results.

### 7.2 CSS

**The panel's stylesheet is not in `style.css`.** It is a Go string constant,
appended to the viewer CSS only for projects that have conformance results:

- `internal/render/conformance_view.go:8` — the marker comment:
  `/* Added only when this viewer contains structured conformance results. */`,
  the first line inside the `conformanceCSS` raw-string literal opened at line 7.
- `internal/render/conformance_view.go:9-22` — the whole rule set, fourteen
  lines: `.claim-conformance` (line 9), the ready/not-ready border colours (10-11),
  `.claim-conformance-head` (13), the marker suppression (14),
  `.claim-conformance-scope` (15), `.claim-conformance-check` (16), the per-state
  border colours (17-18), `.claim-conformance-check-head` (19),
  `.claim-conformance-line` (20-22).
- `internal/render/conformance_view.go:60-68` — `viewerCSSWithConformance`, which
  appends that constant to `style.css`'s bytes.
- `internal/render/conformance_view.go:33-58` — `conformanceStatusFetchGuardJS`,
  marker `/* dossierx-conformance-status-freshness */`.

Sections of `internal/render/viewer/template/style.css` this screen touches.
**`style.css` is 4870 lines and it is a two-layer file** — an early layout layer
and a "System Record" layer further down that re-declares several of the same
selectors at equal specificity and wins on source order. Every address below was
re-derived by locating its marker comment text, not by arithmetic, and every
pair names both layers where a pair exists. `grep -c conformance style.css`
returns **0**: this panel has no selector in `style.css` at all today (its CSS is
the Go constant above), so every address here is a *neighbour* a redesign must
live beside, not a rule it edits.

- `style.css:311-315` — the marker `/* status-draft/status-locked map to the pill
  family below … its one live rule sits with the pills in the System Record layer
  further down (\`.pill.pv, .status-draft\`), which is why there is no
  .status-draft rule here. */`. The early-layer pills are `.pill`
  `style.css:2399-2405`, `.pill.ps` `2407-2410`, `.pill.pw` `2416-2419` — with a
  comment at `2412-2414` recording that `.pill.pv` is deliberately **not**
  declared there. The **live** re-declarations that win are `style.css:4040-4066`:
  `.pill` `4040-4048`, `.pill.ps, .status-locked` `4050-4054`,
  `.pill.pv, .status-draft` `4056-4060`, `.pill.pw, .claim-review-pending`
  `4062-4066`. The verdict badge in §4.3 replaces these pills.
- `style.css:339-348` — marker `/* Inline \`code\` spans, produced anywhere
  markdown.Render's inline pass runs … NO background here. */`; the early rule
  `code { … }` at `349-354`. The **live** `code` rule is `style.css:4086-4091`
  under the marker at `4083-4085`, `/* The LIVE \`code\` background: this rule
  beats the inline-code pill rule near the top of the file at equal specificity,
  on source order. */`. Every machine-layer value is a `<code>` today, so this
  rule is what paints them.
- `style.css:356-378` (marker) and `379-392` (the layout half of the fenced-block
  rule), with the nested `pre code` reset at `394-397` / `398-416`; the live twin
  is `style.css:4093-4096` (marker) / `4097-4106`. Not used by this screen today;
  relevant only if board 12 ever ships.
- `style.css:880-888` — marker text ends "… leaves every one of them applying
  unchanged. */"; `.claim-links` at `889-893`, live twin at `4108-4114`. This is
  the claim footer `<details>` — the strip the checks chip would live in (R09.1).
- `style.css:895-897` — marker `/* Claim readiness is a reviewer-facing work
  queue. The policy engine remains authoritative; … */`; `.claim-readiness` at
  `898-904`. The sibling expansion whose auto-open behaviour R09.3 governs.
- `style.css:1272-1278` — marker `/* The summary is the disclosure's only click
  target and reads as a mono count strip ("4 links - 2 files - 1 drifted") … the
  UA disclosure triangle is deliberately KEPT … */`; `.claim-links-summary` at
  `1279-1284` and `.claim-links-summary:hover, .claim-links-summary:focus-visible`
  at `1286-1289`. The live twin is `.claim-links-summary` `4116-4125`, its
  marker-suppression pair `4127-4128`, and the hover rule at `4134`.
- `style.css:1301-1313` — marker `/* Deep-link auto-open (decision C9). Landing
  on #<claim-id> … */`, whose own text states it is "CSS ONLY, and the two rules
  below are its entire implementation anywhere in the codebase".
- `style.css:3366-3369` — marker `/* Nav-group Lucide hosts stay 14px. The
  evidence-footer chevron is the CSS border triangle the frozen theme-parity
  baselines paint — swapping it for a Lucide stroke repainted every claim card in
  an unthemed project (channel delta ~246, currentColor vs the card). */`;
  `.claim-footer__chevron` at `3376-3383`, the open-state rotation
  `.claim-links[open] .claim-footer__chevron` at `3390-3392`.
  **The boards draw the trigger chevron as a 12 × 12 inline SVG with a 2.6 stroke
  (§4.6), not as a border triangle. That is a direct conflict with the frozen
  theme-parity baselines named in this comment** — see §9, Open decision 3, which
  hangs on this address.
- `style.css:4129-4133` — marker `/* NOT --hover-bg, and deliberately so. This
  row is much larger than the other hover targets and takes a fainter wash: .07,
  not .08 … */`; the hover rule at `4134`. This is the only hover treatment in
  the neighbourhood and the closest precedent for §8's hover states.
- `style.css:4136-4151` — `.claim-footer__identity` (`4136`),
  `.claim-footer__title` (`4138`), `.claim-footer__counts` (`4141-4150`),
  `.claim-footer__chevron` (`4151`).
- Responsive neighbours: `@media (max-width: 860px)` at `4621-4733` already owns
  `.claim-links-summary` (`4726`) and `.claim-footer__counts` (`4728-4729`), and
  `@media (max-width: 560px)` at `4735-4746` owns them again
  (`.claim-links-summary` `4736-4741`, `.claim-footer__counts` `4743-4744`).
  A 520px rule for this screen must be written to win over both. **There is no
  640px rule in this neighbourhood** — the file's only `@media (max-width: 640px)`
  block, `2651-2671`, owns the graph console's `.gcp-*` rows and is unrelated.
- `style.css:4748` — marker `/* ---- print — THE @media print block
  ------------------------------------`, whose comment runs to `4808`;
  `@media print {` itself opens at `4809` and is the last block in the file
  (the file ends at 4870).
- `style.css:1848-1851` — marker `/* Coarse pointers (touch) need a >=44px hit
  target on the chip … */`, with the `@media (pointer: coarse)` block at
  `1852-1860` and `.comment-chip { min-height: 44px }` at `1853-1855`. This is the
  existing `(pointer: coarse)` precedent M13 follows. A second such block —
  marker `style.css:2298`, `@media (pointer: coarse)` at `2299-2309` — does the
  same for seven comment and collapse controls in one selector list, which is the
  shape a rule for this screen's two controls should take.
- `style.css:1248-1261` (`@media (max-width: 860px)`) and `style.css:1263-1270`
  (`@media (max-width: 520px)`) — the existing responsive blocks inside the
  readiness section, showing where a claim-level phone rule already lives.

`internal/render/viewer/template/graph.css` — **no ranges.** `grep -c conformance
graph.css` returns **0**; grepping `check` returns exactly two hits,
`graph.css:1035` and `graph.css:1112`, and both are prose inside comments about
`dossierx check`, not selectors. This screen paints nothing in the graph layer.

### 7.3 Runtime JS

- `internal/render/viewer/template/viewer-runtime.js:1263-1270` —
  `conformanceNotReadyIDs(claimIDs)`, which reads
  `.claim-conformance[data-implementation-ready="false"]` and its
  `data-claim-id`. This is the **only** runtime code that touches the panel
  today; it feeds the status strip, not the panel's own behaviour. The
  open/closed state is native `<details>` and has no JS.
- `internal/render/viewer/template/system-record.js:122-155` —
  `enhanceFooters()`, which rewrites `.claim-links-summary` into
  `.claim-footer__identity` + `.claim-footer__counts` + `.claim-footer__chevron`.
  Line 137 sets `claim-footer__counts`; lines 143-148 build each `count()` pill;
  line 151 appends the chevron. This is the footer the checks chip belongs to
  (R09.1 / R-F.1), and the function a nested-disclosure change must not disturb.
- `internal/render/viewer/template/system-record.js:157-165` —
  `enhanceFieldLabels()`, the precedent for relabelling engine-emitted rows in
  the browser. If the machine layer's keys are ever re-cased or re-ordered,
  this is the shape that does it.
- `internal/render/viewer/template/shell.html:313` —
  `{{if .ConformanceStatusGuardJS}}  <script>{{.ConformanceStatusGuardJS}}</script>`,
  closed by `{{end}}` on `314`. This is the **injection point** for the guard
  documented at `conformance_view.go:33-58`, and it is the file's only
  conformance reference. It sits immediately before the unconditional
  `<script>{{.ViewerRuntimeJS}}</script>` on the same line 314, so the guard is
  installed before the runtime that uses `fetch('/api/status')` ever loads —
  order that a redesign must not disturb. Nothing else in `shell.html` touches
  this screen.
- `internal/render/viewer/template/graph-ui.js`,
  `internal/render/viewer/template/build-order-ui.js` — **no address.** Neither
  file touches this screen.

### 7.4 Tests that assert on these selectors today

- `viewer-tests/conformance_test.go:112` — waits on
  `.claim-conformance-check[data-check-id="public-values"][data-conformance-state="mismatch"]`.
- `viewer-tests/conformance_test.go:114` — asserts the panel's text contains
  `blocked`, `paused`, `schema-version`, and that `data-shape === 'scalar'`.
- `viewer-tests/conformance_test.go:118-125` — reads every
  `.claim-conformance-line`'s trimmed `textContent` off the scalar check and
  asserts `data-implementation-ready === 'false'`. **Any change to the row
  markup breaks this test by construction.**
- `viewer-tests/conformance_test.go:134` — asserts the not-ready panel renders
  the uncheckable child.
- `viewer-tests/conformance_test.go:138-154` — the live/serve polls on
  `data-implementation-ready` and `data-conformance-state` counts.
- `viewer-tests/component_fit_test.go:98-118` —
  `TestReadyConformanceStaysInsideCollapsedClaim`: asserts a **ready** panel
  renders **closed** (`panel.open` must be false, line 105) and that it is not
  visible inside a collapsed claim.
- `viewer-tests/component_fit_test.go:190-198` — the 390px variant: a collapsed
  ready claim must hide implementation checks at phone width.
- `viewer-tests/theme_parity_test.go:1275` — the parity probe suppresses
  `.claim-conformance` (alongside `.claim-readiness`, `#statusStrip`,
  `.comment-chip`, `.dx-icon`) with `display: none !important`. **A redesign of
  this panel does not move the parity baselines, because the baselines never see
  it.** That is a freedom and a trap: parity will not catch a regression here.
- `internal/render/conformance_view_test.go:83` — asserts the CSS and guard are
  absent when there are no results.
- `internal/render/conformance_view_test.go:124-127` — counts
  `class="claim-conformance"` and `class="claim-conformance-check"` per layout.
- `internal/render/conformance_view_test.go:175` — asserts the block lands
  **inside** the claim's closing tag.
- `internal/serve/conformance_capacity_test.go:61`,
  `internal/serve/conformance_graph_scale_test.go:61`,
  `internal/check/conformance_scale_test.go:105`,
  `internal/check/conformance_test.go:109` — all count or assert on the literal
  string `class="claim-conformance"`. The **class name is load-bearing across
  four packages**; renaming it is a four-package change.

### 7.5 What does not exist in the engine today

Stated so a lane agent sizes the work honestly. None of the following has an
address, because none of it is implemented:

- No nested "How this was checked" disclosure. The nine rows are emitted flat,
  always visible whenever the panel is open.
- No `EXAMINED` / `COMPARED` / `FOUND` layer. There is no reviewer-facing
  sentence and no label column; `writeConformanceLine` is the only row shape.
- No coverage step strip. `writeConformanceMembers`
  (`components/conformance.go:139-152`) renders a set as comma-joined `<code>`
  spans.
- No snapshot hash in the panel, no `Copy as JSON` control.
- No `--font-serif` anywhere in the engine (`tokens.md` Disagreement 6), so every
  serif value in §4 — the explanation sentence, the `EXAMINED`/`COMPARED` values,
  `12 of 13 exercised`, all three rationale bodies — is currently
  unimplementable.
- No token for `#FBF7F7` / `#E8ECF1` / `#EFE6E5` (light blocked surface and
  hairlines) and none for `--color-dark-blocked-surface` /
  `-hairline` either (`tokens.md` Disagreement 10). The panel ground has nowhere
  to live in the 28-key allowlist.

---

## 8. States not on the boards

The boards draw one check, one verdict, one shape, two disclosure states. The
engine can render considerably more. For each, what this spec implies, derived
from the rules rather than invented.

1. **Verdict `Matched`** (`conformance.go:98`). Dot and label take
   `--color-locked` `#2C6B52` / `--color-dark-locked` `#63BE9A`, at the same
   13/16 weight 600 and 7px dot as §4.3. The panel ground drops the warm
   blocked tint and takes the neutral code ground (`--color-code-bg`
   `#F3F5F8` / `--color-dark-code-bg` `#1B212B`), because §4.3's `#FBF7F7` /
   `#1D1618` is the *blocked surface* and R08.1 forbids a green panel reading as
   a red one at a glance. Reference board 12's `Matched` badge is the same
   treatment (R12.2).
2. **Verdict `Uncheckable`** (`conformance.go:102`). The engine already groups it
   with `Mismatch` in `conformancePill` (`conformance.go:115`, the shared
   `case conformance.StateMismatch, conformance.StateUncheckable:`), so it takes
   `--color-blocked`, identically to §4.3. The distinguishing word is the label,
   not a third colour — R08.1.
3. **Verdict `Owed`** (`conformance.go:104`, pill `pv`). Not blocked and not
   approved: `--color-draft` `#9A6A16` / `--color-dark-draft` `#DDA94E`, and the
   panel ground takes the draft tint rather than the blocked one. Same reason the
   freshness Stale band takes draft amber (R10.2) — "not settled" is one meaning
   with one colour.
4. **`declared_none` — the claim deliberately has no software embodiment**
   (`conformance.go:24-26` writes `data-conformance-state="declared_none"`;
   `:39-45` is the branch that renders it). Today it renders the scope sentence
   plus two rows, `declaration: none` and `reason: …`. It has **no check article**, so
   there is no verdict, no `EXAMINED`/`COMPARED`/`FOUND` and no machine layer to
   disclose. The disclosure must not render at all — an empty "How this was
   checked" is worse than none. This is board 07's territory ("Claim — boundary,
   no embodiment") and 06a must not draw a second form of it.
5. **More than one check in a panel.** The eyebrow count (§4.3, `3KH-0`) is
   already a count; it reads `1` here and would read `3` there. Each check keeps
   its own `<article>` and therefore **its own** nested disclosure — the
   disclosure is per check, not per panel, because the snapshot hash and the nine
   keys are per check. R09.2's "one open at a time" governs the four expansions
   in the strip, **not** two disclosures inside one checks panel; the board says
   nothing about mutual exclusion here and a lane agent must not invent it.
6. **Shape `scalar`** (`data-shape="scalar"`, asserted in
   `viewer-tests/conformance_test.go:114`). There is no set to draw, so there is
   no coverage strip and no "N of M exercised" line. `FOUND` carries the observed
   value directly. `expected` / `observed` stay as two machine-layer rows;
   `missing` / `extra` are absent, because `conformance.go:80-85` only writes
   them when non-empty.
7. **`observation error` / `adapter message` rows** (`conformance.go:86-89`).
   Two further machine-layer keys the boards never draw. They take the same
   mono key/value treatment as §4.8 and slot in **after `extra` and before
   `detail`**, which is the order the emitter already writes. `observation
   error`'s value is a failure code and takes `--color-blocked`, the same
   treatment as `missing` (`2VR-0`).
8. **Empty `extra`.** Drawn on the boards as an em dash in `--color-faint`
   (`2VU-0`) — but the engine omits the row entirely when the slice is empty
   (`conformance.go:83`). The board shows a row the engine would not emit. Follow
   the board: a key present with an em-dash value says "the engine looked and
   found nothing", which an absent row does not. If the row is omitted instead,
   the nine-key order in §6 becomes variable and the panel stops being a fixed
   envelope.
9. **A ready panel, closed.** `viewer-tests/component_fit_test.go:105` asserts a
   ready panel renders **closed**. The boards only ever draw the not-ready,
   auto-opened form. The closed-panel form is: eyebrow, title, verdict, and
   nothing else — the summary row alone. R09.3 is the governing rule and the test
   already enforces it.
10. **A collapsed claim.** `component_fit_test.go:111-118` and `:193-198` assert
    the panel is not visible inside `.claim--collapsed`, at desktop and at 390.
    The nested disclosure inherits this: collapsing the claim hides everything,
    and re-expanding must restore the disclosure to whatever the reader left it
    at, not reset it to closed.
11. **Hover.** Not drawable in Paper. Both controls (§4.6/4.7 trigger, §4.8 Copy
    row) are text-only, so hover takes the existing footer treatment —
    `background: rgba(125, 137, 154, .07)` and `color: var(--ink)`, the rule at
    `style.css:4134` whose comment (`style.css:4129-4133`) explains why it is
    `.07` and not `--hover-bg`.
    The accent label must **not** darken on hover; the accent already means
    "this is a control" (`tokens.md`: "Navigation only. Never overlaps a status
    meaning").
12. **Focus-visible.** No board draws it. The engine's existing pattern is
    `.claim-links-summary:hover, .claim-links-summary:focus-visible`
    (`style.css:1286-1289`) — one treatment serving both. Follow it; do not
    invent a focus ring for this screen alone. **With one caveat this spec owes
    the reader:** that paired rule only sets `color: var(--ink)`. The live
    System Record rule at `style.css:4134` is `:hover` alone, so the *background*
    half of the hover treatment never reaches a keyboard user. Give this screen's
    trigger and Copy row the same background on `:focus-visible` as on `:hover`
    rather than reproducing that gap.
13. **Long values.** `target` is drawn at 95 characters and wraps to one extra
    line via `overflow-wrap: anywhere` and a `19px` leading (`2VG-0`). A value
    twice that long simply wraps further; **there is no truncation and no
    "show more" anywhere in the machine layer.** R09.8 demoted the target URI
    *behind* this disclosure precisely so it could be shown whole once a reader
    asks for it.
14. **A long check title, or a long claim title above it.** Not drawn. The title
    is `flex-grow: 1` against a `flex-shrink: 0` verdict group (`3KJ-0` /
    `3KK-0`), so the title wraps and the verdict keeps its lane. That is already
    the correct behaviour and must be preserved at 390, where the measure is
    358px.
15. **More than thirteen steps.** R-H.3's arithmetic (13 × 24 + 12 × 3 = 348 of
    358) has **no headroom past thirteen**. Fourteen chips do not fit at 390. The
    board does not answer this. Recorded as an open decision below rather than
    guessed at.
16. **`review_pending` triggers, blocked banners, cycles, zero-thread comment
    chips.** None of these appear on 06a and none of them belong to it. They are
    claim-level chrome (boards 04, 06, 07a, 14) and this screen is a component
    inside the claim body. 06a must not grow a status chip, a banner or a comment
    control.
17. **Print.** `tokens.md` §1: print always uses the light palette, and there is
    exactly one `@media print` block, last in the file (`style.css:4809`, under
    the marker comment at `4748-4808`; `internal/render/theme_tokens_test.go`
    fails if a second one appears). A
    `<details>` that is closed on screen is closed on paper. Whether the machine
    layer should be forced open for print is an open decision below.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

1. **Panel ground: keep the blocked-tinted surface, and name it.** The measured
   `#FBF7F7` (light) and `--color-dark-blocked-surface` `#1D1618` (dark) are a
   warm, near-invisible red wash that says "this check did not pass" without
   spending the blocked hue on a whole surface. `tokens.md` Disagreement 10
   records that the dark pair has no light twin and no engine token; this board
   proves the light twin exists and is just unnamed. **Decision:** treat
   `#FBF7F7` / `#E8ECF1` / `#EFE6E5` as the light twins of
   `--color-dark-blocked-surface` / `-hairline` and derive all six in the engine
   as `color-mix(in srgb, var(--warn) N%, var(--card-bg))` rather than appending
   to the closed 28-key allowlist. The alternative — shipping six raw hexes — is
   untheming a themeable viewer.
2. **The panel ground is verdict-dependent, not constant.** §8.1-8.3 assign a
   different ground per verdict. The boards only ever show the mismatch ground,
   so this is a derivation, not a measurement. **Decision:** ground follows
   verdict — blocked surface for `Mismatch`/`Uncheckable`, draft surface for
   `Owed`, neutral code ground for `Matched`. Flagged as derived so a reviewer
   can overrule it with one board.
3. **Chevron: use the boards' 12 × 12 inline SVG, and accept a parity
   baseline change.** `style.css:3366-3369` says in terms that the
   evidence-footer chevron is a CSS border triangle *because* a Lucide stroke
   "repainted every claim card in an unthemed project (channel delta ~246)".
   The boards draw a 2.6-stroke SVG chevron for this trigger (§4.6). **Decision:**
   this disclosure's chevron is a new control inside a panel that
   `theme_parity_test.go:1275` suppresses outright, so it cannot move a parity
   baseline. Use the SVG here. **Do not** change `.claim-footer__chevron` —
   that one is inside the baseline and the comment's warning stands.
4. **Fourteen or more coverage steps: wrap the strip is forbidden, so cap the
   run and state the overflow.** R-H.3's invariant is "the strip never wraps"
   and the 390 arithmetic has ten pixels spare at thirteen. **Decision:** past
   the number that fits, the strip shows the leading run plus a trailing mono
   `+N` marker in `--color-faint` at the chip's own height, and the full set
   stays readable in the machine layer's `expected` / `observed` rows, which are
   already there and already wrap. This preserves "the reader is looking for the
   one gap in a run" for the common case without wrapping and without dropping
   data. Flagged as derived — no board shows it.
5. **`extra` with no members renders as an em dash, not as an omitted row**
   (§8.8), against the emitter's current behaviour at
   `components/conformance.go:83`. The nine keys are an envelope; an envelope
   with a variable number of fields is not one.
6. **Print: the disclosure prints in whatever state the reader left it.** No
   board addresses it. Forcing it open would put nine rows of engine identifiers
   on paper for every check in the document, which is the exact outcome
   `2UO-0` ("nobody is made to read it") argues against.
7. **Mutual exclusion between two disclosures in a multi-check panel: none**
   (§8.5). R09.2 governs the four strip expansions; extending it downward is an
   invention.
8. **Board 12 stays out of scope.** R12.1 names this disclosure as the snippet's
   future home, and R12.4's WATCH caveat makes it opt-in per project. Neither is
   implemented here. 06a ships the disclosure empty of code.
9. **§7's code addresses are cited by marker text, not by line arithmetic.**
   Every `style.css` address in the first draft of this spec was short by roughly
   70-90 lines, and two of them named the wrong block outright — the spec had
   been written against an older `style.css`. All of §7.2 was re-derived at
   `3ac8844` by locating each marker comment's own words and reading the rule
   that follows it, and each address now names both the early-layer rule and its
   System Record twin where a twin exists. **Decision:** a future re-verification
   of this section greps the quoted marker text rather than trusting the number.
   If a marker's wording changes, the quoted text in §7.2 is what must be
   updated first; the line number is derived from it, never the other way round.
   **This is not a 06a-only problem and 06a cannot fix it alone.** `tokens.md`
   §8 row H3 and its Disagreement 16 cite the same `style.css` neighbourhood from
   the same stale snapshot — `4640-4641` / `4533` for the 860px block (actually
   `4728-4729` inside a block opened at `4621`), `4655-4656` / `4647` for the
   560px block (actually `4743-4744` inside a block opened at `4735`), `1175` for
   the readiness 520px block (actually `1263-1270`) and `2563` for the `.gcp-*`
   640px block (actually `2651`). 06a's §7.2 is now correct and deliberately
   disagrees with `tokens.md` on every one of those numbers. `tokens.md` is
   another group's file and is not edited from here; the disagreement is recorded
   so whoever holds it re-derives by marker rather than reconciling to 06a's
   numbers by hand. The *arguments* in both documents survive intact — only the
   addresses moved.
10. **A cited line must be the named thing, not merely near it.** The same draft
    cited `conformance.go:113` for a grouping that is on 115, and ranges that
    began on a previous function's closing brace. **Decision:** every function
    citation in §7.1 now spans `func` line to closing brace inclusive, and every
    single-line citation names the exact statement quoted beside it. Where a
    behaviour is spread over a `case` and its `return`, the `case` line is cited,
    and §7.1 says so explicitly so the convention is checkable.
11. **Node citations resolve to the leaf that carries the value.** The dark
    column of §4.3-§4.9 previously cited container frames (`7QQ-0`, `7R1-0`,
    `7R7-0`, `7U6-0`, `7UI-0`, `7V8-0`, `7VD-0`, `7VJ-0`) where a text or shape
    leaf was meant. Every value was inside the cited subtree, so nothing was
    wrong — but a lane agent cannot re-measure from a citation that does not
    resolve to one node with one value. **Decision:** all four boards now cite
    the leaf, and a container id appears only where the measured property
    (a `gap`, a `padding`, a `width`) genuinely belongs to the container; those
    are called out in the cell ("the eyebrow row frame", "the dot+label group").
12. **The one remaining measurement gap is closed.** §4.5 previously recorded the
    mobile blocked chip's label as "not measured". It is now read: `819-0`,
    Inter `12px / 16px`, weight `600`, `#9E3B36` — identical to desktop `3L0-0`.
    Measuring it is what surfaced Paper defect 12, so the gap was not harmless.

### Paper defects

Found on these boards, in order of how much damage implementing them as drawn
would do.

1. **The light board paints the machine layer `#F1F3F6` on a `#E3E8EE` hairline
   where `--color-code-bg` is `#F3F5F8` and `--color-border` is `#DDE2E9`**
   (`2V4-0`, `86T-0`). The **dark twin of the same component uses the tokens
   correctly** (`7U5-0`: `--color-dark-code-bg`, `--color-dark-border`; `8F6-0`
   likewise). One component, two spellings, split by mode. Implement the token.
2. **The light closed and open panels paint `#FBF7F7` on `#E8ECF1` with `#EFE6E5`
   hairlines — three raw hexes with no Paper token** (`3KD-0`, `3LZ-0`, `2UX-0`,
   `80G-0`, `83D-0`, `85N-0`, and every row border in `3KO-0`). The dark twins
   are properly tokenised (`7QP-0`, `7SI-0`, `7TZ-0` →
   `--color-dark-blocked-surface`, `--color-dark-blocked-hairline`). See Open
   decision 1.
3. **The dark panels' top border is `#212934`, a raw hex matching no token**
   (`7QP-0`, `7SI-0`, `8BO-0`, `8DJ-0`). It is neither `--color-dark-border`
   `#242C38` nor `--color-dark-blocked-hairline` `#3A2A2B`. On the dark board
   that is otherwise fully tokenised, this is the one leak.
4. **Coverage-chip tints are baked-alpha hexes and the two modes disagree on the
   alpha.** Light `#9E3B361F` (`3KZ-0`, `818-0`, `3LS-0`, `81Y-0`) is blocked at
   ~12 %; dark `#EC8A8329` (`7R7-0`) is ~16 %. The tokens are
   `--color-blocked-bg` at 10 % and `--color-dark-blocked-bg` at 13 %. **Neither
   board matches its own token**, and this is exactly the components-board
   pattern `tokens.md` Disagreement 12 already lists (`#9E3B361F` is named there
   verbatim).
5. **The numbered state badge is `font-weight: 700`** (`2WZ-0`, and its dark and
   mobile twins). `tokens.md` §3 states flatly: "There is no 700 in the set and
   none on the boards." There is one, on this board. It is scaffolding and does
   not ship, so the damage is to the foundations doc's claim rather than to the
   viewer — but the claim is now false and should be corrected to "none on the
   boards except 06a's state-label badges."
6. **The notes strip overstates the mobile hairline.** `8TO-0` column 3 says the
   open panel "gains a 1px top hairline it does not have on desktop". Measured,
   desktop's open check body `3LZ-0` carries `border-top: 1px solid #E8ECF1` and
   `7SI-0` carries `1px solid #212934`. Only the rounded wrapper (`2UQ-0`,
   `7SH-0`) is borderless. A lane agent reading the note alone would remove a
   hairline that is there.
7. **The notes strip documents only half the mobile padding change.** It says
   "panel padding drops from 32px to the 16px gutter"; the block padding also
   drops, `18/24` → `16/20` (`3KD-0` → `80G-0`), and the check-body gap drops
   `16px` → `14px`.
8. **The rationale heading's leading differs by mode-of-width with no stated
   reason:** `14/18` on desktop (`2WH-0`), `14/19` on mobile (`896-0`), same
   size, same weight. Not in the notes' list of mobile softenings. One of the two
   is drift; the desktop value matches the components-board register and should
   win.
9. **Tracking `+0.06em` and `+0.09em` are used without tokens.** `+0.06em` on
   every small-caps label (`3KQ-0`, `3KT-0`, `816-0`, `2WF-0`, `2WL-0`, `2WR-0`,
   `895-0`, `899-0`, `89D-0`) and `+0.09em` on the page eyebrow (`2UM-0`,
   `7QF-0`, `7VY-0`). `tokens.md` §3 already records both as untokened; this
   board is a heavy user of both.
10. **Colour spelling is split by mode inside one component.** The desktop light
    closed panel's subtree spells colour as raw hex throughout (`#101720`,
    `#54606F`, `#6E7C8E`, `#9E3B36`, `#1C4E8C`, `#FFFFFF`, `#DDE2E9` in
    `3KD-0`'s children) while its dark twin `7QP-0` spells every one as a token.
    The values are correct in both; only the spelling differs. Per R00.0 this is
    the exact drift the components board exists to prevent, appearing inside a
    single screen group.
11. **`--color-code-bg` is never used on these boards**, in either mode, even
    though the machine layer is precisely what that token names. The dark board
    uses `--color-dark-code-bg`; the light board uses `#F1F3F6`. Asymmetry (1)
    restated as a token-coverage gap.
12. **The coverage-chip label contradicts the components board, and R00.0 says
    the components board wins.** `reference-rules.md` R-H.3's own *Measured* line
    reads: blocked step "11/14 weight 600", ordinary step "11/14 weight 400 in
    `--color-muted`", citing components section I4 (`7N2-0`). Re-measured there,
    that is exactly what the components board draws — `7P4-0` Inter `11/14`
    weight `600` `var(--color-blocked)`, `7P6-0` Inter `11/14` weight `400`
    `var(--color-muted)`. **06a draws `12/16` in every one of its four boards**:
    desktop light `3L0-0` (w600) / `3L2-0` (w400), desktop dark `7RC-0` (w600) /
    `7RE-0` (w400), mobile light `819-0` (w600) / `81B-0` (**w500**), and the
    mobile dark twins. Two disagreements, not one: the **size/leading** differs
    at all four widths, and the mobile ordinary chip's **weight** is 500 where
    both the components board and 06a's own desktop say 400 — a change §5's M10
    records as intentional but which the components board does not sanction.
    §2 states the rule this board is held to ("Where this board and the
    components board disagree, the components board wins and this board is
    flagged"), so it is flagged here: **implement 11/14 from `7N2-0`**, and treat
    M10's chip-weight half as pending a components-board amendment rather than as
    a licence. The chip *geometry* is not in dispute — `7P3-0`/`7P5-0` are
    `24 × 28`, radius `5px`, `1.5px` and `1px` borders at a `3px` gap (`7P2-0`),
    identical to `818-0`/`81A-0`/`817-0`. Note also that the components board
    spells all four of those colours as tokens (`--color-blocked`,
    `--color-muted`, `--color-card`, `--color-border`) where 06a's light board
    spells them as raw hex — defect 10, seen from the source-of-truth side.
