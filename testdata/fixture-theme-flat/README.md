# fixture-theme-flat — the custom-theme parity corpus

This fixture exists for one test: `viewer-tests/theme_parity_test.go`, which
proves that turning ~14 hard-coded literals in `style.css` into
`var(--token, <the same literal>)` reads changed **nothing** a reader sees in a
project that sets none of those tokens.

That proof is a computed-style comparison in a real browser, and a computed
style can only be read off an element that is actually on the page. So this
corpus's job is to put every themed construct on one page at once. What is not
here is not compared — which is why the inventory below is explicit, and why
the second half of it (what this fixture does NOT instantiate) is the part
worth reading.

`viewer.theme` in `project.config.yaml` is copied verbatim from a real client
project: the **flat, mode-less** shape every project written before per-mode
values existed uses. It is the shape that must merge to exactly the single
`:root{…}` block the engine emitted before `light:`/`dark:` were added.

## What this corpus instantiates

Every row below is present in the committed `build/viewer/index.html`.

| construct | selector the parity probe reads | where it comes from |
|---|---|---|
| inline code span | `code` | `widget.contract.overview` body |
| fenced code block | `.claim-body pre`, `.claim-body pre code` | `widget.contract.overview` body |
| pipe table (claim body) | `.claim-body .md-table th` | `widget.contract.overview` body |
| pipe table (steps body) | `.sbody .md-table th` | `widget.contract.walkthrough` step 2 |
| markdown image | `.sbody .md-img` | `widget.contract.walkthrough` step 1, `claims/assets/flow.svg` |
| numbered step bubble | `.snum`, `.sbody` | `widget.contract.walkthrough` |
| draft pill | `.pill.pv`, `.status-draft` | the four draft `widget.*` claims |
| locked / warn pills | `.pill.ps`, `.pill.pw` | the six locked `panel.*` / `widget.contract.reviewed` claims |
| enum column marker | `.en` (with `.key`, `.ty`, `.ex`) | `widget.internals.fields`, a `layout: table` claim with `field`/`type`/`enum`/`example` columns |
| hard-boundary banner | `.claim-banner` | `widget.decision.boundary` (`layout: banner`, `emphasis: true`) |
| review-pending marker | `.claim-review-pending` | `widget.contract.reviewed`: locked **and** carrying one open comment thread |
| comment chip / composer | `.comment-chip`, `.comment-composer-input`, `.comment-composer-submit` | the open thread above |
| comments chrome | `.comments-panel`, `.comments-rail`, `.comments-toast`, `.comments-overlay` | the viewer shell (always present, inert until opened) |
| track head | `.track-head` | the `checkout` track, owned by `widget.contract.overview` and cited by two more |
| build-order tab | `.bo-phase__head` | `build/build-order/panel.json`: the `panel` module locked end to end across all five phases |
| project mockup | `.mockup-diagram`, `.gcp-console` | `panel.decision.mockup` — `layout: mockup`, locked, `raw_html_reviewed: true`, module in `mockup_modules` |
| system record head | `.system-record-head` | every module's record head |
| sidebar chrome | `.sidebar`, `.logo`, `.sec-tab`, `.facet-toc`, `.facet-toc__item`, `.facet-toc__select`, `.nav-toggle`, `.nav-overlay`, `.system-nav-group__toggle`, `.system-nav-group__count` | the viewer shell |
| claim source anchor | `.claim-source` | the `sources`-free claims still emit the anchor the `:target` rule paints |
| graph pane | `--dxg-*` palette via the graph pane's own palette read | the claims graph, opened by the parity test |

## What this corpus deliberately does NOT instantiate

State it here rather than let a silent absence read as a pass.

- **No `layout: tree` claim**, so `.claim-tree-body` is present in the shipped
  stylesheet and the shipped JS but has **no element on this page**. The
  parity probe reaches it through its synthetic subtree, not through this
  corpus.
- **No cycle of any kind** — the same bound `fixture-graph-demo` documents.
  `check` returns above the render stage on the first error-severity finding
  and every cycle shape is error severity, so a fixture that must render
  cannot carry one.
- **No `sources:` block**, so no real footnote reference is rendered; only the
  `.claim-source` anchor the `:target` rule paints.
- **No `light:`/`dark:` sub-mapping and no `fonts:`**. That is the point of
  this fixture — it is the flat, mode-less case. `fixture-theme-preset` is
  where a preset, a two-mode override and a data:-inlined font face live.
- **No `template_overrides`**, so the theme rules that are skipped under an
  override sheet are not exercised here.
- **One standing lint warning**, on purpose: `comments-unresolved` on
  `widget.contract.reviewed`. An open thread on a locked claim is the only
  thing that sets `review_pending`, and `review_pending` is what renders
  `.claim-review-pending`. Removing the warning would remove the construct.
  There are **no error-severity findings**: `dossierx check` exits 0.

## Regenerating

```
dossierx --config testdata/fixture-theme-flat/project.config.yaml check
```

`build/viewer/index.html` and `build/catalog/catalog.json` are tracked
generated artifacts; `tests/fixture_staleness_test.go` fails if they drift from
what the current engine writes. `build/ledger/comment-digest.json` is a tracked
FIXTURE INPUT, not an output: this project is ledger-covered, `check` never
writes a digest for a ledger-covered project, and without a committed digest a
fresh checkout reports `comment-digest-absent` and exits 1. It was written once
by re-recording the five threadless `panel.*` approvals (unlock, then lock, one
at a time) and re-entering `widget.contract.reviewed`'s thread through
`dossierx comment add`, so the thread's id and `created` stamp are the engine's.
