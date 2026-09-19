# fixture-theme-flat — the shared viewer-regression corpus

This fixture supplies reader structures that the small baseline fixture does
not put on one page. It remains named `fixture-theme-flat` for stable test and
release-history paths; it has no project-specific viewer styling.

## What this corpus instantiates

Every row below is present in the committed `build/viewer/index.html`.

| construct | selector | where it comes from |
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
| claim source row | `.claim-source`, `.claim-source:target` | `widget.contract.overview` carries one external `sources:` entry, cited from its body as `[1]`; the parity probe deep-links to `#widget.contract.overview-source-1` so the `:target` rule fires |
| graph pane | `--dxg-*` palette via the graph pane's own palette read | the claims graph, opened by the parity test |

## Coverage boundaries

State it here rather than let a silent absence read as a pass.

- **No `layout: tree` claim**, so `.claim-tree-body` has no element in this
  corpus.
- **No cycle of any kind** — the same bound `fixture-graph-demo` documents.
  `check` returns above the render stage on the first error-severity finding
  and every cycle shape is error severity, so a fixture that must render
  cannot carry one.
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
