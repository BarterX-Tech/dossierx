# 14a · Comments — states and placement

Screen group **14a**. Companion to group 14 (`3XK-0`, "Comments — rail open on a
claim"). Where 14 draws the rail in its working state — two open threads, a
reply, a composer — 14a draws **the states 14 cannot show at once**: zero
comments, a resolved thread, and a viewer opened as a static file with nothing
to write back to. It also fixes the footer chip's three counts and the five
standing rules about where the comment surface may live.

Source of record: Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`. Every
number below came from `get_jsx` (inline-styles) or `get_computed_styles` on the
named node. Nothing is read off a screenshot. Engine side read from the worktree
`feat/viewer-design-revamp` at `3ac8844`.

Read with `../tokens.md` (the token names and their engine mapping) and
`../reference-rules.md` (the rules this screen is an instance of).

---

## 1. Boards

| Node | Name | Size | What it shows |
|---|---|---|---|
| `45K-0` | 14a · Comments — states and placement | 980 × 1025 | **Desktop light.** The canonical board. Header, then three 290px state columns (empty / resolved / read-only), then "The chip in the footer strip" with the chip's three counts, then the five-rule accent panel. |
| `9XP-0` | …· DARK | 980 × 1025 | **Desktop dark.** Node-for-node mirror of `45K-0` with every colour swapped to a `--color-dark-*` twin. No light hex survives on it. |
| `ABT-0` | …· MOBILE LIGHT | 390 × 2039 | **Mobile light.** The same three states re-drawn as bottom sheets. State 1 is a full 390 × 844 device vignette (`AC9-0`) with the scrim visible and the sheet at its 240px floor (`ACA-0`); states 2 and 3 are the same sheet cropped below their content (`AEG-0`, `AF5-0`) with a dashed bottom edge. The chip section becomes three stacked rows on a 44px lane (`AFR-0`). |
| `AJ8-0` | …· MOBILE DARK | 390 × 2041 | **Mobile dark.** Mirror of `ABT-0`, plus the one structural addition dark requires: a 1px `--color-dark-border` top hairline on every sheet (`AK3-0`, `AL0-0`, `ALO-0`). 2px taller than the light board because of it. |
| `9VR-0` | Group 14a — band | 2940 × 27.5 | The group band: eyebrow `9VT-0` "14A · COMMENTS — STATES AND PLACEMENT", subtitle `9VU-0` "four designs of one screen · light pair left, dark pair right", then a 2px `--color-accent` rule (`9VV-0`). |
| `APK-0` | Group 14a — notes | 2828 × 161 | The notes strip. Three columns (`APL-0`, `APP-0`, `AQ1-0`): "COMMENTS LIVE IN A BOTTOM SHEET" (`APM-0` / `APN-0`), "EVERY DIFFERENCE FROM DESKTOP" (`APQ-0` / `APR-0`), "A SHEET SIZES TO ITS CONTENT" (`AQ2-0` / `AQ3-0`). |

### Viewer states depicted

| Board region | Engine state it depicts |
|---|---|
| `45T-0` / `9Y0-0` / `ACA-0` / `AK3-0` — state 1 | **Zero threads, serve mode.** Panel is openable, the empty line is present, the composer is mounted. Engine class `comment-chip--empty` with a reachable `/api/ping`. |
| `463-0` / `9YA-0` / `AEO-0` / `AL7-0` — state 2 | **A resolved thread.** Engine class `comment-thread--resolved`; the action row offers Reopen, not Resolve. |
| `46F-0` / `9YM-0` / `AF5-0` / `ALO-0` — state 3 | **`file://` / static export.** Threads baked into the HTML, no composer, no per-message actions. Engine: `renderPanelReadOnly`. |
| `46U-0` / `9Z1-0` / `AFT-0` — chip, 2 | **Open threads.** Engine class `comment-chip--open`, count = open thread count. |
| `470-0` / `9Z7-0` / `AG1-0` — chip, 4 | **All resolved.** Engine class `comment-chip--resolved`, count = total threads. |
| `476-0` / `9ZD-0` / `AGJ-0` — chip, 0 | **Never commented.** Engine class `comment-chip--empty`, count 0, dimmed but clickable. |

Not depicted anywhere on 14a: claim status (locked / draft / blocked), readiness,
the graph. 14a is deliberately a single-concern board — see §8 for what that
leaves a lane guessing about, and what the rules say to do about it.

---

## 2. Design intent

**What the reviewer should perceive.** The intro on `45O-0` / `9XV-0` states the
whole thesis: *"The rail is where a reviewer and the agent argue about a claim
without editing it. Two authors, never more: a human and the agent that operates
the tool. The rail is non-modal on desktop, so the claim stays readable beside
it."* Comments are an **argument about a claim, not an edit to it**. That is why
the surface never covers the claim on desktop, why the thread carries an
authorship label rather than an avatar, and why resolving hides nothing.

The three states are chosen because each is a place where a naive implementation
breaks a promise:

1. **Zero comments** would naturally render no surface at all — and then nobody
   could ever write the first comment. Note `460-0`: *"The chip opens even at
   zero, because otherwise nobody could write the first comment on a claim."*
   The empty state is therefore a **state of the open panel**, not a reason to
   suppress it.
2. **A resolved thread** would naturally be deleted or collapsed away — and then
   the reviewer loses the record of what was asked. Note `46C-0`: *"Resolved
   threads live behind one disclosure and keep their text. Nothing is deleted by
   resolving."* Resolved is a **dimming plus a disclosure**, never a deletion.
3. **Opened from a file** would naturally render a composer that silently fails.
   Note `46N-0`: *"A shared HTML file still carries every comment written before
   it was built. The composer is absent rather than disabled."* A control with
   nowhere to POST is not disabled; it is **not built**.

The three columns are equal-width (290px each) and the state labels are
numbered — `45S-0`, `462-0`, `46E-0` — because these are peers, not a degradation
ladder. No state is drawn as "the broken one".

### The five rules on the board (`47C-0` / `9ZJ-0`), verbatim and read as rules

1. `47E-0` — *"One rail, one claim. The rail always names the claim it belongs
   to, because it overlays a page that scrolls behind it."* The reason is the
   rule: an overlay over a scrolling page loses its anchor, so the header must
   restate the anchor. Implemented today (`viewer-runtime.js:542` sets the rail
   title to `'Comments — ' + claimID`).
2. `47F-0` — *"Two authors, and they are never styled alike — the human is the
   accent, the agent is the link blue. A reviewer must be able to tell who said a
   thing without reading the label twice."* See the Paper defect note below: this
   sentence is ambiguous between Paper's token vocabulary and the engine's, and
   the two readings produce opposite colours.
3. `47G-0` — *"The rail and the left navigation are mutually exclusive. Opening
   one closes the other; there is never a second overlay behind the first."*
   Implemented (`viewer-runtime.js:539`, `viewer-runtime.js:338`).
4. `47H-0` — *"A claim with an open thread carries a 3px accent rule down its
   left edge in the reading view, and a ring in the claims graph. The same fact,
   marked the same way, in both places."* The 3px rule exists
   (`style.css:1679`); the graph mark exists but is a **halo**, not a ring — see
   defects.
5. `47I-0` — *"Below 860px the rail becomes a bottom sheet with a backdrop, and
   that is the only place it is modal."* This is the one rule the mobile boards
   are *about*: it is promoted verbatim into the mobile header (`ABZ-0`,
   `AJX-0`), replacing the intro paragraph.

### The notes strip (`APK-0`), paraphrased with intent

- **`APM-0` / `APN-0` — "COMMENTS LIVE IN A BOTTOM SHEET."** Below 860 the rail
  becomes *the bottom sheet already in this file* — same 16px top corners, same
  36 × 4 grabber, same 660px ceiling as the mobile nav sheet. Intent: R-J.1 —
  one shell, many bodies; the comment thread is a body, not a new sheet. The
  note also fixes the scrim literally: **`#10172057` light and `#00000085`
  dark**, and states that the dark sheet carries *"the same 1px top hairline as
  the nav sheet"*. It then explains the board's own drawing convention: state 1
  is drawn at full 390 × 844 so the scrim and the rise are visible once; states
  2 and 3 are the same sheet **cropped**, marked by a dashed bottom edge,
  because three full-height vignettes do not fit the band. **The dashed edge is
  a board artefact and must not be implemented.**
- **`APQ-0` / `APR-0` — "EVERY DIFFERENCE FROM DESKTOP."** Dropped on mobile:
  the intro paragraph with its 760px measure, and four of the five rules with
  the accent panel that held them — annotation that does not change with width.
  Rule five is promoted verbatim into the header, taking the intro's slot, at
  16/26 muted rather than the panel's 15/25 ink. Added: the sheet chrome itself
  (grabber, 16px top corners, scrim, a header row with a close X — *"state 2's
  desktop card had no header row"*), a 1px hairline under the device vignette, a
  dashed edge under the two cropped sheets, and **a fixed 44px leading slot on
  the chip rows**. Restructured: the three state columns lose their 290px width
  and stack; the footer-strip chip row becomes three lines. Resized: **16px
  board gutters instead of 36px**, the card's **18px inner gutter becomes the
  sheet's 16px**, and the card's **14px "Comments" title becomes the sheet's
  16px** title. *"No weight changed, and no wording changed anywhere."*
- **`AQ2-0` / `AQ3-0` — "A SHEET SIZES TO ITS CONTENT."** Recorded as a *ruling
  from the coordinator*: **660px is a ceiling, not a fixed height.** The sheet
  grows between a ~240px floor and that ceiling and scrolls internally beyond
  it. *"Form parity is the chrome … not one height."* State 1 was redrawn to the
  floor at 240px: header, the empty line, the pinned composer, scrim filling the
  rest of the 844px frame.

### Rules from `reference-rules.md` that bind this screen, by id

- **R-J.1** — one bottom sheet in the product; the comment thread is a *body* in
  the existing shell. 14a's mobile sheets must be that shell, not a second one.
- **R-J.2** — the fixed shell: scrim, 16px top corners, 36 × 4 grabber in
  `--color-border-strong` under 8px of top padding, header row with a 1px
  `--color-border` hairline beneath, **and on dark an additional 1px top
  hairline on the sheet itself**. All five are present and measured in §4.
- **R-J.3** — `min-height: 240px`, `max-height: 660px`, `height: fit-content`,
  internal scroll beyond. `ACA-0` / `AK3-0` carry all three literally.
- **R-J.4** — the header's second line and mono count are **slots, not forks**.
  14a's sheets render the header *without* them (title + close only) — which is
  exactly what R-J.4 permits and is the evidence that it is a slot.
- **R-J.5** — where a body has a composer it is pinned and never shrinks. State
  1's composer (`ACV-0`, `AKC-0`) is `flexShrink: 0` against a `flexGrow: 1`
  body (`ACQ-0`, `AKA-0`). State 3 has **no** composer at all, which is R-J.5
  read correctly: the rule governs a body that *has* one.
- **R-J.6** — *"Empty, resolved and read-only are states of the one shell."*
  This is the rule 14a exists to instantiate, and it names 14a explicitly in its
  Binds line.
- **R-J.7** — thread structure: authorship label mono 11/14 w600 `+0.04em`,
  `HUMAN` in `--color-accent`, `AGENT` in `--color-graph-facet-1`; elapsed time
  mono 11/14 `--color-faint`; `(edited)` mono 11/14 italic `--color-faint`; body
  14/22; Resolve pill 1px `--color-border-strong` + `--radius-pill` + 12px tick
  in `--color-locked` + label 12/16 w600 `--color-muted`; Reply bare 12/16 w600
  `--color-accent`; resolved threads collapse to `N resolved` at 12/16 w600
  `--color-muted`. 14a shows the **Reopen** variant of that action pill, which
  R-J.7 does not enumerate — see Open decisions.
- **R-F.1 / R-F.3** — the comment count is the **last** item in the footer
  strip, hard right, at every width; one pill with one variant — passive
  `--color-faint` bubble on desktop, `--color-accent-bg` + `--color-accent` on
  mobile where it is the control that opens the sheet. 14a's chip specimen is
  that pill in its three counts.
- **R-F.4** — the mobile footer is two rows, status + count on row one. 14a does
  not draw the footer strip itself, only the chip; the strip's arrangement stays
  group 02/H3's problem.
- **R-I.3** — *"The chip is the component; the footer's two rows are
  composition."* This is the rule §6's own framing is an instance of: 14a is
  entitled to draw the last item of the metadata strip without drawing the strip,
  because the chip is the component and the strip is composition around it. It is
  also what makes §5's "footer chip row becomes three lines" a **composition**
  change rather than a change to the chip: at 390 the chip keeps the same glyph,
  the same count, the same three states and the same wording — only the lane it
  sits in is new. R-F.4 governs the strip; R-I.3 is why that strip's rearrangement
  does not reach the pill.
- **R-H.0** — *"Only three components change shape below the phone
  breakpoint … a screen must use the form that matches its width."* Reconciled
  against §5, which asserts a rail → sheet shape change: R-H.0's three are R-H.1
  (blocked notice), R-H.2 (claim header) and R-H.3 (coverage chip), all of them
  components **inside the reading page**, all of them changing at the phone
  breakpoint. The comment rail is **not a fourth**. It does not change shape at
  390; it changes at **860**, and that change is owned by bottom-sheet policy
  (R-J.1, one shell with many bodies), which is a separate policy with its own
  query. By the time the boards' 390 width is reached the surface is already the
  sheet, and at 390 it is the same sheet, narrower — exactly the "same names, same
  counts, same wording, different arrangement" R-H.0 describes. Nothing on 14a
  adds to R-H.0's count of three.
- **R10.1 / R10.3 / R10.4** (board `1F7-0`, "Elapsed, not stamped") — **applies
  to every comment timestamp on this screen.** `466-0` reads "6 days ago",
  `46J-0` reads "HUMAN · 3 days ago". One unit, never two; never a stamp; the
  exact time stays on `title`/`datetime` for the rare reader who needs it; and
  the phrase is computed in the browser, not baked into the HTML — *"A relative
  phrase baked at generation time says '23 hours ago' forever, which is worse
  than a date."* R10.2's three-band colouring is **not** inherited: a comment's
  age takes no colour on any 14a board (`466-0` is `--color-faint`, flat). R10.5
  ("Live" behind `dossierx serve`) governs the rail footer's build line, not a
  message's age.
- **R09.7** is the reason 14a carries no health signalling of its own, and
  **R09.2** is why the chip is a peer of the four expansion chips rather than a
  fifth expansion.

### Contradictions on the board, against its own notes (approval is not proof)

- **`47F-0` says "the agent is the link blue".** In Paper's vocabulary
  `--color-accent` *is* the navigation blue (`#1C4E8C`), so "the human is the
  accent, the agent is the link blue" makes both authors the same colour. In the
  engine's vocabulary the sentence is literally true of today's code
  (`.comment-role--human { color: var(--accent) }` = lock green,
  `.comment-role--agent { color: var(--link) }` = navigation blue) — and that
  reading paints the human **green**, which no board does. Both readings are
  wrong. The measured boards and R-J.7 agree on the actual values: HUMAN =
  `--color-accent`, AGENT = `--color-graph-facet-1`. **Paper defect.**
- **`47H-0` says "a ring in the claims graph".** `graph-ui.js:1940-1942`
  reserves *ring* for lock status (locked solid / draft dashed) and *halo* for
  `review_pending` or open comments. Implementing "ring" as written would
  overwrite the lock mark. **Paper defect** — the mark is the halo.
- **The mobile scrim is a raw hex with baked alpha** — `AC9-0` `#10172057`,
  `AK2-0` `#00000085` — where the engine has a themeable `--scrim`
  (`rgba(0,0,0,.22)` light / `rgba(0,0,0,.42)` dark). The board's own note
  `APN-0` repeats the literals as if they were the spec. `#10172057` is
  `--color-ink` at 34%, which is a *different* colour and a *different* alpha
  from the engine token in both modes. **Paper defect** — implement `--scrim`.
- **The dark boards use `--color-dark-border-strong` `#38424F`** for the grabber
  (`AK4-0` child) and the Reopen pill (`ALC-0`) — but `tokens.md` Disagreement 9
  records that the engine's `--border-strong` has **no dark override**. The
  board is right and the engine is short a value; flagged, not resolved here.

---

## 3. Layout

### Desktop, 980px board (`45K-0` / `9XP-0`)

The board is 980 wide, not 1440. It is a **specimen sheet**, not a viewport: the
column widths below are the measure of the specimen, not of the product. The
product geometry this screen governs is the rail on board 14 and the sheet on
`ABT-0`; both are given underneath.

| Value | Measured | Node |
|---|---|---|
| Board width | `980px` | `45K-0` (dark `9XP-0`) |
| Board padding | `36px` top, `36px` inline, `40px` bottom | `45K-0` |
| Board section gap | `26px` | `45K-0` |
| Board ground | `--color-paper` / dark `--color-dark-paper` | `45K-0` / `9XP-0` |
| Header stack gap | `10px` | `45L-0` / `9XS-0` |
| Intro measure | `760px` (the board's own explicit width) | `45O-0` / `9XV-0` |
| State row gap | `22px`, `align-items: start` | `45Q-0` / `9XX-0` |
| State column width | `290px`, `flex-shrink: 0`, internal gap `10px` | `45R-0`, `461-0`, `46D-0` |
| Chip-section gap | `14px`, `padding-top: 6px` | `46P-0` / `9YW-0` |
| Chip specimen row gap | `34px` | `46S-0` / `9YZ-0` |
| Rules panel padding | `20px` block / `24px` inline, gap `10px` | `47C-0` / `9ZJ-0` |

Sticky and scroll regions: **none on this board.** 14a is a flat document. The
sticky/scroll behaviour it specifies belongs to the surfaces it describes:

| Product geometry | Measured | Node |
|---|---|---|
| Desktop comment rail width | `360px`, `flex-shrink: 0` | `43Y-0` (board 14) |
| Rail left edge | `1px solid --color-border-strong` + `#10172014 -8px 0 24px` shadow | `43Y-0` |
| Rail header | `20px` top / `16px` bottom / `22px` inline, gap `12px`, 1px `--color-border` bottom rule | `43Z-0` |
| Rail scroll region | `flex-grow: 1`, `4px` top / `22px` inline padding | `446-0` |
| Rail composer (pinned) | `16px` top / `20px` bottom / `22px` inline, gap `10px`, `--color-paper` ground, 1px `--color-border` top rule | `45B-0` |
| Content column beside the rail | `812px` | `40K-0` (board 14) |
| Left sidebar | `268px` (= `--container-rail`) | `3XL-0` (board 14) |

1440 = 268 + 812 + 360. The rail is **not** `--container-rail`; that token is the
left sidebar. The comment rail has no token.

### Mobile, 390px (`ABT-0` / `AJ8-0`)

| Value | Measured | Node |
|---|---|---|
| Board width / height | `390 × 2039` light, `390 × 2041` dark | `ABT-0` / `AJ8-0` |
| Board padding | `36px` top, `40px` bottom, **no inline padding on the board itself** | `ABT-0` |
| Text gutter | `16px` inline, applied per text block, not to the board | `ABW-0`, `AC2-0`, `ACY-0`, `AFN-0` |
| Sheet / vignette width | `390px` — full bleed, the gutter stops at the sheet edge | `AC9-0`, `ACA-0` |
| Device vignette height | `844px`, `justify-content: end` | `AC9-0` / `AK2-0` |
| Sheet height band | `min-height: 240px`, `max-height: 660px`, `height: fit-content` | `ACA-0` / `AK3-0` |
| Sheet at rest, state 1 | `240px` — the floor, because it holds one line | `ACA-0` (rendered height) |
| Scroll region | the body, `flex: 1 0 0`, `min-height: 0`, `overflow: clip` | `ACQ-0` / `AKA-0` |
| Pinned region | the composer, `flex-shrink: 0` | `ACV-0` / `AKC-0` |
| Sheet inner gutter | `16px` inline everywhere (vs the desktop card's `18px`) | `ACD-0`, `ACQ-0`, `ACV-0` |
| Chip row lane | fixed `44px` width, `flex-shrink: 0` on all three rows | `AFT-0`, `AG1-0`, `AGJ-0` |
| Chip card | `16px` padding, `16px` row gap | `AFR-0` / `AM6-0` |

---

## 4. Components on this screen

**Count of measured values recorded in this section: 168.** Of those, **104 come
from the desktop light board `45K-0`** and its dark twin `9XP-0`; the rest from
`ABT-0` / `AJ8-0`. Every value is a `get_jsx` / `get_computed_styles` read.
Values with no node id are not values.

Colour is given as `token — hex`. Light token first, dark token second. The hexes
are the Paper token values from `get_tokens` (contentHash `cdb7b571`).

### A. Board header (`45L-0` / `9XS-0` / `ABW-0` / `AJU-0`)

| Part | Value | Light token | Dark token | Nodes |
|---|---|---|---|---|
| Eyebrow "COMPANION TO 14" | Inter 12/16, w600, `+0.08em` | `--color-faint` — `#6E7C8E` | `--color-dark-faint` — `#8494A8` | `45M-0` / `9XT-0` / `ABX-0` / `AJV-0` |
| Title | Inter 26/32, w600, `-0.02em` | `--color-ink` — `#101720` | `--color-dark-ink` — `#E6EAF0` | `45N-0` / `9XU-0` / `ABY-0` / `AJW-0` |
| Intro / promoted rule | Source Serif 4 16/26, w400, width 760 (desktop only) | `--color-muted` — `#54606F` | `--color-dark-muted` — `#9AA7B8` | `45O-0` / `9XV-0` / `ABZ-0` / `AJX-0` |
| State label | Inter 12/16, w600, `+0.08em` | `--color-faint` | `--color-dark-faint` | `45S-0`, `462-0`, `46E-0` / `AC3-0`, `AEE-0`, `AF3-0` |
| Column note prose | Source Serif 4 13/21, w400 | `--color-muted` | `--color-dark-muted` | `460-0`, `46C-0`, `46N-0` / `ACZ-0`, `AEZ-0`, `AFL-0` |

26/32 is off the type scale; `tokens.md` §3 records it as the reference-board
heading size. 13/21 is also off-scale and is this board's note size.

### B. Panel card — desktop (`45T-0` / `9Y0-0`, `46F-0` / `9YM-0`)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Ground | — | `--color-card` — `#FFFFFF` | `--color-dark-card` — `#161B22` | `45T-0` / `9Y0-0` |
| Border | `1px solid` | `--color-border` — `#DDE2E9` | `--color-dark-border` — `#242C38` | `45T-0` / `9Y0-0` |
| Radius | `8px` (= `--radius-md`) | — | — | `45T-0` |
| Clipping | `overflow: clip` | — | — | `45T-0` |
| Header padding | `16px` top / `14px` bottom / `18px` inline | — | — | `45U-0` / `9Y1-0` |
| Header rule | `1px solid` bottom | `--color-border` | `--color-dark-border` | `45U-0` |
| Header title "Comments" | Inter 14/18, w600 | `--color-ink` | `--color-dark-ink` | `45V-0` / `46H-0` |

### C. Empty state (state 1)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Body padding, desktop | `22px` block / `18px` inline | — | — | `45W-0` / `9Y3-0` |
| Body padding, mobile | `22px` block / `16px` inline | — | — | `ACQ-0` / `AKA-0` |
| Empty line | Source Serif 4 13/21 | `--color-muted` | `--color-dark-muted` | `45X-0` / `ACR-0` |
| Empty copy | "No comments yet — add the first one below." | — | — | `45X-0` |
| Composer strip ground | — | `--color-paper` — `#EFF1F4` | `--color-dark-paper` — `#0D1117` | `45Y-0` / `ACV-0` |
| Composer strip top rule | `1px solid` | `--color-border` | `--color-dark-border` | `45Y-0` / `ACV-0` |
| Composer padding, desktop | `14px` block / `18px` inline | — | — | `45Y-0` |
| Composer padding, mobile | `14px` block / `16px` inline | — | — | `ACV-0` |
| Placeholder | Inter 13/16, w400 | `--color-faint` | `--color-dark-faint` | `45Z-0` / `ACW-0` |
| Placeholder copy | "Add a comment…" | — | — | `45Z-0` |

Note the composer strip is the **only** place on this screen where the page
ground (`--color-paper`) appears inside a card. On dark that is `#0D1117`, which
is *darker* than the `#161B22` card — the composer recedes on dark and advances
on light. Both boards do it deliberately and identically; it is the same
inversion R-J.5 measures on the sheet composer.

### D. Resolved thread (state 2)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Card, desktop | ground `--color-card`, `1px` `--color-border`, radius `8px` | — | `--color-dark-card` / `--color-dark-border` | `463-0` / `9YA-0` |
| Card padding, desktop | `16px` block / `18px` inline, gap `8px` | — | — | `463-0` |
| **Resolved dimming** | `opacity: 0.72` | — | — | `463-0` (desktop, whole card) / `AEO-0` (mobile, comment block only) |
| Authorship label | IBM Plex Mono 11/14, w600, `+0.04em`, uppercase as authored | `--color-accent` — `#1C4E8C` | `--color-dark-accent` — `#6AA6E8` | `465-0` / `9YC-0` / `AEQ-0` |
| Meta row gap | `9px` | — | — | `464-0` / `AEP-0` |
| Elapsed time | IBM Plex Mono 11/14, w400 | `--color-faint` | `--color-dark-faint` | `466-0` / `AER-0` |
| Elapsed copy | "6 days ago" — one unit, per R10.3 | — | — | `466-0` |
| Comment body | **Inter** 13/21, w400 | `--color-ink` | `--color-dark-ink` | `467-0` / `AES-0` |
| Reopen pill | `1px solid`, radius `999px` (`--radius-pill`), gap `6px`, `margin-top: 4px`, padding `4px` block / `10px` inline, `align-self: start` | `--color-border-strong` — `#C2CAD5` | `--color-dark-border-strong` — `#38424F` | `468-0` / `9YF-0` / `AET-0` |
| Reopen icon | 12 × 12, `stroke-width: 2.2`, round caps, path `M3 12a9 9 0 1 0 3-6.7M3 4v5h5` | `--color-muted` | `--color-dark-muted` | `469-0` / `AEU-0` |
| Reopen label | Inter 12/16, w500 | `--color-muted` | `--color-dark-muted` | `46B-0` / `AEW-0` |

The comment body is **sans, not serif** — 13/21 Inter. That is deliberate and
consistent across all four boards: a comment is interface speech about a claim,
not the claim's own prose. The board's serif is reserved for the annotation
columns (`460-0` etc.), which are the designer speaking, not the product.

### E. Read-only state (state 3)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Body gap | `7px` | — | — | `46I-0` / `9YP-0` / `AFD-0` |
| Body padding, desktop | `16px` block / `18px` inline | — | — | `46I-0` |
| Body padding, mobile | `16px` all sides | — | — | `AFD-0` |
| Combined meta line | IBM Plex Mono 11/14, w600, `+0.04em` | `--color-accent` | `--color-dark-accent` | `46J-0` / `AFE-0` |
| Combined meta copy | "HUMAN · 3 days ago" — label and elapsed on **one** line, both in the accent | — | — | `46J-0` |
| Comment body | Inter 13/21 | `--color-ink` | `--color-dark-ink` | `46K-0` / `AFF-0` |
| Read-only strip ground | — | `--color-paper` | `--color-dark-paper` | `46L-0` / `AFH-0` |
| Read-only strip top rule | `1px solid` | `--color-border` | `--color-dark-border` | `46L-0` |
| Read-only strip padding | `13px` block / `18px` inline desktop, `13px` / `16px` mobile | — | — | `46L-0` / `AFH-0` |
| Read-only text | Inter 12/19, w400 | `--color-muted` | `--color-dark-muted` | `46M-0` / `AFI-0` |
| Read-only copy | "Read only. This viewer was opened as a file, so there is nothing to write back to." | — | — | `46M-0` |

The read-only strip occupies **exactly the composer's slot** — same ground, same
top rule, same position at the bottom of the card. The composer is not disabled;
its slot is re-tenanted by an explanation. That is the whole point of note
`46N-0`.

**Inconsistency to carry forward, not to copy:** state 2 uses a two-node meta row
(`465-0` + `466-0`, elapsed in `--color-faint`); state 3 uses a single text node
(`46J-0`) with the elapsed time **also** in `--color-accent`. R-J.7 fixes elapsed
time at `--color-faint`. Implement the R-J.7 form (two spans, faint time) in both
states. See Open decisions.

### F. Comment-count chip specimen (`46S-0` / `9YZ-0` / `AFR-0` / `AM6-0`)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Specimen card | ground `--color-card`, `1px` `--color-border`, radius `8px`, padding `16px` block / `20px` inline, gap `34px` | — | `--color-dark-*` twins | `46S-0` / `9YZ-0` |
| Bubble icon, all states | 14 × 14, `stroke-width: 1.8`, path `M21 12a8 8 0 0 1-8 8H7l-4 3V12a8 8 0 0 1 8-8h2a8 8 0 0 1 8 8z` | — | — | `46V-0`, `471-0`, `477-0` |
| **Open** pill ground | — | `--color-accent-bg` — `rgb(28 78 140 / 9%)` | `--color-dark-accent-bg` — `rgb(106 166 232 / 14%)` | `46U-0` / `9Z1-0` / `AFT-0` |
| **Open** pill shape | radius `999px`, gap `6px`, padding `3px` block / `8px` inline | — | — | `46U-0` |
| **Open** icon stroke | — | `--color-accent` | `--color-dark-accent` | `46V-0` |
| **Open** count | Inter 12/16, **w600** | `--color-accent` | `--color-dark-accent` | `46X-0` / `AFW-0` |
| **Resolved** pill | **no ground, no border, no radius** — bare icon + count, gap `6px` | — | — | `470-0` / `9Z7-0` / `AG1-0` |
| **Resolved** icon stroke | — | `--color-faint` | `--color-dark-faint` | `471-0` |
| **Resolved** count | Inter 12/16, **w400** | `--color-faint` | `--color-dark-faint` | `473-0` / `AG4-0` |
| **Empty** pill | identical to Resolved plus `opacity: 0.5` | — | — | `476-0` / `9ZD-0` / `AGJ-0` |
| **Empty** count | Inter 12/16, w400, `0` | `--color-faint` | `--color-dark-faint` | `479-0` / `AGM-0` |
| Row label | Inter 13/16, w400 | `--color-muted` | `--color-dark-muted` | `46Y-0`, `474-0`, `47A-0` |
| Row gap (icon-group → label) | `10px` | — | — | `46T-0`, `46Z-0`, `475-0` |
| Desktop group widths | open `44 × 22`, resolved `28 × 16`, empty `28 × 16` | — | — | `46U-0`, `470-0`, `476-0` |
| Mobile lane | **all three forced to `width: 44px`, `flex-shrink: 0`, padding `3px`/`8px`** | — | — | `AFT-0`, `AG1-0`, `AGJ-0` |

Three states, one component. Only **open** takes a tint; that is how "there is
work here" is said, and it is the same grammar as R08.1's "only Critical is
filled". Empty is the same pill at half opacity — *dim, still clickable*
(`47A-0`), never `display: none`, because the chip at zero is the only route to
the first comment (`460-0`).

### G. Bottom sheet — mobile only (`ACA-0` / `AK3-0`, `AEG-0` / `AL0-0`, `AF5-0` / `ALO-0`)

| Property | Value | Light | Dark | Nodes |
|---|---|---|---|---|
| Scrim | full 390 × 844, `justify-content: end` | **`#10172057` (raw)** | **`#00000085` (raw)** | `AC9-0` / `AK2-0` |
| Vignette bottom hairline | `1px solid` | `--color-border-strong` | `--color-dark-border-strong` | `AC9-0` / `AK2-0` |
| Sheet ground | — | `--color-card` | `--color-dark-card` | `ACA-0` / `AK3-0` |
| Sheet top corners | `16px` top-left and top-right | — | — | `ACA-0`, `AEG-0`, `AF5-0` |
| Sheet top hairline | **dark only**, `1px solid` | — (absent) | `--color-dark-border` | `AK3-0`, `AL0-0`, `ALO-0` |
| Sheet height band | `min 240px` / `max 660px` / `fit-content`, `overflow: clip` | — | — | `ACA-0` / `AK3-0` |
| Grabber slot | `padding-top: 8px`, centred | — | — | `ACB-0` / `AK4-0` |
| Grabber | `36 × 4`, radius `999px` | `--color-border-strong` — `#C2CAD5` | `--color-dark-border-strong` — `#38424F` | `ACC-0` / `AL2-0` |
| Header row | padding `14px` top / `12px` bottom / `16px` inline, gap `10px` | — | — | `ACD-0` / `AK6-0` |
| Header bottom hairline | `1px solid` | `--color-border` | `--color-dark-border` | `ACD-0` |
| Header title | Inter 16/22, w600, `-0.01em`, `flex: 1 0 0` | `--color-ink` | `--color-dark-ink` | `ACE-0` / `AL4-0` |
| Close glyph | 20 × 20, `stroke-width: 2`, round caps, path `M6 6l12 12M18 6L6 18` | `--color-muted` | `--color-dark-muted` | `ACF-0` / `AL5-0` |
| Cropped-fragment marker | `1px dashed` bottom | `--color-border-strong` | `--color-dark-border-strong` | `AEG-0`, `AF5-0` / `AL0-0`, `ALO-0` |

Every value in this table except the last row and the two raw scrim hexes is
R-J.2 / R-J.3 restated at measurement. The last row is a **board artefact**: the
dashed edge marks where the drawing was cut, per note `APN-0`. It has no engine
meaning.

### States visible on these boards

| State | Where | Expression |
|---|---|---|
| Thread open | `46U-0` chip; implied by every unresolved thread | accent-tinted pill, w600 count |
| Thread resolved | `463-0` / `AEO-0` | `opacity: 0.72` + the Reopen action replacing Resolve |
| All threads resolved (claim level) | `470-0` | untinted bubble, w400 count, `--color-faint` |
| Never commented | `476-0` | untinted bubble at `opacity: 0.5` |
| Panel empty | `45W-0` / `ACQ-0` | one 13/21 muted line + a live composer |
| Panel read-only | `46L-0` / `AFH-0` | composer slot replaced by a 12/19 explanation |
| Sheet at floor | `ACA-0` | 240px, chrome complete, no scroll |
| Dark sheet | `AK3-0` | the extra 1px top hairline, which is the only structural dark/light difference on the whole screen |

**Not on any 14a board:** hover, focus, active, pressed, disabled, or a selected
thread. Nothing on `45K-0`, `9XP-0`, `ABT-0` or `AJ8-0` depicts a pointer state.
See §8.

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. No 390px breakpoint is
added (`tokens.md` §8).

| Rule at 390 | Measured evidence | Engine breakpoint | Why that one |
|---|---|---|---|
| The rail becomes a modal bottom sheet with a scrim | `AC9-0`, `ACA-0`; board rule `47I-0` says "Below 860px" in words | **`max-width: 860px`** | The board names 860 itself, and `style.css:2227` already owns exactly this switch. This is the one mobile rule on this screen that is *not* a 520 rule. |
| Scrim + body scroll-lock exist only below 860 | note `APN-0`; rule `47I-0` "that is the only place it is modal" | `max-width: 860px` | Already implemented (`style.css:2228-2233`); the desktop rail stays `role="complementary"` and non-modal. |
| Three 290px state columns stack full-bleed | `45R-0` → `AC1-0`; note `APR-0` "lose their 290px width and stack" | `max-width: 520px` | Arrangement is a width question. This is a *board* layout, not product chrome; a lane implements the analogue: the rail's content column stops being a fixed 360 and becomes 100%. |
| Board gutter 36px → 16px; card inner gutter 18px → 16px | `45K-0` `36px` vs per-block `16px` on `ABW-0`; `45U-0` `18px` vs `ACD-0` `16px` | `max-width: 520px` | Pure arrangement. |
| Sheet is full-bleed 390 while text keeps a 16px gutter | `AC9-0` 390 vs `ACY-0` 16px inline | `max-width: 520px` | The sheet spans the viewport; only running text is inset. |
| Card "Comments" title 14/18 → sheet title 16/22 `-0.01em` | `45V-0` vs `ACE-0` | `max-width: 520px` | Type size change tied to arrangement, per note `APR-0` ("No weight changed"). |
| A header row with a close X is **added** to the state-2 fragment | `463-0` has none; `AEG-0` has `AEJ-0` | `max-width: 520px` | A sheet always has a header; a desktop card inside a rail does not need one. |
| Footer chip row becomes three lines on a fixed 44px lane | `46S-0` gap 34 one line → `AFR-0` gap 16 three rows; `AFT-0`/`AG1-0`/`AGJ-0` all `width: 44px` | **`max-width: 520px`** for the stacking, **`(pointer: coarse)`** for the 44px | Split per `tokens.md` §8: arrangement by width, target size by pointer. The engine already has `.comment-chip { min-height: 44px }` at `style.css:1764-1767`; the 44px **width** lane is the new half. |
| The zero-thread chip must stay visible without hover | `476-0` "none yet — dim, still clickable" | **`(pointer: coarse)`** | There is no hover on a phone; `style.css:1769-1771` already pins `.comment-chip--empty { opacity: .75 }` there. The board draws `0.5`; see Open decisions. |
| Close glyph, Reopen pill and composer field need ≥44px targets | close `ACF-0` is 20 × 20 drawn; Reopen `AET-0` is 84 × 26 | **`(pointer: coarse)`** | Drawn size ≠ target size. `style.css:2211-2220` already sets `min-height: 44px` on `.comments-rail-close`, `.comment-action`, `.comment-action-text`, `.comment-composer-submit`, `.comment-composer-input`. The Reopen pill (`.comment-reopen`) is a `.comment-action` and is covered. |
| Sheet grows between 240 and 660 and scrolls internally | `ACA-0` `min-height: 240px` / `max-height: 660px` / `fit-content` | `max-width: 860px` | Same query as the sheet itself. **This replaces the engine's current `max-height: 70vh`** (`style.css:2240`), which is neither a floor nor a fixed ceiling. |
| 16px top corners | `ACA-0` | `max-width: 860px` | The engine currently sets `border-radius: 14px 14px 0 0` (`style.css:2243`). 16 is the sheet-shell value (R-J.2). |
| Grabber, 36 × 4 under 8px top padding | `ACB-0` / `ACC-0` | `max-width: 860px` for the element, `(pointer: coarse)` if it becomes a drag handle | The engine has **no grabber at all** today. |
| Dark sheets get a 1px top hairline | `AK3-0`, `AL0-0`, `ALO-0` | `max-width: 860px`, inside the dark query | The engine already sets `border-top: 1px solid var(--border)` unconditionally at `style.css:2242`; the design wants it dark-only. |

**Hidden on mobile:** the 760px intro paragraph and four of the five rules with
the accent panel that held them (note `APR-0`). Those are board annotation, so
the product analogue is: nothing in the comment surface is hidden at 390 — every
thread, every author label, every elapsed time and the composer all survive. The
only thing group 14's notes record as genuinely dropped on mobile is the pair of
13px hover glyphs (edit, delete) on the reader's own comment, "because there is
no hover on a phone and a 13px target is not a control" (`AOX-0`, group 14).

---

## 6. Footer vocabulary

14a does not draw the full four-chip metadata strip; it draws **the last item in
it**, and fixes that item's three counts. Quoted from the board, in board order
(`46S-0`, left to right; `AFR-0`, top to bottom):

| Position | Glyph + count | Label, quoted exactly | Node |
|---|---|---|---|
| 1 | tinted bubble, `2` | **"open threads"** | `46X-0` + `46Y-0` |
| 2 | bare bubble, `4` | **"all resolved"** | `473-0` + `474-0` |
| 3 | bare bubble at 50%, `0` | **"none yet — dim, still clickable"** | `479-0` + `47A-0` |

Section heading, quoted: **"The chip in the footer strip"** (`46R-0` / `AFP-0`),
18/24 w600 `--color-ink`, over a 1px `--color-border-strong` rule with `8px` of
padding beneath (`46Q-0`).

Rows 1 and 2 are the two product labels; row 3's label is annotation describing
the state, not a string to ship. **The chip ships no label at all** — it is a
glyph and a number, per R-F.3. The words "open threads" / "all resolved" are the
vocabulary for the chip's `aria-label` and for any place the count is spelled
out; the engine's current strings are `"view %d open comment thread(s)"`,
`"view %d comment thread(s), all resolved"` and `"add the first comment on this
claim"` (`components.go:664-675`), which say the same three things in the same
order.

Elapsed-time wording on this screen, quoted from the boards:

- **"6 days ago"** (`466-0` / `AER-0`) — a resolved thread's first message.
- **"HUMAN · 3 days ago"** (`46J-0` / `AFE-0`) — the read-only state's single
  meta line.

One unit, no timestamp, no second unit (R10.1, R10.3). The authorship token is
**"HUMAN"**, uppercase, mono (`465-0`); group 14 and R-J.7 give the other value
as **"AGENT"**. There is no third author.

Panel copy, quoted:

- **"Comments"** — panel/sheet title (`45V-0`, `46H-0`, `ACE-0`).
- **"No comments yet — add the first one below."** (`45X-0`, `ACR-0`).
- **"Add a comment…"** — composer placeholder (`45Z-0`, `ACW-0`).
- **"Reopen"** — the action on a resolved thread (`46B-0`, `AEW-0`).
- **"Read only. This viewer was opened as a file, so there is nothing to write
  back to."** (`46M-0`, `AFI-0`).

The first three already match the engine byte for byte (`viewer-runtime.js:715`,
`:831`, `shell.html:245`). The last one does not exist in the engine at all.

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

**Every line number below is pinned to commit `3ac8844`**, read with
`git show 3ac8844:<path>`. The working tree of this worktree is being edited by
other lanes concurrently — at the time of writing `style.css`, `graph.css`,
`render.go` and the font files were all dirty — so a `sed -n` against the
checkout can disagree by tens of lines. The selector or function name beside each
address is the durable anchor; the number is the convenience.

### Stylesheet

`internal/render/viewer/template/style.css` has two section markers that own this
screen.

1. **`internal/render/viewer/template/style.css:1589`** —
   `/* ---- comments (engine-managed review threads) -------------------------- */`
   Range **1589–1772**. Owns the chip and the baked read-only panel:
   - `.comment-chip` — `style.css:1602-1616` (the base pill: `3px 9px`, `1px
     solid var(--border)`, `999px`, `var(--card-bg)`, `var(--muted)`, **mono**
     12px)
   - `.comment-chip--open` — `style.css:1617-1621` (`--accent` border, `--accent-bg`
     ground, `--accent` text)
   - `.comment-chip--empty` — `style.css:1643-1647` (dashed, `--faint`,
     `opacity: 0`)
   - reveal-on-card-hover — `style.css:1654-1658` (`opacity: .75`)
   - chip hover/focus — `style.css:1664-1670`
   - `.comment-chip-count` — `style.css:1672-1674` (`font-weight: 600`)
   - `.card.claim-card--commented`, `.claim-tree.claim-card--commented` —
     `style.css:1679-1682` (**the 3px accent left border of board rule `47H-0`**)
   - `.comments-panel` / `.comments-panel-title` — `style.css:1684-1704`
   - `.comment-thread`, `.comment-reply`, `.comment-meta`, `.comment-role`,
     `.comment-role--human`, `.comment-role--agent`, `.comment-edited`,
     `.comment-body`, `.comments-resolved > summary` — `style.css:1706-1758`
   - `@media (pointer: coarse)` for the chip — `style.css:1764-1772`
2. **`internal/render/viewer/template/style.css:1774`** —
   `/* ---- interactive comment UI (Phase 5, serve + file://) ----------------- */`
   Range **1774–2010**. Owns the rail, the sheet and the composer:
   - `.comments-overlay` (the scrim, `var(--scrim, rgba(0,0,0,.22))`) —
     `style.css:1793-1803`
   - `.comments-rail` — `style.css:1808-1819` (`position: fixed`, `z-index: 60`,
     `width: min(380px, 90vw)`)
   - `.comments-rail-head` / `-title` / `-close` — `style.css:1829-1862`
   - `.comments-rail-body` (the scroll region) — `style.css:1864-1869`
   - `.comments-loading` / `-error` / `-empty` — `style.css:1871-1881`
   - `.comment-action`, `.comment-action-text`, `.comment-thread-actions` —
     `style.css:1906-1941`
   - `.comment-composer`, `.comment-composer-input`, `.comment-composer-submit` —
     `style.css:1943-1990`
   - `.comments-toast` — `style.css:1992-2010`
3. Touch targets — **`style.css:2210-2221`**, `@media (pointer: coarse)`.
4. The mobile sheet — **`style.css:2223-2244`**: the block comment that states
   the modal/non-modal split is `style.css:2223-2226`, the
   `@media (max-width: 860px)` opens at **`style.css:2227`**, and the
   `.comments-rail` sheet geometry is `style.css:2234-2244`
   (`max-height: 70vh` at `style.css:2240`, `border-top: 1px solid var(--border)`
   at `style.css:2242`, `border-radius: 14px 14px 0 0` at `style.css:2243`).
   `style.css:2245` already starts `.status-strip`, a different rule.
5. Chip placement in the claim head — **`style.css:2307-2309`**
   (`.k > .claim-comments-slot { margin-left: auto }`, which is R-F.1's "hard
   right") and **`style.css:3906-3916`** (the absolute-positioned variant on a
   collapsible head, `inset-inline-end: 32px`, with an `88px` end padding
   reserved on the toggle).
6. System-Record overrides that win on source order —
   **`style.css:4176-4185`** (`.comments-rail` gets `--card-bg` and
   `-14px 0 34px var(--shadow-cast)`; `.comments-rail-head` gets `--card-bg`).
7. Close-button focus ring — **`style.css:4422`**.
8. Narrow-tier chip display — **`style.css:4627-4630`**, inside
   `@media (min-width: 861px) and (max-width: 1180px)`.
9. Print — **`style.css:4753`**, inside the single `@media print` block
   (`style.css:4660`): `.comments-rail` is `display: none !important`.

`graph.css` ranges that touch this screen: **`graph.css:197-200`** declares
`--dxg-halo: #EFB44D` among the state colours, and **`graph.css:241`** re-points
it to `#C07E0C` under `@media (prefers-color-scheme: light), print`. That is the
mark board rule `47H-0` calls "a ring".

### Runtime

`internal/render/viewer/template/viewer-runtime.js`:

- rail element handles — `viewer-runtime.js:366-371`
- `chipsFor` — `viewer-runtime.js:462-468`; `setChipExpanded` —
  `viewer-runtime.js:469-473`
- `updateChips`, the chip state sync that drives §4F —
  `viewer-runtime.js:474-506`. The three-class switch itself
  (`--open` / `--empty` / `--resolved`, mutually exclusive by construction) is
  `viewer-runtime.js:481-487`, and the three `aria-label` strings §6 quotes are
  `viewer-runtime.js:488-492`
- `syncEmptyChips` (probe-gated reveal of the zero-thread chip) —
  `viewer-runtime.js:513-518`
- `commentPanelOpen` — `viewer-runtime.js:534`
- `openCommentPanel` — `viewer-runtime.js:537`; **mutual exclusion with the nav
  is `viewer-runtime.js:539`** (`setDrawer(false)`), and the title assignment
  that satisfies board rule `47E-0` is `viewer-runtime.js:542`
- `closeCommentPanel` — `viewer-runtime.js:560`
- `renderPanel` — `viewer-runtime.js:570`
- `renderPanelReadOnly` (**state 3**) — `viewer-runtime.js:583`; the empty line
  it emits is `'No comments.'` at `viewer-runtime.js:596`
- `renderPanelFromAPI` — `viewer-runtime.js:604`
- `buildPanel` — `viewer-runtime.js:672`
- `syncEmptyLine` (**state 1**) — `viewer-runtime.js:709`; the copy
  `'No comments yet — add the first one below.'` is `viewer-runtime.js:715`
- `buildThread` (resolved-class switch) — `viewer-runtime.js:730`
- `buildMessage` (author label, `<time>`, `(edited)`) —
  `viewer-runtime.js:755-775`
- `buildThreadActions` (**Resolve / Reopen**) — `viewer-runtime.js:788-800`
- `buildComposer` — `viewer-runtime.js:826`; placeholder `'Add a comment…'` at
  `viewer-runtime.js:831`, submit label `'Comment'` at `viewer-runtime.js:834`
- `buildReplyComposer` — `viewer-runtime.js:847`
- optimistic add / resolve / reopen / delete —
  `viewer-runtime.js:885-1000`
- nav↔comments mutual exclusion from the other side —
  `viewer-runtime.js:326-338`
- chip click delegation — `viewer-runtime.js:2183-2191`
- scrim click and close-button wiring — `viewer-runtime.js:2239-2244`
- Escape handling — `viewer-runtime.js:2246-2260`
- live probe adding `comments-live` — `viewer-runtime.js:1912`

`internal/render/viewer/template/shell.html`:

- runtime marker `<meta name="dossierx-viewer-runtime" content="comments-sse">` —
  `shell.html:10`
- `#commentsOverlay` (the scrim) — `shell.html:242`
- `#commentsPanel` / `.comments-rail` — `shell.html:243-249`, header
  `shell.html:244-247`, body `shell.html:248`
- `#commentsToast` — `shell.html:250`
- the sidebar build stamp the freshness rules govern — `shell.html:129`

`internal/render/viewer/template/graph-ui.js`:

- the comments overlay entry in the closed overlay set — `graph-ui.js:164`
- legend row "has an open comment thread" — `graph-ui.js:924-925`
- **`haloKind`** — `graph-ui.js:2189-2203`; the mark vocabulary comment that
  distinguishes ring from halo is `graph-ui.js:1935-1949`
- the rail's "open threads" detail row — `graph-ui.js:3577`

`internal/render/viewer/template/system-record.js` carries **two live
dependencies on the chip's markup**, both on the class `claim-comments-slot`:

- `system-record.js:222` — inside `enhanceClaimDisclosures`
  (`system-record.js:197`), the collapse-toggle rebuild moves the claim head's
  `childNodes` into the new `<button>` and **deliberately skips the comment
  slot**, so the chip stays a sibling of the toggle instead of becoming a
  control nested inside another control.
- `system-record.js:255` — inside `cleanTitle` (`system-record.js:251`), the
  derived title clones the head and strips `.pill, .claim-comments-slot` out of
  the copy, so the chip's glyph and count never leak into a System-Record title
  string.

**Consequence for a lane:** any change to `.claim-comments-slot`'s markup or
class name — which Open decisions 8 and 9 both reach into — breaks System-Record's
collapse toggle and its derived titles. This file is in scope for chip work and
must change in the same commit; the two tests that would catch it are
`viewer-tests/claim_collapse_test.go:98-112` and
`viewer-tests/theme_parity_test.go:572`.

`internal/render/viewer/template/build-order-ui.js`: **no comment code.** Its
only match for `comment` at `3ac8844` is `build-order-ui.js:217`, an ordinary
source comment about the parse-time pass.

### Go emitter

- **`internal/render/components/components.go:659-692`** — `CommentChipHTML`,
  the whole chip. Class selection is `components.go:662-676`; the three
  `aria-label` strings are `components.go:665`, `:670`, `:674`; the `<button>`
  with `aria-controls="commentsPanel"` `aria-expanded="false"` is
  `components.go:681-690`.
- **`internal/render/components/components.go:67`** — `"commentChip"` bound into
  the template FuncMap.
- **`internal/render/components/components.go:98-99`** — `commentsPanelTmpl`.
- **`internal/render/components/components.go:102-127`** — the
  `commentsPanelView` struct (`components.go:107-111`) and
  `newCommentsPanelView` (`components.go:117-127`), which partition a claim's
  threads into open and resolved. (`components.go:135` is `func Load`, a
  different concern.)
- **`internal/render/components/components.go:583-593`** — the panel appended as
  a sibling after the edges footer.
- **`internal/render/components/comments.html`** — the baked read-only panel
  (state 3's data source). The message partial with `.comment-role`,
  `<time class="comment-time" datetime>` and `(edited)` is
  `comments.html:25-27`; the thread partial `comments.html:28-35`; the
  `<details class="comments-resolved"><summary>{{len .Resolved}} resolved` at
  `comments.html:42-44`.
- **`internal/render/render.go:1190`** — the comment slot's place in the claim
  header.
- **`internal/render/depended_by_view.go:106-114`** — the doc comment on
  `attachEdgesOverride` (`depended_by_view.go:127`) that records why the chip
  does **not** ride the shared edges / depended-by footer override: *"The 💬
  comment chip no longer rides this shared footer at all. As of v0.4.1 it is
  emitted by components.CommentChipHTML, bound once as the `commentChip`
  template func and called straight from each chip-bearing partial's claim head,
  because the footer is now a collapsed `<details>` and a chip inside it would be
  invisible and unclickable on every claim whose footer starts closed."* This
  file emits **no chip on a depended-by row** — a grep of the whole file at
  `3ac8844` for `commentChip|comment-chip|claim-comments-slot` returns exactly
  one hit, `depended_by_view.go:107`, and it is inside that comment. The address
  is cited because it is the written record of the placement decision R-F.1 and
  §4F depend on — chip in the claim head, not in the footer — not because
  anything here renders a chip.

### Tests that assert on these selectors today

- `internal/render/components/comments_test.go:61` —
  `TestEdgesHTMLWithLinks_NoComments_EmptyChipHiddenByDefault` (state 1's chip)
- `internal/render/components/comments_test.go:113` —
  `TestEdgesHTMLWithLinks_OpenThread_ChipAccentAndBakedPanel`
- `internal/render/components/comments_test.go:205` —
  `TestEdgesHTMLWithLinks_ResolvedOnly_ChipMutedAndDetailsCollapsed` (the "all
  resolved" chip and the disclosure of note `46C-0`)
- `internal/render/components/comments_test.go:329` —
  `TestEdgesHTMLWithLinks_NoComposerMarkup` (**state 3's whole promise**)
- `internal/render/components/comments_test.go:350` —
  `TestEdgesHTMLWithLinks_AccessibleControls`
- `internal/render/components/comments_test.go:405` —
  `TestCommentChip_AppearsForEveryLayoutExceptBanner`
- `internal/render/components/comments_test.go:476` —
  `TestClaimCardCommented_OnlyOnOpenThreadCards` (board rule `47H-0`, left edge)
- `internal/render/comments_render_test.go:111` —
  `TestRender_EmptyChipOnQuietClaimHiddenInStaticRender`
- `internal/render/comments_render_test.go:209` —
  `TestRender_NoComposerInStaticDocument`
- `viewer-tests/empty_chip_test.go:33` —
  `TestEmptyClaimChipOpensRailAndPostsFirstComment`; asserts
  `.comment-chip--empty`, `#commentsPanel .comments-empty` and the count flip to
  `--open` (`empty_chip_test.go:39-81`)
- `viewer-tests/empty_chip_test.go:92` —
  `TestEmptyChipDoesNotMarkCardAsCommented`
- `viewer-tests/empty_chip_test.go:109` — `TestFileURLHidesEmptyChips`
- `viewer-tests/fix3_test.go:162` —
  `TestDesktopPanelIsNonModalNoBackdropNoScrollLock` (board rule `47I-0`,
  desktop half)
- `viewer-tests/fix3_test.go:188` —
  `TestMobilePanelIsModalHasBackdropAndScrollLock` (board rule `47I-0`, mobile
  half)
- `viewer-tests/claim_collapse_test.go:98-112` — the chip's geometry inside a
  collapsed claim head
- `viewer-tests/autogrow_test.go:94` / `:118` — the composer field's height
  behaviour, which R-J.5's "never shrinks" constrains
- `viewer-tests/theme_parity_test.go:572` —
  `TestThemeParityAgainstThePreChangeRender`; any colour change in §4 moves this
  baseline
- `internal/render/theme_tokens_test.go` — asserts every allowlisted token has a
  winning consumer; adding a `--scrim` consumer or removing one trips it

---

## 8. States not on the boards

Nine states the engine can render on this screen that no 14a board depicts. For
each, what the spec implies, derived from a rule rather than invented.

1. **A thread with replies.** 14a shows only single-message threads. R-J.7 is
   the binding rule: replies sit beneath the first message, indented `12px`
   behind a `2px --color-border` left rule. The engine has this at
   `style.css:1715-1719` with `margin: 6px 0 0 14px; padding-left: 10px` —
   14px + 10px, not 12px. Take R-J.7's numbers; the reply rule is
   `--color-border`, not accent, because a reply is not a second author.
2. **`AGENT` authorship.** Every 14a message is `HUMAN`. R-J.7 fixes AGENT at
   `--color-graph-facet-1` (`#4257C4`) with the same mono 11/14 w600 `+0.04em`
   form. On dark it is **`--color-dark-graph-facet-1` (`#8E9BF0`)** — a real
   Paper token, read from `get_tokens` (contentHash `cdb7b571`) and tabled in
   `tokens.md` §"Categorical colour". Group 14's note `AP0-0` calls `#8E9BF0` "a
   token candidate, not settled"; that wording predates the token and is now
   stale — the token exists, and a raw `#8E9BF0` must not be written into a dark
   rule. What is genuinely unsettled is the **value**: `tokens.md` records
   Paper's `#8E9BF0` against the engine's `graph.css` dark `#7C8CE8`, marked
   "Agrees? **no**". The light `#4257C4` sits at ~2.3:1 on `--color-dark-card`
   and is unusable on dark, so the dark token is not optional. Do **not** fall
   back to `--link`, which would make the two authors identical.
3. **An edited message.** R-J.7: a mono 11/14 **italic** `(edited)` in
   `--color-faint`, after the elapsed time. The engine emits it
   (`comments.html:25`, `style.css:1743-1745`) but only as `font-style: italic`
   with no size or colour of its own — it inherits `.comment-meta`'s mono 11px
   `--faint`, which is already correct.
4. **The Resolve action (the pre-state of state 2).** 14a draws only Reopen.
   R-J.7 gives Resolve as the same pill with a **12px tick in
   `--color-locked`** and the label "Resolve" at 12/16 w600 `--color-muted`.
   Reopen, measured here (`468-0`), is that pill with a 12px rotate-ccw arrow in
   `--color-muted`. The colour difference is the meaning: resolving is an act of
   settling, so it takes the lock green; reopening is not, so it stays neutral.
5. **The resolved-threads disclosure.** 14a shows a resolved thread *expanded*.
   R-J.7: collapsed form is a chevron plus **"N resolved"** at 12/16 w600
   `--color-muted`. The engine has it as a native `<details><summary>`
   (`comments.html:42-44`, `style.css:1753-1758`) at mono 11px `--muted` — the
   size, family and weight all need to move to Inter 12/16 w600.
6. **Optimistic / pending / deleting messages.** The engine has three
   (`.comment-thread--optimistic`, `.comment-reply--optimistic`,
   `.comment-thread--deleting` at `opacity: .55`, `style.css:1885-1889`; plus
   `.comment-pending` italic `--faint`, `style.css:1891-1894`). No board depicts
   them. The implication from state 2: this screen already spends `opacity` on
   "settled, not gone" (`0.72`). A second opacity for "not yet real" (`0.55`)
   is legible against it only because it is paired with the italic "sending…"
   text. Keep both, keep them distinct, and do not let the deleting state reach
   the resolved value.
7. **Loading and error.** `.comments-loading` ("Loading…") and
   `.comments-error` ("Could not load comments.") at `style.css:1871-1881`, both
   12px. Neither is on a board. The implication from state 1 and state 3: the
   panel always says *why* it is not showing threads, in one line, in the body's
   own slot. Loading takes `--color-muted`; error takes `--color-blocked` (the
   engine's `--warn`) — error is the one place on this screen colour is allowed,
   because it is the only place something is wrong.
8. **The zero-thread chip at rest on desktop.** The board draws it at
   `opacity: 0.5` (`476-0`) with no mention of hover. The engine draws it at
   `opacity: 0` and reveals it to `.75` on card hover
   (`style.css:1643-1658`), pinning it to `.75` under `(pointer: coarse)`. The
   board and the engine disagree by a full state. R-F.3 and note `460-0` only
   require that the chip be reachable and clickable; they do not require it to
   be visible at rest. **Keep the engine's reveal-on-hover, and read the board's
   `0.5` as the specimen's way of drawing "dim" on a page with no hover.** Never
   `display: none` — `style.css:1623-1642` explains why in full.
9. **A long claim id in the rail title, and a long comment body.** Rule `47E-0`
   requires the rail to name its claim; `style.css:1839-1850` truncates with
   `text-overflow: ellipsis` on one line. 14a draws neither a long title nor a
   wrapping body. The implication: the title is the *anchor*, so it must never
   wrap and never push the close button off-screen — one line, ellipsis, with
   the full id reachable on `title`. A body wraps freely; the engine routes it
   through the shared markdown renderer (`components.go:99`) so it can contain
   lists, fences and tables, all of which inherit `.comment-body`'s 13px.

Two further states the engine has and 14a is silent on, listed so a lane does not
invent behaviour for them: **`review_pending` trigger variants** (the graph's
halo covers both `review_pending` and open comments, and `haloKind`,
`graph-ui.js:2189`, makes review win — a claim that is both shows the review
halo, not the comment halo) and **a cycle** (a comment thread has no cycle
concept; cycles belong to `graph-core.js` and cannot reach this surface).

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

1. **The two authors' colours are R-J.7's, not rule `47F-0`'s prose.** HUMAN =
   `--color-accent` (`#1C4E8C`) / `--color-dark-accent` (`#6AA6E8`); AGENT =
   `--color-graph-facet-1` (`#4257C4`) / `--color-dark-graph-facet-1`
   (`#8E9BF0`). Both are tokens on both boards — neither author's dark value is
   a raw hex, and the `#8E9BF0` in group 14's note `AP0-0` is that token's value,
   not a loose candidate (see §8.2 for the Paper-vs-engine value disagreement
   that *is* still open). The
   board's sentence "the human is the accent, the agent is the link blue" is
   unimplementable in Paper's vocabulary and wrong in the engine's. **The
   engine's `.comment-role--human { color: var(--accent) }` at `style.css:1735`
   paints the human lock-green today and must change to `var(--link)`.** Flagged
   as the single highest-damage engine correction this screen implies.
2. **The graph mark for an open thread is the halo, not a ring.** Board rule
   `47H-0` says ring; `graph-ui.js:1940` reserves ring for lock status. Take the
   halo (`--dxg-halo`), which is what the engine already draws
   (`graph-ui.js:2193`).
3. **The scrim is `--scrim`, not the boards' raw hexes.** `#10172057` /
   `#00000085` are `--color-ink` at 34% and black at 52%; the engine token is
   `rgba(0,0,0,.22)` / `rgba(0,0,0,.42)`. The allowlist is closed, `--scrim` is
   in it, and it is already the consumer at `style.css:1801`. Implement the
   token; do not re-point it to match the board, because `--scrim` is shared
   with the nav sheet and R-J.1 requires one shell.
4. **Elapsed time is `--color-faint` in both states.** State 3's single meta line
   (`46J-0`) paints "3 days ago" in `--color-accent` because it is one text node
   with the label; state 2 (`466-0`) correctly splits it. R-J.7 fixes elapsed at
   `--color-faint`. Implement two spans in both states.
5. **`max-height: 70vh` becomes the 240/660 band.** The engine's current sheet
   height (`style.css:2240`) is a single viewport fraction. Note `AQ3-0` is an
   explicit coordinator ruling: floor ~240, ceiling 660, `fit-content` between,
   internal scroll beyond. Take the band. On a 844-tall phone 70vh ≈ 591px,
   which is inside the band, so this is a widening, not a narrowing.
6. **Sheet top corners move 14px → 16px** (`style.css:2243` vs `ACA-0` and
   R-J.2). 16 is the shell value and the nav sheet's; one shell, one radius.
7. **The dark-only top hairline.** `style.css:2242` sets
   `border-top: 1px solid var(--border)` unconditionally; the boards put it on
   dark only (`AK3-0` has it, `ACA-0` does not). Take the boards: on light the
   16px corners plus the scrim already separate the sheet from the page, and a
   light hairline there reads as a seam.
8. **`.comment-chip--resolved` has no CSS rule anywhere.** `components.go:668`
   emits the class; `style.css` never styles it, so the "all resolved" chip
   renders as the base `.comment-chip` (bordered, `--muted`). The board wants no
   border, no ground, and `--color-faint` for both glyph and count (`470-0`,
   `471-0`, `473-0`). A rule must be added; it is new surface, not a change to an
   existing one.
9. **The chip's count is mono in the engine and sans on the boards.**
   `style.css:1611` sets `font-family: var(--font-mono)` on `.comment-chip`; every
   14a count is Inter (`46X-0`, `473-0`, `479-0`). `tokens.md` §7 assigns mono to
   "machine output" and sans to "what the interface says about it". A thread
   count is an interface statement about a claim, so **take the boards: Inter**.
10. **The zero-thread chip keeps the engine's reveal-on-hover** (see §8.8). The
    board's flat `opacity: 0.5` is specimen convention, not a repeal of
    `style.css:1623-1642`'s reasoning.
11. **The board's dashed bottom edge is never implemented.** `AEG-0`, `AF5-0`,
    `AL0-0`, `ALO-0` carry `1px dashed --color-border-strong`; note `APN-0` says
    in so many words that it marks where the drawing was cut.
12. **The 290px state column is not a product width.** It is the specimen's
    column. No engine rule derives from it.
13. **The desktop rail width stays the engine's `min(380px, 90vw)`, not board
    14's drawn 360.** 14a does not measure the rail; board 14 draws it at 360
    (`43Y-0`), and group 14's spec owns that number. 14a asserts only that the
    rail is non-modal, names its claim, and is mutually exclusive with the nav.
14. **`.claim-comments-slot` is shared surface, not chip-private.** The chip's
    wrapper class is read by System-Record's collapse toggle
    (`system-record.js:222`, which excludes it when it rebuilds a claim head into
    a `<button>`) and by its `cleanTitle` (`system-record.js:255`, which strips it
    out of a derived title). Decisions 8 and 9 both change chip markup, so both
    have to touch that file. **Decision: the class name `claim-comments-slot` is
    frozen.** Add the `.comment-chip--resolved` rule and swap the count's family
    inside the existing slot; do not rename or re-nest it, because the two
    System-Record call sites are `querySelector`/`classList` string matches with
    no compile-time link to the emitter (`components.go:659-692`).
15. **Sheet chrome the engine does not have** — the 36 × 4 grabber under 8px of
    top padding, and the 16px sheet-title — is new construction, owned by
    whichever lane implements R-J.2's shell first. 14a specifies it; it does not
    claim to be the only screen that needs it.

### Paper defects found

1. **`47F-0` / `9ZM-0`** — "the human is the accent, the agent is the link blue"
   is ambiguous between Paper's and the engine's token vocabularies, and both
   readings collapse or invert the two authors. The board's own drawn value
   (HUMAN = `--color-accent`) and R-J.7 (AGENT = `--color-graph-facet-1`) are
   correct; the prose is not.
2. **`47H-0` / `9ZO-0`** — "a ring in the claims graph" names a mark the graph
   reserves for lock status. The correct mark is the halo.
3. **`AC9-0` (`#10172057`) and `AK2-0` (`#00000085`)** — raw hexes with baked
   alpha where `--scrim` exists, and note `APN-0` repeats them as if they were
   the spec. Neither matches the engine token in either mode.
4. **`AC9-0` / `AK2-0` bottom hairline** — `--color-border-strong` on a scrim is
   board furniture (it separates the vignette from the next section) with no
   product meaning, and is easy to mistake for sheet chrome. It is not.
5. **`46J-0` / `9YQ-0` / `AFE-0` / `ALW-0`** — the read-only state paints its
   elapsed time in `--color-accent`, contradicting state 2 on the same board and
   R-J.7.
6. **Every mobile board spells families as the literal
   `"Inter", system-ui, sans-serif` and `"IBM Plex Mono", system-ui, sans-serif`**
   (`ABX-0`, `AEQ-0`, `AFW-0`, and their dark twins) where the desktop boards use
   `var(--font-sans)` / `var(--font-mono)`. This is `tokens.md` Disagreement 12
   reproduced on this screen. `"IBM Plex Mono", system-ui, sans-serif` is
   additionally wrong on its face: the fallback for a mono face is `monospace`.
7. **`9VR-0` is painted `#FFFFFF`**, a raw hex, where `--color-card` exists. The
   same is true of the notes strip `APK-0`. Band and notes are canvas furniture,
   not product, so this is cosmetic — recorded for completeness.
8. **The band `9VR-0` claims "light pair left, dark pair right"**, but the four
   boards sit at worldX 0 (`45K-0`, desktop light), 1020 (`ABT-0`, mobile light),
   and the dark pair to their right. The desktop-light/mobile-light ordering is
   left-to-right within each pair, which matches; the subtitle is accurate. No
   defect — checked and cleared.
9. **State 2's dimming is applied at two different levels.** Desktop puts
   `opacity: 0.72` on the whole card `463-0` (so the card's own border dims too);
   mobile puts it on the comment block `AEO-0` only (so the sheet chrome stays at
   full strength). Mobile is right — sheet chrome is not part of the resolved
   thread — and desktop should follow it: dim the thread, not its container.
