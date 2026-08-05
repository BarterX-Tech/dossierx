package graph

import (
	"fmt"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// syntheticCorpus builds an n-claim corpus with a realistic edge density:
// two rests_on edges per claim after the third, a mirrors edge on every
// fifth, and a governed_by edge on every seventh pointing at one of five
// doctrine claims. That is deliberately denser than a typical project — a
// benchmark that flatters the implementation measures nothing.
func syntheticCorpus(n int) []model.Claim {
	mods := []string{"engine", "viewer", "cli", "lock", "telemetry"}
	facets := []string{"contract", "schema", "behavior", "verification", "overview"}

	claims := make([]model.Claim, 0, n)
	for i := range n {
		claims = append(claims, model.Claim{
			ID:     fmt.Sprintf("%s.%s.claim-%05d", mods[i%len(mods)], facets[i%len(facets)], i),
			Module: mods[i%len(mods)],
			Facet:  facets[i%len(facets)],
			Status: model.StatusDraft,
		})
	}
	for i := range claims {
		if i >= 3 {
			claims[i].RestsOn = []string{claims[i-1].ID, claims[i-3].ID}
		}
		if i%5 == 0 && i+1 < n {
			claims[i].Mirrors = []string{claims[i+1].ID}
		}
		if i%7 == 0 && n > 5 {
			claims[i].Governed = model.Governed{Type: claims[i%5].ID}
		}
	}
	return claims
}

// BenchmarkBuild measures the per-render cost design §11 accepts rather than
// asserting it. graph.Build runs on EVERY render — under "dossierx serve"
// that is every GET / and every GET /api/fragment, none of which caches
// rendered HTML across cycles — so "this is cheap enough" needs to be a
// number somebody can re-measure, not a claim in a document.
//
// 1,000 claims is the size this benchmark fixes on: it is well above any
// corpus this engine has actually seen and above the 600-node threshold at
// which the pane auto-collapses to module granularity, so it measures the
// case that would hurt rather than the case that is common.
func BenchmarkBuild(b *testing.B) {
	cfg := &config.Config{
		Modules: []string{"engine", "viewer", "cli", "lock", "telemetry"},
		Facets:  []string{"contract", "schema", "behavior", "verification"},
	}
	cat, err := catalog.Build(syntheticCorpus(1000), cfg)
	if err != nil {
		b.Fatalf("catalog.Build: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		p := Build(cat, cfg)
		if len(p.Nodes) != 1000 {
			b.Fatalf("node count = %d, want 1000", len(p.Nodes))
		}
	}
}

// BenchmarkBuildAndEncode is the number that actually lands in a response:
// Build alone never leaves the process. Reported separately so a regression
// can be attributed to the projection or to the marshalling rather than to
// "the graph got slower".
func BenchmarkBuildAndEncode(b *testing.B) {
	cfg := &config.Config{
		Modules: []string{"engine", "viewer", "cli", "lock", "telemetry"},
		Facets:  []string{"contract", "schema", "behavior", "verification"},
	}
	cat, err := catalog.Build(syntheticCorpus(1000), cfg)
	if err != nil {
		b.Fatalf("catalog.Build: %v", err)
	}

	b.ReportAllocs()
	for b.Loop() {
		out, err := Encode(Build(cat, cfg))
		if err != nil {
			b.Fatalf("Encode: %v", err)
		}
		if len(out) == 0 {
			b.Fatal("empty payload")
		}
	}
}
