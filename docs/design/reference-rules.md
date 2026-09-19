# Reference rules

Rules extracted from the five reference boards and from the components board's
mobile sections. Every rule below is stated as a **rule**, not as a picture: it
says what must be true of any screen it binds, so that a screen can be checked
against it without opening Paper.

Evidence ids are Paper node ids in file `01M2MTR6F73KHFR95ZWE8A7YAC`. Text was
read with `get_jsx` / `get_node_info` / `find_nodes`, not from screenshots.

Read this with `tokens.md`. Where a rule names a colour, the token is the one in
`tokens.md`; a raw hex printed on a board is evidence of intent, not a value to
ship.

---

## 08 · Severity & integrity — the vocabulary the Issues screen uses

Board `O2-0`. PNG: `ref-08.png`.

### R08.1 — Five severities, two hues, and no third hue

**Statement.** The severity vocabulary is exactly five terms — Critical, Needs
you, Blocker, Check, Later — and they are painted in two hues plus neutral.
Critical and Needs you are `--color-blocked` at two weights; Blocker and Check
are `--color-draft`; Later is neutral. The strip introduces no colour the rest of
the viewer does not already mean something by.

**Intent.** A reviewer has already learned red = blocked and amber = not settled
from the claim body. Spending a third hue on the severity strip would make them
learn a second vocabulary for the same facts.

**Weights, measured.** Critical is a *filled* pill: ground `--color-blocked`,
label and dot white, count white at 72 % alpha. Needs you is a *tinted* pill:
ground `--color-blocked` at ~12 % alpha, dot / label / count all
`--color-blocked`. Blocker is tinted `--color-draft` at ~13 %. Check is a
`--color-card` pill with a `--color-border` outline, a `--color-draft` dot, a
`--color-muted` label at weight 500 and a `--color-faint` count. Later is the
same outlined pill with a `--color-border-strong` dot. Only Critical is filled;
filling is how "there is nothing above this" is said.

**Binds.** 04 (Issues screen, all four variants), and any filter-chip strip.

**Evidence.** `O2-0`; eyebrow `O4-0`, note `O5-0`; chips `OA-0` (Critical), `OE-0` (Needs you), `OI-0` (Blocker), `OM-0` (Check), `OQ-0` (Later).

### R08.2 — A lint and a ledger finding must never read alike

**Statement.** A lint is advice about how a claim is written. A ledger finding is
a refusal about whether it was ever approved. Therefore integrity findings
(a) **sort first** on the Issues screen, (b) take a **red border** rather than
amber, and (c) are **grouped under `APPROVAL RECORD`** with their own four
verdicts: *missing, released, drifted, abandoned*.

**Intent.** Severity answers "how urgent"; integrity answers "is this record
trustworthy at all". Ranking them on one scale would let a style lint out-rank a
missing approval.

**Binds.** 04 (all four variants).

**Evidence.** `O2-0`; eyebrow `QH-0`, note `QI-0`.

---

## 09 · Placement map — where each component appears

Board `WI-0`. PNG: `ref-09.png`.

### R09.1 — Four expansions of one strip, never four panels

**Statement.** Blockers/readiness, relationships, sources and checks are **four
expansions of the same metadata strip** under a claim's prose. They are never
stacked panels. The strip is one line at rest.

**Intent.** A reader who wants none of the four sees one line; a reader who wants
one sees exactly one. Four panels would force the cost of all four on a reader
who wanted none.

**Binds.** 05, 06, 06a, 07, 07a (all variants), and the claim component
everywhere it appears.

**Evidence.** `WI-0`, header `WL-0`, note `WM-0`; strip label `30Z-0` ("one line
at rest").

### R09.2 — Exactly one expansion open at a time

**Statement.** At most one of the four may be open. Opening one closes whichever
was open.

**Intent.** The reader's question is always singular. Two open expansions mean
the reader has to re-find the prose.

**Binds.** 05 in particular (the board is named for this rule), and every claim.

**Evidence.** `Y6-0` — "EXACTLY ONE OF THE FOUR OPEN AT A TIME".

### R09.3 — Auto-open is reserved for readiness, and only when blocked

**Statement.** No expansion opens by default. Readiness is the **only** one that
may auto-open, and only when the claim is blocked. Relationships explicitly never
opens by default.

**Intent.** Auto-opening is the interface overriding the reader's intent; the one
case that earns it is the case where the claim's own status is a question the
reader did not ask but must answer.

**Binds.** 05, 06, 06a, 07, 07a.

**Evidence.** Readiness `ZD-0` — "Inline, from the strip. The only one that may
auto-open, and only when the claim is blocked." Relationships `Z7-0` — "…three
directions in one section. Never opens by default."

### R09.4 — Relationships is one section with three directions

**Statement.** Governed by, depends on, depended on by are three directions
inside **one** expansion, in that fixed order — not three sections and not three
chips.

**Binds.** 05, 06, 07, 13.

**Evidence.** `Z7-0`; components board section C note `3C3-0`.

### R09.5 — Sources is split out of relationships

**Statement.** Sources is its own expansion, not part of relationships. A
citation is not a graph edge, and the two are read for different reasons.

**Binds.** 05, 06a, 07.

**Evidence.** `ZJ-0`.

### R09.6 — Issues is a screen, and the banner is the only way in

**Statement.** Issues is its own screen, not an expansion. The blocked banner is
the **only** entry point to it, and it appears exactly where the issues are.

**Intent.** An entry point placed anywhere else is an entry point the reader has
to go looking for; the banner is already in front of them at the moment the
question arises.

**Binds.** 04, and the blocked banner on 06 and on every mobile board (component
H1).

**Evidence.** `ZP-0`.

### R09.7 — The facet dot is the only project-level health signal in the reading view

**Statement.** The status dot in `ON THIS FACET` is the only project-level health
signal the reading view carries. Modules with blocked claims are **not** otherwise
marked.

**Intent.** One signal that is always in the same place beats five signals the
reader has to assemble.

**Binds.** 02, 03, and the left TOC on every desktop board.

**Evidence.** `YZ-0`; the `ON THIS FACET` group `YH-0` / `YI-0`, with a `Blocked`
marker at `30T-0`.

### R09.8 — Eight demotions, and nothing is deleted from the data

**Statement.** The engine holds more than the reviewer needs. Each item below
moves **behind one provenance disclosure**, where an agent can still reach it.
Nothing is deleted from the data.

| Field | Verdict | Why (verbatim) |
|---|---|---|
| Adapter · Shape | Demoted | "Which reader read it does not change whether the claim holds." |
| Target URI | Demoted | "95 characters of path, wrapping to two lines, naming a file the reviewer will not open." |
| Expected + Observed | **Merged** | "Two sets became one marked set. The reader sees the difference, not the inputs." |
| Raw diagnostics JSON | Demoted | "An engine envelope. Belongs to the agent operating the tool, not the human reviewing it." |
| "shown via this dependency" | **Cut** | "Engine vocabulary describing its own grouping choice." |
| Claim id under each finding | **Swapped** for the title | "The reviewer recognises the title. The slug is for the agent." |
| Severity "Later" | **Cut** from the default view | "Not actionable now, by its own definition. Behind the filter chips only." |
| Inline dependency map | **Cut** | See R09.9 |

**Binds.** 04, 06a, and every checks expansion.

**Evidence.** `ZS-0`, `ZT-0`, rows `100-0`, `104-0`, `108-0`, `10C-0`, `10G-0`,
`10K-0`, `10O-0`, `32U-0`.

### R09.9 — No inline dependency map; the breadcrumb and the graph pane carry it

**Statement.** There is no inline dependency map. A readiness chain is a DAG, not
a tree: blockers converge on shared ancestors. Drawn as a tree it either
duplicates those nodes — **inventing blockers that do not exist** — or needs
crossings no inline panel can carry. The breadcrumb on each blocker row already
states that row's path; the graph pane draws the real shape.

**Intent.** This is a correctness rule wearing a layout rule's clothes. A
duplicated node in a tree is a false statement about the project.

**Binds.** 05, 06, 06a, 13, and component I1.

**Evidence.** `32V-0` / `32W-0` / `32X-0`.

---

## 10 · Freshness — elapsed, not stamped

Board `1F7-0`. PNG: `ref-10.png`.

### R10.1 — Elapsed time, never a timestamp

**Statement.** The right-rail footer states elapsed time since the build, not the
build time. "Updated 23 hours ago", not "built 15:32 on 16 September".

**Intent, verbatim.** "A reviewer does not need to know the viewer was built at
15:32 on 16 September. They need to know whether what they are reading is still
true. Elapsed time answers that without arithmetic; a timestamp makes them do the
subtraction."

**Binds.** 02, 03, 13, 14 — every screen with the right rail.

**Evidence.** `1F7-0`; eyebrow `1F9-0`, heading `1FA-0`, note `1FB-0`.

### R10.2 — Four bands, and only the last takes colour

**Statement.** Four freshness bands, one presentation rule each.

| Band | Range | Wording | Presentation |
|---|---|---|---|
| Just built | `< 1 h` | "Updated just now" below one minute, else "Updated N minutes ago" | Identical to Fresh. Neutral. |
| Fresh | `1 – 24 h` | "Updated N hours ago" | `--color-muted` label at weight 500, `--color-faint` clock icon. No colour. |
| Ageing | `1 – 7 d` | "Updated N days ago" | Identical to Fresh. Still neutral. |
| Stale | `> 7 d` | "Updated N days ago" | `--color-draft` label at weight **600**, `--color-draft` icon. |

**Amendment note (2026-09-19).** The sub-hour band was added after the viewer was
observed reporting "Updated 1 hour ago" on a page nineteen seconds old. The
formatter clamped every age under 90 minutes up to one hour
(`Math.max(1, Math.round(hours))`) because the board specified no band below
`< 24 h`, so a reader opening a freshly generated viewer was told the claims were
an hour staler than they were. The band is still **one unit** (R10.3): minutes
never pair with seconds, and "just now" replaces "0 minutes ago".

Stale takes the **same amber as a draft, because both mean "not settled"**. This
is the only freshness state that takes colour.

**Evidence.** Fresh `1FK-0` / `1FL-0` / `1FM-0`; Ageing `1FT-0` / `1FU-0` / `1FV-0`; Stale `1G2-0` / `1G3-0` / `1G4-0`. Band labels (`1FL-0`, `1FU-0`, `1G3-0`) are mono 10/12.

### R10.3 — One unit, never two

**Statement.** Hours resolve to hours, days to days. Never "23 h 14 m", never
"3 days 20 hours". The exact build time stays on hover as a `title` attribute,
for the rare reader who needs it.

**Intent.** "Precision nobody acts on" is the board's own phrase for the second
unit.

**Evidence.** `1F7-0` RULES panel `1G7-0`, rule `1G8-0`.

### R10.4 — Computed in the browser, not baked into the HTML

**Statement.** The elapsed phrase is computed in the browser from the build stamp.
It is not written into the generated HTML.

**Intent, verbatim.** "A relative phrase baked at generation time says '23 hours
ago' forever, which is worse than a date."

**Evidence.** `1F7-0` RULES panel `1G7-0`, rule `1G9-0`.

### R10.5 — Behind `dossierx serve`, the line reads "Live"

**Statement.** When the viewer is served rather than built, the footer reads
**Live** instead of an elapsed phrase.

**Intent, verbatim.** "Elapsed time would be meaningless and falsely reassuring."

**Evidence.** `1F7-0` RULES panel `1G7-0`, rule `1GA-0`.

---

## 11 · Focus mode — one control, not two toggles

Board `1E3-0`. PNG: `ref-11.png`.

### R11.1 — One reversible state, not two independent toggles

**Statement.** Focus replaces per-rail show/hide. There is one control and one
state, not two toggles producing four layouts.

**Intent, verbatim.** "A reader who wants fewer distractions wants them all gone
at once, and a single reversible state is easier to reason about than two
independent ones that produce four layouts."

**Binds.** 03 (all four variants), and the header on 02, 04, 13, 14.

**Evidence.** `1E3-0`; eyebrow `1E5-0`, heading `1E6-0`, note `1E7-0`.

### R11.2 — The measure widens in focus, but stays bounded

**Statement.** Outside focus, claim prose holds the reading measure (the card's
694px content box inside the 760px canvas). **In focus the prose widens to a
bounded 900px** — it takes a share of the width the rails gave back, but never
relaxes out to the card's full 1074px content box.

**Measure:** 760px canvas / 694px prose in the default view; 900px prose in
focus. See `tokens.md` § Open decisions for the frozen 760 canvas measure, which
this rule does not change.

**AMENDED 2026-09-19 (maintainer decision).** This rule previously froze the
prose across the toggle — "the paragraph the reader is on is pixel-identical
before and after" — and R11.3 sent the whole freed width to the evidence. On a
real corpus that leaves a visible empty column beside every paragraph, in a mode
whose entire purpose is more room, and the maintainer chose to spend part of it
on the prose. The consequence the original rule protected is real and accepted:
prose now re-typesets by roughly 200px when focus is toggled.

The bound is the substance of the rule, not a detail. The card's content box in
focus is 1074px, which at `--text-body` 17px over `--leading-body` 28px runs
about 145 characters a line against a 65–75 optimum; 900px is about 115. Long
doctrine claims are exactly the content that suffers first, so a future change
that removes the cap and lets prose fill the card is a regression against this
rule, not a continuation of it.

**Evidence.** `1E3-0` rule 1 (marker `1EB-0`), title `1ED-0`, body `1EE-0` —
the board still draws the original frozen-measure wording and is superseded here
until it is redrawn.

### R11.3 — The freed width goes to the evidence, not to the prose

**Statement.** The page grows to **1140px** and the card with it (758 → 1138).
Blockers, breadcrumb paths, dependency maps, fenced code and conformance sets
spread into the freed width. Prose takes a bounded share of it too — see R11.2 as
amended; the original wording of this rule gave prose none of it.

**Intent, verbatim.** "These are the things that were cramped at 760 — the prose
never was."

**Evidence.** `1E3-0` rule 2 (marker `1EG-0`), title `1EI-0`, body `1EJ-0`.

### R11.4 — The control is its own reminder; no banner, no hint strip

**Statement.** There is no banner and no hint strip announcing focus mode. The
Focus button stays in the header in its **active** state, which is the affordance
and the exit in one place. Exits: press `F`, click the button again, or push the
pointer to either edge to **peek** a rail without leaving the mode.

**Evidence.** `1E3-0` rule 3 (marker `1EL-0`), title `1EN-0`, body `1EO-0`.

### R11.5 — Focus survives a reload, and lives in localStorage

**Statement.** Focus is per-reader state, like the theme choice. It belongs in
`localStorage`, **not** in the generated HTML, so a rebuilt viewer does not
silently reopen the rails.

**Evidence.** `1E3-0` rule 4 (marker `1EQ-0`), title `1ES-0`, body `1ET-0`.

### R11.6 — DECIDED: focus does not collapse the claim list

**Statement.** Focus hides the rails and gives their width to the evidence —
nothing more. It must not reduce the page to a single claim.

**Intent, verbatim.** "Reducing the page to a single claim would break scroll
position, the facet TOC's meaning and every deep link into the facet, and it is a
different feature. Focus stays reversible and lossless: the same claims, in the
same order, with more room."

**Evidence.** `1E3-0` DECIDED panel: label `1EW-0` in `--color-locked`, body
`1EX-0`, on a `#A9CBBB` border — the board's own "this is settled" treatment.

---

## 12 · Code evidence — show the reviewer the code

Board `2ML-0`. PNG: `ref-12.png`. **This board is labelled `PROPOSAL — NOT
AVAILABLE TODAY` (node `2MN-0`). Nothing in it is in scope for the current
implementation.** It is recorded so that a lane agent recognises it as a proposal
and does not implement it from a PNG.

### R12.1 — A snippet lives behind "How this was checked", never inline

**Statement.** If code evidence ships, it belongs behind the *How this was
checked* disclosure. The check sentence above already answers the reader's
question; the snippet is the proof under it.

**Intent, verbatim.** "A check currently asks the reviewer to trust the adapter.
A snippet lets them see for themselves."

**Binds (if shipped).** 06a.

**Evidence.** `2ML-0`; label `2MN-0`, heading `2MO-0`, note `2MP-0`.

### R12.2 — The verdict, the source path and the tinted lines are the whole component

**Statement.** The panel carries, in order: a title row with a verdict badge
(`Matched`, dot + label in `--color-locked`); a `SOURCE` row whose directory
prefix is faint mono and whose **filename is accent mono at weight 500**; the
numbered lines with the compared members **tinted** (measured `#2C6B5214` — the
lock green at ~8 % alpha); and a `NOTE` row in serif.

**Evidence.** `2ML-0`: title `2TQ-0`, verdict `2TT-0`, `SOURCE` label `2TV-0`,
directory prefix `2TX-0`, filename `2TY-0`, code lines `2U3-0` / `2U7-0` / `2UB-0`
/ `2UF-0`, line numbers `2U2-0` / `2U6-0` / `2UA-0` / `2UE-0` (mono 12/22 in
`#A9B3C1`; code is mono 13/22 in `--color-ink`).

### R12.3 — Tinting marks the compared members, and set order carries no meaning

**Statement.** The tinted lines are the members compared, not a diff. Where the
claim lists them in one order and the source declares them in another, that is
not a mismatch: **the expectation is a set, so order carries no meaning.**

**Evidence.** `2ML-0` NOTE row: label `2UH-0`, body `2UI-0`.

### R12.4 — The three standing caveats

**Statement.** Any implementation must carry all three.

- **WORKS** — "The adapter already opens the file to extract symbols. Capturing
  the declaration costs one extra field in `observations/*.json`, written in the
  same pass under the same snapshot hash — so the snippet is exactly as fresh as
  the value beside it. No drift risk."
- **LIMIT** — "Only symbol-shaped adapters have a snippet. `test-coverage/v1`
  points at a suite, not a span of lines — the step grid stays its evidence.
  Roughly half of this module's 34 observations would carry code."
- **WATCH** — "A viewer is shareable. Source lines inside it travel wherever the
  file goes, so a project whose code is not public needs a way to turn this off.
  **It should be opt-in per project, not on by default.**"

The three labels are themselves colour-coded: WORKS `--color-locked`, LIMIT
`--color-draft`, WATCH `--color-blocked` — the same three status colours doing a
fourth job, which R08.1 permits because no new hue is introduced.

**Evidence.** `2ML-0`: WORKS `2NY-0` / `2NZ-0`, LIMIT `2O2-0` / `2O3-0`, WATCH
`2O6-0` / `2O7-0`.

---

## 00 · Components — sections H, I, J

Board `377-0`. PNG: `components-00.png`.

### R00.0 — This board is the source of truth, and the rule is procedural

**Statement.** Every screen draws its claim chrome from this board. Paper has no
live instances, so the rule cannot be technical: **change the component here
first, then re-place it on each screen listed beside it. A screen is an instance,
not a place to design.**

**Why it exists, verbatim.** "…the blockers panel had drifted into two designs —
hairline rows on 03, 05 and 06, and bordered cards on 07 — with nothing in the
file saying which was right. The same had happened to the status chip, which lost
its padlock on two claims, and to the checks panel, which 06a had abridged into a
fourth layout."

For a lane agent the equivalent is: **two screens that show the same component
must produce the same markup.** If your screen's PNG disagrees with the component
board, the component board wins and you flag the screen.

**Evidence.** `3GM-0`, `3GN-0`, `3GO-0`, `3GP-0`.

---

## The footer vocabulary

### R-F.1 — Four chips in a fixed order, then the comment count hard right

**Statement.** The metadata strip is **four chips in a fixed order — readiness,
relationships, sources, checks — then the comment count hard right**. Each chip
is **a noun and a count**. None of them is a score. Only one chip may be expanded
at a time (restates R09.2 at the component level).

**Binds.** Every claim on every board, desktop and mobile.

**Evidence.** Section A, `377-0` → `37D-0`, note `37J-0`.

### R-F.2 — Chip states: closed, open, blocked

**Statement.** Three variants and no more.

| Variant | Ground | Border | Label | Chevron |
|---|---|---|---|---|
| Closed | `--color-card` | `--color-border` | `--color-muted`, weight 500 | down, `--color-faint` |
| Open | `--color-card` | **`--color-accent`** | `--color-accent`, weight **600** | up, `--color-accent` |
| Blocked | `--color-blocked-bg` | none | `--color-blocked` weight 600 + count weight 400, preceded by a 7px `--color-blocked` dot | down, `--color-blocked` |

All three are 30px tall with `--radius-pill` and 11px inline padding.

**Intent, verbatim.** Open takes "an accent border as well as an accent label —
at 12px on a phone, colour alone is too quiet to say which door is open."

**Rule.** The blocked chip **always leads row one**.

**Evidence.** Section I3, `377-0` → `7N2-0` (rows `7NA-0`), note `7N8-0`.

### R-F.3 — The comment count is one pill with one variant, not two components

**Statement.** The comment count sits **hard right in the footer strip at every
width**. On desktop it is a **passive count** and its bubble is `--color-faint`.
On mobile it is **the control that opens the sheet**, so its bubble takes
`--color-accent-bg` with an `--color-accent` icon and label. Same pill, one
variant.

**Evidence.** Section J2, `B0C-0`; specimen is a 30px `--radius-pill`
pill, 14px speech-bubble icon, 13/16 accent label at weight 600.

### R-F.4 — Mobile footer: two rows, and a pill never splits

**Statement.** Below the phone breakpoint the footer strip becomes two rows:
**row one** is the status chip left and the comment count right; **row two** is
the detail chips as pills. Pills must read as tappable and **each one fits whole
— nothing spans the width or splits across lines.**

**Maps onto.** `@media (max-width: 520px)`, written to win over the existing
`max-width: 640px` and `max-width: 560px` rules on `.claim-footer__counts`.

**Evidence.** Section H3, `377-0` → `57F-0`, the `H3 · FOOTER STRIP` note.

---

## Mobile forms (section H) — three components change shape, and only three

### R-H.0 — Only three components change shape below the phone breakpoint

**Statement, verbatim.** "Three components change shape below 390px. Everything
else on a mobile board is the desktop component at a narrower width. These are
not different components — same names, same counts, same wording — only a
different arrangement, and **a screen must use the form that matches its width**."

**Evidence.** `57F-0`, note `57L-0`.

### R-H.1 — Blocked notice: inset card, not a full-bleed band

**Statement.** On mobile the blocked notice is an **inset card** (16px side
margins, 10px radius, 1px tinted-red border) and **not** a full-bleed band. The
action gets its **own divided row** — a 1px top hairline and a 38px row — so the
tap target is the card's width.

**Content.** Warning triangle in `--color-blocked` (15px, 2px stroke), body at
13/19 in `--color-ink`, action label "Show issues" at 13/16 weight 600 in
`--color-blocked` with a trailing chevron.

**Evidence.** Section H1, `57F-0`.

### R-H.2 — Claim header: status chip above the title

**Statement.** On mobile the status chip sits **above** the title, not beside it,
so the title runs the full measure. Order is **status, then claim, then id**.

**Measured.** Chip: `--color-locked-bg`, `--radius-pill`, padlock 11px, label
`LOCKED` 11/14 weight 600 tracking `+0.05em` in `--color-locked`. Title: 18/24
weight 600 tracking `-0.01em` in `--color-ink`. Id: mono 11/15 in
`--color-faint`. Gaps 9px / 6px.

**Evidence.** Section H2, `57F-0`.

### R-H.3 — Coverage chip: the strip never wraps

**Statement.** Below 400px the coverage step chip is **24 × 28 with a 3px gap**,
against 32 × 29 and 5px on desktop. **The rule is that the strip never wraps** —
the reader is looking for the one gap in a run, and a second row destroys that
reading. Thirteen steps fit a 358px gutter at this size with 10px to spare.

**Maps onto.** `@media (max-width: 520px)`. 400 is where the fit was measured;
520 is where the engine expresses it, and the no-wrap invariant must hold across
the whole tier.

**Measured.** Blocked step: `--color-blocked` 1.5px border, ~12 % blocked tint,
5px radius, 11/14 weight 600. Ordinary step: `--color-card` on a 1px
`--color-border`, 11/14 weight 400 in `--color-muted`.

**Evidence.** Section I4, `7N2-0`.

---

## Expansion forms (section I) — a ranged row becomes a stacked row

### R-I.0 — Same content, same vocabulary, same order; only the arrangement moves

**Statement, verbatim.** "Four forms for the rows inside an open expansion. Same
content as desktop, same vocabulary, same order — what changes is that a row
ranged across columns becomes a row stacked in lines. A screen must use the form
that matches its width; **these are not alternative components**."

**Evidence.** `7N2-0`, note `7N8-0`.

### R-I.1 — Blocker row: the dependency path stacks, it is never dropped

**Statement.** Title, then the hop pill, then the dependency path **stacked** —
source slug on its own line, target slug below it behind a leading chevron.
**The path is what "blocked by" means, so it is never dropped; it stacks instead
of ranging across the row.**

**Measured.** Title 14/20 weight 600 `--color-ink`. Hop pill: `--color-paper`
ground, `--radius-pill`, 11/14 weight 500 `--color-muted`, text of the form
`direct · 1 hop`. Source slug: `--color-paper` ground, `--radius-sm`, mono 11/14
`--color-muted`. Target slug: ~10 % blocked tint, `--radius-sm`, mono 11/14
`--color-blocked`. Chevron 11px `--color-faint`.

**Evidence.** Section I1, `7N2-0`.

### R-I.2 — Relationship row: four columns become two lines, badge right-ranged

**Statement.** The desktop row's four columns become two lines. **Line one:**
status dot and title. **Line two:** `module · facet` left, lifecycle word right.
The badge is **right-ranged**, so `LOCKED` and `DRAFT` keep a lane **without a
fixed-width slot**.

**Intent.** Right-ranging is what makes the badge lane survive two different word
lengths on a 390px screen, where a fixed slot would waste the width the title
needs.

**Measured.** Dot 7px in the lifecycle colour. Title 14/20 weight 500 in
`--color-accent`. Meta line 12/16 `--color-faint`. Badge 11/14 weight 600
tracking `+0.06em` in the lifecycle colour.

**Evidence.** Section I2, `7N2-0`.

### R-I.3 — The chip is the component; the footer's two rows are composition

**Statement.** The detail chip is the component. The footer's two-row arrangement
is **composition, not part of the chip.** A change to the footer layout is not a
change to the chip, and vice versa.

**Evidence.** Section I3, `7N2-0`.

---

## Bottom-sheet policy (section J)

### R-J.1 — There is one bottom sheet in this product, not several

**Statement, verbatim.** "There is one bottom sheet in this product, not several.
The shell is fixed… What varies is the body. The facet index and the comment
thread are two bodies in one shell, and any future sheet is a third."

**Rule.** A new sheet is a new **body** for the existing shell. A screen that
introduces a second shell is wrong.

**Evidence.** `AYG-0`, note `AYM-0`.

### R-J.2 — The fixed shell

**Statement.** The shell is fixed and consists of exactly:

- a **scrim** (engine `--scrim`: `rgba(0,0,0,.22)` light, `rgba(0,0,0,.42)` dark);
- **16px top corners** (`border-top-left-radius` / `border-top-right-radius`);
- a **36 × 4 grabber** in `--color-border-strong` with `--radius-pill`, under
  **8px of top padding**, centred;
- a **header row** with a **1px `--color-border` hairline** beneath it;
- **on dark, an additional 1px top hairline** on the sheet itself.

Sheet ground is `--color-card`; width is the viewport (390 on the specimen);
`overflow: clip`.

**Evidence.** `AYG-0`, note `AYM-0`, specimen `AYO-0` ("J1 · Bottom sheet").

### R-J.3 — The sheet sizes to content between 240 and 660, and 660 is a ceiling

**Statement.** `min-height: 240px`, `max-height: 660px`, `height: fit-content`,
scrolling internally beyond that.

**Intent, verbatim.** "660 is a **ceiling, not a height** — a sheet holding one
empty-state line is short, and a long thread hits the ceiling and clips at the
composer, which is **the scroll edge, not a crop**."

**Evidence.** `AYG-0` first note column; measured on `AYO-0`.

### R-J.4 — The header's second line and count are slots, not forks

**Statement.** The header carries an **optional second line** and an **optional
mono count** as *slots*. A body that has neither renders the header without them;
it does not get a different header.

**Measured.** Title 16/22 weight 600 tracking `-0.01em` `--color-ink`. Second
line 12/18 `--color-muted`. Count mono 12/16 `--color-faint`, right of the text
block. Close glyph 20px, 2px stroke, `--color-muted`, hard right. Header padding
14px top / 12px bottom / 16px inline, 10px gap.

**Evidence.** `AYG-0` second note column; header inside `AYO-0`.

### R-J.5 — The composer is pinned and never shrinks

**Statement.** Where a body has a composer, it is **pinned** to the bottom of the
sheet and **never shrinks**. The scrolling region above it is what gives way.

**Measured.** Composer strip: `--color-paper` ground, 1px `--color-border` top
rule, 14px top / 18px bottom / 16px inline padding. Field: `--color-card` on 1px
`--color-border-strong`, **8px radius** (`--radius-md`), 11px block / 13px inline
padding, placeholder 14/20 `--color-faint`. Submit: `--color-accent` ground,
**6px radius**, 9px block / 16px inline padding, label 13/16 weight 600 in
`#FFFFFF`. A 11/16 `--color-faint` caption sits left of it: "Saved to the served
viewer, not to this file."

**Evidence.** `AYG-0`; composer strip inside `AYO-0`.

### R-J.6 — Comment states are states of this shell, not separate sheets

**Statement.** **Empty, resolved and read-only are states of the one shell.**
They are not separate sheets and must not be designed as such. Known bodies
today: the **facet index** and the **comment thread**.

**Binds.** 14, 14a (all variants), and the 02 mobile nav sheets.

**Evidence.** `AYG-0` third note column.

### R-J.7 — Thread structure inside the comment body

**Statement.** A first message, **replies beneath it behind a left rule**, then
the actions. Measured: reply block indented 12px behind a **2px `--color-border`
left rule**. Authorship label is mono 11/14 weight 600 tracking `+0.04em` —
`HUMAN` in `--color-accent`, `AGENT` in `--color-graph-facet-1`. Elapsed time is
mono 11/14 `--color-faint`; an edited message adds a mono 11/14 italic `(edited)`
in `--color-faint`. Body is 14/22 `--color-ink`. Actions are a **Resolve** pill
(1px `--color-border-strong`, `--radius-pill`, 12px tick in `--color-locked`,
label 12/16 weight 600 `--color-muted`) and a bare **Reply** label 12/16 weight
600 in `--color-accent`. Resolved threads collapse to a single row — chevron plus
`N resolved` at 12/16 weight 600 in `--color-muted`.

Note that elapsed-time phrasing here follows R10.1 and R10.3: "3 days ago",
"22 hours ago", one unit.

**Evidence.** Components section G (`4DC-0`), note `4DI-0`; specimen inside
`AYO-0`.

---

## Open decisions

- **Board 12 is not implemented.** It is labelled `PROPOSAL — NOT AVAILABLE
  TODAY` on its own face. Its rules are recorded so a lane agent recognises it,
  and R12.4's WATCH caveat (opt-in per project, never on by default) is carried
  forward as a constraint on any future attempt.

- **Where a reference board and a screen board disagree, the reference board
  wins, and the screen is flagged.** This is R00.0 generalised: the reference
  boards state rules with reasons attached; a screen is an instance.

- **"Below 390px" on the component board means `max-width: 520px` in the
  engine.** No 390 breakpoint is added. See `tokens.md` § 8 for the full mapping
  table. Where a component board gives a measurement at a specific width (I4's
  "358px gutter, 10px to spare"), that measurement is the *tightest* case the
  520px rule must survive, not a second breakpoint.

- **R09.1's "one line at rest" and R-F.1's "four chips then the count" are the
  same rule at two altitudes.** Implement once, in the claim footer component;
  do not let a screen restate it.
