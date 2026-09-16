# 05 · Claim — one expansion at a time

Screen group 05. Source of record: Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page
`1-0`. Every number below was read with `get_jsx` (inline-styles) or
`get_computed_styles` on the named node. Nothing here is measured from a
screenshot. Engine side read from the worktree `feat/viewer-design-revamp` at
`3ac8844`.

Read this with `docs/design/tokens.md` (the frozen token set and the light/dark
mechanism) and `docs/design/reference-rules.md` (the rules, cited below by rule
id). Where a board paints a raw hex that a token already names, the token wins —
the hex is recorded as evidence of intent and listed under **Paper defects**.

---

## 1. Boards

| Node | Name | Canvas | What it shows |
|---|---|---|---|
| `29K-0` | 05 · Claim — one expansion at a time | 1220 × 2684.5, light | The desktop board. One claim card drawn **five times**, stacked, each under a numbered caption: `1 AT REST — NOTHING OPEN`, `2 BLOCKERS OPEN`, `3 RELATIONSHIPS OPEN`, `4 SOURCES OPEN`, `5 CHECKS OPEN`. States depicted: claim status **LOCKED**; readiness **blocked** (2 blockers, `Blocked` word + dot in the footer); relationship targets at three lifecycles (LOCKED, DRAFT, LOCKED); one check with verdict **Matched**; comments **zero** (count `0`, passive). |
| `61N-0` | … · DARK | 1220 × 2684.5, dark | The same five stacks on `--color-dark-paper`. Same states, dark twins. Structure is node-for-node parallel (`61S-0`…`69K-0` mirror `29Q-0`…`2EK-0`). |
| `6K9-0` | … · MOBILE LIGHT | 390 × 3620, light | The same five states at 390. Intro block `6KG-0`, then `6KL-0` (at rest), `6MM-0` (blockers), `6QR-0` (relationships), `6VF-0` (sources), `6Y8-0` (checks). The board is a **390px column of fit height, not an 844px device frame, and carries no app bar** — its own note says so; it is a spec sheet for the strip, not a screen. |
| `79P-0` | … · MOBILE DARK | 390 × 3620, dark | Dark twin of the mobile board. Stacks `79V-0`, `7B4-0`, `7DA-0`, `7GC-0`, `7I8-0`. |
| `614-0` | Group 05 — band | 3420 × 27.5 | The group band: eyebrow `05 · CLAIM — ONE EXPANSION AT A TIME` in `--color-accent` 13/16 weight 600 tracking `0.08em`, a `--color-faint` 12/16 subtitle `four designs of one screen · light pair left, dark pair right`, and a 2px `--color-accent` rule beneath. |
| `7KR-0` | Group 05 — notes | 2828 × 141 | The notes strip: three 900px columns — `WHAT THIS SCREEN ANSWERS`, `WHAT MOBILE DOES DIFFERENTLY`, `WHAT MOBILE ADDS, AND WHAT IT KEEPS`. Eyebrows Inter 11/14 weight 600 tracking `0.08em` `--color-muted`; bodies Inter 13/20 `--color-muted`; 64px column gap, 7px label→body gap. |
| `29L-0` | Board header (inside `29K-0`) | 1140 × 140 | Eyebrow `ONE CLAIM · FIVE STATES OF THE FOOTER STRIP`, headline `The strip has four doors and opens one`, serif intro capped at 780px. |

Reference boards that bind this screen: **09 `WI-0` Placement map** (R09.1–R09.5,
R09.9) and **10 `1F7-0` Freshness** (R10.1, R10.3 — governing the elapsed-time
wording this screen's expansions do *not* carry; see §6).

States **not** on these boards are enumerated in §8.

---

## 2. Design intent

### What the reviewer should perceive

A claim is one thing. Everything else the engine knows about it — why it is not
ready, what it hangs off, what it cites, whether the code agrees — is *behind*
the claim, not beside it. The board's own headline states the thesis: **"The
strip has four doors and opens one."** The five stacks are one claim shown five
times, not five claims (`7KR-0`, column 1, verbatim).

The reviewer's eye should land on the title, then the prose, then — if and only
if they have a question — on a single quiet line of four counts. Opening one
count pushes one panel out below the footer hairline, tinted just enough to read
as *inside* the card rather than after it. Nothing else on the card moves. The
reading measure does not change.

### Notes strip, paraphrased with intent

**`7KR-0` column 1 — WHAT THIS SCREEN ANSWERS.** "Where does the rest of a claim
live, and how do I get at it without losing the claim itself." Blockers,
relationships, sources and checks are four peers in one strip under the prose —
**same order, same shape, same weight at every width** — and opening one closes
the others, so the reader is never comparing two expansions at once. *Intent:
peer-ness is the whole argument. The moment one of the four gets a different
shape, the reader starts reading rank into the strip, and there is no rank —
these are four different questions, not four levels of one question.*

**`7KR-0` column 2 — WHAT MOBILE DOES DIFFERENTLY.** The strip wraps to two rows:
the blocked chip and the comment count hold row one, the three neutral chips hold
row two — "the established mobile footer form, unchanged". The status chip moves
above the title. Every expansion row that was one desktop line becomes two: a
relationship is title then `module · status`; a source is title then
`publisher · cited once`; a check field is label above value. **The desktop label
column of 92px would have left 250px for a file path, so it goes.** The open chip
is marked by an accent outline rather than only accent text, "because at 12px the
colour alone is too quiet on a phone". *Intent: the mobile form is an
arrangement, never a different component (R-H.0, R-I.0). The 92px label column is
dropped on arithmetic, not on taste — it is the only element on the row that
cannot pay for itself at 358px.*

**`7KR-0` column 3 — WHAT MOBILE ADDS, AND WHAT IT KEEPS.** Nothing is dropped.
An earlier pass cut the two mono slug chips carrying the edge
(`mechanism-scoped-observation-authority › capabilities-own-their-dimensions`) on
the grounds that they wrap at 358px; group 06 showed they fit when stacked, and
**"the path is what 'blocked by' actually means"**, so the row stacks instead —
source slug on its own line, target slug below it behind a leading chevron. Both
groups use one form. Every count, every label and the whole check explainer
survives at both widths. One thing is *added*: **mobile carries type one weight
step heavier than desktop throughout** — open footer chip 600 against 500, closed
chips 500 against 400, relationship row title 500 against 400. **Status badges
stay at desktop's 11px.** *Intent: this is the same reasoning as R-F.2's — on a
phone, colour and hue carry less than they do on a monitor, so weight does the
work colour cannot. It is a compensation, not a restyle, which is why exactly
three elements move and the badges do not.*

**Band `614-0`.** "four designs of one screen · light pair left, dark pair right"
— the band asserts that the four boards are one design, and that a divergence
between any two of them is a defect rather than a variant.

**Board header `29L-0`.** "Blockers, relationships, sources and checks are peers
— four chips in the same strip, four expansions of the same claim. Opening one
closes the others. The same claim is shown five times below: at rest, then with
each door open."

### Rules from `reference-rules.md` that bind this screen

- **R09.1** — four expansions of *one* strip, never four panels; the strip is one
  line at rest. The desktop board's at-rest state (`2A9-0`) is a single 47px row
  and nothing else.
- **R09.2** — exactly one expansion open at a time. Board 09's node `Y6-0` is
  literally titled "EXACTLY ONE OF THE FOUR OPEN AT A TIME". This board is named
  for this rule and is its primary specimen.
- **R09.3** — no expansion opens by default; readiness is the only one that may
  auto-open, and only when the claim is blocked. This board draws the blocked
  claim with the strip **closed** (`29Q-0`, state 1), which is consistent: state
  1 is the strip's resting form, not the rendered default for *this* claim. The
  auto-open case is the one state 2 depicts.
- **R09.4** — relationships is one section with three directions in fixed order:
  `GOVERNED BY`, `DEPENDS ON`, `DEPENDED ON BY`. `2H1-0` carries exactly those
  three, in that order.
- **R09.5** — sources is its own expansion, split out of relationships. `2I5-0`
  is a separate panel with its own `SOURCES` eyebrow and its own count.
- **R09.6** — issues is a screen, not an expansion. Nothing on this board opens
  an issues panel; the blockers expansion ends in a link to the **graph**, not to
  issues.
- **R09.8** — eight demotions. Two are visible here: "shown via this dependency"
  is **cut** (the route line reads `nearest 1 hop`, not the engine's grouping
  phrase), and the claim id under each finding is **swapped for the title** (the
  blocker rows read `Capabilities own their dimensions is not locally approved`,
  with the slug relegated to the mono path chips beneath).
- **R09.9** — **no inline dependency map.** The blockers expansion carries a
  per-row breadcrumb (`source-slug › target-slug`) and a single
  `See in claims graph` link, and draws no tree. This is a correctness rule: a
  readiness chain is a DAG, and drawn as a tree it invents blockers that do not
  exist.
- **R-F.1** — four chips in fixed order (readiness, relationships, sources,
  checks) then the comment count hard right. Each chip is **a noun and a count**,
  never a score. Verified on `2A9-0`: `Blocked 2 blockers` · `4 relationships` ·
  `2 sources` · `1 check` · flex spacer · `💬 0`.
- **R-F.2** — chip variants are closed / open / blocked, and no more. The blocked
  chip always leads row one. Note the mobile-only clause: R-F.2's accent *border*
  and the 30px pill are the **mobile** form (its evidence is components section
  I3); the desktop form on this board is bare text plus a chevron with no pill and
  no border. This is not a contradiction — see **Open decisions**.
- **R-F.3** — the comment count sits hard right at every width; on desktop it is a
  **passive count** in `--color-faint`. Both boards draw it faint. The mobile
  accent-tinted variant does **not** appear on this board, because this claim has
  zero comments — see **Paper defects**.
- **R-I.0 / R-I.1 / R-I.2 / R-I.3** — a ranged row becomes a stacked row; the
  blocker path stacks and is never dropped; the relationship row becomes two
  lines with a right-ranged badge; the chip is the component and the footer's two
  rows are composition.
- **R-H.0 / R-H.2** — only three components change shape below the phone
  breakpoint; on mobile the status chip sits **above** the title, order status →
  claim → id.
- **R10.1 / R10.3** — elapsed, never stamped; one unit, never two. Nothing on this
  screen carries a time, which is compliant by omission; §6 records what would
  have to be true if one were added.
- **R12.4** — the three standing caveats on code evidence. This screen carries a
  code-evidence surface: the checks expansion (`2QT-0`) names a `FILE`, quotes
  what was `COMPARED` and `FOUND`, and offers `How this was checked`. R12.4 says
  any implementation must carry all three caveats, and its **WATCH** clause binds
  the screen directly — "a viewer is shareable… a project whose code is not
  public needs a way to turn this off… **opt-in per project, not on by default**".
  The boards draw only the switched-**on** case, so the switched-off case is
  derived in §8 item 18. LIMIT also binds: only symbol-shaped adapters have a
  snippet, so a check can legitimately reach the checks expansion with no `FILE`
  row at all.
- **R00.0** — the components board is the source of truth; a screen is an
  instance. Where this board and `377-0` disagree, the component board wins and
  the screen is flagged.

### Contradictions between the boards and their own notes

Recorded here, listed again under **Paper defects**:

1. The notes say "the open footer chip is 600 against 500" for mobile. Measured:
   the mobile open chip label (`6RW-0`) is **600** and the mobile *closed* chip
   labels are **500**; the desktop open chip label (`2CM-0`) is **500** and the
   desktop closed labels are **400**. The note is accurate. But the desktop
   *blocked-open* count (`2BN-0`, `2 blockers`, inside the state-2 strip `2B1-0`)
   is **500** against the closed
   **400** — one step, consistent — while the desktop `Blocked` word is 600 in
   both states. The note does not mention the blocked chip; its behaviour is
   inferred and is written into §4 as a rule.
2. The notes say mobile "carries no app bar — this is a spec sheet for the strip,
   not a screen". The boards are 3620px tall columns, which is consistent, but it
   means **this group specifies no mobile chrome at all**. A lane agent must not
   infer an app bar, a header or a scroll region from these boards; take those
   from group 02.
3. The board's own header prose says the claim is shown "at rest, then with each
   door open" — five states. It does **not** draw hover, focus-visible or
   disabled states for any chip. §8 says what the spec implies for them.

---

## 3. Layout

### Desktop — 1440 tier, board drawn at 1220

| Value | Measured | Node |
|---|---|---|
| Board frame width | `1220px`, `height: fit-content` | `29K-0` (dark: `61N-0`) |
| Board padding | `40px` all round | `29K-0`, `61N-0` |
| Gap between the five stacks | `32px` | `29K-0`, `61N-0` |
| Board ground | `#EFF1F4` literal (light) / `var(--color-dark-paper)` (dark) | `29K-0`, `61N-0` |
| Content column width | `1140px` (1220 − 2×40) | `29L-0`, `29Q-0`, `2AZ-0` |
| Caption → card gap | `10px` | `29Q-0`, `2AZ-0` |
| Claim card width | `1140px` including its 1px border | `29W-0` |
| Card radius | `12px` | `29W-0` |
| Card border | `1px solid var(--color-border)` (dark: `var(--color-dark-border)`) | `29W-0`, `61Y-0` |
| Card ground | `#FFFFFF` literal (light) / `var(--color-dark-card)` (dark) | `29W-0`, `61Y-0` |
| Card `overflow` | `clip` — the expansion's tint is clipped to the radius | `29W-0` |
| Card body padding | `26px` top / `18px` bottom / `32px` inline | body frame of `29W-0` |
| Card body row gap | `14px` (head → prose) | body frame of `29W-0` |
| Head row gap (title block → status chip) | `16px` | head row of `29W-0` |
| Title → id gap | `6px` | identity column of `29W-0` |
| **Prose measure** | `max-width: 760px` | prose node in `29W-0`, `61Y-0` |
| Footer strip inline inset | `margin-left: 32px; margin-right: 32px` — the hairline is **inset**, not full-bleed | `2A9-0` |
| Footer strip vertical padding | `padding-block: 15px` (row height 47px) | `2A9-0` |
| Footer strip inter-chip gap | `20px` | `2A9-0` |
| Footer strip top hairline | `1px solid #E8ECF1` (light) / `1px solid #212934` (dark) | `2A9-0`, `61Y-0` footer |
| Divider between readiness chip and the rest | `1px × 12px` rectangle in `var(--color-border)` / `var(--color-dark-border)` | `2A9-0`, `61Y-0` |
| Comment count pushed right by | a `flex-grow: 1` spacer | `2A9-0` |
| Expansion panel inline padding | `32px` — flush with the card body, **not** with the inset footer | `2XK-0`, `2H1-0`, `2I5-0`, `2QT-0` |
| Expansion panel vertical padding | `18px` top / `22px` bottom (checks: `24px` bottom) | `2XK-0`/`2H1-0`/`2I5-0`; `2QT-0` |
| Expansion panel row gap | blockers `12px`, relationships `13px`, sources `13px`, checks `16px` | `2XK-0`, `2H1-0`, `2I5-0`, `2QT-0` |
| Expansion top hairline | `1px solid #E8ECF1` / `#212934` (dark) | all four |
| Blockers panel ground | `#FBF7F7` (light) / `#EC8A8314` (dark) | `2XK-0`, `646-0` |
| Neutral panel ground (relationships / sources / checks) | `#F6F8FA` (light) / `#1B212B` = `--color-dark-code-bg` (dark) | `2H1-0`/`2I5-0`/`2QT-0`, `66P-0`/`68Z-0`/`6AR-0` |
| Blocker row indent | `padding-left: 24px` under the route header | `2XK-0` blocker list; dark `64J-0` |
| Blocker row hop-pill column | fixed `112px`, `justify-content: end` | `2XK-0` rows |
| Relationship direction-body indent | `padding-left: 19px` | `2H1-0` |
| Relationship badge column | fixed `58px`, `text-align: right` | `2H1-0` |
| Source ref column | fixed `26px`, `padding-top: 2px` | `2I5-0` |
| Checks label column | fixed `92px` | `2QT-0` field rows |
| Checks field row gap (label → value) | `20px` | `2QT-0` |
| Checks explainer measure | `max-width: 780px`; verdict sentence `max-width: 700px` | `2QY-0`, `2R4-0` |

**Sticky / scroll regions:** none. This board specifies a card in a document
flow. No part of the claim card, its footer strip or any expansion is sticky, and
no expansion scrolls internally — the panel takes its natural height and the page
scrolls. (Contrast the engine's `.claim-readiness-map-scroll`, which does scroll;
that element is R09.9-cut and has no board.)

### Mobile — 390 board

| Value | Measured | Node |
|---|---|---|
| Board frame width | `390px`, `height: fit-content` | `6K9-0`, `79P-0` |
| Board padding | `24px` top / `28px` bottom, **no inline padding** | `6K9-0`, `79P-0` |
| Gap between the five stacks | `24px` | `6K9-0`, `79P-0` |
| Board ground | `var(--color-paper)` / `var(--color-dark-paper)` | `6K9-0`, `79P-0` |
| Caption → card gap | `10px` | `6KL-0` |
| Card | **full-bleed 390px**; no radius; `1px` top **and** bottom border in `var(--color-border)` / `var(--color-dark-border)`; ground `var(--color-card)` / `var(--color-dark-card)` | `6KR-0`, `7A1-0` |
| Card body padding | `18px` top / `16px` bottom / `16px` inline | body of `6KR-0` |
| Content measure | `358px` (390 − 2×16) | prose node in `6KR-0` |
| Card body row gap | `13px` | body of `6KR-0` |
| Status chip → title gap | `9px` | head column of `6KR-0` |
| Title → id gap | `6px` | head column of `6KR-0` |
| Footer strip separator | `1px solid #E8ECF1` / `#212934`, with `margin-top: 6px` and `padding-top: 13px` | footer of `6KR-0`, `7A1-0` |
| Footer strip row gap | `8px` between row one and row two | footer of `6KR-0` |
| Footer row one gap | `10px` (blocked chip · spacer · comment count) | footer of `6KR-0` |
| Footer row two gap | `8px` between the three neutral chips | footer of `6KR-0` |
| Expansion panel padding | `16px` top / `18px` bottom / `16px` inline | `6OA-0`, `6SQ-0`, `6XH-0`, `702-0` |
| Expansion panel row gap | blockers `12px`, others `13px` | `6OA-0`; `6SQ-0`/`6XH-0`/`702-0` |
| Blocker row indent | `padding-left: 22px` (desktop is 24px) | `6OA-0` |
| Relationship direction-body indent | `padding-left: 19px`; meta line indented a further `15px` | `6SQ-0` |
| Source ref column | fixed `22px` (desktop is 26px) | `6XH-0` |
| Checks field rows | **no label column**; label stacks above value, row gap `5px`, `padding-block: 11px` | `702-0` |
| Checks panel inner measure | `358px`, field block `342px` | `702-0` |

---

## 4. Components on this screen

**Measured value count for the desktop light board (`29K-0`): 142 distinct
values recorded below** (colours, families, sizes, leadings, weights, trackings,
radii, paddings, gaps, borders, icon sizes and fixed column widths, counted once
per component per state). Every one carries its node id. Dark twins are given as
`--color-dark-*` token names per the standing rule; where a board painted a raw
hex the hex is printed beside the token as evidence, and the token is what ships.

### 4.1 Claim card shell

| Property | Light | Dark | Node |
|---|---|---|---|
| Ground | `--color-card` `#FFFFFF` (board writes the literal `#FFFFFF`) | `--color-dark-card` `#161B22` | `29W-0` / `61Y-0` |
| Border | `1px solid --color-border` `#DDE2E9` | `1px solid --color-dark-border` `#242C38` | `29W-0` / `61Y-0` |
| Radius | `12px` (**off-scale** — not `--radius-sm/md/pill`) | same | `29W-0` |
| Overflow | `clip` | same | `29W-0` |
| Padding | `26px / 32px / 18px / 32px` | same | body of `29W-0` |
| Mobile | full-bleed, radius `0`, 1px top+bottom border, padding `18px / 16px / 16px / 16px` | same, dark tokens | `6KR-0` / `7A1-0` |

### 4.2 Claim title

| Property | Desktop | Mobile | Node |
|---|---|---|---|
| Family | `--font-sans` (Inter) | Inter, spelled as the literal `"Inter", system-ui, sans-serif` | `29W-0` / `6KR-0` |
| Size / leading | `20px / 26px` (`--text-h2` + `--leading-title`) | `18px / 24px` (**off-scale**, sanctioned by R-H.2) | `29W-0` / `6KR-0` |
| Weight | `600` (`--font-weight-semibold`) | `600` | both |
| Tracking | `-0.01em` (**no token**) | `-0.01em` | both |
| Colour | `--color-ink` `#101720` / dark `--color-dark-ink` `#E6EAF0` | same | `29W-0` / `61Y-0` / `6KR-0` / `7A1-0` |

### 4.3 Claim id (slug)

| Property | Desktop | Mobile | Node |
|---|---|---|---|
| Family | `--font-mono` (IBM Plex Mono) | IBM Plex Mono, spelled literally | `29W-0` / `6KR-0` |
| Size / leading | `12px / 16px` | `11px / 15px` | `29W-0` / `6KR-0` |
| Weight | `400` | `400` | both |
| Colour | `--color-faint` `#6E7C8E` / dark `--color-dark-faint` `#8494A8` | same | `29W-0` / `61Y-0` |
| Text | `permission-readiness.contract.mechanism-scoped-observation-authority` | same | `29W-0` |

### 4.4 Claim prose

| Property | Desktop | Mobile | Node |
|---|---|---|---|
| Family | `--font-serif` (Source Serif 4) | Source Serif 4, spelled literally | `29W-0` / `6KR-0` |
| Size / leading | `17px / 28px` (`--text-body` + `--leading-body`) | `16px / 25px` (**off-scale**) | `29W-0` / `6KR-0` |
| Weight | `400` | `400` | both |
| Measure | `max-width: 760px` | `width: 358px` | `29W-0` / `6KR-0` |
| Colour | `--color-ink` / dark `--color-dark-ink` | same | `29W-0` / `61Y-0` |

### 4.5 Status chip (LOCKED)

| Property | Desktop | Mobile | Node |
|---|---|---|---|
| Ground | `--color-locked-bg` `rgb(44 107 82 / 10%)` / dark `--color-dark-locked-bg` `rgb(99 190 154 / 13%)` | board writes `#2C6B521A` (light) and `#63BE9A21` (dark) — **use the tokens** | `29W-0`, `61Y-0` / `6KR-0`, `7A1-0` |
| Radius | `999px` (`--radius-pill`) | same | both |
| Padding | `4px` block, `8px` left, `10px` right | `3px` block, `7px` left, `9px` right | `29W-0` / `6KR-0` |
| Gap (icon → label) | `5px` | `5px` | both |
| Padlock icon | `12px`, `stroke-width 2.4`, `stroke-linecap round` | `11px`, same stroke | `29W-0` / `6KR-0` |
| Icon colour | board writes literal `#2C6B52` on light, `var(--color-dark-locked)` on dark | mobile uses `var(--color-locked)` / `var(--color-dark-locked)` correctly | `29W-0` (defect) |
| Label | `LOCKED`, Inter `11px / 14px`, weight `600`, tracking `+0.05em` | identical — **status badges stay at desktop's 11px** per the notes strip | `29W-0` / `6KR-0` |
| Label colour | `--color-locked` `#2C6B52` / dark `--color-dark-locked` `#63BE9A` | same | all four |
| Position | right of the title, `align-items: start` | **above** the title (R-H.2) | `29W-0` / `6KR-0` |

### 4.6 Footer strip — desktop, closed (state 1)

Container: `2A9-0` — `display: flex`, `align-items: center`, `gap: 20px`,
`margin-inline: 32px`, `padding-block: 15px`, `border-top: 1px solid #E8ECF1`
(dark `#212934`).

**Readiness chip, closed + blocked** (`2A9-0`, first group; dark `61Y-0`):

| Part | Value |
|---|---|
| Layout | `display: flex`, `gap: 6px` — **no pill, no border, no ground on desktop** |
| Status dot | `7 × 7px`, `border-radius: 999px`, `--color-blocked` `#9E3B36` / dark `--color-dark-blocked` `#EC8A83` |
| Status word | `Blocked`, Inter `13px / 16px`, weight `600`, `--color-blocked` / dark `--color-dark-blocked` |
| Count | `2 blockers`, Inter `13px / 16px`, weight `400`, `--color-muted` `#54606F` / dark `--color-dark-muted` `#9AA7B8` |
| Chevron | `12px` down-chevron, `stroke-width 2.4`, `--color-faint` `#6E7C8E` on light; **dark board paints it `--color-dark-blocked`** — asymmetry, see Paper defects |

**Divider**: `1px × 12px` rectangle, `--color-border` / `--color-dark-border`.
Present **once**, after the readiness chip only.

**Neutral chips, closed** (`2A9-0`, groups 2–4):

| Part | Value |
|---|---|
| Layout | `display: flex`, `gap: 5px` — no pill, no border |
| Label | `4 relationships` / `2 sources` / `1 check`, Inter `13px / 16px`, weight `400`, `--color-muted` / dark `--color-dark-muted` |
| Chevron | `12px` down, `stroke-width 2.4`, `--color-faint` / dark `--color-dark-faint` |

**Comment count** (`2A9-0`, last group; R-F.3):

| Part | Value |
|---|---|
| Position | hard right, pushed by a `flex-grow: 1` spacer |
| Layout | `display: flex`, `gap: 6px` |
| Icon | speech bubble, `14px`, `stroke-width 2`, `--color-faint` / dark `--color-dark-faint` |
| Count | `0`, Inter `13px / 16px`, weight `400`, `--color-faint` / dark `--color-dark-faint` |
| Register | **passive** on desktop — no ground, no accent (R-F.3) |

### 4.7 Footer strip — desktop, open

Two distinct open forms, and the difference is load-bearing.

**Neutral chip, open** (`2CM-0`, the relationships chip in state 3):

| Part | Value |
|---|---|
| Label | `--color-accent` `#1C4E8C` / dark `--color-dark-accent` `#6AA6E8`, Inter `13px / 16px`, weight **`500`** (closed is `400`) |
| Chevron | flipped **up**, `12px`, `stroke-width 2.4`, `--color-accent` / dark `--color-dark-accent` |
| Ground / border | **none** — desktop marks open with colour + weight only |

**Blocked chip, open** (`2BK-0`, the leading chip inside the state-2 footer strip
`2B1-0`): the blocked chip **keeps its blocked hue when open; it does not turn
accent.** Chip `gap: 6px`; dot `2BP-0`, word `2BO-0`, count `2BN-0`, chevron
`2JF-0`.

| Part | Value |
|---|---|
| Dot | unchanged, `7px`, `--color-blocked` |
| Status word | `Blocked`, weight `600`, `--color-blocked` — unchanged |
| Count | `2 blockers`, weight **`500`** (closed is `400`), colour **`--color-blocked`** (closed is `--color-muted`) |
| Chevron | flipped **up**, `--color-blocked` (closed light is `--color-faint`) |

*Rule this implies:* openness is signalled by the chevron flip plus one weight
step plus a colour promotion; **which** colour it promotes to is the chip's own
status colour, and only a neutral chip has no status colour of its own and so
borrows the accent.

### 4.8 Footer strip — mobile (two rows, R-F.4)

Row one (`6KR-0` footer, first row): blocked chip left, `flex-grow` spacer
(`min-width: 4px`), comment count right.
Row two: the three neutral chips, `gap: 8px`.

**Blocked chip, mobile** (`6KR-0`; dark `7A1-0`):

| Part | Value |
|---|---|
| Ground | board writes `#9E3B3617` (light) / `#EC8A8321` (dark) — ship `--color-blocked-bg` / `--color-dark-blocked-bg` |
| Height / radius / padding | `30px`, `999px`, `padding-inline: 11px` — matches R-F.2 exactly |
| Gap | `7px` |
| Dot | `7 × 7px`, `--color-blocked` / `--color-dark-blocked` |
| Status word | `Blocked`, Inter `12px / 16px`, weight `600`, `--color-blocked` |
| Count | `2 blockers`, Inter `12px / 16px`, weight `400`, `--color-blocked` |
| Chevron | `11px` down, `stroke-width 2.6`, `--color-blocked` |
| Border | none (the tint *is* the chip) |

**Neutral chip, closed, mobile** (`6KR-0` row two):

| Part | Value |
|---|---|
| Ground | `--color-card` / `--color-dark-card` |
| Border | `1px solid --color-border` / `--color-dark-border` |
| Height / radius / padding / gap | `30px`, `999px`, `padding-inline: 11px`, `gap: 6px` |
| Label | Inter `12px / 16px`, weight **`500`**, `--color-muted` / `--color-dark-muted`, `width: max-content` |
| Chevron | `11px` down, `stroke-width 2.6`, `--color-faint` / `--color-dark-faint` |

**Neutral chip, open, mobile** (`6RV-0`):

| Part | Value |
|---|---|
| Ground | `--color-card` |
| Border | `1px solid --color-accent` — the accent **outline**, which is the mobile-only addition |
| Height / radius / padding / gap | `30px`, `999px`, `padding-inline: 11px`, `gap: 6px` |
| Label | Inter `12px / 16px`, weight **`600`**, `--color-accent` |
| Chevron | `11px` **up**, `stroke-width 2.6`, `--color-accent` |

**Comment count, mobile** (`6KR-0` row one, right): `gap: 6px`, `14px` bubble
icon at `stroke-width 2`, count `0` Inter `13px / 16px` weight `400`, both
`--color-faint` / `--color-dark-faint`. It is **not** accent-tinted here because
the count is zero — R-F.3's accent variant is the control state, and see §8.

### 4.9 Expansion — readiness blockers (`2XK-0` desktop, `646-0` dark, `6OA-0` mobile)

**Panel:** ground `#FBF7F7` (light) / `#EC8A8314` (dark — the dark-blocked token
at ~8 % over the card, which is the `--color-dark-blocked-surface` idea from
`tokens.md` applied as a tint rather than a surface). Top hairline `1px #E8ECF1`
/ `#212934`.

**Section header row** (`2XK-0` first child / `647-0`): `gap: 12px`.
- Eyebrow `READINESS BLOCKERS`, Inter `11px / 14px`, weight `600`, tracking
  `+0.08em` (`--tracking-label`), `--color-blocked` / `--color-dark-blocked`.
- Flexible hairline, `1px`, `#EBD9D8` (light) / `#EC8A8338` (dark) — a tinted red
  rule, **no token**.
- Right count `2 in 1 module`, Inter `12px / 16px`, weight `400`, `--color-faint`
  / `--color-dark-faint`.

**Route row** (`2XK-0` second child): `gap: 12px` desktop / `10px` mobile,
`padding-block: 2px`.
- Chevron `12px`, `stroke-width 2.4`, `--color-muted` / `--color-dark-muted`.
- Module name `Capability support`, Inter `14px / 18px`, weight `600`,
  `--color-ink` / `--color-dark-ink`.
- Hop label `nearest 1 hop`, Inter `12px / 16px`, weight `400`, `--color-faint`.
- Right count pill: ground `#9E3B361A` (→ `--color-blocked-bg`), radius `999px`,
  `padding: 3px / 9px`, label `2` Inter `11px / 14px` weight `600`
  `--color-blocked`.

**Blocker rows** (the blocker list `2XX-0`, third and last child of `2XK-0`; rows
`2XY-0` and `2YB-0`; dark `64J-0`; mobile `6OA-0`): indented `24px`
(mobile `22px`), each row `border-top: 1px solid #EFE6E5` (light) / `#EC8A832E`
(dark) — **hairline-divided rows, no cards** (R00.0's own cautionary tale).
`padding-block: 10px`, `gap: 14px` desktop.
- Title, Inter `14px / 20px`, weight `600`, `--color-ink` / `--color-dark-ink`.
  Mobile fixes `width: 314px`.
- Source slug chip: ground `#EFF1F4` (→ `--color-paper`) / `--color-dark-code-bg`,
  radius `4px` (`--radius-sm`), `padding: 2px / 7px`, IBM Plex Mono `11px / 14px`,
  `--color-muted` / `--color-dark-muted`.
- Chevron between slugs: `11px`, `stroke-width 2.6`, `--color-faint` /
  `--color-dark-faint`. Mobile adds `margin-top: 3px` and the pair **stacks**
  (R-I.1): source slug on its own line, then chevron + target slug on the next,
  `gap: 5px`.
- Target slug chip: ground `#9E3B361A` (→ `--color-blocked-bg`) / `#EC8A831F`
  (→ `--color-dark-blocked-bg`), radius `4px`, same padding and type, colour
  `--color-blocked` / `--color-dark-blocked`.
- Hop pill, desktop: right-ranged in a fixed `112px` column; ground `#EFF1F4`
  (→ `--color-paper`) / `--color-dark-code-bg`, radius `999px`, `padding: 3px /
  9px`, Inter `11px / 14px` weight `500`, `--color-muted` / `--color-dark-muted`,
  text `direct · 1 hop` and `upstream · 2 hops`. Mobile: the same pill, moved to
  its own line **above** the slugs, `width: fit-content`, no column.

**Panel footer** (`3AX-0` — the last child of the blocker list `2XX-0`, **not** of
the panel `2XK-0`): `padding-top: 12px`, `gap: 10px`.
- Summary sentence `Both sit in capability-support. One approval there clears
  this claim.` — **Source Serif 4** `14px / 22px`, `--color-muted` /
  `--color-dark-muted`. Mobile `width: 336px`, on its own line above the link.
- Link `See in claims graph`: `13px` node-graph icon at `stroke-width 2` +
  Inter `13px / 16px` weight `500`, both `--color-accent` /
  `--color-dark-accent`, `gap: 6px`. This is the **only** navigation out of the
  blockers expansion, and it goes to the graph (R09.9).

### 4.10 Expansion — relationships (`2H1-0` desktop, `66P-0` dark, `6SQ-0` mobile)

**Panel:** ground `#F6F8FA` (light — should be `--color-code-bg` `#F3F5F8`) /
`--color-dark-code-bg` `#1B212B`. Hairline `#E8ECF1` / `#212934`. `gap: 13px`.

**Section header** (`66Q-0` is the tokenised dark twin): eyebrow `RELATIONSHIPS`
Inter `11px / 14px` weight `600` tracking `+0.08em` `--color-faint` /
`--color-dark-faint`; `1px` rule `#E3E8EE` (→ `--color-border`) /
`--color-dark-border`; right count `4` in **IBM Plex Mono** `11px / 14px`
`--color-faint`.

**Direction headers** — three, in the fixed order `GOVERNED BY` (up-arrow),
`DEPENDS ON` (down-arrow), `DEPENDED ON BY` (right-arrow), R09.4:
- Arrow icon `12px`, `stroke-width 2.4` desktop / `2.2` mobile,
  `stroke-linejoin round`, `--color-faint` / `--color-dark-faint`.
- Label Inter `11px / 14px`, weight `600`, tracking **`+0.06em`** desktop
  (`+0.08em` on mobile — see Paper defects), `--color-faint`.
- Optional mono count (`1`, `2`) IBM Plex Mono `11px / 14px` `--color-faint`.
  `GOVERNED BY` carries **no** count because it has exactly one target.
- `gap: 7px`; non-first headers get `padding-top: 3px` on desktop.

**Relationship row, desktop** (`2H1-0`): one line, four columns, `gap: 10px`,
`padding-left: 19px`.
- Lifecycle dot `7 × 7px` `999px`, colour is the target's lifecycle:
  `--color-locked` `#2C6B52`, `--color-draft` `#9A6A16`, `--color-blocked`
  `#9E3B36`. **Note:** the board uses `--color-blocked` for the two
  `DEPENDED ON BY` rows whose badges read `LOCKED` — the dot is the *edge's*
  readiness, not the target's lifecycle. See Paper defects.
- Title, Inter `14px / 18px`, weight **`400`**, `--color-accent` (it is a link).
- Meta `Curtainly · Doctrine`, Inter `12px / 16px`, `--color-faint`.
- Badge, fixed `58px` column, `text-align: right`, Inter `11px / 14px`, weight
  `600`, tracking `+0.05em`, in the lifecycle colour (`--color-locked` /
  `--color-draft`).

**Relationship row, mobile** (`6SQ-0`, R-I.2): **two lines**.
- Line one: dot (`7px`, `margin-top: 6px`) + title, `gap: 8px`. Title Inter
  `14px / 20px`, weight **`500`**, `--color-accent`.
- Line two: `padding-left: 15px`, `gap: 8px`, meta Inter `12px / 16px`
  `--color-faint` on the left with `flex-grow: 1`, badge **right-ranged** with
  `width: max-content` and `flex-shrink: 0` — **no fixed slot**, Inter
  `11px / 14px` weight `600` tracking `+0.06em` in the lifecycle colour.
- Row gap `3px`; direction block gap `7px`.

### 4.11 Expansion — sources (`2I5-0` desktop, `68Z-0` dark, `6XH-0` mobile)

**Panel:** same grounds, hairlines and `gap: 13px` as relationships.

**Section header:** eyebrow `SOURCES`, same type as `RELATIONSHIPS`; mono count
`2`.

**Source row, desktop** (`2I5-0`): `gap: 12px`.
- Ref `[1]`, IBM Plex Mono `12px / 16px`, `--color-faint`, in a fixed `26px`
  column with `padding-top: 2px`.
- Title `Change Privacy & Security settings on Mac`, **Source Serif 4**
  `15px / 23px`, weight `400`, `--color-ink`.
- Publisher `macOS User Guide · Apple Support`, Inter `12px / 16px`,
  `--color-faint`; title→publisher gap `3px`.
- Citation count `cited once`, Inter `12px / 16px`, `--color-faint`, its own
  column, `padding-top: 3px`.

**Source row, mobile** (`6XH-0`): `gap: 10px`; ref column `22px`; title serif
`15px / 22px`; **publisher and `cited once` share line two**, publisher
`flex-grow: 1`, `cited once` `flex-shrink: 0`, `gap: 8px`. Both `--color-faint`
Inter `12px / 16px`.

### 4.12 Expansion — implementation checks (`2QT-0` desktop, `6AR-0` dark, `702-0` mobile)

**Panel:** `gap: 16px` (desktop) / `13px` (mobile), `padding-bottom: 24px`
desktop.

**Section header:** eyebrow `IMPLEMENTATION CHECKS`, same type; mono count `1`.

**Explainer** (`2QY-0`): Source Serif 4 `15px / 24px`, `--color-muted`,
`max-width: 780px`. Mobile (`702-0`) `14px / 22px`, `width: 358px`. Text:
`Does the code match what this claim says? Each check compares one declared
expectation against a reading of the real source. It says nothing about whether
the claim is approved.`

**Check title row** (`2QT-0`): `align-items: baseline`, `gap: 14px`,
`padding-top: 4px`.
- Title `Requirement key fields`, Inter `16px / 20px` weight `600` `--color-ink`
  (mobile `15px / 20px`).
- Verdict: `7px` dot + label `Matched`, Inter `13px / 16px`, weight **`600`**
  desktop / **`500`** mobile, both `--color-locked` / `--color-dark-locked`,
  `gap: 6px`.

**Verdict sentence** (`2R4-0`): Source Serif 4 `15px / 23px`, `--color-muted`,
`max-width: 700px`, `margin-top: -8px`. Mobile `14px / 22px`, no negative margin.

**Field rows, desktop** (`2QT-0`): four rows — `EXAMINED`, `FILE`, `COMPARED`,
`FOUND` — each `border-top: 1px solid #E3E8EE` (→ `--color-border`) /
`--color-dark-border`, `padding-block: 10px`, `gap: 20px`,
`align-items: baseline`.
- Label column: fixed `92px`, Inter `11px / 14px`, weight `600`, tracking
  `+0.06em`, `--color-faint`.
- `EXAMINED` value: `RequirementKey` IBM Plex Mono `13px / 16px` weight `500`
  `--color-ink`, then `a type in the Permission Readiness module` Source Serif 4
  `14px / 18px` `--color-muted`, `gap: 8px`, wrapping.
- `FILE` value: directory prefix
  `Modules/PermissionReadiness/Sources/PermissionReadiness/` IBM Plex Mono
  `12px / 16px` in `#8592A3` (**pre-token residue** — ship `--color-faint`), then
  filename `Requirement.swift` IBM Plex Mono `13px / 16px` weight `500`
  `--color-accent`. This matches R12.2's split of prefix from filename.
- `COMPARED` value: `its stored members, as a set — order carries no meaning`,
  Source Serif 4 `14px / 22px`, `--color-muted`. (R12.3 stated inline.)
- `FOUND` value: two tick chips — `11px` check glyph at `stroke-width 3.2` in
  `--color-locked`, `gap: 7px`, member name IBM Plex Mono `13px / 16px`
  `--color-ink` — then `and nothing else` Source Serif 4 `14px / 18px`
  `--color-faint`. Row `gap: 16px`.

**Field rows, mobile** (`702-0`): the `92px` label column is **removed**; each
row is `flex-direction: column`, `gap: 5px`, `padding-block: 11px`, label above
value. Label tracking becomes `+0.08em`. Mono values drop to `12px / 16px`; the
`FILE` prefix drops to `11px / 16px` with `width: 342px` and the filename sits on
its own line at `11px / 16px` `--color-accent`, `gap: 1px`. Tick glyph
`stroke-width 2.6`, `gap: 5px`, row `gap: 10px`.

**Panel footer**: `How this was checked` — `12px` right-chevron at
`stroke-width 2.6` (desktop) / `2.4` (mobile) + Inter `13px / 16px` weight `500`,
both `--color-accent` / `--color-dark-accent`, `gap: 7px` desktop /
`8px` mobile, `padding-top: 14px` desktop / `13px` mobile. On mobile it takes its
own `1px #E3E8EE` top rule. This is the disclosure board 06a owns (R12.1).

### 4.13 Board furniture (not part of the viewer)

Numbered captions `29R-0`: badge ground `#54606F` (`--color-muted`), radius `4px`,
`padding: 2px / 8px`, numeral Inter `10px / 12px` weight `700` tracking `+0.05em`
in `#FFFFFF`; caption Inter `12px / 16px` weight `600` tracking `+0.08em`
`--color-faint`; trailing `1px` rule `--color-border`; `gap: 10px`. **Weight 700
does not exist in the frozen type scale** — this is board chrome, not viewer
chrome, and does not ship.

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. **No 390px breakpoint is
added.** Per `tokens.md` §8: arrangement is a width question and belongs in
`@media (max-width: 520px)`; target size and hover-less affordances are a pointer
question and belong in `@media (pointer: coarse)`.

| # | Rule at 390 | Engine query | Why |
|---|---|---|---|
| M1 | Status chip moves **above** the claim title; order status → title → id (R-H.2) | `max-width: 520px` | Arrangement. Phone tier; 390 < 520 |
| M2 | Claim card goes **full-bleed**: radius `0`, 1px top+bottom border only, `16px` inline padding | `max-width: 520px` | Arrangement |
| M3 | Footer strip splits into **two rows** — blocked chip + comment count, then the three neutral chips (R-F.4) | `max-width: 520px`, written to win over the **two** existing rules that actually touch `.claim-footer__counts`: `max-width: 860px` (block opens `style.css:4533`; sets `flex-wrap: wrap; justify-content: flex-end` at `:4640` and `font-size: 8.5px` at `:4641`) and `max-width: 560px` (block opens `style.css:4647`; sets `justify-content: flex-start` at `:4655` and `padding: 4px 6px` at `:4656`). All three queries are equal-specificity, so source order decides: the 520 rules must go in a **new** block after `style.css:4658`, **not** into the existing `max-width: 520px` at `style.css:1175`, which sits before both competitors and would lose. The `max-width: 640px` block at `style.css:2563` is **not** one of them — its body is `.gcp-console` / `.gcp-row` / `.gcp-sev` / `.gcp-time` and it names no claim-footer selector | Arrangement. Both competing queries also match at 390, so a 520 rule that does not restate `flex-wrap`, `justify-content`, `font-size` and `padding` inherits the 860/560 values and loses |
| M4 | Every chip becomes a 30px `--radius-pill` pill with `11px` inline padding; neutral chips gain a `--color-border` outline, the blocked chip a `--color-blocked-bg` tint | `max-width: 520px` | Arrangement. The **height** of 30px is below the 44px touch minimum and is therefore *not* the tap target — see M8 |
| M5 | Open chip gains a `1px --color-accent` **border** in addition to the accent label (`6RV-0`) | `max-width: 520px` | Arrangement/affordance at width. The board's own reason: at 12px, colour alone is too quiet |
| M6 | Type goes one weight step heavier: open chip `600` (desktop `500`), closed chips `500` (desktop `400`), relationship row title `500` (desktop `400`). **Status badges stay at 11px and weight 600** | `max-width: 520px` | Arrangement-adjacent compensation, stated as a width rule on the board. It is not a pointer question — a 520px desktop window should get it too |
| M7 | Blocker row **stacks** the dependency path: hop pill on its own line, then source slug, then chevron + target slug (R-I.1) | `max-width: 520px` | Arrangement. The path is never dropped |
| M8 | Every chip, the comment count and `How this was checked` must reach a **44px minimum hit target**, achieved with padding/`min-height` rather than by growing the 30px pill's visual box | `(pointer: coarse)` — the engine already sets `.comment-chip { min-height: 44px }` there (`style.css:1764`) | Touch, not width. Do not put this behind a width query |
| M9 | Relationship row becomes **two lines** with the badge right-ranged and no fixed-width slot (R-I.2) | `max-width: 520px` | Arrangement |
| M10 | Source row: publisher and `cited once` share line two; ref column narrows `26px → 22px` | `max-width: 520px` | Arrangement |
| M11 | Checks field rows drop the `92px` label column; label stacks above value | `max-width: 520px` | Arrangement. The board's arithmetic (`92px` would leave `250px` for a file path) is the *reason*, not a second breakpoint |
| M12 | Expansion panels keep `16px` inline padding and their hairline dividers; nothing becomes a card | `max-width: 520px` | Arrangement |

**Bottom sheets: none on this screen.** Per R-J.1 there is one sheet shell in the
product, and this screen introduces no body for it. The four expansions are
**inline disclosures at every width** — the mobile board draws them inline, in
the card, below the footer strip. A lane agent must not promote an expansion to a
sheet: doing so would break R09.1 (four expansions of one strip) and would put a
second shell in the product. The sheet is reached from this screen only through
the comment count, whose sheet body is specified by group 14, not here.

**Hidden at 390:** nothing. The notes strip is explicit — "Nothing is dropped.
… Every count, every label and the whole check explainer survive at both widths."
The only element that disappears is the desktop `92px` label column, and its
*content* survives as a stacked label.

---

## 6. Footer vocabulary

### The metadata strip, quoted from the boards

Order is fixed (R-F.1) and identical at both widths. Desktop reads left to right
on one line; mobile splits after the first item and pushes the count to row one's
right edge.

| Position | Exact words (`2A9-0`, `6KR-0`) | Register |
|---|---|---|
| 1 | `Blocked` then `2 blockers` | status word + noun-and-count |
| — | (1px vertical divider, desktop only) | |
| 2 | `4 relationships` | noun and count |
| 3 | `2 sources` | noun and count |
| 4 | `1 check` | noun and count — **singular, not "1 checks"** |
| 5 (hard right) | speech-bubble glyph then `0` | bare count, no noun |

Each of items 2–4 is **a noun and a count and nothing else**. There is no score,
no percentage, no "ready" word, and no ratio anywhere in the strip.

### Expansion headers and their right-hand counts

| Expansion | Eyebrow (verbatim) | Right-hand value |
|---|---|---|
| Readiness | `READINESS BLOCKERS` | `2 in 1 module` (Inter, not mono) |
| Relationships | `RELATIONSHIPS` | `4` (mono) |
| Sources | `SOURCES` | `2` (mono) |
| Checks | `IMPLEMENTATION CHECKS` | `1` (mono) |

The readiness count is the only one that is a **phrase** rather than a numeral,
and the only one set in Inter rather than mono: it states a *shape* (two blockers
that collapse into one module), which is the fact the summary sentence then
spends a sentence on.

### Other quoted strings on this screen

- Route row: `Capability support` · `nearest 1 hop` · pill `2`
- Blocker hop pills: `direct · 1 hop`, `upstream · 2 hops`
- Blocker titles: `Capabilities own their dimensions is not locally approved`,
  `The five verdicts is not locally approved`
- Blockers summary: `Both sit in capability-support. One approval there clears
  this claim.`
- Blockers action: `See in claims graph`
- Relationship directions: `GOVERNED BY`, `DEPENDS ON`, `DEPENDED ON BY`
- Relationship badges: `LOCKED`, `DRAFT`
- Relationship meta: `Curtainly · Doctrine`, `Capability support · Contract`,
  `Permission readiness · Contract`
- Sources: `[1]`, `[2]`, `cited once`
- Checks verdict: `Matched`
- Checks labels: `EXAMINED`, `FILE`, `COMPARED`, `FOUND`
- Checks values: `a type in the Permission Readiness module`, `its stored
  members, as a set — order carries no meaning`, `and nothing else`
- Checks action: `How this was checked`
- Status chip: `LOCKED`

### Freshness and elapsed time

**This screen carries no time of any kind** — no "updated N ago", no timestamp,
no build stamp. That is correct and deliberate: R10.1 places the elapsed phrase
in the **right-rail footer**, and this board has no rail. If a time ever appears
on a claim card (for example a comment's age surfacing in the count), R10.1 and
R10.3 bind it without amendment: **elapsed, never stamped; one unit, never two**
("3 days ago", not "3 days 20 hours"), the exact instant only on hover as a
`title` attribute, the phrase computed in the browser (R10.4), and the word
**`Live`** instead of a phrase behind `dossierx serve` (R10.5).

### What the engine says today, for contrast

The engine's evidence footer (`internal/render/viewer/template/system-record.js:122`)
rewrites the server's digest into: title **`Evidence & relationships`**, subtitle
**`Trace this claim through its supporting record`**, and count pills in the
order **`N relationships`, `N sources`, `N files`, `N drifted`**, then a chevron.
The board's vocabulary differs in four ways, all of which are the spec: there is
no title and no subtitle; `files` and `drifted` do not appear in the strip;
`checks` does; and the readiness item leads with a status word.

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.
**Every line number below is cited against commit `3ac8844`** (`git show
3ac8844:<path>`), which is the baseline this freeze is pinned to. The working
tree at the time of writing already carries **uncommitted** changes from a
concurrent lane — `internal/config/presets.go`, `internal/render/render.go`,
`internal/render/viewer/template/graph.css`,
`internal/render/viewer/template/style.css` modified;
`internal/render/geist_fonts.go` and the two Geist woff2 files deleted;
`internal/render/engine_fonts.go`, `engine_fonts_test.go` and a new
`viewer/template/fonts/` added (the serif-family work `tokens.md` disagreement 6
calls for). That lane shifts `style.css` by roughly **+88 lines** below line
~800, so a working-tree read will not match these numbers. Each row carries the
selector or marker text so the address is recoverable after any shift.

### 7.1 `internal/render/viewer/template/style.css`

| Lines | Marker comment (quoted) | What it owns today |
|---|---|---|
| `789–806` | `/* .claim-links is the <details> that wraps the whole edges footer` | The structural rule for the footer disclosure: `margin: 0.4rem 0 0; padding: 0.35rem 0 0; border-top: 1px solid var(--border)` |
| `807–1158` | `/* Claim readiness is a reviewer-facing work queue. The policy engine remains` | The **entire** readiness surface: `.claim-readiness` (810), `-head` (818), `-state` (841), `--ready` (854), `-summary` (863), `-counts` (869), `-count` (875), `-label` (894), `-section-head` (905), `-local` (919), `-routes` (931), `-route` (938), `-route-id` (966), `-route-via/-count/-relation` (989–991), `-chevron` (1004), `-route-body` (1016), `-blockers` (1021), `-blocker` (1027), `-path` (1053), `-trace` (1072), `-map` (1076–1133), `-raw` (1136) |
| `1160–1174` | `@media (max-width: 860px)` | Readiness head goes `display: block`; `-route-via` is hidden |
| `1175–1182` | `@media (max-width: 520px)` | `.claim-readiness-blocker` collapses to one column; `-relation` unwraps. **This is where M7's stacked blocker row lands** |
| `1184–1201` | `/* The summary is the disclosure's only click target and reads as a mono` | First `.claim-links-summary` declaration (mono, `0.78rem`, `--muted`) plus its hover |
| `1203–1212` | `/* The separator and top spacing now belong to .claim-links (above); the` | `.claim-links > .claim-edges` reset |
| `1213–1249` | `/* Deep-link auto-open (decision C9). Landing on #<claim-id> — from the` | The CSS-only third auto-open signal. Relevant to R09.3: today a `:target` opens the footer regardless of blocked-ness |
| `1350–1382` | `/* ---- the sources row (components/sources.go) -------------------------` | `.claim-source-list` and the whole sources row block through `1555` |
| `1383–1399` | `/* The ref is the one thing on the row that must be findable by eye, because` | `.claim-source-ref` — the `[1]` / `[2]` marker this board sets in mono `12/16` |
| `1452–1493` | `/* The three-line clamp on a source note, and the control that lifts it.` | Source-note clamp; not drawn on this board (this claim's sources carry no note) |
| `1547–1555` | `/* Colour and background are NOT set here: `.pill.pw, .claim-review-pending`` | `.claim-review-pending` shape; colour lands at `3974–3978` |
| `1764–1773` | `@media (pointer: coarse)` | `.comment-chip { min-height: 44px }` — **the precedent M8 follows** |
| `3974–3978` | `.pill.pw, .claim-review-pending { background: var(--warn-bg); color: var(--warn); }` | The review_pending colour pair |
| `4020–4026` | `.claim-links { margin-top: 17px; border: 1px solid var(--border); border-radius: 8px; background: color-mix(in srgb, var(--paper) 55%, var(--card-bg)); overflow: hidden; }` | **The live footer card.** The board replaces this with an inset hairline inside the claim card — no second border, no second radius, no tinted box |
| `4028–4046` | `.claim-links-summary { … min-height: 56px; padding: 11px 13px; font: 650 10px/1.35 var(--font-mono); }` and its `::marker` kills + the `rgba(125,137,154,.07)` hover | The live summary row. The board replaces `650 10px mono` with Inter `13/16` at `400`/`500`/`600` |
| `4048–4063` | `.claim-footer__identity`, `__title`, `identity small`, `__counts`, `__counts > span`, `__counts strong`, `__chevron` | The live count pills: `5px 7px` padding, `1px var(--border)`, `border-radius: 5px`, `var(--card-bg)`, `var(--faint)` |
| `4533–4646` | `@media (max-width: 860px)` | `.claim-links-summary { align-items: flex-start }` (4638), `.claim-footer__identity small { display: none }` (4639), `.claim-footer__counts { flex-wrap: wrap; justify-content: flex-end }` (4640), count font-size `8.5px` (4641) |
| `4647–4658` | `@media (max-width: 560px)` | `.claim-links-summary` becomes a one-column grid with `9px` gap and `12px` padding; `.claim-footer__counts { justify-content: flex-start }` (4655); count padding `4px 6px` (4656); `.claim-footer__chevron { margin-left: auto }` (4657). **M3 must be written to win over this** |
| `4660` | `/* ---- print — THE @media print block ------------------------------------` | The single print block; last in the file |

`graph.css` has no range relevant to this screen. The only graph touchpoint is
the `See in claims graph` link, which is chrome in the claim card, not a graph
element.

### 7.2 Runtime JavaScript

| Address | What it is |
|---|---|
| `internal/render/viewer/template/system-record.js:122` | `function enhanceFooters()` — rewrites the server's `<summary>` digest into `claim-footer__identity` + `claim-footer__counts`, parsing `N links - N files - N sources - N drifted` with a regex. **This is the function that owns the footer vocabulary today** (§6) |
| `internal/render/viewer/template/system-record.js:137` | `counts.className = 'claim-footer__counts'` |
| `internal/render/viewer/template/system-record.js:146–149` | The count order: `relationship`, `source`, `file`, `drifted` (`:145` is the closing brace of the `count()` helper, not a call) |
| `internal/render/viewer/template/system-record.js:158` | `function enhanceFieldLabels()` — relabels the edge `<li>`s: `Governed By`, `Mirrors`, `Rests On`, `Depended On By`, `Migrated From`, `Implemented In`, `Review Pending`, `Sources` |
| `internal/render/viewer/template/system-record.js:485–486` | Both are called from the boot sequence |
| `internal/render/viewer/template/viewer-runtime.js:1516` | `function renderClaimReadiness(assessments)` — builds `<section class="claim-readiness">` per card, including the state word (`Ready` / `Review required` / `Approval required` / `Dependencies not ready`) at `1537–1542` and the blocker-count strip at `1552–1556` |
| `internal/render/viewer/template/viewer-runtime.js:1444` | `function readinessFactRow(item, rootID)` — one `<li class="claim-readiness-blocker">`; this is the row the board redesigns |
| `internal/render/viewer/template/viewer-runtime.js:1455` | `function readinessRoute(group, rootID, index)` — the per-module route group; its summary emits `shown via this dependency` at `1464`, **which R09.8 cuts** |
| `internal/render/viewer/template/viewer-runtime.js:1437` | `function readinessPathDetails(record)` — the per-row path disclosure the board replaces with an always-visible breadcrumb |
| `internal/render/viewer/template/viewer-runtime.js:1390` | `function readinessMermaidSource(rootID, records)` — builds the inline dependency map **R09.9 cuts** |
| `internal/render/viewer/template/viewer-runtime.js:1493` | `function readinessRawDiagnostics(assessment)` — the raw JSON envelope **R09.8 demotes** |
| `internal/render/viewer/template/viewer-runtime.js:1265` | The conformance panel's not-ready sweep (`.claim-conformance[data-implementation-ready="false"]`) |
| `internal/render/viewer/template/viewer-runtime.js:1641`, `:2141` | The two call sites of `renderClaimReadiness` |
| `internal/render/viewer/template/graph-ui.js:396` | `function openPane()` — the target of the board's `See in claims graph` link; there is **no** per-claim entry point today, only `openFromHashOnLoad` at `:3301` |
| `internal/render/viewer/template/shell.html:10` | `<meta name="dossierx-viewer-runtime" content="comments-sse">` — the only claim-footer-relevant line in the shell; the shell contributes no footer markup |
| `internal/render/viewer/template/build-order-ui.js` | References `claim-readiness` class names but owns no claim-card markup on this screen |

### 7.3 Go emitters

| Address | What it is |
|---|---|
| `internal/render/components/components.go:378` | `func EdgesHTMLWithLinks(c model.Claim, files []implink.ViewFile, dependedBy []string, targetStatuses map[string]TargetStatus) template.HTML` — **the whole footer** |
| `internal/render/components/components.go:357–377` | The doc comment quoted above: `// The whole footer ships inside a <details class="claim-links"> whose` |
| `internal/render/components/components.go:407`, `:431` | `governed_by: none` and the named-target `<li class="claim-governed">` |
| `internal/render/components/components.go:440` | `mirrors` row |
| `internal/render/components/components.go:446` | `<li class="claim-rests-on">rests_on:` — the **DEPENDS ON** direction |
| `internal/render/components/components.go:453` | `<li class="claim-depended-by">depended on by:` — the **DEPENDED ON BY** direction |
| `internal/render/components/components.go:458` | `migrated_from` row |
| `internal/render/components/components.go:473` | `writeSourcesRow` call site |
| `internal/render/components/components.go:480–484` | `reviewPending := c.Status == model.StatusLocked && c.ReviewPending` and the `<li class="claim-review-pending">` |
| `internal/render/components/components.go:486–501` | The drifted counter and the `implemented in:` rows |
| `internal/render/components/components.go:520–540` | The two server-written auto-open signals (drifted file, locked + review_pending) |
| `internal/render/components/components.go:543–567` | The `<summary>` digest string and its pluralisation |
| `internal/render/components/components.go:569–575` | `<details class="claim-links">…<ul class="claim-edges">` |
| `internal/render/components/components.go:659` | `func CommentChipHTML(c model.Claim) template.HTML` — the comment count that sits hard right (variants `--empty`, `--resolved`, `--open` at `663–672`) |
| `internal/render/components/sources.go:120` | `func writeSourcesRow(b *strings.Builder, c model.Claim)` — `<li class="claim-sources">sources:<ul class="claim-source-list">` |
| `internal/render/components/sources.go:129–131` | `<span class="claim-source-ref">[N]</span>` — the `[1]` / `[2]` marker |
| `internal/render/components/sources.go:162`, `:208` | `writeExternalSource` / `writeInternalSource` |
| `internal/render/conformance_view.go:7–22` | `const conformanceCSS` — the entire checks panel stylesheet, injected only when the viewer carries conformance results. `.claim-conformance` at `:9`, `-head` at `:13`, `-check` at `:16`, `-check-head` at `:19`, `-line` at `:20` |
| `internal/render/conformance_view.go:60` | `func viewerCSSWithConformance(base []byte, results map[string]conformance.Result) []byte` |
| `internal/render/depended_by_view.go:38` | `func buildDependedByLookup(cat *catalog.Catalog) map[string][]string` — the reverse index behind `DEPENDED ON BY` |
| `internal/render/depended_by_view.go:73` | `func buildTargetStatusLookup(cat *catalog.Catalog) map[string]components.TargetStatus` — the per-target lifecycle the relationship badge and dot read |
| `internal/render/depended_by_view.go:127` | `func attachEdgesOverride(...)` — binds all three lookups into the partials' `edges` template func |
| `internal/render/render.go:47` | The `//go:embed` line for `style.css`, `system-record.js`, `viewer-runtime.js`, `graph.css`, `graph-ui.js` |
| `internal/render/render.go:64–69` | The template file-name constants |
| `internal/render/components/components.go:67` | `"commentChip": CommentChipHTML` in the template func map |

### 7.4 viewer-tests that assert on these selectors today

| Address | What it pins |
|---|---|
| `viewer-tests/reveal_test.go:98` | `targetRule = ".claim:target .claim-links::details-content"` |
| `viewer-tests/reveal_test.go:99` | `printRule = "@media print { details.claim-links::details-content }"` |
| `viewer-tests/reveal_test.go:149`, `:178`, `:439` | `sec.querySelector('details.claim-links')` open-state assertions |
| `viewer-tests/reveal_test.go:191`, `:456` | Fatal if a fixture renders no `<details class="claim-links">` |
| `viewer-tests/reveal_test.go:260`, `:504` | `chromedp.WaitVisible("details.claim-links")` |
| `viewer-tests/claim_readiness_test.go:82–92` | Counts `.claim-readiness`, `.claim-readiness-blocker`, `.claim-readiness-route`, `.claim-readiness-map pre.mermaid`, `.claim-readiness-map svg` |
| `viewer-tests/claim_readiness_test.go:117`, `:322`, `:346` | `WaitVisible` on `.claim-readiness` inside a specific claim |
| `viewer-tests/claim_readiness_test.go:325`, `:328` | Asserts 14 `.claim-readiness-blocker` and 1 `.claim-readiness-route` |
| `viewer-tests/claim_readiness_test.go:333`, `:336` | Asserts the 12-node map cap and the `Showing 12 of 14` caption — **both R09.9 removals** |
| `viewer-tests/conformance_test.go:112–154` | `.claim-conformance`, `.claim-conformance-check[data-conformance-state=…]`, `.claim-conformance-line`, `data-implementation-ready` |
| `viewer-tests/source_note_clamp_test.go:242`, `:304`, `:466`, `:482`, `:561` | Force-opens every `details.claim-links` before measuring a source note |
| `viewer-tests/theme_parity_test.go:161` | `.claim-links-summary` is in the theme-parity selector list |
| `viewer-tests/theme_parity_test.go:204`, `:220` | Baseline markup for `<ul class="claim-edges">` and `<summary class="claim-links-summary">` |
| `viewer-tests/component_fit_test.go` | References `claim-readiness` (fit assertions) |
| `viewer-tests/claim_collapse_test.go:86`, `:94`, `:104` | The claim-**body** collapse suite — `.claim-collapse-toggle` / `.claim-collapse-content` / `.claim-collapse-chevron`. It contains **no** footer-disclosure assertion: the file has zero occurrences of `claim-links` or `claim-footer`. The footer disclosure is pinned by `reveal_test.go` above, not here. This suite still binds the redesign, because the claim card **body** sits inside this mechanism: `TestIndividualClaimCanCollapseAndExpand` (`:79`) requires the toggle to start `aria-expanded="true"`, the content to go `hidden` on click, and — at `:104` — the chevron to stay at the extreme right of `.k`, **after** `.comment-chip`, within `9px` of the head's right edge. `TestClaimDeepLinkRevealsCollapsedContent` (`:125`) and `TestFacetClaimsCanCollapseAndExpandTogether` (`:140`) pin the same three selectors |
| `viewer-tests/testdata/theme-parity/baseline-flat.html:4528–4537`, `5175–5191` | Frozen golden copies of the `.claim-footer__*` rules — **any change to `style.css:4053–4062` or `4640–4656` must regenerate these** |

---

## 8. States not on the boards

The boards draw one claim: LOCKED, blocked by 2 blockers in 1 module, 4
relationships, 2 sources, 1 matched check, 0 comments. Everything below is a
state the engine can render and the boards do not show. For each, the spec is
**derived** from the rules and the measured component set — a lane agent must not
have to guess.

1. **Readiness auto-open when blocked (R09.3).** State 1 draws a blocked claim
   with the strip *closed*. R09.3 says readiness is the one expansion that may
   auto-open, and only when blocked. **Spec:** a blocked claim renders with the
   readiness expansion open on first paint — i.e. state 2, not state 1 — and
   opening any other chip closes it (R09.2). State 1 is the resting form of the
   strip, which is what an *unblocked* claim shows.

2. **Readiness not blocked — `Ready`.** The engine has four state words
   (`viewer-runtime.js:1537–1542`): `Ready`, `Review required`, `Approval
   required`, `Dependencies not ready`. **Spec:** the footer's first chip keeps
   the noun-and-count contract. Ready takes the **closed neutral chip** form —
   `--color-muted` label, `--color-faint` chevron, **no dot** — because a dot is
   a status signal and "no problem" is not a status. `Review required` takes the
   **draft** hue (`--color-draft` dot and word, count `--color-muted`), not the
   blocked hue: R08.1's two-hue vocabulary puts "not settled" in amber.
   `Approval required` and `Dependencies not ready` both take the blocked hue,
   because both are the claim being unready for a named reason.

3. **`review_pending` trigger variants.** The engine emits one flat
   `<li class="claim-review-pending">review_pending</li>`
   (`components.go:483`) and colours it with `--warn` (`style.css:3974`). Per
   `dossierx-claims` there are three triggers. **Spec:** review_pending is a
   **readiness** state, not a fifth chip — it renders in the readiness chip as
   `Review required` with the draft hue (see 2), and the trigger itself is a
   blocker row inside the readiness expansion, using the same hairline-divided
   row form as a dependency blocker. It must **not** get its own strip item;
   R-F.1 fixes the strip at four.

4. **Zero-thread comment chip.** The board draws `0` in `--color-faint` with no
   ground. The engine has three variants (`components.go:663–672`):
   `--empty`, `--resolved`, `--open`. **Spec:** `--empty` is the board's form.
   `--open` takes `--color-accent-bg` behind an `--color-accent` glyph and label
   (R-F.3's control form). `--resolved` takes `--color-locked` for the glyph and
   a `--color-faint` count — resolution is an approval-shaped fact, so it borrows
   the lock hue, which R08.1 permits because no new hue enters.
   On **mobile** the pill takes `--color-accent-bg` whenever the count is
   non-zero, because there it is the control that opens the sheet.

5. **Zero counts on any of the four chips.** **Spec:** a chip with a count of
   zero still renders, closed, un-openable, at `--color-faint` rather than
   `--color-muted`, and does not carry a chevron. Removing it would make the
   strip's four positions unstable, and R-F.1 fixes the order — a reader who has
   learned "third position is sources" must not find checks there on the next
   claim. An entirely edgeless, sourceless, checkless claim still shows four
   zeros and the comment count.

6. **Empty expansion.** Cannot occur if 5 holds (a zero chip does not open). If
   an expansion is forced open by a deep link onto an empty set, it renders its
   eyebrow, its rule and a `0` count, and nothing else — **no empty-state
   illustration and no sentence.**

7. **Check verdicts other than `Matched`.** The engine has `matched`,
   `mismatch`, `uncheckable` (`conformance_view.go:17–18`,
   `viewer-tests/conformance_test.go:112`). **Spec:** the verdict dot + label
   takes `--color-locked` for `Matched`, `--color-blocked` for `Mismatch` and
   `--color-draft` for `Uncheckable` — two hues plus the lock green, no fourth
   hue (R08.1). The `FOUND` row's tick glyph becomes a cross in
   `--color-blocked` for a member that is missing; a member that is present but
   unexpected keeps the tick and takes `--color-draft`. The row geometry does not
   change.

8. **More than one check.** The board draws exactly one. **Spec:** multiple
   checks stack inside the single checks expansion, separated by the same
   `1px --color-border` hairline the field rows use, with the explainer stated
   **once** at the top of the panel, not per check. Four panels would violate
   R09.1 at the next altitude down.

9. **DRAFT and other lifecycle statuses on the claim itself.** The board draws
   LOCKED only. **Spec:** the status chip's ground, icon and label take
   `--color-draft-bg` / `--color-draft` for DRAFT (board 07a is its specimen) and
   the chip's geometry — `999px`, `4px/8px/10px`, `12px` icon, `11/14` weight
   600 tracking `+0.05em` — does not change. The padlock is **not** drawn for a
   draft.

10. **Long claim titles.** The board's title is one line at 1140px and two at
    390. **Spec:** the title wraps; it is never truncated and never ellipsised.
    The status chip is `flex-shrink: 0` and stays top-aligned
    (`align-items: start`, measured on `29W-0`), so a three-line title pushes the
    prose down and leaves the chip where it was.

11. **Long claim ids / long slugs.** The measured id is 66 characters and fits.
    **Spec:** the id wraps (`overflow-wrap: anywhere` is already on every Paper
    node and is the right behaviour); the mono slug chips inside a blocker row
    wrap as whole chips at desktop (`flex-wrap: wrap` on the chip row, measured)
    and stack at mobile (R-I.1). A slug is never truncated: a truncated slug is
    not an identifier.

12. **Long file paths in the checks `FILE` row.** **Spec:** the directory prefix
    wraps at any character; the filename stays whole and stays accent. On mobile
    the prefix already has a fixed `342px` and its own line, which is the
    arrangement that makes this survive.

13. **Cycles in the readiness chain.** R09.9 forbids an inline map, so a cycle
    has nowhere to be drawn here. **Spec:** a cycle appears as ordinary blocker
    rows whose breadcrumbs happen to close; the readiness expansion states the
    count and the modules as usual and says nothing about cycle-ness. The cycle
    is the **graph's** job — `--color-graph-cycle` exists for exactly that — and
    `See in claims graph` is the route to it.

14. **Blockers spread across more than one module.** The board draws `2 in 1
    module` and one route group. **Spec:** the phrase pluralises to
    `N in M modules`, and each module is one route row with its own count pill
    and its own indented block of hairline-divided blocker rows. Group 06 is the
    specimen for the four-module case; its measurements govern, not an
    extrapolation from this board.

15. **Governed-by absent (`governed_by: none`).** The engine emits a distinct
    `<li class="claim-governed governed-none">` and deliberately does **not**
    count it as a link (`components.go:390–407`). **Spec:** the `GOVERNED BY`
    direction header still renders inside the relationships expansion, with the
    word `none` in `--color-faint` Inter `14/18` where a title would be, no dot
    and no badge. The strip's `N relationships` count excludes it, matching the
    engine.

16. **Drifted implementation file.** The engine has `implemented in:` rows and a
    `drifted` pill, and drift is one of the two server-written auto-open signals.
    Neither appears in the board's strip. **Spec:** files are **not** a fifth
    chip. Drift is a readiness fact, so it becomes a blocker row inside the
    readiness expansion and promotes the readiness chip to blocked; the file path
    renders as a mono slug chip in the row's breadcrumb position. This preserves
    R-F.1's four and R09.3's auto-open-only-readiness.

17. **Hover, focus-visible, active, disabled on any chip.** Not drawn.
    **Spec:** hover on a chip takes the engine's `--hover-bg`
    (`rgba(125,137,154,.08)`, mode-invariant) behind the chip's own box —
    which means on desktop, where there is no box, hover paints a
    `--radius-pill` wash at the chip's inline padding. Focus-visible takes a
    `2px` `--color-accent` outline at `2px` offset. There is no active or
    disabled state: a chip with a zero count simply does not open (state 5), and
    nothing on this screen is ever disabled.

18. **Code evidence switched off for the project (R12.4 WATCH).** This is *not*
    the zero-count case of item 5, and the boards cannot show it: they draw a
    project whose code evidence is on. The engine already gates the surface
    rather than the count — the conformance CSS is injected only when the viewer
    carries conformance results (`internal/render/conformance_view.go:60`,
    `viewerCSSWithConformance`).
    **Spec:** when a project turns code evidence off, the `1 check` chip is
    **absent from the strip entirely**, and the strip renders three chips plus the
    comment count. This is the one permitted exception to the fixed four of
    R-F.1/§8 item 5, and it is permitted because the missing thing is the whole
    *feature*, not a count of zero: a `0 checks` chip would assert that nothing
    was found, which is a different and false claim from "this project does not
    publish code evidence". The three surviving chips keep their R-F.1 order
    (readiness, relationships, sources); nothing re-ranges. If a check exists but
    its adapter is not symbol-shaped (R12.4 LIMIT), the chip and the expansion
    both render and the `FILE` row is simply omitted from that check's field
    block — that *is* item 5's territory, not this one.

19. **Print.** `style.css` has exactly one `@media print` block, last in the file
    (`style.css:4660`), and print always uses the light palette. **Spec:** in
    print every expansion renders **open** — the reveal test already pins
    `@media print { details.claim-links::details-content }`
    (`reveal_test.go:99`) — and R09.2's one-at-a-time rule is a screen rule only.
    Print is the one context where all four are visible at once, because there is
    no reader to confuse and no way to open one.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

- **R-F.2's pill/border spec is mobile-only; the desktop chip is bare text plus a
  chevron.** R-F.2 states "All three are 30px tall with `--radius-pill` and 11px
  inline padding" without qualifying the width. Its evidence node (`7N2-0`,
  section I3) is a **mobile** section, and this board's desktop footer (`2A9-0`,
  `2B1-0`, `2CM-0`) measures no pill, no border and no ground at any of the five
  states — across two independently drawn boards, light and dark. **Decision: the
  30px pill is the ≤520px form; the desktop form is text + chevron.** Reason: two
  boards and five states agree, and R-F.2's own intent sentence ("at 12px on a
  phone, colour alone is too quiet") is explicitly about phones. `reference-rules.md`
  should gain the width qualifier; this file carries the qualification in the
  meantime.

- **The blocked chip does not turn accent when open.** Neither R-F.2 nor R09.2
  says what happens when the *blocked* variant is the open one. Measured
  (`2BK-0`, the blocked chip inside the state-2 strip `2B1-0`): it keeps
  `--color-blocked` and signals openness with the chevron
  flip plus one weight step on the count. **Decision: status colour outranks the
  open accent.** Reason: accent is navigation-only (`tokens.md` §1, board 01 node
  `2C-0`, "Never overlaps a status meaning"), so promoting a blocked chip to
  accent would spend the navigation colour on a status fact.

- **Openness is chevron + one weight step + a colour promotion, and the
  promotion target is the chip's own status colour.** Generalised from the two
  measured cases so that `Review required` (draft hue) has a defined open form
  without a board.

- **A zero-count chip renders, closed, faint, chevron-less.** Chosen over hiding
  it. Reason: R-F.1 fixes the order of four; an order that only sometimes has
  four positions is not an order a reader can learn.

- **`--container-reading` is 760px here, matching the board.** The measured
  `max-width: 760px` on `29W-0`/`61Y-0` agrees with `tokens.md`'s open decision
  and with R11.2. The `720px` token value is not used on this screen.

- **The 12px card radius is a per-component literal, not a new token.** It is off
  the `4 / 8 / 999` scale. Per `tokens.md`'s open decision on off-scale radii, it
  stays a literal in the claim-card rule.

- **`#FBF7F7` (the light blockers ground) and `#F6F8FA` (the light neutral
  expansion ground) get no token.** `#F6F8FA` is 3 units off `--color-code-bg`
  `#F3F5F8` in one channel and is recorded as a defect below; `#FBF7F7` is a
  ~4 % blocked tint over `--color-card` and is specified as
  `color-mix(in srgb, var(--warn) 4%, var(--card-bg))` rather than as a hex, so
  it tracks a themed `--warn`. Dark already does this correctly
  (`#EC8A8314` = `--color-dark-blocked` at 8 %).

- **The mobile boards specify no app bar, no header and no scroll chrome, by
  their own statement.** A lane agent takes all mobile chrome from group 02 and
  must not infer any from these boards.

- **Nothing on this screen becomes a bottom sheet.** Stated in §5; recorded here
  because it is a decision about R-J.1's scope rather than a measurement.

- **`docs/design/tokens.md` §8 no longer carries the same wrong media query this
  file carried; no edit was made there.** Its 390-mapping row H3 once said "the
  existing `max-width: 640px` and `max-width: 560px` rules already touch
  `.claim-footer__counts`". At `3ac8844` that is false: the `max-width: 640px`
  block (`style.css:2563`) contains only `.gcp-console`, `.gcp-row`, `.gcp-sev`
  and `.gcp-time`, and `grep -n 'max-width: *640px'` returns that one hit. When
  this file was repaired, `tokens.md` H3 had **already** been corrected upstream
  by the screen-14 lane (it now names `860px` / `4640–4641` and `560px` /
  `4655–4656`, and cites OD14.11). **Decision: leave `tokens.md` alone and match
  it**, rather than re-edit a shared foundation doc that another lane owns and is
  concurrently writing. `tokens.md`'s breakpoint inventory two rows above was
  always correct — it credits `560px` with `.claim-links-summary` and
  `.claim-footer__counts` and credits `640px` with nothing.

- **The new 520px block must be a new block placed after `style.css:4658`, not
  added to the existing `max-width: 520px` at `style.css:1175`.** Taken from
  `tokens.md` H3 and independently true of the file: `max-width: 860px` opens at
  `4533` and `max-width: 560px` at `4647`, both **after** `1175`, and all three
  are equal-specificity media queries, so source order decides. **Decision:
  record it here as a lane instruction** so an agent reading only this screen
  does not extend the 1175 block and silently lose.

- **`viewer-tests/claim_collapse_test.go` is not the footer suite and this file
  used to say it was.** At `3ac8844` the file has zero occurrences of
  `claim-links` or `claim-footer`; it pins the claim **body** collapse
  (`.claim-collapse-toggle` / `-content` / `-chevron`). **Decision: keep the row
  in §7.4 but relabel it**, rather than delete it. Reason: the redesigned card
  body sits inside that collapse mechanism, so the suite still constrains this
  screen — in particular its chevron-geometry assertion, which requires the
  collapse chevron to remain the right-most thing in the claim head, after the
  comment chip. A lane agent that reads the old label would look for footer
  assertions that are not there and would miss the body assertions that are.

### Paper defects

Approval is not proof. Each item below is a measured conflict between this
screen's boards and the tokens, the reference rules, or the boards' own notes.

1. **The footer hairline is a raw `#E8ECF1` on every light board and a raw
   `#212934` on every dark board.** `--color-border` is `#DDE2E9` and
   `--color-dark-border` is `#242C38`. Six occurrences on the light desktop board
   (`2A9-0`, `2XK-0`, `2H1-0`, `2I5-0`, `2QT-0`, and the mobile card's inner
   separator), matching counts on the other three. *Ship the tokens.*

2. **The light desktop card ground is the literal `#FFFFFF`** (`29W-0`) where the
   dark twin correctly reads `var(--color-dark-card)` (`61Y-0`). *Ship
   `--color-card`.*

3. **The light desktop LOCKED chip paints its ground `--color-locked-bg`
   correctly but its padlock stroke and label the literal `#2C6B52`** (`29W-0`);
   the dark twin uses `var(--color-dark-locked)` throughout. The mobile light
   board (`6KR-0`) gets this right with `var(--color-locked)`. *Ship the token.*

4. **The at-rest blocked chevron is `--color-faint` on light and
   `--color-dark-blocked` on dark.** `2A9-0` vs `61Y-0`, same element, same
   state. One of the two is wrong and the boards do not say which. *Resolution:
   the light board is right — a closed chip's chevron is the neutral
   `--color-faint`/`--color-dark-faint` in every other chip on both boards, and
   the blocked hue is what marks it open (`2JF-0`, the open blocked chip's
   chevron in `2BK-0`). The dark board's chevron is
   the defect.*

5. **The light expansion grounds `#F6F8FA` are 3 units off `--color-code-bg`
   `#F3F5F8`** across `2H1-0`, `2I5-0`, `2QT-0`, `6SQ-0`, `6XH-0`, `702-0`. The
   dark twins correctly use `--color-dark-code-bg` `#1B212B`. This is the same
   drift `tokens.md` disagreement 12 records for board 12's evidence panel.
   *Ship `--color-code-bg`.*

6. **Blocked tints are spelled as hexes with baked alpha throughout:**
   `#9E3B361A` (blocker count pill, target slug chip — light), `#9E3B3617`
   (mobile blocked chip), `#EC8A8314` / `#EC8A831F` / `#EC8A8321` / `#EC8A832E` /
   `#EC8A8338` (dark), `#2C6B521A` (mobile LOCKED chip), `#63BE9A21` (dark mobile
   LOCKED chip), `#EFF1F4` (slug and hop-pill grounds), `#EBD9D8` and `#EFE6E5`
   (the blockers panel's two red hairlines), `#E3E8EE` (the section rules).
   `--color-blocked-bg`, `--color-locked-bg`, `--color-paper` and `--color-border`
   exist for six of these. *Ship the tokens; the other three (`#EBD9D8`,
   `#EFE6E5`, and the dark `#EC8A832E`/`#EC8A8338` pair) have no token and become
   `color-mix` derivations of `--warn`.*

7. **The checks `FILE` directory prefix is `#8592A3`** (`2QT-0`), which
   `tokens.md` disagreement 4 already identifies as pre-token residue for
   `--color-faint` `#6E7C8E`. *Ship `--color-faint`.*

8. **Direction-header tracking disagrees between widths.** Desktop `GOVERNED BY`
   / `DEPENDS ON` / `DEPENDED ON BY` are tracked `+0.06em` (`2H1-0`); the mobile
   twins are tracked `+0.08em` (`6SQ-0`). Same words, same size, same weight, same
   role. Neither value has a token; `--tracking-label` is `0.08em`. *Resolution:
   `+0.08em` (`--tracking-label`) wins at both widths — it is the only one of the
   two that is a token, and R-H.0 says mobile changes arrangement, not type
   details, apart from the three weight steps the notes strip names. The desktop
   board's `+0.06em` is the defect.* The checks field labels have the identical
   split (`+0.06em` desktop, `+0.08em` mobile) and take the same resolution.

9. **The checks verdict label changes weight between widths** — `600` on desktop
   (`2QT-0`), `500` on mobile (`702-0`) — which is the **opposite direction** to
   the notes strip's rule that mobile carries type one step *heavier*. *The mobile
   board is the defect; `Matched` is weight 600 at both widths.*

10. **Two `DEPENDED ON BY` rows carry a `--color-blocked` dot beside a `LOCKED`
    badge** (`2H1-0`, rows 3 and 4). R-I.2 says the dot is "7px in the lifecycle
    colour" and the badge is "in the lifecycle colour" — the same colour twice.
    Here they disagree. *Resolution: R-I.2 binds. The dot and the badge are the
    same colour, and that colour is the target's lifecycle. If the board meant to
    encode the edge's readiness rather than the target's lifecycle, that is a
    second signal the rules do not name and it is not implemented.*

11. **The board draws no accent-tinted comment pill, so R-F.3's mobile control
    form is unspecified at this screen.** The claim has zero comments, so the
    passive form is all four boards can show. Not a contradiction, but a gap:
    the mobile accent variant must be taken from components section J2 (`B0C-0`),
    not inferred from here.

12. **The board's own numbered captions use font-weight `700`** (`29R-0`), which
    does not exist in the frozen type scale (`400 / 500 / 600`, no 700). This is
    board chrome and does not ship, but it is a value the board file carries that
    the token set forbids.

13. **Families are spelled two different ways within this one group.** The two
    desktop boards spell them `var(--font-sans)` / `var(--font-serif)` /
    `var(--font-mono)`; both mobile boards spell them as the literals
    `"Inter", system-ui, sans-serif`, `"Source Serif 4", system-ui, sans-serif`,
    `"IBM Plex Mono", system-ui, sans-serif`. `tokens.md` disagreement 12 records
    this pattern file-wide. *Ship the tokens.* Note also that the mobile literal
    falls back to `sans-serif` for both the serif and the mono stack, which would
    be wrong if it ever fired.
