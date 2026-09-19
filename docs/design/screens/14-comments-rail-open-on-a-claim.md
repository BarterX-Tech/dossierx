# 14 · Comments — rail open on a claim

Screen group 14. Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`.
Engine tree: `/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`,
branch `feat/viewer-design-revamp` at `3ac8844`.

Read this with `../tokens.md` (every token name and its engine mapping) and
`../reference-rules.md` (the rules cited by id below). Where this file names a
colour it names a Paper token; the engine token to actually write is the
right-hand column of `tokens.md` § 1. **Every number here was read with
`get_computed_styles` / `get_jsx` / `get_tokens`. Nothing was measured from a
screenshot.**

---

## 1. Boards

| Node | Name | Size | What it shows | Viewer states depicted |
|---|---|---|---|---|
| `3XK-0` | 14 · Comments — rail open on a claim | 1440 × 1024 | Desktop light. Three-column shell: 268px module sidebar, 812px claim column, 360px comments rail pinned right. | Module `Permission readiness`, 31 of 31 locked; facet tab `Contract 26` active; module-level blocked banner ("All 26 contract claims are blocked…"); claim 1 = LOCKED **and** Blocked, **selected** (3px accent left bar) with its rail open; claims 2 and 3 = LOCKED, Blocked, rail closed; comments rail open with 2 open threads (one carrying a nested reply, one `(edited)`), 4 resolved collapsed, live composer mounted. |
| `9MV-0` | 14 · … · DARK | 1440 × 1024 | Same composition, dark mode. | Identical states. Blocked banner takes `--color-dark-blocked-surface` rather than a tint (the dark rule in `tokens.md` § 1). |
| `A24-0` | 14 · Comments — bottom sheet on a claim · MOBILE LIGHT | 390 × 844 | Phone light. App bar (52px), page behind scrolled to claim 1's footer, scrim over the top 204px, comments **bottom sheet** at its 660px ceiling. | Same thread data. Footer strip in its two-row mobile form; comment count drawn as the accent-tinted pill that opened the sheet; sheet body clipped at the composer (scroll edge, per R-J.3). |
| `AH2-0` | 14 · … · MOBILE DARK | 390 × 844 | Phone dark, same sheet. | Identical, plus the dark-only 1px top hairline on the sheet (R-J.2). |
| `9M9-0` | Group 14 — band | 3860 × 31 | Group band: eyebrow `14 · COMMENTS — RAIL OPEN ON A CLAIM` (accent, Inter 13/16 w600, `+0.08em`) over a 2px `--color-accent` rule; caption "four designs of one screen · light pair left, dark pair right" (faint, 12/16). | — |
| `AOR-0` | Group 14 — notes | 2828 × 201 | Three note columns, 900px each, 64px gutter: `AOS-0` WHAT THIS SCREEN ANSWERS, `AOV-0` THE BOTTOM SHEET, AND THE COUNT THAT OPENS IT, `AOY-0` EVERY OTHER DIFFERENCE FROM DESKTOP (eyebrow in `--color-blocked`). | — |

Governing component board: `377-0` sections **H** (mobile forms, `57F-0`),
**I** (expansion forms, `7N2-0`), **J** (bottom sheets, `AYG-0` / specimen
`AYO-0`), and **G** (`4DC-0`, comment thread structure). Board `45K-0`
("14a · Comments — states and placement") is a *different* screen group and is
not specified here; it holds the empty / resolved / read-only variants and this
spec defers the pixel values for those to it while stating below what the rules
imply.

---

## 2. Design intent

### What the reviewer should perceive

From the notes strip `AOS-0`, verbatim in substance: the screen answers **one**
question — *what has already been said about this claim, and can I answer it
here.* Three consequences are stated as intent, not decoration:

1. **The rail opens beside the claim it belongs to and names that claim in its
   subtitle.** The header is two lines — `Comments` / `on Mechanism-scoped
   observation authority` — so the thread is never orphaned from its subject.
2. **The claim stays readable while the rail is open.** The claim column keeps
   its 760px measure (`41B-0` `max-width: 760px`); the rail takes the width
   that was gutter. "The reviewer never loses the sentence a comment is arguing
   about."
3. **Resolved threads stay collapsed behind one count**, because "a resolved
   thread is history, not work." One row, `4 resolved`, chevron closed.

`AOS-0` also fixes the **entry point**: "the comment count in a claim's footer
strip, at any width. On desktop it opens the rail; on mobile the same count
opens the sheet." There is no second door.

### The rules that bind this screen

- **R10.1 / R10.3 — elapsed time, one unit.** Every timestamp on the boards is
  elapsed and single-unit: `3 days ago` (`44A-0`), `2 days ago` (`44L-0`),
  `22 hours ago` (`44X-0`). No absolute stamp appears anywhere on the four
  boards. R-J.7 restates this for the thread body explicitly. The exact build
  time belongs on `title=` hover only.
- **R-F.1 — four chips in a fixed order, then the comment count hard right.**
  Desktop `41X-0` and dark `9R9-0` both run `Blocked 2 blockers` · divider ·
  `4 relationships` · `2 sources` · `1 check` · spacer · count. Mobile `A3J-0`
  keeps that order across two rows. Order is never re-sorted by this screen.
- **R-F.3 — the comment count is one pill, hard right at every width.** This
  screen is where the pill earns the accent: see the **defect note** below.
- **R-F.4 — mobile footer is two rows and a pill never splits.** `A3J-0` row
  one = blocked chip left, count right; row two = the three detail chips.
- **R-J.1 / R-J.2 / R-J.3 / R-J.4 / R-J.5 / R-J.6 / R-J.7 — the bottom sheet.**
  The mobile board is the comment-thread **body** in the one shell: same scrim,
  same 16px top corners, same 36 × 4 grabber, same header slot set, same pinned
  composer, sized `min 240 / max 660 / fit-content`. `AOV-0` says this in the
  designer's own words: the sheet is "the same object as '02 · Mobile nav — on
  this facet'".
- **R-H.0 — only three components change shape below the phone breakpoint, and
  a screen must use the form that matches its width.** The three are the blocked
  notice (R-H.1), the claim header (R-H.2) and the footer strip (R-H.3). On
  these boards R-H.0 binds exactly two things: the **mobile footer strip**
  (§ 4.11 / M12–M14), which takes its section-H two-row form, and the
  **mobile claim head** (§ 4.12, `A35-0` / `A3B-0` / `A3C-0`), which takes
  R-H.2's. It does **not** reach the sheet's interior — see the reconciliation
  under § 5.
- **R09.2 / R09.8** — the rail does not open any expansion and does not restate
  claim metadata; the claim's own strip stays one line at rest behind it.
- **R00.0 — the components board wins.** Where a value here differs from
  section G / J it is flagged, not copied.

### Every note on the band and the notes strip, paraphrased with its intent

**Band `9M9-0`.** "four designs of one screen · light pair left, dark pair
right" — the four boards are one screen in four renderings, not four designs.
Nothing is allowed to differ between them except mode and width.

**`AOS-0` — WHAT THIS SCREEN ANSWERS.** Covered above. The load-bearing clause
for implementation is *"keeps the claim readable at the same time"*: the rail
must not reflow or narrow the prose measure.

**`AOV-0` — THE BOTTOM SHEET, AND THE COUNT THAT OPENS IT.**
- The sheet is the *existing* shell, not a new one; a second shell would be
  wrong (R-J.1).
- Height is **content-driven**: a floor of ~240px, a ceiling of ~660px, scroll
  inside beyond it. This thread overruns, "so the sheet sits at 660px and the
  body clips at the composer — that is the scroll edge, not a crop."
- The sheet carries **everything the rail carries**: header with claim subtitle
  and close, both open threads with author role, relative timestamp, the
  `(edited)` marker, the nested reply and its rule, Resolve and Reply, the
  `4 resolved` disclosure, and the composer with its caption.
- It leaves behind **only** the two 13px hover glyphs (edit, delete) on the
  reader's own comment: "there is no hover on a phone and a 13px target is not
  a control." This is a deliberate capability difference, not an oversight.
- The footer count "is what opens it, so it is drawn as the accent-tinted pill
  the desktop footer already gives it, still hard right, still the last item in
  the canonical order."

**`AOY-0` — EVERY OTHER DIFFERENCE FROM DESKTOP.** The note is an exhaustive
diff and is treated here as normative:
- *Dropped:* the 268px sidebar (folded into the app-bar hamburger, settled in
  group 02); the rail's left border and its `-8px` shadow, replaced by the
  sheet's corners and scrim.
- *No annotation was dropped* — "the desktop board carries no commentary prose,
  only product UI, so there was none to leave behind."
- *Added:* scrim, grabber, 16px corners, a **1px top hairline on dark only**,
  and a **mono count in the sheet header** that the desktop rail does not show
  — "it is how the established sheet header is built" (R-J.4: the count is a
  slot of the shared header, so the sheet gets it and the rail does not).
- *Restructured:* the footer strip runs as two lines — "which is the group 02
  mobile footer unchanged."
- *Sizes:* comment bodies 13/21 → **14/22**; placeholder 13 → **14**; the rail's
  22px gutter → the **16px** mobile gutter; Resolve pill padding 4/10 → **5/11**
  and the Comment button 7/16 → **9/16** for tap targets; Resolve, Reply and
  `4 resolved` step **w500 → w600** per the mobile chip rule; the nested reply
  **drops its 14px indent and keeps its 2px rule**; thread spacing 10/12/4
  margins → **2/6/2 padding**.
- *Vocabulary:* unchanged everywhere, footer order included.
- *Dark:* "AGENT has no dark token — its `#4257C4` would sit at 2.3:1 on the
  card, so both dark boards use a lightened `#8E9BF0` and it is flagged as a
  token candidate, not settled."

### Where the boards contradict their own notes or the rules — Paper defects

Approval is not proof. These are recorded, not copied.

- **D14.1 — `AOY-0` says the dark boards use "a lightened `#8E9BF0`", and they
  do it through `--color-dark-graph-facet-1`, which `tokens.md` Disagreement 1–2
  rules unusable.** Nodes `ANL-0`, `AMY-0` (mobile dark) and the desktop dark
  rail's two AGENT labels resolve to `#8E9BF0`; `graph.css`'s live dark facet-1
  is `#7C8CE8`. The note itself says this is "a token candidate, not settled."
  *Implement `graph.css`'s dark facet-1, not the Paper token.*
- **D14.2 — the desktop light board spells the claim column and footer with raw
  hexes where tokens exist.** `3XL-0` sidebar `#EFF1F4` / border `#DDE2E9`;
  `41B-0` card `#FFFFFF` / border `#DDE2E9`; `41C-0` banner ground `#9E3B360F`;
  `41X-0` and `42X-0` top rule `#E8ECF1`, `Blocked` `#9E3B36`, chip text
  `#54606F`, chevrons, the passive count **and the selected count pill's speech
  bubble stroke** (`42K-0`, inside the otherwise token-clean pill `42I-0`)
  `#6E7C8E`, divider `#DDE2E9`;
  `42M-0` / `43M-0` claim separators `#DDE2E9`. The **rail** (`43Y-0`) and the
  whole dark board are token-clean. *Implement the token* (`tokens.md`
  Disagreement 12).
- **D14.3 — the dark footer's top rule is `#212934` (node `9R9-0`), which is
  not `--color-dark-border` `#242C38` and has no token.** A near-miss literal on
  an otherwise token-clean dark board. *Use `--color-dark-border`.*
- **D14.4 — the desktop rail title carries no tracking; the sheet title does.**
  `441-0` is Inter 16/22 w600 with no `letter-spacing`; `ACK-0` is the same size
  with `-0.01em`. R-J.4 fixes the sheet header title at 16/22 w600 `-0.01em`.
  *Both should carry `-0.01em`; the rail node is the outlier.*
- **D14.5 — the desktop rail's font families are spelled `var(--font-sans)` /
  `var(--font-mono)` while the mobile sheet spells the literal `"Inter",
  system-ui, sans-serif` / `"IBM Plex Mono", system-ui, sans-serif`.** Same
  component, two spellings (`tokens.md` Disagreement 12). Cosmetic in Paper,
  but it is why a lane agent must read this file and not the board.
- **D14.6 — the scrim is not the engine `--scrim`.** `ABU-0` is `#10172057`
  (ink at 34 %); `AJ6-0` is `#00000085` (black at 52 %). The engine ships
  `--scrim: rgba(0,0,0,.22)` light / `rgba(0,0,0,.42)` dark. R-J.2 names the
  engine token as the shell's scrim. *Use `--scrim`; the board's alphas are a
  drawing convenience over a 204px crop, not a specification.*
- **D14.7 — the desktop board shows a comment-count state R-F.3 does not
  name.** R-F.3 says the count is "a passive count" on desktop with a
  `--color-faint` bubble, and the accent-tinted pill is the *mobile* form.
  This board draws **two** desktop forms: the claim whose rail is open takes the
  accent-tinted pill (`41X-0`), the claim whose rail is closed takes the passive
  faint form (`42X-0`). That is a *selected* state, and it is right — it is the
  only thing on the desktop board that says which claim the rail belongs to,
  alongside the 3px accent left bar. Recorded as an **extension** of R-F.3, not
  a contradiction: one pill, one variant, plus a selected state that exists only
  while a rail is open. Flagged to the components board (section J2, `B0C-0`)
  as needing the third row.
- **D14.8 — `AOV-0` says the sheet "leaves behind only the two 13px hover
  glyphs", but the desktop board draws those glyphs only on the HUMAN thread
  (`44C-0`, `44E-0`) and not on the AGENT thread (`44V-0`).** That is correct
  behaviour — they are the reader's own-comment controls — but the note's word
  "the reader's own comment" is the rule and the board is the only place it is
  stated. Recorded so the lane does not read the asymmetry as a drawing slip.

---

## 3. Layout

### 1440 — desktop (`3XK-0` light, `9MV-0` dark)

| Region | Value | Node |
|---|---|---|
| Page frame | `1440 × 1024`, `display:flex`, `overflow:clip`, ground `--color-paper` / `--color-dark-paper` | `3XK-0` / `9MV-0` |
| Module sidebar | `width: 268px`, `flex-shrink: 0`, full height, `border-right: 1px` | `3XL-0` / `9MW-0` |
| Sidebar search block | `padding: 0 20px 16px` | `3XP-0` |
| Sidebar scroll region | `flex: 1 0 0`, `min-height: 0`, `overflow: clip`, `padding-inline: 12px`, `gap: 4px` | `3XV-0` |
| Sidebar footer (graph / build order / theme) | `padding: 16px 20px 20px`, `gap: 14px`, `border-top: 1px` | `3ZW-0` |
| Claim column | `width: 812px`, `flex-shrink: 0`, `overflow: clip` | `40K-0` / `9PW-0` |
| Claim column header | `padding: 36px 48px 0`, `gap: 20px`, `align-items: center` | `40M-0` / `9PY-0` |
| Header inner row | `width: 760px`, `justify-content: space-between`, `align-items: end` | `40N-0` / `9PZ-0` |
| Claim scroll region | `flex: 1 0 0`, `min-height: 0`, `justify-content: center`, `padding: 24px 48px 0`, `overflow: clip` | `41A-0` / `9QM-0` |
| **Reading measure** | `width: 100%`, `max-width: 760px`; top corners `12px`, bottom corners `0`; `1px` border | `41B-0` / `9QN-0` |
| Module blocked banner | `padding: 13px 32px`, `gap: 12px`, `border-bottom: 1px` | `41C-0` / `9QO-0` |
| Selected claim block | `padding: 34px 32px 28px`, `gap: 16px`, **`border-left: 3px solid --color-accent`** (dark: `--color-dark-accent`) | `41I-0` / `9QU-0` |
| Unselected claim block | `padding: 30px 32px 28px`, `gap: 16px`, `border-top: 1px`, **no left bar** | `42M-0`, `43M-0` / `9RY-0` |
| Claim head row (title + status chip) | `gap: 16px`, `align-items: start` | `41J-0`, `42N-0` |
| **Comments rail** | **`width: 360px`**, `flex-shrink: 0`, column, `border-left: 1px --color-border-strong` (dark `--color-dark-border-strong`), `box-shadow: -8px 0 24px #10172014` (dark `#00000047`), ground `--color-card` / `--color-dark-card` | `43Y-0` / `9TA-0` |
| Rail header | `padding: 20px 22px 16px`, `gap: 12px`, `align-items: start`, `border-bottom: 1px --color-border` | `43Z-0` |
| Rail body (scroll region) | `flex: 1 0 0`, `padding-inline: 22px`, `padding-top: 4px` | `446-0` |
| Rail composer (pinned) | `padding: 16px 22px 20px`, `gap: 10px`, `border-top: 1px --color-border`, ground `--color-paper` / `--color-dark-paper` | `45B-0` |

Column arithmetic: `268 + 812 + 360 = 1440`. The rail is **not**
`--container-rail` (268px) — that token is the *left* module sidebar's width on
this board. The reading measure is `760px`, matching `tokens.md` § Open
decisions ("use 760px"), inside a 812px column with `48px` gutters.

Sticky / scroll: the sidebar scroll region (`3XV-0`), the claim scroll region
(`41A-0`) and the rail body (`446-0`) each scroll independently; the claim
column header (`40M-0`), the rail header (`43Z-0`) and the rail composer
(`45B-0`) are `flex-shrink: 0` and never scroll.

### 390 — mobile (`A24-0` light, `AH2-0` dark)

| Region | Value | Node |
|---|---|---|
| Page frame | `390 × 844`, `flex-direction: column`, `justify-content: end`, ground `--color-paper` / `--color-dark-paper` | `A24-0` / `AH2-0` |
| Page behind | absolute `0,0`, `390 × 844`, `overflow: clip` | `A2G-0` / `AH3-0` |
| App bar | `height: 52px`, `padding-inline: 16px`, `gap: 12px`, `flex-shrink: 0` | `A2H-0` / `AH4-0` |
| Scroll viewport | `390 × 792`, `overflow: clip`, `position: relative` | `A2R-0` / `AHD-0` |
| Scrim | absolute `0,0`, `390 × 204` (= the strip of page not covered by the sheet), `#10172057` light / `#00000085` dark — **see D14.6, ship `--scrim`** | `ABU-0` / `AJ6-0` |
| **Sheet** | `width: 390px` (viewport), `height: fit-content`, `min-height: 240px`, `max-height: 660px`, top corners `16px`, `overflow: clip`, ground `--color-card` / `--color-dark-card`; **dark only** `border-top: 1px --color-dark-border` | `AC5-0` / `AJQ-0` |
| Grabber rail | `padding-top: 8px`, centred | `AC6-0` / `AJR-0` |
| Grabber | `36 × 4`, `border-radius: 999px`, `--color-border-strong` | `AC7-0` |
| Sheet header | `padding: 14px 16px 12px`, `gap: 10px`, `align-items: start`, `border-bottom: 1px --color-border` | `ACI-0` / `AKP-0` |
| Sheet body (scroll region) | `flex: 1 0 0`, `min-height: 0`, `overflow: clip`, `padding-inline: 16px`, `padding-top: 4px` | `ACT-0` (456px) / `ALJ-0` (455px) |
| Composer (pinned) | `padding: 14px 16px 18px`, `gap: 10px`, `border-top: 1px --color-border`, ground `--color-paper` / `--color-dark-paper` | `AE3-0` / `AO2-0` |
| Mobile gutter | `16px` everywhere (app bar, sheet header, body, composer, claim card margins) | `A2H-0`, `ACI-0`, `ACT-0`, `AE3-0`, `A2T-0` |

Sheet height on these boards is `660px` — the ceiling, reached because this
thread overruns it (R-J.3). `844 − 660 = 184`; the scrim is drawn `204px` tall
because it also covers the 20px of the sheet's rounded shoulder. Sheet regions:
grabber `12` + header `71` + body `456` + composer `121` = `660`.

---

## 4. Components on this screen

**Measured values recorded in this section and § 3: 141** (desktop light 64,
desktop dark 41, mobile light 28, mobile dark 8). The brief's floor for a
desktop board is 40; the desktop light board alone carries 64. The fix pass
added seven more nodes, all re-read from Paper: desktop-dark `9TK-0` (`HUMAN`),
`9TU-0` and `9U5-0` (`AGENT`), and the selected count pill `42I-0` with its
`42J-0` / `42K-0` / `42L-0` interior.

Colour is given as `Paper token (hex) → engine token`. Hexes are from
`get_tokens` on this file.

### 4.1 Comments rail — shell (desktop)

| Property | Light | Dark | Node |
|---|---|---|---|
| Width | `360px` (fixed, `flex-shrink: 0`) | same | `43Y-0` / `9TA-0` |
| Ground | `--color-card` `#FFFFFF` → `--card-bg` | `--color-dark-card` `#161B22` | `43Y-0` / `9TA-0` |
| Left edge | `1px solid --color-border-strong` `#C2CAD5` → `--border-strong` | `--color-dark-border-strong` `#38424F` (**engine has no dark `--border-strong`** — `tokens.md` Disagreement 9) | `43Y-0` / `9TA-0` |
| Shadow | `-8px 0 24px #10172014` (ink at 8 %) → `--shadow-cast` | `-8px 0 24px #00000047` (black at 28 %) → `--shadow` dark | `43Y-0` / `9TA-0` |
| Radius | none (full-bleed to the viewport edges) | same | `43Y-0` |

### 4.2 Rail header

| Element | Value | Light token | Dark token | Node |
|---|---|---|---|---|
| Container | `padding: 20px 22px 16px`, `gap: 12px`, `align-items: start`, `border-bottom: 1px` | `--color-border` `#DDE2E9` | `--color-dark-border` `#242C38` | `43Z-0` |
| Text block | column, `gap: 4px`, `flex: 1 0 0` | — | — | `440-0` |
| Title `Comments` | Inter **16 / 22**, w600, tracking **none** (see D14.4) | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `441-0` |
| Subtitle `on Mechanism-scoped observation authority` | Inter **12 / 18**, w400 | `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `442-0` |
| Close glyph | `17 × 17`, `viewBox 0 0 24 24`, `stroke-width 2.2`, `stroke-linecap round`, `flex-shrink: 0` | stroke `--color-muted` | stroke `--color-dark-muted` | `443-0` |
| Mono count | **absent on desktop** (present on mobile only — R-J.4 slot) | — | — | — |

### 4.3 Thread block (rail)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Thread container | `padding-block: 16px`, `gap: 8px`, `border-bottom: 1px` | `--color-border` | `--color-dark-border` | `447-0`, `44U-0` |
| Meta row | `align-items: center`, `gap: 9px` | — | — | `448-0`, `44V-0` |
| `HUMAN` label | IBM Plex Mono **11 / 14**, w600, tracking `+0.04em` | `--color-accent` `#1C4E8C` → `--link` | `--color-dark-accent` `#6AA6E8` | light `449-0` / desktop dark `9TK-0` |
| `AGENT` label | IBM Plex Mono 11 / 14, w600, `+0.04em` | `--color-graph-facet-1` `#4257C4` | Paper `--color-dark-graph-facet-1` `#8E9BF0` — **ship `graph.css` `#7C8CE8`** (D14.1) | light `44K-0`, `44W-0` / desktop dark `9TU-0`, `9U5-0` / mobile dark `ANL-0`, `AMY-0` |
| Elapsed time | IBM Plex Mono 11 / 14, w400 | `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `44A-0`, `44L-0`, `44X-0` |
| `(edited)` | IBM Plex Mono 11 / 14, w400, **italic** | `--color-faint` | `--color-dark-faint` | `44Y-0` |
| Edit glyph (hover, own comment) | `13 × 13`, `stroke-width 2`, no linecap | stroke `--color-faint` | stroke `--color-dark-faint` | `44C-0` |
| Delete glyph (hover, own comment) | `13 × 13`, `stroke-width 2.2`, linecap `round` | stroke `--color-faint` | stroke `--color-dark-faint` | `44E-0` |
| Body | Inter **13 / 21**, w400 | `--color-ink` | `--color-dark-ink` | `44G-0`, `44M-0`, `44Z-0` |
| Reply block | `margin: 10px 0 0 14px`, `padding-left: 12px`, `gap: 7px`, `border-left: 2px` | `--color-border` | `--color-dark-border` | `44I-0` |
| Actions row (thread 1) | `margin-top: 12px`, `gap: 10px` | — | — | `44N-0` |
| Actions row (thread 2) | `margin-top: 4px`, `gap: 10px` | — | — | `450-0` |

The `HUMAN` / `AGENT` node ids in the two label rows are split three ways on
purpose. This file's usual "light / dark" slash convention would otherwise read
`ANL-0` and `AMY-0` as *desktop* dark, and they are not — they are on the
**mobile dark** board `AH2-0`; the desktop dark rail's own labels are `9TK-0`
(`HUMAN`) and `9TU-0` / `9U5-0` (`AGENT`). The colour is the same on all of them
(`var(--color-dark-accent)` for `HUMAN`, `var(--color-dark-graph-facet-1)` for
`AGENT`), but the `font-family` spelling differs by board: `9TK-0` / `9TU-0` /
`9U5-0` read `var(--font-mono)`, `ANL-0` / `AMY-0` read the literal
`"IBM Plex Mono", system-ui, sans-serif`. That is D14.5, and it is the reason a
lane agent must take the token from this file rather than from the board.

### 4.4 Resolve pill / Reply / resolved disclosure (rail)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Resolve pill | `padding: 4px 10px`, `gap: 6px`, `border-radius: 999px` (`--radius-pill`), `border: 1px` | border `--color-border-strong` | `--color-dark-border-strong` | `44O-0`, `451-0` |
| Resolve tick | `12 × 12`, `stroke-width 2.6`, linecap `round` | stroke `--color-locked` `#2C6B52` → `--accent` | `--color-dark-locked` `#63BE9A` | `44P-0`, `452-0` |
| Resolve label | Inter **12 / 16**, **w500** | `--color-muted` | `--color-dark-muted` | `44R-0`, `454-0` |
| `Reply` | Inter 12 / 16, **w500**, bare (no pill, no border) | `--color-accent` | `--color-dark-accent` | `44S-0`, `455-0` |
| Resolved disclosure row | `padding-block: 14px`, `gap: 8px`, `border-bottom: 1px` | `--color-border` | `--color-dark-border` | `456-0` |
| Disclosure chevron (collapsed) | `13 × 13`, **right-pointing** `m9 18 6-6-6-6`, `stroke-width 2.4`, linecap `round` | stroke `--color-muted` | `--color-dark-muted` | `457-0` |
| `4 resolved` | Inter 12 / 16, **w500** | `--color-muted` | `--color-dark-muted` | `459-0` |

### 4.5 Composer (rail)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Strip | `padding: 16px 22px 20px`, `gap: 10px`, `border-top: 1px`, ground `--color-paper` `#EFF1F4` | border `--color-border` | ground `--color-dark-paper` `#0D1117`, border `--color-dark-border` | `45B-0` |
| Field | `padding: 11px 13px`, `border-radius: 8px` (`--radius-md`), `border: 1px`, ground `--color-card` | border `--color-border-strong` | ground `--color-dark-card`, border `--color-dark-border-strong` | `45C-0` |
| Placeholder `Add a comment…` | Inter **13 / 20**, w400 | `--color-faint` | `--color-dark-faint` | `45D-0` |
| Caption `Saved to the served viewer, not to this file.` | Inter **11 / 16**, w400, `width: 170px`, `flex-shrink: 0`, left of the button | `--color-faint` | `--color-dark-faint` | `45F-0` |
| Caption row | `align-items: center`, `gap: 10px`, flex spacer between caption and button | — | — | `45E-0` |
| Submit button | `padding: 7px 16px`, **`border-radius: 6px`** (off-scale, `tokens.md` § 5), `flex-shrink: 0`, ground `--color-accent` | — | ground `--color-dark-accent` | `45H-0` |
| Submit label `Comment` | Inter **13 / 16**, w600, `width: max-content` | `#FFFFFF` (literal, no token) | `--color-dark-paper` `#0D1117` — dark **inverts to the page ground**, which is correct on a light-blue accent | `45I-0` |

### 4.6 Claim footer strip — the control that opens the rail (desktop)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Strip | `margin-top: 8px`, `padding-top: 16px`, `gap: 20px`, `align-items: center`, `border-top: 1px` | `#E8ECF1` literal → use `--color-border` (D14.2) | `#212934` literal → use `--color-dark-border` (D14.3) | `41X-0` / `9R9-0` |
| Blocked group | `gap: 6px` | — | — | `41X-0` |
| Blocked dot | `7 × 7`, `border-radius: 999px` | `#9E3B36` → `--color-blocked` | `--color-dark-blocked` `#EC8A83` | `41X-0` |
| `Blocked` | Inter **13 / 16**, w600 | `--color-blocked` | `--color-dark-blocked` | `41X-0` |
| `2 blockers` | Inter 13 / 16, w400 | `--color-muted` | `--color-dark-muted` | `41X-0` |
| Chevron (all chips) | `12 × 12`, `stroke-width 2.4`, linecap `round`, down `m6 9 6 6 6-6` | stroke `--color-faint` | `--color-dark-faint` | `41X-0` |
| Divider after the blocked group | `1 × 12` rule | `--color-border` | `--color-dark-border` | `41X-0` |
| Detail chips (`4 relationships`, `2 sources`, `1 check`) | Inter 13 / 16, w400, `gap: 5px`, **no pill, no border** on desktop | `--color-muted` | `--color-dark-muted` | `41X-0` |
| Empty-state chip (`No checks declared`) | Inter 13 / 16, w400 | `--color-faint` (dimmer than a counted chip) | `--color-dark-faint` | `42X-0` |
| **Comment count — selected** (this claim's rail is open) | pill `42I-0`, `padding: 3px 8px`, `border-radius: 999px`, ground `var(--color-accent-bg)` `rgb(28 78 140 / 9%)`; icon `14 × 14` speech bubble (`42J-0`), `viewBox 0 0 24 24`, `fill none`, `stroke-width 2`, `stroke-linecap round`, stroke **literal `#6E7C8E`** on the board (`42K-0`) → use `--color-faint` (D14.2); count `42L-0` Inter **13 / 16 w600** `var(--color-accent)` | as stated | ground `--color-dark-accent-bg` `rgb(106 166 232 / 14%)`, icon stroke `--color-dark-faint`, count `--color-dark-accent` | `41X-0` → `42I-0` / `9R9-0` |
| **Comment count — passive** (rail closed) | **no pill**, `gap: 6px`; same `14 × 14` icon, stroke `--color-faint`; count Inter **13 / 16 w400** `--color-faint` | as stated | `--color-dark-faint` | `42X-0` |

### 4.7 Selected-claim marker (desktop)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Left bar on the claim whose rail is open | `border-left: 3px solid` | `--color-accent` `#1C4E8C` → `--link` | `--color-dark-accent` `#6AA6E8` | `41I-0` / `9QU-0` |
| Claim block padding, selected | `34px 32px 28px` (the 3px bar eats 2px of a nominal 36px top) | — | — | `41I-0` |
| Claim block padding, unselected | `30px 32px 28px`, `border-top: 1px` separator | `#DDE2E9` literal → `--color-border` | `--color-dark-border` | `42M-0`, `43M-0` / `9RY-0` |

### 4.8 Bottom sheet — shell (mobile)

| Element | Value | Light | Dark | Node |
|---|---|---|---|---|
| Shell | `width: 390px`, `height: fit-content`, `min-height: 240px`, `max-height: 660px`, `overflow: clip`, top corners `16px` | ground `--color-card` | ground `--color-dark-card` **+ `border-top: 1px --color-dark-border`** | `AC5-0` / `AJQ-0` |
| Scrim | full page, `#10172057` / `#00000085` — **ship `--scrim`** (D14.6) | — | — | `ABU-0` / `AJ6-0` |
| Grabber | `36 × 4`, `border-radius: 999px`, under `padding-top: 8px`, centred | `--color-border-strong` | `--color-dark-border-strong` | `AC7-0` / `AJR-0` |
| Header | `padding: 14px 16px 12px`, `gap: 10px`, `border-bottom: 1px` | `--color-border` | `--color-dark-border` | `ACI-0` / `AKP-0` |
| Header title | Inter **16 / 22**, w600, tracking **`-0.01em`** | `--color-ink` | `--color-dark-ink` | `ACK-0` |
| Header second line | Inter **12 / 18**, w400 | `--color-muted` | `--color-dark-muted` | `ACL-0` |
| Header mono count `2` | IBM Plex Mono **12 / 16**, w400, `flex-shrink: 0`, right of the text block | `--color-faint` | `--color-dark-faint` | `ACM-0` |
| Header close glyph | `20 × 20`, `stroke-width 2`, linecap `round`, hard right | `--color-muted` | `--color-dark-muted` | `ACN-0` |

### 4.9 Thread block (sheet) — the deltas from § 4.3

| Element | Value | Node |
|---|---|---|
| Body | Inter **14 / 22** (rail is 13/21) | inside `AD1-0`, `ADK-0` |
| Reply block | `padding-left: 12px`, `padding-top: 2px`, `border-left: 2px --color-border` — **no `margin-left: 14px`** (the indent is dropped, the rule is kept) | `AD7-0` |
| Actions row, thread 1 | `padding-top: 6px`, `gap: 10px` (rail: `margin-top: 12px`) | `ADD-0` |
| Actions row, thread 2 | `padding-top: 2px`, `gap: 10px` (rail: `margin-top: 4px`) | `ADR-0` |
| Resolve pill | `padding: 5px 11px` (rail: `4px 10px`), same `999px` radius, same `1px --color-border-strong` | inside `ADD-0` |
| Resolve label / `Reply` / `4 resolved` | Inter 12 / 16 **w600** (rail: w500) | inside `ADD-0`, `ADR-0`, `ADY-0` |
| Resolve tick, chevrons | unchanged: `12 × 12` / `13 × 13`, same strokes | — |
| Hover glyphs (edit, delete) | **absent** — the only content difference from the rail | — |

### 4.10 Composer (sheet) — the deltas from § 4.5

| Element | Value | Node |
|---|---|---|
| Strip | `padding: 14px 16px 18px` (rail: `16px 22px 20px`) | `AE3-0` / `AO2-0` |
| Field | `padding: 11px 13px`, `border-radius: 8px`, `1px --color-border-strong` — unchanged | `AE4-0` |
| Placeholder | Inter **14 / 20** (rail: 13/20) | `AE5-0` |
| Caption | Inter 11 / 16, `width: 180px` (rail: `170px`) | `AE7-0` |
| Submit button | `padding: 9px 16px` (rail: `7px 16px`), `border-radius: 6px`, ground `--color-accent`, label Inter 13/16 w600 `#FFFFFF` | `AE9-0` |

### 4.11 Mobile footer strip — two rows (the control that opens the sheet)

| Element | Value | Node |
|---|---|---|
| Strip | column, `gap: 8px`, `margin-top: 6px`, `padding-top: 13px`, `border-top: 1px #E8ECF1` (→ `--color-border`) | `A3J-0` |
| Row one | `align-items: center`, `gap: 10px`, flex spacer between chip and count | `A3K-0` |
| Blocked chip | `height: 30px`, `padding-inline: 11px`, `gap: 7px`, `border-radius: 999px`, ground `#9E3B3617` (→ `--color-blocked-bg`); 7px dot, `Blocked` Inter 12/16 w600, `2 blockers` Inter 12/16 w400, chevron `11 × 11` `stroke-width 2.6` — all `--color-blocked` | `A3L-0` |
| **Comment count pill** | `height: 30px`, `padding-inline: 11px`, `gap: 6px`, `border-radius: 999px`, ground `--color-accent-bg`; icon `14 × 14` `stroke-width 2` stroke **`--color-accent`**; count Inter **13 / 16 w600** `--color-accent`; `flex-shrink: 0`, hard right | `A3S-0` |
| Row two | `align-items: center`, `gap: 8px` | `A3W-0` |
| Detail chips (closed) | `height: 30px`, `padding-inline: 11px`, `gap: 6px`, `border-radius: 999px`, ground `--color-card`, `1px --color-border`; label Inter 12/16 **w500** `--color-muted`, `width: max-content`; chevron `11 × 11` `stroke-width 2.6` `--color-faint` | `A3X-0`, `A41-0`, `A45-0` |

Note the icon stroke: on desktop the bubble is `--color-faint` even inside the
selected pill (`41X-0`); on mobile it is `--color-accent` (`A3S-0`). That is
R-F.3's "on mobile it is the control that opens the sheet" made literal.

### 4.12 Other components visible on these boards (values, not specification)

These belong to groups 02 / 05 and are recorded only so this screen's geometry
is checkable.

| Element | Value | Node |
|---|---|---|
| Mobile claim status chip | `padding: 3px 9px 3px 7px`, `gap: 5px`, `border-radius: 999px`, ground `#2C6B521A` (→ `--color-locked-bg`); padlock `11px`; `LOCKED` Inter **11 / 14** w600 tracking `+0.05em` `--color-locked` — matches R-H.2 exactly | `A35-0`, `A39-0` |
| Mobile claim title | Inter **18 / 24**, w600, tracking `-0.01em`, `--color-ink` — matches R-H.2 | `A3B-0` |
| Mobile claim id | IBM Plex Mono **11 / 15**, w400, `--color-faint` — matches R-H.2 | `A3C-0` |
| Mobile blocked notice | inset card, `margin: 14px 16px 0`, `border-radius: 10px`, `1px #9E3B3633`, ground `#9E3B360F`; body row `padding: 12px 14px`, `gap: 10px`; triangle `15px` `stroke-width 2` `--color-blocked`; body Inter **13 / 19** `--color-ink`; action row `height: 38px`, `padding-inline: 14px`, `border-top: 1px #9E3B362E`; `Show issues` Inter **13 / 16** w600 `--color-blocked` + `13px` chevron `stroke-width 2.6` — matches R-H.1 exactly | `A2T-0` |
| Desktop module blocked banner | `padding: 13px 32px`, `gap: 12px`, ground `#9E3B360F` light / **`--color-dark-blocked-surface` `#1D1618` dark** | `41C-0` / `9QO-0` |

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. **No 390px breakpoint
is added.** The `390` and `1440` figures are canvas widths (`tokens.md` § 8).

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | The 268px module sidebar is dropped and folded into the app-bar hamburger (`A2H-0`, 52px tall, 16px gutter). | `max-width: 860px` | The sidebar→drawer transition is group 02's and already lives at 860 (`style.css:4533`); this screen inherits it and adds nothing. |
| M2 | The comments rail becomes a bottom sheet: scrim, 16px top corners, 36 × 4 grabber, header hairline, `min-height 240 / max-height 660 / height fit-content`, internal scroll. | `max-width: 860px` | See **Open decisions**. The rail→sheet switch already exists at 860 (`style.css:2227–2240`) together with the modal half (`body.comments-open` scroll lock, `.comments-overlay`), and the role/`aria-modal` swap in `viewer-runtime.js:548` is keyed to the same 860 matchMedia. Splitting the shell's geometry to 520 would leave 521–860 with a sheet that has no grabber and the wrong corners. |
| M3 | The rail's left border and `-8px` shadow are dropped; the sheet's corners and the scrim replace them. | `max-width: 860px` | Same block; the engine already does `border-left: 0; border-top: 1px` there. |
| M4 | **Dark only:** a 1px top hairline on the sheet. | `max-width: 860px` **inside** the dark theme blocks (`html[data-theme="dark"]` and `prefers-color-scheme: dark`) | R-J.2 makes the hairline a dark-mode item, not a width item. The engine's current rule sets `border-top` unconditionally — that must become dark-only. |
| M5 | A mono count appears in the sheet header (`ACM-0`); the desktop rail header has none. | `max-width: 860px` | R-J.4: the count is a *slot* of the shared sheet header. It is a property of the sheet, not of the phone. |
| M6 | Gutters step `22px → 16px` throughout the panel (header, body, composer). | `max-width: 860px` | Part of the sheet shell; the sheet is full-bleed and 22px would leave the composer's button crowding the edge. |
| M7 | Comment bodies `13/21 → 14/22`; placeholder `13 → 14`. | `max-width: 860px` | Reading size on a phone-width column, tied to the sheet, not to the footer. |
| M8 | Resolve pill padding `4/10 → 5/11`; Comment button padding `7/16 → 9/16`. | **`(pointer: coarse)`** | Target size is a pointer question (`tokens.md` § 8 rule of thumb). Pair with the engine's existing `min-height: 44px` set in the `@media (pointer: coarse)` block at `style.css:2211–2221`, which already lists `.comment-action`, `.comment-composer-submit`, `.comment-composer-input` and `.comments-rail-close`. |
| M9 | `Resolve`, `Reply` and `4 resolved` step w500 → w600. | **`(pointer: coarse)`** | `AOY-0` calls it "the mobile chip rule" — a legibility-without-hover change, not an arrangement change. |
| M10 | The nested reply drops its `14px` indent and keeps the `2px` left rule. | `max-width: 520px` | Arrangement, and it is the phone tier that needs the 14px back for the body measure. R-J.7 fixes the rule at 2px and does not fix the indent, so dropping it is legal. |
| M11 | Thread spacing switches from `10/12/4` margins to `2/6/2` padding. | `max-width: 520px` | Arrangement. |
| M12 | Footer strip becomes two rows: blocked chip + comment count, then the three detail chips as bordered pills. | **`max-width: 520px`**, written to win over the **only two** responsive rules that touch `.claim-footer__counts`: `style.css:4640–4641` (inside `@media (max-width: 860px)`, opened at `style.css:4533`) and `style.css:4655–4656` (inside `@media (max-width: 560px)`, opened at `style.css:4647`). All three queries have equal specificity, so the new 520 block must be **placed after the 560 block closes at `style.css:4658`**; the *existing* 520 block at `style.css:1175` is earlier in the file and would lose. | R-F.4 and `tokens.md` § 8 row H3 / Disagreement 16 — with the correction in **OD14.11**: the `max-width: 640px` rule those two sources name does **not** touch `.claim-footer__counts` (`style.css:2563` is a `.gcp-*` console block). |
| M13 | The comment count takes `--color-accent-bg` with an **accent** icon stroke (desktop keeps a faint stroke even when selected). | `max-width: 520px` | R-F.3 / `tokens.md` § 8 row J2: below the phone fold the count stops being a passive number and becomes the control. |
| M14 | Detail chips gain the closed-chip pill form (30px, `999px`, `1px --color-border`, w500 label). | `max-width: 520px` | R-F.2 / section I3; arrangement. |
| M15 | The two 13px hover glyphs (edit, delete) are **not rendered**. | **`(pointer: coarse)`** | `AOV-0`: "there is no hover on a phone and a 13px target is not a control." It is a hover capability question, not a width question. Hiding them by width would also hide them on a narrow desktop window that still has a mouse. |
| M16 | Every tap target in the sheet is ≥ 44px. | **`(pointer: coarse)`** | R-J.2 / the engine's existing block. The 36 × 4 grabber is a **drag affordance**, not a control, and is exempt — it must not be padded to 44px, and the header's close button is the keyboard/AT exit. |

Elements **hidden** at 390: the module sidebar (M1), the rail's left border and
shadow (M3), the per-message hover glyphs (M15). Nothing else is dropped —
`AOY-0` is explicit that the sheet "carries everything the rail carries."

### Reconciling M7–M11 with R-H.0

R-H.0 says only three components change shape below the phone breakpoint —
blocked notice (R-H.1), claim header (R-H.2), footer strip (R-H.3) — and
"everything else on a mobile board is the desktop component at a narrower
width." M7 (bodies 13/21 → 14/22), M8, M9 (w500 → w600), M10 (reply indent
dropped) and M11 (thread spacing) look like a fourth shape change. They are not,
and the reason is structural rather than a plea for an exception:

1. **The comments rail is not narrowed on mobile — it is replaced.** R-J.1 makes
   the bottom sheet "one bottom sheet in this product, not several… what varies
   is the body." The mobile board does not render the 360px rail at 390px; it
   renders a *different component*, the section-J shell, with the comment thread
   as its body. R-H.0's clause is about a desktop component appearing at a
   narrower width. Nothing on `A24-0` / `AH2-0` is the rail at a narrower width.
2. **Section J measures the sheet itself, and R00.0 makes that measurement
   win.** Every one of M7, M9, M10 and M11 is a value R-J already fixes, not a
   value this screen invents:
   - M7 body `14/22` — **R-J.7** ("Body is 14/22 `--color-ink`"); the composer
     placeholder `14/20` — **R-J.5**.
   - M9 `Resolve` / `Reply` / `N resolved` at **w600** — **R-J.7** (all three
     are stated at "12/16 weight 600").
   - M10 the reply block at `padding-left: 12px` behind a 2px `--color-border`
     rule, with no `margin-left` — **R-J.7** ("reply block indented 12px behind
     a 2px `--color-border` left rule"). The 14px indent is a *rail* value that
     section J never had.
   - M11 thread spacing — the arrangement that follows from R-J.7's three-part
     structure at a 16px gutter; it is the only one of the five with no literal
     R-J number, and it is arrangement, which is exactly what R-H.0 permits
     ("only a different arrangement").
   - M8's `Comment` button `9px/16px` — **R-J.5** ("Submit… 9px block / 16px
     inline padding"); only the Resolve pill's `5px/11px` is unstated by J, and
     that half of M8 is carried by `(pointer: coarse)`, not by width.
   It is the **desktop rail** that is the outlier against section J, not the
   sheet: `4DC-0` (section G) measures the rail at 13/21 and w500, and R-J.7
   measures the sheet at 14/22 and w600. Both are approved; they are two
   components, and R00.0 says each takes its own board's value.
3. **M8, M9's justification, M15 and M16 are pointer rules, and R-H.0 is a width
   rule.** R-H.0 speaks of "below the phone breakpoint" and "the form that
   matches its **width**." A control sized for a finger or a glyph dropped for
   lack of hover is not a width question — see the per-rule reasons above.

**Net:** R-H.0 binds this screen at M12–M14 and at § 4.12's claim head, and is
satisfied there — the footer strip is section H3's own two-row form, not a new
one. It does not bind M7–M11, because those belong to a section-J component that
R-H.0's "everything else" clause does not describe. Recorded as **OD14.12** so
the next reader does not have to re-derive it.

---

## 6. Footer vocabulary

Quoted from the boards. Order is fixed and is R-F.1's.

**Desktop claim footer, claim with an open rail** (`41X-0`, dark `9R9-0`), left
to right:

> `Blocked` · `2 blockers` · ⌄ │ `4 relationships` ⌄ · `2 sources` ⌄ ·
> `1 check` ⌄ ——— 💬 `2`

**Desktop claim footer, claim with the rail closed** (`42X-0`):

> `Blocked` · `2 blockers` ⌄ │ `3 relationships` ⌄ · `1 source` ⌄ ·
> `No checks declared` ⌄ ——— 💬 `2`

Note the singular/plural pair — `1 source` / `2 sources`, `1 check` /
`4 relationships` — and the **empty-state wording `No checks declared`**, which
is a phrase, not `0 checks`, and is set in `--color-faint` rather than
`--color-muted`.

**Mobile claim footer** (`A3J-0`), two rows:

> row 1: ● `Blocked` `2 blockers` ⌄ ——— 💬 `2`
> row 2: `4 relationships` ⌄ · `2 sources` ⌄ · `1 check` ⌄

**Comments rail / sheet header** (`441-0`, `442-0`; `ACK-0`, `ACL-0`, `ACM-0`):

> `Comments`
> `on Mechanism-scoped observation authority`   (sheet only: mono `2`, hard right)

**Thread meta** (`449-0`, `44A-0`; `44K-0`, `44L-0`; `44W-0`, `44X-0`, `44Y-0`):

> `HUMAN`  `3 days ago`
> `AGENT`  `2 days ago`
> `AGENT`  `22 hours ago`  `(edited)`

Elapsed, one unit, no timestamp — R10.1 and R10.3. `(edited)` is lower-case,
parenthesised and italic.

**Thread actions** (`44R-0`, `44S-0`): `Resolve` · `Reply`.
**Resolved disclosure** (`459-0`): `4 resolved` — the count then the word, no
"threads", no "comments".
**Composer** (`45D-0`, `45F-0`, `45I-0`):

> placeholder `Add a comment…` (single-character ellipsis, not three dots)
> caption `Saved to the served viewer, not to this file.` (full stop included)
> button `Comment`

**Elsewhere on the board**, quoted because this screen's lane will touch the
same strings: `Show issues` (`41H-0`, `A2T-0`),
`All 26 contract claims are blocked by unapproved dependencies outside this module`
(`41G-0`), `31` `of 31 locked` (`40S-0`, `40T-0`), `Focus` `F` (`40W-0`),
`MODULE 06 / 26` (`40P-0`), `…more` (`A3I-0`), `Search 828 claims` (`3XU-0`),
`MODULES` `26` (`3XX-0`, `3XY-0`), `TRACKS` `25` (`3ZR-0`, `3ZS-0`),
`Claims graph` (`403-0`), `Build order` (`409-0`), `Light` / `Dark` (`40F-0`,
`40J-0`).

---

## 7. Code address

All paths relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.
Every address below was read from the tree at `3ac8844`.

> **Read these with `git show 3ac8844:<path>`, not from the working copy.**
> The worktree's checkout of `internal/render/viewer/template/style.css` (and
> of `graph.css`) is dirty relative to `3ac8844` — the in-flight Geist → Inter /
> IBM Plex font swap — which shifts every address in § 7.1 by roughly **+88**
> lines: the comments marker moves 1589 → ~1677, the interactive marker
> 1774 → ~1862, `.comment-chip` 1602 → ~1690 and 4136 → ~4224,
> `.comments-rail` 1808 → ~1896 and 4176 → ~4264. The line numbers below are
> correct against the pinned commit and were verified against it. A lane agent
> that greps the live file will land short; grep for the **selector**, not the
> line, once the swap has landed.

### 7.1 `internal/render/viewer/template/style.css`

Two section markers own this screen.

- **`style.css:1589`** — marker, quoted:
  `/* ---- comments (engine-managed review threads) --------------------------`
  followed by
  `   The 💬 chip rides the claim's .k head line (components.CommentChipHTML),`
  `   pushed to the far right by the .k > .claim-comments-slot rule further`
  `   down; the read-only thread panel is baked in after the edges footer by`
  Range **1589–1773**. Selectors: `.comment-chip` (1602), `.comment-chip--open`
  (1617), `.comment-chip--empty` (1643), hover/focus reveal (1654–1657),
  `.comment-chip--empty:hover` (1664), `.comment-chip-count` (1672),
  `.comments-panel` (1684), `.comments-panel-title` (1697), `.comment-thread`
  (1706), `.comment-reply` (1715), `.comment-meta` (1721), `.comment-role`
  (1730), `--human` (1735), `--agent` (1739), `.comment-edited` (1743),
  `.comment-body` (1747), `.comments-resolved > summary` (1753), and a nested
  coarse-pointer block at 1765–1772.
- **`style.css:1774`** — marker, quoted:
  `/* ---- interactive comment UI (Phase 5, serve + file://) -----------------`
  `   The 💬 chip opens a shared overlay panel: a non-modal right rail on desktop`
  `   (role=complementary), a modal bottom sheet on mobile. …`
  Range **1774–2011** (the next marker is
  `/* ---- status strip (lock-ledger integrity + lint) ---` at **2012**).
  Selectors: `.comments-overlay` (1793), `.comments-rail` (1808 — today
  `width: min(380px, 90vw)`, `background: var(--paper)`,
  `border-left: 1px solid var(--border)`), `.comments-rail[hidden]` (1821),
  `.comments-rail-head` (1829), `.comments-rail-title` (1839 — today mono
  11px uppercase `+0.05em` `--faint`), `.comments-rail-close` (1852),
  `.comments-rail-body` (1864), `.comments-loading/-error/-empty` (1871–1873),
  `.comments-error` (1879), optimistic states (1885–1889), `.comment-pending`
  (1891), `.comment-meta .comment-action` (1897), `.comment-edit` (1902),
  `.comment-action` (1906), `:hover` (1917), `.comment-delete:hover` (1922),
  `.comment-composer-input:focus` (1976), `.comment-composer-submit` (1981),
  `.comments-toast` (1992), `[hidden]` (2008).

Breakpoint blocks this screen writes into:

- **`style.css:2211`** `@media (pointer: coarse)` — already sets
  `min-height: 44px` on `.comments-rail-close`, `.comment-action`,
  `.comment-action-text`, `.comment-composer-submit`, `.comment-composer-input`
  — plus `.status-strip-head` and `.claim-collapse-toggle`, which are not this
  screen's. Query on **2211**, selector list **2212–2218**, `min-height` on
  **2219**, rule brace **2220**, media brace **2221**. M8, M9, M15, M16 land
  here.
- **`style.css:2227`** `@media (max-width: 860px)` — the sheet block. Quoted
  header: `/* Mobile: the rail becomes a modal bottom sheet — the dimming
  backdrop and the body scroll-lock are the MODAL half of the panel and apply
  ONLY at this breakpoint … */`. Today: `body.comments-open { overflow: hidden }`
  (2228–2230), `.comments-overlay:not([hidden]) { display: block }`
  (2231–2233, its closing brace on 2233),
  `.comments-rail { top:auto; left:0; right:0; bottom:0; width:100%;
  max-height:70vh; border-left:0; border-top:1px solid var(--border);
  border-radius:14px 14px 0 0 }` — selector on **2234**, `border-radius` on
  **2243**, closing brace on **2244**, so the block is **2234–2244**. M2–M7
  land here.
- **`style.css:4640–4641`** (inside `@media (max-width: 860px)`, opened at
  **`style.css:4533`**) —
  `.claim-footer__counts { flex-wrap: wrap; justify-content: flex-end; }` and
  `.claim-footer__counts > span:not(.claim-footer__chevron) { font-size: 8.5px; }`
  — and **`style.css:4655–4656`** (inside `@media (max-width: 560px)`, opened at
  **`style.css:4647`**, block closing at **4658**) —
  `.claim-footer__counts { justify-content: flex-start; }` and
  `.claim-footer__counts > span:not(.claim-footer__chevron) { padding: 4px 6px; }`.
  Together with the base declarations at **4053–4062** these are the **only**
  `.claim-footer__counts` rules in the file, and 4640–4641 / 4655–4656 are the
  two M12's 520px rule must be written to beat. Equal specificity, so the new
  520 block goes **after 4658** and before the print marker at 4721 — not into
  the existing 520 block at **`style.css:1175`**, which is earlier in the file
  and would lose to both. **`style.css:2563` is not one of them**: it opens
  `@media (max-width: 640px)` but its body is `.gcp-console`, `.gcp-row`,
  `.gcp-sev`, `.gcp-time`, `.gcp-msg` — the graph-console rows — and it carries
  no `.claim-footer__counts` declaration. See **OD14.11**.
- **`style.css:4136`** `.comment-chip` and **`style.css:4141`**
  `.comment-chip--open` — the *later, winning* re-declaration
  (`border-radius: 5px; font-size: 10px`, and `--open` taking
  `border-color/background/color` from `--accent` / `--accent-bg`). **A lane
  agent changing the chip must edit this copy, not 1602/1617** (`tokens.md`
  § 1, "a token has to be read by the declaration that wins").
- **`style.css:4176`** `.comments-rail` and **`style.css:4182`**
  `.comments-rail-head` — the winning re-declaration:
  `border-left-color: var(--border); background: var(--card-bg);
  box-shadow: -14px 0 34px var(--shadow-cast, rgba(9,20,38,.12))`. The board's
  `-8px 0 24px` supersedes the `-14px 0 34px` here.
- **`style.css:4422`** `.comments-rail-close:focus-visible { outline: 2px solid
  var(--accent); outline-offset: 2px }`.
- **`style.css:4533`** `@media (max-width: 860px)` — the sidebar→drawer and
  `.content-area { padding: 54px 16px 58px }` block that M1 inherits.
- **`style.css:4721`** `@media print` — the single, last print block. The rail
  and the sheet must not print.
- `graph.css` has no rule for this screen (9 matches for "comment" are all
  CSS comments). Its only relevance is `--dxg-facet-1`, the source of the
  `AGENT` colour (D14.1).

### 7.2 Runtime

`internal/render/viewer/template/viewer-runtime.js` — the whole comment surface:

- element handles: `viewer-runtime.js:366–371`
  (`commentsPanel`, `commentsRailBody`, `commentsRailTitle`,
  `commentsRailClose`, `commentsOverlay`, `commentsToast`).
- `chipsFor` `:462`, `setChipExpanded` `:469`, `updateChips` `:474`,
  `syncEmptyChips` `:513`, `recomputeChipsFromPanel` `:522`.
- `commentPanelOpen` `:534`, **`openCommentPanel` `:537`**,
  `closeCommentPanel` `:560`.
  `viewer-runtime.js:542` is the header line this screen replaces:
  `if (railTitle) { railTitle.textContent = 'Comments — ' + claimID; }`
  — today one line carrying the **slug**; the boards want two lines carrying
  `Comments` and `on <claim title>` (R09.8: "The reviewer recognises the title.
  The slug is for the agent.").
  `viewer-runtime.js:548` is the `matchMedia('(max-width: 860px)')` role /
  `aria-modal` swap that M2's decision keeps aligned with the CSS.
- read-only (file://) panel clone: `function renderPanelReadOnly` is declared at
  **`:583`**, under the comment block that opens at `:579`; its body runs to
  `:598`. Live fetch: `function renderPanelFromAPI` is declared at **`:604`**,
  under the comment block that opens at `:602`; its body runs to `:636`.
  (`renderPanel`, the dispatcher between them, is at `:570`.)
- `buildPanel` `:672`, `syncEmptyLine` `:709` (empty strings
  `'No comments.'` `:596` and `'No comments yet — add the first one below.'`
  `:715`), `buildThread` `:730`, `buildMessage` `:755`,
  **`buildThreadActions` `:788`** (today an **icon-only** Resolve/Reopen button,
  `aria-label` `'Resolve thread'` / `'Reopen thread'`, and **no Reply label** —
  the boards want a labelled Resolve pill plus a bare `Reply`),
  `iconButton` `:802`, `growNow` `:817`, `autoGrow` `:822`,
  **`buildComposer` `:826`** (placeholder `'Add a comment…'` `:831`, submit label
  `'Comment'` `:834` — both already match the boards; the caption
  `Saved to the served viewer, not to this file.` **does not exist anywhere in
  the tree** and is new), `buildReplyComposer` `:847`, `threadNode` `:882`,
  `doReply` `:923`, `doResolve` `:948`, `doReopen` `:961` (its
  `POST …/reopen` on `:965`), `doDelete` `:974` (its `DELETE
  /api/claims/…/comments/…` path on `:987`), `startEdit` `:1011` (its `PATCH`
  path on `:1040`). All six are `function` declarations at the cited lines.
- live/SSE body classes `:1912`, `:1966–2040`; overlay click-to-close
  `:2239–2240`.

`internal/render/viewer/template/shell.html:242–250` — the markup the rail and
sheet share:
`<button id="commentsOverlay" class="comments-overlay" …>` (`:242`),
`<aside id="commentsPanel" class="comments-rail" role="complementary"
aria-label="Comments" tabindex="-1" hidden>` (`:243`),
`<header class="comments-rail-head">` (`:244`),
`<p id="commentsRailTitle" class="comments-rail-title">Comments</p>` (`:245`),
`<button id="commentsRailClose" …>` (`:246`),
`<div id="commentsRailBody" class="comments-rail-body"></div>` (`:248`),
`<div id="commentsToast" …>` (`:250`). The grabber, the header's second line
and the header's mono count have **no element today**.
`shell.html:10` declares `<meta name="dossierx-viewer-runtime"
content="comments-sse">`.

`system-record.js`, `graph-ui.js`, `build-order-ui.js` and `build-order.html`
carry no comment code; this screen does not touch them.

### 7.3 Go emitter

- **`internal/render/components/components.go:659`** — `func CommentChipHTML(c
  model.Claim) template.HTML`. Emits
  `<span class="claim-comments-slot"[ hidden]><button type="button"
  class="comment-chip comment-chip--{empty|open|resolved}" data-claim-id=…
  aria-controls="commentsPanel" aria-expanded="false" aria-label=…><span
  class="comment-chip-glyph">…#dx-icon-message-circle…</span> <span
  class="comment-chip-count">N</span></button></span>`.
  Three chip variants exist today — `--empty`, `--open`, `--resolved` — and the
  count is `open` when open > 0, else `total`.
- **`internal/render/components/components.go:67`** — `"commentChip":
  CommentChipHTML` in the funcMap; **`components.go:617–618`** documents that
  each chip-bearing partial calls `{{commentChip .}}` from its
  `<div class="k">` head.
- **`internal/render/components/comments.html:36–49`** — the baked, read-only
  `.comments-panel`: `<p class="comments-panel-title">Comments</p>`, open
  threads, then `<details class="comments-resolved"><summary>{{len .Resolved}}
  resolved</summary>`. The summary string already matches the board's
  `4 resolved`.
  `comments.html:24–27` — `dossierx-comment-message`: `.comment-meta` with
  `.comment-role--{{.Author}}`, `<time class="comment-time"
  datetime=…>{{.Created}}</time>`, optional `.comment-edited` `(edited)`, then
  `.comment-body`. **`{{.Created}}` is printed raw** — R10.1/R10.4 require an
  elapsed phrase computed in the browser from that `datetime`, so this is where
  the absolute stamp still leaks into the HTML.
  `comments.html:28–34` — `dossierx-comment-thread`, `<div class="comment-reply"
  data-reply-id=…>` for replies (the 2px-rule block).
- **`internal/render/render.go:1190–1198`** — the `stripOverviewIDs` contract
  that the chip and slot must keep emitting **no** ` id="` sequence.
- **`internal/render/depended_by_view.go:107`** — the other view that binds the
  chip.

### 7.4 Tests that assert on these selectors today

- `internal/render/theme_tokens_test.go:540` —
  `"shadow": {{".comments-panel", "box-shadow"}}`; `:541`
  `"shadow-strong": {{".comments-toast", "box-shadow"}}`; `:542–543`
  `"shadow-cast": {{".comments-rail", "box-shadow"}, …}`. Changing the rail's
  shadow property must keep a `var(--shadow-cast…)` consumer on `.comments-rail`.
- `internal/render/comments_render_test.go:60, 66, 74, 101–108, 129, 136, 139,
  157–158, 168, 178, 226` — markup counts on
  `class="comment-chip comment-chip--open"`, `class="comments-panel"`,
  `<span class="claim-comments-slot"` and `… hidden>`.
- `internal/render/components/comments_test.go:63, 69–72, 78, 81, 84, 89, 105,
  128, 132, 138, 141, 144` — the exact chip string, including
  `<span class="comment-chip-count">0</span>` and the absence of
  `comment-chip--open` / `--resolved` / `comments-panel` on an empty claim.
- `viewer-tests/viewer_test.go:73, 83, 108, 121, 125, 150, 160, 180, 272–273,
  297, 388, 394, 428–445` — chip click → `body.comments-open`,
  `aria-expanded="true"`, `.comment-chip-count` textContent, `--resolved` and
  `--open` class transitions, and the nav/comments mutual-exclusion pair.
- `viewer-tests/empty_chip_test.go:39–45, 51–52, 59, 62, 76–77` — the zero-thread
  chip, its `aria-label` `"add the first comment on this claim"`, and the
  `closest('.claim-comments-slot').hidden` reveal.
- `viewer-tests/autogrow_test.go:99–101, 110, 124, 130` —
  `#commentsPanel .comment-composer .comment-composer-input` and
  `.comment-composer-submit`, plus
  `#commentsPanel .comments-threads > .comment-thread` counting.
- `viewer-tests/fix3_test.go:153–154, 185, 262–272, 282–300, 310–321, 343–347,
  359–362, 373–383, 396–413` — draft survival across rebuilds, reply composer,
  edit form, optimistic threads.
- `viewer-tests/claim_collapse_test.go:98, 105, 112–113` — the chip's
  `getBoundingClientRect()` inside a collapsed claim head, and
  `!document.getElementById('commentsPanel').hidden`.
- `viewer-tests/live_reload_test.go`, `viewer-tests/theme_parity_test.go`,
  `viewer-tests/fenced_code_seam_test.go` also reference the comment surface.

Any rename of `.comments-rail`, `.comments-rail-head`,
`.comments-rail-title`, `.comments-rail-body`, `.comment-chip`,
`.comment-chip-count`, `.claim-comments-slot`, `.comments-threads`,
`.comment-thread`, `.comment-reply`, `.comment-composer`,
`.comment-composer-input`, `.comment-composer-submit`, `.comments-resolved`,
`.comments-panel` or the body classes `comments-open` / `comments-live`
breaks the lists above. **Restyle these selectors; do not rename them.**

---

## 8. States not on the boards

The four boards depict exactly one situation: a locked-and-blocked claim with
**two open threads, one nested reply, one edited message, four resolved
threads**, on a live serve. Everything below is a state the engine can render
that the boards do not draw. Each entry says what this spec requires, derived
from the rules — no lane may be left guessing.

1. **Zero threads (`comment-chip--empty`).** The chip renders `💬 0`, its slot
   ships `hidden` and shell.html reveals it only after `/api/ping` confirms a
   live serve (`components.go:659` commentary). *Spec:* the empty chip takes the
   **passive** desktop form of § 4.6 (no accent pill, faint icon, faint count) —
   it is not selected and nothing is open. On mobile it takes the 30px pill
   shell of § 4.11 but with `--color-card` ground and a `1px --color-border`
   outline, i.e. the closed detail-chip treatment, because the accent tint in
   R-F.3 marks *a control with content behind it*. Opening it shows the rail /
   sheet with the composer and the empty line
   `No comments yet — add the first one below.`
   (`viewer-runtime.js:715`) — Inter 13/21 (rail) / 14/22 (sheet) in
   `--color-muted`, and **no** thread rule, **no** resolved row.

2. **All threads resolved (`comment-chip--resolved`).** `count = total`,
   `aria-label` "view N comment thread(s), all resolved". *Spec:* the count is
   passive (faint) at every width, because R-F.3's accent means "there is open
   work here" only through the selected state; the rail body shows **no** open
   thread block and the resolved disclosure row of § 4.4 becomes the first and
   only row. The `--resolved` chip must stay visually distinct from `--empty`:
   the engine's own reason (`components.go:640–642`) is that "no one has
   commented" and "everything raised was settled" are different facts.

3. **Rail open on a claim with zero open threads but a live composer.** Header
   subtitle still names the claim (`AOS-0`'s rule). The sheet's mono count slot
   (§ 4.8) renders `0`, not blank — R-J.4 makes it a slot, and a slot that is
   present renders its value.

4. **File:// (static export), no live serve.** `/api/ping` fails, empty slots
   stay hidden, and `viewer-runtime.js:579–598` clones the baked
   `.comments-panel` instead of fetching. *Spec:* **no composer, no Resolve, no
   Reply, no edit/delete glyphs** — a control with nowhere to POST is a dead
   control (`comments.html:18–21`). The composer caption
   `Saved to the served viewer, not to this file.` therefore never appears in a
   static export; it is a serve-only string. The resolved `<details>` still
   works, because it needs no JS.

5. **A read-only viewer that has threads but no write API.** Same as (4) for the
   controls; R-J.6 names read-only as a **state of the one shell**, not a second
   sheet. Board `45K-0` (group 14a) owns its pixel values.

6. **Long claim titles in the header subtitle.** The rail is 360px and the sheet
   390px; the subtitle at Inter 12/18 wraps. *Spec:* the subtitle **wraps to at
   most two lines and then ellipsises**; it does not truncate at one line,
   because the subtitle is the only thing tying the thread to its claim
   (`AOS-0`). The header is `align-items: start` precisely so a two-line
   subtitle does not shift the close glyph. Today `.comments-rail-title`
   (`style.css:1839`) is `white-space: nowrap; text-overflow: ellipsis` on a
   single line — that must change.

7. **Deep reply chains.** The boards draw one reply. *Spec:* R-J.7 fixes the
   structure at **a first message, replies beneath it behind a left rule**. The
   engine's data model is one level of replies (`comments.html:31–33`) — there
   is no nesting to draw. A second rule inside a rule must never be emitted.

8. **A thread with many replies, or many threads.** *Spec:* the rail body
   (`446-0`) and the sheet body (`ACT-0`) are the scroll regions; the header and
   composer are `flex-shrink: 0` and never move (R-J.5). On the sheet the height
   stops at `660px` and the body clips at the composer — "the scroll edge, not a
   crop" (R-J.3). The `240px` floor applies when the body is short.

9. **Optimistic / pending / deleting messages.** `style.css:1885–1891` renders
   them at `opacity: .55` with an italic `--faint` `.comment-pending`. *Spec:*
   keep both. Neither is on a board; both are correct, and the opacity must be
   applied to the message block, never to the thread rule.

10. **Error and loading.** `.comments-loading` "Loading…" and `.comments-error`
    "Could not load comments." (`style.css:1871–1882`). *Spec:* they sit in the
    body scroll region at the same size as the empty line, error in
    `--color-blocked` (engine `--warn`). They are body states of the one shell,
    never a replacement header.

11. **Toast.** `#commentsToast` (`shell.html:250`, `style.css:1992`). Not on any
    board. *Spec:* out of scope for this screen's restyle beyond keeping its
    `var(--shadow-strong)` consumer, which `theme_tokens_test.go:541` asserts.

12. **A claim that is DRAFT, or locked-and-not-blocked, with comments.** The
    boards only show LOCKED + Blocked. *Spec:* the rail is **independent of
    claim status** — nothing in the rail changes colour with the claim's
    lifecycle. Only the footer strip's first chip changes (R-F.1 / R-F.2), and
    the comment count keeps its own two states from § 4.6.

13. **`review_pending` trigger variants.** A claim can carry a `review_pending`
    trigger for an independent open comment thread (`internal/lock/lock.go:1312`,
    `internal/buildorder/buildorder.go:135, 192`). *Spec:* that fact belongs to
    the claim's status chip and to the build-order gate, **not** to the rail. The
    rail must not grow a "this thread is blocking the lock" badge; R09.6's logic
    applies — one signal, in one place, and the thread is already the signal.

14. **The rail open while focus mode is on.** R11.1/R11.4: focus is one
    reversible state that hides the rails. *Spec:* opening comments from a
    footer count while in focus mode opens the rail **over** the focused layout
    without leaving focus — the same "peek" affordance R11.4 grants the edges —
    and closing it returns to the focused layout. Focus must not silently exit.

15. **Narrow desktop, 861–1180px** (`style.css:4494`). Not drawn. *Spec:* the
    rail keeps `360px` and the claim column gives way; the reading measure is
    the first thing allowed to fall below 760px in this tier, because R11.2's
    pixel-identity rule binds the focus toggle, not the viewport. Below 861 the
    sheet takes over (M2).

16. **Dark + `prefers-color-scheme` vs the explicit toggle.** Both dark boards
    are one drawing. *Spec:* every dark value in § 4 must be emitted **twice** —
    under `@media screen { html[data-theme="dark"] }` and under
    `@media screen and (prefers-color-scheme: dark) { :root }` — per `tokens.md`
    § 1. Print always takes the light palette, and § 7.1's print block must keep
    the rail and the sheet off paper.

---

## 9. Open decisions

Recorded rather than asked, per the freeze protocol.

- **OD14.1 — the rail is 360px wide, and `--container-rail` does not apply.**
  `--container-rail` is `268px` and on this board that is the *left module
  sidebar*, not the rail (`3XL-0` = 268, `43Y-0` = 360). The engine ships
  `width: min(380px, 90vw)` (`style.css:1808`). **Decision: `min(360px, 90vw)`.**
  The board is the measurement; the `90vw` clamp is kept because it is the only
  thing protecting a 400px-wide desktop window, and the board cannot measure
  that case.

- **OD14.2 — the sheet's shell geometry goes in `@media (max-width: 860px)`,
  not `520px`.** `tokens.md` § 8 maps "J · bottom sheet" onto `520px`. That
  mapping is right for a *new* sheet; the comments sheet already exists at 860
  (`style.css:2227`), its modal half (`body.comments-open` scroll lock,
  `.comments-overlay`) is documented as scoped there, and
  `viewer-runtime.js:548` swaps `role`/`aria-modal` on the same 860 matchMedia.
  Moving the shell to 520 would create a 521–860 band with a sheet that has no
  grabber, 14px corners and a 70vh cap — a fourth layout nobody drew. **Decision:
  shell geometry (corners, grabber, 240/660 band, gutters, body sizes) at 860;
  arrangement rules that are genuinely phone-tier (footer two rows, chip pills,
  reply indent, thread spacing) at 520; target sizes and hover-less affordances
  at `(pointer: coarse)`.** Recorded as a refinement to `tokens.md` § 8 row J,
  not a contradiction of it: no new breakpoint is added either way.

- **OD14.3 — the desktop selected-count pill is ratified as a third state of
  the comment count.** See D14.7. Implemented as a `.comment-chip--open`-style
  *selected* modifier applied while `#commentsPanel` is showing that claim,
  distinct from the emitter's existing `comment-chip--open` (which means "has
  open threads", not "its rail is showing"). **Decision: add a
  `.comment-chip--active` modifier driven by `setChipExpanded`
  (`viewer-runtime.js:469`) rather than overloading `--open`**, because the two
  facts are independent — a claim can have open threads with the rail shut.

- **OD14.4 — `AGENT` is `--dxg-facet-1`, read from `graph.css`, in both modes.**
  Paper's `--color-dark-graph-facet-1` `#8E9BF0` is unusable (`tokens.md`
  Disagreements 1–2) and the notes strip itself calls it "a token candidate, not
  settled". **Decision: light `#4257C4`, dark `#7C8CE8` (graph.css).** The
  contrast concern `AOY-0` raises is real and is solved by the engine's value,
  which is lighter than the light twin by design.

- **OD14.5 — the header's second line is the claim TITLE, and the slug does not
  appear in the rail at all.** Today `viewer-runtime.js:542` writes
  `'Comments — ' + claimID`. R09.8 ("swapped for the title") and `AOS-0` ("names
  that claim in its subtitle") both point at the title. **Decision: two lines,
  `Comments` and `on <title>`; the slug is available on `title=` hover only.**

- **OD14.6 — elapsed time is computed in the browser.** `comments.html:25`
  prints `{{.Created}}` raw into `<time datetime=…>`. R10.4 forbids a baked
  relative phrase and R10.1 forbids a bare stamp. **Decision: keep `datetime`
  as the machine value, render the elapsed phrase client-side from it, one unit
  (R10.3), and put the absolute time on `title=`.** Under `dossierx serve` the
  freshness footer reads `Live` (R10.5) — that is the right rail's footer, not
  this rail, and is unaffected.

- **OD14.7 — the composer caption is new copy and is serve-only.**
  `Saved to the served viewer, not to this file.` exists nowhere in the tree.
  **Decision: emit it only where the composer is emitted** (live serve), since
  in a static export there is no composer for it to caption.

- **OD14.8 — `Resolve` becomes a labelled pill and `Reply` a bare label.**
  Today `buildThreadActions` (`viewer-runtime.js:788`) emits an icon-only button
  and there is no Reply control in that row (reply is a separate composer).
  **Decision: follow the boards and R-J.7** — a Resolve pill (tick + label) and
  a bare accent `Reply` that reveals the existing reply composer. The
  `Reopen` variant (`:795`) keeps the same pill shell with the rotate-ccw glyph
  and the word `Reopen`; it is not drawn on any board and this is the minimal
  consistent extension.

- **OD14.9 — the 36 × 4 grabber is exempt from the 44px minimum.** R-J.2 calls
  it a drag affordance; padding it to a 44px control would push the header down
  and make it a tab stop. **Decision: exempt, and keep the header close button
  as the accessible exit.**

- **OD14.10 — the scrim uses the engine `--scrim`, not the board's alphas.**
  See D14.6.

- **OD14.11 — the "`max-width: 640px` rule on `.claim-footer__counts`" does not
  exist, and two upstream docs say it does.** Read at `3ac8844`, the file's only
  `.claim-footer__counts` declarations are the base rules at `style.css:4053–4062`
  and two responsive ones: `4640–4641` (inside `@media (max-width: 860px)` at
  `4533`) and `4655–4656` (inside `@media (max-width: 560px)` at `4647`).
  `style.css:2563` does open `@media (max-width: 640px)`, but its body is
  `.gcp-console` / `.gcp-row` / `.gcp-sev` / `.gcp-time` / `.gcp-msg` and it
  never names the footer counts. Two upstream documents inherit the error:
  `tokens.md` § 8 row H3 ("the existing `max-width: 640px` and `max-width: 560px`
  rules already touch `.claim-footer__counts`") and `reference-rules.md`'s
  **R-F.4 "Maps onto"** clause, which repeats it verbatim. `tokens.md`
  Disagreement 16 and its breakpoint table row for 560 are, by contrast, both
  correct — they name 560 and only 560. **Decision: M12 is written to beat
  `4640–4641` and `4655–4656`, in a new `@media (max-width: 520px)` block placed
  after `style.css:4658`. `tokens.md` § 8 row H3 is corrected in place as part
  of this fix. `reference-rules.md` R-F.4 is left as written and flagged here
  instead** — R00.0 makes the rules file canonical for other groups, and a
  cross-group edit to a rule's "Maps onto" clause is not this screen's to make.
  The lane implementing R-F.4 must take the addresses from this decision, not
  from the rule text.

- **OD14.12 — R-H.0 binds this screen at the footer strip and the claim head,
  and does not reach the sheet's interior.** The full argument is in § 5,
  "Reconciling M7–M11 with R-H.0". In short: the mobile board does not narrow
  the comments rail, it substitutes the section-J shell (R-J.1), and R-J.5 /
  R-J.7 measure that shell's body independently — 14/22 bodies, w600 actions, a
  12px reply indent with no left margin, a 9/16 submit. **Decision: those values
  come from section J, under R00.0, and R-H.0's "everything else is the desktop
  component at a narrower width" clause does not describe a component that is
  not on the mobile board at all.** The genuine section-H obligations (M12–M14,
  § 4.12) are met.

### Paper defects found

| id | Board / node | Defect |
|---|---|---|
| D14.1 | `9TA-0`, `ANL-0`, `AMY-0` | `AGENT` uses `--color-dark-graph-facet-1` `#8E9BF0`, which disagrees with `graph.css`'s live `#7C8CE8`; the notes strip itself calls it unsettled. |
| D14.2 | `3XL-0`, `41B-0`, `41C-0`, `41X-0` (incl. `42K-0`), `42X-0`, `42M-0`, `43M-0` | The desktop **light** board spells the sidebar, card, banner, footer and separators as raw hexes (`#EFF1F4`, `#FFFFFF`, `#DDE2E9`, `#E8ECF1`, `#9E3B36`, `#54606F`, `#6E7C8E`, `#9E3B360F`) where tokens exist. Includes `42K-0`, the speech-bubble stroke inside the *selected* count pill: the pill's ground and count are `var(--color-accent-bg)` / `var(--color-accent)` but the stroke is the literal `#6E7C8E` — use `--color-faint`. The rail and the whole dark board are token-clean. |
| D14.3 | `9R9-0` | Dark footer top rule is `#212934`; `--color-dark-border` is `#242C38`. A near-miss literal with no token. |
| D14.4 | `441-0` vs `ACK-0` | The rail header title carries no `letter-spacing`; the sheet header title carries `-0.01em`. R-J.4 fixes `-0.01em`. |
| D14.5 | `43Y-0` vs `AC5-0` | Same component, two font-family spellings: `var(--font-sans)`/`var(--font-mono)` on desktop, the literal `"Inter", system-ui, sans-serif` / `"IBM Plex Mono", system-ui, sans-serif` on mobile. |
| D14.6 | `ABU-0`, `AJ6-0` | Scrim is `#10172057` / `#00000085` against the engine `--scrim` `rgba(0,0,0,.22)` / `rgba(0,0,0,.42)`. R-J.2 names the token. |
| D14.7 | `41X-0` vs `42X-0` vs R-F.3 / `B0C-0` | The desktop board draws **two** comment-count forms (selected pill, passive count); R-F.3 and components section J2 describe only one desktop form. The components board needs the third row. Ratified as an extension, OD14.3. |
| D14.8 | `44C-0`, `44E-0` vs `44V-0` | The edit/delete hover glyphs appear on the HUMAN thread and not the AGENT thread. Correct behaviour (own-comment controls) but stated nowhere except `AOV-0`'s prose; recorded so it is not read as a drawing slip. |
| D14.9 | `9M9-0` band | The band's own caption says "light pair left, dark pair right", but the page order places the mobile light board (`A24-0`, worldX 1480) between the desktop light (`3XK-0`, worldX 0) and the desktop dark (`9MV-0`, worldX 1990). The captioned reading order and the canvas order disagree by one column. Cosmetic; noted so a later reader does not mis-pair the boards. |
