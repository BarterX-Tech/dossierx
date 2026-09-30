package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// BenchmarkBriefRelationsAtScale is the NIT-202 graph-safety scale evidence
// (docs/graph-safety/nit-202-brief-relations.md): 2,000 claims, 60 briefs
// (the default max_briefs) each resting on 200 claims, and every claim citing
// three briefs with a holding pin. It reports the rows the lookup derives and
// the bytes they add to the claim cards, so the note's linear bound is
// measured, not asserted.
func BenchmarkBriefRelationsAtScale(b *testing.B) {
	const claims, nBriefs, restsOn, cites = 2000, 60, 200, 3
	files := map[string]string{}
	var digests []string
	for i := 0; i < nBriefs; i++ {
		var ids []string
		for j := 0; j < restsOn; j++ {
			ids = append(ids, fmt.Sprintf("widget.contract.c%04d", (i*restsOn+j)%claims))
		}
		body := fmt.Sprintf("---\nsummary: Brief %d.\nstatus: locked\nrests_on: [%s]\n---\n# Brief number %d\n\n%s\n", i, strings.Join(ids, ", "), i, strings.Repeat("words ", 1900))
		files[fmt.Sprintf("briefs/f%02d/b%02d.md", i%5, i)] = body
		digests = append(digests, sha(body))
	}
	cfg, set := briefRelationsProject(b, files)
	cat := &catalog.Catalog{}
	for i := 0; i < claims; i++ {
		c := model.Claim{ID: fmt.Sprintf("widget.contract.c%04d", i), Module: "widget", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft}
		for k := 0; k < cites; k++ {
			n := (i + k) % nBriefs
			c.Sources = append(c.Sources, model.Source{Ref: k + 1, Kind: model.SourceKindInternal, Title: "t", Path: fmt.Sprintf("briefs/f%02d/b%02d.md", n%5, n), SHA256: digests[n]})
		}
		cat.Claims = append(cat.Claims, c)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var lookup map[string]components.BriefRelations
	for i := 0; i < b.N; i++ {
		lookup = buildBriefRelationsLookup(cat, cfg, set)
	}
	b.StopTimer()
	rows, bytes := 0, 0
	for _, c := range cat.Claims {
		rows += lookup[c.ID].Len()
		bytes += len(components.EdgesHTMLWithCodeLinks(c, nil, false, nil, nil, lookup[c.ID])) - len(components.EdgesHTMLWithCodeLinks(c, nil, false, nil, nil, components.BriefRelations{}))
	}
	b.ReportMetric(float64(rows), "rows")
	b.ReportMetric(float64(bytes), "card-bytes")
}
