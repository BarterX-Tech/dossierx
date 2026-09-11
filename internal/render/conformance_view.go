package render

import (
	"github.com/BarterX-Tech/dossierx/internal/conformance"
)

const conformanceCSS = `
/* Added only when this viewer contains structured conformance results. */
.claim-conformance{margin:.65rem 0 0;padding:.65rem .75rem;border:1px solid var(--border);border-left:3px solid var(--border-strong,var(--border));border-radius:4px;background:var(--hover-bg,transparent);font-size:.78rem}
.claim-conformance[data-implementation-ready="true"]{border-left-color:var(--accent)}
.claim-conformance[data-implementation-ready="false"]{border-left-color:var(--warn)}
.claim-conformance-head,.claim-conformance-check-head,.claim-conformance-line,.claim-conformance-scope{margin:0}
.claim-conformance-head{display:flex;align-items:center;justify-content:space-between;gap:.5rem;color:var(--ink)}
.claim-conformance-scope{margin-top:.35rem;color:var(--muted)}
.claim-conformance-check{margin-top:.55rem;padding:.5rem .6rem;border:1px solid var(--border);border-left:3px solid var(--border-strong,var(--border));border-radius:4px;background:var(--card-bg,transparent)}
.claim-conformance-check[data-conformance-state="matched"]{border-left-color:var(--accent)}
.claim-conformance-check[data-conformance-state="mismatch"],.claim-conformance-check[data-conformance-state="uncheckable"]{border-left-color:var(--warn)}
.claim-conformance-check-head{display:flex;align-items:center;justify-content:space-between;gap:.5rem;color:var(--ink)}
.claim-conformance-line{margin-top:.3rem;overflow-wrap:anywhere}
.claim-conformance-line>span{color:var(--muted)}
`

// conformanceStatusFetchGuardJS is injected only when the generated viewer
// contains conformance results. Serve can legitimately have two status polls
// in flight while a watched observation changes; the older transport may
// finish last even though the server computed the newer verdict later. The
// guard rejects that superseded response both when headers arrive and after
// JSON parsing, so the ordinary runtime's existing failure path leaves the
// newer panel intact. Keeping this out of viewer-runtime.js is deliberate:
// projects that never opt in retain the exact historical viewer bytes.
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
})();`

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
