package render

import (
	"fmt"
	"html/template"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/graph"
)

// graphPayloadJSON builds the claims-graph payload for cat and returns it
// ready for injection into shell.html's
// <script type="application/json" id="dossierx-graph"> block.
//
// It is the render path's SINGLE call site of graph.Build, and it is named as
// such because it is THE CACHE SEAM.
//
// graph.Build runs on every render — under "dossierx serve" that is every
// GET / and every GET /api/fragment, and there is no caching of rendered HTML
// across render cycles. That cost is accepted for v0.5.0 and the seam is
// named rather than built: a future memoization belongs HERE, keyed on the
// catalog's content and returning the cached template.JS with only
// generated_at restamped. Nothing outside this function would have to change
// to add it, and graph.Build staying a pure, clock-free function of
// (catalog, config) is exactly what keeps that true. internal/graph ships a
// benchmark so the number being traded away is measured rather than guessed.
//
// GET /api/graph deliberately does NOT use this function. That is not an
// oversight and not duplication to remove: the seam exists to avoid
// re-deriving a payload nothing asked to change, and that endpoint's entire
// contract is that something did. It builds and stamps its own, with
// time.Now() at request time rather than the render's generatedAt.
//
// generatedAt is threaded in from Render rather than read here, for the same
// reason generatedHeader takes it: the payload's timestamp, line 1's
// generated-by comment and the sidebar footer's visible "Generated ..."
// string are then all the SAME instant, and a reviewer comparing them never
// sees a mismatch from three clock reads a few instructions apart. graph.Build
// itself never reads a clock at all — it leaves GeneratedAt empty and every
// caller stamps it, which is what keeps the payload a pure function of the
// corpus and keeps a moving byte out of the unit under test.
func graphPayloadJSON(cat *catalog.Catalog, cfg *config.Config, generatedAt time.Time) (template.JS, error) {
	p := graph.Build(cat, cfg)
	p.GeneratedAt = generatedAt.UTC().Format(time.RFC3339)

	// graph.Encode marshals with encoding/json's DEFAULT HTML escaping, which
	// is the only thing standing between an author-authored claim label and a
	// </script> breakout in the JSON block. The bytes are handed to
	// template.JS verbatim: html/template applies no escaping of its own in
	// that context, so any post-processing here would be removing the guard.
	b, err := graph.Encode(p)
	if err != nil {
		return "", fmt.Errorf("render: encode graph payload: %w", err)
	}
	return template.JS(b), nil
}
