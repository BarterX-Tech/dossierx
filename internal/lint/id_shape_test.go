package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func idShapeTestConfig() *config.Config {
	return &config.Config{
		SchemaVersion: 1,
		Facets:        []string{"contract", "internals"},
		Modules:       []string{"widget"},
		ClaimsDir:     "claims",
	}
}

func TestIDShapeLint(t *testing.T) {
	cases := []struct {
		name         string
		claims       []model.Claim
		wantFindings int
	}{
		{
			name: "passing: well-formed ids matching config and fields",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Module: "widget", Facet: "contract"},
				{ID: "widget.internals.field-list", Module: "widget", Facet: "internals"},
			},
			wantFindings: 0,
		},
		{
			name: "failing: wrong segment count, unknown module/facet, field mismatch, bad slug",
			claims: []model.Claim{
				{ID: "widget.contract", Module: "widget", Facet: "contract"},            // wrong segment count
				{ID: "gadget.contract.overview", Module: "widget", Facet: "contract"},   // unknown module + mismatch
				{ID: "widget.doctrine.overview", Module: "widget", Facet: "contract"},   // unknown facet + mismatch
				{ID: "widget.contract.Overview_1", Module: "widget", Facet: "contract"}, // bad slug
			},
			wantFindings: 6,
		},
		{
			// Row: claim's facet segment is not in the project's configured
			// facets, in isolation (no other defect on the id).
			name: "failing: facet not in configured facets, reported alone",
			claims: []model.Claim{
				{ID: "widget.doctrine.overview", Module: "widget", Facet: "doctrine"},
			},
			wantFindings: 1,
		},
		{
			// Row: claim's module segment is not in the project's
			// configured modules, in isolation.
			name: "failing: module not in configured modules, reported alone",
			claims: []model.Claim{
				{ID: "gadget.contract.overview", Module: "gadget", Facet: "contract"},
			},
			wantFindings: 1,
		},
		{
			// Row: non-ASCII/unicode in the slug segment falls outside the
			// slugPattern character class ([a-z0-9] + single hyphens) and
			// must be rejected, not silently accepted as "some word chars".
			name: "failing: unicode slug rejected by the grammar's character class",
			claims: []model.Claim{
				{ID: "widget.contract.café-overview", Module: "widget", Facet: "contract"},
				{ID: "widget.contract.日本語", Module: "widget", Facet: "contract"},
			},
			wantFindings: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := IDShapeLint{}.Check(tc.claims, idShapeTestConfig())
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
		})
	}
}

func TestIDShape_ReservedOverviewFacetAlwaysValid(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract", "internals"}}
	claims := []model.Claim{
		{ID: "widget.overview.router", Module: "widget", Facet: "overview"},
	}
	findings := IDShapeLint{}.Check(claims, cfg)
	if len(findings) != 0 {
		t.Fatalf("got findings %#v, want none — overview must be valid even though it's not in cfg.Facets", findings)
	}
}
