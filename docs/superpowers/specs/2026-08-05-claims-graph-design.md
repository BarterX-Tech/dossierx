# DossierX claims graph — design

Date: 2026-08-05
Branch: `v0.5.0` (cut from `main`)
Version: minor bump to **v0.5.0**
Plan: [`docs/superpowers/plans/2026-08-05-claims-graph-plan.md`](../plans/2026-08-05-claims-graph-plan.md)

---

## 0. The job, in one paragraph

The viewer renders claims as cards you read one at a time. Edges exist —
`rests_on`, `governed_by`, `mirrors` — but only as footer text on individual
cards. Nothing shows the shape of the whole corpus, so the failures that matter
most stay invisible: the claim nothing rests on, the module nothing links into,
the phase that was never written, the loop that alternates edge types. This
feature adds **one global graph pane** to the viewer that makes absence legible,
plus one new lint for the loop shape the graph made obvious, plus two test-hygiene
fixes the same investigation surfaced. No new CLI noun, no schema field, no
network.

The design principle everything else falls out of: **Go emits facts; the browser
computes verdicts.** Any gap list precomputed in Go would be wrong the moment a
reader scopes the view to one module.

**The graph audits claims, not code.** Source files are not nodes, and whether a
claim has an implementation link is not a fact this payload carries. That
question belongs to `dossierx check`'s impl-link stage, which already reports it
with its own vocabulary and its own exit code. A second surface for it would be
a second place to keep correct.

---

## 1. Architecture

```
catalog.Catalog          internal/graph.Build          render.Render                 browser
(claims, ByFacet,   ->   pure function over        ->  inlines the JSON as a    ->   graph-core.js  (pure logic)
 ByModule)               catalog + config only;        <script type=             +   graph-ui.js    (DOM + canvas)
                         no clock, no I/O,             "application/json">       +   graph.css      (pane chrome)
                         no HTML                       block in shell.html
                              │
                              └──────────────────────> GET /api/graph  (serve only)
                                                       same payload, stamped now
```

The same payload serves both consumers of the static path — `dossierx check`'s
`viewer/index.html` and `dossierx serve`'s live render — so a file handed to a
reviewer over email has a fully working graph with no server behind it. Under
`serve` there is additionally a read-only JSON endpoint the pane's refresh
button calls; §6 covers it.

### 1.1 Why the pane mounts outside `div.layout`

`dossierx serve` pushes updates by SSE. The browser re-fetches
`/api/fragment`, which returns exactly two subtrees — `<nav id="nav">` and
`<main class="content-area">` — and the client replaces both by `outerHTML`
assignment. [VERIFIED: `internal/serve/handlers.go` `extractElement` targets
only those two open tags; `shell.html` `applyFragment()` assigns
`oldContent.outerHTML` / `oldNav.outerHTML`.]

Anything that must survive a live-reload therefore sits **outside** `.layout`,
as a sibling of `#commentsPanel` and `#statusStrip`. The graph pane holds zoom,
pan, drag positions, filter selection and an expanded-group set — all of it
client-only state that a fragment swap would silently destroy. So the pane root
is a sibling of `#statusStrip`, and this is the *already-proven* pattern, not a
new one: `shell.html`'s own comment says of the comments rail, "the panel node
itself lives OUTSIDE the swapped subtree so it survived".

**The payload does not refresh on a fragment swap.** The
`<script type="application/json">` block is not inside either swapped anchor and
is never re-delivered. A claim edited during a live session updates the reading
view but *not* the graph. This is accepted, and §6 is what makes it honest: the
pane states the payload's generation time, and offers a button that fetches a
fresh one on demand.

### 1.2 The trigger

A single button in `<nav id="nav">`, above the module tabs, carrying
`data-dxg-open` and **not** the class `sec-tab` — the existing delegated click
handler matches `e.target.closest('.sec-tab')` and would otherwise also switch
modules [VERIFIED: `shell.html:1395` `document.addEventListener('click', …)`,
`:1407` `var secTab = e.target.closest('.sec-tab');`].

The button lives inside the swapped subtree, so it is destroyed and re-created
on every fragment swap. `graph-ui.js` therefore binds **one delegated listener
on `document`**, never a listener on the button — the same pattern the existing
tab navigation uses and the one `TestDelegatedTabNavigationSwitchesModules`
already protects.

---

## 2. The payload contract

`internal/graph` exports exactly two functions plus the types they move.

```go
// SchemaVersion is bumped when the payload shape changes in a way a
// browser built against an older shape cannot read.
const SchemaVersion = 1

func Build(cat *catalog.Catalog, cfg *config.Config) Payload
func Encode(p Payload) ([]byte, error)
```

`Build` takes **two arguments and no more**. It is total: nil `cat` and nil
`cfg` both produce a valid empty-ish payload rather than an error. It performs
no I/O, reads no clock, and returns no error, so it cannot fail a render.

There is deliberately no third argument. An earlier draft of this design passed
`render.buildImplinkLookup(cfg)` in so the payload could carry a
`has_code_link` flag. That is gone — see §0 and §12.

### 2.1 Shape

```json
{
  "schema": 1,
  "generated_at": "2026-08-05T14:02:11Z",
  "nodes": [
    {
      "id": "viewer.contract.render-is-pure",
      "title": "Render Is Pure",
      "module": "viewer",
      "facet": "contract",
      "status": "locked",
      "kind": "fact",
      "build_role": "api",
      "emphasis": false,
      "review_pending": false,
      "open_comments": 0,
      "in_degree": 3,
      "out_degree": 2
    }
  ],
  "edges": [
    { "from": "viewer.contract.render-is-pure",
      "to": "engine.schema.catalog-shape",
      "type": "rests_on" }
  ],
  "groups": {
    "modules": ["engine", "viewer", "cli", "lock", "telemetry"],
    "facets":  ["contract", "schema", "behavior", "verification", "overview"]
  },
  "dropped": { "unresolved_edges": 0 }
}
```

### 2.2 Field derivation, exactly

| Field | Source | Note |
|---|---|---|
| `schema` | `SchemaVersion` | |
| `generated_at` | **the caller**, never `Build` | RFC3339 UTC. See §2.5. |
| `id` | `model.Claim.ID` | |
| `title` | `components.ClaimLabel(c.ID)` | Derived from the id slug, **not** author prose — `model.Claim` has no `Title` field [VERIFIED: `internal/model/claim.go:248-306`; `ClaimLabel` at `components.go:889`]. |
| `module` / `facet` | `c.Module` / `c.Facet`, verbatim | May be empty or not in `cfg`; the browser buckets those under a catch-all label and the reserved `--dxg-facet-other` slot. |
| `status` | `c.Status` | Closed enum: `draft` \| `locked` [VERIFIED: `internal/model/claim.go:15-21`]. |
| `kind` | `c.EffectiveKind()` | Not the raw field — the reserved overview facet implies `orientation-note` without the author repeating it. |
| `build_role` | `c.BuildRole` | May be empty; empty is meaningful (see `missing_build_phase`). |
| `emphasis` | `c.Emphasis` | |
| `review_pending` | `c.ReviewPending` | Engine-managed; only meaningful on a locked claim. |
| `open_comments` | `len(c.OpenThreadIDs())` | |
| `in_degree` / `out_degree` | counted over all three edge types, **project-wide** | A fact. The browser recomputes scope-relative degree for radius and the connectivity rules. |

There is no `has_code_link` field and no field derived from `internal/implink`.
`internal/graph` does not import that package.

### 2.3 Edges

One edge per declared relation, in the direction the claim declares it:

- `rests_on`: one edge per entry of `c.RestsOn`.
- `mirrors`: one edge per entry of `c.Mirrors`. Directional in storage even
  though reciprocity is a lint, not a model invariant [VERIFIED:
  `internal/lint/mirror_reciprocal.go`].
- `governed_by`: at most one edge, emitted only when
  `c.Governed.Type != "" && c.Governed.Type != "none"` — the same guard
  `dangling.go` uses [VERIFIED: `internal/lint/dangling.go:23-58`].

**Edges whose target is not a known claim id are dropped** and counted in
`dropped.unresolved_edges`. `check` refuses to render a corpus with a dangling
edge — it returns before the catalog and render stages on any error-severity
lint finding [VERIFIED: `internal/check/check.go:225-227` returns
`fmt.Errorf("lint: %d error-level finding(s)")` *above* the `catalog.Build`
call] — but **`serve` never lints**; it loads, builds, renders [VERIFIED:
`internal/serve/server.go:395-406`]. So a live session genuinely can carry
dangling edges, and the pane shows a one-line notice rather than silently
drawing a smaller graph than the data describes.

### 2.4 Determinism, and the one field that moves

`Build` is a pure function of its inputs. Two calls on the same catalog produce
identical `Payload` values, because the payload lands in three tracked,
committed fixture viewers.

- `nodes` sorted by `id`.
- `edges` sorted by `(from, type, to)`.
- `groups.modules` = `cfg.Modules` in **config order** (matching the sidebar's
  reading order), then any other distinct module value found in claims, sorted.
  `groups.facets` likewise from `cfg.Facets`.
- No map iteration reaching the output. **No `time.Now()` anywhere in the
  package.**

### 2.5 `generated_at` is stamped by the caller, not by `Build`

The pane must tell a reviewer how fresh what they are looking at is, which needs
a timestamp in the payload. Reading the clock inside `Build` would destroy the
purity property above and put a moving byte inside the unit under test.

So `Build` leaves `GeneratedAt` empty and each call site stamps it:

| Call site | Value stamped | Why |
|---|---|---|
| `internal/render/graph_view.go` | `Render`'s existing `generatedAt` | The payload's time is then *the same instant* as line 1's `<!-- generated by dossierx check at … -->` header and the sidebar footer's `Generated …` string, all three from the one `generatedAt := time.Now().UTC()` at `render.go:369`. |
| `internal/serve` `handleGraph` | `time.Now().UTC()` at request time | The endpoint's entire purpose is freshness. |

Consequence for release hygiene: the RFC3339 timestamp now appears **twice** in
a rendered document (line 1 and the payload's `generated_at`) and the
human-readable `2006-01-02 15:04 UTC` form once (the sidebar footer). That is
still exactly **two formats**, which is what the fixture-staleness test of §13.3
normalizes, and `docs/RELEASING.md`'s "the only expected changes are the
generation timestamp" sentence is updated to name the third occurrence.

### 2.6 Encoding — the one place this feature can be unsafe

`html/template` performs **no escaping at all** inside
`<script type="application/json">` for a `template.JS` value. A payload value
containing `</script>` writes straight out of the tag and everything after it is
parsed as HTML. [VERIFIED, Phase 0's P2 standalone probe: `template.JS` →
`<script type="application/json" id="p">{"a":"</script><img src=x>"}</script>`
— breaks out, no escaping applied.]

The guard is `encoding/json`'s **default** HTML escaping, which turns `<`, `>`
and `&` into `<`, `>` and `&` before the bytes ever reach the
template. Therefore:

- `Encode` uses `json.Marshal`. It never uses `json.Encoder`.
- **`SetEscapeHTML(false)` is a forbidden call in this feature and in this
  repository.** It is named as forbidden in the package doc comment, in the
  implementation-lane brief, and proven absent by a repo-wide grep that is part
  of the payload lane's own proving command — not a later hardening pass.
- The payload is never hand-assembled from string concatenation.
- The `GET /api/graph` handler writes `Encode`'s bytes directly rather than
  going through `internal/serve`'s `writeJSON` helper, so there is exactly **one**
  encoder for this payload and exactly one escaping rule to keep correct.

This is reachable, not theoretical: `module`, `facet` and the id slug that
`title` derives from are author-authored YAML scalars, and under `serve` no lint
has run to constrain them. A test whose claim label literally contains
`</script>` is the first test written in the payload lane.

---

## 3. The representative-node rule

Scope, granularity and drill-down collapse to one rule:

> **Every claim resolves to a representative node — itself if its group is
> expanded, its group node otherwise. Edges map through the representative,
> aggregate by `(from, to, type)`, and drop self-loops.**

| Control | What it sets |
|---|---|
| Scope (`all` \| `module:<m>` \| `facet:<f>`) | Which claims are in play at all. |
| Granularity (`claims` \| `module` \| `facet`) | The default expanded/collapsed state of every group. |
| Drill-down (double-click a group) | A per-group override of that default. |

Edge endpoints outside the current scope resolve to a **ghost node**: hollow,
unlabeled, not counted in any gap rule. Scoping must never hide that a claim
reaches outward.

### 3.1 Cycle detection runs at claim level, never on aggregated edges

This is the one place the representative rule must **not** be applied first, and
getting it wrong produces a graph that lies.

When a module collapses to one node, *every* intra-module edge becomes a
self-loop on that node. If SCC ran over the aggregated edge set with self-loops
treated as cycles, every module with any internal edge would be ringed red.

So: **Tarjan SCC runs over the scope-filtered, claim-level edge set**, over the
directed types only (`rests_on` + `governed_by`; `mirrors` excluded, it is
reciprocal by design). Cycle membership is a property of *claims*. A group node
is ringed red iff it contains at least one claim in a cycle. Red edges are the
claim-level edges inside an SCC, drawn through their representatives.

A component of size 1 is a cycle only if it carries a literal self-edge
(`A rests_on A`). That case is reported under its own name, `self_edge`, and is
never merged into the cycle list — the engine already has a dedicated
error-severity `self-edge` lint for it, distinct from `cycle` [VERIFIED:
`internal/lint/self_edge.go`].

---

## 4. Node and edge encoding (frozen)

The overlay swaps the node fill **wholesale** — grey for non-matching, semantic
colour for matching — rather than cramming four signals into one node.

| Channel | Carries | Why this channel |
|---|---|---|
| Fill colour | facet, by slot (§4.2) | Identity, stable across sessions. Overridden wholesale while an overlay is active. |
| Radius | total edge degree **within the current scope** | Makes an isolated claim pre-attentive: literally the smallest thing on screen. |
| Ring | locked (solid) / draft (dashed) | Independent of fill and size, so it survives every overlay. |
| Halo | `review_pending`, open comments | Reserved for engine-managed states that demand human action. Never more than one halo at a time. |
| Wedge marker | node governs ≥1 other claim | §4.3. |
| Edge style | see §4.3 | Each type independently toggleable, so a cluttered graph reads one relation at a time. |
| Ghost node | edge endpoint outside current scope | Hollow, unlabeled. Keeps the reach visible without dragging the rest of the project in. |

Six overlays, plus "none": isolated & weakly linked · dependency cycles ·
**governance** · review pending · open comment threads · draft vs locked.

### 4.1 Control layout (frozen from the prototype)

One control bar, five groups, in this order: **Scope** (select) · **Granularity**
(select) · **Highlight overlay** (select) · **Edge types** (three toggle buttons)
· **View** (labels toggle, re-run layout).

Above the control bar sits a one-line pane header carrying the payload's
generation time and — under `serve` only — the refresh button (§6). Below the
control bar, the canvas with a hint line (`drag node · drag background to pan ·
scroll to zoom · double-click a group to expand it`) and a right-hand rail
carrying the gaps list above a detail footer. A legend strip runs along the
bottom.

Below the existing 860px breakpoint the rail stacks under the canvas. The
breakpoint is reused, not invented — it is the file's single responsive
breakpoint for the sidebar/layout system.

### 4.2 The facet palette is 20 slots, assigned by index, and colour is not the only channel

The prototype hardcodes five facet names. DossierX has none: `cfg.Facets` is an
arbitrary project-authored list, and `testdata/fixture-portability` exists
specifically to prove the engine has zero hardcoded facet or module names
[VERIFIED: `internal/config/config.go:122` `Facets []string`].

`graph.css` therefore defines slots `--dxg-facet-1` … `--dxg-facet-20`, plus
`--dxg-facet-other` for a claim whose facet is empty or absent from
`groups.facets`. **A facet is assigned slot `(index in payload.groups.facets)
mod 20`, never by name.** Assignment is deterministic and stable between
sessions for a given project.

Slots 1–5 carry the prototype's frozen values:

| Slot | Light | Dark |
|---|---|---|
| 1 | `#4257C4` | `#7C8CE8` |
| 2 | `#12897F` | `#3FB3A6` |
| 3 | `#B65A34` | `#DE8A62` |
| 4 | `#7050A8` | `#A98CD8` |
| 5 | `#67717E` | `#8E9AA8` |

Slots 6–20 continue the ramp by rotating hue and alternating one lighter and one
darker step per rotation, so adjacent slots never differ only in lightness.

**And this is where honesty is required: roughly twelve is the practical ceiling
on distinguishable categorical colour.** A project with 20 facets will have
slots a reader genuinely cannot tell apart at 6px radius. Widening the ramp to
20 does not solve that; it only stops the palette from *repeating* inside the
range most projects live in. So facet identity is carried by three channels, not
one:

1. **Colour**, by slot, for gestalt clustering.
2. **The legend**, which lists every facet by its real project name against its
   assigned swatch, and dims non-members on hover — the disambiguator that keeps
   working at 20 facets.
3. **The detail panel**, which names the selected node's facet in text.

A reader never has to resolve a colour to answer "which facet is this?". That is
the property that makes a 20-slot ramp acceptable rather than a lie.

### 4.3 `governed_by` gets four channels, not a dash pattern

`governed_by` is the edge a reader most often needs to isolate, and it is the
one an edge style alone cannot make findable — telling a dashed line from a
dotted line at a glance across a 400-node canvas means tracing lines, which is
exactly the work this pane exists to remove. So it gets four independent
channels plus an overlay:

| Channel | `rests_on` | `mirrors` | `governed_by` |
|---|---|---|---|
| Stroke colour | muted ink | muted ink | **`--dxg-governed`, a reserved hue outside the 20-slot facet ramp** |
| Routing | straight | straight | **quadratic curve** |
| Arrowhead | single chevron | none | **double chevron** |
| Node marker | — | — | **wedge on the governing node** |

The reserved hue is the load-bearing one: because it is outside the facet ramp,
a governance edge can never be mistaken for a facet's colour, in either theme,
at any facet count. The wedge marker sits on the node that **governs** — the
target of a `governed_by` edge — so a doctrine claim is findable without
following any edge at all.

**The governance overlay** dims every node and edge except governors, the claims
they govern, and the edges between them. That is the channel that answers "what
does this doctrine actually reach?" in one click instead of one trace.

---

## 5. Gap rules

Every rule is computed **against the current scope, in the browser**. A claim
isolated within `module:viewer` may be well-connected project-wide, and a panel
that ignored that would lie. Facts and heuristics are rendered in visually
separate blocks; heuristics are labelled as guesses and block nothing.

| Rule id | Group | Definition | Kind |
|---|---|---|---|
| `cycle` | structural | Tarjan SCC (size ≥ 2) over `rests_on` + `governed_by` at claim level within scope. Members ringed red, member edges drawn red, in every overlay. | fact |
| `self_edge` | structural | A claim that is its own `rests_on` / `mirrors` / `governed_by` target. Reported separately from `cycle`. | fact |
| `isolated` | connectivity | Zero edges of any *enabled* type within scope. | fact |
| `weakly_linked` | connectivity | Exactly one edge within scope. | fact |
| `review_pending` | attention | Engine-set `review_pending`. | fact |
| `open_threads` | attention | `open_comments > 0`. | fact |
| `sink_group` | group | A group with outbound edges to other groups and zero inbound. | fact |
| `orphan_group` | group | A group with no edges to or from any other group. | fact |
| `missing_build_phase` | hint | A module with ≥ 1 locked claim and zero claims in some `build_role` phase. Verification is the usual absentee. | **heuristic** |
| `density_outlier` | hint | A facet whose claim count in one module is far below its median across the others. | **heuristic** |

Each rule emits `{rule, node_ids, kind}` with a stable `rule` id, so the panel's
rendering and the browser tests both key off the id, not off display text.

There is no `locked_ungrounded` rule. Whether a locked claim is grounded in real
code is `dossierx check`'s impl-link verdict, computed by
`internal/implink.Status`'s `UnlinkedIDs`, and it stays there.

### 5.1 One deliberate alignment with the engine

**`isolated` is defined over enabled edge types, which by default includes
`governed_by` — and lint's `orphan` rule does not.** `orphanLint` builds its
incoming/outgoing sets from `Mirrors` + `RestsOn` only, deliberately excluding
`governed_by` because `type: none` with a reason is the normal expected state
[VERIFIED: `internal/lint/orphan.go:27-49` and its doc comment]. So the graph's
default isolated set is a *subset* of lint's orphan set. The parity test
therefore pins the **unscoped, `rests_on` + `mirrors`-only** isolated set equal
to lint's `orphan` findings — the only configuration in which the two are
defined over the same edges. The panel carries a line pointing at `check` for
the project-wide verdict.

---

## 6. Freshness: the timestamp and the refresh button

§1.1 established that a fragment swap does not re-deliver the payload. Two
additions make that honest rather than a trap.

### 6.1 The pane states the payload's generation time

The pane header renders `generated_at` as a relative phrase with the absolute
value on hover (`payload generated 4 minutes ago`). A reader who has been in a
live session for an hour can see at a glance that the shape on screen is an
hour old. This is the mitigation §1.1 owes the reader, and it works identically
in the static `file://` viewer, where the answer is simply "when `check` ran".

### 6.2 `GET /api/graph`

A new read-only endpoint in `internal/serve`:

```
GET /api/graph  →  200 application/json; charset=utf-8
                   Cache-Control: no-store
                   body: graph.Encode(p) bytes, p stamped time.Now().UTC()
```

Registered in `Server.routes()` alongside the other reads. Its handler is the
graph analogue of `handleStatus`: load claims fresh, build the catalog the same
way `renderViewer` does — `catalog.Build(disarmUngatedMockups(claims, cfg), cfg)`
— call `graph.Build`, stamp, `graph.Encode`, write the bytes. It never writes to
disk, which is what makes it safe on a CSRF-exempt GET; the same argument
`handleStatus`'s doc comment already makes applies verbatim.

It does **not** go through `graphPayloadJSON`'s cache seam (§11). That is
deliberate: the seam exists to avoid re-deriving a payload nothing asked to
change, and this endpoint's entire contract is that something did.

The endpoint's bytes must equal the inline block's bytes modulo `generated_at`
for the same corpus. That is a test, not a hope — one encoder, one build, one
assertion.

### 6.3 The button is absent, not disabled, without a server

`shell.html` already probes for a live serve: a relative `fetch('/api/ping')`
with a ~1s timeout requires `res.ok`, a JSON content type, and
`body.dossierx === "serve"`, and only then adds `comments-live` to `<body>`
[VERIFIED: `shell.html:1172-1180`]. On `file://` the fetch rejects and is
swallowed — the read-only viewer is the correct outcome.

`graph-ui.js` reuses that verdict rather than probing again. The refresh button
is **created** when the pane mounts its header and `document.body` carries
`comments-live`, and is **not created at all** otherwise. A disabled-looking
control in a static file would be a promise the artifact cannot keep; a control
that is simply not there says the truth, which is that this document is a
snapshot.

Because the pane is inert until first opened and the probe resolves in under a
second at page load, a human opening the pane finds the class already settled.
The header is rebuilt on every open, so a pane opened inside the probe window
gains the button on the next open rather than being wrong forever.

### 6.4 Refresh preserves the view

Pressing refresh must not throw away what the reader was looking at. The client
fetches, then calls `applyPayload(next)`, which preserves:

- **camera** — zoom level and pan offset, verbatim;
- **controls** — scope, granularity, overlay, enabled edge types, label toggle;
- **expanded-group set**, by group name;
- **node positions**, by id, for every node that still exists. A node whose id
  is new is seeded near its group's centroid rather than at the origin, so a
  refresh does not fling the layout apart.

Selection is preserved if the selected id survives; otherwise the detail panel
clears. The header timestamp updates to the new payload's `generated_at` — which
is the visible proof the button did something. A failed fetch leaves the graph
untouched and shows an inline notice; a broken pane is never an acceptable
outcome of asking for fresher data.

---

## 7. Where each piece of work lives

| Layer | New | Responsibility |
|---|---|---|
| `internal/graph` | new package | `Build` + `Encode`. Pure, deterministic, clock-free, no HTML, no I/O, fully unit-testable in Go. |
| `internal/render/graph_view.go` | new file | `graphPayloadJSON(cat, cfg, generatedAt) (template.JS, error)` — the render path's one call site, and **the named cache seam** (§11). Follows the existing `implink_view.go` / `depended_by_view.go` naming. |
| `internal/render/render.go` | edited | `go:embed` gains three paths; `loadTemplates` reads three byte blobs; `shellData` gains four typed fields; `buildShellData` takes a struct. |
| `viewer/template/shell.html` | edited | Four injection points, one pane root, one nav trigger, a three-line hash change. |
| `internal/serve/handlers.go` + `server.go` | edited | `handleGraph` and its one route line. |
| `graph-core.js` | new | Pure computation. No DOM, no canvas, no globals except one namespace. |
| `graph-ui.js` | new | DOM construction, canvas rendering, force layout, controls, panels, refresh, hash state. |
| `graph.css` | new | Pane chrome, the 20-slot categorical palette, the reserved governance hue. |
| `internal/lint/mixed_cycle.go` | new | §13.1. Independent of everything above. |

### 7.1 Why three separate embedded files, not one more inline block

`shell.html` today carries exactly one `<script>` block, 1353 lines long
[VERIFIED: `grep -n '<script\|</script>' shell.html` → `107:  <script>` and
`1459:  </script>`, the only two matches]. Adding a graph engine to it would make
one file the single point of review for two unrelated subsystems.

Three files also draw a boundary that matters for testing: `graph-core.js` is
DOM-free and therefore unit-testable through a `chromedp.Evaluate` call against
one loaded page, at ~0.00s per case. `graph-ui.js` is not, and is proven by a
much smaller set of behavioural tests.

### 7.2 No `OverrideFile` path for any of the three

`shell.html` and `style.css` each carry a soft-fallback override sourced from
`cfg.Viewer.TemplateOverrides`. The three new files deliberately do **not**. They
are engine internals with a tight contract against the payload shape and against
each other; a project that swapped one for its own copy would get a pane that
fails in ways no error message could usefully describe. They are read straight
from `shellFS` with no override branch, and `render.go`'s package doc comment
gains a sentence saying so.

### 7.3 The client files are data, never templates

The `.js` and `.css` are read as bytes via `shellFS.ReadFile` and injected as
field values. They are never passed through `template.ParseFS`, so `{{ }}` in
their source is inert. Anything that must vary per project reaches the client
through the JSON payload, never through template actions.

### 7.4 Type at every injection site — this fails silently

| Field | Go type | Injection site |
|---|---|---|
| `GraphCSS` | `template.CSS` | `<style>{{.GraphCSS}}</style>` |
| `GraphPayload` | `template.JS` | `<script type="application/json" id="dossierx-graph">{{.GraphPayload}}</script>` |
| `GraphCoreJS` | `template.JS` | `<script>{{.GraphCoreJS}}</script>` |
| `GraphUIJS` | `template.JS` | `<script>{{.GraphUIJS}}</script>` |

A plain `string` in a `<script>` context is contextually escaped into a quoted JS
string literal. There is **no error at build time, render time or test time** —
the pane simply never initializes. Every assertion protecting this is therefore
made against **rendered output**, never against the Go value, and the wiring
lane's very first proving command is one of them.

---

## 8. Document order in `shell.html`

Script and style blocks land in source order, and nothing enforces it.

```
<head>
  <style>{{.GraphCSS}}</style>     <-- FIRST style block
  <style>{{.CSS}}</style>          <-- existing base stylesheet
  <style>{{.ThemeCSS}}</style>     <-- existing theme overrides, LAST
</head>
<body>
  <div class="layout"> … </div>
  … #commentsPanel, #statusStrip …
  <section id="dxgPane" hidden> … </section>
  <script type="application/json" id="dossierx-graph">{{.GraphPayload}}</script>
  <script>… existing 1353-line viewer runtime …</script>
  <script>{{.GraphCoreJS}}</script>
  <script>{{.GraphUIJS}}</script>
</body>
```

**`graph.css` must be the first `<style>` block, not the last.**
`TestRender_ThemeCSSInjectedAfterBaseCSS` locates blocks with `strings.Index` /
`strings.LastIndex` and asserts the *last* one carries the theme override
[VERIFIED: `internal/render/render_test.go:297-315`]. A third block placed last,
or between base and theme, breaks it. Placed first, both that test and
`TestRender_NoThemeConfiguredEmitsEmptyThemeStyleBlock` stay green **untouched**
— which is the point: an existing test that still passes for the same reason is
worth more than one edited to accommodate a new feature.

Ordering first also means `style.css` cascades over `graph.css`, and the theme
block cascades over both. `graph.css` selectors are all `#dxg`-prefixed and
`style.css` contains no graph-adjacent selector at all [VERIFIED: `grep -n
'graph' style.css` matches only the word "paragraph"], so there is no conflict to
resolve — only a documented direction if one ever arises.

`graph-core.js` must precede `graph-ui.js`. The payload block must precede both.

---

## 9. URL hash contract

Today the entire hash is one id: `hashId()` strips the leading `#` and
`resolve()` looks it up as a claim, then a build-order id, then a facet, then a
module, **falling back to the first module for anything unrecognized** [VERIFIED:
read `shell.html:192-208`]. A bare graph-state hash would therefore reset the
reading view to the first module, and `showModuleFacet`'s
`history.replaceState(null, '', '#' + hashTarget)` would then erase the graph
state it had just written.

The contract:

```
#<existing-target-id>!g=<compact-graph-state>
```

- `shell.html` gains a `hashGraphSuffix()` helper and three touched lines:
  `hashId()` truncates at the first `!`, and the two places that build the
  replacement hash append the preserved suffix. Hashes without `!` behave
  exactly as before.
- `graph-ui.js` owns everything after `!`. It writes via `history.replaceState`
  **only** — which does not fire `hashchange`, so a filter change never re-enters
  the reading-view routing at all.
- `graph-ui.js` listens for `hashchange` solely to pick up an externally pasted
  URL.
- The detail panel's "open claim" link sets `location.hash` to the claim id with
  the graph suffix preserved, which fires `hashchange` and reuses the existing
  deep-link scroll-and-highlight path with zero new code.

---

## 10. Scale

Canvas from day one; no SVG, no DOM node per claim.

Above **600 claim nodes** the pane opens at module granularity automatically,
shows a visible notice naming the number ("showing 1,240 claims collapsed into 9
modules"), and offers a manual override that **warns rather than blocks**. 600 is
a guess and will be wrong for somebody; it lives in one named constant in
`graph-ui.js`, is stated in the notice text, and gets revisited when a real large
corpus exists.

The pane is **inert until first opened**. At parse time `graph-ui.js` registers
one delegated listener and checks the hash. It builds no DOM, **parses no
payload**, starts no simulation. A reader who never opens the pane pays one
listener registration.

Parsing the payload at first open rather than at parse time is not only a cost
decision — it is what lets a browser test replace the payload block's
`textContent` before opening the pane, which §13.4 depends on.

---

## 11. Cost, and the named cache seam

`graph.Build` runs on **every render**. Under `serve` that means every `GET /`
and every `GET /api/fragment`, plus once per `GET /api/graph`.

`s.pipe.get` coalesces concurrent requests through a single-flight pipeline — at
most one render in flight plus one queued — but every render calls
`renderViewer` fully from scratch. There is no caching of rendered HTML across
render cycles [VERIFIED: `internal/serve/pipeline.go:47-96`]. There is also no
ETag or conditional-GET path that a larger document would interact with
[VERIFIED: grep for `ETag|If-None-Match|Cache-Control` across `internal/serve/`
returns only `claim_assets.go`'s `no-store` and `sse.go`'s `no-cache`].

That cost is **accepted** for v0.5.0. The seam is named rather than built:

> `internal/render/graph_view.go`'s `graphPayloadJSON` is the render path's
> single call site of `graph.Build` and the single place a future memoization
> belongs — keyed on the catalog's content, returning the cached `template.JS`
> with only `generated_at` restamped. Nothing outside that function needs to
> change to add it, and `graph.Build` staying a pure, clock-free function is what
> keeps that true.

`internal/graph` ships a benchmark so the number is measured rather than
asserted.

The other cost is document size: a 400-node payload took the generated
`index.html` from 108,577 to 218,214 bytes in Phase 0's P2 probe. That size is
asserted nowhere, but it lands in three tracked fixture viewers.

---

## 12. Explicitly out of scope

| Not shipping | Why |
|---|---|
| **Any code-grounding signal in the payload** | The graph audits claims, not code. `has_code_link`, the `locked_ungrounded` rule, its overlay and the `implinks` argument to `Build` are all removed outright, not deferred. `dossierx check`'s impl-link stage already answers this with its own vocabulary and exit code. |
| Per-claim local graphs | Doubles the client code; the global pane is what serves gap-hunting. |
| Code files as nodes | Node count balloons and every filter grows a second node class. |
| Editing from the graph | The viewer is a review surface; mutation is the CLI's job, and that separation is load-bearing. |
| A new CLI noun | The graph rides in render output. The noun list stays at six. |
| Server-side layout / persisted node positions | Layout is client state; a stored position would drift the moment a claim changes. |
| A new schema field | Nothing in `model.Claim` changes. |
| `OverrideFile` support for the three new files | §7.2. |
| Live refresh of the graph payload on an SSE fragment swap | §1.1. The swap does not re-deliver the payload; §6 is the answer, and it is a button, not a subscription. |
| `implink` drift state in the payload | Same reason as the first row. |
| Pixel/screenshot baselines | A force layout has no stable pixels. Every verdict is computed before anything is drawn, so a wrong pixel hides no logic. |
| A migration document for the new lint | §13.1. |

---

## 13. The three independent fixes shipping alongside

These ride in the same PR and the same branch because they are each one commit,
each fully independent of the graph, and each was surfaced by the work above.

### 13.1 A `mixed-cycle` lint, at error severity

A loop that alternates edge types — `A rests_on B`, `B governed_by A` — is
invisible to both existing cycle rules. `cycle.go`'s `findEdgeCycles` is a
single shared DFS walked once per call with a **single** `edgesOf` function;
`CycleLint` passes `c.RestsOn` only and `GovernedCycleLint` passes
`governedByEdges` only, and neither takes the union [VERIFIED:
`internal/lint/cycle.go:36-44,68-151`, `internal/lint/governed_cycle.go:49-63`].
So the mixed loop has no `rests_on` back-edge for one walk to find and no
`governed_by` forward edge for the other.

`mixed-cycle` walks the **union** graph with the edge kind carried on each hop,
and reports a cycle **only when that cycle's hops include at least one
`rests_on` and at least one `governed_by`**. That restriction is not
fastidiousness: `tests/lint_fixtures_test.go`'s
`testLintFixtureFiresExactlyOneRule` requires each coverage fixture to trip its
own rule and nothing else, so a rule that also fired on the `cycle` and
`governed-cycle` fixtures would break both of them [VERIFIED: read
`tests/lint_fixtures_test.go:75-113`, including the `coFiresWith` map and the
`len(entries) != 27` assertion].

Severity is **error**, matching `cycle` and `governed-cycle`. The message names
the path with its edge kinds: `mixed rests_on/governed_by cycle detected: a
-(rests_on)-> b -(governed_by)-> a`.

The walk is iterative over an explicit frame stack, for the same reason
`findEdgeCycles` is: recursion depth here is the length of the longest authored
edge chain, with no engine-imposed bound.

**There is no migration document.** A corpus containing a mixed cycle was always
malformed; the engine simply could not see it. Naming that as a migration would
imply projects had been relying on it.

Registering a rule is three files, not one, and the repository says so out loud:
`internal/lint/lint.go`'s package doc enumerates every rule and states "THIS
COUNT AND THIS LIST ARE LOAD-BEARING"; `tests/lint_fixtures_test.go` asserts an
exact directory count; `tests/lint_coverage_meta_test.go` fails on any registered
rule that never fires across the corpus.

### 13.2 The offline scan must not fail a build over a comment

`tests/portability_test.go`'s `TestNoNetworkReferencesAnywhereInEngine` walks
`.go`/`.html`/`.css`/`.js` under `cmd/` and `internal/` and fails on
`(?i)https?://|cdn\.|fonts\.googleapis|fonts\.gstatic|analytics|telemetry|sentry|segment\.io`
after stripping loopback URLs [VERIFIED: read
`tests/portability_test.go:430-495`; Phase 0's P2 negative control confirmed the
walk reaches a new `.js` file under `internal/render/viewer/template/`].

The property it protects is real and must not weaken: the shipped engine makes no
network request. But a **citation in a comment** makes no request. Today a doc
comment naming the paper an algorithm came from fails the build, which pushes an
author toward writing worse comments — precisely the wrong incentive for two new
1,000-line client files whose non-obvious algorithms deserve citation.

So `scanForNetworkRefs` gains a `.js`-only pre-pass that blanks `//` line
comments and `/* … */` block comments before matching, preserving line numbering
so offender line numbers stay correct. It is **string-literal aware**: a `//`
inside a `'`, `"` or backtick string does not start a comment, so
`const u = "https://evil.example/x";` still fails, and the existing positive
control (`fetch("https://evil.example/x")`) still fires.

Scope is `.js` only. `.go`, `.html` and `.css` are unchanged.

### 13.3 A fixture-staleness test

`testdata/fixture-basic/viewer/index.html` and
`testdata/fixture-portability/viewer/index.html` are tracked generated artifacts
that **no test reads**, so a rendering or CSS change ships without them going
stale. `docs/RELEASING.md` says so in its own words and names the two releases
it happened in: v0.3.1 and v0.4.1, caught by review each time, not by CI
[VERIFIED: read `docs/RELEASING.md:57-74`].

The new test closes that hole. For every `testdata/fixture-*/` that has both a
`project.config.yaml` and a committed `viewer/index.html`: copy the fixture into
a temp dir, run `dossierx check` against the copy, normalize the timestamps, and
compare `viewer/index.html` and `.catalog.json` byte-for-byte against the
committed ones.

Two normalizers, by **format**, not by position:

1. RFC3339 (`2026-08-03T20:51:05Z`) — line 1's `<!-- generated by dossierx check
   at … -->` header and, from this release, the payload's `generated_at`.
2. `2006-01-02 15:04 UTC` — the sidebar footer's `Generated …` string.

`.catalog.json` carries no timestamp at all [VERIFIED: `grep -c generated
testdata/fixture-basic/.catalog.json` → 0], so it is compared raw.

The fixture list is **discovered, not hardcoded**, so the test cannot itself go
stale when a fourth fixture appears — and so it needs no knowledge of whether
`testdata/fixture-graph-demo` has been created yet.

The consequence to state plainly, because it changes how this PR is committed:
**a commit that changes rendered output and does not regenerate the fixtures is
now red.** Regeneration stops being a release-time checklist habit and becomes a
mechanical property of the commit that caused it.

### 13.4 What C1 costs: the demo corpus cannot demonstrate cycle detection

`dossierx check` returns on the *first* error-severity lint partition, above the
catalog and render stages [VERIFIED: `internal/check/check.go:225-227`]. `cycle`,
`governed-cycle`, `self-edge` and now `mixed-cycle` are all error severity.
Therefore **no fixture that must pass `check` can contain any cycle of any
shape** — not a `rests_on` loop, not a `governed_by` loop, and, from this
release, not the mixed loop either.

An earlier draft of this design seeded exactly one mixed cycle into the demo
corpus, precisely because it was the only cycle shape `check` would render.
Adding `mixed-cycle` closes that door. This is a deliberate loss, not an
oversight, and it is worth taking: a lint that fails the build is a stronger
guarantee than a picture of the defect.

Cycle detection is instead proven two ways, neither of which needs a
cycle-carrying fixture:

1. **Unit tests over synthetic edge sets.** `graph-core.js`'s `scc` is
   table-driven from Go over arrays that never touch a claim file; the
   `mixed-cycle` lint's Go tests are table-driven over `[]model.Claim` literals.
2. **A viewer test that injects a cycle-carrying payload directly.** Because the
   pane parses the payload block at first open (§10), a browser test can replace
   `#dossierx-graph`'s `textContent` with a payload containing a cycle, then open
   the pane and assert the gaps rail lists the `cycle` rule id with the expected
   member ids. The rendered document never contained a cycle; the pane did.

The demo fixture still seeds every other gap class. The gaps rail's cycle block
is simply empty when a reader opens it, which — for a corpus that passes `check`
— is the correct and honest reading.

---

## 14. Colour and tokens

`style.css` declares a **fixed** `viewer.theme` token allowlist — `--accent`,
`--accent-bg`, `--ink`, `--muted`, `--faint`, `--paper`, `--card-bg`, `--border`,
`--link`, `--warn`, `--warn-bg`, `--font-sans`, `--font-mono`, `--radius` —
cross-referenced to `internal/config/config.go`. Extending it is a config change
and is out of scope here.

So:

- **Pane chrome** (panel, control bar, rail, legend, buttons, borders, text)
  consumes those tokens via `var(--token)` exclusively. It inherits project
  theming and light/dark behaviour for free.
- **Categorical data colours** (the 20 facet slots, the reserved
  `--dxg-governed` hue, cycle red, halo amber) cannot come from that allowlist —
  it has no categorical slots. `graph.css` defines its own `--dxg-*` custom
  properties with light and dark values, and this is the file's documented,
  deliberate second exception to the token rule. The first is the `.gcp-*` /
  `.mockup-*` block, which is explicitly framed as "not a precedent" — so this one
  carries its own justification rather than citing that one: a categorical
  palette must stay mutually distinguishable, which a single-accent theme token
  cannot guarantee.
- The canvas reads those custom properties through
  `getComputedStyle(document.documentElement).getPropertyValue(name)` — the same
  mechanism the prototype uses — so a canvas repaint after an OS light/dark
  switch picks up the new values with no JS palette duplication.

### 14.1 z-index

The existing stack is hand-assigned with explicit reasoning: nav overlay 20,
30, nav toggle 40, status strip 45, comments overlay 50, comments rail 60, toast
70. The graph pane is a full-viewport overlay and takes the band **above all of
them**: backdrop 80, pane 81. Its own body-scroll-lock class is additive with the
existing `body.nav-open` / `body.comments-open` locks — three classes each
setting `overflow: hidden` compose without a release-order hazard. The z-index
ledger comment in `style.css` is extended to record the new band; that comment is
the single place the stack is reasoned about, and it stays that way.

---

## 15. What proves what

| Property | Proven by | Where |
|---|---|---|
| Payload correctness, determinism, nil-safety, clock-freedom | Go unit tests over `graph.Build` | `internal/graph` (root suite) |
| Payload escaping (a claim label containing `</script>`) | Go test asserting on **rendered output**, written first | `internal/render` (root suite) |
| Typed injection actually reached the page | Go test asserting the rendered `<script>` body is the file's bytes, not a quoted literal | `internal/render` (root suite) |
| `SetEscapeHTML(false)` appears nowhere | repo-wide grep, inside the payload lane's own proving command | payload lane gate |
| Block ordering (`graph.css` first, payload before core before ui) | Go test on index positions | `internal/render` (root suite) |
| `GET /api/graph` agrees with the inline block | Go test comparing bytes modulo `generated_at` | `internal/serve` (root suite) |
| The offline property of the new `.js`/`.css` | `TestNoNetworkReferencesAnywhereInEngine`, now comment-aware | `tests/` (root suite) |
| Committed fixture viewers are not stale | fixture-staleness test | `tests/` (root suite) |
| The mixed-cycle defect class | Go unit tests + a coverage fixture | `internal/lint`, `tests/` (root suite) |
| Pure client logic: SCC, representatives, aggregation, degrees, facet slots, governors, gap rules | Table-driven `chromedp.Evaluate` over **one** page load | `viewer-tests` (**not** the root suite) |
| Cycle rendering in the pane | injected cycle-carrying payload, §13.4 | `viewer-tests` |
| Pane behaviour: inert-until-opened, hash, fragment-swap survival, refresh, timestamp | chromedp against static and live pages | `viewer-tests` |
| Refresh button absent on `file://`, present under `serve` | chromedp, both page kinds | `viewer-tests` |
| Isolated-vs-orphan parity | chromedp + `dossierx check --format json` | `viewer-tests` |
| Every remaining gap class is seeded in the demo fixture | Go test over the fixture's payload facts | `internal/graph` (root suite) |

**`viewer-tests` is a separate Go module and the root `go test ./...` cannot
reach it.** It runs only via `make viewer-test` or CI's `viewer` job [VERIFIED:
`Makefile`, `.github/workflows/ci.yml`]. Its coverage is real but invisible to
the default gate, and must not be counted as covered by it.

The test shape in `viewer-tests` is load-bearing, not stylistic: **one Go test
func = one page load = N table cases**. In that shape a case costs ~0.00s; the
naive one-func-per-case shape costs ~1.0s each [VERIFIED by Phase 0's P3 timing
decomposition]. `browserContext` imposes a 60s per-tab ceiling, so one test func
must stay well under it. Exported client functions therefore take and return
**plain JSON-able values only** — no `Map`, no `Set`, no cyclic objects — because
the test boundary is `json.Marshal` in and CDP `returnByValue` out.

---

## 16. Accepted risks

1. **Stale graph during a live session.** §1.1. Mitigated, not eliminated, by
   §6: the pane says how old the payload is, and offers a button that replaces
   it. A reader who does neither still sees stale data.
2. **Twenty categorical colours are not twenty distinguishable colours.** §4.2.
   Mitigated by making colour one of three identity channels rather than the
   only one.
3. **Heuristic false positives are guaranteed.** A module legitimately without a
   verification phase will be listed. The visual separation and the wording are
   what keep that honest rather than annoying.
4. **The panel may diverge from `check`.** Scope makes divergence unavoidable.
   One parity test (§5.1) is load-bearing for a property nobody can see by
   reading either side, and is named for what it protects.
5. **Canvas drawing code stays untested.** Accepted: it carries no logic a wrong
   pixel would hide.
6. **The three client files have no live-reload story.** They are baked in at
   compile time; the watcher only watches `cfg.ClaimsDir` [VERIFIED:
   `internal/serve/watcher.go` + `server.go:257`]. Editing `graph-core.js`
   triggers nothing — a rebuild and a `serve` restart are required.
7. **`mixed-cycle` at error severity can fail a corpus that used to pass.** That
   corpus was malformed; §13.1 argues why that is the right trade and why no
   migration document accompanies it.
8. **The fixture-staleness test runs `check` as a subprocess for each fixture**,
   which is the slowest thing in the root suite. Three fixtures, a few seconds.
   If it ever becomes the reason someone skips the suite, it is worth revisiting
   — but a test nobody runs is exactly the state this test exists to end.

---

## 17. Evidence appendix

Everything marked VERIFIED was read or commanded during Phase 0 or this design
pass. What remains ASSUMED, and where it bites:

- **ASSUMED** — that `dossierx lock` on a brand-new project creates its lock
  store without a pre-ledger migration step. `preLedgerPrecondition` exists in
  `cmd/dossierx/main.go` and applies to projects whose locks *predate* the
  ledger, which a fixture created today is not; `internal/lock` was not read.
  Affects demo-fixture construction only; the plan carries it as a conditional
  step with a named fallback.
- **ASSUMED** — that opening a comment thread on an already-locked claim sets
  `review_pending`. Taken from `internal/model/claim.go:330-357` and
  `internal/lint/comments_unresolved.go`'s doc comments, not from reading
  `internal/lock`. Same blast radius, same treatment.
- **ASSUMED** — that a rendered `viewer/index.html` contains no bytes derived
  from its project's absolute path. If it does, the fixture-staleness test needs
  a third normalizer. The plan carries this as a conditional step.
- **ASSUMED** — that `cfg.DoctrineFacet` being unset means no doctrine gate
  applies to `governed_by` targets. Inferred from the field being `omitempty`
  and from `testdata/fixture-coverage/lifecycle/doctrine-gate` existing as a
  *separate* fixture; `internal/lint`'s doctrine rule was not read. Affects the
  demo fixture's construction only.

`dossierx check --format json` **does** emit a per-finding rule id — the
envelope's `data.lint_findings[].lint` key, read today by
`tests/lint_fixtures_test.go`'s `lintFinding` struct [VERIFIED this pass]. The
parity test keys off it, and this is no longer an assumption.
