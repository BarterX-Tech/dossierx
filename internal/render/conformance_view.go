package render

import (
	"github.com/BarterX-Tech/dossierx/internal/conformance"
)

// conformanceCSS is the whole stylesheet for the "Implementation checks"
// panel and its nested "How this was checked" disclosure
// (docs/design/screens/06a-how-this-was-checked-inline-disclosure.md,
// docs/design/screens/07-claim-boundary-no-embodiment.md §4.12,
// docs/design/reference-rules.md R09.1-R09.3/R12.1-R12.4/R08.1).
//
// This lives in Go, not style.css (`grep -c conformance style.css` is 0 —
// 06a §7.2), because the panel is only appended to the viewer's CSS when
// the catalogue actually carries conformance results (viewerCSSWithConformance
// below).
//
// Token discipline (tokens.md §1, G2, G7): every colour below is a token or
// a color-mix() of tokens, never a literal hex, and light/dark both fall out
// of the same declaration because --link/--accent/--warn/--status-draft are
// themselves re-pointed per mode by the palette blocks this lane does not
// own. --card-gutter (G8) is read, never re-literalized, for every
// horizontal inset so the panel's 32px/16px edges track the card's own
// gutter instead of hard-coding a breakpoint. Per G2, the disclosure
// trigger/chevron/copy-row use --link (Paper's navy "navigation only"
// --color-accent), never --accent (Paper's green --color-locked, which is
// reserved for the Matched verdict) — the inversion G2 warns against.
const conformanceCSS = `
/* Added only when this viewer contains structured conformance results. */
.claim-conformance{margin:.65rem 0 0;padding:13px 14px;border:1px solid var(--border);border-radius:var(--radius);background:var(--hover-bg,transparent);font-size:.78rem;font-family:var(--font-sans)}
.claim-conformance[data-implementation-ready="true"]{border-color:color-mix(in srgb,var(--accent) 40%,var(--border))}
.claim-conformance[data-implementation-ready="false"]{border-color:color-mix(in srgb,var(--warn) 40%,var(--border))}
.claim-conformance-head,.claim-conformance-check-head,.claim-conformance-line,.claim-conformance-scope{margin:0}
.claim-conformance-head{display:flex;align-items:center;justify-content:space-between;gap:.5rem;color:var(--ink);cursor:pointer;list-style:none;font-family:var(--font-sans);font-size:13px}
.claim-conformance-head::-webkit-details-marker{display:none}
.claim-conformance-scope{margin-top:.35rem;color:var(--muted);font-size:11.5px}

/* Panel-body eyebrow (06a §4.3 "IMPLEMENTATION CHECKS … 1") — the section
   label + hairline + check count shown once, above every check article,
   when the outer <details> is open. */
.claim-conformance-eyebrow{display:flex;align-items:center;gap:8px;margin-top:.6rem;padding-top:10px;border-top:1px solid var(--border);font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--faint)}
.claim-conformance-eyebrow-count{font-family:var(--font-mono);font-weight:400;letter-spacing:0}

/* One check (06a §4.3-§4.8). The panel ground follows the verdict
   (06a §9 Open decision 1-2): a warm/blocked wash for Mismatch/Uncheckable,
   a draft wash for Owed, the neutral code ground for Matched — derived with
   color-mix() rather than the six raw hexes the Paper boards paint (06a
   Paper defects 1-3), so both colour modes fall out of one declaration. */
.claim-conformance-check{margin-top:10px;padding:18px var(--card-gutter, 32px) 24px;border-top:1px solid var(--border);background:var(--code-bg)}
.claim-conformance-check:first-of-type{margin-top:16px}
.claim-conformance-check[data-conformance-state="mismatch"],
.claim-conformance-check[data-conformance-state="uncheckable"]{background:color-mix(in srgb,var(--warn) 6%,var(--card-bg));border-top-color:color-mix(in srgb,var(--warn) 22%,var(--border))}
.claim-conformance-check[data-conformance-state="owed"]{background:color-mix(in srgb,var(--status-draft) 6%,var(--card-bg));border-top-color:color-mix(in srgb,var(--status-draft) 22%,var(--border))}

.claim-conformance-check-head{display:flex;align-items:baseline;justify-content:space-between;gap:14px;color:var(--ink);font-family:var(--font-sans);font-size:13px}
/* flex:1 1 auto + min-width:0 (06a §8.14: "the title wraps and the verdict
   keeps its lane") — without min-width:0 a flex item's default min-width is
   its content's shrink-to-fit size, so a long check id would push the
   verdict out rather than wrapping (the same failure writeConformanceMembers'
   doc comment fixes for multi-value machine-layer rows). */
.claim-conformance-check-title{flex:1 1 auto;min-width:0;font-size:16px;line-height:20px;font-weight:600;color:var(--ink);overflow-wrap:anywhere}

/* Verdict — dot + label, one hue per state (R08.1 — no third hue: Mismatch
   and Uncheckable share --warn, only the word differs). */
.claim-conformance-verdict{display:inline-flex;align-items:center;gap:6px;flex-shrink:0;font-size:13px;line-height:16px;font-weight:600;color:var(--muted)}
.claim-conformance-verdict-dot{width:7px;height:7px;border-radius:999px;background:currentColor}
.claim-conformance-check[data-conformance-state="matched"] .claim-conformance-verdict{color:var(--accent)}
.claim-conformance-check[data-conformance-state="mismatch"] .claim-conformance-verdict,
.claim-conformance-check[data-conformance-state="uncheckable"] .claim-conformance-verdict{color:var(--warn)}
.claim-conformance-check[data-conformance-state="owed"] .claim-conformance-verdict{color:var(--status-draft)}

/* The reviewer-facing explanation sentence — serif, above the line
   (06a §2, §4.3; tokens.md §7's family rule). */
.claim-conformance-explain{margin:8px 0 0;max-width:700px;font-family:var(--font-serif);font-size:15px;line-height:23px;font-weight:400;color:var(--muted)}

/* EXAMINED / COMPARED / FOUND (06a §4.4-§4.5, §6) — a fixed label column,
   unwound to a stacked block at 520px (M5). */
.claim-conformance-row{display:flex;align-items:baseline;gap:20px;padding-block:10px;border-top:1px solid color-mix(in srgb,var(--warn) 18%,var(--border))}
.claim-conformance-check[data-conformance-state="matched"] .claim-conformance-row{border-top-color:var(--border)}
.claim-conformance-check[data-conformance-state="owed"] .claim-conformance-row{border-top-color:color-mix(in srgb,var(--status-draft) 18%,var(--border))}
.claim-conformance-row-label{width:92px;flex-shrink:0;font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:600;letter-spacing:.06em;color:var(--faint)}
.claim-conformance-row-value{font-family:var(--font-serif);font-size:14px;line-height:22px;font-weight:400;color:var(--muted);overflow-wrap:anywhere}
.claim-conformance-row--found{align-items:flex-start}
.claim-conformance-row--found>.claim-conformance-row-value{display:flex;flex-direction:column;gap:9px}

/* The coverage strip (R12.3, R-H.3, 06a §4.5) — a marked set, not a diff;
   left-to-right order is the declared members' own order and carries no
   claim of its own. Component-board sizing wins over 06a's own boards per
   R00.0 (06a Paper defect 12): 11/14 chip labels, not 12/16. */
.claim-conformance-coverage{display:flex;flex-wrap:wrap;gap:5px}
.claim-conformance-chip{display:inline-flex;align-items:center;justify-content:center;min-width:32px;height:29px;padding-inline:6px;border-radius:5px;border:1px solid var(--border);background:var(--card-bg);font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:400;color:var(--muted);overflow-wrap:anywhere}
.claim-conformance-chip--missing{border:1.5px solid var(--warn);background:color-mix(in srgb,var(--warn) 12%,var(--card-bg));font-weight:600;color:var(--warn)}
.claim-conformance-legend{display:flex;align-items:center;gap:6px;flex-wrap:wrap}
.claim-conformance-legend-count{font-family:var(--font-serif);font-size:14px;line-height:18px;font-weight:400;color:var(--faint)}
.claim-conformance-legend-swatch{width:11px;height:11px;border-radius:3px;border:1.5px solid var(--warn);background:color-mix(in srgb,var(--warn) 12%,var(--card-bg));margin-left:8px}
.claim-conformance-legend-label{font-family:var(--font-sans);font-size:12px;line-height:16px;font-weight:400;color:var(--faint)}

/* The nested "How this was checked" disclosure (R12.1, R09.1, R09.3) — a
   second level of disclosure inside the checks expansion, closed by
   default, that never auto-opens even when the outer panel does. */
.claim-conformance-disclosure{margin-top:2px}
.claim-conformance-disclosure-trigger{display:flex;align-items:center;gap:7px;margin-inline:-8px;padding:12px 8px 0;border-radius:var(--radius);color:var(--link);cursor:pointer;list-style:none;font-family:var(--font-sans);font-size:13px;line-height:16px;font-weight:500}
.claim-conformance-disclosure-trigger::-webkit-details-marker{display:none}
.claim-conformance-disclosure[open]>.claim-conformance-disclosure-trigger{padding:10px 8px 14px;margin-top:10px;border-top:1px solid color-mix(in srgb,var(--warn) 14%,var(--border))}
.claim-conformance-chevron{flex-shrink:0;color:var(--link);transition:transform .12s ease}
.claim-conformance-disclosure[open] .claim-conformance-chevron{transform:rotate(-90deg)}

/* The machine layer (06a §4.8) — the engine's own nine-key envelope,
   verbatim, lowercase mono; --code-bg is the token 06a Paper defects 1/11
   flag as the light board's own unspelled ground. */
.claim-conformance-machine{padding:16px var(--card-gutter, 32px) 22px;background:var(--code-bg);border-top:1px solid var(--border)}
.claim-conformance-line{display:flex;align-items:baseline;gap:18px;padding-block:9px;overflow-wrap:anywhere;font-size:11.5px}
.claim-conformance-line+.claim-conformance-line{border-top:1px solid var(--border)}
.claim-conformance-line>span:not(.claim-conformance-values){width:84px;flex-shrink:0;font-family:var(--font-mono);font-size:11px;line-height:14px;font-weight:400;color:var(--faint)}
.claim-conformance-line>code,.claim-conformance-values{font-family:var(--font-mono);font-size:12px;line-height:19px;color:var(--ink);overflow-wrap:anywhere}
/* The multi-member value (missing/extra/expected/observed on a set-shaped
   check) is ONE flex item, not several — see writeConformanceMembers'
   doc comment. min-width:0 is required for a flex item to be allowed to
   wrap/shrink below its content's intrinsic width at all. */
.claim-conformance-values{flex:1 1 auto;min-width:0}

/* Copy as JSON (06a §4.8, §6). A plain <button>, styled to read as text —
   the icon/label take --muted, never --link, so it does not compete with
   the trigger above it as a second "control" colour. */
.claim-conformance-copy{display:flex;align-items:center;gap:14px;margin:14px -8px 0;padding:6px 8px;border:0;border-radius:var(--radius);background:none;font:inherit;color:inherit;cursor:pointer;text-align:left}
.claim-conformance-copy-icon{flex-shrink:0;color:var(--muted)}
.claim-conformance-copy-label{font-family:var(--font-sans);font-size:12px;line-height:16px;font-weight:500;color:var(--muted)}
.claim-conformance-copy-caption{margin-left:6px;font-family:var(--font-sans);font-size:12px;line-height:16px;font-weight:400;color:var(--faint)}

/* NOT --hover-bg, deliberately: this row is much larger than the other
   hover targets and takes the same fainter .07 wash as the existing footer
   precedent (style.css:4129-4133), extended to :focus-visible so a keyboard
   user gets the background half too (06a §8.12's caveat). The disclosure
   trigger keeps its --link colour on hover/focus — 06a §8.11: "the accent
   label must not darken on hover; the accent already means 'this is a
   control'" — only the Copy row's --muted label takes the ink darkening
   the footer precedent otherwise pairs with the wash. */
.claim-conformance-disclosure-trigger:hover,
.claim-conformance-disclosure-trigger:focus-visible,
.claim-conformance-copy:hover,
.claim-conformance-copy:focus-visible{background:rgba(125,137,154,.07);outline:none}
.claim-conformance-copy:hover .claim-conformance-copy-label,
.claim-conformance-copy:focus-visible .claim-conformance-copy-label{color:var(--ink)}

/* M5/M6/M7 (06a §5) — the fixed label columns unwind to stacked blocks
   below 520px; arrangement only, per tokens.md §8, mapped onto the engine's
   existing breakpoint rather than a new 390px query. */
@media (max-width: 520px) {
  .claim-conformance-check{padding-block:16px 20px}
  .claim-conformance-check:first-of-type{margin-top:14px}
  .claim-conformance-row{flex-direction:column;gap:3px;padding-block:10px}
  .claim-conformance-row-label{width:auto}
  .claim-conformance-explain{max-width:none;margin-top:6px}
  .claim-conformance-chip{min-width:24px;height:28px;font-weight:500}
  .claim-conformance-chip--missing{font-weight:600}
  .claim-conformance-coverage{gap:3px}
  .claim-conformance-machine{padding:12px var(--card-gutter, 16px) 18px}
  .claim-conformance-line{flex-direction:column;gap:2px;padding-block:8px}
  .claim-conformance-line>span:not(.claim-conformance-values){width:auto;font-weight:500}
}

/* M13 (06a §5) — the disclosure trigger and Copy row are this screen's only
   controls; a coarse pointer needs a >=44px hit target achieved by padding,
   not by growing the label, matching the engine's existing
   .comment-chip precedent (style.css:1852-1860). */
@media (pointer: coarse) {
  .claim-conformance-disclosure-trigger,
  .claim-conformance-copy{min-height:44px}
}
`

// conformanceStatusFetchGuardJS is injected only when the generated viewer
// contains conformance results. Serve can legitimately have two status polls
// in flight while a watched observation changes; the older transport may
// finish last even though the server computed the newer verdict later. The
// guard rejects that superseded response both when headers arrive and after
// JSON parsing, so the ordinary runtime's existing failure path leaves the
// newer panel intact. Keeping this out of viewer-runtime.js is deliberate:
// projects that never opt in retain the exact historical viewer bytes.
//
// The same guarded script also wires the "Copy as JSON" control
// (docs/design/screens/06a-how-this-was-checked-inline-disclosure.md §4.8,
// §6) — a delegated click listener, appended here rather than as a second
// shell.html injection point, so opted-out projects still retain the exact
// historical viewer bytes.
const conformanceStatusFetchGuardJS = `/* dossierx-conformance-status-freshness */
(function () {
  var nativeFetch = window.fetch.bind(window);
  var latestStatusRequest = 0;
  function superseded() {
    var error = new Error('superseded status response');
    error.name = 'AbortError';
    return error;
  }
  window.fetch = function (input, init) {
    var url = typeof input === 'string' ? input : (input && input.url);
    if (url !== '/api/status') { return nativeFetch(input, init); }
    var request = ++latestStatusRequest;
    return nativeFetch(input, init).then(function (response) {
      if (request !== latestStatusRequest) { throw superseded(); }
      var nativeJSON = response.json.bind(response);
      response.json = function () {
        return nativeJSON().then(function (value) {
          if (request !== latestStatusRequest) { throw superseded(); }
          return value;
        });
      };
      return response;
    });
  };
})();
/* dossierx-conformance-copy-json */
document.addEventListener('click', function (event) {
  var button = event.target && event.target.closest && event.target.closest('.claim-conformance-copy');
  if (!button) { return; }
  var payload = button.getAttribute('data-copy-json') || '';
  if (window.navigator && navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(payload).catch(function () {});
  }
});`

func viewerCSSWithConformance(base []byte, results map[string]conformance.Result) []byte {
	if len(results) == 0 {
		return base
	}
	out := make([]byte, 0, len(base)+len(conformanceCSS))
	out = append(out, base...)
	out = append(out, conformanceCSS...)
	return out
}

func statusFetchGuardWithConformance(results map[string]conformance.Result) []byte {
	if len(results) == 0 {
		return nil
	}
	return []byte(conformanceStatusFetchGuardJS)
}
