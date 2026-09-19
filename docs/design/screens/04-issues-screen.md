# 04 · Issues screen

Screen group 04 of the viewer design revamp. Paper file
`01M2MTR6F73KHFR95ZWE8A7YAC`, page `1-0`.

Every number in this document was read with `get_computed_styles` or `get_jsx`
(`inline-styles`) on the node id printed beside it. Nothing here is measured off
a screenshot. Where a board prints a raw hex, the token name from
`docs/design/tokens.md` is given first and the board's hex second — the token is
what ships, per tokens.md § Disagreements item 12.

Read with `docs/design/tokens.md` and `docs/design/reference-rules.md`.

---

## 1. Boards

| Node | Name | What it shows | Viewer state depicted |
|---|---|---|---|
| `1GC-0` | 04 · Issues screen | 1440 × 1024 desktop, light. Left nav rail, page header (breadcrumb, `Issues`, serif subtitle, scope segmented control), severity filter strip + sort control, grouped findings card, right rail panel `WHERE THE BLOCKERS LIVE`. | Facet `Permission readiness · Contract`, every claim locked, 26 blocked claims across 36 paths, scope = **This facet**, sort = **Most claims blocked**, no filter chip pressed. Comments closed, focus mode off. |
| `5PG-0` | 04 · Issues screen · DARK | Same composition at 1440 × 1024 on dark ground. | Identical state to `1GC-0`; the theme toggle is the only difference. |
| `5VR-0` | 04 · Issues screen · MOBILE LIGHT | 390 × 844. Top app bar, page header with the count caveat moved under the subtitle, full-width scope segmented control, horizontally-running filter chips, sort row, edge-to-edge grouped list. No right rail. | Same facet and same counts; the rail is gone and the rows lead with the count. |
| `5YF-0` | 04 · Issues screen · MOBILE DARK | 390 × 844, dark. | Identical state to `5VR-0`. |
| `5OU-0` | Group 04 — band | The group's title band: eyebrow `04 · ISSUES` and the caption `four designs of one screen · light pair left, dark pair right`, over a 2px accent rule 3860px wide. | Not a screen. |
| `60T-0` | Group 04 — notes | Three note columns, 900px each, 64px apart: `WHAT THIS SCREEN ANSWERS`, `THE COUNTS DO NOT SUM`, `WHAT MOBILE DOES DIFFERENTLY`. | Not a screen. |
| `5P0-0` / `5P4-0` / `5P8-0` / `5PC-0` | Caption 04 — desktop light / mobile light / desktop dark / mobile dark | Per-board captions above each artboard. | Not screens. |
| `O2-0` | 08 · Severity & integrity (reference) | The five severity pills as specimens (`O8-0` Critical, `OC-0` Needs you, `OG-0` Blocker, `OK-0` Check, `OO-0` Later) and the `INTEGRITY — A DIFFERENT KIND OF FINDING` note. | Reference board. Binds this screen directly (R08.1, R08.2). |

---

## 2. Design intent

### What the reviewer should perceive

The screen answers exactly one question, and the notes strip says it in those
words: **"One question: which module do I chase."** (note `60T-0`, column 1).
Everything on the screen is arranged so that the answer arrives before the
reader has to read a single claim title:

- Findings are **grouped by the module that owns the blocking claim**, not by
  severity. The group header is the module name (`CAPABILITY SUPPORT` `1KO-0`,
  `VERIFICATION` `1LH-0`) and carries, hard right, how much of the facet it
  holds up — `blocks 24 of 26 claims here` (`1KQ-0`).
- Inside a group the rows are **sorted by that weight**, largest first: 24, 24,
  6 in `CAPABILITY SUPPORT`; 3, 4 in `VERIFICATION`. The sort control names the
  rule out loud: `Most claims blocked` (`1KH-0`).
- The right rail repeats the same four figures as a **ranking with bars**
  (`1M0-0`), "so it survives scrolling" (note `60T-0`, column 1). It is a
  restatement, not new data — which is why it is allowed to disappear at 390.

The severity vocabulary is a **filter**, not the organising axis. The chips sit
above the list and nothing on the board is pressed, so the default view is
"everything actionable, ranked by blast radius".

### The counts do not sum, and the screen says so

Note `60T-0` column 2 is the hardest rule on this screen: **24 + 7 + 3 + 2 = 36
against 26 blocked claims.** A claim blocked through two chains is counted under
both modules, which is what a reader chasing one module needs. The board refuses
to hide this: the caveat line sits directly under the rail heading
(`5OS-0`, desktop) and under the subtitle on mobile (`5X2-0` / `5YY-0`), reading
`26 claims blocked, 36 paths — a claim blocked through two modules counts under
both.`

The note also records a wording change with its reason: **the rail heading is no
longer "who owns the fix" — that wording promised one owner per claim.** It is
now `WHERE THE BLOCKERS LIVE` (`1M1-0`). A lane agent must not reintroduce
ownership language anywhere on this screen.

### Rules from `reference-rules.md` that bind this screen

- **R08.1 — five severities, two hues, no third hue.** The filter strip is the
  canonical instance. Measured against reference board 08 the four chips this
  board shows are pixel-identical to the specimens: `1JX-0` ≡ `OC-0` (Needs you,
  tinted blocked), `1K1-0` ≡ `OG-0` (Blocker, tinted draft), `1K5-0` ≡ `OK-0`
  (Check, card + border, draft dot), `1K9-0` ≡ `OO-0` (Later, card + border,
  border-strong dot) — same `5px/10px/12px` padding, same `7px` gap, same 6px
  dot, same `--radius-pill`. **Critical (`O8-0`, the only filled pill) is absent
  from all four boards** because the fixture has a zero count for it; it is
  specified in § 8 so no lane is left guessing.
- **R08.2 — a lint and a ledger finding must never read alike.** Binds "04, all
  four variants". Integrity findings sort first, take a red border rather than
  amber, and group under `APPROVAL RECORD` with four verdicts (missing,
  released, drifted, abandoned). **No board in group 04 depicts this.** See
  § 9 Paper defects and § 8.
- **R09.6 — Issues is a screen, and the banner is the only way in.** This screen
  is reached from the blocked banner and from nowhere else; there is no nav item
  for it in the left rail (`1GN-0` holds `MODULES` and `TRACKS` only, and the
  rail footer `1IO-0` holds only `Claims graph` and `Build order`). The engine
  already spells the entry point as `Show issues` (`shell.html:278`).
- **R09.8 — eight demotions.** Two land here directly. *"Claim id under each
  finding — swapped for the title: the reviewer recognises the title, the slug
  is for the agent."* The desktop row honours the swap and keeps the slug as a
  second, quieter line (`1KW-0` title over `1KX-0` slug); **mobile drops the
  slug entirely**, which the notes strip justifies (see § 5). *"Severity 'Later'
  — cut from the default view, behind the filter chips only."* The board is
  consistent with this: `Later` appears as a chip (`1K9-0`, count 1) and no
  `Later` row is in the list.
- **R09.9 — no inline dependency map.** There is none on this screen, and there
  must not be one. The breadcrumb states the path; the graph pane draws the
  shape.
- **R11.1 — one reversible state, not two independent toggles.** Its `Binds`
  line names "03 (all four variants), and the header on 02, 04, 13, 14"
  (`reference-rules.md:276`), so it binds **this screen's header** and this spec
  must say what it constrains here. It constrains what the header may not grow.
  `1JE-0` holds exactly three rows — breadcrumb `1JF-0`, title row `1JK-0`
  (title stack `1JL-0` + scope control `1JO-0`), filter/sort row `1JW-0` — and
  the dark twin `5SH-0` holds the same three (`5SI-0`, `5SN-0`, `5SY-0`).
  **Neither board carries a per-rail show/hide pair**, and that absence is the
  rule, not an omission: § 1 records this board's depicted state as *focus mode
  off*, so what is drawn is one side of R11.1's single reversible state, with
  both rails present (`1GD-0` and `1LZ-0`, measured in § 3). If focus is ever
  brought to this screen it is **one** control and **one** state, removing the
  left nav rail and the right ranking rail together; two toggles producing four
  layouts are forbidden here for
  the same reason they are forbidden on 03. *Intent: a reader who wants fewer
  distractions wants them all gone at once.* The 390 boards have no focus
  control at all — see § 5 (M12) — and the undrawn on-state is specified in
  § 8 item 14.
- **R-H.0 / R-I.0 — a screen must use the form that matches its width, and the
  mobile forms are not alternative components.** The mobile issue row here is a
  new arrangement of the same content (§ 5), not a new component.
- **R00.0 — the component board wins over a screen.** The severity chips on
  `1GC-0` agree with `O2-0` exactly, so nothing is in conflict there.

### Every note on the band and the notes strip, paraphrased with its intent

**Band `5OU-0`** — `04 · ISSUES`, "four designs of one screen · light pair left,
dark pair right". Intent: the four artboards are one screen in four
presentations, not four designs to choose between. A lane implements one screen.

**Note column 1, `WHAT THIS SCREEN ANSWERS`.** One question: which module do I
chase. Grouping is by owning module, each group states how much of the facet it
holds up, rows are sorted by that weight, and the rail repeats the figures as a
ranking so the answer survives scrolling. *Intent: the reader's decision is
"where do I go next", so the screen is organised by destination, not by taxonomy.*

**Note column 2, `THE COUNTS DO NOT SUM`.** Eyebrow is set in
`--color-blocked` (`60T-0` column 2 heading) — the board flags its own arithmetic
as a hazard. 36 paths against 26 claims; double-counting is correct for a reader
chasing one module; the rail therefore states the discrepancy in a line under its
heading "rather than quietly showing a total that cannot be right"; and the
heading was changed away from "who owns the fix" because that wording promised
one owner per claim. *Intent: a number a reader can falsify by adding up the
column destroys trust in every other number on the screen.*

**Note column 3, `WHAT MOBILE DOES DIFFERENTLY`.** The rail goes, because every
figure in it is already in the group headers a few lines below; only its caveat
is kept, moved under the subtitle. Rows lead with the count and drop the claim
slug — the group header already carries the module prefix, and "five wrapped mono
lines would bury the titles". The filter chips run off the right edge instead of
wrapping. *Intent: each mobile change is justified by something the narrow screen
already says, not by a space budget.*

### Where a board contradicts its own notes or its reference board

Recorded here, itemised in § 9:

1. **R08.2 binds 04 and no variant shows it.** No `APPROVAL RECORD` group, no
   red-bordered integrity row, none of the four verdicts. The board shows a
   pure-lint / pure-readiness view.
2. **The dark boards carry three unbound literals** — `#080B10` (segmented
   trough, `5SR-0` / `5YZ-0`), `#1B212B` (group header ground, `5TO-0` /
   `5ZU-0`) and `#212934` (row divider, `5ZX-0` / `60F-0`). `#1B212B` is
   `--color-dark-code-bg`'s value written as a literal; the other two have no
   token at all.
3. **The severity tints on the boards are not the tokens.** Light `#9E3B361F`
   (≈12 %) against `--color-blocked-bg` (10 %); `#9A6A1621` (≈13 %) against
   `--color-draft-bg` (11 %). Dark `#EC8A8326` / `#DDA94E26` (≈15 %) against 13 %.
   Approval of the board is not approval of the alpha.

---

## 3. Layout

### Desktop, 1440 (boards `1GC-0` light / `5PG-0` dark)

| Region | Value | Node |
|---|---|---|
| Artboard | `1440 × 1024`, `display: flex`, `overflow: clip` | `1GC-0` / `5PG-0` |
| Page ground | `--color-paper` `#EFF1F4` / `--color-dark-paper` `#0D1117` | `1GC-0` / `5PG-0` |
| Left nav rail width | `268px`, `flex-shrink: 0`, full 1024 height | `1GD-0` / `5PH-0` |
| Left rail ground | `--color-paper` / `--color-dark-paper` (same as the page — the rail is not a raised surface) | `1GD-0` / `5PH-0` |
| Left rail edge | `border-right: 1px solid --color-border` / `--color-dark-border` | `1GD-0` / `5PH-0` |
| Main column | `flex: 1 1 0`, `min-width: 0`, `height: 1024px`, `overflow: clip`, ground `--color-paper` | `1JD-0` / `5SG-0` |
| Main column computed width | `1172px` (1440 − 268) | `1JD-0` |
| Header block | `padding-top: 32px`, `padding-inline: 40px`, `gap: 18px`, `flex-shrink: 0` | `1JE-0` / `5SH-0` |
| Header block height | `181px` | `1JE-0` |
| Header content width | `1092px` (1172 − 2 × 40) | `1JF-0`, `1JK-0`, `1JW-0` |
| Breadcrumb row | `display: flex`, `align-items: center`, `gap: 10px`, height 16 | `1JF-0` |
| Title row | `align-items: end`, `justify-content: space-between`, height 65 | `1JK-0` |
| Title stack | `flex-direction: column`, `gap: 7px` | `1JL-0` |
| Filter row | `align-items: center`, `gap: 8px`, `padding-bottom: 4px`, height 32 | `1JW-0` |
| Filter-to-sort spacer | zero-height `flex: 1 1 0` rule, computed `482px` on this fixture | `1KD-0` |
| Body region | `flex: 1 1 0`, `min-height: 0`, `overflow: clip`, `padding-top: 20px`, `padding-inline: 40px`, `gap: 20px` | `1KL-0` / `5TM-0` |
| Body region height | `843px` | `1KL-0` |
| Findings card | `flex: 1 1 0`, `min-width: 0`, computed `792 × 823` | `1KM-0` / `5TN-0` |
| Right rail column | `width: 280px`, `flex-shrink: 0`, `gap: 14px` | `1LZ-0` / `5UX-0` |
| Rail panel inner width | `242px` (280 − 2 × 18 − 2 × 1) | `1M2-0` |

**Sticky and scroll regions.** The body region `1KL-0` is the scroll region:
`flex: 1 1 0` with `min-height: 0` and `overflow: clip`, under a `flex-shrink: 0`
header. The header (`1JE-0`) — breadcrumb, title, subtitle, scope control, filter
strip and sort — is therefore **sticky by construction**: it never scrolls. The
findings card is bottom-open (`border-top-left-radius: 12px`,
`border-top-right-radius: 12px`, **both bottom radii `0px`** — `1KM-0`), which is
how the board says the list runs off the bottom edge rather than ending. The
right rail column (`1LZ-0`) is inside the same clipped region and scrolls with
it; its panel is not independently sticky on this board.

### Mobile, 390 (boards `5VR-0` light / `5YF-0` dark)

| Region | Value | Node |
|---|---|---|
| Artboard | `390 × 844`, `flex-direction: column`, `overflow: clip` | `5VR-0` / `5YF-0` |
| Page ground | `--color-card` `#FFFFFF` / `--color-dark-card` `#161B22` — **not** paper | `5VR-0` / `5YF-0` |
| Top app bar | `height: 52px`, `padding-inline: 16px`, `gap: 12px`, `flex-shrink: 0`, no border | `5VS-0` / `5YG-0` |
| Page header block | `padding-top: 16px`, `padding-inline: 16px`, `gap: 10px`, height 174 | `5W3-0` / `5YQ-0` |
| Side gutter | `16px` throughout; content measure `358px` | `5W4-0`, `5W9-0`, `5XE-0` |
| Scope segmented control | `margin: 14px 16px 0`, `padding: 3px`, `radius: 9px`, `gap: 3px`, width 358 | `5WC-0` / `5YZ-0` |
| Filter chip rail | `margin-top: 14px`, `margin-left: 16px`, **no right margin**, `gap: 8px`, width `374px` on a 390 artboard | `5WK-0` / `5Z6-0` |
| Sort row | `margin: 16px 16px 0`, `gap: 8px`, height 28 | `5X4-0` / `5ZN-0` |
| Issue list | `margin-top: 14px`, `flex-direction: column`, **full-bleed 390** | `5XB-0` / `5ZT-0` |
| Group header | `padding: 10px 16px`, `gap: 3px`, 1px top and bottom rules | `5XD-0` / `5ZU-0` |
| Issue row | `padding: 12px 16px`, `gap: 5px`, 1px bottom rule | `5XH-0`, `5XN-0`, `5Y3-0` / `5ZX-0`, `60F-0` |
| Right rail | absent | — |

The mobile list is **full-bleed**: the list frame is 390 wide while every row's
content is inset 16px. There is no card, no radius and no side border — the group
headers' top/bottom hairlines are the only structure.

---

## 4. Components on this screen

**Count recorded.** This section carries **108 measured rows**, each one a
property read with `get_computed_styles` or `get_jsx` from the node named beside
it; most carry a light value and its dark twin, so the value count is higher than
the row count. § 3 adds **20** measured geometry values for the 1440 boards and
**12** for the 390 boards, and § 5 adds **13** measured mobile rules — **153
measured rows in total for group 04**, of which 128 are read off the desktop
light board `1GC-0` and its dark twin `5PG-0`. Nothing in this document is
estimated from a screenshot. Node ids cited for **structure only** — § 2's
R11.1 bullet (`1JE-0` / `5SH-0` and their three children each) and § 4.10's
dark-twin note (`5PI-0`, `5PL-0`, `5PR-0`, `5RS-0`) — were read with
`get_tree_summary` and are not counted above; completing a dark citation on an
existing row (§ 4.8's divider, § 4.9's caveat) does not add a row either.

### 4.1 Breadcrumb (`1JF-0` desktop / `5W4-0` mobile)

| Property | Light | Dark | Node |
|---|---|---|---|
| Back chevron | `13 × 13` SVG, `stroke-width: 2.4`, `stroke-linecap: round`, path `m15 18-6-6 6-6` | same geometry | `1JG-0` / `5SJ-0` |
| Chevron stroke | `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `1JG-0` / `5SJ-0` |
| Facet label | Inter 13/16, weight 500, `--color-muted` `#54606F` | `--color-dark-muted` | `1JI-0` / `5SL-0` |
| Facet suffix (`· Contract`) | Inter 13/16, weight 400, `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `1JJ-0` / `5SM-0` |
| Row gap | `10px` | same | `1JF-0` |

Mobile is identical except the row gap is `8px` (`5W4-0`) — text sizes,
weights and colours do not change (`5W7-0`, `5W8-0`).

### 4.2 Page title and subtitle (`1JL-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Title `Issues` | Inter **28/34**, weight 600, tracking `-0.02em` | — | `1JM-0` / `5SO-0`'s title |
| Title colour | `--color-ink` `#101720` | `--color-dark-ink` `#E6EAF0` | `1JM-0` |
| Subtitle | **Source Serif 4 16/24**, weight 400 | — | `1JN-0` |
| Subtitle colour | `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `1JN-0` |
| Title→subtitle gap | `7px` | same | `1JL-0` |

The subtitle is the one piece of running prose on the screen and takes the serif,
per tokens.md § 7: it is read for meaning, not scanned.

### 4.3 Scope segmented control (`1JO-0` desktop / `5WC-0` mobile)

Three segments, fixed order **This facet · Module · Project**. One selected.

| Property | Light | Dark | Node |
|---|---|---|---|
| Trough ground | `#E3E7ED` — **no token** | `#080B10` — **no token**, darker than `--color-dark-paper` | `1JO-0` / `5SR-0` |
| Trough radius | `8px` (`--radius-md`) | same | `1JO-0` |
| Trough padding | `3px`; inter-segment `gap: 3px` | same | `1JO-0` |
| Segment padding | `5px` block / `12px` inline | same | `1JP-0` |
| Segment radius | `6px` | same | `1JP-0` |
| **Selected** segment ground | `--color-card` `#FFFFFF` | `--color-dark-card` `#161B22` | `1JP-0` / `5SR-0`'s first child |
| **Selected** label | Inter 13/16 weight **500**, `--color-ink` | `--color-dark-ink` | `1JQ-0` |
| **Unselected** segment ground | none (transparent) | none | `1JR-0`, `1JT-0` |
| **Unselected** label | Inter 13/16 weight **400**, `--color-muted` | `--color-dark-muted` | `1JS-0`, `1JU-0` |
| Control height | `32px` desktop, `30px` per segment on mobile | same | `1JO-0` / `5WD-0` |

No border, no shadow, on either mode. Selection is said with a raised ground and
one weight step, nothing else.

### 4.4 Severity filter chips (`1JW-0`; chips `1JX-0`, `1K1-0`, `1K5-0`, `1K9-0`)

All four chips share a shell, measured identically on board 04 and on reference
board 08:

| Shell property | Value | Node |
|---|---|---|
| Padding | `5px` block, `10px` left, `12px` right | `1JX-0`, `1K1-0`, `1K5-0`, `1K9-0` |
| Radius | `999px` (`--radius-pill`) | all four |
| Internal gap | `7px` | all four |
| Dot | `6 × 6`, `border-radius: 999px`, `flex-shrink: 0` | `1JY-0`, `1K2-0`, `1K6-0`, `1KA-0` |
| Label | Inter 12/16 | all four |
| Count | **IBM Plex Mono** 12/16, weight 400 | `1K0-0`, `1K4-0`, `1K8-0`, `1KC-0` |
| Strip gap | `8px` | `1JW-0` |

Per-variant, light then dark:

| Variant | Ground | Border | Dot | Label | Count |
|---|---|---|---|---|---|
| **Needs you** `1JX-0` / `5SZ-0` | `--color-blocked-bg` (board `#9E3B361F` ≈12 %) / dark `--color-dark-blocked-bg` (board `#EC8A8326` ≈15 %) | none | `--color-blocked` `#9E3B36` / `--color-dark-blocked` `#EC8A83` | w **600**, `--color-blocked` / `--color-dark-blocked` (`1JZ-0` / `5T0-0`) | mono, `--color-blocked` / `--color-dark-blocked` (`1K0-0`) |
| **Blocker** `1K1-0` / `5T3-0` | `--color-draft-bg` (board `#9A6A1621` ≈13 %) / `--color-dark-draft-bg` (board `#DDA94E26` ≈15 %) | none | `--color-draft` `#9A6A16` / `--color-dark-draft` `#DDA94E` | w **600**, `--color-draft` / `--color-dark-draft` (`1K3-0`) | mono, `--color-draft` / `--color-dark-draft` (`1K4-0`) |
| **Check** `1K5-0` / `5T7-0` | `--color-card` `#FFFFFF` / `--color-dark-card` `#161B22` | `1px solid --color-border` `#DDE2E9` / `--color-dark-border` `#242C38` | `--color-draft` / `--color-dark-draft` (`1K6-0`) | w **500**, `--color-muted` / `--color-dark-muted` (`1K7-0`) | mono, `--color-faint` / `--color-dark-faint` (`1K8-0`) |
| **Later** `1K9-0` / `5TB-0` | `--color-card` / `--color-dark-card` | `1px solid --color-border` / `--color-dark-border` | `--color-border-strong` `#C2CAD5` / `--color-dark-border-strong` `#38424F` (`1KA-0`) | w **500**, `--color-muted` / `--color-dark-muted` (`1KB-0`) | mono, `--color-faint` / `--color-dark-faint` (`1KC-0`) |
| **Critical** (not on 04; spec from `O8-0`) | filled `--color-blocked` `#9E3B36` | none | `#FFFFFF` (`O9-0`) | w 600, `#FFFFFF` (`OA-0`) | mono, `#FFFFFFB8` — white at 72 % (`OB-0`) |

**States.** No chip is pressed on any board, so only the *unpressed* state is
measured. The pressed state is specified in § 8.

### 4.5 Sort control (`1KE-0`, `1KG-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| `Sort` label | Inter 12/16, weight 400, `--color-faint` `#6E7C8E` | `--color-dark-faint` | `1KF-0` / `5TG-0`'s label |
| Label→control gap | `6px` | same | `1KE-0` |
| Control ground | `--color-card` `#FFFFFF` | `--color-dark-card` | `1KG-0` |
| Control border | `1px solid --color-border` `#DDE2E9` | `--color-dark-border` | `1KG-0` |
| Control radius | **`7px`** (off-scale; between `--radius-sm` and `--radius-md`) | same | `1KG-0` |
| Control padding | `5px` block / `10px` inline | same | `1KG-0` |
| Control gap | `5px` | same | `1KG-0` |
| Value label | Inter 12/16, weight 500, `--color-ink` `#101720` | `--color-dark-ink` | `1KH-0` |
| Chevron | `11 × 11`, path `m6 9 6 6 6-6`, `stroke-width: 2.6`, round caps | same | `1KI-0` |
| Chevron stroke | `--color-faint` `#6E7C8E` | `--color-dark-faint` | `1KI-0` |
| Control height | `28px` | same | `1KE-0` |

### 4.6 Findings card (`1KM-0` / `5TN-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Ground | `--color-card` `#FFFFFF` | `--color-dark-card` `#161B22` | `1KM-0` / `5TN-0` |
| Border | `1px solid --color-border` `#DDE2E9` | `--color-dark-border` `#242C38` | `1KM-0` / `5TN-0` |
| Radius | top `12px` / `12px`, **bottom `0px` / `0px`** | same | `1KM-0` / `5TN-0` |
| Overflow | `clip` | same | `1KM-0` |
| Shadow | **none** | none | `1KM-0` |

`12px` is off the radius scale (`--radius-md` is 8) and is the card radius this
screen uses; record it as a per-component literal per tokens.md § Open decisions.

### 4.7 Module group header (`1KN-0`, `1LG-0` / `5TO-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Ground | `#F6F8FA` — **no token**; nearest is `--color-code-bg` `#F3F5F8` | `#1B212B` — literal equal to `--color-dark-code-bg` | `1KN-0` / `5TO-0` |
| Padding | `13px` block / `20px` inline | same | `1KN-0` |
| Gap | `10px` | same | `1KN-0` |
| Bottom rule | `1px solid --color-border` | `--color-dark-border` | `1KN-0` |
| Top rule (second group onward) | `1px solid --color-border` | `--color-dark-border` | `1LG-0` |
| Module name | Inter **11/14**, weight 600, tracking `0.08em` (`--tracking-label`), uppercase copy | same metrics | `1KO-0`, `1LH-0` |
| Module name colour | `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `1KO-0` / `5TO-0`'s label |
| Filler rule | `1px` high, `flex: 1 1 0`, `--color-border` `#DDE2E9` | `--color-dark-border` | `1KP-0`, `1LI-0` |
| Weight phrase | Inter 12/16, weight 400, `--color-faint` `#6E7C8E` | `--color-dark-faint` | `1KQ-0`, `1LJ-0` |
| Header height | `43px` first, `44px` subsequent (the extra 1px is the top rule) | same | `1KN-0`, `1LG-0` |

### 4.8 Issue row — desktop (`1KT-0`, `1L0-0`, `1L7-0`, `1LK-0`, `1LR-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Padding | `14px` block / `20px` inline | same | `1KT-0` / `5TS-0`'s rows |
| Gap (dot → text → count) | `14px` | same | `1KT-0` |
| Alignment | `align-items: start` | same | `1KT-0` |
| Divider | `1px solid #E8ECF1` — **no token**, lighter than `--color-border` | `1px solid #212934` — **no token** | light `1KT-0`; dark desktop `5TT-0`, `5U0-0`, `5UJ-0`; dark mobile `5ZX-0`, `60F-0` |
| **Last row in a group carries no divider** | `1L7-0`, `1LR-0` have no `border-bottom` | `5U7-0`, `5UQ-0` have no `border-bottom` | `1L7-0`, `1LR-0` / `5U7-0`, `5UQ-0` |
| Severity dot | `6 × 6`, radius `999px`, `margin-top: 6px` (optically centred on the 20px title line) | same | `1KU-0`, `1L1-0`, `1L8-0`, `1LL-0`, `1LS-0` |
| Dot colour — Blocker rows | `--color-draft` `#9A6A16` | `--color-dark-draft` `#DDA94E` | `1KU-0`, `1L1-0`, `1L8-0`, `1LS-0` |
| Dot colour — Needs-you row | `--color-blocked` `#9E3B36` | `--color-dark-blocked` `#EC8A83` | `1LL-0` |
| Text column | `flex: 1 1 0`, `min-width: 0`, `gap: 4px` | same | `1KV-0` |
| Finding title | Inter **14/20**, weight 500, `--color-ink` `#101720` | `--color-dark-ink` | `1KW-0`, `1L3-0`, `1LA-0`, `1LN-0`, `1LU-0` |
| Claim slug | **IBM Plex Mono 11/14**, weight 400, `--color-faint` `#6E7C8E` | `--color-dark-faint` `#8494A8` | `1KX-0`, `1L4-0`, `1LB-0`, `1LO-0`, `1LV-0` |
| Count slot | fixed `width: 104px`, `flex-shrink: 0`, `justify-items: end` | same | `1KY-0`, `1L5-0`, `1LC-0`, `1LP-0`, `1LW-0` |
| Count label | Inter 13/16, weight **600**, `text-align: right` | same | `1KZ-0`, `1L6-0`, `1LD-0`, `1LQ-0`, `1LX-0` |
| Count colour | matches the row's severity: `--color-draft` or `--color-blocked` | `--color-dark-draft` / `--color-dark-blocked` | `1KZ-0` (draft), `1LQ-0` (blocked) |
| Row height | `67px` (two-line title), `66px` (last in group, no divider) | same | `1KT-0`, `1L7-0` |

The `104px` fixed count slot is the vertical-lane rule: the counts form a lane
whatever the title length, and the slot is present even when the number is one
digit.

### 4.9 Right rail panel — `WHERE THE BLOCKERS LIVE` (`1LZ-0` / `1M0-0`)

| Property | Light | Dark | Node |
|---|---|---|---|
| Panel ground | `--color-card` `#FFFFFF` | `--color-dark-card` `#161B22` | `1M0-0` / `5UY-0` |
| Panel border | `1px solid --color-border` | `--color-dark-border` | `1M0-0` / `5UY-0` |
| Panel radius | `12px` (all four corners — unlike the list card) | same | `1M0-0` |
| Panel padding | `18px` top / `20px` bottom / `18px` inline | same | `1M0-0` |
| Panel gap | `12px` | same | `1M0-0` |
| Heading | Inter 11/14, weight 600, tracking `0.08em` | same | `1M1-0` |
| Heading colour | `--color-faint` `#6E7C8E` (**not** `--color-muted` like the group headers) | `--color-dark-faint` `#8494A8` | `1M1-0` / `5UY-0`'s heading |
| Caveat line | Inter **11/16**, weight 400, `--color-faint`; `margin-top: -6px`, `margin-bottom: 2px` | `--color-dark-faint`, same 11/16 and same `-6px` / `+2px` margins | light `5OS-0` (parent `1M0-0`, board `1GC-0`) / dark `5V0-0` (parent `5UY-0`, board `5PG-0`) |
| Rows container gap | `10px` | same | `1M2-0` |
| Row internal gap | `5px` (label line → bar) | same | `1M3-0` |
| Label/count gap | `8px`, `align-items: baseline` | same | `1M4-0` |
| Module label | Inter 13/16, weight 500, `--color-ink` `#101720` | `--color-dark-ink` | `1M5-0`, `1MB-0`, `1MH-0`, `1MN-0` |
| Count | **IBM Plex Mono 12/16**, weight 400, `--color-muted` `#54606F` | `--color-dark-muted` `#9AA7B8` | `1M6-0`, `1MC-0`, `1MI-0`, `1MO-0` |
| Bar track | `height: 5px`, radius `999px`, `overflow: clip`, ground `--color-paper` `#EFF1F4` | `--color-dark-paper` `#0D1117` | `1M7-0`, `1MD-0`, `1MJ-0`, `1MP-0` |
| Bar fill | `height: 5px`, `--color-draft` `#9A6A16` | `--color-dark-draft` `#DDA94E` | `1M8-0`, `1ME-0`, `1MK-0`, `1MQ-0` |
| Bar fill widths | `round(92%, 1px)`, `round(27%, 1px)`, `round(12%, 1px)`, `round(8%, 1px)` — proportions of the **largest count**, not of the total | same | `1M8-0`, `1ME-0`, `1MK-0`, `1MQ-0` |
| Panel height | `256px`; the rail column is `280 × 823` with `gap: 14px` for a second panel that does not exist on this fixture | same | `1M0-0`, `1LZ-0` |

The bar fill is **always draft amber**, even on the row whose list contains a
blocked-severity finding (`Verification`). The rail ranks *volume*, not severity;
severity is said by the dot in the list. Do not colour these bars per severity.

### 4.10 Left nav rail, as this screen composes it (`1GD-0`)

Measured here because the rail is part of the 1440 composition; the rail
component itself belongs to group 02.

**This subsection is light-only by construction**, unlike the rest of § 4. The
dark twins exist on `5PG-0` — header block `5PI-0`, search `5PL-0`, nav scroll
region `5PR-0`, footer `5RS-0`, under the rail frame `5PH-0` whose ground and
`border-right` are measured in § 3 — but the rail is group 02's component and
its dark values are group 02's to specify. Cited here so a lane does not read
the missing dark column as an undrawn state.

| Property | Light | Node |
|---|---|---|
| Header block | `padding: 24px 20px 20px`, `gap: 3px` | `1GE-0` |
| Project title | Inter 19/24, weight 600, tracking `-0.01em`, `--color-ink` | `1GF-0` |
| Project eyebrow | Inter 12/16, weight 400, `--color-faint` | `1GG-0` |
| Search field | `height: 34px`, `padding-inline: 10px`, radius `8px`, `gap: 8px`, `--color-card` ground, `1px --color-border` | `1GI-0` |
| Search placeholder | Inter 14/18, weight 400, `--color-faint`; copy `Search 828 claims` | `1GM-0` |
| Nav scroll region | `flex: 1 1 0`, `min-height: 0`, `overflow: clip`, `padding-inline: 12px`, `gap: 4px` | `1GN-0` |
| Group label `MODULES` | Inter 11/14, weight 600, tracking **`0.09em`**, `--color-faint` | `1GP-0` |
| Group count | IBM Plex Mono 11/14, `--color-faint` | `1GQ-0` |
| Nav row | `height: 32px`, `padding-inline: 10px`, radius `6px` | `1GU-0`, `1HA-0` |
| Nav row label | Inter 14/18, weight 400, `--color-muted` | `1GV-0` |
| Nav row count | `width: 16px`, mono 11/14, right-aligned, `--color-faint` | `1GW-0` |
| Nav row with padlock | same row, plus `gap: 6px` for the lock glyph slot | `1HA-0` |
| Rail footer | `padding: 16px 20px 20px`, `gap: 14px`, `border-top: 1px solid --color-border` | `1IO-0` |
| Footer link row | `height: 30px`, `gap: 9px`, icon `15 × 15` `stroke-width: 2`, label Inter 14/18 `--color-muted` | `1IQ-0`, `1IW-0` |
| Theme toggle trough | `#E3E7ED`, radius `8px`, `padding: 3px` | `1J2-0` |
| Theme toggle segment | `height: 28px`, radius `6px`, `gap: 6px`, icon `13 × 13` | `1J3-0`, `1J8-0` |
| Theme toggle — selected | ground `--color-card`, icon stroke `--color-ink`, label Inter 13/16 weight 500 `--color-ink` | `1J3-0` |
| Theme toggle — unselected | no ground, icon stroke `--color-muted`, label Inter 13/16 weight 400 `--color-muted` | `1J8-0` |

### States visible across the four boards

- Scope segmented control: **selected** (`1JP-0`) and **unselected** (`1JR-0`,
  `1JT-0`).
- Theme toggle: **selected** (`1J3-0`) and **unselected** (`1J8-0`).
- Severity chips: **unpressed only**, in four of five variants.
- Issue rows: **last-in-group** (no divider, `1L7-0`, `1LR-0`) vs **mid-group**
  (divider, `1KT-0`).
- Nav rows: **plain** (`1GU-0`) vs **carrying a padlock** (`1HA-0`).

No hover, focus, active, expanded, collapsed or disabled state is drawn on any
board in group 04. § 8 says what the spec implies for each.

---

## 5. Mobile rules

Every rule below maps onto an **existing** engine query. **No 390 breakpoint is
added** (tokens.md § 8 and § Open decisions).

| # | Rule at 390 | Measured | Engine query |
|---|---|---|---|
| M1 | **The right rail is removed entirely.** Reason on the board: every figure in it is already in the group headers a few lines below (note `60T-0` col 3). | `5VR-0` / `5YF-0` have no rail node | `max-width: 860px` — the tier where the engine already collapses rails |
| M2 | **The rail's caveat survives the rail**, moved under the subtitle: `26 claims blocked, 36 paths — a claim blocked through two modules counts under both.` Inter 12/17, `--color-faint` / `--color-dark-faint`. | `5X2-0` / `5YY-0` | `max-width: 860px` — it moves when the rail goes |
| M3 | **The page ground becomes `--color-card`**, not `--color-paper`: the list is full-bleed and there is no card to raise off a ground. | `5VR-0` ground `--color-card`; `5YF-0` ground `--color-dark-card` | `max-width: 520px` |
| M4 | **The findings card dissolves.** No border, no radius, no side inset; group headers keep 1px top and bottom rules and rows keep a 1px bottom rule. | `5XB-0` (no border/radius); `5XD-0` top+bottom `--color-border`; `5XH-0` bottom `#E8ECF1` | `max-width: 520px` |
| M5 | **The issue row restacks and leads with the count.** Line 1 is `dot + "N claims"`; line 2 is the finding title. **The claim slug is dropped**, because the group header already carries the module prefix and five wrapped mono lines would bury the titles (note `60T-0` col 3). | `5XI-0` (`gap: 8px`, dot `5XJ-0` `6 × 6`), count `5XK-0` Inter 13/16 w600, title `5XL-0` Inter 14/20 w500 `--color-ink`; row `5XH-0` `padding: 12px 16px`, `gap: 5px` | `max-width: 520px` (arrangement) |
| M6 | **Group headers stack** — module name over weight phrase, instead of ranged across a filler rule. The filler rule is dropped. | `5XD-0` `flex-direction: column`, `gap: 3px`, `padding: 10px 16px`; `5XE-0` Inter 11/14 w600 `0.08em`; `5XF-0` Inter 12/16 `--color-faint` | `max-width: 520px` |
| M7 | **The scope segmented control goes full width**, three equal `flex: 1 1 0` segments, and its radius grows from 8 to **9px** with a 7px segment radius. | `5WC-0` radius `9px`, `padding: 3px`, `gap: 3px`; `5WD-0` `flex: 1 1 0`, `height: 30px`, radius `7px` | `max-width: 520px` |
| M8 | **The filter chips run off the right edge and never wrap.** The strip has a left margin and **no right margin** (374 wide inside 390), so the last chip is deliberately clipped as the affordance that more exist. | `5WK-0` `margin-left: 16px`, no `margin-right`, `gap: 8px`; chips `5WL-0` … `5WX-0` | `max-width: 520px`. The no-wrap invariant is the same family as R-H.3 |
| M9 | **Chips grow to a 30px touch height** with symmetric `12px` inline padding, against the desktop 26/28px asymmetric `10px`/`12px`. | `5WL-0` / `5WP-0` / `5WT-0` / `5WX-0`: `height: 30px`, `padding-inline: 12px` | `(pointer: coarse)` for the height; `max-width: 520px` for the padding change |
| M10 | **The sort row becomes its own line**, `Sort` pushed left as `flex: 1 1 0` and the control hard right. | `5X4-0` `margin: 16px 16px 0`; `5X5-0` `flex: 1 1 0`; `5X6-0` `height: 28px` | `max-width: 520px` |
| M11 | **The title drops one step**, 28/34 → **26/32**, tracking unchanged at `-0.02em`. The subtitle drops 16/24 → **15/23**. | `5W9-0`, `5WA-0` (and `5YW-0`, `5YX-0` on dark) | `max-width: 520px` |
| M12 | **A 52px top app bar replaces the left rail**: hamburger `20 × 20`, project name Inter 14/18 w500 as `flex: 1 1 0`, search `19 × 19` and theme glyph `19 × 19` / `18 × 18` hard right. No bottom border. **Its three glyphs are hamburger, search and theme — there is no focus control at 390**, which is correct under R11.1: focus is one reversible state that hides the rails, and at 390 there are no rails to hide (M1, and the left rail is already this bar). A phone focus toggle would be a second, differently-scoped state and is forbidden. | `5VS-0` / `5YG-0`; `5VV-0` / `5YJ-0`; `5VT-0`, `5VW-0`, `5VZ-0` / `60Q-0` | `max-width: 860px` for existence; `(pointer: coarse)` for the 44px hit targets on its three glyphs |
| M13 | **Touch targets.** No control on these boards reaches 44px on its own (chips 30, sort 28, segments 30, app-bar glyphs 19–20). The engine's existing `(pointer: coarse)` block must therefore grow the hit area — padding or a pseudo-element — **without changing the drawn box**, exactly as it already does for `.comment-chip` and `.status-strip-head`. | measured above | `(pointer: coarse)` — `style.css:2210` |

**Bottom sheets.** This screen has **none**, and must not grow one. Per R-J.1
there is one sheet shell in the product with two known bodies — the facet index
and the comment thread — and neither is on the Issues screen. The scope control,
the sort control and the severity chips are all in-page controls on the 390
boards (`5WC-0`, `5X6-0`, `5WK-0`), not sheet triggers. If the sort control later
needs a picker on a phone, it is a **third body in the existing shell** (R-J.1),
not a second shell, and it inherits R-J.2's fixed chrome unchanged.

**Nothing on this screen changes shape at the tablet tier other than the rail
removal (M1).** R-H.0's "only three components change shape below the phone
breakpoint" names the blocked notice, the claim header and the coverage chip —
none of which appear here — so every mobile rule above is a *screen* rule, not a
component-shape rule, and belongs in this screen's own selectors.

---

## 6. Footer vocabulary

This screen has **no evidence footer, no freshness line and no elapsed-time
phrase.** R10.1 binds "02, 03, 13, 14 — every screen with the right rail"; board
04's right rail is a ranking panel, not the rail that carries the build stamp,
and the left rail footer on `1IO-0` carries navigation and the theme toggle only.
A lane must not add an `Updated N ago` line to this screen.

The meta vocabulary it *does* carry, quoted exactly and in board order:

**Breadcrumb** (`1JI-0`, `1JJ-0`): `Permission readiness` then `· Contract`.

**Page header** (`1JM-0`, `1JN-0`): `Issues` / `Every claim in this facet is
locked. None can advance until four other modules approve.`

**Scope segmented control** (`1JQ-0`, `1JS-0`, `1JU-0`), fixed order:
`This facet` · `Module` · `Project`.

**Severity chips** (`1JZ-0`/`1K0-0`, `1K3-0`/`1K4-0`, `1K7-0`/`1K8-0`,
`1KB-0`/`1KC-0`), fixed order, each a **word then a mono count**:
`Needs you 3` · `Blocker 26` · `Check 1` · `Later 1`. With Critical present the
order is `Critical` · `Needs you` · `Blocker` · `Check` · `Later` (`O2-0`,
matching `STATUS_SEVERITIES` in the engine).

**Sort row** (`1KF-0`, `1KH-0`): `Sort` then `Most claims blocked`.

**Group header weight phrase** (`1KQ-0`, `1LJ-0`) — the load-bearing wording of
this screen: `blocks 24 of 26 claims here` / `blocks 7 of 26 claims here`. The
shape is **`blocks <N> of <M> claims here`**, where `M` is the facet's blocked
count and the trailing `here` scopes the claim to the current facet. Do not
render it as a percentage and do not drop `here`.

**Row count** (`1KZ-0`, `1L6-0`, `1LD-0`, `1LQ-0`, `1LX-0`):
`24 claims` · `24 claims` · `6 claims` · `3 claims` · `4 claims`. Singular is
`1 claim`.

**Rail heading and caveat** (`1M1-0`, `5OS-0`): `WHERE THE BLOCKERS LIVE`, then
`26 claims blocked, 36 paths — a claim blocked through two modules counts under
both.` Note the em dash and the lower-case continuation. The forbidden earlier
wording, recorded so it is not reintroduced: **"who owns the fix"**.

**Rail rows** (`1M5-0`/`1M6-0` …): module name, then a bare mono integer —
`Capability support 24`, `Verification 7`, `Audience boundary 3`,
`Startup readiness 2`. No unit, no "claims", no percentage.

**Left rail footer** (`1IQ-0`, `1IW-0`, `1J3-0`, `1J8-0`): `Claims graph` ·
`Build order`, then the theme toggle `Light` · `Dark`.

**Left rail nav** (`1GM-0`, `1GP-0`, `1GQ-0`): `Search 828 claims`;
`MODULES` `26`; `TRACKS` `25`.

**Mobile caveat placement** (`5X2-0` / `5YY-0`): the rail caveat sentence, word
for word, under the subtitle.

---

## 7. Code address

Branch `feat/viewer-design-revamp` at `3ac8844`, worktree
`/Users/nitinkhanna/Desktop/dossierx/.claude/worktrees/viewer-design-revamp`.

**Every line number below is pinned to commit `3ac8844`, not to the working
tree.** While this spec was being written a parallel lane was editing
`internal/render/viewer/template/style.css`, `graph.css` and `render.go` in this
same worktree, and style.css line numbers shifted by roughly 88 lines mid-session.
Verify any address with `git show 3ac8844:<path>`, and treat the **quoted
selector or marker comment** as the real address — the number is a convenience
that a concurrent edit can invalidate.

**There is no Issues *screen* in the engine today.** `grep -n 'issue'` over
`internal/render/viewer/template/style.css` returns nothing, and `grep -rn
'issues' internal/render/*.go` returns nothing. What exists is the **status
strip**: a collapsible dock that renders the same findings, from the same data,
under the same five-severity vocabulary, with `Show issues` as its action label.
This screen is that strip promoted to a view. Every address below is a real
address that a lane will read, extend or replace.

### 7.1 `internal/render/viewer/template/style.css`

- **`style.css:2012`** — section marker, quoted:
  `/* ---- status strip (lock-ledger integrity + lint) -----------------------`.
  Its comment block runs `2012–2053` and contains **THE Z-INDEX LEDGER**
  (`style.css:2027–2053`), which assigns the strip `45`. A promoted Issues view
  is a *document view*, not an overlay, and must not take a z-index band; if it
  does, the ledger is where the number is recorded.
- **`style.css:2055`** — the second marker, quoted: `/* Project health belongs to
  the reading flow, between the active module's record header and its facet
  controls. It is deliberately quiet when closed: enough contrast to be noticed,
  but no floating dock or shadow competing with the document and its two
  navigation rails. */` (`2055–2058`). This is the intent board 04 inherits.
- **`style.css:2059–2208`** — the strip's own rules. Selector map:
  `.status-strip` `2059`; `.status-strip[hidden]` `2069`;
  `.status-strip--lint` `2073`; `.status-strip--integrity` `2078`;
  `.status-strip-head` `2084`; `.status-strip-summary` `2097`;
  `.status-strip-title` `2103`; `.status-strip-note` `2109`;
  `.status-strip-action` `2115`; `.status-strip-caret` `2123`;
  `.status-strip--open .status-strip-caret` `2130`; `.status-strip-body` `2136`;
  `.status-strip-body[hidden]` `2143`; **`.status-strip-filters` `2147`** (the
  severity chip strip — board 04's `1JW-0`); `.status-group + .status-group`
  `2154`; **`.status-group-head` `2158`** (board 04's module group header
  `1KN-0`); `.status-finding-list` `2166`; **`.status-finding` `2172`** (board
  04's issue row `1KT-0`); `.status-finding + .status-finding` `2182`;
  `.status-finding-rule, .status-finding-claim` `2186`;
  **`.status-finding-rule` `2191`** (the title `1KW-0`);
  **`.status-finding-claim` `2200`** (the slug `1KX-0`);
  **`.status-finding-msg` `2204`** (the `blocks N claims` phrase — board 04
  promotes it to the count slot `1KZ-0`).
- **`style.css:2210–2221`** — `@media (pointer: coarse)`, quoted marker
  `/* Coarse pointers (touch) need >=44px hit targets on every comment control. */`.
  `.status-strip-head` is already in its selector list (`style.css:2217`). This
  is where M9/M13's touch sizing belongs.
- **`style.css:2227` → `2275`** — `@media (max-width: 860px)`; the block's
  closing brace is `2275` and `2274` closes `.status-strip-caret`. The strip's
  mobile overrides are `.status-strip-head` `2256`, `.status-strip-summary` `2261`,
  `.status-strip-action` `2265`, `.status-strip-caret` `2270`.
- **`style.css:4187–4190`** — the dark re-point: `.status-strip { border-top-color:
  var(--border); color: var(--muted); }`. Any new Issues selector needs its dark
  twin here or in the `html[data-theme="dark"]` block, per tokens.md § 1.
- **`style.css:4754`** — `.status-strip { display: none !important; }` inside
  **the** `@media print` block. An Issues view that is a document view needs a
  deliberate print decision instead of inheriting this.
- **`style.css:807`** — readiness section marker, quoted: `/* Claim readiness is a
  reviewer-facing work queue. The policy engine remains authoritative; this
  surface groups its complete fact list by the first hop of each representative
  path, with an optional focused Mermaid trace. */` — `.claim-readiness` at
  `style.css:810`. This is the per-claim sibling of board 04's per-facet list and
  the source of the "grouped by first hop" idea the board generalises to
  "grouped by module".
- **`style.css:1160–1173`** — `@media (max-width: 860px)` for readiness;
  **`style.css:1175–1182`** — `@media (max-width: 520px)` for readiness. These
  two are the existing precedent for the § 5 mapping.
- Layout anchors board 04's geometry must be reconciled against:
  `.layout` `style.css:2595`; `.sidebar { width: 210px }` `2601–2602`, overridden
  in the System Record block to `var(--system-record-sidebar-width, 270px)` at
  `3076–3079` (board: **268px**, `1GD-0`); `.content-area` `2784` / `3369`;
  `.facet-toc { position: fixed; width: 236px }` `3668–3673` (board's right rail:
  **280px**, `1LZ-0`).
- `graph.css`: **no range.** The verification command is
  `git show 3ac8844:internal/render/viewer/template/graph.css | grep -n
  'status-strip\|status-group\|status-finding'`, which returns nothing:
  graph.css owns only `#dxg` / `.dxg-` selectors, so this screen has no range in
  it. A looser `grep -n 'status\|issue'` over the same file is **not** empty at
  `3ac8844` — it returns `graph.css:257`, a prose echo of the z-index ledger
  (`overlay 20/30, nav toggle 40, status strip 45, comments overlay 50,`), and
  `graph.css:874`, a comment about the graph's own status ring (`--ink, the same
  ink the status ring already uses, precisely so the`). Both are comment text,
  neither is a selector, and the ledger echo is a copy of the authoritative
  ledger at `style.css:2027–2053`. Use the scoped grep above; the loose one
  proves nothing either way.

### 7.2 Runtime JavaScript

All in `internal/render/viewer/template/viewer-runtime.js`:

- **`viewer-runtime.js:1083–1089`** — the element handles: `statusStrip`,
  `statusStripToggle`, `statusStripSummary`, `statusStripTitle`,
  `statusStripNote`, `statusStripAction`, `statusStripBody`.
- **`viewer-runtime.js:1100`** — `countLabel(n, word)`, the singular/plural
  helper behind `24 claims` / `1 claim` (§ 6).
- **`viewer-runtime.js:1124`** — `findingsForActiveFacet(findings, claimIDs)`;
  this is what makes `blocks 24 of 26 claims **here**` true.
- **`viewer-runtime.js:1139–1145`** — `STATUS_SEVERITIES`, the exact five of
  R08.1 in the exact board order:
  `critical` / `needs_you` / `blocker` / `check` / `later`, with labels
  `Critical`, `Needs you`, `Blocker`, `Check`, `Later`.
- **`viewer-runtime.js:1146`** — `var stripSeverityFilter = '';` — the pressed
  chip. One at a time, empty means none; § 8's pressed state must not change this
  to a multi-select without a decision.
- **`viewer-runtime.js:1148`** — `addStatusGroup`, the grouping key.
- **`viewer-runtime.js:1235`** — `countSeverity`; **`1239`** — `statusChipLine`;
  **`1245`** — `blockerHeadline` (the subtitle `1JN-0` comes from here);
  **`1272`** — `renderStatusGroup` (emits `status-finding--group`, the slug at
  `1278`, and the `blocks N claim(s)` phrase at `1281`);
  **`1290`** — `positionStatusStrip`; **`1323`** — `findingGroup(heading, rows)`
  (emits `.status-group` + `.status-group-head` — board 04's module header).
- **`viewer-runtime.js:1355–1366`** — the readiness verdict labels, including the
  four integrity ones R08.2 names: `approval_content_drift` (`1361`),
  **`approval_missing`** (`1362`), **`approval_released`** (`1363`),
  `approval_unknown` (`1364`).
- **`viewer-runtime.js:1376`** — `readinessHopLabel`, which produces
  `direct · 1 hop` (R-I.1's hop pill).
- **`viewer-runtime.js:1438–1530`** — the readiness DOM builder
  (`claim-readiness-*`), mounted at `1524–1526`.
- **`viewer-runtime.js:1638`** — **`renderStatusStrip(data)`**, the function this
  screen replaces. Inside it: `1643` reads `lastStatusData.ledger_findings`;
  `1670–1684` builds the severity chip strip (`pill ps` when pressed, `pill pv`
  when not, `aria-pressed`, click toggles `stripSeverityFilter`);
  **`1687–1695` groups `bySeverity` and renders one `findingGroup` per severity**
  — this is the line board 04 changes, because the board groups by **module**;
  `1700` writes the `N approval record issue(s) in this facet need attention`
  headline; `1713–1714` toggles `status-strip--integrity` / `--lint`;
  `1715` auto-expands only when `critical > 0 || needsYou > 0`.
- **`viewer-runtime.js:1721`** — `setStripExpanded`; **`1734`** —
  `refreshStatus()`, which polls `GET /api/status`.
- `system-record.js`, `graph-ui.js`, `build-order-ui.js`: **no address.**
  `grep -n 'readiness\|blocked\|status-strip' internal/render/viewer/template/system-record.js`
  is empty; the graph and build-order UIs do not touch this screen.

### 7.3 `shell.html`

- **`shell.html:265–271`** — the comment that governs the whole feature, quoted:
  `It exists ONLY against a live serve. The reachability probe reveals it,
  exactly as it reveals the composer; a static file:// viewer has no /api/status
  to poll, and a strip baked into the rendered HTML would be permanently stale —
  which for an integrity verdict is worse than absent, because a stale green
  strip is an assurance nobody checked.` **This is the hardest constraint on
  board 04** — see § 8 and § 9.
- **`shell.html:272–282`** — the markup: `<section id="statusStrip"
  class="status-strip" role="status" aria-live="polite" hidden>`, the toggle
  button with `aria-expanded` / `aria-controls`, `#statusStripTitle`,
  `#statusStripNote`, **`shell.html:278` `<span id="statusStripAction"
  class="status-strip-action">Show issues</span>`** — R09.6's entry point,
  already worded as the board wants — the caret at `279` and
  `#statusStripBody` at `281`.

### 7.4 Go

- **`internal/render/render.go:203–207`** — `HasReadinessMaps`, and
  `render.go:879–884` / `render.go:919` where it is computed from
  `in.cat.Readiness` (`DependencyConditions`, `Conditions`, `ReviewCauses`,
  `Causes`) and put on `shellData`. This is the proof that **the blocked set is
  known at build time**, which § 8 and § 9 turn on.
- **`internal/render/render.go:217`** — the comment that `shell.html` injects for
  a locked Build order or readiness map.
- **`internal/serve/handlers.go:231`** — `// GET /api/status — structured check
  result for the status strip`.
- **`internal/serve/handlers.go:816`** — `LedgerFindings []lock.Finding
  \`json:"ledger_findings"\`` — the integrity array R08.2's `APPROVAL RECORD`
  group must read.
- **`internal/serve/handlers.go:859`** — the comment forbidding a headline that
  contradicts a populated `ledger_findings`.
- **`internal/serve/server.go:426`** — `mux.HandleFunc("GET /api/status",
  s.handleStatus)`.
- **`internal/lock/policy.go:33`** — `LintFindings []lint.Finding
  \`json:"lint_findings,omitempty"\`` — the lint half of R08.2's pair.
- **`internal/check/check.go:600`** — `check.Status`, the build that drives
  `GET`/`HEAD /api/status` without Run's disk writes.
- **`internal/cliout/codes.go:138`** — `// themselves are in
  data.ledger_findings, each with its own stable rule name.`
- **`internal/render/components/`** — **no address for this screen.** The
  partials there (`banner.html`, `card.html`, `list.html`, `steps.html`,
  `table.html`, `tree.html`, `comments.html`, `mockup.html`) are claim layouts;
  the Issues view is not a claim. There is no `*_view.go` for it either —
  `build_order_view.go`, `graph_view.go`, `track_view.go`, `conformance_view.go`,
  `depended_by_view.go`, `implink_view.go` are the six views that exist, and a
  seventh would be the natural home.

### 7.5 viewer-tests that assert on these selectors today

- **`viewer-tests/component_fit_test.go:67`** —
  `TestStatusStripGroupsBlockersAndStaysCollapsed`. Asserts the strip becomes
  visible (`:74`), the title contains `blocked by unapproved dependencies`
  (`:75`), that `#statusStripBody .status-finding--group` is between 1 and 6
  (`:78`), and that the strip does **not** carry `status-strip--open` (`:81`),
  with the title/note echoed at `:86–:94`. Any regrouping by module changes
  `:78`.
- **`viewer-tests/component_fit_test.go:163`** — `TestPhone390SoftMountSmoke`,
  with the strip assertion at `:184–:185`. This is the 390 test the mobile rules
  in § 5 land in.
- **`viewer-tests/claim_collapse_test.go:212–241`** — asserts the exact string
  `1 issue in this facet needs attention` on `#statusStripTitle` (`:215`,
  `:241`), and that the strip is hidden in the two cases at `:224` and `:232`.
  Copy changes to the headline break this test by design.
- **`viewer-tests/theme_parity_test.go:154`** — `.status-strip` is in the
  parity surface list; `:217` is its probe markup; **`:828–:829`** are the
  golden rows `24 surfaces|.status-strip|background-color` and
  `24 surfaces|.status-strip|border-color`. A new Issues surface adds rows here
  and shifts the `24 surfaces` count.
- **`viewer-tests/theme_parity_test.go:1275`** — the screenshot suppressor
  hides `#statusStrip` and `.claim-readiness` for visual goldens.
- **`viewer-tests/claim_readiness_test.go:77–98`** and `:117–:186` — the
  readiness-card budgets (`.claim-readiness`, `-blocker`, `-route`, `-map`),
  including `TestReadinessBrowserScaleBudgets` at `:98`.

---

## 8. States not on the boards

The boards draw one fixture in one state. Everything below is a state the engine
can reach for this screen and no board depicts. Each entry says what the spec
implies, derived from the rules — no lane is left guessing.

1. **`Critical` severity present.** The chip exists in `STATUS_SEVERITIES`
   (`viewer-runtime.js:1140`) and on reference board `O2-0`, with count 0 on
   every fixture in group 04. *Spec:* render the **filled** pill from `O8-0` —
   ground `--color-blocked` / `--color-dark-blocked`, white dot, white label at
   weight 600, count white at 72 % alpha (`#FFFFFFB8`, `OB-0`). Filling is how
   "there is nothing above this" is said (R08.1); it is the **only** filled chip
   and no other chip may be filled to match it. Its rows take a
   `--color-blocked` dot and a `--color-blocked` count, like the Needs-you row
   `1LK-0`.

2. **A pressed severity chip.** `stripSeverityFilter` (`viewer-runtime.js:1146`)
   holds **one** severity or empty; clicking the pressed chip clears it
   (`:1680`). No board draws the pressed state. *Spec:* pressed is the **open**
   treatment from R-F.2 — the same pill with an `--color-accent` /
   `--color-dark-accent` border and its label stepped to weight 600 — because
   R-F.2's intent applies verbatim here ("at 12px on a phone, colour alone is too
   quiet to say which door is open"), and because the severity hues are already
   spent on meaning and cannot also mean "selected". `aria-pressed` is already
   emitted (`:1676`). Selection stays **single**; a multi-select is a different
   feature and is not authorised by these boards.

3. **Integrity findings — the `APPROVAL RECORD` group.** R08.2 binds 04 in all
   four variants and **no board shows it**. The data is there
   (`handlers.go:816`, labels at `viewer-runtime.js:1361–1364`). *Spec, derived
   from R08.2:* (a) the `APPROVAL RECORD` group **sorts first**, above every
   module group, regardless of how many claims it blocks — the module-weight sort
   of § 2 governs the module groups only; (b) its rows take a **red border**
   (`--color-blocked` / `--color-dark-blocked`) rather than the amber vocabulary,
   which is the one place on this screen a border carries meaning; (c) its group
   header label is literally `APPROVAL RECORD`, in the same Inter 11/14 w600
   `0.08em` as a module header (`1KO-0`), and its four verdicts are **missing,
   released, drifted, abandoned**. Severity and integrity are two scales, never
   one: a style lint must not be able to out-rank a missing approval.

4. **`review_pending` trigger variants.** The engine distinguishes three
   (`own_thread`, `own_flag`, and the two dependency triggers
   `direct_dependency_change` / `upstream_dependency_review` —
   `viewer-runtime.js:1357–1360`). *Spec:* they are **row copy, not new
   components**. Each keeps the § 4.8 row shape and the § 6 count phrasing; the
   verdict sentence from `viewer-runtime.js:1355–1366` is the title text
   (`1KW-0`'s slot) and the severity dot carries the severity. Do not invent a
   badge, a sub-label or a second colour for a trigger.

5. **Empty state — nothing blocked in this facet.** The engine already hides the
   strip in this case (`claim_collapse_test.go:224`, `:232`). *Spec:* the Issues
   screen is reached only from the blocked banner (R09.6), and the banner does
   not exist when nothing is blocked — so the empty state is a **direct
   navigation** (a bookmark, a back button, a stale deep link). It renders the
   header unchanged, the scope control unchanged, **no filter strip** (there are
   no severities to filter), and one line in place of the card: serif 16/24 in
   `--color-muted`, matching the subtitle `1JN-0`, stating that nothing in this
   facet is blocked. The right rail panel is **omitted**, not rendered empty —
   an empty ranking is a ranking of nothing.

6. **A severity filter that matches nothing.** Pressing `Later` when the only
   `Later` finding sits in a module already filtered out. *Spec:* keep the chip
   strip and the pressed chip on screen — the reader must be able to undo the
   filter — and replace the card body with the same one-line statement as (5),
   named to the filter. Never hide the control that caused the empty result.

7. **Scope = Module or Project.** `1JR-0` / `1JT-0` are drawn unselected only.
   *Spec:* selecting them changes the **denominator and nothing else**. The group
   header phrase is `blocks <N> of <M> claims here` where `here` means the current
   scope, so `M` becomes the module's or the project's blocked count. At project
   scope the group key stays the owning module — the screen's one question is
   still "which module do I chase" — and the rail's four rows become the top four
   of however many modules exist, which makes the caveat line (`5OS-0`) *more*
   load-bearing, not less.

8. **More than four modules in the rail.** The fixture has exactly four
   (`1M3-0`, `1M9-0`, `1MF-0`, `1ML-0`) and the rail column has
   `gap: 14px` for a second panel that is not there (`1LZ-0`). *Spec:* the rail
   is a **ranking**, so it is truncated, not scrolled: show the top rows that fit
   the panel and say how many were not shown in the same 11/16 `--color-faint`
   type as the caveat (`5OS-0`). The bars are proportions of the **largest
   count** (`round(92%, 1px)` for 24 — `1M8-0`), not of the total, so truncation
   never changes the drawn lengths.

9. **Long finding titles and long slugs.** The board's longest title wraps to two
   lines inside the 612px text column (`1L9-0`) and its longest slug —
   `capability-support.contract.a-result-is-about-one-audience-output-context`,
   73 characters — stays on one line at mono 11/14 (`1LB-0`). *Spec:* the title
   **wraps** (the row is `align-items: start` and the dot is offset
   `margin-top: 6px` precisely so a wrapped title still hangs correctly —
   `1KU-0`). The slug **truncates with an ellipsis** rather than wrapping: it is
   the demoted line under R09.8 ("the slug is for the agent"), and a slug that
   wraps to three mono lines buries the title above it, which is the exact
   failure the mobile note (`60T-0` col 3) names. The count slot is fixed at
   `104px` (`1KY-0`) and never shrinks.

10. **Cycles.** A dependency cycle has its own verdict string
    (`dependency_cycle`, `viewer-runtime.js:1355`). *Spec:* a cycle is a row like
    any other on this screen, with its verdict sentence as the title. **No cycle
    is drawn here.** R09.9 forbids an inline dependency map on the grounds that a
    tree drawn from a DAG invents blockers that do not exist; a cycle is the worst
    case of exactly that. The graph pane draws it, and `--color-graph-cycle` /
    `--color-dark-graph-cycle` belong to graph.css, not to this screen.

11. **`file://` — no `/api/status`.** `shell.html:265–271` states the strip
    exists only against a live serve. *Spec:* board 04 depicts a screen with no
    live affordance in it — the counts, the ranking and the group weights are all
    derivable at build time from `in.cat.Readiness` (`render.go:879–884`), which
    is how `.claim-readiness` already renders statically. A static Issues screen
    is therefore **coherent for readiness and lint** and **not coherent for
    integrity**: a baked `APPROVAL RECORD` verdict is the "stale green strip"
    the shell comment forbids. The implied split, recorded as a decision in § 9:
    readiness/lint groups render statically; the `APPROVAL RECORD` group renders
    only against a live serve, and its absence on `file://` is stated, not
    silently omitted.

12. **Hover, focus-visible, active, disabled.** No board draws any of them.
    *Spec:* hover on an issue row uses the engine's existing mode-invariant
    `--hover-bg` (`rgba(125,137,154,.08)`, tokens.md § Engine-only tokens) and
    **introduces no new token**. Focus-visible follows whatever the sheet already
    does for `.status-strip-head`; it is not this screen's to invent. There is no
    disabled state on this screen — a chip with a zero count is still pressable
    and returns the state in (6), which is more useful than a dead control.

13. **The sort control's other values.** Only `Most claims blocked` is drawn
    (`1KH-0`). *Spec:* whatever the other values are, the control's box does not
    change — `7px` radius, `1px --color-border`, `5px`/`10px` padding (`1KG-0`) —
    and the value label stays Inter 12/16 weight 500 `--color-ink`. Changing the
    sort **re-orders rows within groups and re-orders groups**; it must not
    regroup, because grouping by module is the screen's thesis (§ 2), not a
    setting.

14. **Focus mode reaching this screen — R11.1's on-state.** Every board in
    group 04 draws focus **off**: both rails present on `1GC-0` / `5PG-0`, and
    no focus control anywhere in the header (`1JE-0` / `5SH-0`) or in the 390
    app bar (`5VS-0` / `5YG-0`). R11.1 nevertheless binds the header on 04.
    *Spec, derived from the rule:* if focus is brought here it is **one control
    and one state** — pressed, it hides the left nav rail (`1GD-0`, `268px`) and
    the right ranking rail (`1LZ-0`, `280px`) **together**, and the main column
    (`1JD-0`, `flex: 1 1 0`, `min-width: 0`) takes the freed width; released, the
    board as drawn returns. Two toggles producing four layouts are forbidden, as
    is a rail-only or a header-only variant. R11.2 and R11.3 (`1E3-0`, board 03)
    carry no `Binds` line and do **not** bind 04, but their reasoning is the
    only guidance on where freed width goes, so it is adopted electively here:
    the findings card takes the width, the row shape of § 4.8 does not change,
    and the `104px` count slot (`1KY-0`) stays fixed.
    The control itself is **not** specified by these boards; it is the 03
    control reused, and this screen adds nothing to it. At 390 there is no focus
    state at all (§ 5, M12).

---

## 9. Open decisions

Decided here rather than asked, per the freeze protocol.

1. **Severity chip tints ship as the tokens, not as the board hexes.** The boards
   print `#9E3B361F` (≈12 %) and `#9A6A1621` (≈13 %) light, `#EC8A8326` and
   `#DDA94E26` (≈15 %) dark. `--color-blocked-bg` is 10 %, `--color-draft-bg`
   11 %, and their dark twins 13 %. No note anywhere claims the severity strip
   needs a heavier tint than a status chip, and R08.1's own prose says "~12 %"
   and "~13 %" — an approximation, not a specification. Ship
   `--color-blocked-bg` / `--color-draft-bg` (engine `--warn-bg` /
   `--status-draft-bg`). A 2-point alpha difference nobody argued for is not
   design intent.

2. **The three unbound greys become derived values, not new tokens.** The
   allowlist is closed (tokens.md § 1). Therefore:
   - Segmented-control trough `#E3E7ED` light / `#080B10` dark (`1JO-0`,
     `1J2-0`, `5WC-0` / `5SR-0`, `5YZ-0`) → a `color-mix` of `--border` into
     `--paper`, following the precedent tokens.md sets for the navigation accent
     tint. The dark trough being **darker than `--color-dark-paper`** is the
     intent to preserve: the selected segment is `--card-bg` and must read as
     raised out of a well.
   - Group-header ground `#F6F8FA` light / `#1B212B` dark (`1KN-0`, `5XD-0` /
     `5TO-0`, `5ZU-0`) → **`--code-bg`**. The dark board's literal is already
     `--color-dark-code-bg`'s exact value, which is the board telling us what it
     meant; the light board is 2 units off `#F3F5F8` and that difference is not
     intent.
   - Row divider `#E8ECF1` light / `#212934` dark (`1KT-0`, `5XH-0` / `5ZX-0`) →
     a `color-mix` of `--border` into `--card-bg`. A within-card divider being
     lighter than a card edge is a real and deliberate distinction on both
     boards; it is a derivation, not a token.

3. **`12px` card radius and `7px` control radius stay as per-component
   literals.** tokens.md § Open decisions forbids widening the radius scale to
   swallow off-scale values, and the engine has one themeable radius token
   (`--radius: 6px`). Record both in this file and write them as literals.

4. **The findings card's zero bottom radii are load-bearing.** `1KM-0` /
   `5TN-0` carry `border-top-*-radius: 12px` with both bottom radii `0px`. That
   is not an oversight — it is how a list that continues past the fold is said.
   Implement it; do not "fix" it to a closed card.

5. **Static vs served, split by finding kind.** Per § 8 item 11: readiness and
   lint groups render at build time from `in.cat.Readiness`
   (`render.go:879–884`); the `APPROVAL RECORD` group renders only against a live
   serve, and on a `file://` viewer its absence is stated in one line rather than
   silently omitted. This is the only reading of board 04 that does not violate
   `shell.html:265–271`.

6. **Grouping by module replaces grouping by severity.** `renderStatusStrip`
   groups `bySeverity` today (`viewer-runtime.js:1687–1695`). Board 04 groups by
   owning module and demotes severity to the filter strip. The board and its
   notes state the reason; the code has none recorded. **The board wins.**
   `viewer-tests/component_fit_test.go:78` asserts a `status-finding--group`
   count and will need updating with the change, not around it.

7. **`Later` stays in the chip strip and out of the list.** R09.8 cuts `Later`
   from the default view, "behind the filter chips only". The board shows the
   chip with count 1 and no `Later` row. That is the correct reading and is the
   spec: the chip is how a reader opts in.

8. **Sidebar width: the board's 268px is the spec for this screen, and the
   mismatch with the engine's 270px default is left to group 02.** The engine
   has `var(--system-record-sidebar-width, 270px)` (`style.css:3077`), user
   resizable between 220 and 420. 268 is inside that band; this screen does not
   re-declare it.

9. **No bottom sheet on this screen** (§ 5). If a phone sort picker is ever
   needed it is a third **body** in the one shell (R-J.1), never a second shell.

10. **No freshness line on this screen** (§ 6). R10.1 binds 02/03/13/14. Board 04
    omits it and that omission is correct, not a gap.

11. **R11.1 binds this screen's header, and the boards satisfy it by absence.**
    `reference-rules.md:276` names "the header on 02, 04, 13, 14". No board in
    group 04 draws a focus control, and an earlier draft of this spec read that
    as the rule not applying. Decided the other way: the rule applies, the
    boards depict its *off* state, and the binding is a **prohibition on the
    header** — no per-rail show/hide pair may be added to `1JE-0` / `5SH-0`, and
    no phone-only focus toggle may be added to `5VS-0` / `5YG-0`. The on-state
    is specified in § 8 item 14 from the rule rather than measured, because
    nothing measurable exists to read. A lane that wants focus on Issues reuses
    the 03 control unchanged; it does not design one here.

### Paper defects

Found on the group 04 boards. Recorded, not fixed — Paper is read-only for this
lane.

1. **R08.2 is unrepresented on all four boards.** The rule binds "04, all four
   variants" and requires integrity findings to sort first, take a red border and
   group under `APPROVAL RECORD` with four verdicts. `1GC-0`, `5PG-0`, `5VR-0`
   and `5YF-0` show only module groups with amber and red **dots** — no border,
   no `APPROVAL RECORD` header, no verdict vocabulary. Approval of these boards
   is not approval of dropping R08.2. Specified from the rule in § 8 item 3.

2. **`#080B10` on the dark boards is an unbound literal with no light twin
   relationship.** `5SR-0` (desktop dark scope trough) and `5YZ-0` (mobile dark
   scope trough). It is darker than `--color-dark-paper` `#0D1117` and no token
   in the file resolves to it. Its light counterpart `#E3E7ED` is likewise
   unbound. Resolution in § 9 item 2.

3. **`#1B212B` is written as a literal on a dark board where the token exists.**
   `5TO-0` and `5ZU-0` set the group-header ground to the exact value of
   `--color-dark-code-bg`. A literal that happens to equal a token is still a
   literal, and tokens.md § Disagreements item 12 is explicit that a board hex is
   evidence, not permission.

4. **`#212934` (dark row divider — desktop `5TT-0`, `5U0-0`, `5UJ-0`; mobile
   `5ZX-0`, `60F-0`) and `#E8ECF1` (light row divider, `1KT-0`, `5XH-0`) are
   unbound.** `#E8ECF1` is already on tokens.md's
   list of raw hexes the components board leaks; group 04 repeats it.

5. **The severity tints on all four boards disagree with the tint tokens** —
   ≈12 %/≈13 % light and ≈15 %/≈15 % dark against 10 %/11 % and 13 %/13 %. See
   § 9 item 1.

6. **The mobile boards spell font families as the literal
   `"Inter", system-ui, sans-serif` and `"Source Serif 4", system-ui,
   sans-serif`** (`5W9-0`, `5WA-0`, `5XL-0`, `5YW-0`, and every mobile text
   node), while the desktop boards spell them as `var(--font-sans)` /
   `var(--font-serif)` (`1JM-0`, `1JN-0`, `1KW-0`). Same file, same screen, two
   conventions — tokens.md § Disagreements item 12 records this as a
   file-wide pattern and group 04 is an instance of it.

7. **The rail caveat nodes use negative top margin to fake a gap** — light
   `5OS-0` inside the `gap: 12px` column `1M0-0`, and its dark twin `5V0-0`
   inside `5UY-0`, both `margin-top: -6px` with `margin-bottom: 2px`. The drawn result is correct, but it encodes "this line belongs to
   the heading above it" as an override rather than as structure. Implement the
   intent — caveat grouped with the heading — not the arithmetic.

8. **The desktop boards' `WHERE THE BLOCKERS LIVE` heading is
   `--color-faint`, while the list's module headers at the same size, weight and
   tracking are `--color-muted`** (`1M1-0` vs `1KO-0`). Two eyebrow colours for
   the same typographic role, on the same screen, with no note explaining the
   split. Recorded; the board is followed as drawn because the rail is a
   restatement and reading quieter than the primary list is defensible — but it
   is an inconsistency the component board does not sanction.

9. **The `Later` chip's dot is `--color-border-strong`** (`1KA-0` `#C2CAD5`,
   dark `5TB-0` `--color-dark-border-strong` `#38424F`). This matches reference
   board `O2-0` (`OP-0`) exactly, so it is not a group 04 defect — but it means a
   *border* token is doing duty as a *severity* token in the one place R08.1
   calls "neutral". Flagged because a future token audit will see it as a misuse
   and should find this note first.
