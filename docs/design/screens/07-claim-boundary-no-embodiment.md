# 07 · Claim — boundary, no embodiment

Screen group 07 of the viewer design revamp. Source of record: Paper file
`01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`. Engine side read from the worktree
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp` at `3ac8844`.

Read this with `docs/design/tokens.md` (token names and their engine mapping)
and `docs/design/reference-rules.md` (the rules cited by id below). Every number
here was read from Paper with `get_jsx` / `get_computed_styles` /
`get_tokens`. Nothing is estimated from a screenshot; screenshots were used only
to check reading order and to decide what to measure.

Two standing rules apply to everything below. **Approval is not proof** — this
board shipped with defects, listed under "Paper defects", and a defect is
flagged, not copied. **Dark is expressed as dark tokens** — a light hex on a
dark board is a defect.

---

## 1. Boards

| Node | Name | Size | What it shows |
|---|---|---|---|
| `1Z5-0` | 07 · Claim — boundary, no embodiment | 1220 × 1244 | Desktop light. One LOCKED claim with **all four expansions open at once** — readiness blockers, relationships, sources, implementation checks — so the two absences (no governing doctrine, no software embodiment) can be read together. |
| `810-0` | 07 · … · MOBILE LIGHT | 390 × 1537 | The same claim at phone width, `height: fit-content`, no app bar. Every section survives; nothing is dropped. |
| `8I1-0` | 07 · … · DARK | 1220 × 1244 | Desktop dark twin of `1Z5-0`, node-for-node. |
| `8V1-0` | 07 · … · MOBILE DARK | 390 × 1537 | Mobile dark twin of `810-0`, node-for-node. |
| `7VQ-0` | Group 07 — band | 3420 × 27.5 | The group band: eyebrow "07 · CLAIM — BOUNDARY, NO EMBODIMENT" in `--color-accent`, 13/16 weight 600 tracking `0.08em`; caption "four designs of one screen · light pair left, dark pair right" in `--color-faint` 12/16; a 2px `--color-accent` rule 3420px wide. |
| `967-0` | Group 07 — notes | 2828 × 256 | Three note columns, 900px each, 64px gutters: *WHAT THIS BOARD IS*, *ROWS THAT BECOME LINES*, *ADDED, RESIZED, RE-WEIGHTED*. Read in full; paraphrased in §2. |
| `WI-0` | 09 · Placement map | reference | Binds R09.1–R09.9 on this screen. |
| `1F7-0` | 10 · Freshness | reference | Binds R10.1–R10.5; this screen carries no freshness line itself (no right rail) but inherits the vocabulary if one is added. |

**Viewer states depicted.** Claim status `locked` (padlock chip, no
`review_pending`). Readiness verdict **blocked** — 17 blockers across 2 modules,
grouped into two module routes (`Capability support` 12, `Audience boundary` 5).
Relationships: `governed_by: none` (stated, not empty), 3 dependencies, 1
dependent, all four targets `LOCKED`. Sources: exactly 1, cited once.
Implementation checks: **embodiment mode `none`** — declaration `none` plus a
reason sentence. Comments: **zero threads**, count `0`. Comments are *not* open
on any of the four boards; there is no sheet, no rail, no composer anywhere in
group 07.

---

## 2. Design intent

**What the reviewer should perceive.** This claim is a *boundary*: it says what
a module does not own. Two things a reader expects to find are deliberately
missing — no doctrine governs it, and there is no software to check it against.
The whole design problem of this screen is that **absent and empty look
identical**. The board's own lede (`22O-0`) states it: "The two absences are the
point. A claim with no governing doctrine and no embodiment must say so in
words, not by showing an empty section — absent and empty look identical
otherwise." So both absences are rendered as *sentences*, never as a section
that happens to have no rows:

- `GOVERNED BY` carries a `NONE` badge (`287-0`/`288-0`) **and** a serif
  sentence beneath it (`28A-0`) explaining why nothing governs it.
- `IMPLEMENTATION CHECKS` carries `DECLARATION → none` (`23P-0`) plus a
  `REASON` sentence (`23S-0`), and closes with the scope note `23U-0`: "Ready
  here means this claim deliberately declares no software embodiment — not that
  something was checked and passed."
- The checks chip in the footer reads **"No checks declared"** (`2JY-0`), not
  "0 checks".

The second thing the reviewer should perceive is that **a claim can be LOCKED
and blocked at the same time**. The head chip says LOCKED (`22G-0`, lock green);
the footer's first chip says Blocked with 17 blockers (`226-0`/`225-0`, blocked
red). These are two different questions — approval, and dependency readiness —
and the design deliberately puts them at two different altitudes: status in the
head, readiness in the strip.

### Rules from `reference-rules.md` that bind this screen

- **R09.1** — the four things under the prose are *four expansions of one
  metadata strip*, not four panels. The strip is `21J-0`: one row, 16px block
  padding, four chips then the comment count. Binds every board in this group.
- **R09.2** — exactly one expansion open at a time. **The desktop board shows
  all four open simultaneously.** The notes strip resolves this in its own
  words: "A spec board, not a device screen: one claim shown with every section
  already open, so the two absences can be read together." The board is a
  specimen of the four open forms, not a depiction of a legal runtime state.
  The engine must still enforce one-at-a-time.
- **R09.3** — no expansion auto-opens except readiness, and only when blocked.
  This claim *is* blocked, so `READINESS BLOCKERS` (`22V-0`) is the one
  expansion allowed to be open on load. Relationships, sources and checks are
  closed on load here despite being drawn open.
- **R09.4** — relationships is one section with three directions in fixed
  order: `GOVERNED BY` (`286-0`), `DEPENDS ON` (`28E-0`), `DEPENDED ON BY`
  (`28Y-0`). Not three sections, not three chips. The board obeys this exactly.
- **R09.5** — sources is its own expansion (`296-0`), split out of
  relationships. The board obeys.
- **R09.6** — Issues is a screen reached only from the blocked banner. This
  screen carries **no blocked banner** (the claim is blocked but the board draws
  no H1 notice), so there is no route to the Issues screen from group 07. See
  §8 — this is a gap the lane must close, not a licence to omit it.
- **R09.7** — the facet dot is the only project-level health signal in the
  reading view. Not applicable here: group 07 draws the claim card alone, with
  no TOC and no rail.
- **R09.8** — eight demotions. Two land on this screen: the claim id under each
  relationship row is **swapped for the title** (`28I-0` reads "The capability
  owner declares the requirement", not a slug), and raw diagnostics stay behind
  a provenance disclosure that this board does not draw.
- **R09.9** — no inline dependency map. The readiness expansion draws **two
  collapsed group rows** (`36Q-0`, `36Y-0`) with a hop breadcrumb
  ("nearest 2 hops" `36U-0`, "nearest 3 hops" `372-0`) and a **"See in claims
  graph"** link (`3BM-0`). No tree, no map. This is R09.9 implemented exactly:
  the breadcrumb states the path, the graph pane draws the shape.
- **R-F.1** — four chips in fixed order (readiness, relationships, sources,
  checks) then the comment count hard right. The board obeys: `222-0`, `21X-0`,
  `21T-0`, `2JX-0`, spacer `21O-0`, count `21K-0`.
- **R-F.2 / R-F.3 / R-F.4** — chip variants and the mobile two-row footer. See
  §4 and §5. R-F.2 needs a scope before it can be applied: **D14** splits it into
  a box that binds only below the phone breakpoint, a chevron direction that
  binds at every width, and a label treatment the desktop board states for
  itself. R-F.4's "two rows" is read through **R-I.3** in D2.
- **R-H.0** — only three components change shape below the phone breakpoint
  (H1 blocked notice, H2 claim header, H3 footer strip); everything else is the
  desktop component at a narrower width. This screen's mobile board changes the
  shape of **two components outside that set** — the source row (M11) and the
  implementation-checks label column (M12). That is a conflict on its face and
  it is resolved in §9 as **D13**, not waved through.
- **R-H.1** — the mobile blocked notice is an inset card with its own divided
  action row. Not exercised on these boards, because group 07 draws no blocked
  banner at all; that omission is **D8**, and when the banner is built it takes
  R-H.1's form, not a full-bleed band.
- **R-H.2** — on mobile the status chip moves above the title. The board obeys
  (`810-0`: chip, then title, then id).
- **R-I.0** — "Same content, same vocabulary, same order; only the arrangement
  moves … these are not alternative components." This is the rule behind §5's
  **"Nothing is hidden at 390"** and behind notes item 2's insistence that every
  stated absence survives the narrow width. It is also the rule that makes M11
  and M12 *restacks* rather than redesigns: the source row keeps its marker, its
  title, its publisher and its "cited once"; the checks block keeps
  `DECLARATION` and `REASON` in that order. Nothing is dropped, nothing is
  reworded, nothing is reordered — so the boards satisfy R-I.0 even where D13
  leaves them owing the components board a form.
- **R-I.2** — on mobile a relationship row's four columns become two lines with
  the badge right-ranged. The board obeys.
- **R-I.3** — "The chip is the component; the footer's two rows are
  composition." This is what actually settles **D2**: because the two-row
  arrangement is composition and not part of the chip, "two rows in the markup
  that wrap to three visually" is R-I.3-compliant by construction, and notes
  item 6's "three rows" is a description of the composition, not a claim about
  the component. D2 is a reading of R-I.3, not a judgement call.
- **R10.1 / R10.3** — elapsed-time vocabulary. No freshness line exists on this
  screen; if one is added it takes this vocabulary, one unit, never a timestamp.
- **R-J.*** — bottom-sheet policy. Not exercised: group 07 draws no sheet. If a
  comment sheet is opened from this screen it is the one shell in R-J.1, with a
  new body, never a second shell.
- **R00.0** — the components board wins over a screen board. Two places where
  this screen and the component board disagree are recorded under "Paper
  defects" below.

### Every note on the band and notes strip, paraphrased with intent

**Band `7VQ-0`.** "07 · CLAIM — BOUNDARY, NO EMBODIMENT — four designs of one
screen · light pair left, dark pair right." Intent: the four boards are one
screen in four renderings, not four screens. A rule stated on the light desktop
board binds the other three.

**Notes column 1 — WHAT THIS BOARD IS.**

1. "A spec board, not a device screen: one claim shown with every section
   already open, so the two absences can be read together. The mobile pair is a
   390px fit-content column with no app bar, matching group 06." Intent: the
   simultaneous-open state is a *reading device for the spec*, not a state the
   engine should produce. The absent app bar on mobile is a deliberate omission
   so the claim card can be measured without chrome, consistent with group 06.
2. "Nothing is dropped at 390px. Every section survives — the GOVERNED BY ·
   NONE block with its sentence, the four relationships, the one source, the
   DECLARATION none / REASON pair, and the closing 'Ready here means…' note.
   All three absences stay stated in words, never shown as an empty section: no
   governing doctrine, no embodiment, no checks declared." Intent: narrow width
   is never a reason to drop a stated absence. Dropping the sentence would turn
   "deliberately none" back into "empty", which is the exact failure the screen
   exists to prevent.

**Notes column 2 — ROWS THAT BECOME LINES.**

3. "Claim header: the LOCKED badge moves from the right of the title to above
   it, and the title takes the full gutter width." Intent: R-H.2. A 18px title
   on a 358px gutter cannot share a line with a badge without losing words.
4. "Relationship rows: desktop's four columns (dot · title · module · facet ·
   status) stack into two lines — title first, then module · facet with the
   status badge hard right. Badges stay at 11px." Intent: R-I.2. Right-ranging
   the badge keeps a lane for `LOCKED`/`DRAFT` without spending a fixed-width
   slot the title needs.
5. "Implementation checks: the 96px DECLARATION / REASON label column is
   removed; each label sits above its value in its own hairline-separated
   block." Intent: a 96px label gutter on 358px leaves 262px for prose. The
   hairline replaces the column as the thing that separates a label from the
   value beneath it.
6. "Footer strip: one text row becomes three rows of pills. Blocked with the
   comment count hard right, then 4 relationships · 1 source, then No checks
   declared alone on row three. It wraps rather than shorten the wording —
   vocabulary and order are unchanged at both widths." Intent: the wording is
   load-bearing (R-F.1: each chip is a noun and a count). Shortening "No checks
   declared" to fit would change what the screen says.
7. "Source rows: desktop's three columns become two. The [1] marker and the
   title stay on line one; 'cited once' leaves its own right-hand column and
   joins the publisher sub-line, right-ranged. The marker and 'cited once' go
   12px → 11px and the title's leading 23px → 20px." Intent: the citation
   marker and its title are the pair a reader scans for; the provenance is
   subordinate and can share a line.
8. "Blocker group rows keep desktop's shape unchanged; both names are short
   enough to fit at 358px." Intent: an arrangement change that is not needed is
   not made. This is the only expansion row form that does *not* restack.

**Notes column 3 — ADDED, RESIZED, RE-WEIGHTED.**

9. "Added: the footer chips gain a tinted fill (Blocked) and a 1px outline (the
   other three) that desktop does not have, and the Blocked pill's '17
   blockers' is blocked-coloured where desktop's is faint. Outlines are what
   makes a wrapped pill row legible as separate targets." Intent: on desktop the
   chips sit in one uninterrupted row, so whitespace separates them; once the
   row wraps, whitespace no longer reads as a boundary and each chip needs its
   own edge.
10. "Re-weighted: 'No checks declared' goes 400 → 500, per the rule that closed
    chips run one step heavier on a phone. No other weight changes — this
    board's desktop titles are already 600." Intent: 12px at weight 400 inside a
    pill is below the legibility floor on a phone.
11. "Resized: section padding 32px → 16px; lede serif 16/25 → 14/21; claim
    prose serif 17/28 → 16/26; the card loses its 12px radius, its side borders
    and the 40px paper inset and runs full-bleed with 16px gutters." Intent: at
    390 the card *is* the page; a radius and a paper inset spend 80px of gutter
    to draw a boundary the viewport already draws.
12. "Moved: 'See in claims graph' is right-aligned on desktop and left-aligned
    on mobile; the flex spacer desktop used for that alignment is gone." Intent:
    on a phone a right-ranged link at the end of a tall panel is far from the
    thumb and far from the content it refers to.

### Where the board contradicts its own notes

- Notes item 2 says "**All three absences**… no governing doctrine, no
  embodiment, no checks declared." The board's own lede (`22O-0`, both desktop
  and mobile, light and dark) says "**The two absences** are the point", and the
  eyebrow (`22P-0`) names two: "GOVERNED BY NOTHING · NO SOFTWARE EMBODIMENT".
  Two counts against three. Recorded as a Paper defect; resolved in §9.
- Notes item 6 says the mobile footer becomes "**three** rows of pills". R-F.4
  states the mobile footer is "**two rows**". Reconciled in §5.

---

## 3. Layout

### Desktop, 1440-class board drawn at 1220

| Value | Node | Measured |
|---|---|---|
| Artboard width | `1Z5-0` | `1220px`, `height: fit-content` (rendered 1244) |
| Artboard padding | `1Z5-0` | `40px` all round — the paper inset |
| Artboard gap (lede → card) | `1Z5-0` | `20px` |
| Artboard ground | `1Z5-0` | `#EFF1F4` (raw hex; equals `--color-paper`) |
| Content column | derived from `1Z5-0` | `1220 − 80 = 1140px` |
| Card | `1Z6-0` | `border-radius: 12px`, `border: 1px solid var(--color-border)`, `overflow: clip`, ground `#FFFFFF` |
| Card inner width | `228-0` etc. | `1138px` (1140 − 2 × 1px border) |
| Lede measure | `22O-0` | `max-width: 760px` |
| Eyebrow → lede gap | `22N-0` | `6px` |
| Head section padding | `228-0` | `30px` top / `22px` bottom / `32px` inline |
| Head section gap | `228-0` | `16px` |
| Head row gap (title block → chip) | `22E-0` | `16px`, `align-items: start` |
| Title → id gap | `22K-0` (per `get_jsx`) | `7px` |
| Prose measure | `229-0` | `max-width: 760px`, gap `16px` |
| Footer strip | `21J-0` | `margin-inline: 32px` (inset hairline), `padding-block: 16px`, `gap: 20px`, `border-top: 1px solid #E8ECF1` |
| Footer chip inner gaps | `222-0` / `21X-0` | `7px` (blocked chip) / `5px` (the other three) |
| Footer divider | `221-0` | `1 × 12px`, `--color-border` |
| Footer spacer | `21O-0` | flex `1` (rendered 446 × 0) |
| Readiness panel | `22V-0` | ground `#FBF7F7`, `border-top: 1px solid #E8ECF1`, padding `18px` top / `22px` bottom / `32px` inline, gap `12px` |
| Readiness head rule | `22Y-0` | `794 × 1px`, `#EBD9D8`; head gap `12px` |
| Readiness group row | `36Q-0`, `36Y-0` | `border-top: 1px solid #EBD9D8`, `padding-block: 11px`, `gap: 12px` |
| Readiness footer row | `3BF-0` | `padding-top: 12px`, `gap: 10px`; link block gap `6px` |
| Relationships panel | `27Y-0` | ground `#F6F8FA`, `border-top: 1px solid #E8ECF1`, padding `20px` top / `24px` bottom / `32px` inline, gap `14px` |
| Relationships head rule | `281-0` | `944 × 1px`, `#E3E8EE` |
| Direction header gap / lead | `283-0`, `28B-0`, `28V-0` | gap `7px`; `28B-0` and `28V-0` add `padding-top: 4px` |
| Relationship row indent | `28G-0` etc. | `padding-left: 19px`, gap `10px` |
| Governed-by sentence | `289-0` | `padding-left: 19px`, `max-width: 700px` |
| Sources panel | `296-0` | no ground (inherits card), `border-top: 1px solid #E8ECF1`, padding `20px` / `24px` / `32px`, gap `13px` |
| Sources head rule | `299-0` | `984 × 1px`, `#E8ECF1` |
| Source row | `29B-0` | gap `12px`; marker column `29C-0` fixed `26px` wide, `padding-top: 2px`; title block gap `3px`; trailing column `29H-0` `padding-top: 3px` |
| Checks panel | `23J-0` | `border-top: 1px solid #E8ECF1`, padding `20px` top / `26px` bottom / `32px` inline, gap `12px` |
| Checks head rule | `23M-0` | `896 × 1px`, `#E8ECF1` |
| Checks label column | `23O-0`, `23R-0` | fixed `96px`, `flex-shrink: 0`; row gap `16px` |
| Checks reason measure | `23S-0` | `max-width: 640px` |
| Closing scope note | `23T-0` | `border-top: 1px solid #E8ECF1`, `margin-top: 4px`, `padding-top: 4px`, `max-width: 760px` |

**Sticky and scroll regions: none.** Group 07 draws the claim card alone on the
page ground. There is no header, no left TOC, no right rail, no sticky element
and no internal scroll region on any of the four boards. Whatever sticky chrome
the reading view carries comes from group 02/03, not from here.

### Mobile, 390

| Value | Node | Measured |
|---|---|---|
| Artboard width | `810-0` | `390px`, `height: fit-content` (rendered 1537) |
| Artboard padding | `810-0` | **none** — the card is full-bleed |
| Lede block | `810-0` child 1 | padding `20px` top / `16px` bottom / `16px` inline, gap `6px` |
| Card | `810-0` child 2 | ground `--color-card`, `border-top` and `border-bottom` `1px solid var(--color-border)`, **no side borders, no radius** |
| Head block | card child 1 | `padding-inline: 16px`, `padding-top: 20px`, `gap: 9px`, `flex-direction: column`, `align-items: start` |
| Title → id gap | head inner | `6px` |
| Prose block | card child 2 | `padding-inline: 16px`, `padding-top: 14px`, gap `12px` |
| Footer strip | card child 3 | `margin-inline: 16px`, `padding-top: 13px`, `padding-bottom: 16px`, gap `8px` between the two rows, `border-top: 1px solid #E8ECF1` |
| Footer row 1 | footer child 1 | gap `10px`; blocked chip left, flex spacer `min-width: 4px`, comment count right |
| Footer row 2 | footer child 2 | `flex-wrap: wrap`, gap `8px` |
| Readiness panel | card child 4 | `padding: 16px` (uniform), gap `12px`, ground `#FBF7F7` |
| Relationships panel | card child 5 | `padding: 16px`, gap `12px`, ground `#F6F8FA` |
| Sources panel | card child 6 | `padding: 16px`, gap `13px` |
| Checks panel | card child 7 | `padding: 16px`, gap `12px` |
| Relationship row | rel rows | `padding-left: 19px` (unchanged from desktop), gap `10px`; dot `margin-top: 6px`; inner column gap `3px`; meta line gap `8px` |
| Source row | sources | gap `10px`; marker column `26px`; inner gap `2px`; sub-line gap `10px` |
| Checks block | checks | `border-top: 1px solid #E8ECF1`, `padding-block: 11px`, gap `3px`, **no label column** |
| Gutter | derived | `390 − 32 = 358px` |

---

## 4. Components on this screen

Colour is given as the `tokens.md` token name with the hex beside it. Where the
board spells a raw hex that a token already covers, both are shown and the raw
hex is flagged `(raw, = --token)`. Where the board spells a raw hex with **no**
token, that is recorded in §9.

**How to read the colour cells.** A bare token name on the light side means the
board itself spells `var(--token)`. A hex followed by `(raw, = --token)` means
the board spells the literal and the token already exists — same value, wrong
authoring. The two are not interchangeable: the first is correct, the second is
Paper defect 11, and a lane copying the second into CSS would hard-code a colour
that must re-point under `html[data-theme="dark"]`.

This distinction matters more on this screen than on most, because the light and
dark boards are split almost exactly down it. **45 of the 55 palette-coloured
text nodes on the desktop light board `1Z5-0` spell a raw literal where the dark
twin `8I1-0` spells the token**, and so does every light-mode SVG stroke. The complete node-by-node inventory
— read with `find_nodes` scoped to `1Z5-0`, six literals queried — is Paper
defect 11 in §9. Every table below flags each occurrence it names.

### 4.1 Board eyebrow — `22P-0` (desktop) / `810-0` head (mobile)

| Property | Desktop | Mobile |
|---|---|---|
| Family / size / leading | `--font-sans` Inter, `11px` / `14px` | same |
| Weight / tracking | `600` / `0.09em` (`22P-0`) | `600` / **`0.08em`** |
| Colour light | `--color-faint` `#6E7C8E` | same |
| Colour dark (`8I1-0` / `8V1-0`) | `--color-dark-faint` `#8494A8` | same |

The desktop tracking `0.09em` is off the scale (`--tracking-label` is `0.08em`);
mobile uses the token value. Recorded in §9.

### 4.2 Lede — `22O-0`

- Family `--font-serif` Source Serif 4; **desktop** `16px / 25px`, **mobile**
  `14px / 21px`; weight `400`; `max-width: 760px` (desktop only).
- Light `--color-muted` `#54606F`; dark `--color-dark-muted` `#9AA7B8`.

### 4.3 Claim title — `22M-0`

- Inter, **desktop** `20px / 26px` weight `600` tracking `-0.01em`; **mobile**
  `18px / 24px` weight `600` tracking `-0.01em`.
- Light `--color-ink` `#101720`; dark `--color-dark-ink` `#E6EAF0`.
- `20px/26px` is `--text-h2` at `--leading-title`. Tracking `-0.01em` has no
  token (see `tokens.md` §3).

### 4.4 Claim id — `22L-0`

- `--font-mono` IBM Plex Mono; **desktop** `12px / 16px`; **mobile**
  `11px / 15px`; weight `400`.
- Light `--color-faint` `#6E7C8E`; dark `--color-dark-faint` `#8494A8`.

### 4.5 Status chip, LOCKED — `22F-0` (frame), `22H-0` (padlock), `22G-0` (label)

| Property | Value |
|---|---|
| Ground light | `--color-locked-bg` `rgb(44 107 82 / 10%)` — **mobile board spells the raw `#2C6B521A`** |
| Ground dark | `--color-dark-locked-bg` `rgb(99 190 154 / 13%)` |
| Radius | `999px` (`--radius-pill`) |
| Padding | `4px` block, `8px` left, `10px` right |
| Gap | `5px` |
| Icon | `22H-0` 12 × 12 padlock, `stroke-width: 2.4`, `stroke-linecap: round`; light stroke `#2C6B52` (raw, = `--color-locked`), dark stroke `var(--color-dark-locked)` `#63BE9A` |
| Label | Inter `11px / 14px` weight `600` tracking `0.05em`; light `#2C6B52` (raw, = `--color-locked`), dark `--color-dark-locked` `#63BE9A` |
| Placement | Desktop: right of the title, `flex-shrink: 0`, cross-axis `start`. Mobile: **above** the title (R-H.2) |

States visible: **locked only**. No draft, no review-pending variant is drawn in
group 07 (board `47K-0` / `7W2-0`, group 07a, carries draft — out of scope here).

### 4.6 Claim prose — `22D-0`

- `--font-serif`; **desktop** `17px / 28px` (`--text-body` at `--leading-body`);
  **mobile** `16px / 26px`; weight `400`.
- Light `--color-ink` `#101720`; dark `--color-dark-ink` `#E6EAF0`.
- Desktop measure `max-width: 760px` on `229-0`.

### 4.7 Footer strip — desktop, `21J-0`

Four chips then the comment count hard right (R-F.1). On desktop the chips are
**bare text + chevron**, with no ground and no outline — the outline is a mobile
addition (notes item 9). R-F.2's variant table gives every chip a 30px pill with
a ground and a border; **that table is mobile-scoped** and does not contradict
what is measured here. See D14, which splits R-F.2 into a box (phone tier only),
a chevron direction (every width) and a label treatment (this screen's, on
desktop).

**Chip 1, readiness / blocked — `222-0`**

| Part | Node | Value |
|---|---|---|
| Dot | `227-0` | 7 × 7, `border-radius: 999px`, light `--color-blocked` `#9E3B36`, dark `--color-dark-blocked` `#EC8A83` |
| Label "Blocked" | `226-0` | Inter `13px / 16px` weight `600`, light `--color-blocked`, dark `--color-dark-blocked` |
| Count "17 blockers" | `225-0` | Inter `13px / 16px` weight `400`, light `--color-faint` `#6E7C8E`, dark `--color-dark-faint` `#8494A8` |
| Chevron | `223-0` | 12 × 12 chevron-**up** (open state), `stroke-width: 2.4`; light stroke `#9E3B36` (raw), dark `var(--color-dark-blocked)` |
| Gap | `222-0` | `7px` |

**Chips 2–4 — `21X-0` (4 relationships), `21T-0` (1 source), `2JX-0` (No checks declared)**

| Part | Node | Value |
|---|---|---|
| Label, chips 2–3 | `220-0`, `21W-0` | Inter `13px / 16px` weight **`500`**, light `--color-accent` `#1C4E8C`, dark `--color-dark-accent` `#6AA6E8` |
| Label, chip 4 | `2JY-0` | Inter `13px / 16px` weight **`400`**, light `#54606F` (raw, = `--color-muted`), dark `--color-dark-muted` `#9AA7B8` |
| Chevron | `21Y-0`, `21U-0`, `2JZ-0` | 12 × 12 up, `stroke-width: 2.4`, light `#1C4E8C` (raw, = `--color-accent`), dark `var(--color-dark-accent)` |
| Gap | each frame | `5px` |

Chip 4's label is muted, not accent, and weight 400, not 500 — because "No
checks declared" is a **stated absence**, not a count you can open onto rows.
That difference is the design intent, and it is what makes the fourth chip read
as quieter than the other three without introducing a new colour (R08.1).

**Comment count — `21K-0`**

| Part | Node | Value |
|---|---|---|
| Icon | `21M-0` | 14 × 14 speech bubble, `stroke-width: 2`, light raw stroke `#6E7C8E` (= `--color-faint`), dark `var(--color-dark-faint)` |
| Count "0" | `21L-0` | Inter `13px / 16px` weight `400`, light `var(--color-faint)` (**token, not raw** — one of only four text nodes on `1Z5-0` that spell the faint token), dark `var(--color-dark-faint)` |
| Gap | `21K-0` | `6px` |

Desktop is the **passive** variant per R-F.3: faint bubble, faint count, no
tint. The accent-tinted control variant is the mobile form — but see §9: this
board does **not** apply it.

### 4.8 Footer strip — mobile, `810-0` / `8V1-0`

**Row 1.** Blocked chip left, flex spacer, comment count right.

| Part | Value |
|---|---|
| Blocked chip ground | light `#9E3B3617` (raw, ≈ `--color-blocked-bg` at 9 %); dark `#EC8A8329` (raw, ≈ 16 %) |
| Chip box | `height: 30px`, `border-radius: 999px`, `padding-inline: 11px`, gap `7px`, no border |
| Dot | 7 × 7, `--color-blocked` / `--color-dark-blocked` |
| "Blocked" | Inter `12px / 16px` weight `600`, blocked colour |
| "17 blockers" | Inter `12px / 16px` weight `400`, **blocked colour** (desktop is faint — notes item 9) |
| Chevron | 11 × 11 up, `stroke-width: 2.6`, blocked colour |
| Comment count | icon 14 × 14 `--color-faint` / `--color-dark-faint`, label Inter `13px / 16px` faint, gap `6px` — **unchanged from desktop** |

**Row 2.** Three outlined pills, `flex-wrap: wrap`, gap `8px`.

| Part | Value |
|---|---|
| Ground | `--color-card` `#FFFFFF` / `--color-dark-card` `#161B22` |
| Border | `1px solid var(--color-border)` `#DDE2E9` / `var(--color-dark-border)` `#242C38` |
| Box | `height: 30px`, `border-radius: 999px`, `padding-inline: 11px`, gap `6px` |
| Label | Inter `12px / 16px` weight **`500`**, `--color-muted` / `--color-dark-muted`; `width: max-content` so a pill never splits (R-F.4) |
| Chevron | 11 × 11 up, `stroke-width: 2.6`, `--color-faint` / `--color-dark-faint` |

All three labels are weight 500 and muted on mobile — including "4
relationships" and "1 source", which are **accent** on desktop. The accent is
traded for an outline: an outlined pill already reads as a target, so the colour
is free to go back to neutral.

### 4.9 Readiness expansion — `22V-0`

| Part | Node | Value |
|---|---|---|
| Panel ground | `22V-0` | light `#FBF7F7` (**raw, no token**); dark `--color-dark-blocked-surface` `#1D1618` |
| Panel hairline | `22V-0` | light `#E8ECF1` (**raw, no token**); dark `#212934` (**raw, no token**) — the section hairline, six occurrences per dark board; D10 |
| Section label | `22X-0` | Inter `11px / 14px` weight `600` tracking `0.08em` (`--tracking-label`), light `#9E3B36` (raw, = `--color-blocked`), dark `var(--color-dark-blocked)` `#EC8A83` |
| Head rule | `22Y-0` | 1px, light `#EBD9D8` (**raw, no token**); dark `--color-dark-blocked-hairline` `#3A2A2B` |
| Head count | `22Z-0` | Inter `12px / 16px` weight `400`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| Row chevron | `36R-0`, `36Z-0` | 12 × 12 chevron-**right** (collapsed), `stroke-width: 2.4`, light `#54606F` (raw, = `--color-muted`), dark `var(--color-dark-muted)` |
| Row title | `36T-0`, `371-0` | Inter `14px / 18px` weight `600`, light `#101720` (raw, = `--color-ink`), dark `var(--color-dark-ink)` `#E6EAF0` |
| Row hop breadcrumb | `36U-0`, `372-0` | Inter `12px / 16px` weight `400`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| Row count pill | `36W-0`, `374-0` | ground light `#9E3B361A` (raw, ≈ 10 % blocked), dark `#EC8A8329` (raw); `border-radius: 999px`; padding `3px` block / `9px` inline |
| Row count label | `36X-0`, `375-0` (inside `36W-0` / `374-0`) | Inter `11px / 14px` weight `600`, light `#9E3B36` (raw, = `--color-blocked`), dark `var(--color-dark-blocked)` |
| Graph link icon | `3BI-0` | 13 × 13 git-branch, `stroke-width: 2`, light stroke `#1C4E8C` (raw, = `--color-accent`), dark `var(--color-dark-accent)` |
| Graph link label | `3BM-0` | Inter `13px / 16px` weight `500`, light `#1C4E8C` (raw, = `--color-accent`), dark `var(--color-dark-accent)` |

States visible: both group rows are **collapsed** (chevron right). No expanded
group row is drawn on any of the four boards. The expanded form must come from
the component board, not be invented here.

### 4.10 Relationships expansion — `27Y-0`

| Part | Node | Value |
|---|---|---|
| Panel ground | `27Y-0` | light `#F6F8FA` (**raw, no token**; nearest is `--color-code-bg` `#F3F5F8`); dark `#1B212B` (raw, **equals** `--color-dark-code-bg`) |
| Section label | `280-0` | Inter `11px / 14px` weight `600` tracking `0.08em`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| Head rule | `281-0` | 1px, light `#E3E8EE` (raw); dark `--color-dark-border` `#242C38` |
| Head count | `282-0` | **mono** IBM Plex Mono `11px / 14px`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| Direction icon | `284-0` ↑, `28C-0` ↓, `28W-0` → | 12 × 12, `stroke-width: 2.4`, `stroke-linejoin: round`; light stroke `#6E7C8E` (raw, = `--color-faint`; read from `get_jsx` on `283-0`), dark `var(--color-dark-faint)` |
| Direction label | `286-0`, `28E-0`, `28Y-0` | Inter `11px / 14px` weight `600` tracking **`0.06em`** (no token), light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| Direction count | `28F-0`, `28Z-0` | mono `11px / 14px`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` |
| NONE badge frame | `287-0` | ground light `#E3E8EE` (raw), dark `--color-dark-border`; `border-radius: 4px` (`--radius-sm`); padding `1px` block / `7px` inline |
| NONE badge label | `288-0` | Inter `11px / 14px` weight `600`, light `#54606F` (raw, = `--color-muted`), dark `var(--color-dark-muted)` |
| Absence sentence | `28A-0` | `--font-serif` `14px / 22px` weight `400`, light `#54606F` (raw, = `--color-muted`), dark `var(--color-dark-muted)`, `max-width: 700px` |
| Row dot | `28H-0`, `28M-0`, `28R-0`, `291-0` | 7 × 7 pill, `--color-blocked` / `--color-dark-blocked` |
| Row title | `28I-0`, `28N-0`, `28S-0`, `292-0` | Inter `14px / 18px` weight **`400`**, light `#1C4E8C` (raw, = `--color-accent`), dark `var(--color-dark-accent)`, `flex: 1` |
| Row meta | `28J-0`, `28O-0`, `28T-0`, `293-0` | Inter `12px / 16px` weight `400`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)`; text form `Module · Facet` |
| Row badge | `28K-0`, `28P-0`, `28U-0`, `294-0` | Inter `11px / 14px` weight `600` tracking `0.05em`; **light `#2C6B52` (raw, = `--color-locked`)**, dark twin `8L5-0` spells `var(--color-dark-locked)`; **fixed `width: 58px`, `text-align: right`, `justify-content: end`** |

The row dot is **blocked red on a LOCKED target**. That is deliberate and it is
the one thing on this screen most likely to be misread: the dot carries the
*dependency's contribution to this claim's readiness*, not the dependency's own
lifecycle. The lifecycle is the badge at the far right, and it says `LOCKED`.
A reviewer reading "red dot, LOCKED badge" is being told: this dependency is
approved, and it is still why you are blocked. If the engine cannot distinguish
those two facts, the dot must take the lifecycle colour and the readiness signal
must move — see §9.

On mobile the row becomes two lines (R-I.2): title `14px / 19px` weight `400`
accent; meta line `12px / 16px` faint with the badge right-ranged at `11px / 14px`
weight `600` tracking `0.05em` locked-green, **no fixed 58px slot** — the mobile
badge is `flex-shrink: 0` with `width: max-content` on the dark board.

### 4.11 Sources expansion — `296-0`

| Part | Node | Desktop | Mobile |
|---|---|---|---|
| Section label | `298-0` | Inter `11/14` w600 tracking `0.08em`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` | same |
| Head rule | `299-0` | 1px, light `#E8ECF1` (**raw, no token**) / dark `#212934` (**raw, no token**) — both resolved by D10 as `color-mix(in srgb, var(--border) 70%, var(--card-bg))` | same |
| Head count | `29A-0` | mono `11/14`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` | same |
| Marker `[1]` | `29D-0` | mono `12px / 16px`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)`, column `26px` | mono **`11px / 20px`** faint, column `26px` |
| Title | `29F-0` | `--font-serif` `15px / 23px` w400, light `#101720` (raw, = `--color-ink`), dark `var(--color-dark-ink)` | serif `15px / **20px**` |
| Publisher | `29G-0` | Inter `12px / 16px`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` | same |
| "cited once" | `29I-0` | Inter `12px / 16px`, light `#6E7C8E` (raw, = `--color-faint`), own right column | Inter **`11px / 16px`** faint, right-ranged on the publisher line |

### 4.12 Implementation checks expansion — `23J-0`

| Part | Node | Desktop | Mobile |
|---|---|---|---|
| Section label | `23L-0` | Inter `11/14` w600 tracking `0.08em`, light `#6E7C8E` (raw, = `--color-faint`), dark `var(--color-dark-faint)` | same |
| Head rule | `23M-0` | 1px, light `#E8ECF1` (**raw, no token**) / dark `#212934` (**raw, no token**) — see D10 | same |
| Field label | `23O-0`, `23R-0` | Inter `11px / 14px` w600 tracking **`0.07em`**, **light `#6E7C8E` (raw, = `--color-faint`)**; dark twin `8M8-0` spells `var(--color-dark-faint)`; fixed `96px` column | Inter `11/14` w600 tracking **`0.06em`**, faint, **above** its value; block `padding-block: 11px`, `border-top: 1px solid #E8ECF1`, gap `3px` |
| `DECLARATION` value | `23P-0` | mono `12px / 20px`, light `#54606F` (raw, = `--color-muted`), dark `var(--color-dark-muted)` | same |
| `REASON` value | `23S-0` | serif `14px / 22px`, **light `#54606F` (raw, = `--color-muted`)**, dark `var(--color-dark-muted)`, `max-width: 640px` | serif `14px / **21px**`, muted |
| Closing scope note | `23U-0` | serif `15px / 24px`, **light `#54606F` (raw, = `--color-muted`)**, dark `var(--color-dark-muted)`, `max-width: 760px` | serif **`14px / 22px`**, muted |

The `DECLARATION` value is **mono** and the `REASON` value is **serif** in the
same two-row block. That is the family rule doing real work (`tokens.md` §7):
`none` is the machine's word for what was configured; the reason is a human's
sentence about why. Rendering both in one family would erase the distinction.

### Count of measured values

**Recorded for the desktop light board (`1Z5-0`): 96 distinct measured values**
— 27 layout values in §3 plus 69 component values in §4 (family, size, leading,
weight, tracking, fill, stroke, radius, padding, gap, icon size and fixed widths
across the eleven components), each bound to a named node id. The dark board
`8I1-0` contributes 31 dark colour values; the mobile pair contributes 41
further values. Every one was read with `get_jsx` or `get_computed_styles`.

**Separately, a colour-*spelling* audit** was run with `find_nodes` scoped to
`1Z5-0`, one query per light literal (`#101720`, `#54606F`, `#6E7C8E`,
`#2C6B52`, `#1C4E8C`, `#9E3B36`). `find_nodes` reports a token-bound usage as
`var(--token)` and a literal as the hex, so the two are distinguishable. Result,
across the 55 text nodes those six queries reach: **45 spell the literal, 10
spell the token.** That inventory is Paper
defect 11 and it is a spelling finding, not a new measurement — every value it
touches is identical to what the tables above already record.

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. **No 390px breakpoint
is added** (`tokens.md` §8).

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | Card loses its `12px` radius, its side borders and the `40px` paper inset; runs full-bleed with `16px` gutters and keeps only top/bottom hairlines | `@media (max-width: 520px)` | Arrangement, not touch. Page chrome, not a component — **R-H.0's closed set is about components**, so this is outside it by construction (D13) |
| M2 | Section padding `32px` → `16px` uniform | `max-width: 520px` | Arrangement; section chrome, not a component (D13) |
| M3 | Status chip moves **above** the title; order is status → title → id (R-H.2) | `max-width: 520px` | Arrangement |
| M4 | Claim title `20/26` → `18/24`; id mono `12/16` → `11/15` | `max-width: 520px` | Arrangement |
| M5 | Lede serif `16/25` → `14/21`; claim prose serif `17/28` → `16/26`; both lose their `760px` cap | `max-width: 520px` | Arrangement |
| M6 | Footer strip becomes row 1 (blocked chip + comment count) and row 2 (detail pills, `flex-wrap: wrap`) — R-F.4 | `max-width: 520px`, **written to win over the existing `max-width: 560px` block, `style.css:4735-4746`, which today sets `.claim-links-summary { display: grid }` (4736) and three `.claim-footer__counts` / `__chevron` rules (4743-4745)** | Arrangement. Correction to `tokens.md` §8: the `max-width: 640px` block (`style.css:2651`) does **not** touch the claim footer — it owns `.gcp-*`. Only 560 does |
| M7 | Detail chips gain a `1px --color-border` outline and a `--color-card` ground; blocked chip gains a tinted fill; every pill is `height: 30px`, `radius: 999px`, `padding-inline: 11px` | `max-width: 520px` | Arrangement — an outline is what makes a wrapped row read as separate targets |
| M8 | "No checks declared" goes weight `400` → `500` | `max-width: 520px` | Legibility at 12px, tied to the wrap |
| M9 | Blocked chip's count goes faint → `--color-blocked` | `max-width: 520px` | Part of the tinted-fill treatment |
| M10 | Relationship row: four columns → two lines, badge right-ranged, no fixed `58px` slot (R-I.2) | `max-width: 520px` | Arrangement |
| M11 | Source row: three columns → two; "cited once" joins the publisher line right-ranged; marker `12/16` → `11/20`; title leading `23px` → `20px` | `max-width: 520px` | Arrangement. **Outside R-H.0's three components and outside section I's four row forms — see D13**, which keeps it (holding the third column at 390 wraps `SMAppService`) and raises the gap against the components board |
| M12 | Implementation checks: the `96px` label column is removed; each label sits above its value in a hairline-separated block (`padding-block: 11px`, gap `3px`) | `max-width: 520px` | Arrangement. **Also outside R-H.0 and section I — see D13**: a 96px gutter on 358px leaves 262px for a serif 14/22 `REASON` that is capped at 640px on desktop |
| M13 | "See in claims graph" moves from right-aligned to left-aligned; the desktop flex spacer is dropped | `max-width: 520px` | Arrangement |
| M14 | Readiness group rows keep desktop's shape — **no rule fires** | — | Notes item 8: an unnecessary change is not made |
| M15 | Every footer chip and the comment count must reach a **44px minimum hit target**, achieved by padding around the 30px pill box rather than by growing it | `@media (pointer: coarse)` | Target size is a pointer question, never a width question. The engine already sets `.comment-chip { min-height: 44px }` here (`style.css:1852-1855`) and pins the zero-thread chip back to `opacity: .75` because a phone has no hover (`style.css:1857-1859`) |
| M16 | Chevrons go `12px`/`stroke 2.4` → `11px`/`stroke 2.6` | `max-width: 520px` | Arrangement; the heavier stroke keeps a smaller glyph legible |

**Bottom sheets: none on this screen.** Group 07 draws no sheet, so R-J.1–R-J.7
are not exercised here. The policy still binds: if a comment sheet is opened
from this claim's comment count, it is **a new body in the one existing shell**
(R-J.1), with the fixed shell of R-J.2 — scrim `--scrim`, 16px top corners,
36 × 4 grabber in `--color-border-strong`, header hairline, plus the extra 1px
top hairline on dark — sized `240–660px` per R-J.3. A lane must not build a
second shell for this screen.

**Nothing is hidden at 390.** Explicitly: no section is dropped, no sentence is
truncated, no wording is shortened. This is the screen's own rule (notes item 2
and item 6), and it is **R-I.0** stated at screen scope — "same content, same
vocabulary, same order; only the arrangement moves." It outranks fit, and on
this screen it outranks R-H.0's closed set too, which is what D13 decides: a
dropped sentence here turns "deliberately none" back into "empty", the one
failure the screen exists to prevent.

---

## 6. Footer vocabulary

The metadata strip, quoted from the board in **left-to-right order**, identical
at 1440 and 390 (notes item 6: "vocabulary and order are unchanged at both
widths"):

1. `Blocked` `17 blockers` — `226-0` / `225-0`
2. `4 relationships` — `220-0`
3. `1 source` — `21W-0`
4. `No checks declared` — `2JY-0`
5. `0` (comment count, hard right) — `21L-0`

Readiness panel head, left to right: `READINESS BLOCKERS` (`22X-0`) … rule …
`17 across 2 modules` (`22Z-0`).

Readiness group rows: `Capability support` `nearest 2 hops` `12` (`36T-0` /
`36U-0` / `36W-0`); `Audience boundary` `nearest 3 hops` `5` (`371-0` / `372-0` /
`374-0`). Panel footer link: `See in claims graph` (`3BM-0`).

Relationships panel: `RELATIONSHIPS` `4` (`280-0` / `282-0`); directions
`GOVERNED BY` `NONE` (`286-0` / `288-0`), `DEPENDS ON` `3` (`28E-0` / `28F-0`),
`DEPENDED ON BY` `1` (`28Y-0` / `28Z-0`). Every relationship row's meta reads
`Permission readiness · Contract` and its badge reads `LOCKED`.

Sources panel: `SOURCES` `1` (`298-0` / `29A-0`); row `[1]` `SMAppService`
`Apple Developer Documentation` `cited once` (`29D-0` / `29F-0` / `29G-0` /
`29I-0`).

Implementation checks panel: `IMPLEMENTATION CHECKS` (`23L-0`); `DECLARATION`
`none` (`23O-0` / `23P-0`); `REASON` `This claim fixes a boundary of ownership.
It has no software embodiment to observe.` (`23R-0` / `23S-0`); closing note
`Ready here means this claim deliberately declares no software embodiment — not
that something was checked and passed.` (`23U-0`).

**No freshness line, no elapsed time, no build stamp appears anywhere in group
07.** Those live in the right rail (R10.1), which this screen does not draw.

**Engine wording today, for the diff the lane must make.** The engine's footer
summary reads `Evidence & relationships` / `Trace this claim through its
supporting record` with counts in the order **relationship, source, file,
drifted** (`system-record.js:122-155`). The board replaces that entirely: four
named chips, no "files" chip, no digest sentence. The engine's relationship
direction labels are `Governed By`, `Mirrors`, `Rests On`, `Depended On By`,
`Migrated From`, `Implemented In`, `Review Pending`, `Sources`
(`system-record.js:158-182`); the board's `DEPENDS ON` is a **rename of `Rests
On`**, and `Mirrors`, `Migrated From` and `Implemented In` have no home on this
board at all (see §8).

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.
Every address below was read, not inferred.

### 7.1 `internal/render/viewer/template/style.css` (4870 lines at `3ac8844`)

> Line numbers below were derived with `awk`, not `grep`. In this shell `grep`
> is aliased to `ugrep`, and on this file it reports line numbers that are wrong
> by a drifting offset. Verify with `awk 'NR==N'` before trusting any number a
> `grep -n` gives you here.

| Region | Lines | Section marker comment (quoted) | Relevance |
|---|---|---|---|
| Palette `:root` | `49`–`175` | `/* THE palette. One unconditional :root, light-first: the values below are what` | Where `--paper`, `--card-bg`, `--ink`, `--muted`, `--faint`, `--border`, `--border-strong`, `--link`, `--accent`, `--warn`, `--status-draft` are set |
| Dark re-point (toggle) | `176`–`201` | `@media screen {` → `html[data-theme="dark"]` | Explicit dark toggle |
| Dark re-point (OS) | `202`–` …` | `@media screen and (prefers-color-scheme: dark) {` | OS dark |
| Claim body prose | `332` | `.claim-body {` | Claim prose — today **no serif**, no `17/28` |
| Edges footer wrapper | `877`–`893` | `/* .claim-links is the <details> that wraps the whole edges footer` | `.claim-links` **889** (the docs-era rule, superseded at 4108) |
| Claim readiness | `895`–`1247` | `/* Claim readiness is a reviewer-facing work queue. The policy engine remains` | **The readiness expansion.** `.claim-readiness` **898**, `-head` **906**, `-id` **915**, `-title` **919**, `-state` **929**, `--ready` **942**, `--ready .claim-readiness-state` **946**, `-summary` **951**, `-counts` **957**, `-count` **963**, `-label` **982**, `-section-head` **993**, `-routes` **1019**, `-route` **1026**, `-route-id` **1054**, `-raw` **1224** |
| Readiness ≤860 | `1248`–`1262` | `@media (max-width: 860px) {` | Head stacks, `-route-via` hidden |
| Readiness ≤520 | `1263`–`1270` | `@media (max-width: 520px) {` | **The phone tier this screen's mobile rules land in.** Today it holds only `.claim-readiness-blocker { grid-template-columns: 1fr }` (1264) and `.claim-readiness-relation { white-space: normal }` (1267) |
| Footer summary | `1272`–`1294` | `/* The summary is the disclosure's only click target and reads as a mono` | `.claim-links-summary` **1279**, hover/focus **1286** |
| Edges reset | `1295`–`1300` | `.claim-links > .claim-edges {` | |
| Deep-link auto-open | `1301`–`1346` | `/* Deep-link auto-open (decision C9). Landing on #<claim-id> — from the` | `.claim:target .claim-links > *:not(summary)` **1331**, `::details-content` **1335** |
| Edges list | `1347`–`1437` | `.claim-edges {` | `> li` **1359** |
| Sources row | `1438`–`1676` | `/* ---- the sources row (components/sources.go) -------------------------` | `.claim-source-ref` **1477**, `.claim-source-meta` **1517**, `.claim-source-note` **1528**, `.claim-source-note-toggle` **1570**, `[hidden]` **1596**, hover **1600** |
| Comments chip | `1677`–`1847` | `/* ---- comments (engine-managed review threads) --------------------------` | `.comment-chip` **1690**, `--empty` **1731**, reveal-on-hover comment **1737**, `--empty:hover` **1752**, `-count` **1760** |
| Touch targets | `1848`–`1860` | `/* Coarse pointers (touch) need a >=44px hit target on the chip. They also` | **`@media (pointer: coarse)` at 1852 — where M15 lands.** `.comment-chip { min-height: 44px }` **1853–1855**, `.comment-chip--empty { opacity: .75 }` **1857–1859** |
| Comment controls, coarse | `2298`–` …` | `/* Coarse pointers (touch) need >=44px hit targets on every comment control. */` (2298) | Second coarse block at **2299** |
| Tablet tier | `2315`–` …` | `@media (max-width: 860px) {` | Rails collapse |
| Claim head `.k` | `2365`–`2398` | `/* The claim head line. v0.4.1 gives it two children instead of loose text:` | `.k` **2380** — where the status chip and comment slot live |
| Docs pill (legacy) | `2399`–`2430` | `.pill {` | `.pill.ps` **2407**, `.pill.pw` **2416** |
| Narrow tier | `2651`–` …` | `@media (max-width: 640px) {` | **Does *not* touch the claim footer** — it owns `.gcp-*` (2652+). Correcting the assumption in `tokens.md` §8 |
| Tablet tier | `3020`–` …` | `@media (max-width: 860px) {` | |
| **System Record** | `3138`–`4747` | `/* System Record visual language for the generated viewer. */` | **The live reading-view design. Everything below is inside it** |
| Widest desktop | `3245`, `3466` | `@media (min-width: 861px) {` / `@media (min-width: 1181px) {` | |
| Footer chevron | `3376`, `3390` | `.claim-footer__chevron {` (3376) | `.claim-links[open] .claim-footer__chevron` **3390** |
| Claim collapse | `4033`–`4038` | `.claim--collapsed .claim-collapse-chevron {` | The card's own disclosure, distinct from the four expansions |
| System Record pill | `4040`–`4075` | `.pill {` (re-declared) | `.pill.ps,` **4050** (lock green), `.pill.pv,` **4056** (draft amber), `.pill.pw,` **4062** (warn red) — the status chip's real colours |
| **Claim footer, live** | `4108`–`4176` | `.claim-links {` (**4108**) | `.claim-links` **4108** (`margin-top: 17px`, `border: 1px solid var(--border)`, `border-radius: 8px`, ground `color-mix(in srgb, var(--paper) 55%, var(--card-bg))`), `.claim-links-summary` **4116** (`min-height: 56px`, `padding: 11px 13px`, `650 10px/1.35 var(--font-mono)`), marker reset **4127–4128**, hover **4134**, `.claim-footer__identity` **4136**, `__title` **4138**, `__counts` **4141–4150**, `__chevron` **4151**, `> .claim-edges` **4153** |
| Relation labels | `4177`–`4190` | `.claim-relation-label::before {` (4177) | `.claim-relation-label` **4182** — the `GOVERNED BY` / `RESTS ON` eyebrows |
| Comment chip, live | `4224`–` …` | `.comment-chip {` | System Record re-declaration |
| Tablet | `4512`, `4621` | `@media (max-width: 860px) {` | |
| Narrow desktop | `4582`–` …` | `@media (min-width: 861px) and (max-width: 1180px) {` | Where the 1220 claim column sits |
| **560 tier** | `4735`–`4746` | `@media (max-width: 560px) {` | **The only query that owns the claim footer today:** `.claim-links-summary` → `display: grid` **4736–4741**, `.claim-footer__counts { justify-content: flex-start }` **4743**, `> span:not(.claim-footer__chevron) { padding: 4px 6px }` **4744**, `.claim-footer__chevron { margin-left: auto }` **4745**. A 520 rule must be written to win over these three |
| Print | `4748`–`4870` | `/* ---- print — THE @media print block ------------------------------------` | Comment 4748; `@media print {` **4809**. The one print block; light palette only |

**`.claim-conformance` has zero rules in `style.css`** — verified with
`awk '/claim-conformance/{n++} END{print n+0}' style.css` → `0`. The
implementation-checks panel is emitted (§7.4) but entirely unstyled, inheriting
UA `<details>` plus `.pill`. Everything in §4.12 is new CSS.

### 7.2 `internal/render/viewer/template/graph.css`

| Region | Lines | Relevance |
|---|---|---|
| File header | `1`–`160` | `/* graph.css — chrome and categorical palette for the DossierX claims graph pane.` — states the inverted convention: unconditional `:root` is **dark**, `@media (prefers-color-scheme: light), print` overrides |
| Dark `:root` | `161`–`235` | The nine design-named dark tokens are **declared in the engine**: `--color-dark-graph-facet-1: #7C8CE8` **180** … `-facet-5: #8E9AA8` **184**, `-other: #6C7F75` **185**, `-cycle: #F5615C` **186**, `-halo: #EFB44D` **187**, `-governed: #F06A9C` **188**. The ramp aliases them: `--dxg-facet-1: var(--color-dark-graph-facet-1)` **190** … `-5` **194**. Slots 6–20 from **200** |
| Light override | `236`–` …` | `@media (prefers-color-scheme: light), print {` **236**; `--color-graph-facet-1: #4257C4` **241** … `-governed: #C11F5B` **249**; aliases **251–255** |

The engine now declares the design's own nine names and reads them into the
`--dxg-*` ramp, with its own comment (lines 169–176) recording that Paper's
`--color-dark-graph-*` set is a defect and that `graph.css` wins. That matches
`tokens.md` § Disagreements 1–2.

Group 07 touches `graph.css` only through the **"See in claims graph"** link
(`3BM-0`), which does not exist yet. The link's destination is the graph pane;
its own colour is `--color-accent` → engine `--link`, from `style.css`, not from
`graph.css`.

### 7.3 Runtime and shell

| Address | What it is |
|---|---|
| `internal/render/viewer/template/viewer-runtime.js:1516` | `function renderClaimReadiness(assessments)` — **builds the whole readiness panel client-side**, `.claim-readiness` section, header, state word, summary sentence, counts |
| `viewer-runtime.js:1553-1555` | Blocker count: `String(facts.length)` + `' blocker'`/`' blockers'` — the engine's `17 blockers` |
| `viewer-runtime.js:1588` | `'Readiness blockers'` — the section label the board draws as `READINESS BLOCKERS` |
| `viewer-runtime.js:1455-1490` | `readinessRoute()` — the group row (`.claim-readiness-route`), its summary, `-route-via` (`'direct dependency'` / `'shown via this dependency'` — **cut by R09.8**) and `-route-count` |
| `viewer-runtime.js:1444-1453` | `readinessFactRow()` — `.claim-readiness-blocker` rows inside an open group |
| `viewer-runtime.js:1437-1442` | `readinessPathDetails()` — the dependency path disclosure |
| `viewer-runtime.js:1477-1487` | `claim-readiness-trace` + `claim-readiness-map` Mermaid block — **the inline dependency map R09.9 cuts** |
| `viewer-runtime.js:1493-1511` | `readinessRawDiagnostics()` — the raw JSON R09.8 demotes |
| `viewer-runtime.js:1598-1613` | Insertion point: the readiness box is inserted **before `.claim-links`** inside the card's collapse-content wrapper |
| `viewer-runtime.js:1641`, `:2141` | The two call sites of `renderClaimReadiness` |
| `viewer-runtime.js:1516-1614` | The whole function — the lane's edit surface for §4.9 |
| `internal/render/viewer/template/system-record.js:122-155` | `enhanceFooters()` — **rewrites the footer summary** into `.claim-footer__identity` + `.claim-footer__counts`; emits `Evidence & relationships` / `Trace this claim through its supporting record`; `count()` at 138–145; order relationship → source → file → drifted (146–149); chevron 151–153. **This is where the four-chip strip replaces the digest** |
| `system-record.js:158-182` | `enhanceFieldLabels()` — the direction eyebrows; `'Rests On'` at 162 is what the board renames to `DEPENDS ON`; `'Depended On By'` at 163; `'Governed By'` at 160; `'Sources'` at 167 |
| `system-record.js:186-199` | `setClaimExpanded()` / claim collapse — the card's own disclosure, distinct from the four expansions |
| `internal/render/viewer/template/shell.html:66-73` | The **only** graph trigger today: `<button id="dxgOpen" … data-dxg-open …> Claims graph</button>`. The board's per-claim `See in claims graph` link is new and must reuse this attribute contract |
| `internal/render/viewer/template/graph-ui.js:68` | `var OPEN_SELECTOR = '[data-dxg-open]';` — the contract a new per-claim link binds to |
| `internal/render/viewer/template/graph-ui.js:78` | `var BODY_OPEN_CLASS = 'dxg-open';` |
| `internal/render/viewer/template/graph-ui.js:3578` | `detailRow(rows, 'governed by', governorsOf(id).join(', ') \|\| 'nothing')` — the graph pane's own `governed by … nothing`, the closest existing precedent for the board's `GOVERNED BY · NONE` |
| `internal/render/viewer/template/build-order-ui.js:72,74,81,133-138` | Mermaid host wiring for `.claim-readiness-map` — dead weight once R09.9's map is cut |

### 7.4 Go emitters, `internal/render`

| Address | What it is |
|---|---|
| `internal/render/components/card.html:38-42` | The claim card: `<section class="claim claim-card card…">`, head `.k` with `{{claimLabel .ID}}` + status pill + `{{commentChip .}}`, `.claim-body`, `{{edges .}}` |
| `internal/render/components/card.html:39` | The head line — where §4.3/§4.4/§4.5 land |
| `internal/render/components/components.go:262-278` | `StatusLabel` — `"Locked"`, `"Draft"`, `"Review pending"`. The board's chip reads `LOCKED` (caps via CSS) |
| `internal/render/components/components.go:330-341` | `targetPillHTML` — the per-target status pill; **only emitted when the target is actionable**, so a `LOCKED` target renders **no** badge today. The board draws `LOCKED` on all four rows (§4.10) |
| `internal/render/components/components.go:346-377` | `EdgesHTMLWithLinks` doc comment — the footer's contract |
| `internal/render/components/components.go:378-569` | `EdgesHTMLWithLinks` body |
| `internal/render/components/components.go:388-434` | The `governed_by: none` branch — `<li class="claim-governed governed-none">governed_by: none` + reason, **and the rule that a bare `none` does not count as a link and suppresses the whole `<details>` when it is the only row**. This is the engine's existing "stated absence" mechanism and the board's `GOVERNED BY · NONE` + sentence is its redesign |
| `internal/render/components/components.go:437-449` | The `mirrors:` row (`if len(c.Mirrors) > 0` — **437**, `<li class="claim-mirrors">` — 439) and the `rests_on:` row (`if len(c.RestsOn) > 0` — **444**, `<li class="claim-rests-on">` — 446). `rests_on` is what the board renames to `DEPENDS ON` (§6) |
| `internal/render/components/components.go:451-455` | The `depended on by:` row — the block this screen's `DEPENDED ON BY` direction (§4.10) comes from. `if len(dependedBy) > 0 {` **451**; `links += len(dependedBy)` **452**; **the `<li class="claim-depended-by">depended on by:` string is written at 453**; `writeIDListItems(...)` **454**; the closing `</li>` **455**. *Correction:* an earlier draft of this table cited `455-458` for that `<li>`. That range is the closing `</li>` (455), the block's `}` (456), a blank line (457) and `if c.MigratedFrom != "" {` (458) — a different row entirely. Re-derived with `awk 'NR>=430 && NR<=470'`, per the §7.1 warning about `grep`. |
| `internal/render/components/components.go:506-575` | The `<details class="claim-links">` prologue and the summary digest `"N links - N files - N sources - N drifted"` |
| `internal/render/components/components.go:601-615` | `countSegment` — pluralisation |
| `internal/render/components/components.go:659` | `CommentChipHTML` — the comment count pill (§4.7) |
| `internal/render/components/sources.go:120-160` | `writeSourcesRow` — `<li class="claim-sources">sources:<ul class="claim-source-list">`, `.claim-source-ref` `[n]` |
| `internal/render/components/sources.go:162-207` | `writeExternalSource` — `.claim-source-title` anchor, `.claim-source-anchor`, `.claim-source-meta` |
| `internal/render/components/sources.go:188-198` | The meta join — host and `accessed <date>`. **There is no "cited once" / "cited N times" wording in the engine**; §6's `cited once` is new |
| `internal/render/components/sources.go:208-268` | `writeInternalSource` |
| `internal/render/components/sources.go:269-300` | `writeSourceNote` — the clamped note and its toggle |
| `internal/render/components/conformance.go:16-53` | `ConformanceHTML` — `<details class="claim-conformance" data-conformance-mode … data-implementation-ready …>`, summary `<strong>Implementation checks</strong>` + `Ready`/`Not ready` pill |
| `internal/render/components/conformance.go:25-26` | `data-conformance-state="declared_none"` — **the exact state this screen depicts** |
| `internal/render/components/conformance.go:38-43` | The embodiment-`none` branch: scope sentence `"Ready here means this claim deliberately declares no software embodiment. Claim and release readiness remain separate."` then `declaration → none` and `reason`. The board's closing note (§6) is a **rewrite** of this sentence: it drops the second clause and adds "— not that something was checked and passed" |
| `internal/render/components/conformance.go:122-146` | `writeConformanceValue` / `writeConformanceLine` / `writeConformanceMembers` — `<p class="claim-conformance-line"><span>LABEL</span>…` , the `DECLARATION` / `REASON` pair |
| `internal/render/render.go:465-469` | `attachEdgesOverride(tmpl.partials, buildImplinkLookup(cfg), buildDependedByLookup(cat), buildTargetStatusLookup(cat))` — how `depended on by` and target statuses reach the partials |
| `internal/render/render.go:803-825` | `renderClaimsWithBudget` — `insertEngineBlockBeforeClose(buf.String(), string(components.ConformanceHTML(result)))`; the checks panel is appended **after** the edges footer in the card |
| `internal/render/render.go:146` | `// ---- the claims graph pane's four injection sites ----` |
| `internal/render/render.go:482,488` | `viewerCSSWithConformance`, `statusFetchGuardWithConformance` |
| `internal/render/conformance_view.go:60,70` | `viewerCSSWithConformance` (60) and `statusFetchGuardWithConformance` (70) — the whole file is 75 lines |
| `internal/render/depended_by_view.go:38,73,127` | `buildDependedByLookup` (38), `buildTargetStatusLookup` (73), `attachEdgesOverride` (127) — the three lookups `render.go:469` binds into the partials |
| `internal/render/implink_view.go` | `implemented in:` rows — **no home on this board** (§8) |
| `internal/config/config.go:159` | `ThemeTokenAllowlist` — the closed 28-key list |
| `internal/render/theme_tokens_test.go` | Asserts every allowlisted token has a winning consumer in `style.css`/`graph.css` |

### 7.5 viewer-tests that assert on these selectors today

| Address | Assertion |
|---|---|
| `viewer-tests/claim_readiness_test.go:77` | `.claim-readiness-raw pre` |
| `viewer-tests/claim_readiness_test.go:82-92` | Counts `.claim-readiness`, `.claim-readiness-blocker`, `.claim-readiness-route`, `.claim-readiness-map pre.mermaid`, `.claim-readiness-map svg` |
| `viewer-tests/claim_readiness_test.go:117` | `WaitVisible("#widget\\.contract\\.l000-n00 .claim-readiness")` |
| `viewer-tests/claim_readiness_test.go:154-158` | Picks the route with the most `.claim-readiness-blocker` children |
| `viewer-tests/conformance_test.go:112` | `.claim-conformance-check[data-check-id="public-values"][data-conformance-state="mismatch"]` |
| `viewer-tests/conformance_test.go:114-125` | `.claim-conformance` text content and `.claim-conformance-line` list |
| `viewer-tests/conformance_test.go:134-154` | `.claim-conformance[data-implementation-ready="false"]` and the `matched`/`uncheckable` state counts |
| `viewer-tests/component_fit_test.go:72` | `#widget\.contract\.root .claim-readiness` |
| `viewer-tests/component_fit_test.go:104-111,190-194` | `.claim-conformance` presence, `open` state and panel box |
| `viewer-tests/claim_collapse_test.go:98-112` | `.claim--collapsed .comment-chip`, its rect, and a click on `.comment-chip` |
| `viewer-tests/empty_chip_test.go:39-45` | `.comment-chip--empty`, `.comment-chip-count` `=== "0"`, `aria-label === "add the first comment on this claim"` — **the zero-thread chip this screen depicts** |
| `viewer-tests/empty_chip_test.go:51-78,119-128` | Chip click, `aria-expanded`, `--open` transition, per-claim chip lookup |
| `viewer-tests/source_note_clamp_test.go:160-206` | `.claim-source-note-body`, `.claim-source-note-toggle` hit-testing |
| `viewer-tests/source_note_clamp_test.go:242,304` | Forces `details.claim-links` open — **breaks the moment the footer stops being one `<details>`** |
| `viewer-tests/source_note_clamp_test.go:317-338` | `.claim-source-note` clamp helpers |
| `viewer-tests/theme_parity_test.go` | Light/dark parity across the reading view |
| `viewer-tests/testdata/theme-parity/baseline-flat.html`, `baseline-basic.html` | Golden HTML containing the current claim-footer markup — **both regenerate when the footer changes** |
| `internal/render/components/components_test.go:909-924` | Asserts `"depended on by"` present / absent |
| `internal/render/components/comments_test.go:63-150` | `CommentChipHTML` exact markup, including `<span class="claim-comments-slot" hidden>` and `<span class="comment-chip-count">0</span>` |

---

## 8. States not on the boards

Each entry says what the spec implies, derived from the rules above. A lane must
not be left guessing.

1. **All four expansions closed (the load state).** This is the *normal* state
   and no board draws it. Per R09.1 the strip is one line at rest; per R09.3
   readiness may auto-open only because this claim is blocked. On a claim that
   is **not** blocked, all four are closed and the strip is the only thing
   between the prose and the next claim. Chevrons point **down** (R-F.2,
   closed); the boards draw them all **up**.
2. **One expansion open, the other three closed.** R09.2. The open chip takes
   the R-F.2 *open* variant — `--color-accent` border and label, weight `600`,
   chevron up in accent. **The boards draw no accent border on any chip**; they
   show the closed/blocked colours with up chevrons. The component board wins
   (R00.0): implement the open variant from R-F.2, not from this screen.
3. **Readiness group row expanded.** Both rows are collapsed on all four boards.
   The expanded form is the engine's `.claim-readiness-route-body` +
   `.claim-readiness-blockers` list (`viewer-runtime.js:1472-1476`); per R09.9
   it carries the blocker rows and their breadcrumbs, and **not** the Mermaid
   map. Row form comes from component I1 (R-I.1): title `14/20` w600, hop pill,
   stacked source and target slugs.
4. **`review_pending` on this claim.** `StatusLabel`
   (`components.go:262-265`) returns `"Review pending"` for locked +
   review_pending, and `EdgesHTMLWithLinks` writes a bare `open` server-side for
   it (`components.go` doc, lines 362-366). The status chip takes the
   `--color-draft` family (`.pill.pv` / `--status-draft`), **not** lock green,
   and readiness auto-opens per R09.3 only if the claim is additionally blocked.
   The three review_pending triggers are engine facts; the chip does not fork
   per trigger — one word, one colour.
5. **A `DRAFT` relationship target.** Every row on this board is `LOCKED`. A
   draft target takes `--color-draft` `#9A6A16` / `--color-dark-draft` `#DDA94E`
   for both the badge and — per R-I.2's "dot 7px in the lifecycle colour" — the
   dot, *if* the dot carries lifecycle. See §9 decision D3. The desktop badge
   slot is a fixed `58px`; `DRAFT` is five characters against `LOCKED`'s six and
   fits. `REVIEW PENDING` does **not** fit 58px — the desktop badge must be
   allowed to grow, or the slot widened, before that state ships.
6. **A named `governed_by` target.** This claim is `governed_by: none`. When a
   doctrine hub is named, the `NONE` badge is replaced by a relationship row
   (dot, title, `Module · Facet`, badge) and the serif absence sentence
   (`28A-0`) **is not rendered** — a sentence explaining why nothing governs it
   would be false. The engine already branches here
   (`components.go:388-434` vs `435-456`).
7. **Zero relationships / zero sources.** The chip is a noun and a count
   (R-F.1), so it reads `0 relationships` / `0 sources` — **not** hidden.
   Compare the checks chip, which reads `No checks declared` rather than
   `0 checks`, because a *declared* absence is a different fact from a count of
   zero. Rule for the lane: **a count of zero keeps the count wording; a
   declared absence gets the prose wording.**
8. **Zero-thread comment chip.** This board draws `0` (§4.7) and the engine
   already has the state: `comment-chip--empty`, reveal-on-hover
   (`style.css:1737-1751`, the reveal-on-claim-hover rule), `aria-label` `"add the first comment on this
   claim"` (`empty_chip_test.go:45`). At `pointer: coarse` there is no hover, so
   per R-F.3 the chip is always visible on a phone — which is what the mobile
   board shows.
9. **Comments open.** No board in group 07 opens comments. On desktop the rail
   form (group 14) applies; on mobile the one bottom sheet (R-J.1) with the
   comment-thread body. The comment count switches to the R-F.3 accent variant
   as the control that opens it.
10. **Blocked, with the banner.** R09.6 makes the blocked banner the only route
    to the Issues screen. **This claim is blocked and the board draws no
    banner.** The spec implies the banner is a claim-level component that group
    07 simply did not draw, in its mobile form (R-H.1: inset card, 16px side
    margins, 10px radius, 1px tinted-red border, divided 38px action row,
    `Show issues` label). A lane implementing group 07 must not conclude that a
    blocked claim has no banner.
11. **Not blocked / ready.** The first chip loses its red dot and reads the
    ready vocabulary; `--color-locked` is its colour. Readiness does **not**
    auto-open (R09.3). The engine's ready branch already exists:
    `.claim-readiness--ready` (`style.css:942-950`) and the `'Ready'` state word
    (`viewer-runtime.js:1537-1538`).
12. **Embodiment mode other than `none`.** This screen is the `declared_none`
    branch. With real checks, `DECLARATION` / `REASON` are replaced by
    `.claim-conformance-check` articles
    (`conformance.go:55-95`) and the scope sentence becomes "Ready here means
    every declared check matches" (`conformance.go:47`). The board's closing note
    is specific to `none` and must not be reused verbatim for the checked case.
13. **`mirrors`, `migrated_from`, `implemented in`.** All three are real engine
    rows — `mirrors` at `components.go:437-442` (`<li class="claim-mirrors">`
    439), `migrated_from` at `458-463` (`<li class="claim-migrated">` 460),
    `implemented in` in `implink_view.go` — and **none has
    a home on this board**. Per R09.4 relationships is exactly three directions,
    so these belong under the provenance disclosure R09.8 creates, not as a
    fourth direction and not as a fifth chip. This is the largest unresolved
    placement question on the screen; recorded as decision D5.
14. **Drifted implementation link.** `.pill.pw` `drifted`
    (`components.go:499`) and the server-written `open` attribute. No board
    depicts it. Per R09.8 it rides with `implemented in` behind the provenance
    disclosure, but the auto-open signal it carries conflicts with R09.3, which
    reserves auto-open for readiness. Recorded as decision D6.
15. **Long title, long id, long slug.** The card is `overflow: clip` and every
    board carries `overflow-wrap: anywhere`. The desktop title has no
    `max-width`; the id is mono and will break mid-token. On mobile the title
    takes the full 358px gutter (notes item 3). A relationship row's title is
    `flex: 1` beside a `12px` meta and a fixed `58px` badge, so it is the part
    that gives.
16. **Many relationships / many sources.** Four rows and one source are drawn.
    Nothing on the board caps either list. The engine caps nothing here either;
    the readiness map caps at 12 (`viewer-runtime.js:1485`), and that map is cut
    by R09.9. Decision: **no cap on this screen** — the expansion is opened
    deliberately, and truncating what the reader asked for is the opposite of
    what an expansion is.
17. **Cycles in the dependency graph.** `--color-graph-cycle` exists
    (`graph.css`), and R09.9 exists precisely because a readiness chain is a DAG
    that a tree would falsify. This screen draws no cycle affordance; a cycle
    surfaces in the graph pane, reached by the `See in claims graph` link.
18. **Print.** `style.css:4809` is the one `@media print` block (its section
    comment opens at 4748) and it pins the light
    palette. The four expansions have no print state on any board. Implied: in
    print a `<details>` that is closed prints closed, so a printed claim would
    lose its readiness, relationships, sources and checks. Recorded as decision
    D7.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

- **D1 — "Two absences" or "three"?** The lede, the eyebrow and the board's
  title all say **two** (no governing doctrine, no software embodiment); notes
  item 2 says three, counting "no checks declared". **Decision: two.** "No
  checks declared" is not a third absence — it is the footer chip's wording for
  the second one. The `IMPLEMENTATION CHECKS` panel and the checks chip describe
  the same missing embodiment at two altitudes. Ship the lede's wording; treat
  the notes' "three" as a miscount.
  **A third number appears on the board and is not a contradiction.** The claim
  prose (`22D-0`) opens "Six things a reader might expect to find here are not
  here…" and the governed-by sentence (`28A-0`) opens "This claim states where
  six responsibilities sit…" — both read from Paper, both on all four boards.
  Six is a count of **responsibilities this claim routes elsewhere**, inside the
  claim's own body text; two is a count of **structural absences in the record**
  — no governing doctrine, no software embodiment. Different register, different
  subject, different altitude. They are reconciled here explicitly so the next
  reader does not re-open D1 on finding a "six" three lines below a "two": the
  only miscount is the notes' "three".
- **D2 — Two rows or three in the mobile footer?** R-F.4 says two; notes item 6
  says three. **Decision: two rows in the markup, wrapping to three visually.**
  Row one is the status chip plus the comment count; row two is the detail
  chips with `flex-wrap: wrap`. At 390 with these three labels the wrap
  produces three visual rows, which is what the board draws. R-F.4's
  "each one fits whole" is satisfied by `width: max-content` on each pill. A
  lane must not hard-code three rows.
- **D3 — What does the relationship row's dot mean?** The board paints it
  `--color-blocked` on four `LOCKED` targets. **Decision: the dot carries the
  target's contribution to *this* claim's readiness; the badge carries the
  target's own lifecycle.** This preserves the board as drawn and keeps R-I.2's
  "dot in the lifecycle colour" as the mobile-component statement of a
  *different* dot. If the engine cannot compute the per-edge readiness
  contribution, the dot must fall back to the lifecycle colour rather than be
  painted red unconditionally — an unconditional red dot beside a green `LOCKED`
  badge is a false statement.
- **D4 — All four expansions open.** **Decision: the board is a specimen.** It
  documents the four open forms side by side; R09.2 still binds the engine to
  one at a time, and R09.3 makes readiness the only one open on load, and only
  because this claim is blocked.
- **D5 — Where `mirrors`, `migrated_from` and `implemented in` go.**
  **Decision: behind the provenance disclosure R09.8 creates, not as a fourth
  relationship direction and not as a fifth footer chip.** R09.4 fixes
  relationships at exactly three directions and R-F.1 fixes the strip at exactly
  four chips; both are stated as closed sets. A lane that needs them visible
  raises it against the components board, not against this screen.
- **D6 — Drifted auto-open.** The engine writes `open` server-side for a drifted
  link (`components.go` doc, 362-366); R09.3 reserves auto-open for readiness.
  **Decision: drop the drifted auto-open when the footer becomes four
  expansions.** A drifted link is a provenance fact behind a disclosure, and
  R09.3 is stated as an exclusive ("Readiness is the **only** one that may
  auto-open"). The drift signal survives as the `.pill.pw drifted` badge.
- **D7 — Print.** **Decision: in print, every expansion prints open.** R09.1's
  "one line at rest" is a rule about a reader's attention on screen; paper has
  no click. The `@media print` block (`style.css:4809`) already pins the light
  palette and is the right place. This is a decision, not something the boards
  state.
- **D8 — No banner on a blocked claim.** **Decision: the banner exists and
  group 07 did not draw it.** R09.6 makes it the only route to Issues, so a
  blocked claim without one is unreachable. Implement it per R-H.1 / component
  H1; do not take its absence here as a rule.
- **D9 — The comment count on mobile.** R-F.3 says the mobile count takes
  `--color-accent-bg` with an accent icon and label because it becomes the
  control that opens the sheet. **The mobile boards keep the desktop faint
  form.** **Decision: R-F.3 wins (R00.0 — the component board beats a screen).**
  Implement the accent variant at `max-width: 520px`, spelled as
  `color-mix(in srgb, var(--link) 9%, transparent)` per `tokens.md` § Open
  decisions, and flag the boards.
- **D10 — Section grounds have no tokens.** `#FBF7F7` (readiness),
  `#F6F8FA` (relationships), `#EBD9D8` (readiness hairline), `#E3E8EE`
  (relationships hairline), `#E8ECF1` (section hairline) are raw, and none maps
  to a light token. **Decision:** derive each from a token by `color-mix` rather
  than adding a key to the closed allowlist — readiness ground from
  `--warn-bg` over `--card-bg`, relationships ground from `--code-bg`
  (`#F3F5F8`, three units from `#F6F8FA`), hairlines from `--border`. The dark
  twins already have tokens (`--color-dark-blocked-surface`,
  `--color-dark-blocked-hairline`, `--color-dark-code-bg`), which is evidence
  the light side simply was not tokenised.
  **The dark section hairline `#212934` is added to this decision.** It was
  missing from the first draft and it is the one raw dark value with **no** token
  at all: it paints all six section hairlines on `8I1-0` (`8J0-0` footer strip,
  `8JP-0` readiness, `8KJ-0` relationships, `8LQ-0` sources, `8M3-0` checks,
  `8MD-0` closing note — read with `find_nodes` on `border-*` scoped to `8I1-0`)
  and the same hairline on `8V1-0`. It is **not** `--color-dark-border`
  `#242C38`; it is three to four units darker on every channel, so spelling it
  as that token would change the drawing. **Decision: derive it, do not add a
  token.** The section hairline in both modes is one step softer than the border
  token *toward the card ground*, and it resolves almost exactly:
  `color-mix(in srgb, var(--color-dark-border) 80%, var(--color-dark-card))` is
  `#212934` to the byte, and
  `color-mix(in srgb, var(--color-border) 65%, var(--color-card))` is `#E9ECF1`,
  one unit of red off the light board's `#E8ECF1`. The engine writes it **once**,
  against the mode-agnostic `var(--border)` / `var(--card-bg)` pair at a single
  ratio; `70 %` lands within three units of both boards and is the value to ship.
  That keeps `#212934` out of the closed 28-key allowlist
  (`internal/config/config.go:159`) for the same reason the five light grounds
  stay out, and it means the light and dark hairlines stop being two unrelated
  literals and become one rule read twice.
- **D11 — `See in claims graph` has no engine counterpart.** Today the only
  graph entry is the global header button (`shell.html:73`). **Decision:** the
  new per-claim link reuses the `[data-dxg-open]` attribute contract
  (`graph-ui.js:68`) and additionally carries the claim id, so the pane can open
  focused. Do not invent a second open mechanism.
- **D12 — `cited once` has no engine wording.** `writeExternalSource`
  (`sources.go:188-198`) emits host and `accessed <date>`, nothing about
  citation frequency. **Decision:** `cited once` / `cited N times` is derived
  from the count of `[n]` markers in the claim body (`claimCitations`,
  `sources.go:96`), computed at render time, never authored.

- **D13 — M11 and M12 restack components that R-H.0 does not license.**
  R-H.0 is a closed set: "Three components change shape below 390px" — H1
  blocked notice, H2 claim header, H3 footer strip — "everything else on a
  mobile board is the desktop component at a narrower width." M3/M4 (claim
  header) and M6–M9 (footer strip) are inside that set. **M11 (source row: three
  columns → two) and M12 (checks: the 96px label column removed, label above
  value) are not**, and neither is one of section I's four row forms — I1
  blocker row, I2 relationship row, I3 footer chips, I4 coverage chip. M10 is
  I2; M14 is the explicit no-change. So this screen restacks two components that
  no rule currently covers, and saying nothing about it would be the spec
  copying an unlicensed change.
  **Decision: keep the boards as drawn and raise the gap against the components
  board — do not delete the restacks, and do not quietly widen R-H.0 from this
  file.** Two reasons, both measurable from §3 and §4. A 96px label gutter on a
  358px width leaves 262px for the `REASON` sentence, where the desktop `REASON`
  is already capped at 640px (`23S-0`); collapsing to 262px would set a serif
  14/22 paragraph at roughly thirty characters a line. And the source row's
  third column exists only to right-rank "cited once" (`29I-0`); holding it at
  390 forces `SMAppService` (`29F-0`, serif 15) to wrap. Dropping either restack
  therefore means dropping or truncating content, which R-I.0 forbids outright.
  R-I.0 and R-H.0 cannot both be satisfied at 390 with the desktop arrangement,
  and R-I.0 is the rule about what the screen *says* — on this screen, whose
  entire problem is that a dropped sentence turns "deliberately none" back into
  "empty", content wins over form. The components board owes two more mobile
  forms. **M1 and M2 are not part of this gap:** the card shell and section
  padding are page chrome, not components, and R-H.0 is stated about components.
- **D14 — R-F.2's variant table is mobile-scoped.** R-F.2 gives all three chip
  variants a 30px pill with `--radius-pill` and 11px inline padding — closed on
  `--color-card` over `--color-border`, blocked on `--color-blocked-bg`. The
  desktop boards draw **none** of that: `21J-0`'s chips are bare text plus a
  chevron, with no ground, no outline and no fixed height (§4.7, read from
  `21J-0`'s subtree). **Decision: split R-F.2 into a box and a state.**
  • **The box — 30px height, `--radius-pill`, 11px inline padding, the ground
  and the border — is mobile-only.** R-F.2's evidence node is `377-0` → `7N2-0`
  section I3, the mobile-forms board, and notes item 9 independently calls the
  grounds and outlines a mobile *addition*: "the footer chips gain a tinted fill
  … that desktop does not have". Two sources agree, so the desktop bare chip is
  not a defect and §4.7 stands as measured.
  • **The chevron's direction — down when closed, up when open — binds at every
  width**, because it is a statement about state, not about arrangement, and
  both boards draw chevrons. §8 items 1 and 2 rely on this and remain correct.
  • **The label's colour and weight are where desktop and R-F.2 genuinely
  disagree**, and this file records the desktop form with its own intent
  (§4.7: chips 2–3 accent weight 500, chip 4 muted weight 400 because a stated
  absence is not a count you can open onto rows). R-F.2's closed row says muted
  weight 500 for all. Under R00.0 the components board wins on any width R-F.2
  covers — which, per the first bullet, is the phone tier, where §4.8 already
  measures all three mobile labels at muted weight 500. **So there is no live
  conflict**, and the desktop label colours are this screen's to state.
  The consequence for the defect list is that **Paper defect 5 is a mobile
  defect, not a desktop one**; it is restated there. A lane must not read
  R-F.2's box row as a desktop specification. If the components board intends it
  to bind at every width, that is a change to R-F.2 and it invalidates §4.7
  rather than the reverse.
- **D15 — `tokens.md` §8's engine breakpoint line numbers are stale at
  `3ac8844`; nothing in this file depends on them.** Every query address in §5
  and §7 was re-derived here with `awk` against the worktree:
  `max-width: 520px` at `style.css:1263`, `560px` at `4735`, `640px` at `2651`,
  `(pointer: coarse)` at `1852` and `2299`, `min-width: 861px` at `3245`,
  `min-width: 1181px` at `3466`, `max-width: 860px` at `1248` / `2315` / `3020`
  / `4512` / `4621`, and `(min-width: 861px) and (max-width: 1180px)` at `4582`.
  `tokens.md` §8 lists a lower number for each. **Decision: do not edit
  `tokens.md` from this lane** — it is another owner's file, and a screen spec
  that silently rewrites the shared token document makes the next disagreement
  invisible. The correction is recorded here and in M6, which is the one place
  this screen contradicts `tokens.md` on substance rather than on a line number:
  the `max-width: 640px` block (`style.css:2651-2671`) contains **zero**
  occurrences of `claim` and owns `.gcp-*`; only the `560` block (`4735-4746`)
  touches the claim footer. The `tokens.md` owner should re-derive §8 at
  `3ac8844`.
### Paper defects

1. **`22P-0` eyebrow tracking is `0.09em`** where the token is `--tracking-label`
   `0.08em`; the mobile twin uses `0.08em`. Two spellings of one eyebrow inside
   one group. Ship `0.08em`.
2. **Notes item 2 says "three absences"** where the board's own lede, eyebrow
   and title say two. See D1.
3. **Notes item 6 says the mobile footer becomes "three rows"** where R-F.4 says
   two. See D2.
4. **All four expansions drawn open**, contradicting R09.2 and R09.3 on the
   board's own face. The notes excuse it; the board does not label itself. See
   D4.
5. **No chip is drawn in the R-F.2 *open* variant — and per D14 this is a defect
   of the mobile boards, not of the desktop pair.** On `810-0` / `8V1-0` all four
   expansions are drawn open, yet every chip carries the **closed** variant:
   `--color-card` ground, `1px --color-border` outline, weight-500
   `--color-muted` label (§4.8), where R-F.2's open row fixes an
   `--color-accent` border and a weight-600 `--color-accent` label. Closed
   colouring under open chevrons. R-F.2's own intent sentence says why it
   matters: at 12px on a phone, colour alone is too quiet to say which door is
   open. The desktop boards are exempt because D14 scopes R-F.2's box to the
   phone tier; what they do owe is the **chevron direction**, which binds at
   every width — and all four desktop chevrons point up beside four expansions
   the engine will draw closed on load (§8 item 1). The mobile blocked chip's
   ground is a second miss in the same table: the board paints `#9E3B3617`
   rather than `--color-blocked-bg` `rgb(158 59 54 / 10%)`, which is defect 8's
   class one level down.
6. **The mobile comment count keeps the desktop faint form**, contradicting
   R-F.3's accent control variant. See D9.
7. **`1Z5-0` paints its ground `#EFF1F4` and `1Z6-0` its card `#FFFFFF` as raw
   hexes**, where the three sibling boards (`810-0`, `8I1-0`, `8V1-0`) spell
   `var(--color-paper)` / `var(--color-card)`. Same values, inconsistent
   authoring, inside one group.
8. **Light-only raw hexes with no token, on both light boards:** `#FBF7F7`,
   `#F6F8FA`, `#EBD9D8`, `#E3E8EE`, `#E8ECF1`, `#9E3B361A`, `#9E3B3617`,
   `#2C6B521A`. See D10.
9. **Raw hexes on the dark boards.** `#212934` (every section hairline on
   `8I1-0` and `8V1-0`), `#1B212B` (relationships ground — this is exactly
   `--color-dark-code-bg`, spelled raw), `#EC8A8329` (the blocked tint). These
   are *dark* values on dark boards, so not the "light hex on a dark board"
   class of defect — but `#1B212B` has a token and ignores it, and `#212934`
   sits between `--color-dark-border` `#242C38` and nothing, with no token at
   all. **Both are resolved in D10**, which now covers `#212934` explicitly:
   the section hairline in either mode is
   `color-mix(in srgb, var(--border) 70%, var(--card-bg))`, and `#1B212B` is
   simply spelled `var(--color-dark-code-bg)`. Neither takes a new allowlist key.
10. **`3BG-0` is an empty `Text` node** carrying serif `14/22` in `#54606F`,
    used as a flex spacer in the readiness footer row. A spacer should not be a
    styled text node; it will emit an empty element with inherited typography.
    Notes item 12 confirms the spacer is deliberate; its *node type* is the
    defect.
11. **The desktop light board spells 45 of its 54 text colours as raw literals**
    where the dark twin spells the token. This is defect 7 at full extent, and it
    is the single largest authoring defect in group 07. Inventory, read with
    `find_nodes` scoped to `1Z5-0`, one query per literal — `find_nodes` reports
    a token-bound usage as `var(--token)` and a literal as the hex, so the two
    are distinguishable and the counts below are exact:

    | Light literal | Token it duplicates | Raw nodes on `1Z5-0` | Nodes that spell the token |
    |---|---|---|---|
    | `#6E7C8E` | `--color-faint` | 22 — `22Z-0`, `280-0`, `282-0`, `286-0`, `28E-0`, `28F-0`, `28J-0`, `28O-0`, `28T-0`, `28Y-0`, `28Z-0`, `293-0`, `298-0`, `29A-0`, `23L-0`, `23O-0`, `23R-0`, `36U-0`, `372-0`, `29D-0`, `29G-0`, `29I-0` | 4 — `22P-0`, `225-0`, `21L-0`, `22L-0` |
    | `#54606F` | `--color-muted` | 7 — `2JY-0`, `3BG-0`, `28A-0`, `23P-0`, `23S-0`, `23U-0`, `288-0` | 1 — `22O-0` |
    | `#2C6B52` | `--color-locked` | 5 — `22G-0`, `28K-0`, `28P-0`, `28U-0`, `294-0` | 0 |
    | `#1C4E8C` | `--color-accent` | 5 — `28I-0`, `28N-0`, `28S-0`, `292-0`, `3BM-0` | 2 — `220-0`, `21W-0` |
    | `#9E3B36` | `--color-blocked` | 3 — `22X-0`, `36X-0`, `375-0` | 1 — `226-0` |
    | `#101720` | `--color-ink` | 3 — `36T-0`, `371-0`, `29F-0` | 2 — `22D-0`, `22M-0` |

    The four spelled tokens for `--color-faint` are the give-away: the author
    *did* reach for the token in the head and the footer, then stopped at the
    panel boundary. Not one `LOCKED` badge on the board spells `--color-locked`.
    **Light-mode SVG strokes carry the same defect one level down** — `#2C6B52`
    (padlock `22H-0`), `#9E3B36` (chip-1 chevron `223-0`), `#1C4E8C` (chips 2–4
    chevrons, graph icon `3BI-0`), `#6E7C8E` (direction icons: confirmed by
    `get_jsx` on `283-0`, which spells `stroke="#6E7C8E"`), `#54606F` (readiness
    row chevrons) — where every dark twin spells `var(--color-dark-*)`.
    **Every value is correct; only the spelling is wrong**, so this is a
    find-and-replace for the lane, not a redraw. It matters because a literal
    cannot re-point under `html[data-theme="dark"]`: a lane that transcribes
    `#6E7C8E` into CSS ships a light colour that survives into dark mode, which
    *is* the group-09 defect class. Chip 4's label (`2JY-0`, listed separately as
    defect 12 in the first draft) is one row of this table, not its own defect.
12. **The flagging inside §4 was itself inconsistent in the first draft of this
    file.** `2JY-0` was flagged `(raw, = --color-muted)` while its siblings
    `23S-0` and `23U-0`, carrying the identical literal, were written as bare
    token names; `36T-0`, `23O-0` and `28K-0` were likewise unflagged. Every
    colour cell in §4.7–§4.12 now carries the raw hex wherever the board spells
    one, and the §4 preamble states the reading rule. This is a defect of the
    spec rather than of the board, recorded rather than silently repaired,
    because a reader holding an earlier copy needs to know which of the two is
    the measurement. The values never changed; only the annotation did.
13. **Desktop relationship badge is a fixed `58px` slot.** `LOCKED` and `DRAFT`
    fit; `REVIEW PENDING` does not. R-I.2 explicitly rejects a fixed-width slot
    on mobile for exactly this reason; the desktop board keeps one. Flagged, not
    fixed here.
14. **No blocked banner on a blocked claim** — R09.6 leaves the Issues screen
    unreachable from this board. See D8.

---

*Paper is read-only for this lane. Nothing in the file was modified.*
