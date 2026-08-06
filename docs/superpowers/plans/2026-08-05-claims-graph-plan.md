# DossierX claims graph — implementation plan

Date: 2026-08-05
Branch: `v0.5.0` (cut from `main`)
Design: [`docs/superpowers/specs/2026-08-05-claims-graph-design.md`](../specs/2026-08-05-claims-graph-design.md)

Six lanes, 78 lane steps, 2 integration steps, **eight checkpoint commits**.
Every commit leaves the tree green under both gates.

---

## 0. Governing constraints

1. **Every step names the exact command that proves it worked.** Where a step's
   proof is weaker than execution — a `grep`, a syntax check — the step says so
   and names the later step that actually executes it.
2. **Every file has exactly one owning lane.** The table in §2 is exhaustive.
   Scan it column-wise: no path appears twice.
3. **No step depends on a file another lane has not yet written.** The lane order
   in §1 is load-bearing, not a preference.
4. **No step rests on an unverified claim.** Five do; each is marked
   `CONDITIONAL` inline and listed in §7 with its fallback.

---

## 1. Lane order, and the correction to the stated derivation

```
        ┌──────────────────────────┬──────────────────────────┬─────────────────────────┐
        │ L1  internal/graph       │ L3  graph-core.js        │ L6  lint + test hygiene │
        │     + demo fixture inputs│     (pure computation)   │     (3 commits)         │
        └───────────┬──────────────┴───────────┬──────────────┴─────────────────────────┘
                    │                          │
                    │                          ▼
                    │              L4  graph-ui.js + graph.css
                    │                          │
                    └────────────┬─────────────┘
                                 ▼
                    L2  render.go + shell.html + style.css
                        + serve handler + fixture regeneration
                                 │
                                 ▼
                    L5  viewer-tests/
                                 │
                                 ▼
                    Integration  whole-suite gate
```

**L1, L3 and L6 run together.** Their owned paths are disjoint and no step in any
of them reads a file another writes. L3 is written against the payload *contract*
in design §2.1 — a JSON shape stated in a document — not against L1's Go files,
so it does not wait on L1.

**Two corrections to the derivation as briefed.**

1. **L2 depends on L1, not only on L3 and L4.** `internal/render/graph_view.go`
   imports `internal/graph` and calls `Build` and `Encode`. The brief's derived
   order named only the `go:embed` compile-time dependency. Both hold; L2 is
   after all three.
2. **L2 owns the regenerated fixture viewers, and this is forced by L6.** L6's
   fixture-staleness test (step 33) fails the moment rendered output changes and
   the committed fixtures do not. Deferring regeneration to an integration phase
   would leave L2's own commit red. So the regeneration steps live at the end of
   L2, in L2's commit. This is the staleness test working as designed: it
   converts a release-time checklist habit into a property of the commit that
   caused the drift.

Everything else in the briefed order is confirmed:

- **L4 after L3** — `graph-ui.js` calls `window.dossierxGraphCore`.
- **L2 after L3 and L4** — `//go:embed` is resolved at compile time; a directive
  naming `viewer/template/graph-core.js` fails `go build` if the file does not
  exist. The alternatives were each rejected: embedding the whole directory and
  tolerating a missing file at render time turns a wiring mistake into a silent
  empty pane; having L2 create stubs that L3/L4 overwrite gives two lanes one
  file.
- **L5 after L2** — every test in it needs a rendered page.

**The cost of that order, stated plainly.** L3 and L4 ship roughly 1,600 lines of
JavaScript that no CI gate executes until L2 lands and no test asserts on until
L5. That is the single largest risk in this plan. Three things reduce it:

- Every L3/L4 step carries a `node --check` syntax proof, and every *pure* L3
  function carries a `node -e` execution proof — both dependency-free, both
  developer-loop only, never a CI gate. [VERIFIED on this machine: `node
  --version` → `v24.13.0`.]
- §6's traceability table names, for each L3/L4 function, the L5 test that
  executes it. Nothing is written without a named later executor.
- L3 and L4 agents may build a throwaway scratch harness **outside the repo**;
  each lane gate asserts `git status --porcelain` shows only owned paths.

**One cross-lane invariant, briefed into both L1 and L6.** L6 adds `mixed-cycle`
at error severity. Design §13.4: no fixture that must pass `check` may contain a
cycle of *any* shape. L1's demo corpus therefore contains no cycle at all, and
this holds regardless of which of the two lanes commits first.

---

## 2. File ownership

**No path appears twice.**

| Path | Lane |
|---|---|
| `internal/graph/payload.go` | L1 |
| `internal/graph/build.go` | L1 |
| `internal/graph/encode.go` | L1 |
| `internal/graph/build_test.go` | L1 |
| `internal/graph/encode_test.go` | L1 |
| `internal/graph/fixture_test.go` | L1 |
| `internal/graph/bench_test.go` | L1 |
| `.golangci.yml` | L1 |
| `testdata/fixture-graph-demo/project.config.yaml` | L1 |
| `testdata/fixture-graph-demo/claims/**` | L1 |
| `testdata/fixture-graph-demo/.dossierx-lock-store.json` | L1 |
| `testdata/fixture-graph-demo/.dossierx-comment-digest.json` | L1 |
| `internal/render/viewer/template/graph-core.js` | L3 |
| `internal/lint/mixed_cycle.go` | L6 |
| `internal/lint/mixed_cycle_test.go` | L6 |
| `internal/lint/lint.go` | L6 |
| `tests/lint_fixtures_test.go` | L6 |
| `testdata/fixture-coverage/lint/mixed-cycle/**` | L6 |
| `tests/portability_test.go` | L6 |
| `tests/fixture_staleness_test.go` | L6 |
| `internal/render/viewer/template/graph-ui.js` | L4 |
| `internal/render/viewer/template/graph.css` | L4 |
| `internal/render/render.go` | L2 |
| `internal/render/graph_view.go` | L2 |
| `internal/render/graph_render_test.go` | L2 |
| `internal/render/viewer/template/shell.html` | L2 |
| `internal/render/viewer/template/style.css` | L2 |
| `internal/serve/handlers.go` | L2 |
| `internal/serve/server.go` | L2 |
| `internal/serve/graph_handler_test.go` | L2 |
| `docs/RELEASING.md` | L2 |
| `testdata/fixture-basic/viewer/index.html` | L2 |
| `testdata/fixture-basic/.catalog.json` | L2 |
| `testdata/fixture-portability/viewer/index.html` | L2 |
| `testdata/fixture-portability/.catalog.json` | L2 |
| `testdata/fixture-graph-demo/viewer/index.html` | L2 |
| `testdata/fixture-graph-demo/.catalog.json` | L2 |
| `viewer-tests/graph_core_test.go` | L5 |
| `viewer-tests/graph_pane_test.go` | L5 |
| `viewer-tests/graph_parity_test.go` | L5 |

**`internal/serve/server.go` is L2's, and it was not in the brief.** The brief
assigned `internal/serve/handlers.go` to the render-wiring lane, but route
registration lives in `Server.routes()` at `server.go:367-384`, not in
`handlers.go` [VERIFIED]. `handleGraph`'s body goes in `handlers.go`; its one
`mux.HandleFunc` line goes in `server.go`. Both are L2's.

**`internal/render/render_test.go` is touched by nobody.** L2's new assertions go
in a new file. The two existing style-block tests must keep passing *unedited* —
see step 54.

**`testdata/fixture-graph-demo`'s inputs are L1's; its generated output is
L2's.** L1 authors the corpus and proves it legal, then deletes the generated
`viewer/` and `.catalog.json` before committing. Between L1's commit and L2's,
the demo fixture has no committed viewer — and L6's staleness test discovers
fixtures rather than hardcoding them, so it simply does not consider a fixture
that has no committed viewer to compare against. That window is deliberate and
closes in L2.

---

## 3. Commands used repeatedly

### `provetest` — the only sanctioned way to run a `-run` gate

**Every `go test … -run` proof in this plan MUST go through this.** A `-run`
pattern that matches no test prints `ok … [no tests to run]` and exits **0**, so
the step reads as proven when nothing ran. That has now been caught five times in
this plan alone — twice by the audit, three times during the build — and never by
the gate itself.

```sh
provetest() {            # provetest <pkg> <pattern> [extra go test flags…]
  pkg=$1; pat=$2; shift 2
  n=$(go test -list "$pat" "$pkg" 2>/dev/null | grep -c '^\(Test\|Benchmark\|Example\)')
  [ "$n" -gt 0 ] || { echo "VACUOUS GATE: pattern '$pat' matches no test in $pkg" >&2; return 1; }
  go test "$pkg" -run "$pat" -count=1 "$@"
}
```

It refuses to report success for a pattern that matches nothing. Use it even
when you are certain the test exists — the case that bites is the one where a
lane silently skipped writing it.

The one deliberate exception is `-run '^$'`, which exists to run *only*
benchmarks and is supposed to match no test. Call `go test` directly there and
say why in the step.

```sh
# Root gate (what CI's default job runs)
go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go')

# Browser gate (CI's `viewer` job; the root gate CANNOT reach it)
make viewer-test

# Offline scan — walks .go/.html/.css/.js under cmd/ and internal/.
# VERIFIED by Phase 0's P2 negative control that it reaches a new .js here.
provetest ./tests/ 'TestNoNetworkReferencesAnywhereInEngine' -v

# The forbidden call. Must print nothing, anywhere, ever.
! grep -rn "SetEscapeHTML" internal/ cmd/ tests/

# JS syntax check (CONDITIONAL on node; developer loop only, never CI)
node --check internal/render/viewer/template/graph-core.js

# JS execution harness for pure functions (CONDITIONAL on node)
node -e "globalThis.window=globalThis; \
  (0,eval)(require('fs').readFileSync('internal/render/viewer/template/graph-core.js','utf8')); \
  console.log(JSON.stringify(window.dossierxGraphCore.scc(['a','b','c'],[['a','b'],['b','a']])))"
```

`golangci-lint` is **not installed on the development machine** [VERIFIED:
`which golangci-lint` → not found]. No step's proof may require it; CI's `lint`
job is its only runner.

---

## 4. Lanes

### L1 — `internal/graph` + the demo fixture's inputs

Pure Go, no browser, no client files. Everything here runs under the root gate.

**Brief for the implementing agent**

- **`Build(cat, cfg)` takes two arguments.** There is no `implinks` parameter, no
  `has_code_link` node field, and no import of `internal/implink`. If you find
  yourself needing one, you are building the wrong feature: design §0 and §12.
- `Build` is **total**: nil `cat` and nil `cfg` must both return a valid payload,
  never panic, never error. `Render` must not be able to fail because of this
  package.
- `Build` does **no I/O and reads no clock**. `time` must not appear in
  `build.go`. `GeneratedAt` is stamped by the caller — design §2.5.
- **`SetEscapeHTML(false)` is a forbidden call.** So is `json.Encoder`, and so is
  any hand-assembled JSON. Use `json.Marshal`. Its default HTML escaping is the
  *only* thing between an author-authored claim label and a `</script>` breakout
  in a `<script type="application/json">` block — design §2.6. This is step 2,
  not a hardening pass.
- Read `Kind` via `EffectiveKind()`, never the raw field.
- `governed_by` produces an edge only when the type is neither `""` nor `"none"`
  — the same guard `internal/lint/dangling.go` uses.
- Do not import `internal/render`, `internal/lint` or `internal/implink`.
- **The demo corpus contains no cycle of any shape.** Design §13.4.

| # | Step | Proving command |
|---|---|---|
| 1 | `payload.go`: `SchemaVersion = 1`, `Payload`, `Node`, `Edge`, `Groups`, `Dropped` with the exact JSON tags of design §2.1. No `has_code_link`. `GeneratedAt` documented as caller-stamped. Package doc comment states the escaping rule and names `SetEscapeHTML(false)` as forbidden. | `go build ./internal/graph/ && gofmt -l internal/graph` (expect empty) |
| 2 | **XSS first.** `encode.go`: `Encode(Payload) ([]byte, error)` via `json.Marshal`. Test feeds a claim id whose derived label contains `</script><img src=x>` and a facet of the same, and asserts the output contains no literal `</script>`, contains the escaped `</script`, and round-trips through `json.Unmarshal` to the original strings. | `provetest ./internal/graph/ 'TestEncodeEscapesScriptClose' -v && ! grep -rn "SetEscapeHTML" internal/ cmd/ tests/` |
| 3 | `Build` node emission: every field of design §2.2, sorted by `id`. Table test covers empty module, empty facet, empty `build_role`, overview-facet kind inference, and asserts the marshalled node object has **no** `has_code_link` key. | `provetest ./internal/graph/ 'TestBuildNodes' -v` |
| 4 | `Build` edge emission: `rests_on`, `mirrors`, `governed_by` (with the `""`/`"none"` guard); targets not in the known-id set dropped and counted into `dropped.unresolved_edges`; sorted by `(from, type, to)`. | `provetest ./internal/graph/ 'TestBuildEdges' -v` |
| 5 | `in_degree` / `out_degree` over all three types project-wide; `groups.modules` = `cfg.Modules` order then extras sorted; `groups.facets` likewise. | `provetest ./internal/graph/ 'TestBuildDegrees|TestBuildGroups' -v` |
| 6 | Determinism: build the same corpus 100× from shuffled input slices and from a `map`-sourced catalog; assert `Encode` output is byte-identical every time. | `provetest ./internal/graph/ 'TestBuildDeterministic' -v` |
| 7 | Clock-freedom: `Build` leaves `GeneratedAt == ""`; two builds a measurable interval apart are byte-identical. | `provetest ./internal/graph/ 'TestBuildDoesNotStampTime' -v && ! grep -n '"time"' internal/graph/build.go` |
| 8 | Nil-safety: `Build(nil, nil)`, `Build(&catalog.Catalog{}, nil)`, claims with empty IDs. | `provetest ./internal/graph/ 'TestBuildNilSafe' -v` |
| 9 | `BenchmarkBuild` over a 1,000-claim synthetic corpus, so the per-request cost design §11 accepts is a measured number. | `go test ./internal/graph/ -run '^$' -bench BenchmarkBuild -benchtime 20x` |
| 10 | Import hygiene: add a `depguard` rule to `.golangci.yml` forbidding `internal/render`, `internal/lint` and `internal/implink` from `internal/graph`. **CONDITIONAL** (§7.1) — the rule is only executable by CI's lint job; the property is proven locally by grep. | `grep -rn "dossierx/internal/\(render\|lint\|implink\)" internal/graph/` (expect no output) |
| 11 | Demo fixture inputs: `project.config.yaml` (5 modules, 5 facets, **no `doctrine_facet`**) and ~60 claim YAMLs, all `status: draft` at this step. Honour the legality checklist below. | `go run ./cmd/dossierx --format text check --config testdata/fixture-graph-demo/project.config.yaml` → `check: OK` |
| 12 | Seed the engine-managed state, in this order: lock the locked subset (`dossierx lock <id> --reason …`), then `dossierx comment add` on one already-locked claim to raise `review_pending`. **CONDITIONAL** (§7.2). | `go run ./cmd/dossierx --format text check --config testdata/fixture-graph-demo/project.config.yaml` → `check: OK`, exit 0 |
| 13 | `fixture_test.go`: load the fixture through `loader` + `catalog` + `graph.Build` and assert **on payload facts** that each remaining gap class has ≥1 instance: a zero-edge node, a one-edge node, a module with no cross-module inbound edge, a module with no cross-module edges at all, a `review_pending` node, an `open_comments > 0` node, a draft node, a locked node, a governed node whose governor also governs something else, and a module missing a `build_role` phase. Assert additionally that the corpus contains **zero** cycles of every shape (test-local union-graph SCC). | `provetest ./internal/graph/ 'TestDemoFixtureSeedsEveryGapClass' -v` |
| 14 | Lane gate. Remove the generated `viewer/` and `.catalog.json` — L2 owns them. | `rm -rf testdata/fixture-graph-demo/viewer testdata/fixture-graph-demo/.catalog.json && go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go') && git status --porcelain` |

**Fixture legality checklist — the corpus must pass `check`, which returns above
the render stage on any error-severity finding.** Getting this wrong is the most
likely way L1 stalls.

| Must NOT contain | Because |
|---|---|
| A `rests_on` cycle | `cycle` lint, error severity. |
| A `governed_by` cycle | `governed-cycle` lint, error severity. |
| **A mixed `rests_on`/`governed_by` cycle** | `mixed-cycle` lint, error severity, landing in L6 this same PR. |
| A self-edge of any kind | `self-edge` lint, error severity. |
| A dangling `rests_on` / `mirrors` / `governed_by` target | `dangling` + `validated-on-missing`, error severity. |
| A non-reciprocal `mirrors` pair | `mirror-reciprocal`, error severity. |
| Mirrored claims whose body/rows/steps differ | `mirror-mismatch`, error severity. |
| A locked claim resting on a draft claim | `rest-on-locked`, error severity. |
| A locked claim with no `build_role` in a module that adopted the field | `build-role-required-for-locked`, error severity. |
| A claim with no `governed_by.type`, or `type: none` with no reason | `governed-required`, error severity. |

Warning-severity classes are free to include: `orphan` (isolated claims) and
`comments-unresolved` are both warnings and do not fail `check`.

---

### L3 — `graph-core.js`

Pure computation. **No DOM, no canvas, no `document`, no `window` beyond one
namespace assignment.** Everything here must be callable from a bare `node -e`
and from a single `chromedp.Evaluate`.

**Brief for the implementing agent**

- The file is an IIFE that assigns exactly one global:
  `window.dossierxGraphCore = { … }`. Bind the root as
  `(typeof window !== "undefined" ? window : globalThis)` so the node harness
  works. This exported set is a **stated API of the file**, listed in a doc block
  at the top, not an accident — L5 hangs its whole suite off it.
- **Every exported function takes and returns plain JSON-able values only.** No
  `Map`, no `Set`, no cyclic objects, no `undefined`, no `NaN`, no `Infinity`
  crossing the boundary. Internal use of `Map`/`Set` is fine. The test boundary
  is `json.Marshal` in and CDP `returnByValue` out; a `Map` return is
  unassertable.
- **`tests/portability_test.go` walks this file.** Its regex is
  `(?i)https?://|cdn\.|fonts\.googleapis|fonts\.gstatic|analytics|telemetry|sentry|segment\.io`
  after stripping loopback URLs. From L6 step 30 onward, `//` and `/* */`
  comments in `.js` are exempt — but **executable code is not**, and a URL inside
  a string literal still fails. The demo fixture uses a module named `telemetry`;
  do not hardcode that string anywhere in this file.
- SCC must be **iterative**, not recursive — a deep `rests_on` chain must not
  blow the JS stack.
- Output ordering must be deterministic: components sorted by their smallest
  member id, members sorted within a component. The Go tests assert on exact
  arrays.
- Nothing here knows about colour. `facetSlot` returns an integer index; which
  hex it maps to is `graph.css`'s business.

| # | Step | Proving command |
|---|---|---|
| 15 | File skeleton: IIFE, root binding, `dossierxGraphCore` namespace, and the doc block listing the exported testing surface. | `node --check …/graph-core.js` **(CONDITIONAL §7.3)** and `grep -n "dossierxGraphCore" internal/render/viewer/template/graph-core.js` |
| 16 | `scopeFilter(nodes, scope)` and `representatives(nodes, granularity, expandedGroups)` → `{repByClaim, repNodes}`. Design §3. | `node -e` harness calling `representatives` on a 3-module fixture, printing JSON **(CONDITIONAL)**; executed for real by step 69 |
| 17 | `aggregateEdges(edges, repByClaim, enabledTypes)` → `[{from,to,type,weight}]`, self-loops dropped, aggregated by `(from,to,type)`, sorted. | `node -e` harness **(CONDITIONAL)**; executed for real by step 69 |
| 18 | `degrees(nodeIds, edges)` → `{id:{in,out,total}}`, scope-relative. | `node -e` harness **(CONDITIONAL)**; executed for real by step 69 |
| 19 | `scc(nodeIds, edges)` → `[[id,…]]`. Iterative Tarjan over the **claim-level** edge set, directed types only, deterministic ordering. Components of size 1 returned only when they carry a literal self-edge; `selfEdges(nodeIds, edges)` returns those separately. Design §3.1. Include a 10,000-node chain case to prove the iterative walk. | `node -e "…scc(['a','b','c'],[['a','b'],['b','a']])"` **(CONDITIONAL)**; executed for real by step 69 |
| 20 | `facetSlot(facets, facet)` → integer `0..19`, `(index in facets) mod 20`, and `-1` for a facet absent from the list or empty (the `--dxg-facet-other` case). **Never keyed on the facet's name.** Design §4.2. | `node -e` printing the slot for a 23-facet list, asserting slot 20 wraps to 0 **(CONDITIONAL)**; executed for real by step 69 |
| 21 | `governors(edges)` → sorted ids that are the target of ≥1 `governed_by` edge (the wedge-marker set), and `governanceScope(edges)` → `{nodeIds, edgeKeys}` for the governance overlay. Design §4.3. | `node -e` harness **(CONDITIONAL)**; executed for real by step 69 |
| 22 | `gapRules` — the eight **fact** rules of design §5, each emitting `{rule, node_ids, kind:"fact"}` with the stable rule ids from that table. No `locked_ungrounded`. | `node -e` harness over the seeded shapes **(CONDITIONAL)**; executed for real by step 69 |
| 23 | The two **heuristic** rules (`missing_build_phase`, `density_outlier`), emitted with `kind:"hint"` into a separate array so the panel cannot render them alongside facts by accident. | `node -e` harness **(CONDITIONAL)**; executed for real by step 69 |
| 24 | `encodeState(state)` / `decodeState(string)` for the hash segment of design §9. Round-trip lossless and stable (same state → same string). | `node -e` round-trip printing `encodeState(decodeState(encodeState(s))) === encodeState(s)` **(CONDITIONAL)**; executed for real by step 69 |
| 25 | Lane gate. | `provetest ./tests/ 'TestNoNetworkReferencesAnywhereInEngine' -v && go build ./... && go test ./... && git status --porcelain` |

---

### L6 — the mixed-cycle lint and two test-hygiene fixes

Three independent fixes, **three commits**. Nothing here touches the graph.

**Brief for the implementing agent**

- Commit after each of the three sub-gates (steps 29, 32, 35). They are
  independent and must be reviewable independently.
- **No migration document accompanies the new lint.** Design §13.1 says why.
- The staleness test **discovers** fixtures; it must never hardcode a list.

#### L6a — `mixed-cycle`

| # | Step | Proving command |
|---|---|---|
| 26 | **Written first, red.** `mixed_cycle_test.go`, table-driven over `[]model.Claim` literals: a 2-claim mixed cycle **fires**; a pure `rests_on` cycle does **not**; a pure `governed_by` cycle does **not**; a `rests_on` self-edge does **not**; a 10,000-link chain terminates without a stack overflow; `type: none` and `type: ""` contribute no edge. | `provetest ./internal/lint/ 'TestMixedCycle' -v` → **FAIL** (compile error: no `MixedCycleLint`). That failure IS this step's proof — a `PASS … [no tests to run]` here means the file was not written. |
| 27 | `internal/lint/mixed_cycle.go`: `MixedCycleLint`, `Name() == "mixed-cycle"`, explicit `SeverityError`. Its own iterative typed-edge DFS (do **not** modify `findEdgeCycles` — `cycle` and `governed-cycle` depend on its exact finding order). It reports a cycle **only when the cycle's hops include ≥1 `rests_on` and ≥1 `governed_by`**, so it cannot co-fire on the `cycle` or `governed-cycle` coverage fixtures. Message: `mixed rests_on/governed_by cycle detected: a -(rests_on)-> b -(governed_by)-> a`. Registered via `init()`. **Step 26's file now goes green.** | `go build ./internal/lint/ && provetest ./internal/lint/ 'TestMixedCycle' -v` — all pass |
| 28 | Registry bookkeeping, all three places the repo says are load-bearing: `internal/lint/lint.go`'s package doc enumeration gains `mixed-cycle` and its count goes 27 → 28; `tests/lint_fixtures_test.go:113`'s `!= 27` becomes `!= 28`; new `testdata/fixture-coverage/lint/mixed-cycle/` with `project.config.yaml` (1 facet, 1 module) and exactly two draft claims — `A` with `rests_on: [B]` and `governed_by: {type: none, reason: …}`, `B` with `governed_by: {type: A}`. A has an outgoing `rests_on` and B an incoming one, so `orphan` stays quiet; keep both bodies free of edge-shaped prose so `body-edge-hint` stays quiet. No `coFiresWith` entry is needed. | `provetest ./tests/ 'TestLintRuleCoverageFixtures|TestEveryRegisteredLintHasACoverageFixture' -v` |
| 29 | **Commit gate 1.** | `go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go') && ls testdata/fixture-coverage/lint \| wc -l` → `28` |

#### L6b — the offline scan stops failing on comments

| # | Step | Proving command |
|---|---|---|
| 30 | `tests/portability_test.go`: add `stripJSComments(content string) string` — blanks `//` line comments and `/* … */` block comments, **preserving line count and line numbering** (replace comment bytes with spaces, keep newlines). String-literal aware for `'`, `"` and backtick, honouring backslash escapes, so a `//` inside a string does not start a comment. `scanForNetworkRefs` applies it **only when the label's extension is `.js`**; `.go`, `.html`, `.css` are untouched. | `provetest ./tests/ 'TestStripJSComments' -v` |
| 31 | `TestStripJSComments` table: a URL in a `//` comment → no offender; a URL in a `/* */` block → no offender; `fetch("https://evil.example/x")` → offender (the existing positive control's exact content); `const u = "https://evil.example/x";` → offender; `var s = "// not a comment https://evil.example/x";` → offender; a URL on the same line *after* code (`foo(); // https://x`) → no offender but the code half still scanned; offender line numbers unchanged by stripping. Then confirm the real walk still passes and the built-in positive control still fires. | `provetest ./tests/ 'TestStripJSComments|TestNoNetworkReferencesAnywhereInEngine' -v` |
| 32 | **Commit gate 2.** | `go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go')` |

#### L6c — the fixture-staleness test

| # | Step | Proving command |
|---|---|---|
| 33 | `tests/fixture_staleness_test.go`: for every `testdata/fixture-*/` carrying **both** a `project.config.yaml` and a committed `viewer/index.html` (**discovered**, never hardcoded), copy the fixture tree into `t.TempDir()`, run the built `dossierx check --config <copy>/project.config.yaml`, then compare `viewer/index.html` and `.catalog.json` against the committed originals. Normalize **two timestamp formats** before comparing, per design §13.3: RFC3339 (`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`) and `\d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC`. `.catalog.json` carries no timestamp [VERIFIED] and is compared raw. On mismatch, fail with the first differing line and the exact regeneration command. **CONDITIONAL** (§7.4). | `provetest ./tests/ 'TestCommittedFixtureViewersAreNotStale' -v` |
| 34 | Two controls, so the test cannot be vacuous: (a) assert at least one fixture was actually compared (a discovery bug that finds zero must fail, not pass); (b) inject a one-byte difference into the temp copy after rendering and assert the comparison reports it. | `provetest ./tests/ 'TestCommittedFixtureViewersAreNotStale' -v` |
| 35 | **Commit gate 3.** At this moment the committed fixtures are still current with the pre-graph renderer, so the test is green. It goes red the moment L2 changes rendered output and green again when L2 regenerates — that is the whole point. | `go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go') && git status --porcelain` |

---

### L4 — `graph-ui.js` + `graph.css`

The pane itself. DOM, canvas, controls, panels, refresh, hash writing.

**Brief for the implementing agent**

- Everything visual is frozen by the requirements prototype: the five-group
  control bar in the order of design §4.1, the canvas + right rail + legend
  layout, the node/edge encoding of design §4.
- **The offline scan of step 25 applies to both files too.** Comments in `.js`
  are exempt from L6b onward; executable code and string literals are not, and
  `.css` is not exempt at all.
- **Do not emit any of the literal strings `internal/render`'s existing tests
  count.** `comments_render_test.go` counts comment chips with attribute-precise
  selectors and comments explicitly that the shell's `<style>`/`<script>` must
  not add matches. `class="comment-chip"` and `data-claim=` must not appear in
  either file.
- The pane's trigger is bound by **one delegated listener on `document`** matching
  `[data-dxg-open]`, never a listener on the button — the button is destroyed and
  re-created by every SSE fragment swap.
- The trigger must **not** carry class `sec-tab`; the existing delegated handler
  matches `.sec-tab` and would also switch modules.
- The pane writes its hash segment via `history.replaceState` only. Never assign
  to `location.hash` except from the detail panel's "open claim" action.
- **Inert until first opened, and the payload is parsed at first open, not at
  parse time.** That is a contract, not an optimisation: step 70 replaces the
  payload block's `textContent` before opening the pane, and design §13.4 depends
  on it.
- All chrome colour comes from `var(--token)` over `style.css`'s fixed 14-token
  allowlist. The only literal colours permitted are the `--dxg-*` categorical
  block, which carries its own justifying comment.
- Reuse the existing `860px` breakpoint and the `pointer: coarse` convention. Do
  not invent a second breakpoint.

| # | Step | Proving command |
|---|---|---|
| 36 | `graph.css`: the `--dxg-*` token block with light and dark values — **20 facet slots** `--dxg-facet-1..20` (slots 1–5 the prototype's frozen values of design §4.2; 6–20 continue the ramp, rotating hue and alternating one lighter/one darker step so adjacent slots never differ only in lightness), `--dxg-facet-other`, cycle red, halo amber, and `--dxg-governed` — **a reserved hue outside the facet ramp**. Doc comment carries the justification of design §4.2 and §14, including the ~12-colour ceiling and why identity is also carried by legend and detail panel. | `grep -nE '#[0-9a-fA-F]{3,8}' internal/render/viewer/template/graph.css` — every hit inside the documented palette block; `grep -c 'dxg-facet-' …/graph.css` ≥ 42 (21 slots × light+dark) |
| 37 | `graph.css`: pane chrome — backdrop `z-index: 80`, pane `81`, header row, control bar, canvas holder, rail, legend, the 860px stacked layout, `pointer: coarse` targets. All colours via `var(…)`. | `grep -c 'var(--' internal/render/viewer/template/graph.css` (non-zero); proof of appearance is step 73 |
| 38 | `graph.css`: legend strip — one row per facet with swatch + **real project facet name**, a `governed_by` row using `--dxg-governed` with the curved/double-chevron sample, and the hover-dims-non-members rule. Design §4.2 channel 2. | `grep -n 'dxg-legend' …/graph.css`; executed by step 73 |
| 39 | `graph-ui.js` skeleton: IIFE, one delegated `document` click listener on `[data-dxg-open]`, lazy first-open mount, hash check at parse time. **No `JSON.parse` of the payload before first open.** | `node --check …/graph-ui.js` **(CONDITIONAL)** and `grep -n 'JSON.parse' …/graph-ui.js` — every hit inside the first-open path; executed by step 73 |
| 40 | Pane DOM + the five-group control bar of design §4.1, built once on first open. | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 41 | Pane header: renders `payload.generated_at` as a relative phrase with the absolute value on `title` (design §6.1). Rebuilt on every open. | `node --check` **(CONDITIONAL)**; executed by step 71 |
| 42 | Refresh button: created **only** when `document.body.classList.contains('comments-live')` at header-build time; not created at all otherwise (design §6.3). Click → `fetch('/api/graph')` → `applyPayload`. A failed fetch leaves the graph untouched and shows an inline notice. | `node --check` **(CONDITIONAL)**; executed by step 74 |
| 43 | `applyPayload(next)` preserving camera (zoom + pan), all control state, the expanded-group set, and node positions by id; unknown new ids seeded near their group centroid; selection preserved if its id survives; header timestamp updated. Design §6.4. | `node --check` **(CONDITIONAL)**; executed by step 74 |
| 44 | Canvas renderer: fill by `facetSlot`, radius by scope-relative degree, ring by status, halo for `review_pending` / open comments, ghost nodes. Palette read via `getComputedStyle(document.documentElement).getPropertyValue`. | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 45 | `governed_by`'s four channels (design §4.3): `--dxg-governed` stroke, quadratic-curve routing where `rests_on`/`mirrors` are straight, double-chevron arrowhead, wedge marker drawn on every id in `governors(edges)`. | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 46 | The governance overlay: dims every node and edge outside `governanceScope(edges)`. Listed in the overlay select between "dependency cycles" and "review pending". | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 47 | Force layout + interaction: node drag, background pan, scroll zoom, double-click to expand a group. | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 48 | Gaps rail: facts block above a visually separated hints block, hint items labelled as guesses, jump-to-node on click, live count. Keys off `graph-core.js`'s stable rule ids, never display text. | `node --check` **(CONDITIONAL)**; executed by step 70 |
| 49 | Detail panel: id, **facet by name**, status, build role, scope-relative degree, project-wide degree, governor and governed lists, cycle membership, and an explicit "open claim" link. Design §4.2 channel 3. | `node --check` **(CONDITIONAL)**; executed by step 73 |
| 50 | Auto-collapse: `AUTO_COLLAPSE_ABOVE = 300` (measured; was 600) as one named constant, the notice naming the real numbers, and a manual override that warns rather than blocks. | `grep -n 'AUTO_COLLAPSE_ABOVE' internal/render/viewer/template/graph-ui.js` |
| 51 | Hash state: read on open, write via `history.replaceState`, `hashchange` listener for externally pasted URLs only. Design §9. | `node --check` **(CONDITIONAL)**; executed by step 75 |
| 52 | Lane gate. | `provetest ./tests/ 'TestNoNetworkReferencesAnywhereInEngine' -v && go build ./... && go test ./... && git status --porcelain` |

---

### L2 — render wiring, the serve endpoint, and fixture regeneration

Turns the three inert files on, inlines the payload, adds `GET /api/graph`, and
regenerates the three tracked viewers. This is the lane where a silent failure is
most likely, so **the first proving command asserts on rendered output** and
every assertion after it does too.

**Brief for the implementing agent**

- **Type at every injection site, or the pane never initializes.** `GraphCSS` is
  `template.CSS`; `GraphPayload`, `GraphCoreJS`, `GraphUIJS` are `template.JS`. A
  plain `string` is contextually escaped into a quoted JS string literal with **no
  error at build, render or test time**. Never assert on the Go value; assert that
  the rendered document contains a distinctive byte sequence from the source file.
- **No `OverrideFile` branch for the three new files.** They are read straight
  from `shellFS` with a plain `ReadFile` + error wrap. Add a sentence to
  `render.go`'s package doc comment saying the override mechanism covers the shell
  and CSS only.
- **`graph.css`'s `<style>` block goes FIRST**, before `{{.CSS}}`. Placed last or
  in the middle it breaks `TestRender_ThemeCSSInjectedAfterBaseCSS`, which uses
  `strings.LastIndex`. Placed first, that test and
  `TestRender_NoThemeConfiguredEmitsEmptyThemeStyleBlock` both stay green
  **unedited** — and they must stay unedited.
- Do not touch `internal/render/render_test.go`.
- `graph.Build` takes **two** arguments. Do not call `buildImplinkLookup` from
  `graph_view.go`.

| # | Step | Proving command |
|---|---|---|
| 53 | **Written first, red.** `internal/render/graph_render_test.go`, asserting only on rendered output: (a) a distinctive byte sequence from each of `graph-core.js`, `graph-ui.js`, `graph.css` appears verbatim in the document — proving `template.JS`/`template.CSS` typing took effect; (b) the payload block's contents parse as JSON; (c) a claim whose id yields a label containing `</script>` and whose facet is `</script><img src=x>` produces a payload block with no literal `</script>` before its own closing tag; (d) block order: `graph.css` `<style>` before `{{.CSS}}`, payload before core before ui. | `provetest ./internal/render/ 'TestGraph' -v` → **FAIL**, naming the missing block. That failure IS this step's proof: the assertions are live and are made against rendered output. |
| 54 | Refactor `buildShellData` to take a single `shellInputs` struct instead of six positional arguments (it would otherwise become ten). Pure refactor, no behaviour change. | `go test ./internal/render/... -count=1` — every existing test passes unchanged |
| 55 | Extend the `go:embed` directive with the three paths; add `graphCoreFileName` / `graphUIFileName` / `graphCSSFileName` and their `*TemplatePath` consts following the existing two-const-block pattern; add three `[]byte` fields to `loadedTemplates` and three `shellFS.ReadFile` + error-wrap reads to `loadTemplates`. No override branch. | `go build ./... && go test ./internal/render/... -count=1` |
| 56 | Add `GraphCSS template.CSS` and `GraphPayload` / `GraphCoreJS` / `GraphUIJS template.JS` to `shellData` and to `buildShellData`'s return literal. | `go build ./... && go test ./internal/render/... -count=1` |
| 57 | New `internal/render/graph_view.go`: `graphPayloadJSON(cat, cfg, generatedAt) (template.JS, error)` → `graph.Build(cat, cfg)`, stamp `GeneratedAt` from `generatedAt` in RFC3339 UTC, `graph.Encode`. Doc comment names it as **the cache seam** of design §11 and states that `GET /api/graph` deliberately does not use it. Call it from `Render` between `renderClaims` and `buildShellData`, threading the existing `generatedAt`. | `go build ./... && go test ./internal/render/... -count=1` — the payload block does not exist in `shell.html` until step 58, so no `-run` pattern can honestly pass here; §6 assigns this step's behavioural proof to steps 53/59. |
| 58 | `shell.html`: `<style>{{.GraphCSS}}</style>` as the **first** style block; `<script type="application/json" id="dossierx-graph">{{.GraphPayload}}</script>` then `<script>{{.GraphCoreJS}}</script>` then `<script>{{.GraphUIJS}}</script>` after the existing inline script, before `</body>`. | `provetest ./internal/render/ 'TestRender_ThemeCSSInjectedAfterBaseCSS|TestRender_NoThemeConfiguredEmitsEmptyThemeStyleBlock|TestGraphBlockOrder' -v` |
| 59 | `shell.html`: the pane root `<section id="dxgPane" hidden>` as a sibling of `#statusStrip`, **outside `div.layout`**; the nav trigger button carrying `data-dxg-open`, **without** class `sec-tab`, above the `{{range .ModuleGroups}}` block inside `<nav id="nav">`. **Step 53's file now goes green.** | `provetest ./internal/render/ 'TestGraph|TestGraphPaneMountsOutsideLayout|TestGraphTriggerIsNotASecTab' -v` — all pass |
| 60 | `shell.html`: the three-line hash change — a `hashGraphSuffix()` helper, `hashId()` truncating at the first `!`, and the two replacement-hash builders appending the preserved suffix. Design §9. | `go test ./internal/render/... ./tests/... -count=1` |
| 61 | `style.css`: extend the z-index ledger comment near the status-strip block to record the graph pane's 80/81 band. Comment only — no rule changes. | `go test ./internal/render/... -count=1 && grep -n 'z-index' internal/render/viewer/template/style.css` |
| 62 | **Written first, red.** `internal/serve/graph_handler_test.go`: the endpoint returns 200 + the JSON content type; its body parses; its bytes are **byte-identical to the inline block's** after replacing both `generated_at` values with a fixed token; a HEAD/GET leaves the project directory unmodified (no `viewer/index.html`, no `.catalog.json` written). | `provetest ./internal/serve/ 'TestGraphEndpoint' -v` → **FAIL** (missing route). That failure IS this step's proof — a `PASS … [no tests to run]` here means the file was not written. |
| 63 | `internal/serve/handlers.go`: `handleGraph` — load claims, `catalog.Build(disarmUngatedMockups(claims, s.cfg), s.cfg)`, `graph.Build`, stamp `time.Now().UTC()`, `graph.Encode`, write the bytes with `Content-Type: application/json; charset=utf-8` and `Cache-Control: no-store`. Not via `writeJSON` — one encoder for this payload. Writes nothing to disk. `internal/serve/server.go`: one `mux.HandleFunc("GET /api/graph", s.handleGraph)` line in `routes()`. **Step 62's file now goes green.** | `provetest ./internal/serve/ 'TestGraphEndpoint|TestGraphEndpointMatchesInlinePayload' -v` — all pass |
| 64 | Forbidden-call sweep and the offline scan over the now-embedded client files. | `! grep -rn "SetEscapeHTML" internal/ cmd/ tests/ && provetest ./tests/ 'TestNoNetworkReferencesAnywhereInEngine' -v` |
| 65 | `docs/RELEASING.md`: the checklist item currently reads "The two committed sample viewers are regenerated" and names two commands. Make it three, adding `testdata/fixture-graph-demo`. Update the "the only expected changes are the generation timestamp (line 1, and the sidebar-footer 'Generated …' string)" parenthetical to name the third occurrence — the graph payload's `generated_at` — and add one sentence pointing at `TestCommittedFixtureViewersAreNotStale` as the gate that now enforces this item. | `grep -c 'cmd/dossierx check --config testdata/fixture' docs/RELEASING.md` → `3` and `grep -n 'generated_at' docs/RELEASING.md` (non-empty) |
| 66 | Regenerate all three fixtures, in the order `docs/RELEASING.md` now lists. | `go run ./cmd/dossierx --format text check --config testdata/fixture-basic/project.config.yaml && go run ./cmd/dossierx --format text check --config testdata/fixture-portability/project.config.yaml && go run ./cmd/dossierx --format text check --config testdata/fixture-graph-demo/project.config.yaml` — each prints `check: OK` |
| 67 | Verify the diff's shape rather than trusting it. Note the counting hazard: a bare `grep -c '<style>'` returns 4 on today's fixture because two matches are the literal text `<style>` inside CSS comments [VERIFIED]. Count indented tags only. Expect roughly a doubling in size (Phase 0's P2 probe measured 108,577 → 218,214 bytes for a 400-node payload). Anything outside the graph blocks and the timestamps is a regression, not drift. | `grep -c '^  <script' testdata/fixture-*/viewer/index.html` → `4` each; `grep -c '^  <style>' testdata/fixture-*/viewer/index.html` → `3` each; `git diff --stat` |
| 68 | Lane gate, with the staleness test now the thing that proves step 66 actually happened. | `go build ./... && go vet ./... && go test ./... && gofmt -l $(git ls-files '*.go') && provetest ./tests/ 'TestCommittedFixtureViewersAreNotStale' -v && git status --porcelain` |

---

### L5 — `viewer-tests`

The only place JS behaviour is proven. Three new files; no existing
`viewer-tests` file is touched.

**Brief for the implementing agent**

- **The test shape is load-bearing, not stylistic. Table-driven over ONE page
  load.** One Go test func = one page load = N table cases. A case in that shape
  costs ~0.00s; the naive one-func-per-case shape costs ~1.0s each and a 100-case
  suite becomes ~100s [VERIFIED by Phase 0's P3 timing decomposition]. Write it
  the first way.
- **60s hard ceiling per Go test func** — that is `browserContext`'s per-tab
  timeout. If a table grows large, split across two or three test funcs, each
  paying ~1.0s, rather than letting one approach the ceiling.
- **Exported client functions take and return plain JSON-able values only** — no
  `Map`, no `Set`, no cyclic objects. Inputs cross as `json.Marshal` +
  `fmt.Sprintf` into the expression string; outputs cross as CDP
  `returnByValue`. Assert into plain Go types (`map[string]int`, `[][]string`,
  structs).
- Reuse `newProject`, `newProjectRaw`, `renderStatic`, `browserContext`,
  `evalBool`, `evalInt`, `runCDP`, `pollTrue`, `newLiveTab` unchanged. **Budget
  zero harness work.** Do not add a JS toolchain, a bundler, or a node runner.
- Static `file://` via `p.renderStatic()` is enough for everything except the
  fragment-swap and refresh tests, which need `p.ensureServe()` / `newLiveTab`.
- **Do not build a cycle-carrying corpus.** `p.renderStatic()` calls
  `dossierx check`, which returns above the render stage on any error-severity
  finding, so a corpus with a cycle produces no `viewer/index.html` at all and the
  harness `t.Fatal`s. Inject the payload instead — step 70.
- **Do not read `testdata/fixture-graph-demo`.** These tests build their own
  corpora through the harness. The demo fixture is a human-facing artifact, not a
  test input, and reading it would couple this lane to L1's file set.

| # | Step | Proving command |
|---|---|---|
| 69 | `graph_core_test.go`: **one** page load, then table-driven subtests over `representatives`, `aggregateEdges`, `degrees`, `scc` (mixed-type cycle, self-edge, long chain, disjoint components), `selfEdges`, `facetSlot` (including the 20-wrap and the `-1` cases), `governors`, `governanceScope`, `gapRules` facts, the two hints, and `encodeState`/`decodeState` round-trip. Include a negative control that fails when the expectation is wrong, and delete it once seen to fail. | `cd viewer-tests && go test -count=1 -run TestGraphCore -v ./...` — total elapsed for the func well under 60s |
| 70 | **The cycle proof (design §13.4).** Static page; before any click, replace `#dossierx-graph`'s `textContent` with a payload whose edges form (a) a `rests_on` cycle, (b) a mixed `rests_on`/`governed_by` cycle, and (c) a self-edge; then click `[data-dxg-open]` and assert the gaps rail lists rule id `cycle` with exactly the two expected member sets and rule id `self_edge` with the third, keyed off rule ids not display text. | `cd viewer-tests && go test -count=1 -run TestGraphPaneRendersInjectedCycles -v ./...` |
| 71 | Payload sanity + freshness: the block exists, `JSON.parse` succeeds, `nodes.length` equals the project's claim count, and the pane header shows a timestamp matching the payload's `generated_at`. | `cd viewer-tests && go test -count=1 -run TestGraphPayloadParsesAndHeaderShowsTimestamp -v ./...` |
| 72 | Escaping, in a real browser: a claim whose facet contains `</script><img src=x>`; assert `JSON.parse` of the block succeeds, the injected string survives verbatim through `JSON.parse`, and `document.images.length` is unchanged. | `cd viewer-tests && go test -count=1 -run TestGraphPayloadSurvivesScriptClose -v ./...` |
| 73 | Inert until opened, then opens — and the structural DOM test for everything L4 draws. Before any click: no pane DOM children, no canvas, **no parsed payload**. After clicking `[data-dxg-open]`: the canvas exists; the control bar carries all five groups and the overlay select carries all six overlays including `governance`; the legend lists every one of the project's facets **by its real name**; selecting a node fills the detail panel and that panel names the node's facet in text; a corpus seeded above `AUTO_COLLAPSE_ABOVE` opens collapsed with the notice naming the real numbers. Escape / close returns to inert-but-mounted. Pixels are never asserted. | `cd viewer-tests && go test -count=1 -run TestGraphPaneInertUntilOpened -v ./...` |
| 74 | Refresh, both page kinds (design §6.2–6.4): on the **static** page the refresh control does not exist in the DOM at all after opening the pane; on a **live** tab it does. On the live tab, set zoom/pan and a non-default scope, click refresh, and assert the header timestamp advanced while zoom, pan and scope are unchanged. | `cd viewer-tests && go test -count=1 -run TestGraphRefresh -v ./...` |
| 75 | Hash: change a filter, assert `location.hash` carries the `!g=` segment **and** the reading view's module did not change; then paste a full deep-link hash and assert both halves apply. | `cd viewer-tests && go test -count=1 -run TestGraphHashDoesNotClobberReadingView -v ./...` |
| 76 | Fragment-swap survival, and design §1.1 made visible: live tab, open the pane, set a filter, write a claim file, wait for the SSE-driven swap (reuse `live_reload_test.go`'s wait pattern), assert the pane node is the **same** element, its filter state is intact, **and its header timestamp is unchanged** — the swap does not re-deliver the payload, and the pane says so. | `cd viewer-tests && go test -count=1 -run TestGraphPaneSurvivesFragmentSwap -v ./...` |
| 77 | `graph_parity_test.go`: compute the browser's `isolated` set unscoped with **only `rests_on` + `mirrors` enabled**, and assert it equals the `orphan` findings from `dossierx check --format json` on the same project, read off `data.lint_findings[].lint` [VERIFIED shape: `tests/lint_fixtures_test.go`'s `lintFinding` struct]. Doc comment states why `governed_by` is excluded. | `cd viewer-tests && go test -count=1 -run TestGraphIsolatedMatchesOrphanLintUnscoped -v ./...` |
| 78 | Lane gate. | `make viewer-test && git status --porcelain` |

---

## 5. Integration

Not a lane. Both steps need every lane landed.

| # | Step | Proving command |
|---|---|---|
| 79 | Whole-suite proof, both gates, with the race detector. | `go build ./... && go vet ./... && go test -race ./... && gofmt -l $(git ls-files '*.go') && make viewer-test` |
| 80 | Cross-lane invariant sweep, each one a property no single lane owns end-to-end. | `! grep -rn "SetEscapeHTML" internal/ cmd/ tests/ && ! grep -rn "dossierx/internal/implink" internal/graph/ && ! grep -rn "has_code_link" internal/ --include='*.go' | grep -v _test.go && ls testdata/fixture-coverage/lint \| wc -l` → `28` `&& provetest ./tests/ 'TestNoNetworkReferencesAnywhereInEngine|TestCommittedFixtureViewersAreNotStale|TestLintRuleCoverageFixtures' -v && git status --porcelain` (last must be empty) |

---

## 6. Traceability — what executes the code L3 and L4 write

No client function is written without a named later executor. If a row's
executor is missing when its lane gate runs, the lane is not done.

| Written in | Function / behaviour | First executed by |
|---|---|---|
| L3 step 16 | `scopeFilter`, `representatives` | step 69 |
| L3 step 17 | `aggregateEdges` | step 69 |
| L3 step 18 | `degrees` | step 69 |
| L3 step 19 | `scc`, `selfEdges` | steps 69, 70 |
| L3 step 20 | `facetSlot` | step 69 |
| L3 step 21 | `governors`, `governanceScope` | step 69 |
| L3 step 22 | eight fact rules | steps 69, 70, 77 |
| L3 step 23 | two hint rules | step 69 |
| L3 step 24 | `encodeState` / `decodeState` | steps 69, 75 |
| L4 steps 39–40 | delegated open, lazy mount, control bar | steps 73, 70 |
| L4 step 41 | header timestamp | steps 71, 76 |
| L4 steps 42–43 | refresh button gating, `applyPayload` | step 74 |
| L4 steps 44–45 | canvas encoding, governed_by's four channels | step 73 (existence and structure only; pixels are never asserted). The pure inputs — `facetSlot`, `governors` — are executed by step 69. |
| L4 step 46 | governance overlay | step 73 (the overlay is selectable and dims); step 69 for `governanceScope`'s membership |
| L4 step 47 | force layout, drag/pan/zoom/expand | steps 73, 74 (expand and camera only; layout has no stable assertion) |
| L4 step 48 | gaps rail | step 70 |
| L4 step 49 | detail panel | step 73 |
| L4 step 50 | 300-node auto-collapse | step 73 (seeded above the threshold) |
| L4 step 51 | hash read/write | steps 75, 76 |
| L2 steps 55–57 | embed, typing, payload call | steps 53/59, 71 |
| L2 step 59 | pane mount point | step 76 |
| L2 step 60 | hash suffix preservation | step 75 |
| L2 steps 62–63 | `GET /api/graph` | step 74 |
| L6 step 27 | `mixed-cycle` lint | steps 26, 28 (Go, root suite) |
| L6 step 30 | `stripJSComments` | step 31 |
| L6 step 33 | staleness test | steps 34, 68 |

---

## 7. Conditional steps

Five steps rest on something Phase 0 did not verify. Each is written as a
conditional and each names its fallback.

1. **Step 10 — the `depguard` rule.** `golangci-lint` is not installed on the
   development machine [VERIFIED: `which golangci-lint` → not found]. The
   *property* is proven locally by grep; the *rule* is only executed by CI's lint
   job. **If** CI's lint job rejects the new rule block, drop the rule and keep
   the grep — the import direction is what matters, not its enforcement
   mechanism.
2. **Step 12 — locking and commenting the demo fixture.** Two behaviours are
   taken from doc comments, not from reading `internal/lock`: that `dossierx
   lock` on a brand-new project creates its lock store without a pre-ledger step
   (`preLedgerPrecondition` exists in `cmd/dossierx/main.go` and applies to
   projects whose locks *predate* the ledger, which a fixture created today is
   not), and that a comment opened on an already-locked claim sets
   `review_pending`. **If** either refuses, seed that state the next way down the
   list — for `review_pending`, a claim edited directly in YAML — and record in
   the fixture's config comment which state is hand-seeded and why.
3. **Steps 15–24 and 39–51 — the `node --check` / `node -e` proofs.** Conditional
   on `node` being on PATH [VERIFIED present here: `v24.13.0`]. These are
   developer-loop proofs only and are **never** a CI gate; if node is absent, the
   step's proof degrades to the `grep` named alongside it and the real executor is
   the L5 test in §6.
4. **Step 33 — the staleness test's path-independence.** Rests on a rendered
   `viewer/index.html` containing no bytes derived from its project's absolute
   path. Not verified in Phase 0. **If** the temp-dir render differs from the
   committed one in a path-shaped way, add a third normalizer replacing the
   fixture's absolute root with a fixed token, and say in the test's doc comment
   which bytes needed it and why — do not weaken the comparison to a substring
   check.
5. **Step 12's fixture legality against a doctrine gate.** The demo config sets
   no `doctrine_facet`, on the inference that an unset field means no doctrine
   gate applies to `governed_by` targets; `internal/lint`'s doctrine rule was not
   read. **If** `check` reports a doctrine finding, add the gate's required facet
   to the config and re-point the fixture's `governed_by` targets at claims in it.

---

## 8. Checkpoint commits

Eight commits. Each is made only after that lane's (or sub-lane's) gate command
passes, and **every one leaves both gates green** — which is why L2's fixture
regeneration is inside L2's commit rather than a trailing chore commit.

```
1.  feat(graph): add internal/graph and the claims-graph demo fixture

    graph.Build is a pure, deterministic, clock-free function over catalog
    and config, emitting the JSON payload the viewer's graph pane reads. It
    takes two arguments: the graph audits claims, not code, so there is no
    implink lookup, no has_code_link field and no ungrounded-claim rule
    anywhere in this package.

    Encode marshals with encoding/json's default HTML escaping, which is the
    only thing between an author-authored claim label and a </script>
    breakout in a <script type="application/json"> block. SetEscapeHTML(false)
    is forbidden here, the doc comment says so, and the lane's own gate greps
    the repository to prove it.

    testdata/fixture-graph-demo is a third tracked fixture: five modules,
    ~60 claims, every gap class the payload can express seeded and asserted
    present by a test. It contains no cycle of any shape, because check
    returns above the render stage on any error-severity lint finding.

2.  feat(viewer): add graph-core.js, the dependency-free graph core

    Pure computation, no DOM: scope filtering, the representative-node rule,
    edge aggregation, scope-relative degrees, iterative Tarjan SCC, facet
    slot assignment by index, the governor set, and the scope-relative gap
    rules with facts and heuristics kept apart. Exports one namespace whose
    members are a stated API — viewer-tests hangs its whole table-driven
    suite off them, so every one takes and returns plain JSON-able values.

    Not yet embedded; render wiring lands three commits later.

3.  feat(lint): detect cycles that alternate rests_on and governed_by

    A loop like "A rests_on B, B governed_by A" is invisible to both existing
    cycle rules: findEdgeCycles is one DFS with one edgesOf function, and
    cycle passes rests_on while governed-cycle passes governed_by. Neither
    takes the union, so the mixed loop has no back edge either walk can find.

    mixed-cycle walks the union graph carrying the edge kind on each hop and
    reports only cycles whose hops include both kinds — which is what keeps
    it from co-firing on the cycle and governed-cycle coverage fixtures, a
    property tests/lint_fixtures_test.go enforces per fixture.

    Error severity, matching cycle and governed-cycle. No migration document:
    a corpus containing this shape was always malformed, the engine simply
    could not see it.

4.  fix(tests): stop the offline scan failing a build over a comment

    TestNoNetworkReferencesAnywhereInEngine walks .js under internal/ and
    fails on anything URL-shaped, which meant a doc comment citing the paper
    an algorithm came from failed the build — the wrong incentive for two new
    client files whose non-obvious algorithms deserve citation.

    The scan now blanks // and /* */ comments in .js before matching, keeping
    line numbers exact. It is string-literal aware, so a URL in executable
    code — including one inside a string that merely looks like a comment —
    still fails, and the existing positive control still fires unchanged.

5.  test: fail the build when a committed sample viewer goes stale

    The tracked viewer/index.html fixtures are generated artifacts no test
    reads, so a rendering or CSS change ships without them. That happened in
    v0.3.1 and v0.4.1, caught by review both times, never by CI.

    This regenerates every discovered fixture into a temp dir, normalizes the
    two timestamp formats docs/RELEASING.md names as the only expected drift,
    and diffs the rest. The fixture list is discovered rather than hardcoded,
    so the test cannot itself go stale when a fourth fixture appears.

6.  feat(viewer): add graph-ui.js and graph.css, the claims graph pane

    Canvas pane, five-group control bar, gaps rail with facts above labelled
    heuristics, detail panel, 300-node auto-collapse, and filter state in the
    URL hash. Chrome colour comes from style.css's fixed theme token
    allowlist; the categorical palette is the file's one documented
    literal-colour block, because that allowlist has no categorical slots.

    The facet ramp is 20 slots assigned by index, never by name — real
    projects name their own facets. Twenty distinguishable colours do not
    exist, so facet identity is also carried by the legend, which lists real
    facet names and dims non-members on hover, and by the detail panel, which
    names the facet in text.

    governed_by gets four channels rather than a dash pattern: a reserved hue
    outside the facet ramp, curved routing where the other two are straight, a
    double-chevron head, and a wedge on every node that governs another —
    plus an overlay that dims everything else, because an edge style alone
    still makes a reader trace lines.

    The pane binds one delegated listener on document, never on its trigger
    button: the trigger lives inside the subtree an SSE fragment swap
    replaces. It parses the payload at first open, not at parse time.

7.  feat(render): embed the graph client files, inline the payload, serve it

    go:embed gains three paths, loadTemplates three reads, shellData four
    typed fields — template.CSS for the stylesheet, template.JS for the two
    scripts and the payload. The typing is load-bearing and fails silently: a
    plain string is escaped into a quoted JS literal with no error at build,
    render or test time, so the first test in this commit was written before
    the wiring and asserts on rendered output, as does every one after it.

    graph.css is the FIRST <style> block so the two existing style-ordering
    tests keep passing untouched. The pane mounts outside div.layout, beside
    #statusStrip, so it survives a fragment swap with its state intact.
    buildShellData now takes a struct; ten positional arguments was not an
    option.

    A fragment swap does not re-deliver the payload, so the pane states the
    payload's generation time and, behind a live serve only, offers a refresh
    button backed by the new GET /api/graph. The button is absent rather than
    disabled in a static file:// viewer, because a control a document cannot
    honour is a promise, not an affordance.

    The three sample viewers are regenerated in this commit rather than a
    trailing chore: the staleness test added earlier in this branch makes a
    render change that leaves them behind a red build, which is the point.

8.  test(viewer): cover the claims graph in the chromedp suite

    One page load, table-driven cases: SCC, representative mapping, edge
    aggregation, degrees, facet slots, governors, gap rules, hash round-trip.
    That shape costs ~0.00s per case where a test-func-per-case costs ~1.0s,
    and it keeps every func well under browserContext's 60s ceiling.

    Cycle rendering is proven by injecting a cycle-carrying payload into the
    page before the pane opens, not by a fixture: with mixed-cycle now at
    error severity, no corpus that renders at all can contain one.

    Also pins four things nothing else can see: that a </script> in a facet
    name reaches the browser as data, that the refresh control is absent on
    file:// and present under serve, that a fragment swap leaves the pane and
    its stated timestamp alone, and that the unscoped isolated set equals
    lint's orphan findings when governed_by is excluded.
```

---

## 9. What this plan deliberately does not do

- **No branch is created and nothing is committed by the planning phase.**
- **No code-grounding signal anywhere.** `has_code_link`, the `locked_ungrounded`
  rule, the "locked, ungrounded" overlay and `Build`'s `implinks` argument are
  removed, not deferred. Step 80 greps for the field name across `internal/` as a
  standing check that it did not come back.
- **`site/src/content.ts`'s release entry and the version bump are release
  housekeeping**, performed by `docs/RELEASING.md`'s own checklist at release
  time, not by this plan.
- **No new CLI noun and no new schema field.** The graph rides in render output;
  `model.Claim` is untouched.
