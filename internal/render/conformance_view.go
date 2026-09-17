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
   label + hairline rule + check count shown once, above every check
   article, when the outer <details> is open. The rule element right-ranges
   the count instead of it sitting immediately after the label (06a §4.3
   '3KE-0' gap 12px, '3KG-0' rule). */
.claim-conformance-eyebrow{display:flex;align-items:center;gap:12px;margin-top:.6rem;padding-top:10px;border-top:1px solid var(--border);font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:var(--faint)}
.claim-conformance-eyebrow-rule{flex:1 1 auto;height:1px;background:color-mix(in srgb,var(--warn) 13%,var(--card-bg))}
.claim-conformance-eyebrow-count{font-family:var(--font-mono);font-weight:400;letter-spacing:0}

/* One check (06a §4.3-§4.8). The panel ground follows the verdict
   (06a §9 Open decision 1-2): a warm/blocked wash for Mismatch/Uncheckable,
   a draft wash for Owed, the neutral code ground for Matched — derived with
   color-mix() rather than the six raw hexes the Paper boards paint (06a
   Paper defects 1-3), so both colour modes fall out of one declaration. */
.claim-conformance-check{margin-top:10px;padding:18px var(--card-gutter, 32px) 24px;border-top:1px solid var(--border);background:var(--code-bg)}
.claim-conformance-check:first-of-type{margin-top:16px}
/* RETRY FIX (verifier item 14): 6% read as a neutral lift rather than a
   warm wash on dark, where --card-bg's blue channel already outweighs
   --warn's red contribution at that share (measured rgb(35,34,40) against
   the board's warm #1D1618). Raised to 14% so the hue survives the mix;
   06a §9 Open decisions 1-2 sanction the derivation, not a fixed 6%. */
.claim-conformance-check[data-conformance-state="mismatch"],
.claim-conformance-check[data-conformance-state="uncheckable"]{background:color-mix(in srgb,var(--warn) 14%,var(--card-bg));border-top-color:color-mix(in srgb,var(--warn) 22%,var(--border))}
.claim-conformance-check[data-conformance-state="owed"]{background:color-mix(in srgb,var(--status-draft) 14%,var(--card-bg));border-top-color:color-mix(in srgb,var(--status-draft) 22%,var(--border))}

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
   (06a §2, §4.3; tokens.md §7's family rule). RETRY FIX (verifier item 16,
   06a §4.3 '3KN-0'): the pull-up is negative, not positive. */
.claim-conformance-explain{margin:-8px 0 0;max-width:700px;font-family:var(--font-serif);font-size:15px;line-height:23px;font-weight:400;color:var(--muted)}

/* EXAMINED / COMPARED / FOUND (06a §4.4-§4.5, §6) — a fixed label column,
   unwound to a stacked block at 520px (M5). RETRY FIX (verifier item 13,
   06a §9 Open decision 1): the hairline mixes into --card-bg, matching the
   panel ground's own derivation, not into --border. */
.claim-conformance-row{display:flex;align-items:baseline;gap:20px;padding-block:10px;border-top:1px solid color-mix(in srgb,var(--warn) 13%,var(--card-bg))}
.claim-conformance-check[data-conformance-state="matched"] .claim-conformance-row{border-top-color:var(--border)}
.claim-conformance-check[data-conformance-state="owed"] .claim-conformance-row{border-top-color:color-mix(in srgb,var(--status-draft) 13%,var(--card-bg))}
.claim-conformance-row-label{width:92px;flex-shrink:0;font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:600;letter-spacing:.06em;color:var(--faint)}
.claim-conformance-row-value{font-family:var(--font-serif);font-size:14px;line-height:22px;font-weight:400;color:var(--muted);overflow-wrap:anywhere}
.claim-conformance-row--found{align-items:flex-start}
.claim-conformance-row--found>.claim-conformance-row-value{display:flex;flex-direction:column;gap:9px}

/* The coverage strip (R12.3, R-H.3, 06a §4.5) — a marked set, not a diff;
   left-to-right order is the declared members' own order and carries no
   claim of its own. Component-board sizing wins over 06a's own boards per
   R00.0 (06a Paper defect 12): 11/14 chip labels, not 12/16. RETRY FIX
   (verifier item 4, 06a §9 Open decision 4): the strip never wraps —
   writeConformanceFoundRow caps the run at 13 chips (R-H.3's own proven
   fit) and appends a "+N" marker instead of letting the flex row wrap. */
.claim-conformance-coverage{display:flex;flex-wrap:nowrap;gap:5px}
/* RETRY FIX (verifier item 3, 06a §4.5, R-H.3): a fixed 32×29 box with no
   padding-inline — the chip now holds a short ordinal (writeConformanceFoundRow),
   never a project-supplied member id of arbitrary length, so the box no
   longer needs to grow with content; that is what makes thirteen chips'
   worth of width (348px) predictable and the nowrap strip above safe. */
.claim-conformance-chip{display:inline-flex;align-items:center;justify-content:center;width:32px;height:29px;flex-shrink:0;border-radius:5px;border:1px solid var(--border);background:var(--card-bg);font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:400;color:var(--muted)}
.claim-conformance-chip--missing{border:1.5px solid var(--warn);background:color-mix(in srgb,var(--warn) 12%,var(--card-bg));font-weight:600;color:var(--warn)}
.claim-conformance-chip-overflow{display:inline-flex;align-items:center;justify-content:center;height:29px;flex-shrink:0;font-family:var(--font-mono);font-size:11px;line-height:14px;font-weight:400;color:var(--faint)}
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
/* RETRY FIX (verifier items 9, 13): the open strip goes full-bleed to the
   check body's own inner measure — margin-inline cancels the check's
   --card-gutter padding and padding-inline restores it as the strip's own,
   so the strip (and its hairline) spans the check's full width instead of
   sitting inset inside it (06a §4.7 '2UX-0': padding 10px 32px 14px on a
   1100-wide strip, the check body's own width). The hairline mixes into
   --card-bg per Open decision 1, not --border. */
.claim-conformance-disclosure[open]>.claim-conformance-disclosure-trigger{margin-inline:calc(-1 * var(--card-gutter, 32px));padding:10px var(--card-gutter, 32px) 14px;margin-top:10px;border-top:1px solid color-mix(in srgb,var(--warn) 13%,var(--card-bg))}
.claim-conformance-chevron{flex-shrink:0;color:var(--link);transition:transform .12s ease}
.claim-conformance-disclosure[open] .claim-conformance-chevron{transform:rotate(-90deg)}
/* RETRY FIX (verifier item 10, 06a §4.7 '2V1-0'/'2V2-0'): the spacer and
   snapshot line exist in the DOM in both states (see
   writeConformanceDisclosure) but only the open strip shows them — the
   closed trigger (§4.6) carries no snapshot. */
.claim-conformance-disclosure-spacer{display:none}
.claim-conformance-snapshot{display:none;flex-shrink:0;font-family:var(--font-mono);font-size:11px;line-height:14px;font-weight:400;color:var(--faint);white-space:nowrap}
.claim-conformance-disclosure[open] .claim-conformance-disclosure-spacer{display:block;flex:1 1 auto;height:0}
.claim-conformance-disclosure[open] .claim-conformance-snapshot{display:block}

/* The machine layer (06a §4.8) — the engine's own nine-key envelope,
   verbatim, lowercase mono; --code-bg is the token 06a Paper defects 1/11
   flag as the light board's own unspelled ground. */
.claim-conformance-machine{padding:16px var(--card-gutter, 32px) 22px;background:var(--code-bg);border-top:1px solid var(--border)}
.claim-conformance-line{display:flex;align-items:baseline;gap:18px;padding-block:9px;overflow-wrap:anywhere;font-size:11.5px}
.claim-conformance-line+.claim-conformance-line{border-top:1px solid var(--border)}
/* RETRY FIX (verifier item 18): min-width, not width, and nowrap — the
   nine short keys (06a §4.8) still sit at 84px, but the two longer §8.7
   keys ("observation error:", "adapter message:") are allowed to size past
   it instead of wrapping inside a box too narrow for them. */
.claim-conformance-line>span:not(.claim-conformance-values){min-width:84px;flex-shrink:0;white-space:nowrap;font-family:var(--font-mono);font-size:11px;line-height:14px;font-weight:400;color:var(--faint)}
/* RETRY FIX (verifier item 17): the viewer's live 'code' pill (a background
   + border + padding box) is neutralised inside the machine layer — the
   board draws plain mono text on the code ground, not a run of small boxed
   chips with floating commas between them. */
.claim-conformance-machine code{background:none;border:0;padding:0}
.claim-conformance-line>code,.claim-conformance-values{font-family:var(--font-mono);font-size:12px;line-height:19px;color:var(--ink);overflow-wrap:anywhere}
/* The multi-member value (missing/extra/expected/observed on a set-shaped
   check) is ONE flex item, not several — see writeConformanceMembers'
   doc comment. min-width:0 is required for a flex item to be allowed to
   wrap/shrink below its content's intrinsic width at all. */
.claim-conformance-values{flex:1 1 auto;min-width:0}
/* RETRY FIX (verifier items 1-2, 06a §4.8 '2VR-0'/'2VU-0', §8.7): the
   'missing' and 'observation error' values take the blocked hue; an empty
   'extra' row's em dash takes --color-faint. The descendant combinator
   reaches writeConformanceMembersModified's wrapper span too. */
.claim-conformance-line--blocked>code,.claim-conformance-line--blocked .claim-conformance-values{color:var(--warn);font-weight:500}
.claim-conformance-line--empty>code{color:var(--faint)}

/* declared_none (07 §4.12, verifier item 15) — DECLARATION / REASON, a
   96px uppercase label column, mono declaration value, serif reason; the
   closing scope note sits below the pair, serif and wider-measured. This
   is a distinct shape from the machine layer above (84px, tracking .06em,
   mono-only) — 06a §8.4 keeps this branch free of any nested disclosure. */
.claim-conformance-declared{margin-top:.6rem}
.claim-conformance-declared-row{display:flex;align-items:baseline;gap:20px;padding-block:10px;border-top:1px solid var(--border)}
.claim-conformance-declared-label{width:96px;flex-shrink:0;font-family:var(--font-sans);font-size:11px;line-height:14px;font-weight:600;letter-spacing:.07em;text-transform:uppercase;color:var(--faint)}
.claim-conformance-declared-value{font-family:var(--font-mono);font-size:12px;line-height:20px;font-weight:400;color:var(--muted);overflow-wrap:anywhere}
.claim-conformance-declared-reason{font-family:var(--font-serif);font-size:14px;line-height:22px;max-width:640px}
.claim-conformance-declared-note{margin:14px 0 0;max-width:760px;font-family:var(--font-serif);font-size:15px;line-height:24px;font-weight:400;color:var(--muted)}

/* Copy as JSON (06a §4.8, §6). A plain <button>, styled to read as text —
   the icon/label take --muted, never --link, so it does not compete with
   the trigger above it as a second "control" colour. RETRY FIX (verifier
   item 11): icon↔label is its own 6px gap; label↔caption is that same 6px
   plus the caption's own 8px margin, totalling the row's 14px (06a §4.8
   '2VZ-0' 6px icon↔label). */
.claim-conformance-copy{display:flex;align-items:center;gap:6px;margin:14px -8px 0;padding:6px 8px;border:0;border-radius:var(--radius);background:none;font:inherit;color:inherit;cursor:pointer;text-align:left}
.claim-conformance-copy-icon{flex-shrink:0;color:var(--muted)}
.claim-conformance-copy-label{font-family:var(--font-sans);font-size:12px;line-height:16px;font-weight:500;color:var(--muted)}
.claim-conformance-copy-caption{margin-left:8px;font-family:var(--font-sans);font-size:12px;line-height:16px;font-weight:400;color:var(--faint)}

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
  /* RETRY FIX (verifier item 5, 06a §3 '80G-0'/'83D-0', §5 M2/M3): the
     panel's own padding/border stop adding a second inset on top of the
     card's own --card-gutter, so the check body's 358px inner measure
     starts flush from the card edge instead of x=31 inside it. */
  .claim-conformance{padding-inline:0;border-inline:0;border-radius:0}
  .claim-conformance-check{padding-block:16px 20px}
  .claim-conformance-check:first-of-type{margin-top:14px}
  .claim-conformance-row{flex-direction:column;gap:3px;padding-block:10px}
  .claim-conformance-row-label{width:auto}
  /* RETRY FIX (verifier item 16, 06a §4.3 '80Q-0'): -6px, not +6px. */
  .claim-conformance-explain{max-width:none;margin-top:-6px}
  .claim-conformance-chip{width:24px;height:28px}
  .claim-conformance-chip--missing{font-weight:600}
  .claim-conformance-chip-overflow{height:28px}
  .claim-conformance-coverage{gap:3px}
  /* RETRY FIX (verifier item 9, 06a §4.7 mobile column: padding 10px 16px
     12px): the open trigger's full-bleed inset and vertical rhythm both
     change at 390, not just the inline padding-gutter's own value. */
  .claim-conformance-disclosure[open]>.claim-conformance-disclosure-trigger{margin-inline:calc(-1 * var(--card-gutter, 16px));padding:10px var(--card-gutter, 16px) 12px}
  .claim-conformance-machine{padding:12px var(--card-gutter, 16px) 18px}
  .claim-conformance-line{flex-direction:column;gap:2px;padding-block:8px}
  .claim-conformance-line>span:not(.claim-conformance-values){width:auto;min-width:0;font-weight:500}
  .claim-conformance-declared-row{flex-direction:column;gap:3px;padding-block:11px}
  .claim-conformance-declared-label{width:auto}
  .claim-conformance-declared-reason{font-size:14px;line-height:21px;max-width:none}
  .claim-conformance-declared-note{font-size:14px;line-height:22px;max-width:none}
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
