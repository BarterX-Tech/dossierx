package render

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// mockupRenderClaim builds a layout: mockup claim carrying the given status,
// reviewed flag, and raw_html, in module "widget" / facet "internals".
func mockupRenderClaim(status model.Status, reviewed bool, rawHTML string) model.Claim {
	return model.Claim{
		ID:              "widget.internals.console-mockup",
		Facet:           "internals",
		Module:          "widget",
		Layout:          model.LayoutMockup,
		RawHTML:         rawHTML,
		RawHTMLReviewed: reviewed,
		Status:          status,
	}
}

// TestRender_MockupDefenseInDepth is the DX-AUD-08 render-layer regression:
// mockup.html must emit .RawHTML unescaped ONLY when the claim is locked AND
// raw_html_reviewed AND its module is in cfg.MockupModules; in every other
// case the same content must be HTML-escaped, so a draft, unreviewed, or
// non-allowlisted mockup that somehow reaches render can never inject live
// markup into the client-shared viewer.
func TestRender_MockupDefenseInDepth(t *testing.T) {
	const safe = `<div class="gcp-row"><span class="mockup-badge">ERROR</span></div>`
	allowCfg := &config.Config{
		Modules:       []string{"widget"},
		Facets:        []string{"internals"},
		MockupModules: []string{"widget"},
	}
	noAllowCfg := &config.Config{
		Modules: []string{"widget"},
		Facets:  []string{"internals"},
		// MockupModules deliberately empty.
	}

	cases := []struct {
		name         string
		cfg          *config.Config
		claim        model.Claim
		wantUnescape bool
	}{
		{
			name:         "locked + reviewed + allowlisted renders unescaped",
			cfg:          allowCfg,
			claim:        mockupRenderClaim(model.StatusLocked, true, safe),
			wantUnescape: true,
		},
		{
			name:         "draft but reviewed + allowlisted is escaped (not locked)",
			cfg:          allowCfg,
			claim:        mockupRenderClaim(model.StatusDraft, true, safe),
			wantUnescape: false,
		},
		{
			name:         "locked + allowlisted but not reviewed is escaped",
			cfg:          allowCfg,
			claim:        mockupRenderClaim(model.StatusLocked, false, safe),
			wantUnescape: false,
		},
		{
			name:         "locked + reviewed but module not allowlisted is escaped",
			cfg:          noAllowCfg,
			claim:        mockupRenderClaim(model.StatusLocked, true, safe),
			wantUnescape: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cat, err := catalog.Build([]model.Claim{tc.claim}, tc.cfg)
			if err != nil {
				t.Fatalf("catalog.Build: %v", err)
			}
			out, err := Render(cat, tc.cfg)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			hasLive := strings.Contains(out, `<span class="mockup-badge">ERROR</span>`)
			hasEscaped := strings.Contains(out, `&lt;span class=&#34;mockup-badge&#34;&gt;`)
			if tc.wantUnescape {
				if !hasLive {
					t.Fatalf("expected unescaped live markup in output, got:\n%s", out)
				}
			} else {
				if hasLive {
					t.Fatalf("expected mockup markup to be escaped, but live markup appeared:\n%s", out)
				}
				if !hasEscaped {
					t.Fatalf("expected HTML-escaped mockup markup in output, got:\n%s", out)
				}
			}
		})
	}
}
