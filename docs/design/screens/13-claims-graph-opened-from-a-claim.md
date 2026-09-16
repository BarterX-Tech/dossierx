# 13 · Claims graph — opened from a claim

Screen group 13. Source of record: Paper file `01M2MTR6F73KHFR95ZWE8A7YAC`, page
`1-0`. Engine side read from the worktree `feat/viewer-design-revamp` at
`3ac8844`.

Read this with `../tokens.md` (every token name and hex) and
`../reference-rules.md` (the rules cited by id below). Where this file names a
colour it names the **token**; the hex beside it is the token's value, not a
licence to spell the literal.

Two standing rules apply here harder than anywhere else in the set, because this
screen is the only one that owns a categorical ramp:

- **Approval is not proof.** Four of these boards shipped with light graph
  tokens painted on a dark ground. They are listed under *Paper defects*.
- **`graph.css` is authoritative for every graph colour, light and dark.** Paper's
  `--color-dark-graph-*` set is not implemented and must not be implemented as
  drawn (`tokens.md` § 2 and Disagreement 1–3).

---

## 1. Boards

| Node | Name | Width × height | What it shows |
|---|---|---|---|
| `3NL-0` | 13 · Claims graph — opened from a claim | 1440 × 1024 | **Desktop light.** The full pane: header, control bar, scope notice, node-link canvas, facet/marks legend, and a 348px right rail. Depicts: one **selected** claim (`Mechanism-scoped observation authority`, accent ring), status **Locked · blocked by 2**, **governed by** one claim (two governance arcs drawn), **two nodes carrying open comment threads** (halo rings), one **collapsed module** node (`Verification · 25`), **no cycle** (`IN A CYCLE · No`), relations **Depends on** and **Governed by** ON / **Says the same thing** OFF, VIEW `labels` ON / `re-run layout` off, scope narrowed to module `Permission readiness` + facet `Contract`, track `All tracks`. Rail's lower half shows **3 of 6 layout rules** firing (`G-04`, `G-01`, `G-02`) and a collapsed “3 rules found nothing”. |
| `A5J-0` | 13 · … · DARK | 1440 × 1024 | **Desktop dark.** Same state, same geometry, dark chrome tokens throughout. Deviates from light only in palette. |
| `AFZ-0` | 13 · … · MOBILE LIGHT | 390 × 844 | **Mobile light.** No canvas. App bar (`Curtainly`), screen header with `Close`, wrapped scope selects, wrapped relation chips, scope notice, a **selected-claim block** (title, mono slug, STATUS, IN A CYCLE), then **AROUND THIS CLAIM** — three edge groups (`GOVERNED BY 1`, `DEPENDS ON 3`, `DEPENDED ON BY 4`) as two-line rows — then the back link, the readings block and the legend. Depicts two **blocked** dependency rows, one **Draft** dependent, one **out-of-scope/outlined** node, one **halo** node, one collapsed module row. |
| `AST-0` | 13 · … · MOBILE DARK | 390 × 844 | **Mobile dark.** Same structure and same states as `AFZ-0`, dark chrome. |
| `A1K-0` | Group 13 — band | 3860 × 27.5 | Group band. Eyebrow `13 · CLAIMS GRAPH — OPENED FROM A CLAIM` in `--color-accent`, 13/16 w600 tracking `0.08em`; caption `four designs of one screen · light pair left, dark pair right` in `--color-faint` 12/16; a 2px `--color-accent` rule under both. |
| `AXY-0` | Group 13 — notes | three 900px columns, 64px gutters | The notes strip. Column 1 `THE GRAPH AT 390 — THE DECISION` + *How you get here*. Column 2 `WHAT MOBILE DOES DIFFERENTLY — EVERY DIFFERENCE` (four paragraphs) + a `--color-blocked` disclosure about invented titles. Column 3 `FOUR GRAPH HUES ARE THIN ON THE DARK GROUND` (heading in `--color-blocked`) + two paragraphs. Note headings: Inter 11/14 w600 tracking `0.08em`; note bodies Inter 13/20 `--color-muted`. |

Boards read: `3NL-0`, `AFZ-0`, `A5J-0`, `AST-0`, `A1K-0`, `AXY-0`.

---

## 2. Design intent

### What the reviewer should perceive

The pane is **a different view of the whole corpus, not a dialog about one
claim**. It fills the viewport, dims nothing behind it, and its header states
where the reader came from (`arrived from Mechanism-scoped observation
authority`) so that opening it never feels like losing the claim. The one thing
a reviewer came for is **the shape of a readiness neighbourhood**: what this
claim rests on, what rests on it, and which claim governs it. Everything else on
the screen is subordinate to that picture.

The visual grammar is deliberately layered so that no single channel carries two
meanings:

- **Hue is identity** — a node's fill is its facet, and only its facet. The five
  named facets on these boards are `Contract`, `Internals`, `Doctrine`,
  `Evidence`, `Recovery`, plus a reserved slot for a collapsed module.
- **Size is degree** — the largest thing on the canvas is the most-connected
  thing; an isolated claim is literally the smallest, which is what makes
  `G-02 · One claim is barely linked` pre-attentive rather than a sentence to
  read.
- **Rings are state** — accent ring = selected, amber ring = has an open comment
  thread, red = cycle. A state colour is never used as a fill, so a state can
  never be mistaken for an identity.
- **The governance edge is a different colour and a curve**, because governance
  is not dependency and must not be read along the same lane.

The rail is the pane's prose voice. It answers the selected node in **noun ·
value** rows, then offers exactly one way back (`Back to this claim in the
reading view`), then states what the layout itself found — labelled as
**readings about the picture, not findings about the corpus**, which is the
whole reason those rules are allowed on screen at all.

On a phone the picture is abandoned and the same facts are read as lines. That
is the single largest decision on this screen and the notes strip argues it at
length; see *Mobile rules*.

### Rules from `reference-rules.md` that bind this screen

- **R09.9 — No inline dependency map; the breadcrumb and the graph pane carry
  it.** This screen *is* the other half of that rule. R09.9 says a readiness
  chain is a DAG and that drawing it as a tree invents blockers that do not
  exist; the graph pane is where the real shape is allowed to be drawn, with
  crossings. Everything on this screen inherits the obligation R09.9 creates: if
  the pane draws a wrong shape, the claim screens have nowhere else to send the
  reader. Binds explicitly (`reference-rules.md` names *05, 06, 06a, 13 and
  component I1*).
- **R09.4 — Relationships is one section with three directions.** The mobile
  form obeys this literally: `GOVERNED BY`, `DEPENDS ON`, `DEPENDED ON BY` are
  three directions of one section, not three components. The desktop relation
  chips (`Depends on`, `Says the same thing`, `Governed by`) use the same
  vocabulary.
- **R09.5 — Sources is split out of relationships.** Nothing on this screen
  draws a citation. A source is not a graph edge and no edge type on these
  boards is one.
- **R09.7 — The facet dot is the only project-level health signal in the reading
  view.** The facet dot reappears here as the node fill and as the mobile row's
  leading dot, carrying the same meaning in both places; it must not acquire a
  second meaning on this screen.
- **R10.1 / R10.3 — Elapsed time, never a timestamp; one unit, never two.** The
  header's freshness reads `last read 2h ago` — one unit, elapsed. See *Footer
  vocabulary*.
- **R10.5 — Behind `dossierx serve`, the line reads “Live”.** The `Refresh`
  button beside the freshness line is the served-mode affordance and the notes
  confirm it is dropped on mobile.
- **R11.1 — One reversible state, not two independent toggles.**
  `reference-rules.md` binds this rule to *“03 (all four variants), and the
  header on 02, 04, **13**, 14”*, so it names this screen explicitly. Its
  statement — *“a single reversible state is easier to reason about than two
  independent ones that produce four layouts”* — reaches the `VIEW` group, which
  is the only place on this screen where two pill-shaped controls sit side by
  side. Measured, they are **not** two toggles: `labels` is a real reversible
  state (`graph-ui.js:648`, `toggleButton('labels', true, …)`, `data-dxg-labels`,
  `aria-pressed`), while `re-run layout` is a momentary **action** that persists
  no state at all (`graph-ui.js:653-661`, `h('button', 'dxg-btn', 're-run
  layout')`, `data-dxg-relayout`, whose handler clears `positions` and restarts
  the layout). One state × one action is one layout, not four, so R11.1 holds —
  but the **board draws them identically** (`3OT-0` and `3OV-0` both measure
  `padding: 5px 11px`, `border-radius: 999px`, `1px solid`), which is what makes
  the group *look* like the shape R11.1 forbids. Resolved by D14; also why M4
  can drop the whole group at 520 without touching a reversible state.
- **R-I.0 — Same content, same vocabulary, same order; only the arrangement
  moves.** The rule the entire mobile argument rests on. *“A screen must use the
  form that matches its width; these are not alternative components.”* The notes
  strip's own *Added* bullet — *“the mobile form is a rearrangement, not a new
  visual language”* — and its *Vocabulary: unchanged* bullet (both quoted under
  *Every note on the band and the notes strip* above) are this rule restated in
  the board's words. R-I.0 carries no `Binds` line, so it binds by subject
  rather than by name; it is named here because § 5's M10 mirrors R-I.2, which
  is R-I.0's own worked example, and because M1–M12 are otherwise a list of
  removals with no rule saying why removing is allowed at all. **The one place this screen strains R-I.0 is the
  canvas**, which is not rearranged but removed; that is D12's subject and is
  argued there rather than hidden here.
- **R-H.0 — Only three components change shape below the phone breakpoint.**
  This screen is the deliberate, argued exception: the canvas is not a component
  that narrows, it is a component that is **not drawn at all** at 390. The notes
  strip carries the reasoning and the coordinator's approval; it is recorded
  here as an *Open decision* rather than as a silent fourth shape change.
- **R-J.1 / R-J.6 — One bottom sheet, one shell.** This screen introduces **no
  sheet**. The mobile form is a scrolling screen, not an overlay. A lane agent
  must not turn `AROUND THIS CLAIM` into a sheet: the notes explicitly reject a
  third overlay beside the comments rail and the graph pane.
- **R00.0 — The components board is the source of truth and the rule is
  procedural.** Where these boards and a reference board disagree, the reference
  board wins and the screen is flagged.

### Every note on the band and the notes strip, with intent

**Band (`A1K-0`).** “four designs of one screen · light pair left, dark pair
right.” Intent: the four boards are one screen at two widths in two themes, not
four screens. Nothing is a variant that exists only in one theme.

**Notes column 1 — `THE GRAPH AT 390 — THE DECISION` (`AY0-0`, body `AY1-0`).**
The node-link canvas is **not drawn** at 390. Twenty-six nodes reduced to 358px
have labels no one can read, and pan-and-zoom asks the reader to redo the
layout's job one thumb-width at a time. What the screen exists to answer is
*local* — what this claim rests on, what rests on it, where the governing edge
sits — so mobile shows that neighbourhood **as the edges themselves, grouped by
relation, one hop out in both directions**. Same nodes, same facet dots, same
marks; the lines become the group headings and the labels get their full size
back. *Intent:* the payload of a dependency neighbourhood is the **names**, and
a name is the first thing a reduction destroys. Overview-plus-detail (the choice
group 09 made for the placement map) was the other candidate and is explicitly
rejected here, because a placement map still has a readable shape when shrunk and
a dependency neighbourhood does not.

**Notes column 1 — `How you get here` (`B0O-0`).** “See in claims graph” on any
claim, or the left nav. **There is no other entry.** *Intent:* the pane is never
auto-opened and never deep-linked from anywhere else; two entries, both
deliberate.

**Notes column 2 — `WHAT MOBILE DOES DIFFERENTLY — EVERY DIFFERENCE`.** Four
paragraphs, paraphrased with intent:

- *Dropped.* The canvas; the VIEW controls (`labels`, `re-run layout`), which
  steer only that drawing; the two arrow marks in the legend and the `selected`
  mark, because no lines are drawn and no row is the selected node; the
  `MODULE` / `FACET` / `TRACK` eyebrows, because the values name themselves;
  `Refresh`; `828 claims · last read 2h ago`; the `Esc` hint on `Close`. From the
  rail: `MODULE` and `FACET` (they are the two filter chips a screen-length
  above) and `DEGREE HERE` (it is the two group counts). **`GOVERNED BY` is not
  dropped** — it leaves the rail and becomes its own edge group, *where it always
  belonged*. *Intent:* every removal is either “this steers a drawing that is not
  drawn” or “this fact is already stated twice on the same screen”.
- *Added.* The `AROUND THIS CLAIM` heading and its one sentence, three group
  headings with counts, and a row hairline. **No new colour, outline, tinted fill
  or chrome.** *Intent:* the mobile form is a rearrangement, not a new visual
  language.
- *Restructured / resized.* Each node becomes a two-line row (title, then
  `facet · status`). The rail's label/value pairs stay two columns at an **88px
  label** — they fit. Both chip strips and the legend **wrap**; nothing scrolls
  sideways. Chips and selects keep desktop **12px**. The inactive relation chip
  goes **w500 → w600** and the two active ones stay w600, “because nothing in
  this system is heavier”. Row titles **14/19** against the rail's **13/19**;
  reading headings **14px** against **13px**. Everything else is desktop size.
- *Vocabulary: unchanged.* `Depends on`, `Says the same thing` and `Governed by`
  are both the chips and the group headings. `Depended on by` is the inbound
  direction of the same edge, the wording group 09 already uses. `Says the same
  thing` is OFF in this state, **so that group is absent** — which is what the
  canvas does too. **One deliberate wording change**, approved by the
  coordinator and made on the two mobile boards only: the readings intro said
  *observations about this picture*, which is factually wrong where no picture is
  drawn, so it now reads *observations about these relationships*. The rest of
  the sentence is verbatim and the desktop boards are untouched.

**Notes column 2 — the `--color-blocked` disclosure (`APX`-adjacent, red body).**
Two claim titles on the mobile boards are **invented, not from the corpus**:
*“Permission prompts name the mechanism”* and *“Recovery restores prior
grants”*. The desktop canvas labels seven of its nodes and leaves the rest as
unlabelled dots; the mobile rows need a name for every edge, so two were written
to fill them. They are plausible in voice and facet-consistent, and every count
still matches the desktop rail — 1 governed by, 3 out, 4 in, two of them blocked.
**Read them as placeholder, not as data.**

**Notes column 3 — `FOUR GRAPH HUES ARE THIN ON THE DARK GROUND` (`AYA-0`,
heading in `--color-blocked`).** The categorical hues were picked against a
near-white ground and several do not carry to `--color-dark-paper`. Measured on
the board: `Contract #4257C4` and `Evidence #7050A8` both **3.1:1** — at the
floor for a 12px node and under it for the 9px legend swatch, and worse again on
`--color-dark-card` where the mobile rows sit. Governed-by `#C11F5B` is
**3.3:1**: it survives as a 1.8px arc but not as type. Cycle `#D1201A` is
**3.6:1** and is *the one to fix first, because it is an alarm and this dataset
has no cycle to expose it.*

**Notes column 3 — `Nothing is patched here` (`AYC-0`).** The one thing the
designer did fix: the mobile `GOVERNED BY` heading had been set in
graph-governed pink and measured **2.8:1** on the dark section band, so it is
back to **`--color-muted` on both themes** rather than each board inventing a
local pink — “the relation is already named by the word”. *Intent, and the
instruction to the lane:* **dark variants belong in the token set, decided once,
not four boards each inventing one.**

### Where the boards contradict their own notes (Paper defects, detail in § 9)

1. The notice says nine out-of-scope blockers “**are drawn as outlines**”. The
   desktop canvas (`3SY-0`, `A7D-0`) draws **zero** outline nodes — every circle
   is a filled disc, plus two halo rings and one selection ring.
2. The mobile note says out-of-scope claims carry an outlined dot “**exactly as
   desktop draws them**”. The mobile row `Adding a permission` (`APU-0`) is an
   outlined dot; the same claim on desktop (`3SY-0`, node at `390,512`) is a
   **filled** `#4257C4` disc with a halo. The two boards disagree about the same
   claim.
3. Column 3's contrast figures are quoted against `#4257C4` / `#7050A8` /
   `#C11F5B` / `#D1201A` — the **light** hexes — while the dark canvas
   (`A7D-0`) actually paints `--color-dark-graph-facet-1` and
   `-facet-4` for those two. The note's premise and the board's paint disagree.
4. The dark boards spell four **light** graph tokens on a dark ground.

---

## 3. Layout

### Desktop, 1440 (`3NL-0` light / `A5J-0` dark)

| Value | Measured | Node |
|---|---|---|
| Pane frame | `1440 × 1024`, `display:flex`, `flex-direction:column`, `overflow:clip` | `3NL-0` |
| Pane ground | `--color-paper` `#EFF1F4` / dark `--color-dark-paper` `#0D1117` | `3NL-0` / `A5J-0` |
| Header row height | `60px`, `flex-shrink:0` | `3NM-0` / `A5O-0` |
| Header inline padding | `28px` | `3NM-0` |
| Header gap | `14px`, `align-items:center` | `3NM-0` |
| Header bottom rule | `1px solid --color-border` / dark `--color-dark-border` | `3NM-0` / `A5O-0` |
| Control bar height | `54px`, `flex-shrink:0` | `3O2-0` / `A64-0` |
| Control bar inline padding | `28px`; group gap `26px` | `3O2-0` |
| Control bar bottom rule | `1px solid --color-border` | `3O2-0` |
| Label→control gap inside a group | `9px` | `3O3-0`, `3O7-0`, `3OB-0`, `3OH-0`, `3OR-0` |
| Relation-chip row gap | `6px` | `3OJ-0` |
| Vertical divider in control bar | `1 × 20px`, `--color-border` | `3OG-0` |
| Scope-notice block padding | `9px` block / `28px` inline; gap `10px` | `3OY-0` / `A74-0` |
| Scope-notice bottom rule | `1px solid --color-border` | `3OY-0` |
| Body row | `flex:1 1 0%`, `min-height:0` | `3P6-0` / `A7B-0` |
| Canvas column | `flex:1 1 0%`, `min-width:0`, column | `3PA-0` / `A7C-0` |
| Canvas SVG | `1092 × 812` rendered, `viewBox="140 60 900 560"` | `3SY-0` / `A7D-0` |
| Legend strip padding | `14px` block / `28px` inline | `3UR-0` / `A95-0` |
| Legend gaps | column `18px`, row `8px`, `flex-wrap:wrap` | `3UR-0` |
| Legend top rule | `1px solid --color-border`, ground `--color-card` | `3UR-0` |
| **Rail width** | **`348px`**, `flex-shrink:0` | `3VZ-0` / `AAB-0` |
| Rail ground / left rule | `--color-card`, `1px solid --color-border` | `3VZ-0` |
| Rail — SELECTED block | padding `20px` top / `22px` bottom / `24px` inline, gap `12px`, bottom rule `1px --color-border` | `3W0-0` |
| Rail — property block | padding `6px` top / `18px` bottom / `24px` inline, bottom rule `1px` | `3W5-0` |
| Rail — property row | `padding-block: 9px`, gap `16px`, `align-items:baseline`, bottom rule `1px --color-border` (last row has none) | rows inside `3W5-0`, `3WL-0` |
| Rail — property label column | fixed `96px`, `flex-shrink:0` | first child of each row in `3W5-0` |
| Rail — back-link row | `padding-block:16px`, `padding-inline:24px`, gap `8px`, bottom rule `1px` | `3WP-0` |
| Rail — readings block | `padding-block:20px`, `padding-inline:24px`, gap `12px` | `3WT-0` |
| Rail — one reading | `padding-block:12px`, gap `5px`, top rule `1px --color-border` | `3X0-0`, `3X5-0`, `3XA-0` |
| Rail — “N rules found nothing” row | `padding-block:12px`, gap `7px`, top rule `1px` | `3XF-0` |

**Sticky / scroll regions.** The boards are static frames and carry no
`position:sticky` or `overflow` declaration below the pane root, so the sticky
contract is not measurable from Paper. It is therefore inherited from the engine
and stated as a rule, not a measurement: header, control bar, scope notice and
legend are **fixed** (`flex-shrink:0` on all four, measured); the body row is the
only flexible region; **the rail scrolls internally** and the canvas holder
clips. See `graph.css:640` (`.dxg-rail`), `graph.css:667-670` (`.dxg-gaps`,
`overflow-y:auto` on `670`) and `graph.css:693-696` (`.dxg-detail`,
`max-height:46%` on `695`, `overflow-y:auto` on `696`).

### Mobile, 390 (`AFZ-0` light / `AST-0` dark)

| Value | Measured | Node |
|---|---|---|
| Frame | `390 × 844`, column, `overflow:clip` | `AFZ-0` / `AST-0` |
| Ground | `--color-card` `#FFFFFF` / dark `--color-dark-card` `#161B22` — **not** paper | `AFZ-0` / `AST-0` |
| App bar | `52px` tall, `padding-inline:16px`, gap `12px` | `AG7-0` |
| Screen header | `2px` top / `14px` bottom / `16px` inline, gap `5px`, bottom rule `1px --color-border` | `AGP-0` |
| Header title row gap | `10px` | `AGQ-0` |
| Sub-line indent | `padding-left:27px` (aligns under the title, past the 17px icon + 10px gap) | `AGZ-0` |
| Filter strip | `padding-block:12px`, `padding-inline:16px`, gap `9px`, bottom rule `1px` | `AJ9-0` |
| Both chip rows | `flex-wrap:wrap`, gap `8px` | `B0Q-0`, `AJH-0` |
| Scope notice | `padding-block:10px`, `padding-inline:16px`, gap `10px`, bottom rule `1px` | `AKH-0` |
| Selected-claim block | `padding:16px`, gap `8px`, bottom rule `1px` | `AN9-0` |
| Selected-claim property row | `padding-top:4px`, gap `16px`, label column **`88px`** | `ANC-0`, `AND-0` |
| AROUND THIS CLAIM heading block | `16px` top / `12px` bottom / `16px` inline, gap `6px` | `AOC-0` |
| Edge-group band | `padding-block:8px`, `padding-inline:16px`, gap `8px`, top **and** bottom rule `1px --color-border` | `AOG-0`, `AP2-0`, `AQ5-0`, dark `AUL-0` |
| Edge row | `padding-block:11px`, `padding-inline:16px`, gap `10px`, `align-items:start` | `AOK-0` … `AQV-0`, dark `AUO-0` |
| Edge-row dot slot | `16 × 16` SVG, `flex-shrink:0`, `margin-top:3px` | `AOL-0`, dark `AUP-0` |
| Edge-row text stack | `flex:1 1 0%`, gap `3px` | `AON-0` |
| Back-link row | `padding:16px`, gap `8px`, bottom rule `1px` | `AR2-0` |
| Readings block | `padding-block:20px`, `padding-inline:16px`, gap `12px` | `AR7-0` |
| One reading | `padding-block:12px`, gap `5px`, top rule `1px` | `ARD-0`, `ARJ-0`, `ARP-0` |
| Legend | `padding-block:14px`, `padding-inline:16px`, column gap `16px`, row gap `8px`, `flex-wrap:wrap`, top rule `1px`, ground `--color-card` | `AS0-0` |
| Board content width | `358px` inside 16px gutters | `AGQ-0`, `AKL-0`, `ANA-0` (all measured 358 or 334 where inset by the icon) |

The mobile boards are a **single scrolling column**. Nothing is sticky, nothing
is a sheet, and there is no horizontal scroll anywhere — both chip strips and the
legend wrap (measured `flex-wrap:wrap` on `B0Q-0`, `AJH-0`, `AS0-0`).

---

## 4. Components on this screen

**Measured-value count for the desktop light board (`3NL-0`): 118 values
recorded below** (every geometry figure in § 3's desktop table plus every
typographic, colour, radius, border, icon and state value in this section, each
read through `get_computed_styles` or `get_jsx`). The mobile boards contribute a
further 61. No value in this document was taken from a screenshot.

Family throughout: `--font-sans` (Inter) for chrome, `--font-mono` (IBM Plex
Mono) for ids, counts and rule ids, `--font-serif` (Source Serif 4) for the
readings prose. Letter-spacing is `0em` unless stated.

### 4.1 Pane header (`3NM-0` / dark `A5O-0`)

| Part | Value | Light token | Dark token | Node |
|---|---|---|---|---|
| Graph glyph | `17 × 17` SVG, three circles `r=3` + one path, `stroke-width:2`, `stroke-linecap:round` | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `3NN-0` / `A5P-0` |
| Title “Claims graph” | sans **19/24 w600** | `--color-ink` | `--color-dark-ink` `#E6EAF0` | `3NS-0` / `A5U-0` |
| Divider | `1 × 18px` | `--color-border` `#DDE2E9` | `--color-dark-border` `#242C38` | `3NT-0` / `A5V-0` |
| Provenance line | sans **13/18 w400** | `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `3NU-0` / `A5W-0` |
| Spacer | `flex:1 1 0%` (pushes the right cluster) | — | — | `3NV-0` |
| Freshness stamp | **mono 12/16 w400** | `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `3NW-0` / `A5Y-0` |
| `Refresh` button | `padding: 5px 12px`, radius **`6px`**, `1px` border, gap `8px`; label sans **13/16 w500** | border `--color-border-strong` `#C2CAD5`, label `--color-muted` | border `--color-dark-border-strong` `#38424F`, label `--color-dark-muted` | `3NX-0` / `3NY-0` / `A5Z-0` |
| `Close Esc` button | identical box to `Refresh`; label sans 13/16 w500 | as above | as above | `3NZ-0` / `3O0-0` / `A61-0` |

States visible: both buttons at **rest** only. No hover, focus, disabled or
pressed state is drawn on any board in this group.

### 4.2 Control bar (`3O2-0` / dark `A64-0`)

**Eyebrow label** (`MODULE`, `FACET`, `TRACK`, `RELATIONS`, `VIEW`) — sans
**11/14 w600**, tracking **`0.08em`**, `--color-faint` / `--color-dark-faint`.
Nodes `3O4-0`, `3O8-0`, `3OC-0`, `3OI-0`, `3OS-0`.

**Select control** (`3O5-0` module, `3O9-0` facet, `3OD-0` track):
`padding: 5px 10px`, radius **`6px`**, `1px solid --color-border-strong`, gap
`8px`, no fill. Value text sans **13/16 w500**. Chevron `11 × 11`, path
`m6 9 6 6 6-6`, `stroke-width: 2.6`, `stroke-linecap: round`, `--color-faint`.

- Module value `Permission readiness` and facet value `Contract` are
  **`--color-ink`** — a narrowed axis.
- Track value `All tracks` is **`--color-muted`** — an axis at its default.
  *Intent: a filter that is doing nothing says so by being quieter.*

**Relation toggle chip** — two states on the board, both measured:

| State | Ground | Border | Label | Nodes |
|---|---|---|---|---|
| **ON** (`Depends on`, `Governed by`) | `--color-accent-bg` `rgb(28 78 140 / 9%)` / dark `--color-dark-accent-bg` `rgb(106 166 232 / 14%)` | `1px --color-accent` `#1C4E8C` / dark `--color-dark-accent` `#6AA6E8` | sans **12/16 w600** `--color-accent` / `--color-dark-accent` | `3OK-0`, `3OO-0` |
| **OFF** (`Says the same thing`) | none | `1px --color-border-strong` | sans **12/16 w500** `--color-faint` | `3OM-0` |

Both states: `padding: 5px 11px`, radius `999px` (`--radius-pill`).

**VIEW controls** — the board draws both as the same pill box. `labels` is ON
(`--color-accent-bg` ground, `--color-accent` border, 12/16 w600
`--color-accent`; frame `3OT-0`, label `3OU-0`). `re-run layout` is drawn in the
OFF pill (`3OV-0`: `padding: 5px 11px`, `border-radius: 999px`, `1px solid
--color-border-strong`, no fill) but its label is **`--color-muted`, not
`--color-faint`** (`3OW-0`, 12/16 w500) — see *Paper defects* P9 and D8.

**These two are not the same kind of control.** `labels` is a toggle; `re-run
layout` is an action that holds no state (`graph-ui.js:648` vs `:653-661`). The
board's identical pills assert a symmetry that neither the engine nor R11.1
allows. **Spec:** `labels` keeps the toggle pill in both states;
`re-run layout` is rendered as a `.dxg-btn` — the same rectangle-with-`8px`-
radius the header's `Refresh` and `Close` take (`graph.css:423-432`) — so that
the `VIEW` group reads as one reversible state beside one button. See D14.

### 4.3 Scope notice (`3OY-0` / dark `A74-0`)

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Band ground | full-bleed | `--color-draft-bg` `rgb(154 106 22 / 11%)` | `--color-dark-draft-bg` `rgb(221 169 78 / 13%)` | `3OY-0` / `A74-0` |
| Bottom rule | `1px solid` | `--color-border` | `--color-dark-border` | `3OY-0` |
| Icon | `14 × 14`, circle `r=9` + `M12 8v5M12 16.5v.01`, `stroke-width: 2.2`, `stroke-linecap:round` | `--color-draft` `#9A6A16` | `--color-dark-draft` `#DDA94E` | `3OZ-0` |
| Body | sans **13/18 w400** | `--color-ink` | `--color-dark-ink` | `3P2-0` / `A78-0` |
| Action | sans **13/18 w600**, no underline, no box | `--color-accent` | `--color-dark-accent` | `3P3-0` / `A79-0` |

This is the **draft/attention** band, not the blocked band: the scope is
incomplete, not wrong. Only one state is drawn.

### 4.4 Canvas (`3SY-0` light / `A7D-0` dark)

Rendered `1092 × 812` over `viewBox="140 60 900 560"`. Every value below is read
off the SVG markup.

| Mark | Value | Light | Dark (as painted) | Engine token (authoritative) |
|---|---|---|---|---|
| Dependency edge | `stroke-width: 1.4`, straight | `#C2CAD5` (= `--color-border-strong`) | `--color-dark-border-strong` | `var(--border-strong)` — chrome, not ramp |
| Governance edge | `stroke-width: 1.8`, quadratic curve (`Q`) | `#C11F5B` (= `--color-graph-governed`) | `--color-dark-graph-governed` | `--dxg-governed` `#C11F5B` light / `#F06A9C` dark |
| Facet 1 node (`Contract`) | disc `r=6`, `r=7` when degree is higher | `#4257C4` | `--color-dark-graph-facet-1` | `--dxg-facet-1` `#4257C4` / `#7C8CE8` |
| Facet 2 node (`Internals`) | disc `r=6`–`7` | `#12897F` | **`#12897F` literal on dark** | `--dxg-facet-2` `#12897F` / `#3FB3A6` |
| Facet 3 node (`Doctrine`) | disc `r=8` (the governor, highest degree here) | `#B65A34` | **`#B65A34` literal on dark** | `--dxg-facet-3` `#B65A34` / `#DE8A62` |
| Facet 4 node (`Evidence`) | disc `r=6` | `#7050A8` | `--color-dark-graph-facet-4` | `--dxg-facet-4` `#7050A8` / `#A98CD8` |
| Facet 5 node (`Recovery`) | disc `r=6` | `#67717E` | **`#67717E` literal on dark** | `--dxg-facet-5` `#67717E` / `#8E9AA8` |
| Collapsed module node | disc `r=12` | `#7D8C85` | **`#7D8C85` literal on dark** | **live value is `--dxg-facet-other` from `style.css`: `#0d55b5` light / `#77a9e8` dark** |
| Open-comment halo | ring at `r=10` around an `r=6` node, `stroke-width: 1.6`, `fill:none` | `#C07E0C` | **`#C07E0C` literal on dark** | `--dxg-halo` `#C07E0C` / `#EFB44D` |
| Selection ring | ring at `r=14` around an `r=7` node, `stroke-width: 2.4`, `fill:none` | `#1C4E8C` (= `--color-accent`) | `--color-dark-accent` | `var(--accent)` in `graph-ui.js:2143` (`ctx.strokeStyle = pal.accent`; the `lineWidth` is `2144`) |
| Cycle ring | **not drawn on any board** — this dataset has no cycle | — | — | `--dxg-cycle` `#D1201A` / `#F5615C` |
| Selected node label | Inter **11px w600** | `#101720` (= `--color-ink`) | `--color-dark-ink` | `var(--ink)` |
| Governor node label | Inter **10.5px w600** | `#101720` | `--color-dark-ink` | `var(--ink)` |
| Ordinary node label | Inter **10.5px w400**, anchored `start` / `end` per side | `#54606F` (= `--color-muted`) | `--color-dark-muted` | `var(--muted)` |
| Collapsed-module label | Inter **10.5px w600** | `#54606F` | `--color-dark-muted` | `var(--muted)` |

Node radii on the board are **discrete** (`6`, `7`, `8`, `12`, plus the `r+4` /
`r+7` ring offsets). The engine computes radius continuously
(`r = 4.5 + 2.1·√degree`, clamped `[4, 26]`, `graph-ui.js:1966`). The board is
consistent with that formula and must be read as a sample of it, not as a set of
fixed sizes.

### 4.5 Legend (`3UR-0` / dark `A95-0`)

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Section label (`FACETS`, `MARKS`) | sans **10/12 w600**, tracking `0.08em` | `--color-faint` | `--color-dark-faint` | `3US-0`, `3VD-0` |
| Facet swatch | `9 × 9`, radius `999px` | `--color-graph-facet-1…5` | **Implement `--color-dark-graph-facet-1…5`.** The boards spell `-facet-1`/`-facet-4` with the dark token and `-facet-2`/`-3`/`-5` with the **light** one (P1) — a light token on a dark ground is a defect, not a value | `3UU-0` … `A9B-0`, `A9E-0`, `A9K-0` |
| Collapsed-module swatch | **`11 × 11`**, radius `999px` (deliberately larger) | `--color-graph-other` | **Implement `--color-dark-graph-other`.** The boards spell the **light** `--color-graph-other` (P1); and per D13 the value that actually paints is `--dxg-facet-other` = `#77a9e8` (`style.css:215`), not `graph.css:185`'s `#6C7F75` | `3V9-0` / `A9N-0` |
| Swatch→label gap | `7px` | — | — | every `3UT-0`-family frame |
| Legend label | sans **12/16 w400** | `--color-muted` | `--color-dark-muted` | `3UV-0` etc. |
| Vertical divider | `1 × 14px` | `--color-border` | `--color-dark-border` | `3VC-0` |
| `governed by` mark | `34 × 12` SVG: `M1 9 Q 13 1 24 6` + chevron `M20 2.5 L25 6 L20 9.5`, both `stroke-width: 2` | `--color-graph-governed` | `--color-dark-graph-governed` | `3VF-0` |
| `depends on` mark | `34 × 12` SVG: `M1 6 L27 6` + chevron `M23 2.5 L28 6 L23 9.5`, `stroke-width: 2` | `--color-border-strong` | `--color-dark-border-strong` | `3VK-0` |
| `has an open comment thread` mark | `16 × 16`: `r=3` fill + `r=6` ring `stroke-width: 1.6` | fill `--color-graph-facet-1`, ring `--color-graph-halo` | fill `-dark-graph-facet-1`, ring `-dark-graph-halo` | `3VP-0` |
| `selected` mark | `16 × 16`: `r=3.5` fill + `r=7` ring `stroke-width: 2` | fill `--color-graph-facet-1`, ring `--color-accent` | fill `-dark-graph-facet-1`, ring `--color-dark-accent` | `3VU-0` |

Legend order, exactly: `FACETS` · Contract · Internals · Doctrine · Evidence ·
Recovery · a collapsed module │ `MARKS` · governed by · depends on · has an open
comment thread · selected.

### 4.6 Rail — SELECTED block (`3W0-0` / dark `AAC-0`)

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Eyebrow `SELECTED` | sans **11/14 w600**, tracking `0.08em` | `--color-faint` | `--color-dark-faint` | `3W1-0` / `AAD-0` |
| Claim title | sans **16/23 w600** | `--color-ink` | `--color-dark-ink` | `3W2-0` / `AAE-0` |
| Claim id | **mono 11/17 w400** | `--color-faint` | `--color-dark-faint` | `3W3-0` / `AAF-0` |

### 4.7 Rail — property rows (`3W5-0`)

Six rows, fixed order: `MODULE`, `FACET`, `STATUS`, `GOVERNED BY`,
`DEGREE HERE`, `IN A CYCLE`.

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Row | `padding-block: 9px`, gap `16px`, `align-items: baseline`, `1px` bottom rule `--color-border` (omitted on the last row) | — | `--color-dark-border` | `3W6-0` … `3WL-0` |
| Label | sans **11/14 w600**, tracking **`0.06em`**, width **`96px`**, `flex-shrink:0` | `--color-faint` | `--color-dark-faint` | first child of each row |
| Value, neutral | sans **13/19 w400** | `--color-ink` | `--color-dark-ink` | `3W8-0` (`Permission readiness`), `3WB-0` (`Contract`), `3WK-0` (`4 in · 3 out`) |
| Value, status | sans 13/19 w400 | **`--color-locked`** `#2C6B52` | `--color-dark-locked` `#63BE9A` | `3WE-0` (`Locked · blocked by 2`) |
| Value, link | sans 13/19 w400 | **`--color-accent`** `#1C4E8C` | `--color-dark-accent` `#6AA6E8` | `3WH-0` (governing claim title) |
| Value, negative answer | sans 13/19 w400 | **`--color-muted`** | `--color-dark-muted` | `3WN-0` (`No`) |

*Intent:* the label column is a fixed lane so six rows form one vertical rule;
the value colour is the only thing that varies, and it varies for exactly three
reasons — status, navigation, and “this answer is a quiet no”.

### 4.8 Rail — back link (`3WP-0`)

Arrow `14 × 14`, path `M5 12h13M13 6l6 6-6 6`, `stroke-width: 2.2`,
`stroke-linecap: round`, `--color-accent` / `--color-dark-accent`. Label sans
**14/18 w600** in the same token. Row `padding-block:16px`,
`padding-inline:24px`, gap `8px`, bottom rule `1px --color-border`. Rest state
only; no hover drawn.

### 4.9 Rail — readings block (`3WT-0`)

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Heading `WHAT THE LAYOUT FOUND` | sans **11/14 w600**, tracking `0.08em` | `--color-faint` | `--color-dark-faint` | `3WV-0` |
| Count `3 of 6 rules` | sans **11/14 w400**, right-ranged by a `flex:1` spacer | `--color-faint` | `--color-dark-faint` | `3WX-0` |
| Intro prose | **serif 13/21 w400** | `--color-muted` | `--color-dark-muted` | `3WY-0` |
| Reading title | sans **13/19 w600** | `--color-ink` | `--color-dark-ink` | `3X2-0`, `3X7-0`, `3XC-0` |
| Rule id (`G-04`, `G-01`, `G-02`) | **mono 10/12 w400**, right-ranged | `--color-faint` | `--color-dark-faint` | `3X3-0`, `3X8-0`, `3XD-0` |
| Reading body | **serif 13/20 w400** | `--color-muted` | `--color-dark-muted` | `3X4-0`, `3X9-0`, `3XE-0` |
| Reading block | `padding-block: 12px`, gap `5px`, top rule `1px --color-border` | — | — | `3X0-0`, `3X5-0`, `3XA-0` |
| Collapsed tail `3 rules found nothing` | chevron `13 × 13`, path `M6 9l6 6 6-6`, `stroke-width: 2.4`; label sans **13/16 w500** | `--color-accent` | `--color-dark-accent` | `3XF-0` / `3XI-0` |

The tail row is a **collapsed disclosure** — the one collapsed/expanded pair on
this screen. Its expanded form is not drawn; see *States not on the boards*.

### 4.10 Mobile — app bar and screen header (`AG7-0`, `AGP-0`)

App bar `52px`, `padding-inline:16px`, gap `12px`: hamburger `20 × 20`, project
name `Curtainly`, search glyph `19 × 19`, theme glyph `19 × 19` (`AG8-0`,
`AGA-0`, `AGB-0`, `AGE-0`). It is the viewer's standing mobile chrome, not part
of this screen.

Screen header: glyph `17 × 17`; title `Claims graph` sans **19/24 w600**
`--color-ink` with `flex:1 1 0%` (node `AGW-0`); `Close` button
`padding: 5px 12px`, radius `6px`, `1px --color-border-strong` (node `AGX-0`) —
**the `Esc` hint is dropped**; sub-line sans **13/18 w400** `--color-muted`,
indented `27px` (nodes `AH0-0`, `AGZ-0`).

### 4.11 Mobile — filter strip (`AJ9-0`)

Select chips: `padding: 5px 9px`, radius `6px`, `1px --color-border-strong`, gap
`4px`, label sans **12/16 w500**, chevron `11 × 11` `stroke-width: 2.6`
`--color-faint`. `Permission readiness` and `Contract` are `--color-ink`;
`All tracks` is `--color-muted`. The three eyebrows are **dropped**.

Relation chips: `padding: 5px 11px`, radius `999px`.

| State | Ground | Border | Label |
|---|---|---|---|
| ON (`Depends on`, `Governed by`) | `--color-accent-bg` | `1px --color-accent` | sans **12/16 w600** `--color-accent` |
| OFF (`Says the same thing`) | none | `1px --color-border-strong` | sans **12/16 w600** `--color-faint` |

The OFF chip's weight steps **500 → 600** relative to desktop; the ON chips stay
at 600. This is the note's explicit rule: *nothing in this system is heavier than
600.*

### 4.12 Mobile — selected-claim block (`AN9-0`)

Title sans **17/24 w600** `--color-ink` (`ANA-0`); id **mono 11/17 w400**
`--color-faint` (`ANB-0`); two property rows only — `STATUS` and `IN A CYCLE` —
label sans **11/14 w600** tracking `0.06em` `--color-faint` at width **`88px`**
(`AND-0`, `ANG-0`), value sans **13/19 w400**, `--color-locked` for the status
(`ANE-0`) and `--color-muted` for `No` (`ANH-0`). `MODULE`, `FACET`,
`GOVERNED BY` and `DEGREE HERE` are dropped per the notes.

### 4.13 Mobile — AROUND THIS CLAIM (`AOB-0` / dark `AT…`/`AUL-0`/`AUO-0`)

| Part | Value | Light | Dark | Node |
|---|---|---|---|---|
| Heading | sans **11/14 w600**, tracking `0.08em` | **`--color-muted`** (not faint) | `--color-dark-muted` | `AOD-0` |
| One-sentence intro | **serif 13/20 w400** | `--color-muted` | `--color-dark-muted` | `AOE-0` |
| Group band ground | full-bleed | **`#F6F8FA` literal** (P7) — **implement `--color-code-bg` `#F3F5F8`** | **`#1B212B` literal** (P7) — **implement `--color-dark-code-bg`**, whose value *is* `#1B212B`, so only the spelling changes | `AOG-0` / `AUL-0` |
| Group band rules | `1px` top **and** bottom | `--color-border` | `--color-dark-border` | `AOG-0` / `AUL-0` |
| Group heading | sans **11/14 w600**, tracking `0.08em` | `--color-muted` | `--color-dark-muted` | `AOH-0` / `AUM-0` |
| Group count | **mono 11/14 w400** | `--color-faint` | `--color-dark-faint` | `AOI-0` / `AUN-0` |
| Row hairline | `1px` bottom | **`#E8ECF1` literal** (P8) — **implement `--color-border` `#DDE2E9`** | **`#212934` literal** (P8) — **implement `--color-dark-border` `#242C38`**; here the literal is also the wrong *value*, not just the wrong spelling | `AOK-0` / `AUO-0` |
| Last row in the section | `1px` bottom `--color-border` (the token, not the literal) | | | `AQV-0` |
| Row title | sans **14/19 w600** | `--color-ink` | `--color-dark-ink` | `AOO-0` / row titles |
| Row sub-line, neutral | sans **12/16 w400** | `--color-muted` | `--color-dark-muted` | `AOP-0` |
| Row sub-line, blocked | sans 12/16 w400 | **`--color-blocked`** `#9E3B36` | `--color-dark-blocked` `#EC8A83` | `APB-0` (`Internals · Locked · blocked by 4`), `API-0` (`Internals · Locked · blocked by 2`) |
| Row sub-line, draft | sans 12/16 w400 | **`--color-draft`** `#9A6A16` | `--color-dark-draft` `#DDA94E` | `AQM-0` (`Contract · Draft`, title `AQL-0`) |

**Row dot — five variants, all measured inside a `16 × 16` slot:**

| Variant | Geometry | Colour | Row |
|---|---|---|---|
| plain facet node | `circle r=5` filled | `--color-graph-facet-N` | most rows |
| node with an open thread | `circle r=4.2` filled + `circle r=7.1` ring `stroke-width: 1.6` | fill `--color-graph-facet-1`, ring `--color-graph-halo` | `The published observation record` |
| out-of-scope node | `circle r=4.2` **`fill:none`**, `stroke-width: 1.8` + halo ring `r=7.1` `stroke-width: 1.6` | stroke `--color-graph-facet-1`, ring `--color-graph-halo` | `Adding a permission` |
| collapsed module | `circle r=6.4` filled (larger, matching the legend's 11px swatch) | `--color-graph-other` | `Verification · 25` |
| (cycle) | not drawn | `--color-graph-cycle` | — |

The colours in that table are read off the **light** board (`AFZ-0`). On
`AST-0` the same five dots must take their dark twins —
`--color-dark-graph-facet-N`, `--color-dark-graph-halo`,
`--color-dark-graph-other`, `--color-dark-graph-cycle` — and per D13 the
implemented hex comes from `graph.css:180-188` / `style.css:215`, never from the
light board. The dark mobile board spells four of these with the light token
(`AX9-0`, `AXC-0`, `AXI-0`, `AXL-0`); that is P1, not an instruction.

### 4.14 Mobile — back link, readings, legend

Back link identical to desktop except the row is `padding:16px` (`AR2-0`) and the
label is sans **14/18 w600** `--color-accent` (`AR5-0`).

Readings: heading `WHAT THE LAYOUT FOUND` sans 11/14 w600 `--color-faint`
(`AR9-0`); count `3 of 6 rules` sans 11/14 w400 `--color-faint` (`ARA-0`); intro
**serif 13/21** `--color-muted` (`ARB-0`); reading title sans **14/20 w600**
`--color-ink` (`ARF-0`) — one step up from desktop's 13/19; rule id **mono 10/12**
`--color-faint` (`ARG-0`); body **serif 13/20** `--color-muted` (`ARH-0`); tail
row label sans **13/16 w600** `--color-accent` (`ARY-0`) — note this is **w600**
where the desktop tail is **w500**.

Legend: section labels sans 10/12 w600 tracking `0.08em` `--color-faint`
(`AS1-0`); swatches `9 × 9` radius `999px`, collapsed-module swatch `11 × 11`
(`AS3-0`, `ASI-0`); labels sans 12/16 w400 `--color-muted` (`AS4-0`). The two
arrow marks and the `selected` mark are **dropped**; only
`has an open comment thread` survives (`ASN-0`).

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. **No 390px breakpoint
is added** (`tokens.md` § 8).

| # | Rule at 390 | Engine query | Why that query |
|---|---|---|---|
| M1 | **The node-link canvas is not drawn at all.** The pane renders no `<canvas>`, no legend arrows, and no `selected` mark. | `@media (max-width: 520px)` | Arrangement, not target size. Today `graph.css:704` (`@media (max-width: 860px)`, section comment `702`) merely *stacks* the rail under the canvas and `style.css:4557` gives the holder `min-height: 210px`; the 520 rule must **win over both** and remove the canvas holder. |
| M2 | `AROUND THIS CLAIM` replaces the canvas: three edge groups, one hop out in both directions. | `max-width: 520px` | Same width question; it is the substitute arrangement for M1. |
| M3 | Rail collapses into the page flow — selected-claim block, then edge groups, then the back link, then the readings block. Nothing is a rail, nothing is a sheet. | `max-width: 520px` | The engine already stacks the rail at `max-width: 860px` (`graph.css:719-724`, inside the query opened at `704`); 520 keeps that and drops the rail's own `border-left` (`graph.css:646`) and `max-height` (`style.css:4559-4564`, `max-height: 210px` on `4562`). |
| M4 | **VIEW group dropped** (`labels`, `re-run layout`). | `max-width: 520px` | They steer a drawing that does not exist. `labels` is the group's only reversible state and `re-run layout` is a stateless action (D14), so dropping the group at 520 removes **one** state, not two — R11.1 is satisfied rather than dodged. Today `.dxg-ctl:nth-child(7)` gets `flex-basis: 190px` at 860 (`style.css:4543`, beside `nth-child(3)`/`(5)` at `4539-4540`); at 520 it is `display: none`. |
| M5 | **`MODULE` / `FACET` / `TRACK` eyebrows dropped**; the select values name themselves. | `max-width: 520px` | Pure arrangement. Targets `.dxg-ctl-label` (`graph.css:472-478`, `style.css:4344-4352`). |
| M6 | **`Refresh` dropped and the freshness stamp dropped**; `Close` loses its `Esc` hint. | `max-width: 520px` | Arrangement. `Refresh` is already absent without a server (`graph-ui.js:1222`); at 520 it is absent regardless. |
| M7 | **Control strip wraps; it does not scroll sideways.** Both chip rows are `flex-wrap: wrap`, gap `8px`. | `max-width: 520px` | This **reverses** the current `overflow-x: auto` + `scroll-snap-type: x proximity` on `.dxg-controls` at `max-width: 860px` (`style.css:4521-4531`; `overflow-x` on `4526`, `scroll-snap-type` on `4530`). The 520 rule must set `flex-wrap: wrap; overflow-x: visible` to win. |
| M8 | **Legend wraps**; `flex-wrap: wrap`, column gap `16px`, row gap `8px`. | `max-width: 520px` | Same reversal: `style.css:4579` currently sets `.dxg-legend-list { flex-wrap: nowrap; width: max-content }` at 860. |
| M9 | Rail property rows keep **two columns** at an `88px` label (down from 96px). Only `STATUS` and `IN A CYCLE` survive. | `max-width: 520px` | Arrangement. `.dxg-detail-rows` is already `grid-template-columns: minmax(72px, auto) minmax(0, 1fr)` (`style.css:4462-4463`); 88px is the phone value. |
| M10 | Edge rows: two lines, `16 × 16` leading dot slot with `margin-top: 3px`, `padding-block: 11px`. | `max-width: 520px` | Arrangement. Mirrors R-I.2's “four columns become two lines”. |
| M11 | **Relation chip weight 500 → 600 in the OFF state.** | `max-width: 520px` | Arrangement/legibility at 12px, same class of change as R-F.2's “at 12px on a phone, colour alone is too quiet”. |
| M12 | Row titles 14/19, reading headings 14/20 — one step up from the desktop 13/19. | `max-width: 520px` | Arrangement. |
| M13 | **Every tappable control ≥ 44px.** Select chips, relation chips, `Close`, the back link, each edge row, and the readings tail. | `@media (pointer: coarse)` | Target size, never width. The engine already sets `min-height: 44px` on `.dxg-btn, .dxg-toggle, .dxg-select, .dxg-notice-action` here (`graph.css:733-739`, the query opening on `733`); the edge row and the back link must be added. A 44px minimum **must not** be written into a width query. |
| M14 | The canvas pan hint is hidden. | `(pointer: coarse)` | Already true: `graph.css:741-743` sets `.dxg-canvas-hint { display: none }`. With M1 there is no canvas either, so this is belt-and-braces. |
| M15 | **No bottom sheet is introduced.** | — | R-J.1 / R-J.6: one shell, two known bodies (facet index, comment thread). This screen adds neither a third body nor a second shell. |
| M16 | Page ground is `--color-card` / `--color-dark-card`, not `--color-paper`. | `max-width: 520px` | Measured on `AFZ-0` / `AST-0`. The banded sections supply the only contrast, so a paper ground would flatten them. |

`(pointer: coarse)` and `max-width: 520px` are independent: a 1440px touch
display gets M13/M14 and none of M1–M12.

---

## 6. Footer vocabulary

This screen has **no claim footer** — there is no readiness/relationships/
sources/checks chip strip and no comment count, so **R-F.1 through R-F.4 do not
bind here**. What it does have are three meta/count strings and three group
headings, quoted exactly from the boards.

**Header meta line**, node `3NW-0` / dark `A5Y-0`, mono 12/16 `--color-faint`:

> `828 claims · last read 2h ago`

Order: **corpus count**, then `·`, then **elapsed freshness**. One unit
(`2h`), elapsed not stamped — R10.1 and R10.3. Dropped entirely at 390.

**Provenance line**, node `3NU-0`, sans 13/18 `--color-muted`:

> `arrived from Mechanism-scoped observation authority`

Lower case `arrived from`; the claim title is not quoted and not linked.

**Scope notice**, nodes `3P2-0` then `3P3-0` (mobile `AKM-0` then `AKN-0`):

> `26 claims in scope. 9 of their blockers sit outside it and are drawn as outlines.` `Widen to the whole project`

Order: **count in scope**, then **count outside it and how they are drawn**, then
the widening action hard after it. Two sentences, then a bare accent label. The
wording is identical at 390; only the wrap changes.

**Rail readings header**, nodes `3WV-0` then `3WX-0`:

> `WHAT THE LAYOUT FOUND` … `3 of 6 rules`

Heading hard left, count hard right across a `flex:1` spacer. The count is
`N of M rules` — a fraction, not a score.

**Rail readings tail**, node `3XI-0` (mobile `ARY-0`):

> `3 rules found nothing`

**Rail intro prose**, node `3WY-0` (desktop, verbatim):

> `Readings the layout can make on its own. They are observations about this picture, not findings about the corpus.`

**Mobile variant**, node `ARB-0` — one word changed, coordinator-approved:

> `Readings the layout can make on its own. They are observations about these relationships, not findings about the corpus.`

**Mobile section intro**, node `AOE-0`:

> `One hop out, in both directions. The picture is not drawn at this width — these are the same edges, read as lines.`

**Mobile group headings and counts**, in this order, nodes `AOH-0`/`AOI-0`,
`AP3-0`/`AP4-0`, `AQ6-0`/`AQ7-0`:

> `GOVERNED BY` `1` · `DEPENDS ON` `3` · `DEPENDED ON BY` `4`

Heading in `--color-muted`, count in `--color-faint` mono, `8px` apart. A group
with zero members is **absent, not zero** — `SAYS THE SAME THING` does not appear
because that relation is toggled off.

**Rail row labels**, in fixed order (`3W1-0` block then `3W5-0`):

> `SELECTED` │ `MODULE` · `FACET` · `STATUS` · `GOVERNED BY` · `DEGREE HERE` · `IN A CYCLE`

**Rail row values as drawn**: `Permission readiness`, `Contract`,
`Locked · blocked by 2`, `Protection language requires current evidence`,
`4 in · 3 out`, `No`.

**Back link**, node `3WS-0` / `AR5-0`:

> `Back to this claim in the reading view`

**Buttons**: `Refresh` (`3NY-0`), `Close Esc` (`3O0-0`), mobile `Close`
(`AGY-0`). **Relation chips**: `Depends on` · `Says the same thing` ·
`Governed by`. **VIEW chips**: `labels` · `re-run layout` (both lower case,
deliberately — they are verbs on a drawing, not nouns).

**Legend words**: `FACETS` · `Contract` · `Internals` · `Doctrine` · `Evidence` ·
`Recovery` · `a collapsed module` │ `MARKS` · `governed by` · `depends on` ·
`has an open comment thread` · `selected`.

---

## 7. Code address

The graph pane is **built entirely on the client**. The Go emitter produces a
JSON payload and nothing else; `shell.html` ships an empty mount point. Every
address below exists in the worktree at `3ac8844`.

### Go emitter

- `internal/render/graph_view.go:44` — `graphPayloadJSONWithBudget(cat, cfg, generatedAt, budget)`. The render path's **single** call site of `graph.Build`, and the named cache seam. 61 lines total.
- `internal/render/lazy_shell.go:77` — `func (d *lazyShellData) GraphPayload() (template.JS, error)`, the lazy-shell arm of the same value.
- `internal/render/render.go:47` — the `//go:embed` line that carries `graph-core.js`, `graph-ui.js` and `graph.css` into the binary.
- `internal/render/render.go:65-67` — `graphCoreFileName`, `graphUIFileName`, `graphCSSFileName`.
- `internal/render/render.go:94` — `graphCSSTemplatePath`.
- `internal/render/render.go:168` — `GraphCSS template.CSS`; `:182` — `GraphPayload template.JS`; `:187-188` — `GraphCoreJS` / `GraphUIJS`.
- `internal/render/render.go:721-723` — the read of `graph.css` off the embedded FS; `:750` — `graphCSS` into `loadedTemplates`; `:841-851` — the three client files threaded into the shell input; `:908` — `GraphPayload` assigned.
- `internal/graph/build.go`, `internal/graph/payload.go`, `internal/graph/encode.go` — the payload itself (node set, edge set, groups, dropped counts). `encode.go` is what applies `encoding/json`'s HTML escaping that `shell.html:302-311` relies on.
- There is **no** `*_view.go` and **no** `internal/render/components/*` template for this screen. Nothing in the pane is server-rendered markup.

### Shell

- `internal/render/viewer/template/shell.html:12-19` — the comment that fixes `graph.css` as the **first** `<style>` block (“style.css cascades over graph.css and the theme block over both”).
- `internal/render/viewer/template/shell.html:66-73` — the graph trigger. `:73` is the literal button: `<button id="dxgOpen" type="button" data-dxg-open aria-expanded="false">…  Claims graph</button>`.
- `internal/render/viewer/template/shell.html:284-299` — the mount point comment and `:299` `<section id="dxgPane" hidden></section>`. Empty on purpose and **outside `.layout`** so an SSE fragment swap cannot destroy the pane's client-only state.
- `internal/render/viewer/template/shell.html:301-311` — the payload block; `:311` `<script type="application/json" id="dossierx-graph">{{.GraphPayload}}</script>`.
- `internal/render/viewer/template/shell.html:316-323` — `{{.GraphCoreJS}}` then `{{.GraphUIJS}}`, in dependency order.
- `internal/render/viewer/template/shell.html:239` — the z-index ledger note naming `#dxgPane` / `body.dxg-open` as the third full-viewport surface.

### `graph.css` (1186 lines) — the pane's own sheet

Marker comment, `internal/render/viewer/template/graph.css:1-2`:

> `/* graph.css — chrome and categorical palette for the DossierX claims graph pane.`
> `   =========================================================================`

Every line number in this table was re-derived against `3ac8844` and lands on the
named selector or the first line of the named comment.

| Lines | Marker / selector | What it owns on this screen |
|---|---|---|
| `147-231` | `/* DARK IS THE BASE, light is the override. … */` (`147`) then `:root` (`161`) | The dark ramp. The eight authoritative `--color-dark-graph-*` hexes are declared at `180-188`; `--dxg-facet-1…5` alias them at `190-194`, `--dxg-facet-other` at `223`, `--dxg-cycle` `226`, `--dxg-halo` `227`, `--dxg-governed` `230`. |
| `233-235` | `/* \`, print\`: paper is light, whatever the OS says. … */` | Why the light query carries `print`. |
| `236-286` | `@media (prefers-color-scheme: light), print { :root { … } }` — the inner `:root` at `237`, `--dxg-facet-1…5` at `251-255`, `--dxg-cycle/-halo` `281-282`, `--dxg-governed` `284` | The light ramp — the nine values the Paper tokens match exactly. |
| `288-313` | `/* ====== PANE CHROME ====== */` banner (the words `PANE CHROME` are on `289`) | Section head. There is no `/* =====  PANE CHROME  ===== */` one-liner in this file. |
| `315-317` | `body.dxg-open` | The open-state body class. |
| `328-336` | `#dxgPane` (`[hidden]` arm at `338-340`) | `position: fixed; z-index: 80; inset: 0; background: var(--paper); font: 13px var(--font-sans)`. |
| `342-351` | `.dxg-surface` (`:focus` at `353-355`) | `z-index: 81`, the scroll-owning column. |
| `357` / `359-366` | `/* ---- header row: title, payload stamp, refresh, close ---- */` then `.dxg-head` | **§ 4.1.** `padding: 10px 14px`, `gap: 10px`, `border-bottom: 1px solid var(--border)`. |
| `368-391` / `393-400` | `/* ---- TEXT COLOUR AND WCAG AA, stated once for the whole file ---- */` then `.dxg-title` | Title: mono 11px uppercase `.05em` `--muted`. |
| `402-404` / `406-414` | `/* The generation stamp is the mitigation … */` then `.dxg-stamp` | **§ 6 freshness.** 12px `--muted`, `cursor: help`. |
| `416-442` | `.dxg-head-actions` `416`, `.dxg-btn` `423`, `.dxg-btn:hover` `434`, `.dxg-btn:disabled` `438` | **§ 4.1 buttons.** radius `8px`, `padding: 7px 10px`, hover `border-color: var(--accent)`. |
| `444-488` | `/* ---- control bar: six groups, seven with a track axis ---- */` `444`, `.dxg-controls` `454`, `.dxg-ctl` `465`, `.dxg-ctl-label` `472`, `.dxg-select` `480` | **§ 4.2.** |
| `490-493` | `.dxg-select:focus` | `outline: 2px solid var(--accent); outline-offset: -1px`. |
| `495-537` | `/* ---- the nav trigger ---- */` `495`, `#dxgOpen` `515`, `body.dxg-open #dxgOpen` `532` | The only entry point besides “See in claims graph”. |
| `539-563` | `.dxg-toggle` `542`, `.dxg-toggle[aria-pressed="true"]` `553` | **§ 4.2 relation/VIEW chips.** The ON state is border + tint, **never** text colour — the comment at `556-562` explains why. |
| `565-598` | `/* ---- notice strip: dangling edges, auto-collapse, refresh failure ---- */` `565`, `.dxg-notices:empty` `567`, `.dxg-notice` `571`, `.dxg-notice--warn` `582`, `.dxg-notice-action` `587` | **§ 4.3.** |
| `600-638` | `/* ---- body: canvas holder beside the rail ---- */` `600`, `.dxg-body` `602`, `.dxg-canvas-holder` `608`, `.dxg-canvas` `618`, `.dxg-canvas--drag` `626`, `.dxg-canvas-hint` `630` | **§ 4.4.** `touch-action: none` at `622`, under the comment at `616-617`. |
| `640-648` | `.dxg-rail` | **§ 4.6–4.9.** `width: min(320px, 36%)`, `border-left: 1px solid var(--border)`. |
| `650-691` | `/* THE RAIL SCROLLS, AND IT HAS TO SAY SO. … */` `650`, `.dxg-gaps` `667` (`overflow-y: auto` at `670`) + scrollbar pseudo-elements `679-691` | The scroll region. |
| `693-700` | `.dxg-detail` | `max-height: 46%` (`695`), `overflow-y: auto` (`696`). |
| `702` / `704-729` | `/* ---- below 860px: the rail stacks under the canvas ---- */` then `@media (max-width: 860px)` | The only width query in this file. `.dxg-body` `711`, `.dxg-canvas-holder` `715`, `.dxg-rail` `719`, `.dxg-detail` `726`. |
| `731` / `733-744` | the touch-target comment then `@media (pointer: coarse)` | `min-height: 44px` on four controls (`734-739`); hides the canvas hint (`741-743`). |
| `746-776` | `/* ====== LEGEND STRIP — facet identity's second channel ====== */` banner | Section head. The title text is on `747`; there is no `LEGEND` one-liner marker. |
| `778-933` | `.dxg-legend` `778`, `.dxg-legend-list` `785`, `.dxg-legend-item` `795`, `.dxg-legend-name` `825`, `.dxg-legend-swatch` `833`, `.dxg-legend-edge` `899`, `.dxg-legend-mark` `921` | **§ 4.5.** Slot swatch classes `844-863`; `.dxg-swatch-other` `864`; `.dxg-swatch-cycle/halo/governed` `870-872`; `.dxg-swatch-dim` `880`; `.dxg-legend-group` `887`. |
| `935-946` | `/* ====== RAIL INTERIOR — the gaps list, then the detail footer ====== */` banner | Section head. There is no `GAPS RAIL` marker anywhere in this file. |
| `951-991` | `.dxg-rail-head` `951`, `.dxg-rail-title` `964`, `.dxg-rail-jump` `975` (`:hover` `988`) | Today's sticky rail header. |
| `993-1076` | `.dxg-rule` `993`, `.dxg-rule-head` `997`, `.dxg-rule-name` `1006`, `.dxg-rule-phrase` `1010`, `.dxg-rule-count` `1027`, `.dxg-rule--empty .dxg-rule-head` `1037`, `.dxg-rule-ids` `1041`, `.dxg-jump` `1050`, `.dxg-rule-more` `1072` | **§ 4.9 readings**, as they exist today. |
| `1078-1109` | `.dxg-hints` `1080`, `.dxg-hints-caption` `1087`, `.dxg-hint-label` `1099` | The heuristics block. |
| `1113-1117` | `.dxg-scope-note` | “run `dossierx check`”. |
| `1119-1186` | `/* ---- detail panel ---- */` `1119`, `.dxg-detail-empty` `1121`, `.dxg-detail-id` `1127`, `.dxg-detail-rows` `1135`, `dt` `1143`, `dd` `1152`, `.dxg-detail-note` `1160`, `.dxg-detail-open` `1167` | **§ 4.6–4.8.** `overflow-wrap: anywhere` at `1132` and `1155`. |

### `style.css` (4870 lines) — the overrides that win

Re-derived against `3ac8844`, same as the table above.

- `internal/render/viewer/template/style.css:45` — the comment recording that `graph.css` keeps the **opposite** mode convention.
- `style.css:76` — `--dxg-facet-other: #0d55b5;` in the unconditional `:root`. This is the declaration that **wins**; `graph.css:223` is dead.
- `style.css:189` and `style.css:215` — `--dxg-facet-other: #77a9e8;` in the two dark arms. **This hex lives here, not in `graph.css`** — `graph.css:185` declares `--color-dark-graph-other: #6C7F75`, which the shipped sheet never paints.
- `style.css:265-270` — `.sec-tab, .subtab, #dxgOpen { align-items: center; gap: 0.4em; }`.
- `style.css:2128-2129` — the z-index ledger rows `80 graph pane root` / `81 graph pane surface`; the marker comment that explains the split (`/* … Its 80/81 pair lives in graph.css, not in this file — graph.css owns every #dxg / .dxg- selector — but the NUMBERS are recorded here, because a ledger split across two files is not a ledger. */`) runs `2131-2137`, with the `80/81` sentence on `2135`.
- `style.css:3401-3412` — `#dxgOpen, .sec-tab { display: flex; min-height: 35px; margin: 1px 0; padding: 8px 10px; border: 0; border-radius: 4px; color: var(--muted); font-size: 12px; line-height: 1.4 }`; the `:hover` pair at `3414-3418`.
- `style.css:4280` — the section marker: `/* Claims graph: a first-class System Record view, not a utility overlay. */`
- `style.css:4281-4510` — the whole desktop override block: `#dxgPane` `4281`, `.dxg-surface` `4287`, `.dxg-head` `4294`, `.dxg-title` `4302`, `.dxg-stamp` `4311`, `.dxg-head-actions` `4316`, `.dxg-controls` `4318` (a **grid**, with the seven `nth-child` spans at `4336-4342`), `.dxg-ctl` `4328`, `.dxg-ctl-label` `4344`, the control pad/hover cluster `4355-4385`, `.dxg-toggle` `4387-4398`, `.dxg-notice` `4400-4408`, `.dxg-notice-action` `4410-4414`, `.dxg-body` `4416`, `.dxg-canvas-holder` `4418`, `.dxg-canvas-hint` `4422-4429`.
- **`style.css:4431-4433`** — `/* Diagnostics are available elsewhere; this view reserves the rail for the selected node only and returns the rest of the width to the graph. */` then `.dxg-gaps { display: none !important; }` on `4433` — **the readings/gaps rail is hidden in the shipped viewer today.** Board 13 reinstates it as “WHAT THE LAYOUT FOUND”.
- `style.css:4435-4441` — `.dxg-rail { width: min(310px, 32%) }` (`4435-4439`) and `.dxg-rail:has(.dxg-detail-empty) { display: none }` (`4441`) — **the rail disappears entirely with nothing selected.**
- `style.css:4443-4478` — `.dxg-detail` `4443` (`padding: 18px`), `.dxg-detail-id` `4452` (sans 14px w700 `-.012em`), `.dxg-detail-rows` `4462` with `grid-template-columns: minmax(72px, auto) minmax(0, 1fr)` on `4463`, `dt` `4469` (sans 10px w680 `.04em` **uppercase**), `.dxg-detail-open` `4478` (`margin-top: 16px`).
- `style.css:4480-4505` — legend overrides; `.dxg-legend` `4480`, `.dxg-legend-list` `4486`, `.dxg-legend-item` `4487`, `.dxg-legend-group` `4489`, `.dxg-legend-swatch { width: 10px; height: 10px }` at `4495`; `.dxg-legend-module` at `4497-4505` is a **12px rounded square with a 1.5px `--ink` border** filled `var(--dxg-facet-other)` — the board draws an 11px circle instead.
- `style.css:4507-4510` — the shared `:focus-visible` outline for `.dxg-btn`, `.dxg-select`, `.dxg-toggle` (and `.comments-rail-close`).
- `style.css:4512-4580` — `@media (max-width: 860px)`: `.dxg-head` `min-height: 54px` at `4513-4516`, `.dxg-controls` becomes a horizontal **scroller** (`overflow-x: auto` `4526`, `scroll-snap-type: x proximity` `4530`) at `4521-4531`, `.dxg-ctl` `flex: 0 0 150px` at `4533-4537` with the `nth-child` basis overrides at `4539-4543`, `.dxg-btn/.dxg-select/.dxg-toggle { min-height: 38px }` at `4545-4547`, `.dxg-canvas-holder { min-height: 210px }` at `4557`, `.dxg-rail { max-height: 210px }` at `4559-4564` (`max-height` on `4562`), `.dxg-legend-list { flex-wrap: nowrap; width: max-content }` at `4579`.
- `style.css:4735` — the `@media (max-width: 560px)` block the mobile footer rules must out-specify (it owns `.claim-footer__counts`; it does not touch `.dxg-*`, but it is the breakpoint the 520 rules sit beside).

### Runtime

`internal/render/viewer/template/graph-core.js` (1713 lines) — pure computation,
no DOM. Its stated API is documented at `graph-core.js:26-60`: `EDGE_TYPES`,
`DIRECTED_EDGE_TYPES`, `GHOST_PREFIX`, `FACET_SLOT_COUNT` (20), `FACT_RULE_IDS`,
`HINT_RULE_IDS`, `OVERLAYS`, `BUILD_PHASES`, plus `scopeFilter`, `trackRole`,
`representatives`, `aggregateEdges`, `degrees`, `degreeFor`, `scc`, `selfEdges`,
`facetSlot`, `encodeState`.

`internal/render/viewer/template/graph-ui.js` (3864 lines) — every function this
screen depends on:

| Line | Function | Owns |
|---|---|---|
| `338` | `mountPane(pane)` | Builds every child of `#dxgPane` on first open. |
| `396` / `455` / `466` | `openPane()` / `closePane()` / `togglePane()` | The `Close Esc` control and `body.dxg-open`. |
| `538` / `544` / `557` | `controlGroup()` / `selectControl()` / `toggleButton()` | **§ 4.2** — the eyebrow + select + chip triple. |
| `565` | `buildControls()` | The six/seven control groups; `Module` `587`, `Facet` `595`. |
| `697` | `ensureTrackControl()` | The seventh group, present only with tracks. |
| `726` / `747` / `840` | `toggleType()` / `refreshControls()` / `onControlChange()` | Relation-chip state. |
| `956`–`1143` | `buildLegend()` and friends (`appendFacetRows` `1022`, `appendEdgeRows` `1068`, `appendOverlayRows` `1054`, `bindLegendHover` `1115`) | **§ 4.5.** |
| `1144` | `relativeStamp(iso)` | **§ 6 freshness** — `payload generated just now` / `N minutes ago` / `N hours ago` / `N days ago` (lines `1157-1168`). One unit, elapsed. |
| `1176` | `buildHeader()` | **§ 4.1.** |
| `1222` / `1239` | `maybeBuildRefresh()` / `doRefresh()` | `Refresh` is **absent** without a server, never disabled. |
| `1281`–`1300` | `showNotice` / `clearNotice` / `noticeRow` / `renderNotices` | **§ 4.3.** |
| `1357` / `1394` / `1434` | `renderEmptyScopeNotice()` / `scopeSelectionPhrase()` / `renderEmptyOverlayNotice()` | Empty-scope and empty-overlay states. |
| `1467` / `1602` | `applyPayload()` / `recompute()` | Scene assembly; ghost creation at `1632-1642`. |
| `1885` | `addGhost(nodes, byId, id, prefix)` | Out-of-scope endpoints. |
| `1904` / `1910` | `cssVar()` / `readPalette()` | **The only place graph colour is read.** Reads `--dxg-facet-1…20`, `--dxg-facet-other`, `--dxg-cycle`, `--dxg-halo`, `--dxg-governed`, `--ink`, `--muted`, `--faint`, `--paper`, `--accent`, `--link`, `--warn`. |
| `1950` / `1966` / `1986` | `facetSlotOf()` / `radiusOf()` / `baseFill()` | **§ 4.4** hue-is-identity, size-is-degree. `radiusOf` = `4.5 + 2.1·√degree`, clamped `[4, 26]`, ghost `3.5`. |
| `2003` / `2029` / `2054` | `nodeAlpha()` / `nodePath()` / `drawNodes()` | The moat ring (`r+1`, `--paper`, 2px), the draft-vs-locked fill opacity (`0.42`), the status ring (locked solid 1.6px `--ink`; draft dashed 1.4px; cycle 2.4px `--dxg-cycle`; ghost dashed `[2,2]` 1px `--muted`), the halo (`r+4`, review 2px solid / threads 1.2px dashed `[2,3]`), the selection ring (`r+7`, 1.6px `--accent`). |
| `2178` / `2189` | `drawTrackOwner()` / `haloKind()` | Track ownership; at most one halo, `review_pending` beating open threads. |
| `2290`–`2452` | `labelsVisible()` / `labelOf()` / `labelOrder()` / `drawLabels()` | The `labels` VIEW toggle and collision suppression. |
| `2532` / `2544` | `watchColorScheme()` / `onSchemeChange()` | Re-reads the palette on an OS theme change. |
| `2661`–`2785` | `edgeStroke()` / `drawEdges()` / `controlPoint()` / `chevron()` | **§ 4.4 edges**; `governed_by` is the curved one. |
| `2786` | `drawWedge()` | The “this node governs something” mark. |
| `3095`–`3272` | `screenToWorld` / `hitTest` / `bindCanvas` / `expandGroup` / `collapseGroup` / `select` | Interaction. |
| `3334` / `3403` | `renderGaps()` / `ruleBlock()` | **§ 4.9** — today's `Gaps in this view` rail, currently hidden by `style.css:4433`. |
| `3491` | `renderDetail()` | **§ 4.7** — the property rows, today lower-case and thirteen deep. |
| `3586` | the `.dxg-detail-open` anchor | **§ 4.8** — today the label is `open this claim in the reading view`. |
| `3631` / `3651` / `3668` / `3685` / `3696` | `degreeValue()` / `groupMemberCount()` / `facetNamesOf()` / `governorsOf()` / `governedOf()` | The values behind `DEGREE HERE` and `GOVERNED BY`. |
| `3728`–`3826` | `applyAutoCollapse()` / `renderCollapseNotice()` / `countGroups()` | Auto-collapse and its notice. |
| `3828` / `3842` / `3301` | `writeHash()` / `bindHashListener()` / `openFromHashOnLoad()` | Deep-link round-trip. |

Not used by this screen: `viewer-runtime.js`, `system-record.js`,
`build-order-ui.js`, `build-order.html`. The only cross-file coupling is
`shell.html:73`'s trigger and the reading view's hash-scroll path, which
`graph-ui.js:3586-3592` fires into.

### Tests that assert on these selectors today

- `viewer-tests/graph_pane_test.go:116-117` — `chromedp.Click("[data-dxg-open]")` then `waitVisible("#dxgPane .dxg-canvas")`. **M1 (no canvas at 390) must not break this** — it is a desktop-width test.
- `viewer-tests/graph_pane_test.go:154-158` — jump buttons located by `[data-dxg-rule=…] [data-dxg-jump]`, “off the STABLE rule id, never off display text”.
- `viewer-tests/graph_pane_test.go:279` — asserts the stylesheet resolves `--dxg-cycle` non-empty.
- `viewer-tests/graph_pane_test.go:382-387` — `[data-dxg-stamp]` `title` and `textContent`, the latter required to start with `payload generated`. **§ 6's `last read 2h ago` wording change touches this assertion.**
- `viewer-tests/graph_pane_test.go:460-461` — `.dxg-legend [data-dxg-facet]`.
- `viewer-tests/graph_rail_test.go:145`, `:266-272`, `:299`, `:339-358` — `#dxgPane [data-dxg-jump]`, `.dxg-detail-rows dt`, `.dxg-detail-rows .dxg-detail-note`, `.dxg-detail-id`, `.dxg-legend [data-dxg-edge]`, `.dxg-legend .dxg-legend-group`, `.dxg-legend [data-dxg-overlay-key]`.
- `viewer-tests/graph_canvas_test.go:158` — reads `--dxg-cycle`, `--dxg-halo`, `--dxg-governed` off the document.
- `viewer-tests/graph_canvas_test.go:353-356`, `:468-504` — `.dxg-detail-id`, `.dxg-canvas`, `.dxg-canvas-holder` geometry and DPR.
- `viewer-tests/graph_scope_test.go:124-133`, `:259-262`, `:278`, `:386` — `#dxgModule option`, `#dxgFacet option`, `#dxgPane .dxg-canvas` (exactly one), **`.dxg-controls .dxg-ctl` count must be 6**, `.dxg-notices .dxg-notice-action`, `.dxg-notices` text.
- `viewer-tests/graph_redrive_test.go:489` — `TestGraphPaneControlsMeetContrastAA`. **Every colour change in § 4.2 and § 4.3 must survive this.**
- `viewer-tests/graph_track_test.go:140,266`, `graph_parity_test.go:132`, `graph_offline_test.go:109`, `graph_rail_test.go:161,276,361`.
- `internal/render/theme_tokens_test.go:75-135` — the JS arm: every allowlisted theme token must have a consumer in `style.css`/`graph.css` **or** a `cssVar(cs, '--name')` call in `graph-ui.js`.
- `internal/render/theme_tokens_test.go:318-341` — `TestGraphCSSModeStructure`: exactly one light query in `graph.css`, it must carry `, print`, and there must be **no** dark query in `graph.css` at all.
- `internal/render/graph_render_test.go:166`, `:244`, `:275`, `:314`, `:348`, `:376` — client files verbatim, payload parses, hostile facet escaped, **block order**, pane mounts outside `.layout`, trigger is not a `.sec-tab`.

---

## 8. States not on the boards

Each entry states what the spec implies, derived from the rules — a lane must not
be left guessing.

1. **Nothing selected.** The engine hides the whole rail
   (`style.css:4441`, `.dxg-rail:has(.dxg-detail-empty) { display: none }`) and
   `renderDetail()` writes `select a node for its facet, status, degree and
   governors` (`graph-ui.js:3498`). The boards never show it because the screen
   is *opened from a claim*. **Spec:** the rail stays, and shows the empty line in
   `--color-faint` at the same 13/19 the values use; it must **not** collapse,
   because a rail that appears and disappears makes the canvas width jump. The
   readings block is independent of selection and stays visible.
2. **A cycle exists.** `--color-graph-cycle` / `--dxg-cycle` is drawn as a
   **2.4px ring** on every member node (`graph-ui.js:2103-2106`) and on the edge.
   The rail's `IN A CYCLE` value becomes `Yes` — and by the § 4.7 rule, a
   positive answer takes `--color-blocked`, not `--color-muted`, because muted is
   reserved for the quiet no. On dark, use `--dxg-cycle`'s dark value `#F5615C`,
   **never** Paper's `#FF6A62`. The notes flag cycle as the *first* hue to fix.
3. **A self-edge.** `graph-core.js` `selfEdges` and `graph-ui.js:3580` render
   `in a cycle: self-edge` (`3578` is the `governed by` row above it). **Spec:** the value is `Self-edge`, styled like the
   positive cycle answer.
4. **`review_pending` and its three triggers.** `haloKind()`
   (`graph-ui.js:2189-2203`) gives `review_pending` a **2px solid** halo that
   beats an open-comment thread's **1.2px dashed** one. The boards draw only the
   dashed/thread form. **Spec:** the solid halo is the second `MARKS` entry and
   must be added to the legend as `needs a human` in `--color-graph-halo`; the
   three triggers are not distinguished on the canvas — a halo says *a human is
   needed*, and which trigger is a rail/reading-view question.
5. **Zero open threads.** No halo, no ring. **Spec:** nothing is drawn and
   nothing is said; R-F.3's zero-thread chip has no analogue here because this
   screen has no comment count.
6. **Out-of-scope (ghost) nodes.** `graph-ui.js:1885` and `:2081-2110`: a ghost
   is `r=3.5`, filled `--paper`, ringed **1px dashed `[2,2]` `--muted`**, and
   **unlabelled**. The boards' scope notice promises outlines and the canvas draws
   none. **Spec:** implement the engine's form — a hollow, dashed, muted, small,
   unlabelled dot. The mobile board's facet-coloured solid outline ring
   (`APU-0`) is **wrong** and must not be copied; on mobile the same node gets the
   ghost treatment in `--color-faint`, not in a facet hue, because a ghost has no
   known facet.
7. **A collapsed group that is not all-locked.** `statusOf()` +
   `graph-ui.js:2090-2097`: the group is drawn at **0.42 fill opacity** because
   “this module is locked” is a claim about *all* of it. The boards draw only the
   solid form. **Spec:** pale fill, same hue, same radius.
8. **Draft claims.** Same `0.42` opacity plus a **dashed status ring** whose dash
   segment scales with the radius (`graph-ui.js:2116-2121`). Only the mobile row
   shows a draft, and it shows it as a **word** (`Contract · Draft` in
   `--color-draft`). **Spec:** both channels — pale fill + dashed ring on the
   canvas, `--color-draft` sub-line on a row.
9. **Empty scope intersection.** `renderEmptyScopeNotice()`
   (`graph-ui.js:1357-1381`) names **both** narrowed axes and offers
   `widen the scope`. **Spec:** it reuses the § 4.3 notice band exactly — same
   strip, same draft ground, same accent action. A second empty-state device is
   forbidden: “a pane with two ways of saying *nothing here* ends up saying it
   two different ways.”
10. **Empty overlay.** `renderEmptyOverlayNotice()` (`graph-ui.js:1434`) — the
    overlay leaves the graph exactly as it would be with no overlay, and says so.
    **Spec:** same band. The six overlays (`graph-core.js` `OVERLAYS`) are not on
    any board at all and inherit § 4.5's legend grammar: the overlay's own rows
    replace the facet rows, and `.dxg-swatch-dim` covers “everything else”.
11. **The heuristics / guesses block.** `graph-ui.js:3379-3392` and
    `graph.css:1080-1109` (`.dxg-hints` `1080`, `.dxg-hints-caption` `1087`,
    `.dxg-hint-label` `1099`): heuristics are separated from facts, carry a `guess`
    badge, and are prefaced with *“Heuristics — guesses about this project, not
    findings. They block nothing, and some of them are wrong by construction.”*
    The board shows `3 of 6 rules` with no hint block. **Spec:** the board's
    readings block is the **facts** half; the hints half keeps its caption, its
    badge and its visual separation, and the `N of M rules` count must state
    which half it counts.
12. **A rule that found nothing.** `graph.css:1037-1039`
    (`.dxg-rule--empty .dxg-rule-head`) — it still renders, dimmed; *“an empty cycle block is a result, not a blank”*. The
    board collapses all three into `3 rules found nothing`. **Spec:** the
    collapsed tail is the rest state; expanded, each empty rule renders as a
    dimmed `.dxg-rule` with a `0` count, and `G-01 · No dependency cycles` proves
    the pattern — it is a *found-nothing* rule shown expanded because a checked
    absence is a result.
13. **`+N more` on a long rule.** `graph.css:1072-1076` (`.dxg-rule-more`),
    `graph-ui.js:3427`. **Spec:** mono, `--color-faint`, same size as a rule id.
14. **Long claim titles.** `.dxg-detail-id` sets `overflow-wrap: anywhere`
    (`graph.css:1127-1133`, the declaration on `1132`) and `.dxg-detail-rows dd`
    the same (`graph.css:1152-1156`, the declaration on `1155`).
    **Spec:** the rail title wraps and never truncates; the **canvas** label is
    the one place truncation is allowed, because `drawLabels()` already suppresses
    colliding labels rather than shrinking them. A mobile row title wraps to as
    many lines as it needs — the row is `align-items: start` with the dot pinned
    at `margin-top: 3px` precisely so a three-line title still hangs correctly.
15. **Tracks.** A seventh control group appears only with tracks
    (`graph-ui.js:697`), and an owning claim gets an **inner ruling** in `--ink`
    (`graph-ui.js:2178`, `drawTrackOwner`) — deliberately not a colour, so
    owns-vs-cites survives colour blindness, print and both themes. The rail gains
    a `TRACKS` row. **Spec:** all of this is additive; a project without tracks
    gets exactly the boards as drawn.
16. **Served vs. file://.** `Refresh` is absent, not disabled, without a server
    (`graph-ui.js:1222`); per R10.5 the freshness line reads **`Live`** behind
    `dossierx serve`. **Spec:** `828 claims · Live` in served mode, the elapsed
    form otherwise.
17. **Refresh failure.** `doRefresh()` (`graph-ui.js:1239`) posts a transient
    notice through the same band. **Spec:** `--color-blocked` ground
    (`.dxg-notice--warn`, `graph.css:582-585`), not the draft ground.
18. **Unresolved edges.** `renderNotices()` (`graph-ui.js:1313-1325`) —
    *“N edges point at an id this project does not define, and is not drawn”*, in
    the warn variant. Not on any board. **Spec:** blocked ground, stacked above
    the scope notice.
19. **No payload at all.** `graph-ui.js:1306-1309` —
    *“this document carries no readable graph payload”*, warn variant, and no
    canvas. **Spec:** the pane still opens and still closes; the header and the
    `Close` control are never conditional.
20. **Hover on a legend entry.** `graph.css:808-823` dims every other entry and
    `bindLegendHover()` (`graph-ui.js:1115`) dims non-matching nodes. **Spec:**
    dimming, never hiding — “a reader must still be able to see the shape they
    are filtering against”. No hover state is drawn on any board; this is a
    `(pointer: fine)` affordance only and must not be the sole channel for
    anything.
21. **Focus-visible.** `style.css:4507-4510` gives `.dxg-btn`, `.dxg-select`,
    `.dxg-toggle` a `2px --accent` outline at `offset: 2px`; `graph.css:490-493`
    gives `.dxg-select:focus` a `-1px` inset outline. Not drawn. **Spec:** keep both;
    the boards' rest states say nothing about focus and must not be read as
    removing it.
22. **Auto-collapse to module granularity on a large corpus.** `applyAutoCollapse()`
    / `renderCollapseNotice()` (`graph-ui.js:3728`, `:3744`), proved by
    `viewer-tests/graph_pane_test.go:617`. The board shows **one** collapsed group
    among 26 individual claims. **Spec:** the collapsed-module node form in § 4.4
    is the same at any scale; only how many of them there are changes.

---

## 9. Open decisions

Decided here rather than asked, per the freeze protocol.

- **D1 — Rail width: the board's 348px wins over the engine's 310px.** The engine
  clamps to `min(310px, 32%)` (`style.css:4435-4439`, the `width` on `4436`); the board measures a flat
  `348px` at 1440 (`3VZ-0`). The board's rail carries a six-row property list at a
  fixed `96px` label column plus serif prose, and 310px cannot hold
  `Protection language requires current evidence` on two lines beside a 96px
  label. Ship `min(348px, 32%)`.
- **D2 — Reinstate the readings rail.** `style.css:4433` currently hides
  `.dxg-gaps` outright. Board 13 designs it back in, retitled
  `WHAT THE LAYOUT FOUND`, with a `N of M rules` count, serif bodies and a
  collapsed found-nothing tail. The board is newer than the hide and states an
  intent the hide does not contradict (the hide's reason was *“diagnostics are
  available elsewhere”*; the board's reason is *“readings about this picture, not
  findings about the corpus”* — a different claim). **Decision: reinstate.** The
  hints/heuristics half stays separated per § 8.11.
- **D3 — The rail never disappears.** Remove
  `.dxg-rail:has(.dxg-detail-empty) { display: none }` (`style.css:4441`). A rail
  that vanishes makes the canvas width jump between selections, which is a layout
  shift in the middle of a reading task. Empty state per § 8.1.
- **D4 — Rail property rows: six, in the board's order, in the board's
  vocabulary.** The engine renders up to thirteen lower-case rows
  (`graph-ui.js:3550-3579`). The board shows six: `MODULE`, `FACET`, `STATUS`,
  `GOVERNED BY`, `DEGREE HERE`, `IN A CYCLE`. **Decision: the board wins on
  order, case and count for the six it names; the remaining engine rows —
  `kind`, `build role`, `degree (project)`, `tracks`, `review pending`,
  `open threads`, `governs` — are retained below them in the same row form**,
  because deleting a fact from a diagnostic rail is a data loss the board did not
  argue for. `DEGREE HERE` is the board's rename of `degree (view)`; the engine's
  own comment (`graph-ui.js:3618-3630`) insists the row must name *whose* degree
  it is, and `HERE` does that.
- **D5 — Back-link wording: `Back to this claim in the reading view`.** The
  engine says `open this claim in the reading view` (`graph-ui.js:3586`). The
  board's wording is better because the reader *arrived from* that claim — the
  header says so — and `Back` names the return. Change the string; keep
  `data-dxg-open-claim` and the anchor's hash behaviour untouched.
- **D6 — Freshness wording: `828 claims · last read 2h ago`.** The engine's
  stamp is `payload generated 2 hours ago` (`graph-ui.js:1157-1168`) and
  `viewer-tests/graph_pane_test.go:387` asserts the `payload generated` prefix.
  The board's form is shorter, adds the corpus count, and abbreviates the unit.
  **Decision: adopt the board's wording and update that assertion**, keeping the
  absolute value on the `title` attribute (`graph-ui.js` `.dxg-stamp`,
  `cursor: help`) so nothing is lost. R10.3's one-unit rule is satisfied by both.
- **D7 — Collapsed-module swatch: a circle, not a square.** `style.css:4497-4505`
  draws `.dxg-legend-module` as a **12px rounded square with a 1.5px `--ink`
  border**; the boards draw an **11px circle** in `--color-graph-other`
  (`3V9-0`, `ASI-0`). **Decision: the board wins.** The canvas already
  distinguishes a group by *size*, and `nodePath()` draws a group as a rounded
  square on the canvas — a square legend swatch beside a circular canvas node in
  the same strip is the thing that confuses. Size alone carries it in the legend.
- **D8 — `re-run layout` label colour, reconciled with D14.** The board paints
  the OFF `VIEW` chip's label `--color-muted` (`3OW-0`) while the OFF
  `RELATIONS` chip's label is `--color-faint` (`3ON-0`) — two off-states, two
  colours, apparently one component. D14 dissolves the premise: they are **not**
  one component. `Says the same thing` is a toggle at rest and keeps
  `--color-faint` / `--color-dark-faint`, the quietest label on the bar, because
  an off toggle is a filter that is doing nothing. `re-run layout` is a button
  and takes the `.dxg-btn` label colour — `--color-ink` / `--color-dark-ink`
  (`graph.css:426`, `style.css:4364`, both `color: var(--ink)`) — because a
  button is always available and has no off state to be quiet about.
  **Decision: `--color-faint` for the OFF toggle, `--color-ink` for the button.**
  The board's `--color-muted` is wrong for either reading and is not adopted.
- **D9 — Mobile out-of-scope dot.** Per § 8.6 the board's facet-coloured outline
  is rejected in favour of the engine's hollow dashed muted form, rendered in
  `--color-faint`. A ghost has no known facet, so painting it a facet hue asserts
  something false — the same class of error R09.9 forbids.
- **D10 — “blocked by N” colour.** The rail paints
  `Locked · blocked by 2` entirely in `--color-locked` (`3WE-0`); the mobile edge
  rows paint `Internals · Locked · blocked by 4` entirely in `--color-blocked`
  (`APB-0`). **Decision: `--color-blocked` wins wherever the phrase
  `blocked by N` appears.** A blocked claim is blocked in both places; the rail is
  the inconsistent one. The bare `Locked` (no blockers) stays `--color-locked`.
- **D11 — Sticky behaviour is inherited, not measured.** Paper frames carry no
  `position` or `overflow`, so § 3's sticky/scroll paragraph is stated from the
  engine (`graph.css:640` `.dxg-rail`, `:667-670` `.dxg-gaps`, `:693-696`
  `.dxg-detail`, `:951-962` `.dxg-rail-head`) rather than measured. Recorded
  so a later reader does not mistake it for a Paper measurement.
- **D12 — The 390 canvas removal is not a fourth R-H.0 shape change.** R-H.0
  says only three components change shape below the phone breakpoint. The canvas
  does not change shape here; it is **not rendered**, and a substitute component
  (`AROUND THIS CLAIM`) takes its place. **Decision: record it as a
  not-rendered/substituted pair rather than amend R-H.0**, and flag it to the
  reference-rules owner.
- **D13 — Dark graph colour comes from `graph.css`, full stop.** Restated here
  because this is the screen where it bites: every `--color-dark-graph-*` value
  on these four boards is either a light hex or disagrees with the engine's
  maximin-solved ramp. Lane agents read `graph.css:161-231` (dark, with the
  eight `--color-dark-graph-*` hexes at `180-188`) and `graph.css:236-286`
  (light), and `--dxg-facet-other` from `style.css:76` / `:189` / `:215`, which
  is the declaration that wins.

- **D14 — The `VIEW` group is one toggle beside one button, and must be drawn
  that way.** R11.1 forbids *“two toggles producing four layouts”* and binds
  screen 13 by name. The boards draw `labels` (`3OT-0`) and `re-run layout`
  (`3OV-0`) in the identical pill — both `padding: 5px 11px`,
  `border-radius: 999px`, `1px solid`, differing only in ground and border
  colour — which reads as two independent toggles. The engine already disagrees:
  `graph-ui.js:648` builds `labels` through `toggleButton()` with
  `aria-pressed` and `data-dxg-labels`, while `graph-ui.js:653-661` builds
  `re-run layout` as `h('button', 'dxg-btn', …)` with `data-dxg-relayout` and a
  handler that resets `positions`, calls `recompute()`, `requestFit()` and
  `startLayout(true)` and stores nothing. **Decision: the engine wins on kind and
  the board wins on nothing here.** `labels` keeps the toggle pill;
  `re-run layout` ships as `.dxg-btn` (`graph.css:423-432`, `style.css:4355-4373`
  — `8px` radius in `graph.css`, flattened to `5px` by `style.css:4362`). The
  `VIEW` group then carries exactly one reversible state, R11.1 is satisfied on
  its own terms rather than by exception, and M4's drop at 520 removes one state
  and one action. Recorded as a Paper defect at **P14**.

### Paper defects

**P1 — The dark boards spell light graph tokens.** `A9B-0`
(`--color-graph-facet-2`), `A9E-0` (`-facet-3`), `A9K-0` (`-facet-5`), `A9N-0`
(`-graph-other`) on `A5J-0`; `AX9-0`, `AXC-0`, `AXI-0`, `AXL-0` on `AST-0`. A
light token on a dark board is a defect even where Paper's dark twin happens to
carry the same hex. Implement `graph.css`'s dark ramp.

**P2 — The dark canvas paints raw light hexes.** `A7D-0` fills nodes with
`#B65A34`, `#12897F`, `#67717E`, `#7D8C85` and rings halos with `#C07E0C` — five
literals, all light values, on `--color-dark-paper`. Four of the five dark
replacements are declared in `graph.css`: `--color-dark-graph-facet-3: #DE8A62`
(`graph.css:182`), `-facet-2: #3FB3A6` (`:181`), `-facet-5: #8E9AA8` (`:184`)
and `-graph-halo: #EFB44D` (`:187`). The fifth — the collapsed-module fill — is
**not** a `graph.css` value: `graph.css:185` declares
`--color-dark-graph-other: #6C7F75`, which the shipped sheet never paints,
because `style.css:189` and `style.css:215` redeclare `--dxg-facet-other:
#77a9e8` in the two dark arms and `style.css` is emitted after `graph.css`. So
the dark collapsed-module node is `#77a9e8`, addressed in `style.css`, exactly
as § 4.4 and D13 state.

**P3 — The light canvas is entirely raw hexes.** `3SY-0` uses `#C2CAD5`,
`#C11F5B`, `#4257C4`, `#12897F`, `#B65A34`, `#7050A8`, `#67717E`, `#7D8C85`,
`#C07E0C`, `#1C4E8C`, `#101720`, `#54606F` where twelve tokens exist. The values
are all correct; the spelling is not. `tokens.md` Disagreement 12: *implement the
token, never the literal.*

**P4 — The scope notice contradicts the canvas.** The notice
(`3P2-0`, `A78-0`, `AKM-0`) states nine out-of-scope blockers *“are drawn as
outlines”*. Neither desktop canvas draws a single outline node. The board makes a
promise it does not keep.

**P5 — The mobile note contradicts the desktop board.** The notes strip says
out-of-scope claims carry an outlined dot *“exactly as desktop draws them”*
(`AXY-0`, column 2). `Adding a permission` is outlined on mobile (`APU-0`) and a
solid `#4257C4` disc on desktop (`3SY-0`, node at `390,512`). One claim, two
drawings, one note asserting they match.

**P6 — Column 3's contrast argument is measured against the wrong values.** The
note quotes `#4257C4` and `#7050A8` as failing on `--color-dark-paper`, but the
dark canvas paints `--color-dark-graph-facet-1` and `-facet-4` for those two
(`A7D-0`). The note's numbers describe a board that does not exist; the four hues
that *are* thin on dark are the four in **P2**.

**P7 — `#F6F8FA` where `--color-code-bg` is `#F3F5F8`.** The mobile light
edge-group band (`AOG-0`, `AP2-0`, `AQ5-0`). Identical to `tokens.md`
Disagreement 12's board-12 finding; three units of drift in each channel, with a
token sitting right there. The dark twin (`AUL-0`) uses `#1B212B`, which *is*
`--color-dark-code-bg` — spelled as a literal.

**P8 — `#E8ECF1` where `--color-border` is `#DDE2E9`.** Every mobile light edge
row's hairline (`AOK-0` and siblings). The dark twin uses `#212934` where
`--color-dark-border` is `#242C38` — a *different* colour, not a literal of the
token. Both are defects; the dark one also changes the value.

**P9 — Two off-state colours for what the board draws as one chip family.**
`Says the same thing` (`3ON-0`) is `--color-faint`; `re-run layout` (`3OW-0`) is
`--color-muted`; both sit in the identical pill. The split is real but the
family is not — see P14. Resolved by D8 together with D14.

**P10 — `blocked by N` takes two different colours.** `--color-locked` in the
rail (`3WE-0`, `ANE-0`), `--color-blocked` in the mobile edge rows (`APB-0`,
`API-0`). Resolved by D10.

**P14 — An action is drawn as a toggle.** `re-run layout` (`3OV-0`) is drawn in
the OFF relation-chip pill — measured `padding: 5px 11px`,
`border-radius: 999px`, `1px solid --color-border-strong` — identical to
`Says the same thing` (`3OM-0`) and to the OFF form of a real toggle. It is not
a toggle: it holds no state (`graph-ui.js:653-661`). Drawing it as one makes the
`VIEW` group look like the two-independent-toggles shape R11.1 forbids, on a
screen R11.1 binds by name. Resolved by D14.

**P11 — The mobile board invents two claim titles.** Disclosed in the notes and
therefore not a hidden defect, but recorded: *“Permission prompts name the
mechanism”* and *“Recovery restores prior grants”* are not in the corpus. A lane
agent must not treat any mobile row title as data.

**P12 — Font-family spelling is not one family across the group.** The desktop
boards spell `var(--font-sans)` / `var(--font-mono)` / `var(--font-serif)`; the
mobile boards spell the literal `"Inter", system-ui, sans-serif` and
`"IBM Plex Mono", system-ui, sans-serif` while *still* using
`var(--font-serif)` for prose. Same group, three conventions. Matches
`tokens.md` Disagreement 12 and needs no new decision — the token wins.

**P13 — The pane is claimed to be theme-agnostic but the boards are not.** The
mobile board grounds the page in `--color-card` and the desktop board in
`--color-paper` (`AFZ-0` vs `3NL-0`). That is the intended difference, but it is
nowhere stated on the board or in the notes; recorded here as § 5 M16 so a lane
agent does not read it as a slip and "fix" it.
