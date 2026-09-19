# 07a · Claim — draft, not yet approved

Screen group 07a. Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`.
Engine side read from the worktree `feat/viewer-design-revamp` at `3ac8844`.

Read this with [`../tokens.md`](../tokens.md) and
[`../reference-rules.md`](../reference-rules.md). Token names below are the
**Paper** names from `tokens.md` §1; the engine token each one maps onto is in
that file's right-hand column and is what a lane agent actually writes. Every
number here was read with `get_jsx` (inline-styles), `get_computed_styles`,
`get_tree_summary` or `get_node_info`. Nothing on this page was measured off a
screenshot.

---

## 1. Boards

| Node | Name | Size | What it shows |
|---|---|---|---|
| `47K-0` | 07a · Claim — draft, not yet approved | 1220 × 1345 | **Desktop light.** One draft claim, `capability-support.contract.capabilities-own-their-dimensions`, with **all four footer expansions open at once**: readiness blockers, relationships, sources, implementation checks. |
| `7W2-0` | …· DARK | 1220 × 1345 | **Desktop dark.** Same claim, same four expansions, dark twin of every value on `47K-0`. |
| `88A-0` | …· MOBILE LIGHT | 390 × 1810 | **Mobile light.** Same claim at phone width as a fit-content column (not a 390 × 844 device frame): full-bleed card, two-row footer of pills, stacked blocker path, two-line relationship rows, label-above-value checks. |
| `8V2-0` | …· MOBILE DARK | 390 × 1810 | **Mobile dark.** Dark twin of `88A-0`. |
| `931-0` | Group 07a — band | 3420 × 28 | The group band. `933-0` "07A · CLAIM — DRAFT, NOT YET APPROVED"; `934-0` "four designs of one screen · light pair left, dark pair right"; `935-0` a 3420 × 2 rule. |
| `96Q-0` | Group 07a — notes | 2828 × 308 | The notes strip. **Not in the first 100 root children** — reached via `find_nodes` on the text "WHAT THIS SCREEN ANSWERS" (`96S-0`), whose `artboardId` is `96Q-0`. Three columns: `96R-0` (what the screen answers), `96U-0` (what mobile does differently, plus a late-disclosure addendum `9XQ-0`), `96X-0` ("DRAFT IS ALWAYS AMBER — RESOLVED"). |
| `47L-0` / `7WE-0` / `88X-0` / `8Z4-0` | spec caption blocks | — | The eyebrow + lede that sit above the card on each of the four boards. They are board furniture, **not** viewer chrome — do not implement them. |

Reference boards that bind this screen: **08 · Severity & integrity** `O2-0`
and **09 · Placement map** `WI-0`. Components board `377-0` is the source of
truth for every piece of claim chrome here (R00.0).

### Viewer states depicted

All four boards depict **exactly one** state combination:

- claim **status = draft** (`DRAFT` chip, amber, beside/above the title);
- claim **readiness = blocked**, 1 blocker, in 1 module, at 1 hop, direct;
- **3 relationships** — 1 governed-by (target LOCKED), 1 depends-on (target
  DRAFT), 1 depended-on-by (target DRAFT);
- **0 sources**, by design, with a prose reason;
- **embodiment mode = none** ("DECLARATION: none" + a REASON), i.e. the
  engine's `model.EmbodimentModeNone` / `data-conformance-state="declared_none"`
  branch;
- **0 comment threads**;
- **not** review_pending, **not** locked, **no** drifted implemented-in file,
  **no** cycle.

All four expansions are drawn open simultaneously. Per note `96T-0` that is a
property of the **board**, not of the product: "This is a spec board, not a
device screen — a single claim shown with all four footer expansions open at
once, so the four states can be read together." Note `9XQ-0` restates it: "in
the product only one door is ever open at a time." R09.2 is the binding rule.

---

## 2. Design intent

### What the reviewer should perceive

The lede `47N-0` / `7WG-0` / `88Z-0` / `8Z6-0` states the screen's whole thesis
in one paragraph, and it is the intent every measurement below serves:

> "Draft is not a lesser locked. It is the state of a claim nobody has approved
> yet, and the viewer cannot change that — approval is recorded in the lock
> ledger by the tool, not by a button here. So the only honest thing a draft
> claim shows is what is still true of it and what is still missing."

Three perceptual consequences:

1. **A draft claim is not a degraded locked claim.** Nothing on the card is
   greyed, dimmed, struck through or hidden because the claim is draft. The
   prose is `--color-ink` at the full body size (`481-0`: serif 17/28), the
   relationships are complete, the sources reason is written out at 15/25 rather
   than shown as an empty panel. The *only* thing the draft status changes is
   the chip.
2. **Draft and blocked are two different facts and never share a colour.**
   Draft is a lifecycle state of *this* claim and is amber (`--color-draft`).
   Blocked is a statement about this claim's *dependencies* and is red
   (`--color-blocked`). Note `96T-0`: "draft is the amber lifecycle chip beside
   the title, blocked is the red footer chip and the tinted readiness region,
   and the wording never mixes them." This is why the readiness region gets a
   red-washed *ground* (`48R-0` `#FBF7F7` light; `7XJ-0`
   `--color-dark-blocked-surface`) and the status chip gets an amber *tint* —
   two different surfaces carrying two different meanings.
3. **Absence is stated in prose, not drawn as emptiness.** "No sources" and "No
   checks declared" are not empty panels. Each expansion opens onto a sentence
   that says why the thing is absent — `4C0-0` for sources, `4BG-0` for checks.
   The claim is arguing, not failing to render.

### Rules from `reference-rules.md` that bind this screen

| Rule | How it binds 07a |
|---|---|
| **R09.1** — four expansions of one strip, never four panels | The strip `482-0` is one 49px line carrying readiness / relationships / sources / checks then the comment count. The four opened sections `48R-0`, `49L-0`, `4AS-0`, `4B5-0` are expansions *of that strip*, stacked inside the same card, each separated by a 1px hairline — not four cards. |
| **R09.2** — exactly one expansion open at a time | Violated by the board on purpose (see §9). In the product, opening one closes the others. |
| **R09.3** — auto-open is reserved for readiness, and only when blocked | This claim **is** blocked, so readiness auto-opening here is legal. Relationships, sources and checks must **never** auto-open. |
| **R09.4** — relationships is one section with three directions in fixed order | `49L-0` carries `GOVERNED BY` (`49Q-0`) → `DEPENDS ON` (`49Y-0`) → `DEPENDED ON BY` (`4AI-0`), in that order, inside one expansion. Not three chips, not three sections. |
| **R09.5** — sources is split out of relationships | `4AS-0` is its own expansion with its own `SOURCES` heading and its own count, even though the count is 0. |
| **R09.6** — issues is a screen and the banner is the only way in | 07a carries **no** blocked banner, because the claim is blocked by one blocker in one module and the readiness expansion says so inline. Do not add a banner here; do not add a second route to the Issues screen. |
| **R09.8** — eight demotions, nothing deleted | Visible on this board: the claim **title** carries each blocker row ("The five verdicts is not locally approved", `4C4-0`), not the slug; the slug survives as the mono path chip `the-five-verdicts`. The engine's "shown via this dependency" wording is **absent** — R09.8 cuts it. |
| **R09.9** — no inline dependency map | There is no readiness tree or mermaid map on this board. The **breadcrumb** carries the path (`4C5-0`: source slug › target slug) and the graph pane draws the real shape — hence the "See in claims graph" affordance at `49F-0`. |
| **R-F.1** — four chips, fixed order, then the comment count hard right | `482-0`: Blocked/readiness → relationships → sources → checks → flexible spacer `48M-0` → comment count `48N-0`. Each chip is a noun and a count; none is a score. |
| **R-F.2** — chip states closed / open / blocked | Binds the **mobile** form exactly (`8I3-0`, `8I7-0`, `8IB-0`: 30px, `--radius-pill`, 11px inline padding, accent border + accent label at weight 600 for open; `8HF-0` tinted, no border, 7px dot, weight 600 for blocked). Desktop draws the same three states as bare text — see §9. |
| **R-F.3** — one comment pill, one variant | Contested on this board; the count is 0 here. See §9. |
| **R-F.4** — mobile footer is two rows and a pill never splits | `8HD-0` is two flex children: row one `8HE-0` (blocked chip left, comment count hard right) and the wrapping chip row `8I2-0`. Each pill fits whole. |
| **R-H.0** — only three components change shape below the phone breakpoint | 07a exercises all three: the claim header (R-H.2), the footer strip (R-F.4), and — absent here — the blocked notice (R-H.1, not drawn because 07a has no banner). |
| **R-H.2** — status chip above the title on mobile | `8A8-0`: chip `8A9-0`, then title `8AF-0`, then id `8AG-0`; gaps 9px / 6px, exactly as R-H.2 measures them. |
| **R-I.0** — same content, same vocabulary, same order; only arrangement moves | Every count, label and sentence on `88A-0` is byte-identical to `47K-0`. |
| **R-I.1** — the blocker row's dependency path stacks, never drops | `8NT-0`: source slug on its own line (`8NU-0`), target slug below behind a leading chevron (`8NW-0`). |
| **R-I.2** — relationship row: four columns become two lines, badge right-ranged | `8SQ-0`, `8T6-0`, `8U8-0`: dot + title on line one, `module · facet` left and the lifecycle word right on line two, with `width: max-content` + `flexShrink: 0` rather than a fixed slot. |
| **R-I.3** — the chip is the component; the footer's rows are composition | The 07a mobile footer's two-row arrangement is composition; the chip specification is R-F.2's. |
| **R08.1 / R08.2** (board `O2-0`) | 07a introduces **no** severity vocabulary and **no** integrity verdict. It uses exactly two hues plus neutral — `--color-blocked` for blocked, `--color-draft` for draft — and no third hue. A lane agent must not import a severity pill onto this screen. |
| **R10.x** (board `1F7-0`) | **Does not bind.** 07a carries no right rail and therefore no freshness footer. See §6. |
| **R11.x** (board `1E3-0`) | Binds only through R11.2: the prose measure is 760px and must be **pixel-identical** in and out of focus mode. `480-0` `maxWidth: 760px` — the block that wraps the prose text node `481-0` — is that measure. |
| **R12.x** (board `2ML-0`) | Proposal, not implemented. 07a's checks expansion is the *no-embodiment* case and carries no snippet. Do not add one. |

### Every note on the band and the notes strip

**Band `931-0`.**
`933-0`: "07A · CLAIM — DRAFT, NOT YET APPROVED".
`934-0`: "four designs of one screen · light pair left, dark pair right".
*Intent:* the group is one screen at two widths in two modes, not four screens.
A value that differs between the light and dark member of a pair is a mode
difference; a value that differs between the desktop and mobile member is an
arrangement difference. Nothing else may differ.

**`96S-0` / `96T-0` — WHAT THIS SCREEN ANSWERS.**
Verbatim: "One question: what is still true of a claim nobody has approved yet,
and what is still missing. This is a spec board, not a device screen — a single
claim shown with all four footer expansions open at once, so the four states can
be read together. The mobile boards are therefore 390px fit-content columns with
no app bar, not 390 × 844 device frames. Draft and blocked are held apart the
same way at every width: draft is the amber lifecycle chip beside the title,
blocked is the red footer chip and the tinted readiness region, and the wording
never mixes them."
*Intent:* the screen answers one question and the layout is subordinate to it.
The four-open state is a reading device for the spec, not a product state. The
draft/blocked separation is the invariant a lane agent must preserve at every
width and in both modes.

**`96V-0` / `96W-0` — WHAT MOBILE DOES DIFFERENTLY.**
Paraphrased with its intent: nothing is dropped at 390 — all four expansions,
both closing footnotes, the dependency path, the module group header and its
count survive. **Nine** things change, in three kinds.
*Added:* the card loses its 12px radius and outline and goes full-bleed with top
and bottom hairlines only; the three neutral footer chips gain pill outlines
they do not carry on desktop. *Restructured into lines:* the footer becomes
three lines (Blocked with the comment count hard right, then the other three
chips wrapping); the blocker row becomes title → hop pill → path stacked with
the target slug behind a leading chevron; the blockers footnote puts "See in
claims graph" on its own line; the relationship rows become dot-and-title over
`module · facet` with the lifecycle word right-ranged instead of a fixed 58px
slot; the implementation-checks label column is dropped so DECLARATION and
REASON sit above their values. *Resized:* card gutters 32 → 16; lede 16/26 →
14/21 and its eyebrow 12 → 11; claim title 20 → 18; claim prose 17/28 → 16/26;
the mono slug 12 → 11; the sources and checks footnotes 15 → 14. *Re-weighted:*
relationship row titles 400 → 500 and the open footer chips 500 → 600; status
badges stay at 11px. "No word of the footer vocabulary changes, and its order is
unchanged."
*Intent:* the mobile board is the same document at a narrower measure. Weight
does on a phone what colour alone cannot — at 12px a colour shift is too quiet
to mark which door is open — and the outlines exist so a wrapped pill row reads
as separate tap targets rather than one run of text.

**`9XQ-0` — late disclosures (four, added after review).**
(1) The DRAFT status chip leaves the claim-title row and takes its own line
above the title. (2) The Blocked chip becomes a tinted pill "where desktop has
bare text and a dot", so **all four** footer chips change on mobile, not only
the three neutral ones. (3) "An open chip takes the accent border and accent
label the components board specifies, not only the heavier weight — every
expansion on this board is open, so the whole footer reads accent; in the
product only one door is ever open at a time." (4) The blockers section
compresses vertically as well as horizontally: 16/16 padding against desktop's
18/22, and a 22px run-in to the footnote against 34px.
*Intent:* (1) is R-H.2 applied to a draft chip — the title must run the full
measure. (2) and (3) are the mobile chip taking R-F.2's full specification, not
a weight-only approximation. (4) says the readiness region is the one section
that gets tighter on a phone, because it is the section a blocked reader is
there for and vertical budget is what a phone is short of.

**`96Y-0` / `96Z-0` — DRAFT IS ALWAYS AMBER — RESOLVED.**
Verbatim: "The approved desktop board set the two DRAFT badges under DEPENDS ON
and DEPENDED ON BY in the locked green while the claim's own DRAFT chip was
amber, so the same lifecycle word carried two colours on the one board whose job
is holding these states apart. The coordinator ruled that DRAFT is always amber.
All four boards now render every DRAFT badge in `--color-draft` on light and
`--color-dark-draft` on dark; genuine LOCKED badges keep the green. Draft is a
lifecycle state of this claim; blocked is about this claim's dependencies, and
the two never share a colour."
*Intent:* this is the screen's own record that **approval is not proof**. A
board that had already been approved carried a wrong colour on the one axis it
exists to teach. The rule that came out of it — one lifecycle word, one colour,
at every width and in both modes — outranks any pixel.
*Verified on the current boards:* `4A7-0` and `4AR-0` (desktop light) are
`var(--color-draft)`; `7Z5-0`/`7ZF-0`'s badges (desktop dark) are
`var(--color-dark-draft)`; `8TC-0`/`8UE-0` and `91T-0`/`925-0` likewise. The
governed-by target's `LOCKED` badge (`4BR-0`'s last child, `8SW-0`) remains
`--color-locked` / `--color-dark-locked`. **The fix is in place on all four
boards.**

### Contradictions between a board and its own notes

Recorded here as intent failures, listed as defects in §9:

- Note `9XQ-0`(3) says "An open chip takes the accent border and accent label …
  every expansion on this board is open, so the whole footer reads accent." On
  the **desktop light** board the `No checks declared` chip (`48J-0`) is
  `#54606F` at weight 400 while `3 relationships` (`48B-0`) and `No sources`
  (`48F-0`) are `--color-accent` at weight 500 — and its own section is open.
  Its chevron *is* accent. One of the four open chips does not read as open.
  Desktop dark (`7XA-0`) repeats the same error with `--color-dark-muted`.
- R-F.3 states the mobile comment count "takes `--color-accent-bg` with an
  `--color-accent` icon and label". Both mobile boards draw it faint and
  un-pilled (`8HN-0`/`8HP-0`, `8ZK-0`). Resolved in §9 on the grounds that the
  count here is **zero**.

---

## 3. Layout

### 1440 tier — read from `47K-0` / `7W2-0`

07a is drawn at **1220**, not 1440. Per `tokens.md` §8 the 1220-wide claim
boards (05, 06, 06a, 07, 07a) are where the claim column sits inside the
**narrow-desktop** tier, `@media (min-width: 861px) and (max-width: 1180px)`.
The board carries **no sidebar, no facet TOC and no right rail** — it is a spec
board (note `96T-0`), so §3 records the *claim column's* geometry, and the
rail/TOC widths are governed by group 02/03, not by 07a.

| Value | Measurement | Node |
|---|---|---|
| Board width | `1220px` | `47K-0` / `7W2-0` |
| Board height | `1345px` (`height: fit-content`) | `47K-0` |
| Board padding | `40px` top, `44px` bottom, `40px` inline | `47K-0` |
| Board gap (lede → card) | `24px` | `47K-0` |
| Board ground | `var(--color-paper)` / `var(--color-dark-paper)` | `47K-0` / `7W2-0` |
| Content column | `1140px` (1220 − 2 × 40) | `47L-0`, `47P-0` |
| Card outer width | `1140px`; card body `1138px` inside its 1px border | `47P-0`, `47Q-0` |
| Card inner gutter | `32px` inline on every section | `47Q-0`, `48R-0`, `49L-0`, `4AS-0`, `4B5-0` |
| Text column inside the card | `1074px` (1138 − 2 × 32) | `47R-0`, `482-0`, `48S-0`, `49M-0` |
| **Reading measure (prose)** | `760px` — `maxWidth: 760px` on the prose **block** `480-0`; the text node `481-0` inside it carries no `maxWidth` | `480-0` (measure), `481-0` (type) |
| Lede measure | `760px` — `width: 760px` | `47N-0` |
| Checks footnote measure | `760px` — `maxWidth: 760px` | `4BF-0` |
| Checks REASON sub-measure | `640px` — `maxWidth: 640px` | `4BE-0` |
| Footer strip height | `49px` | `482-0` |
| Footer strip inset | `marginLeft: 32px`, `marginRight: 32px`, `paddingBlock: 16px` (a hairline that stops short of the card edge, unlike every section rule below it, which is a full-bleed `border-top`) | `482-0` |
| Footer strip gap | `20px` between chips; `5px` inside each chip; `7px` inside the blocked chip; `6px` inside the comment group | `482-0`, `48A-0`, `483-0`, `48N-0` |
| Readiness section box | `1138 × 223`; padding `18px` top / `22px` bottom / `32px` inline; gap `12px` | `48R-0` |
| Relationships section box | `1138 × 308`; padding `20px` top / `24px` bottom / `32px` inline; gap `14px` | `49L-0` |
| Sources section box | `1138 × 134`; padding `20px` top / `24px` bottom / `32px` inline; gap `13px` | `4AS-0` |
| Checks section box | `1138 × 218`; padding `20px` top / `26px` bottom / `32px` inline; gap `12px` | `4B5-0` |
| Readiness row indent | blocker rows are full-width; the hop slot is a fixed `112px`, `justifyContent: end`, `alignItems: start` | `4CC-0` |
| Relationship row indent | `paddingLeft: 20px` for the governed-by block, `19px` for the depends-on / depended-on-by rows; the bind note adds a further `paddingLeft: 16px` | `4BQ-0`, `4A3-0`, `4AN-0`, `4BW-0` |
| Relationship badge slot | fixed `58px`, `textAlign: right`, `justifyContent: end`, `flexShrink: 0` | `4A7-0`, `4AR-0` |
| Checks label column | fixed `96px`, `flexShrink: 0`; `16px` gap to the value | `4BA-0`, `4BD-0` |

**Sticky and scroll regions: none.** Nothing on 07a is sticky, and the card does
not scroll internally. `overflow: clip` on `47K-0` and on the card `47P-0` is a
Paper containment artefact, not a scroll region. The page scrolls; the card does
not.

### 390 tier — read from `88A-0` / `8V2-0`

| Value | Measurement | Node |
|---|---|---|
| Board width | `390px` | `88A-0` / `8V2-0` |
| Board height | `1810px` (`height: fit-content`) | `88A-0` |
| Gutter | `16px` inline on every section | `8A8-0`, `8AI-0`, `8HD-0`, `8NB-0`, `8OV-0`, `8UG-0`, `8UN-0` |
| Content column | `358px` (390 − 2 × 16) | `8AE-0`, `8HD-0`, `8NC-0`, `8OW-0` |
| Card | full-bleed `390px`, **no radius**, `border-top` + `border-bottom` 1px only | `891-0` / `8Z7-0` |
| Spec caption block | padding `20px` top / `16px` bottom / `16px` inline, gap `6px` | `88X-0` |
| Claim head | padding `20px` top, `16px` inline; gap `9px` (chip → title block), `6px` (title → id) | `8A8-0`, `8AE-0` |
| Claim prose block | `paddingTop: 16px`, `16px` inline, no bottom padding (the footer's own `marginTop` carries the gap) | `8AI-0` |
| Footer strip | inset `marginLeft/Right: 16px`, `marginTop: 16px`; `paddingTop: 13px`, `paddingBottom: 16px`; `border-top` 1px; gap `8px` between the two rows | `8HD-0` |
| Footer row one | `358 × 30`, gap `10px`, spacer `flexGrow: 1; minWidth: 4px` | `8HE-0` |
| Footer chip row | `358 × 68` (two 30px lines + one 8px gap), `flexWrap: wrap`, gap `8px` | `8I2-0` |
| Chip widths | `127px` / `107px` / `157px` — 127 + 8 + 107 = 242 fits 358; the third wraps | `8I3-0`, `8I7-0`, `8IB-0` |
| Blockers section | `padding: 16px` on all four sides; heading run-in `paddingBottom: 12px`; footnote run-in `paddingTop: 12px` | `8NB-0` |
| Relationships section | `padding: 16px`, gap `12px`; row indent `paddingLeft: 19px`; sub-line indent a further `15px`; bind note `paddingLeft: 34px` | `8OV-0`, `8SQ-0`, `8SU-0`, `8SY-0` |
| Sources section | `padding: 16px`, gap `13px` | `8UG-0` |
| Checks section | `padding: 16px`, gap `12px`; label/value stacks gap `3px`; footnote `marginTop: 2px`, `paddingTop: 12px` | `8UN-0`, `8UR-0`, `8UY-0` |

**No bottom sheet appears on 07a.** The comment count is 0, so nothing opens a
sheet on this screen. Sheet policy (R-J.1 … R-J.7) is inherited unchanged from
group 14 the moment a 07a-shaped claim carries a thread; 07a introduces no
second shell and must not.

---

## 4. Components on this screen

**Measured-value count: no fewer than 180 distinct property values across 104
nodes on the desktop light board `47K-0`** (the 104 are `47K-0`, `47L-0` …
`4BG-0`, every node named in this section and in §6), **plus their dark twins on
`7W2-0` and the mobile forms on `88A-0` / `8V2-0`.** Every value below was read
with `get_jsx` (inline-styles) or `get_computed_styles`; none is an estimate and
none was taken from a screenshot.

Light hexes below are the Paper token's own value. Where a board spells a raw
hex instead of the token, the hex is printed with **(literal)** and the token a
lane agent must implement is named beside it — per `tokens.md`
Disagreement 12, *implement the token, never the literal*.

### 4.1 Card shell

| Property | Desktop light (`47P-0`) | Desktop dark (`7WH-0`) | Mobile light (`891-0`) | Mobile dark (`8Z7-0`) |
|---|---|---|---|---|
| Background | `#FFFFFF` **(literal)** → `--color-card` `#FFFFFF` | `var(--color-dark-card)` `#161B22` | `var(--color-card)` | `var(--color-dark-card)` |
| Border | `1px solid var(--color-border)` `#DDE2E9`, all four sides | `1px solid var(--color-dark-border)` `#242C38` | top + bottom 1px `--color-border`, **no side borders** | top + bottom 1px `--color-dark-border` |
| Radius | `12px` (off-scale literal) | `12px` | **none** | **none** |
| Overflow | `clip` | `clip` | `clip` | `clip` |

*Intent:* on desktop the claim is an object on a field — outline plus radius.
On a phone the field is only 16px wide on each side, so an outline and a radius
buy nothing and cost measure; the card becomes a band of the page separated by
two hairlines (note `96W-0`).

### 4.2 Claim head — title, id, DRAFT chip

| Element | Desktop light | Desktop dark | Mobile light | Mobile dark |
|---|---|---|---|---|
| **Head block** | `47Q-0`: padding `30px` top / `22px` bottom / `32px` inline, gap `16px`; title row `47R-0` `alignItems: start`, gap `16px` | `7WI-0` same | `8A8-0`: column, `alignItems: start`, padding `20px` top / `16px` inline, gap `9px` | `8Z8-0` same |
| **Title** | `47T-0`: Inter `20px` / `26px`, weight `600`, tracking `-0.01em`, `--color-ink` `#101720` | `7WJ-0`'s title: `--color-dark-ink` `#E6EAF0` | `8AF-0`: `18px` / `24px`, weight `600`, tracking `-0.01em`, `--color-ink` | `8ZE-0`'s title: `--color-dark-ink` |
| **Claim id** | `47U-0`: IBM Plex Mono `12px` / `16px`, weight 400, `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `8AG-0`: mono `11px` / `15px`, `--color-faint` | `--color-dark-faint` |
| **DRAFT chip box** | `4BI-0`: `75 × 22`; bg `var(--color-draft-bg)` `rgb(154 106 22 / 11%)`; radius `999px`; gap `5px`; padding `4px` block, `8px` left, `10px` right; `flexShrink: 0` | `7WJ-0`'s chip: bg `#DDA94E21` **(literal, = `--color-dark-draft-bg` 13%)**; same box | `8A9-0`: identical box, `var(--color-draft-bg)` | `8Z9-0`: `#DDA94E21` **(literal)** |
| **DRAFT padlock** | `4BJ-0`: SVG `12 × 12`, viewBox 24, `stroke: var(--color-draft)` `#9A6A16`, `strokeWidth 2.4`, `strokeLinecap round`, `fill none` | `stroke: var(--color-dark-draft)` `#DDA94E` | `8AA-0` same as desktop light | `stroke: var(--color-dark-draft)` |
| **DRAFT label** | `4BM-0`: Inter `11px` / `14px`, weight `600`, tracking `+0.05em`, `--color-draft` | `--color-dark-draft` | `8AD-0` identical | `--color-dark-draft` |

*Asymmetry to preserve:* the chip's padding is **8px left / 10px right** — the
padlock's own optical sidebearing is what pays for the difference. Do not
normalise it to a symmetric value.

*States:* only `DRAFT` is drawn on the claim head. The chip **never appears
without its padlock** (components board note `4D7-0`: "A word alone reads as a
label; the padlock is what makes it a state").

### 4.3 Claim prose

| Property | Desktop light (`481-0`) | Desktop dark (`7WT-0`) | Mobile light (`8AJ-0`) | Mobile dark (`8ZI-0`) |
|---|---|---|---|---|
| Family / size / leading | Source Serif 4 `17px` / `28px` | same | `16px` / `26px` | same |
| Weight / tracking | `400` / `0em` | `400` | `400` | `400` |
| Colour | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `--color-ink` | `--color-dark-ink` |
| Measure | `maxWidth: 760px` — **on the parent block `480-0`, not on the text node**; `481-0` itself carries no `maxWidth` | `maxWidth: 760px` on the parent block `7WS-0`; `7WT-0` carries none | `358px` (the gutter — `8AI-0`'s `paddingInline: 16px`; `8AJ-0` carries no `maxWidth`) | `358px` (`8ZH-0`'s gutter; `8ZI-0` carries none) |

### 4.4 Footer strip — desktop (bare-text form)

`482-0` / `7WU-0`. Height `49px`, gap `20px`, `paddingBlock: 16px`,
`marginInline: 32px`, `border-top: 1px solid #E8ECF1` **(literal → `--color-border`)**;
dark `#212934` **(literal → `--color-dark-border` `#242C38`)**.

| Chip | Parts and values (light) | Dark |
|---|---|---|
| **Blocked / readiness** `483-0` | dot `484-0` `7 × 7`, radius `999px`, `var(--color-blocked)` `#9E3B36` · label `485-0` "Blocked" Inter `13/16` weight `600` `--color-blocked` · count `486-0` "1 blocker" Inter `13/16` weight `400` `--color-faint` · chevron **up** `487-0` SVG `12 × 12`, `stroke #9E3B36` **(literal)**, `strokeWidth 2.4` · group gap `7px` | `7WV-0`: **three** of the four parts take `var(--color-dark-blocked)` `#EC8A83` — the `7 × 7` dot, the label "Blocked" (13/16 weight 600) and the chevron (`strokeWidth 2.4`). The count "1 blocker" (13/16 weight 400) takes `var(--color-dark-faint)` `#8494A8`, exactly mirroring the light board's faint count. **Do not paint the count red.** Group gap `7px`, as light. |
| **Divider** `489-0` | `1 × 12`, `var(--color-border)`; the **only** divider in the strip, and it sits only between the blocked chip and the first neutral chip | `7X1-0`: `var(--color-dark-border)` |
| **Relationships** `48A-0` | label `48B-0` "3 relationships" Inter `13/16` weight `500` `--color-accent` `#1C4E8C` · chevron up `48C-0` `12 × 12` `stroke #1C4E8C` **(literal)** `sw 2.4` · gap `5px` | `7X2-0`: `var(--color-dark-accent)` `#6AA6E8` |
| **Sources** `48E-0` | label `48F-0` "No sources" Inter `13/16` weight `500` `--color-accent` · chevron `48G-0` as above | `7X6-0`: `var(--color-dark-accent)` |
| **Checks** `48I-0` | label `48J-0` "No checks declared" Inter `13/16` weight **`400`**, colour **`#54606F`** (= `--color-muted`) · chevron `48K-0` `stroke #1C4E8C` accent · gap `5px` — **the open-state defect, §9** | `7XA-0`: label `var(--color-dark-muted)` weight 400, chevron `var(--color-dark-accent)` |
| **Spacer** `48M-0` | `flexGrow: 1` — this is what puts the comment count hard right (R-F.1) | `7XE-0` |
| **Comment count** `48N-0` | speech-bubble SVG `48O-0` `14 × 14`, `stroke #6E7C8E` **(literal → `--color-faint`)**, `strokeWidth 2`, `strokeLinecap round` · count `48Q-0` "0" Inter `13/16` weight 400 `--color-faint` · gap `6px` · **no pill, no ground, no border** | `7XF-0`: both `var(--color-dark-faint)` `#8494A8` |

**Chevron direction is the open/closed state.** Every chevron on this board is
`m18 15-6-6-6 6` (up) because every expansion is open. The closed form is the
mirrored down path; per R-F.2 the blocked variant's chevron takes the blocked
colour whether up or down, and an open neutral chip's chevron takes the accent.

### 4.5 Footer strip — mobile (pill form)

`8HD-0` / `8ZJ-0`. Two rows, gap `8px`; `border-top: 1px solid #E8ECF1`
(light, literal) / `#212934` (dark, literal).

| Element | Mobile light | Mobile dark |
|---|---|---|
| **Blocked pill** `8HF-0` / `8ZK-0`'s first child | `159 × 30`; bg `#9E3B3617` **(literal, 9% — the token `--color-blocked-bg` is 10%)**; radius `999px`; `paddingInline: 11px`; gap `7px`; **no border** | bg `#EC8A8321` **(literal, 13% = `--color-dark-blocked-bg`)**; same box |
| ·  dot `8HG-0` | `7 × 7`, radius `999px`, `var(--color-blocked)` | `var(--color-dark-blocked)` |
| ·  label `8HH-0` | "Blocked" Inter `12/16` weight `600` `--color-blocked` | `--color-dark-blocked` |
| ·  count `8HI-0` | "1 blocker" Inter `12/16` weight `400` **`--color-blocked`** (not faint — the whole pill takes the state colour) | `--color-dark-blocked` |
| ·  chevron `8HJ-0` | `11 × 11`, `stroke var(--color-blocked)`, `strokeWidth 2.6` | `var(--color-dark-blocked)` |
| **Spacer** `8HL-0` | `flexGrow: 1; height: 1px; minWidth: 4px` | same |
| **Comment count** `8HM-0` | icon `8HN-0` `14 × 14` `stroke var(--color-faint)` `sw 2`; count `8HP-0` "0" Inter `13/16` `--color-faint`; gap `6px`; **no pill** | `var(--color-dark-faint)` |
| **Open chips** `8I3-0` / `8I7-0` / `8IB-0` | height `30px`; bg `var(--color-card)`; border `1px solid var(--color-accent)`; radius `999px`; `paddingInline: 11px`; gap `6px`; label Inter `12/16` weight `600` `--color-accent`, `width: max-content`; chevron `11 × 11` `stroke var(--color-accent)` `sw 2.6` | bg `var(--color-dark-card)`; border `1px solid var(--color-dark-accent)`; label + chevron `var(--color-dark-accent)` |

*Intent (note `96W-0`, `9XQ-0`):* the outline is what makes a wrapped pill row
read as three separate tap targets; the weight step 500 → 600 is what makes
"open" legible at 12px where colour alone is too quiet.

### 4.6 Readiness expansion

| Element | Desktop light | Desktop dark | Mobile light | Mobile dark |
|---|---|---|---|---|
| **Section ground** | `48R-0` `#FBF7F7` **(no token — a ~4% blocked wash over card)** | `7XJ-0` `var(--color-dark-blocked-surface)` `#1D1618` | `8NB-0` `#FBF7F7` | `909-0` `var(--color-dark-blocked-surface)` |
| **Section top border** | `1px #E8ECF1` (neutral) | `1px #212934` (neutral) | `1px #EFE6E5` (blocked-tinted, **no token**) | `1px var(--color-dark-blocked-hairline)` `#3A2A2B` |
| **Eyebrow** | `48T-0` "READINESS BLOCKERS" Inter `11/14` weight `600` tracking `+0.08em`, `#9E3B36` **(literal → `--color-blocked`)** | `var(--color-dark-blocked)` | `8ND-0` `var(--color-blocked)` | `90A-0`'s label `var(--color-dark-blocked)` |
| **Eyebrow rule** | `48U-0` `838 × 1`, `#EBD9D8` **(no token — the heavier blocked hairline)** | `var(--color-dark-blocked-hairline)` | `8NE-0` `122 × 1` `#EBD9D8` | `var(--color-dark-blocked-hairline)` |
| **Eyebrow count** | `48V-0` "1 in 1 module" Inter `12/16` weight 400, `#6E7C8E` **(literal → `--color-faint`)** | `var(--color-dark-faint)` | `8NF-0` `var(--color-faint)`, `flexShrink: 0` | `var(--color-dark-faint)` |
| **Module group row** | `48X-0` `1074 × 43`; `border-top 1px #EBD9D8`; `paddingBlock: 11px`; gap `12px` | `border-top var(--color-dark-blocked-hairline)` | `8NG-0` gap `10px`, same paddings | `90E-0` same |
| ·  disclosure chevron | `4CG-0` `12 × 12`, `stroke var(--color-muted)`, `sw 2.4`, **down** (the group is collapsible and shown expanded) | `var(--color-dark-muted)` | `8NH-0` same | same |
| ·  module name | `490-0` "Capability support" Inter `14/18` weight `600` `#101720` **(literal → `--color-ink`)** | `var(--color-dark-ink)` | `8NJ-0` `var(--color-ink)` | `var(--color-dark-ink)` |
| ·  hop hint | `491-0` "nearest 1 hop" Inter `12/16` `#6E7C8E` | `var(--color-dark-faint)` | `8NK-0` `var(--color-faint)` | `var(--color-dark-faint)` |
| ·  count pill | `493-0` bg `#9E3B361A` **(literal, 10% = `--color-blocked-bg`)**, radius `999px`, padding `3px` / `9px`; `494-0` "1" Inter `11/14` weight `600` `#9E3B36` | `#EC8A8329` **(literal, 16% — above the 13% `--color-dark-blocked-bg`)**; label `var(--color-dark-blocked)` | `8NM-0` same as desktop light, plus `flexShrink: 0` | `#EC8A8329` |
| **Blocker row** | `4C2-0` `border-top 1px #EFE6E5` **(the lighter blocked hairline, no token)**; `paddingBlock: 10px`; gap `14px` | `var(--color-dark-blocked-hairline)` | `8NP-0` `border-top 1px #EFE6E5`; `paddingTop: 11px`, `paddingBottom: 10px`; column, gap `6px` | `90M-0` `var(--color-dark-blocked-hairline)` |
| ·  row title | `4C4-0` Inter `14/20` weight `600` `#101720` | `var(--color-dark-ink)` | `8NQ-0` identical | `var(--color-dark-ink)` |
| ·  hop pill | `4CD-0` bg `#EFF1F4` **(literal → `--color-paper`)**, radius `999px`, padding `3px` / `9px`, label Inter `11/14` weight `500` `#54606F`, `width: max-content`; in a fixed `112px` right-ranged slot `4CC-0` | bg `var(--color-dark-paper)`, label `var(--color-dark-muted)` | `8NR-0` same pill, `width: fit-content`, **no slot** — it sits on its own line under the title | same |
| ·  source slug chip | `4C5-0`'s first child: bg `#EFF1F4`, radius `4px` (`--radius-sm`), padding `2px` / `7px`, mono `11/14` `#54606F` | bg `var(--color-dark-paper)`, text `var(--color-dark-muted)` | `8NU-0` identical | same |
| ·  path chevron | `11 × 11`, `stroke #6E7C8E`, `strokeWidth 2.6`, path `m9 18 6-6-6-6` (right) | `var(--color-dark-faint)` | `8NW-0`'s SVG, **leading** the target slug, `marginTop: 3px` | same |
| ·  target slug chip | bg `#9E3B361A`, radius `4px`, padding `2px` / `7px`, mono `11/14` `#9E3B36` | bg `#EC8A8329`, text `var(--color-dark-blocked)` | `8NW-0`'s chip, identical | same |
| ·  path arrangement | one line: `4C5-0` `display: flex; flexWrap: wrap; gap: 6px`, `alignItems: center` | same | **stacked**: `8NT-0` column, gap `5px`, `alignItems: start` (R-I.1) | same |
| **Footnote** | `49D-0` gap `10px`, `paddingTop: 12px`; sentence `49E-0` Source Serif 4 `14/22` `#54606F` | `var(--color-dark-muted)` | `8OL-0` column, gap `10px`, `paddingTop: 12px`; `8OM-0` serif `14/22` `--color-muted` | `var(--color-dark-muted)` |
| ·  graph affordance | `49F-0` gap `6px`, `flexShrink: 0`, right-ranged on the same line; glyph `49G-0` `13 × 13` `stroke #1C4E8C` `sw 2` (a branch mark: one path + two 3px circles); label `49K-0` "See in claims graph" Inter `13/16` weight `500` `#1C4E8C` | `var(--color-dark-accent)` | `8ON-0` — **its own line below the sentence** | `90Y-0`'s action row |

*Intent:* the readiness region is the only part of the card that carries a
ground colour, and it carries a *blocked* ground rather than a *draft* one. That
is the draft/blocked separation made physical: the amber chip is about the
claim, the red region is about what the claim rests on.

### 4.7 Relationships expansion

| Element | Desktop light | Desktop dark | Mobile light | Mobile dark |
|---|---|---|---|---|
| **Section ground** | `49L-0` `#F6F8FA` **(no token; `--color-code-bg` is `#F3F5F8`)** | `7YI-0` `#1B212B` **(literal, = `--color-dark-code-bg`)** | `8OV-0` `#F6F8FA` | `916-0` `#1B212B` |
| **Section top border** | `1px #E8ECF1` | `1px #212934` | `1px #E8ECF1` | `1px #212934` |
| **Eyebrow** | `49N-0` "RELATIONSHIPS" Inter `11/14` weight `600` `+0.08em` `#6E7C8E` | `var(--color-dark-faint)` | `8OX-0` `var(--color-faint)` | `var(--color-dark-faint)` |
| **Eyebrow rule** | `49O-0` `944 × 1` `#E3E8EE` **(no token)** | `var(--color-dark-border)` | `8OY-0` `228 × 1` `#E3E8EE` | `var(--color-dark-border)` |
| **Eyebrow count** | `49P-0` "3" IBM Plex Mono `11/14` `#6E7C8E` | `var(--color-dark-faint)` | `8OZ-0` mono `11/14` `--color-faint` | `var(--color-dark-faint)` |
| **Direction heading** | `49Q-0` / `49Y-0` / `4AI-0`: gap `7px`; `paddingTop: 4px` on the 2nd and 3rd; arrow SVG `12 × 12` `stroke #6E7C8E` `sw 2.4` `strokeLinejoin round` — **up** `M12 19V5M5 12l7-7 7 7` for GOVERNED BY, **down** `M12 5v14M5 12l7 7 7-7` for DEPENDS ON, **right** `M5 12h14M12 5l7 7-7 7` for DEPENDED ON BY; label Inter `11/14` weight `600` tracking **`+0.06em`** `#6E7C8E`; count mono `11/14` `#6E7C8E` | all `var(--color-dark-faint)` | `8P0-0` / `8T1-0` / `8U3-0` identical | `91B-0` / `91O-0` / `920-0` |
| **Governed-by row** | `4BR-0`: gap `10px`; dot `6 × 6` radius `999px` `var(--color-locked)` `#2C6B52`; title Inter `14/21` weight **`400`** `var(--color-accent)`; meta "Curtainly · Doctrine" Inter `12/16` `var(--color-faint)`; badge "LOCKED" Inter `11/14` weight `600` tracking `+0.05em` `var(--color-locked)` | dot `var(--color-dark-locked)` `#63BE9A`; title `var(--color-dark-accent)`; badge `var(--color-dark-locked)` | `8SQ-0`: two lines. Dot `8SS-0` **`7 × 7`**, `marginTop: 6px`; title `8ST-0` `14/20` weight **`500`** `--color-accent`; sub-line `8SU-0` `paddingLeft: 15px`, `alignItems: baseline`, gap `8px`; meta `8SV-0` `12/16` `--color-faint`; badge `8SW-0` `11/14` weight `600` tracking **`+0.06em`** `--color-locked`, `width: max-content`, `flexShrink: 0` | `91F-0`: dark twins |
| **Bind note** | `4BW-0` `paddingLeft: 16px`; Source Serif 4 `13/21` `var(--color-muted)` | `var(--color-dark-muted)` | `8SY-0` `paddingLeft: 34px`; serif `13/21` `--color-muted` | `91M-0` |
| **Depends-on row** | `4A3-0`: `paddingLeft: 19px`; gap `10px`; dot `7 × 7` `#9E3B36`; title Inter `14/18` weight `400` `#1C4E8C`; meta "Capability support · Contract" Inter `12/16` `#6E7C8E`; badge "DRAFT" Inter `11/14` weight `600` `+0.05em` `var(--color-draft)` in a fixed `58px` right-ranged slot | `7Z5-0`: dot `var(--color-dark-blocked)`; title `var(--color-dark-accent)`; badge `var(--color-dark-draft)` | `8T6-0`: dot `8T8-0` `7 × 7` `--color-blocked`; title `8T9-0` `14/20` weight `500` `--color-accent`; meta `8TB-0` `12/16` `--color-faint`; badge `8TC-0` `11/14` weight `600` `+0.06em` `--color-draft`, **no fixed slot** (`width: max-content`, `flexShrink: 0`) | `91T-0` |
| **Depended-on-by row** | `4AN-0`: identical to `4A3-0` | `7ZF-0` | `8U8-0`: identical to `8T6-0` | `925-0` |

*The dot's meaning:* the dot carries the **target's** state, not the edge's.
Governed-by's target is locked → green. Both draft targets are drawn red on this
board because both are also *blocking* — the red dot is the blocked signal, the
`DRAFT` badge is the lifecycle word, and note `96Z-0` is the record that keeping
those two on separate colours took a correction.

### 4.8 Sources expansion

| Element | Desktop light | Desktop dark | Mobile light | Mobile dark |
|---|---|---|---|---|
| Section | `4AS-0` no ground; `border-top 1px #E8ECF1`; padding `20/24/32`; gap `13px` | `7ZK-0` `#212934` | `8UG-0` `padding: 16px`; gap `13px` | `92C-0` |
| Eyebrow | `4AU-0` "SOURCES" Inter `11/14` weight `600` `+0.08em` `#6E7C8E` | `var(--color-dark-faint)` | `8UI-0` | `92D-0` |
| Rule | `4AV-0` `984 × 1` `#E8ECF1` | `#212934` | `8UJ-0` `268 × 1` `#E8ECF1` | `#212934` |
| Count | `4AW-0` "0" mono `11/14` `#6E7C8E` | `var(--color-dark-faint)` | `8UK-0` | `92D-0`'s count |
| Body | `4BZ-0` `paddingTop: 10px`, `paddingBottom: 2px`; `4C0-0` Source Serif 4 **`15/25`** `var(--color-muted)` | `7ZP-0` `var(--color-dark-muted)` | `8UL-0` serif **`14/23`** `--color-muted`, no extra padding | `92H-0` |

*Intent:* a count of 0 still gets its own expansion and its own sentence. R09.5
says a citation is not a graph edge; an absent citation is still a claim about
the record and is written out, not omitted.

### 4.9 Implementation-checks expansion

| Element | Desktop light | Desktop dark | Mobile light | Mobile dark |
|---|---|---|---|---|
| Section | `4B5-0` no ground; `border-top 1px #E8ECF1`; padding `20/26/32`; gap `12px` | `7ZR-0` `#212934` | `8UN-0` `padding: 16px`; gap `12px` | `92I-0` |
| Eyebrow | `4B7-0` "IMPLEMENTATION CHECKS" Inter `11/14` weight `600` `+0.08em` `#6E7C8E` | `var(--color-dark-faint)` | `8UP-0` | `92J-0` |
| Rule | `4B8-0` `896 × 1` `#E8ECF1` — **no count on this heading**, unlike RELATIONSHIPS and SOURCES | `#212934` | `8UQ-0` `180 × 1` | `#212934` |
| DECLARATION label | `4BA-0` Inter `11/14` weight `600` tracking **`+0.07em`** `#6E7C8E`, fixed `96px`, `flexShrink: 0` | `var(--color-dark-faint)` | `8US-0` same type, **no width** — label sits above its value, stack gap `3px` | `92M-0` |
| DECLARATION value | `4BB-0` "none" IBM Plex Mono `12/20` `#54606F` | `var(--color-dark-muted)` | `8UT-0` mono `12/20` `--color-muted` | `92M-0`'s value |
| REASON label | `4BD-0` identical to `4BA-0` | `var(--color-dark-faint)` | `8UV-0` | `92P-0` |
| REASON value | `4BE-0` Source Serif 4 `14/22` `#54606F`, `maxWidth: 640px` | `var(--color-dark-muted)` | `8UW-0` serif `14/22` `--color-muted`, gutter measure | `92P-0`'s value |
| Footnote | `4BF-0` `border-top 1px #E8ECF1`, `marginTop: 4px`, `paddingTop: 4px`, `maxWidth: 760px`; `4BG-0` Source Serif 4 **`15/24`** `#54606F` | `801-0` `#212934`, `var(--color-dark-muted)` | `8UY-0` `border-top 1px #E8ECF1`, `marginTop: 2px`, `paddingTop: 12px`; `8UZ-0` serif **`14/23`** | `92S-0` |

*Tracking note:* the two checks labels are `+0.07em`, every other caps eyebrow
on the board is `+0.08em` and the direction headings are `+0.06em`. Three
trackings for caps labels on one card, none of which has a token
(`tokens.md` §3). Preserved as measured, flagged in §9.

### 4.10 States visible on the boards

Every state drawn across the four boards, and nothing else:

| State | Where | Treatment |
|---|---|---|
| Expansion **open** | all four chips, all four boards | Desktop: label accent weight 500 + chevron up accent. Mobile: card ground, 1px accent border, label accent weight 600, chevron up accent. |
| Expansion **blocked** (and open) | readiness chip | Desktop: 7px blocked dot, "Blocked" weight 600 blocked, count weight 400 faint, chevron up blocked, no ground. Mobile: tinted pill, count also blocked, no border. |
| Expansion **closed** | **not drawn on 07a** | Take it from R-F.2 and the components board. |
| Group row **expanded** | `48X-0` / `8NG-0` | Chevron **down** (`m6 9 6 6 6-6`) in `--color-muted`. The collapsed form is not drawn. |
| Claim status **draft** | `4BI-0` / `8A9-0` | Amber tinted pill with padlock. |
| Edge target **locked** | `4BR-0` / `8SQ-0` | Green dot + green `LOCKED` badge. |
| Edge target **draft** | `4A3-0`, `4AN-0` / `8T6-0`, `8U8-0` | Red dot (it is blocking) + amber `DRAFT` badge. |
| Comment chip **zero** | `48N-0` / `8HM-0` | Faint glyph + faint "0", no pill. |
| **Hover / focus / active / disabled / selected** | **not drawn on any of the four boards.** | Inherit from the components board and the engine; 07a specifies none of them. |

---

## 5. Mobile rules

Every rule below is a rule about **arrangement** and therefore belongs in
`@media (max-width: 520px)`, except where it says otherwise. **No 390px
breakpoint is added** (`tokens.md` §8, and reference-rules Open decisions).

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | Card loses its 12px radius and its side borders and goes full-bleed with a top and bottom hairline only (`891-0`) | `max-width: 520px` | Arrangement. Phone tier; 390 < 520. |
| M2 | Card gutters 32px → 16px on every section (`8A8-0`, `8NB-0`, `8OV-0`, `8UG-0`, `8UN-0`) | `max-width: 520px` | Arrangement. |
| M3 | DRAFT status chip leaves the title row and takes its own line above the title; order becomes status → title → id, gaps 9px / 6px (`8A8-0`) | `max-width: 520px` | R-H.2 verbatim; arrangement. |
| M4 | Claim title 20/26 → 18/24, id mono 12/16 → 11/15, prose 17/28 → 16/26 (`8AF-0`, `8AG-0`, `8AJ-0`) | `max-width: 520px` | Arrangement/measure, not target size. |
| M5 | Footer strip becomes two flex rows: row one blocked chip left + comment count hard right; row two the three neutral chips, wrapping (`8HD-0`, `8HE-0`, `8I2-0`) | `max-width: 520px`, **and the block must be written after `style.css:4658` and before the print block's doc comment, which opens at `style.css:4660`** | R-F.4. The existing `.claim-footer__counts` rules live at `style.css:4640` (inside `@media (max-width: 860px)`, opened `4533`, closed `4645`) and `style.css:4655` (inside `@media (max-width: 560px)`, opened `4647`, **closed `4658`**). A 520px rule carries equal specificity, so only source order can make it win — it must be the **last** screen block in the file. Insert it at `4659` (the blank line), i.e. after the 560 block's closing brace and **before** the comment `/* ---- print — THE @media print block ---` at `4660`. Inserting "after 4663" splits that comment in half and detaches it from the `@media print {` it documents at `4721` — the exact hazard the comment's own "IT MUST STAY LAST IN THE FILE" text warns about. |
| M6 | All four footer chips become pills — blocked tinted (no border), the other three card-ground with a 1px accent border when open — 30px tall, `--radius-pill`, 11px inline padding, labels 12/16 (`8HF-0`, `8I3-0`, `8I7-0`, `8IB-0`) | `max-width: 520px` for the form; **`(pointer: coarse)` for the ≥44px target** | R-F.2 gives the form; R-J/`tokens.md` §8 puts target size on pointer. **The 30px pill is not a 44px target** — the engine already sets `.comment-chip { min-height: 44px }` at `style.css:1764`; the four footer chips need the same treatment there, not at 520. |
| M7 | Closed chips run one weight step heavier than desktop (400 → 500) and open chips 500 → 600 (`8I4-0`, `8I8-0`, `8IC-0`) | `max-width: 520px` | Note `96W-0`: "At 12px on a phone the colour shift alone was too quiet to mark which door is open." Arrangement-adjacent, but it is a width fact, not a pointer fact. |
| M8 | Blocker row stacks: title → hop pill on its own line → path with the target slug on a second line behind a leading chevron (`8NP-0`, `8NR-0`, `8NT-0`) | `max-width: 520px` | R-I.1. The fixed 112px hop slot (`4CC-0`) is dropped, not narrowed. |
| M9 | Relationship rows become two lines with the lifecycle badge right-ranged via `width: max-content` + `flexShrink: 0`, replacing desktop's fixed 58px slot (`8SQ-0`, `8T6-0`, `8U8-0`) | `max-width: 520px` | R-I.2 verbatim: right-ranging is what lets `LOCKED` and `DRAFT` share a lane without wasting the width the title needs. |
| M10 | Relationship row title weight 400 → 500 and badge tracking `+0.05em` → `+0.06em` (`8ST-0`, `8SW-0`, `8TC-0`) | `max-width: 520px` | R-I.2's measured values. |
| M11 | Relationship dot moves from centre-aligned to `alignItems: start` with `marginTop: 6px`, and the governed-by dot grows 6px → 7px (`8SS-0`) | `max-width: 520px` | Two-line rows have no single baseline to centre on. (The 6px desktop dot is a defect — §9.) |
| M12 | Implementation-checks 96px label column is dropped; DECLARATION and REASON sit above their values, stack gap 3px (`8UR-0`, `8UU-0`) | `max-width: 520px` | 96px of a 358px gutter would leave the value column too narrow. |
| M13 | Readiness section compresses to `padding: 16px` all round (desktop `18px` top / `22px` bottom) and the footnote run-in drops 34px → 22px (`8NB-0`, note `9XQ-0`) | `max-width: 520px` | Note `9XQ-0`(4). |
| M14 | "See in claims graph" leaves the footnote's right edge and takes its own line under the sentence (`8ON-0`) | `max-width: 520px` | Note `96W-0`. |
| M15 | Sources and checks footnotes 15/25 and 15/24 → 14/23 (`8UL-0`, `8UZ-0`) | `max-width: 520px` | Note `96W-0`. |
| M16 | Readiness section's top hairline switches from the neutral card hairline to the blocked-tinted one (`8NB-0` `#EFE6E5`, `909-0` `--color-dark-blocked-hairline`) | `max-width: 520px` | On a full-bleed card the section boundary is the only thing separating the blocked region from the prose; the neutral rule reads as a page seam. |
| M17 | Every interactive target on the card — the four footer chips, the module group disclosure, the "See in claims graph" link, the comment count — needs a ≥44px hit area | **`(pointer: coarse)`** | `tokens.md` §8's rule of thumb: target size is a pointer question. Do **not** put this behind 520. |
| M18 | Hover-only affordances must have a resting state | **`(pointer: coarse)`** | 07a draws no hover state, but the engine reveals the zero-thread chip on card hover (`style.css:1649`) and pins it back at `style.css:1764`. A 07a claim carries exactly that zero chip. |

**Elements hidden at 390: none.** Note `96W-0` is explicit — "Nothing is
dropped". Every count, every label, both closing footnotes, the dependency path,
the module group header and its count all survive. A lane agent that drops
anything at 520 has broken this screen's rule.

**Bottom sheets: none on 07a.** The comment count is 0, so no sheet is reachable
from this screen. If a 07a-shaped claim gains a thread, the sheet is the one
shell from R-J.1–R-J.7 with the comment-thread body; 07a adds no shell, no
variant and no second sheet.

---

## 6. Footer vocabulary

### The metadata strip — desktop, in order (`482-0`)

> **Blocked**  1 blocker  ⌃   │   **3 relationships** ⌃   **No sources** ⌃   **No checks declared** ⌃   … 💬 **0**

Exact strings, in board order, with node ids:

1. `485-0` — "Blocked"
2. `486-0` — "1 blocker"
3. *(divider `489-0`)*
4. `48B-0` — "3 relationships"
5. `48F-0` — "No sources"
6. `48J-0` — "No checks declared"
7. *(spacer `48M-0`, which is what makes the count hard right)*
8. `48Q-0` — "0"

### The metadata strip — mobile, in order (`8HD-0`)

Row one: `8HH-0` "Blocked" · `8HI-0` "1 blocker" … `8HP-0` "0"
Row two: `8I4-0` "3 relationships" · `8I8-0` "No sources" · `8IC-0` "No checks declared"

**The words and their order are identical at both widths** (note `96W-0`: "No
word of the footer vocabulary changes, and its order is unchanged"). Only the
line breaks move.

### Section headings and counts

| String | Node (desktop / mobile) |
|---|---|
| "READINESS BLOCKERS" · "1 in 1 module" | `48T-0` / `48V-0`; `8ND-0` / `8NF-0` |
| "RELATIONSHIPS" · "3" | `49N-0` / `49P-0`; `8OX-0` / `8OZ-0` |
| "GOVERNED BY" *(no count)* | `49T-0`; `8P3-0` |
| "DEPENDS ON" · "1" | `4A1-0` / `4A2-0`; `8T4-0` / `8T5-0` |
| "DEPENDED ON BY" · "1" | `4AL-0` / `4AM-0`; `8U6-0` / `8U7-0` |
| "SOURCES" · "0" | `4AU-0` / `4AW-0`; `8UI-0` / `8UK-0` |
| "IMPLEMENTATION CHECKS" *(no count)* | `4B7-0`; `8UP-0` |
| "DECLARATION" · "none" | `4BA-0` / `4BB-0`; `8US-0` / `8UT-0` |
| "REASON" | `4BD-0`; `8UV-0` |

### Readiness row vocabulary

- `490-0` / `8NJ-0` — "Capability support" *(the module name, not a slug)*
- `491-0` / `8NK-0` — "nearest 1 hop"
- `494-0` / `8NN-0` — "1" *(the count pill)*
- `4C4-0` / `8NQ-0` — "The five verdicts is not locally approved"
- `4CD-0` / `8NS-0` — "direct · 1 hop"
- path chips — "capabilities-own-their-dimensions" › "the-five-verdicts"
- `49E-0` / `8OM-0` — "The only blocker sits in this module. One approval there clears this claim."
- `49K-0` / `8OS-0` — "See in claims graph"

### Lifecycle badges

`4BR-0`'s badge / `8SW-0` — "LOCKED".
`4A7-0`, `4AR-0` / `8TC-0`, `8UE-0` — "DRAFT".

### Closing footnotes, quoted

Sources (`4C0-0`, `8UL-0`, `7ZP-0`, `92H-0`):

> "None, and deliberately so. This is a division of responsibility inside one
> product, not a reading of anything outside it — what a reviewer is approving
> here is the direction of ownership."

Checks (`4BG-0`, `8UZ-0`, `801-0`, `92S-0`):

> "No checks declared is not a gap here. A claim can only be checked against
> code once it says something a reading of the code could contradict, and this
> one does not yet."

### Board furniture — do NOT implement

`47M-0` / `7WF-0` / `88Y-0` / `8Z5-0` — "FULL CLAIM · DRAFT · NO SOURCE BY
DESIGN · NO EMBODIMENT", and the lede `47N-0` and its three twins. These are the
spec board's own caption, not viewer chrome.

### Freshness / elapsed time

**07a carries no evidence footer, no freshness line and no elapsed time.** There
is no right rail on this board. R10.1–R10.5 bind groups 02, 03, 13 and 14 and do
not bind this screen. A lane agent must not invent a "Updated N hours ago" line
on a claim card.

---

## 7. Code address

All paths are relative to
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

> **Every line number below is read from the pinned commit `3ac8844`, not from
> the working tree.** At the time this spec was written another lane was editing
> that tree concurrently — `git status` showed `style.css` +202 lines,
> `graph.css` +79, `internal/render/geist_fonts.go` deleted in favour of
> `internal/render/engine_fonts.go`, and Inter / IBM Plex Mono / Source Serif 4
> `.woff2` files added under `viewer/template/fonts/`. Numbers had already
> shifted by ~88 lines in `style.css` while this was being written. **Verify
> every address with `git show 3ac8844:<path>` or by the quoted marker text, not
> by line number alone.** The marker comment or opening selector is given beside
> each address for exactly that reason.
>
> That in-flight change is directly relevant: it appears to be closing
> `tokens.md` Disagreement 6 (the engine has no serif family). If
> `engine_fonts.go` has landed by the time this screen is implemented, the
> Source Serif 4 values in §4 are shippable rather than aspirational.

### 7.1 `internal/render/viewer/template/style.css`

| Region | Lines | Marker / opening selector |
|---|---|---|
| The palette | `style.css:49` | `/* THE palette. One unconditional :root, light-first: the values below are what` |
| Claim card shell | `style.css:227-233` | `.claim { border: 1px solid var(--border); border-radius: 8px; padding: 1rem 1.25rem; … }` — **8px radius against the board's 12px** |
| `status-locked` + the comment that routes draft elsewhere | `style.css:234-243` | `/* status-draft/status-locked map to the pill family below: locked -> accent,` |
| Claim body prose | `style.css:245-249`, re-declared LIVE at `style.css:3980-3987` | `.claim-body { font-size: 13px; … }` then `.claim-body, .sbody, .claim-list-items, .claim-table-scroll td { color: var(--ink); font-size: 14.5px; line-height: 1.68 }` — **14.5px/1.68 against the board's serif 17/28** |
| Claim head `.k` | `style.css:2292-2309`, re-declared `style.css:3863-3921` | `.k { … }`; `.k > .label`; `.k > .claim-comments-slot`; `.claim-collapse-toggle`; `.claim-collapse-chevron` |
| Status pill, dead layer | `style.css:2311-2330` | `.pill { display: inline-block; font-size: 11px; padding: 2px 9px; border-radius: 20px; }`, `.pill.ps`, `.pill.pw` |
| **Status pill, LIVE layer** | `style.css:3952-3978` | `.pill {` opens at `3952` and closes at `3960`; the whole live pill layer, including `.pill.ps`/`.status-locked` (`3962-3966`), `.pill.pv`/`.status-draft` (`3968-3972`) and `.pill.pw`/`.claim-review-pending` (`3974-3978`), runs `3952-3978`. Marker: `.pill { margin: 0; padding: 4px 8px; border-radius: 3px; font: 700 9px/1.2 var(--font-mono); … }`. The one live DRAFT rule is at `style.css:3968-3971` — `.pill.pv, .status-draft { background: var(--status-draft-bg, rgba(151,102,0,.12)); color: var(--status-draft, #976600); }`. **Do not read the layer as ending at `3981`**: `3980` is already the first selector of the next rule (`.claim-body, .sbody, …`), cited separately in this table as `3980-3987`. |
| Claim footer wrapper, first layer | `style.css:789-805` | `/* .claim-links is the <details> that wraps the whole edges footer` |
| Claim footer wrapper, LIVE layer | `style.css:4020-4026` | `.claim-links { margin-top: 17px; border: 1px solid var(--border); border-radius: 8px; background: color-mix(in srgb, var(--paper) 55%, var(--card-bg)); overflow: hidden; }` |
| Footer summary | `style.css:1184-1201`, LIVE `style.css:4028-4046` | `/* The summary is the disclosure's only click target and reads as a mono count strip` |
| Footer identity + counts | `style.css:4048-4063` | `.claim-footer__identity`, `.claim-footer__title`, `.claim-footer__counts`, `.claim-footer__chevron` |
| Footer chevron rotation | `style.css:3288-3305` | `.claim-footer__chevron { … }` / `.claim-links[open] .claim-footer__chevron` |
| Edges list | `style.css:1251-1295`; LIVE `style.css:4065-4101` | `/* .claim-edges is a <ul> (see components.EdgesHTMLWithLinks): each` (comment opens at `1251`, `.claim-edges {` at `1259`) |
| Edge row marker + ref styling | `style.css:1276-1349` | `.claim-edges > li::before`, `.claim-ref-prefix`, `.claim-ref-label`, `.claim-ref-raw` |
| **Readiness panel** | `style.css:807-1158` | `/* Claim readiness is a reviewer-facing work queue. The policy engine remains` — `.claim-readiness` `810`, `-head` `818`, `-summary` `863`, `-counts` `869`, `-count` `875`, `-label` `894`, `-section-head` `905`, `-routes` `931`, `-route` `938`, `-route > summary` `945`, `-route-id` `966`, `-route-via` `989`, `-route-count` `998`, `-chevron` `1004`, `-blockers` `1021`, `-blocker` `1027`, `-relation` `1049`, `-path` `1053`, `-trace` `1072`, `-map` `1076`, `-raw` `1136` |
| Readiness responsive | `style.css:1160-1173` (`@media (max-width: 860px)`), `style.css:1175-1182` (`@media (max-width: 520px)`) | the only two existing readiness breakpoints. `1174` is the blank line between them; the 520 block's closing brace is `1182` and the next doc comment opens at `1184`. |
| **Sources row** | `style.css:1350-1554` | `/* ---- the sources row (components/sources.go) -------------------------` — `.claim-source-list` `1359`, `.claim-source` `1365`, `::before` `1376`, `-ref` `1389`, `-title` `1396`, `-anchor` `1414`, `-hash` `1423`, `-meta` `1429`, `-note` `1440`, `-note-toggle` `1482`, `:target` `1542` |
| Comment chip | `style.css:1602-1680`, LIVE `style.css:4136-4143` | `.comment-chip`, `--open` `1617`, `--empty` `1643`, `-count` `1672` |
| Comment chip reveal-on-hover | `style.css:1649-1674` | `/* Reveal-on-claim-hover. The .claim-tree duplicates are REQUIRED, not` |
| Comment chip touch target | `style.css:1764-1772` | `@media (pointer: coarse) { .comment-chip { min-height: 44px } … }` |
| Content column | `style.css:2784`, LIVE `style.css:3369-3376` | `.content-area { … padding: 0 282px 80px 42px; }` |
| Facet TOC | `style.css:3668-3679` | `.facet-toc { position: fixed; … width: 236px; }` |
| Sidebar | `style.css:3076-3082` | `.sidebar { width: var(--system-record-sidebar-width, 270px); min-width: 220px; max-width: 420px; }` |
| Widest-desktop tier | `style.css:3378-3380`, `style.css:3391-3393` | `@media (min-width: 1181px)` / `@media screen and (min-width: 1181px)` |
| **Narrow-desktop tier — the tier 07a's 1220 board sits in** | `style.css:4494-4531` | `@media (min-width: 861px) and (max-width: 1180px) { .content-area { padding: 0 34px 76px; } … }` |
| Tablet-and-below tier | `style.css:4533-4645` | `@media (max-width: 860px)`; the footer rules are at `4638-4641` |
| 560 tier | `style.css:4647-4658` | `@media (max-width: 560px) { .claim-links-summary { display: grid; … } .claim-footer__counts { … } }` — opens at `4647`, closes at `4658`. `4659` is blank and `4660` is where the print block's doc comment begins, so nothing of this block lives past `4658`. |
| Print block (must stay last) | marker `style.css:4660`; `@media print {` opens at `style.css:4721`; file ends at `4782` | `/* ---- print — THE @media print block ------------------------------------` — "There is exactly one @media print block in this stylesheet and this is it" |

### 7.2 `internal/render/viewer/template/graph.css`

**No range of `graph.css` binds this screen.** 07a uses no facet ramp, no cycle
colour, no halo and no governance-edge colour; its dots are lifecycle colours
from `style.css`. The file's dark-first convention — `graph.css:161` (`:root {`)
holding the **dark** values, light restored at `graph.css:208`
(`@media (prefers-color-scheme: light), print {`), stated in its own header at
`graph.css:16` — is recorded here only so a lane agent does not reach into it
for `--color-blocked` or `--color-locked`. (`graph.css` is one of the files the
concurrent lane is editing; these numbers are from `3ac8844`.)

### 7.3 Runtime JavaScript

| Behaviour | Address |
|---|---|
| Readiness panel construction (the whole expansion) | `internal/render/viewer/template/viewer-runtime.js:1516` — `function renderClaimReadiness(assessments)`, with the doc comment at `:1511-1515` |
| Readiness header, state word and summary sentence | `viewer-runtime.js:1533-1550` |
| Readiness blocker count ("1 blocker") | `viewer-runtime.js:1552-1557` |
| "Readiness blockers" section label + "N shown route(s)" | `viewer-runtime.js:1587-1589` |
| Route group summary — **the string R09.8 cuts** | `viewer-runtime.js:1464` — `'shown via this dependency'` |
| Route chevron | `viewer-runtime.js:1466` |
| Blocker `<li>` and its copy | `viewer-runtime.js:1445-1452` |
| "Show representative path" disclosure | `viewer-runtime.js:1437-1441` |
| Raw diagnostics disclosure — **R09.8 demotes this** | `viewer-runtime.js:1493` — `function readinessRawDiagnostics(assessment)` |
| Conformance "not ready" open-state sweep | `viewer-runtime.js:1265` — `.claim-conformance[data-implementation-ready="false"]` |
| Comment chip count sync (`0` / `N`) | `viewer-runtime.js:474` (`updateChips`); empty-chip reveal `viewer-runtime.js:513` (`syncEmptyChips`); recompute `:522` |
| **Footer rewrite into `.claim-footer__*`** | `internal/render/viewer/template/system-record.js:122` — `function enhanceFooters()`; the count nouns are emitted at `:146-149` (`count(relationships, 'relationship')`, `count(sources, 'source')`, `count(files, 'file')`, `count(drifted, 'drifted', 'claim-footer__drifted')`) |
| Edge field re-labelling ("Governed By", "Depended On By", …) | `system-record.js:158` — `function enhanceFieldLabels()` |
| Graph pane (the destination of "See in claims graph") | `internal/render/viewer/template/graph-ui.js:345` and `:1178` (`'Claims graph'`), `graph-ui.js:3588` (the graph→reading-view link, `data-dxg-open-claim`). **There is no reading-view→graph per-claim affordance today** — the only entry point is the global button. |
| Global graph trigger | `internal/render/viewer/template/shell.html:73` — `<button id="dxgOpen" type="button" data-dxg-open …> Claims graph</button>` |
| Graph pane mount point | `shell.html:299` — `<section id="dxgPane" hidden></section>` |
| Comments rail (the desktop counterpart of the sheet) | `shell.html:243-249` |
| Build order UI | `internal/render/viewer/template/build-order-ui.js` — **not on this screen**; listed only to record that 07a touches none of it |

### 7.4 Go emitters — `internal/render`

| Thing | Address |
|---|---|
| Claim head, status pill, comment chip (the `.k` line) | `internal/render/components/card.html:39`; identical lines at `components/list.html:34`, `components/tree.html:35`, `components/steps.html:28`, `components/table.html:46` |
| Pill class selection (`pw` / `ps` / `pv`) | `internal/render/components/components.go:251-260` |
| Status word ("Draft" / "Locked" / "Review pending") | `components/components.go:262-278` |
| Footer `<details class="claim-links">` + `<ul class="claim-edges">` | `components/components.go:378` (`EdgesHTMLWithLinks`); summary assembled at `:554` (`summary := countSegment(links, "link") + " - " + countSegment(len(files), "file")`), sources segment `:562`, drifted segment `:565`, `<details>` emitted at `:569` |
| Footer count nouns ("1 link", "0 files", "N sources", "N drifted") | `components/components.go:608` (`countSegment`) |
| Comment chip HTML and its three variants | `components/components.go:659` (`CommentChipHTML`); the `hidden` slot at `:662`, `comment-chip--empty` at `:663`, `--resolved` at `:668` |
| Sources row | `internal/render/components/sources.go:120` (`writeSourcesRow`), external `:162`, internal `:208`, note `:269` |
| **Implementation checks — the `declared_none` branch this screen shows** | `internal/render/components/conformance.go:16` (`ConformanceHTML`); the `declared_none` data attribute at `:24`; the summary head at `:31`; the no-embodiment branch at `:39`, which emits exactly `declaration: none` (`:41`) and `reason: <Reason>` (`:42`) via `writeConformanceLine` at `:131` |
| Conformance CSS (injected only when a viewer has structured conformance) | `internal/render/conformance_view.go:7-75`; `.claim-conformance` `:9`, `-head` `:13`, `-scope` `:15`, `-check` `:16`, `-check-head` `:19`, `-line` `:20-22` |
| Conformance block inserted inside the claim root | `internal/render/render.go:803` (`renderClaimsWithBudget`); the insertion at `:818` |
| Readiness payload plumbed to the shell | `internal/render/render.go:879`, `:883`, `:919` (`HasReadinessMaps`) |
| Theme token allowlist (28 keys, order load-bearing) | `internal/config/config.go:154-159` (`ThemeTokenAllowlist`) |
| Token-consumer assertions | `internal/render/theme_tokens_test.go:549-550` — `"status-draft": {{".pill.pv, .status-draft", "color"}}` |
| Engine-owned inlined font faces — the precedent for a serif | `internal/render/geist_fonts.go` at `3ac8844`; **being replaced in the working tree by `internal/render/engine_fonts.go`** (see the note at the head of §7) |

### 7.5 viewer-tests that assert on these selectors today

| Test | Address | What it pins |
|---|---|---|
| `TestReadinessBrowserScaleBudgets` | `viewer-tests/claim_readiness_test.go:98`; selector counts at `:82-92`; per-card wait at `:117` | `.claim-readiness`, `.claim-readiness-blocker`, `.claim-readiness-route`, `.claim-readiness-map` |
| `TestStaticReadinessGroupsCompleteFactsAndRendersFocusedMapOnDemand` | `claim_readiness_test.go:341` | the route grouping this screen's module group row replaces |
| `TestReadinessMapCapNeverCapsTheAuthoritativeList` | `claim_readiness_test.go:278` | the map cap — R09.9 removes the map, so this test's subject moves |
| `TestLiveReadinessRefreshesAfterAnUpstreamApproval` | `claim_readiness_test.go:431` | the exact transition this claim is one approval away from |
| `TestStatusStripGroupsBlockersAndStaysCollapsed` | `viewer-tests/component_fit_test.go:67`, waiting on `.claim-readiness` at `:72` | readiness mounting inside a claim |
| `TestReadyConformanceStaysInsideCollapsedClaim` | `component_fit_test.go:98-118` | `.claim-conformance` starts closed and stays inside the collapse |
| `TestPhone390SoftMountSmoke` | `component_fit_test.go:163` | the 390 tier |
| `TestClaimDeepLinkRevealsCollapsedContent` | `viewer-tests/claim_collapse_test.go:125` | `details.claim-links` reveal-on-`:target` |
| `TestIndividualClaimCanCollapseAndExpand` | `claim_collapse_test.go:79` | `.k` / `.claim-collapse-toggle` |
| Footer reveal + print | `viewer-tests/reveal_test.go:98-99` (`.claim:target .claim-links::details-content`, `@media print { details.claim-links::details-content }`), `:149`, `:178`, `:260`, `:439`, `:504` | the footer disclosure contract |
| Source-note clamp (forces every footer open) | `viewer-tests/source_note_clamp_test.go:242`, `:304`, `:466`, `:482`, `:561` | `details.claim-links` open-all |
| Theme parity — draft pill and footer summary | `viewer-tests/theme_parity_test.go:125` (`{"08 status-draft", []string{".status-draft"}, …}`), `:126` (`{"09 draft pill", []string{".pill.pv"}, …}`), `:128` (locked/warn pills), `:161` (`.claim-links-summary` in the hover set), fixture markup at `:201-220`, suppression list at `:1275` | every colour this screen's chips take, in both modes |
| Zero-thread chip | `viewer-tests/empty_chip_test.go:33`, `:92`, `:109` (`TestFileURLHidesEmptyChips`), `:154` | the "0" this screen draws |
| Conformance panel states | `viewer-tests/conformance_test.go:105-155` (`func TestConformancePanelVisibleStaticAndRefreshesWhenServed`; `106` is its first body line) | `.claim-conformance[data-implementation-ready]` and per-check state |

### 7.6 The gap this screen opens

Recorded so a lane agent is not surprised. Today the claim footer's summary is a
mono count strip — `"1 link - 2 files - 1 source - 1 drifted"`
(`components/components.go:554-567`) — rewritten at runtime into "Evidence &
relationships" plus `relationship` / `source` / `file` / `drifted` count chips
(`system-record.js:134-151`). **Board 07a's vocabulary — Blocked / relationships
/ sources / checks, each a noun and a count, with the comment count hard right —
does not exist in the engine at any address.** Neither does the per-claim
"See in claims graph" affordance. Both are new surface, not restyling.

---

## 8. States not on the boards

07a draws exactly one state combination. Everything below is a state the engine
can render for this screen that the boards do not depict. For each, the spec
implied by the rules — a lane is not left guessing.

| State | Engine origin | What this spec implies |
|---|---|---|
| **Locked claim** | `pillClass` → `ps`, `StatusLabel` → "Locked" (`components.go:251`, `:262`) | The head chip takes `--color-locked` / `--color-locked-bg` with the same padlock, box, type and tracking as the DRAFT chip (`4BI-0`). Nothing else on the card changes. Draft is not a lesser locked, and locked is not a richer draft. |
| **Locked + review_pending** | `pillClass` → `pw`, `StatusLabel` → "Review pending" (`components.go:252-254`) | Head chip takes `--color-blocked` / `--color-blocked-bg`. This is the **one** place on a claim card where blocked-red is allowed to describe the claim itself rather than its dependencies, and it is why the word is "Review pending" and not "Blocked". It must never be amber; R08.1 forbids a third hue, so it reuses the blocked pair. |
| **review_pending trigger variants** (the three triggers in `dossierx-claims`) | `assessment.review_causes` → `viewer-runtime.js:1540`, `:1546-1547` | All three render into the **same** chip and the same readiness expansion. The trigger is a *cause row*, not a chip variant. The readiness state word becomes "Review required" (`viewer-runtime.js:1539`) and the causes list under `READINESS BLOCKERS`; the eyebrow count string becomes "N in M modules" on the same rule. |
| **Draft and not blocked** | readiness `assessment.ready === true` | The first footer chip is **not** the blocked variant. It takes the closed/open neutral form (R-F.2), with a noun and a count and no dot, no red, no tinted region. The readiness expansion must **not** auto-open (R09.3 — auto-open is earned only by blocked). The claim head chip stays amber. This is the most common draft claim in a healthy project and it is the state 07a does not show. |
| **Blocked across several modules** | `groups.length > 1` (`viewer-runtime.js:1577-1591`) | The eyebrow count pluralises ("N in M modules", `48V-0`'s slot) and the section carries several module group rows (`48X-0`'s shape), each with its own `nearest N hops` and its own count pill. Only the nearest module is opened; the rest stay one-line rows. This is board 06's subject — take the shape from there, unchanged. |
| **Blocked at more than one hop** | `record.path` length > 2 | The hop pill (`4CD-0`) reads `indirect · N hops` on the same `direct · 1 hop` pattern; the path chips (`4C5-0`) carry the **representative** path only. Per R09.9 the path is a breadcrumb, never a tree, and must not grow a second dimension. |
| **A readiness cycle** | `scene.cycleIds` in the graph; readiness records with a repeating id | 07a draws no cycle. R09.9 is the governing rule: a duplicated node in an inline tree is a false statement about the project, so a cycle is still a **flat breadcrumb** here and the graph pane is where its shape is read. The blocker row's wording does not change; the "See in claims graph" affordance is what carries a reader with a cycle question. |
| **Zero-thread comment chip, static export** | `CommentChipHTML` ships the slot with a bare `hidden` (`components.go:662`); `shell.html` reveals it only after `/api/ping` succeeds; `TestFileURLHidesEmptyChips` (`empty_chip_test.go:109`) | 07a draws the "0" **visible** (`48Q-0`). That depicts the **served** case. On a `file://` export the whole comment group (`48N-0`) is absent, the spacer `48M-0` still runs to the card's right edge, and the strip must not reflow the four chips to fill the gap. |
| **N > 0 comment threads** | `comment-chip--open` / `--resolved` (`components.go:672-680`) | Desktop: the count leaves `--color-faint` and takes the chip's state colour, staying hard right with the same 14px glyph and 13/16 type. Mobile: R-F.3 applies — an `--color-accent-bg` ground with an `--color-accent` glyph and label at weight 600, because at that point it is the control that opens the sheet. See §9 for why 07a's zero-count chip stays faint. |
| **Sources present** | `len(c.Sources) > 0` (`components.go:563`) | The `SOURCES` count stops being `0` and the prose reason `4C0-0` is replaced — not supplemented — by the source rows. The footer chip's word changes from "No sources" to "N sources". A claim never shows both a source list and a "no sources" reason. |
| **Checks that actually run** | `result.Mode != EmbodimentModeNone` (`conformance.go:46-50`) | The `DECLARATION` / `REASON` pair (`4B9-0`, `4BC-0`) is replaced by per-check `<article>` rows (`conformance.go:55`). The footer chip's word changes from "No checks declared" to a noun and a count. A **failing** check makes the claim's readiness blocked through a different route than a dependency — the readiness region's ground stays the same red wash; do not introduce a second tint for a check failure (R08.1). |
| **Drifted implemented-in file** | `drifted > 0` (`components.go:565`); the server writes a bare `open` on that claim's footer | 07a has no drifted segment. The board gives no chip for it. Until a screen specifies one, keep the engine's existing `claim-footer__drifted` count chip (`system-record.js:149`) and do **not** fold "drifted" into the four-chip vocabulary — it is an adjective about files, not a fifth noun. |
| **Long claim title** | any | The title block is `flexGrow: 1` with `flexBasis: 0%` (`47S-0`) and the DRAFT chip is `flexShrink: 0`, so on desktop a long title wraps within `983px` and the chip holds its lane. On mobile the chip is already on its own line (R-H.2), so the title takes the full `358px`. Never truncate a claim title. |
| **Long claim id** | any | `47U-0` / `8AG-0` are mono and the board's parent sets `overflowWrap: anywhere` at the artboard (`47K-0`). The id wraps; it is never truncated and never ellipsised — an id a reader cannot copy in full is useless (`components.go`'s `ClaimLabel` rationale in `card.html:32-37`). |
| **Long module · facet meta** | any | Desktop: `4A6-0` sits between a `flexGrow: 1` title and a fixed `58px` badge slot, so the title yields first. Mobile: the meta is `flexGrow: 1` on the sub-line with the badge `flexShrink: 0` at `width: max-content` (R-I.2) — the meta wraps, the badge never does. |
| **Layouts other than `card`** | `list.html`, `tree.html`, `steps.html`, `table.html`, `banner.html` | The head line (`.k`) and the footer are identical in all five — `banner.html` is the exception that emits neither a chip nor a footer. 07a's chrome therefore applies to every layout except banner, and a banner claim has no 07a surface at all. |
| **Focus mode** | R11.x, group 03 | Focus removes the rails and grows the page to 1140px. The claim prose column stays at **760px, pixel-identical** (R11.2). The readiness path chips, the relationship rows and the checks REASON get the freed width (R11.3). The card's 32px gutters and 12px radius do not change. |
| **Print** | `style.css:4721` | Print uses the light palette always (`tokens.md` §1). The footer disclosure is revealed in print (`reveal_test.go:99`). Nothing on 07a is colour-only: every state carries a word ("Blocked", "DRAFT", "LOCKED") as well as a hue, so the card survives greyscale. |

---

## 9. Open decisions

Decided here rather than asked, per the freeze protocol.

1. **Desktop footer chips stay bare text; the pill form is the mobile form.**
   R-F.2 measures all three chip variants at 30px with `--radius-pill` and 11px
   inline padding, and its evidence (`7N2-0` section I3) is the *mobile*
   expansion-forms section, whose intent quote is explicitly about "12px on a
   phone". Three independent notes agree that desktop has no pill — `9XQ-0`
   ("the Blocked chip becomes a tinted pill where desktop has bare text and a
   dot"), `96W-0` ("the three neutral footer chips gain pill outlines they do
   not carry on desktop") and group 07's `96L-0` ("the footer chips gain a
   tinted fill … that desktop does not have"). **Decision:** R-F.2's box
   measurements bind `max-width: 520px`; at wider tiers the four chips are bare
   text with a chevron and the separator rule `489-0`, as `482-0` draws them.
   R-F.2 should be read as scoped to the phone tier.

2. **An open chip reads accent at every width, including "No checks declared".**
   `48J-0` (desktop light) and `7XA-0` (desktop dark) draw the checks chip muted
   at weight 400 while its own section is open and its own chevron is accent.
   Note `9XQ-0` says the opposite in words. **Decision:** implement
   `--color-accent` at weight 500 on desktop and weight 600 on mobile for every
   open chip, checks included. Listed as a Paper defect below.

3. **The zero-count comment chip stays faint and un-pilled at every width.**
   R-F.3 says the mobile count "takes `--color-accent-bg` with an
   `--color-accent` icon and label", and `tokens.md` §8 maps that to
   `max-width: 520px`. Both 07a mobile boards draw it faint. **Decision:** the
   accent treatment is a property of *being a control that opens something*, and
   a zero-thread chip opens an empty sheet. At count 0 the chip stays
   `--color-faint` with no ground — which is also exactly the engine's own third
   variant, `comment-chip--empty` (`components.go:663`). At count ≥ 1 R-F.3
   applies in full. The 07a mobile boards are therefore correct for their state
   and are **not** counter-evidence against R-F.3.

4. **The claim card's desktop radius is 12px.** Off the design radius scale
   (4 / 8 / 999), off the engine's `--radius: 6px`, and off the engine's current
   `.claim { border-radius: 8px }` (`style.css:229`). **Decision:** carry 12px as
   a per-component literal, per `tokens.md` § Open decisions ("off-scale radii
   stay as per-component literals"). Do not widen `--radius` and do not add a
   token.

5. **The reading measure on this screen is 760px, and it is confirmed three
   times.** `480-0` `maxWidth: 760px` (the prose **block**; its text child
   `481-0` carries no `maxWidth` of its own, and the dark pair is `7WS-0` /
   `7WT-0` the same way), `47N-0` `width: 760px`, `4BF-0` `maxWidth: 760px`. This agrees with `tokens.md`'s frozen decision (760, not
   the token's 720 and not board 01's 680). The checks REASON's `640px`
   (`4BE-0`) is a narrower sub-measure inside one section, not a second reading
   measure.

6. **A new `max-width: 520px` block for the footer must be written after
   `style.css:4658`, immediately before the print block's doc comment at
   `style.css:4660`.** `tokens.md` §8 says the existing `.claim-footer__counts`
   rules live at `max-width: 640px` and `max-width: 560px`. Read at `3ac8844`
   they are actually at `style.css:4638-4641` (inside `@media (max-width: 860px)`,
   opened at `4533`, closed at `4645`) and `style.css:4655-4657` (inside
   `@media (max-width: 560px)`, opened at `4647` and **closed at `4658`**). The
   `@media (max-width: 640px)` block at `style.css:2563` owns `.gcp-*` only and
   never touches the footer.
   **Decision:** the correction stands; a 520 footer block carries equal
   specificity to both and must therefore be the **last** screen block in the
   file. The insertion point is the blank line `4659` — after the 560 block's
   closing brace `4658` and before the print block's doc comment, which runs
   `4660-4720` and belongs to the `@media print {` that opens at `4721` (the
   file ends at `4782`). The block must **not** go "after 4663": that line is
   inside the print comment, and splitting it detaches the comment from the
   block it documents, which is precisely what its own "IT MUST STAY LAST IN THE
   FILE" paragraph exists to prevent.
   (`tokens.md` §8 and its Disagreement 16 should be corrected by whoever owns
   that file; this is a foundations inaccuracy, not a Paper defect.)

7. **The three caps trackings are preserved as measured.** `+0.08em` for section
   eyebrows, `+0.06em` for relationship direction headings, `+0.07em` for the
   two checks labels, `+0.05em` for desktop lifecycle badges and `+0.06em` for
   mobile ones. None has a token (`tokens.md` §3). **Decision:** ship them as
   per-component literals; do not normalise them to `--tracking-label` and do
   not add tokens.

8. **The readiness ground and its two hairlines need a light twin the token set
   does not have.** Dark has `--color-dark-blocked-surface` `#1D1618` and
   `--color-dark-blocked-hairline` `#3A2A2B`, both used on 07a. Light uses
   `#FBF7F7` for the ground and **two** weights of blocked hairline — `#EBD9D8`
   for the section rule and the module group row, `#EFE6E5` for the blocker row
   and (on mobile) the section's own top border — none of which is a token.
   **Decision:** record the requirement as three missing light tokens
   (`--color-blocked-surface`, `--color-blocked-hairline`,
   `--color-blocked-hairline-soft`) and implement the measured hexes as
   per-component literals until foundations adds them. Do **not** substitute
   `--color-blocked-bg` (10%) for the ground — it is roughly 2.5× too strong.

9. **The readiness section's top border is neutral on desktop and
   blocked-tinted on mobile, and that is a rule, not drift.** Desktop light
   `48R-0` `#E8ECF1`, desktop dark `7XJ-0` `#212934`; mobile light `8NB-0`
   `#EFE6E5`, mobile dark `909-0` `--color-dark-blocked-hairline`. Consistent
   per width tier in both modes. **Decision:** keep it. On a full-bleed card the
   section boundary is the only separator, and a neutral rule there reads as a
   page seam rather than as the edge of the blocked region.

10. **"See in claims graph" is new surface with no engine address.** The only
    reading-view→graph entry point today is the global `#dxgOpen` button
    (`shell.html:73`); `graph-ui.js:3585-3593` goes the other way.
    **Decision:** treat it as a new per-claim affordance living inside the
    readiness footnote, right-ranged on desktop (`49F-0`) and on its own line on
    mobile (`8ON-0`), and note that R09.6's "the banner is the only way in"
    constrains the **Issues screen**, not the graph pane — this affordance does
    not violate it.

11. **Line numbers in §7 are pinned to `3ac8844`, and the working tree is
    moving underneath them.** While this spec was being written another lane
    rewrote `style.css` (+202 lines), `graph.css` (+79), replaced
    `internal/render/geist_fonts.go` with `internal/render/engine_fonts.go`, and
    added Inter / IBM Plex Mono / Source Serif 4 `.woff2` files under
    `viewer/template/fonts/`. Line numbers in `style.css` had already shifted by
    ~88 within the writing of this document. **Decision:** every address in §7 is
    cited against `git show 3ac8844:<path>` and is accompanied by its marker
    comment or opening selector. A lane agent must locate an address by its
    marker text, not by its number. This is a working-practice hazard for the
    whole revamp, not a fact about 07a, and should be surfaced to the
    coordinator.

12. **`tokens.md` names the engine's inlined-font precedent as
    `geist_fonts.go`; that file is being deleted.** `tokens.md` § 7 uses
    `internal/render/geist_fonts.go` as the precedent for adding an engine-owned
    serif. In the live working tree that file is gone and `engine_fonts.go` has
    taken its place alongside the three families 07a actually needs.
    **Decision:** 07a's serif values (`481-0` 17/28, `4C0-0` 15/25, `4BE-0`
    14/22, `4BG-0` 15/24, `4BW-0` 13/21, `49E-0` 14/22) are specified as Source
    Serif 4 regardless; whether they are shippable on the day depends on that
    lane landing, not on this screen.

13. **The §7 addresses were re-verified line by line after the first
    verification pass, and four were wrong.** The first draft of this spec cited
    the live pill layer as `style.css:3957-3981` (`3957` is mid-rule and `3980`
    already belongs to the next rule), the 560 tier as `4647-4663` (it closes at
    `4658`; `4663` sits inside the print block's doc comment), the M5 insertion
    point as "after `4663`" (which would split that comment), and the readiness
    breakpoints as `1160-1172` / `1174-1181` (actually `1160-1173` /
    `1175-1182`). All four are corrected above against
    `git show 3ac8844:internal/render/viewer/template/style.css`.
    **Decision:** the marker text beside every address is the authority and the
    number is the convenience, but a number that lands inside a *different*
    construct is not a tolerable rounding error — it is an instruction to break
    the file. Any future edit to §7 must re-read the surrounding lines, not just
    the cited one. This is the concrete cost of Open decision 11.

### Paper defects

Found on the four 07a boards, listed most damaging first. **Approval is not
proof**: all four boards are approved and every item below is still wrong.

1. **Desktop "No checks declared" chip does not read as open.** `48J-0` is
   `#54606F` weight 400; `7XA-0` is `var(--color-dark-muted)` weight 400. Its
   section is open and its own chevron is accent. The board contradicts its own
   notes strip (`9XQ-0`: "every expansion on this board is open, so the whole
   footer reads accent"). Both mobile boards get it right (`8IC-0`, `8ZW-0`'s
   third child: accent, weight 600, accent border). **Fix: accent, weight 500 on
   desktop.**

2. **The governed-by relationship dot is 6px on desktop and 7px everywhere
   else.** `4BR-0`'s dot is `6 × 6`; the two blocked dots beside it (`4A4-0`,
   `4AO-0`) are `7 × 7`, and all three mobile dots (`8SS-0`, `8T8-0`, `8UA-0`)
   are `7 × 7`. R-I.2 measures the dot at 7px. A locked dot reading one pixel
   smaller than a blocked dot on the same list is a false hierarchy. **Fix: 7px
   everywhere.**

3. **The relationships section ground is `#F6F8FA` on light, where the token is
   `#F3F5F8`.** `49L-0` and `8OV-0`. The dark twin (`7YI-0`, `916-0`) is
   `#1B212B`, which *is* `--color-dark-code-bg` exactly. So the dark board
   spells the right value and the light board misses it by three units per
   channel. Same class as `tokens.md` Disagreement 12's board-12 finding.
   **Fix: `--color-code-bg` on both.**

4. **Intra-card hairlines are raw hexes that are not the border token.** Light
   `#E8ECF1` against `--color-border` `#DDE2E9`; dark `#212934` against
   `--color-dark-border` `#242C38`. Used on `482-0`, `48R-0`, `49L-0`, `4AS-0`,
   `4B5-0`, `4BF-0` and their twins on all four boards. `#E8ECF1` and `#212934`
   are pre-token residue (the same literals `tokens.md` Disagreement 12 lists
   for the components board). **Fix: implement `--color-border` / its dark twin.**

5. **The eyebrow rules are a fourth and fifth undocumented grey.** `49O-0` /
   `8OY-0` `#E3E8EE` (relationships) and `4AV-0` / `4B8-0` / `8UJ-0` / `8UQ-0`
   `#E8ECF1` (sources, checks). Two different rules in two adjacent sections of
   one card. The dark boards use `var(--color-dark-border)` for the first and
   `#212934` for the second — the same split, spelled one way as a token and one
   way as a literal. **Fix: one hairline token for all of them.**

6. **The desktop card background is the literal `#FFFFFF`, not
   `var(--color-card)`.** `47P-0`. The dark twin `7WH-0` correctly spells
   `var(--color-dark-card)`, and both mobile boards spell `var(--color-card)` /
   `var(--color-dark-card)`. One board out of four. **Fix: the token.**

7. **The dark DRAFT chip ground is the literal `#DDA94E21` instead of
   `var(--color-dark-draft-bg)`.** `7WJ-0`'s chip and `8Z9-0`. The value is
   correct (13% of `#DDA94E` = the token) — only the spelling is wrong, but a
   literal is what makes a theme change silently miss an element. Both light
   boards spell `var(--color-draft-bg)`. **Fix: the token.**

8. **Blocked tints are three different alphas across the four boards.** Light
   count-pill and target-slug `#9E3B361A` (10% — matches
   `--color-blocked-bg`); light mobile Blocked chip `#9E3B3617` (**9%**, below
   the token); dark count-pill and target-slug `#EC8A8329` (**16%**, above the
   13% `--color-dark-blocked-bg`); dark mobile Blocked chip `#EC8A8321` (13% —
   matches). Four values for what should be one or two. **Fix: one tint per
   mode, from the token; if the slug chip genuinely needs a stronger wash than
   the pill, that is a second token, not an ad-hoc alpha.**

9. **Chevron and icon strokes on the desktop light board are literal hexes where
   the dark board uses tokens.** `487-0` `stroke="#9E3B36"`, `48C-0` / `48G-0` /
   `48K-0` `stroke="#1C4E8C"`, `48O-0` `stroke="#6E7C8E"`, `49G-0`'s three
   elements `stroke="#1C4E8C"`, `4C5-0`'s chevron `stroke="#6E7C8E"`. Every
   corresponding dark node spells `var(--color-dark-*)`. **Fix: tokens on both
   sides.**

10. **Text colours on the desktop light board are literal hexes in six places
    where the mobile light board uses tokens.** `48J-0` `#54606F`, `48T-0`
    `#9E3B36`, `48V-0` / `491-0` / `4A6-0` `#6E7C8E`, `490-0` / `4C4-0`
    `#101720`, `49E-0` / `4BB-0` / `4BE-0` / `4BG-0` / `4C0-0` `#54606F`. The
    values are all correct token values; the spelling is not. **Fix: tokens.**

11. **The desktop board omits a hairline the mobile board has, and vice versa.**
    The desktop footer strip (`482-0`) uses `marginInline: 32px` so its rule
    stops short of the card edge, while every section below it uses a full-bleed
    `border-top`. Mobile does the same (`8HD-0` `marginInline: 16px`). This is
    consistent, but it means the card carries **two** kinds of horizontal rule
    with no note explaining the difference. Recorded as an ambiguity rather than
    an error; a lane agent should preserve it exactly as measured.

12. **`tokens.md` §3 has no token for four of the sizes this card uses most.**
    `13/16` (every desktop footer chip label), `14/18` and `14/20` and `14/21`
    (three different leadings on 14px across the readiness and relationship
    rows), `15/24` and `15/25` (the two closing footnotes, one unit apart), and
    `12/20` (the mono `none`). Not a board defect, but it is why this spec is
    exhaustive about leading: the scale cannot carry these and the literals are
    the specification.
