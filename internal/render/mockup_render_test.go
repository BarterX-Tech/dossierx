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

// ---------------------------------------------------------------------
// raw_html on EVERY layout (v0.4.1, issue #25)
//
// v0.4.1 dropped the "layout must be mockup" leg of the raw-html gate and
// gave all seven layout partials the same attachment block:
//
//	{{if .RawHTML}}<div class="claim-mockup-body">{{mockupHTML .}}</div>{{end}}
//
// Nothing asserted any of those seven lines. A future edit writing
// {{.RawHTML}} directly into a partial — bypassing the gated "mockupHTML"
// func — would leak unescaped project HTML into the client-shared viewer with
// the whole suite still green, and check_parity_test.go's old
// raw_html-by-layout assertion (deleted in this release) is no longer there to
// catch it either. The two tests below are that missing coverage: for each of
// the seven layouts, the attachment renders in its own wrapper in the right
// place, AND the DX-AUD-08 escape gate still bites on that layout for every
// negative combination of locked / raw_html_reviewed / module-allowlisted.
// ---------------------------------------------------------------------

// rawHTMLLayouts is the complete set of model.Layout values a claim can carry.
// A new layout added to the engine without a line here is the gap this whole
// block exists to prevent, so keep it in sync with components.fileForLayout.
var rawHTMLLayouts = []model.Layout{
	model.LayoutCard,
	model.LayoutTable,
	model.LayoutList,
	model.LayoutSteps,
	model.LayoutTree,
	model.LayoutMockup,
	model.LayoutBanner,
}

// rawHTMLBodyMarker is a nonsense token planted in each layout's OWN content
// field (Body / Rows / Steps) so the position assertion can find where that
// content ended without matching viewer chrome or the raw_html itself.
const rawHTMLBodyMarker = "ZZLAYOUTCONTENTZZ"

// rawHTMLLayoutClaim builds a claim on the given layout carrying raw_html plus
// whatever content field that layout actually renders, so the attachment has
// something to sit after. RestsOn is always set: it guarantees a non-empty
// edges footer on the six partials that emit one, which is what the
// "before the footer" half of the position assertion needs.
func rawHTMLLayoutClaim(layout model.Layout, status model.Status, reviewed bool, rawHTML string) model.Claim {
	c := model.Claim{
		ID:              "widget.internals.rawhtml-" + string(layout),
		Facet:           "internals",
		Module:          "widget",
		Layout:          layout,
		RawHTML:         rawHTML,
		RawHTMLReviewed: reviewed,
		Status:          status,
		RestsOn:         []string{"widget.internals.anchor"},
	}
	switch layout {
	case model.LayoutTable:
		c.Rows = []model.Row{{"field": rawHTMLBodyMarker}}
	case model.LayoutSteps:
		c.Steps = []string{rawHTMLBodyMarker}
	case model.LayoutList:
		c.Body = "- " + rawHTMLBodyMarker
	default:
		// card, tree, mockup, banner all render Body.
		c.Body = rawHTMLBodyMarker
	}
	return c
}

// rawHTMLRender renders a single-claim catalog and returns just that claim's
// <section>, so the ordering assertions below cannot be satisfied by an
// incidental match somewhere else in the shell (the embedded stylesheet, for
// one, mentions .claim-mockup-body and .claim-links by name).
func rawHTMLRender(t *testing.T, c model.Claim, cfg *config.Config) string {
	t.Helper()
	cat, err := catalog.Build([]model.Claim{c}, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	open := `<section class="claim claim-` + string(c.Layout)
	i := strings.Index(out, open)
	if i < 0 {
		t.Fatalf("no %q section in output:\n%s", open, out)
	}
	rest := out[i:]
	j := strings.Index(rest, "</section>")
	if j < 0 {
		t.Fatalf("unterminated %q section in output:\n%s", open, rest)
	}
	return rest[:j]
}

const (
	rawHTMLSample  = `<div class="gcp-row"><span class="mockup-badge">ERROR</span></div>`
	rawHTMLLive    = `<span class="mockup-badge">ERROR</span>`
	rawHTMLEscaped = `&lt;span class=&#34;mockup-badge&#34;&gt;`
	rawHTMLWrapper = `<div class="claim-mockup-body">`
	rawHTMLFooter  = `<details class="claim-links"`
)

// TestRender_RawHTMLAttachment_EveryLayout is the structural half: every one of
// the seven partials must render a raw_html-carrying claim's markup inside a
// .claim-mockup-body wrapper, positioned AFTER that layout's own content and
// BEFORE the edges footer (banner, the one partial with no footer, only has
// the "after content" half to check).
func TestRender_RawHTMLAttachment_EveryLayout(t *testing.T) {
	cfg := &config.Config{
		Modules:       []string{"widget"},
		Facets:        []string{"internals"},
		MockupModules: []string{"widget"},
	}

	for _, layout := range rawHTMLLayouts {
		t.Run(string(layout), func(t *testing.T) {
			sec := rawHTMLRender(t, rawHTMLLayoutClaim(layout, model.StatusLocked, true, rawHTMLSample), cfg)

			wrapper := strings.Index(sec, rawHTMLWrapper)
			if wrapper < 0 {
				t.Fatalf("layout %s renders no %s wrapper for a raw_html claim:\n%s", layout, rawHTMLWrapper, sec)
			}
			live := strings.Index(sec, rawHTMLLive)
			if live < 0 {
				t.Fatalf("layout %s: locked+reviewed+allowlisted raw_html did not render live:\n%s", layout, sec)
			}
			if live < wrapper {
				t.Fatalf("layout %s renders raw_html outside/before its %s wrapper:\n%s", layout, rawHTMLWrapper, sec)
			}

			content := strings.Index(sec, rawHTMLBodyMarker)
			if content < 0 {
				t.Fatalf("layout %s dropped its own body/rows/steps content:\n%s", layout, sec)
			}
			if wrapper < content {
				t.Fatalf("layout %s renders the raw_html attachment BEFORE its own content; it must follow it:\n%s", layout, sec)
			}

			footer := strings.Index(sec, rawHTMLFooter)
			if layout == model.LayoutBanner {
				// banner.html deliberately emits no {{edges .}} footer.
				if footer >= 0 {
					t.Fatalf("banner grew an edges footer; this test's position rule needs updating:\n%s", sec)
				}
				return
			}
			if footer < 0 {
				t.Fatalf("layout %s rendered no edges footer for a claim with rests_on:\n%s", layout, sec)
			}
			if footer < wrapper {
				t.Fatalf("layout %s renders the raw_html attachment AFTER its edges footer; it must precede it:\n%s", layout, sec)
			}
		})
	}
}

// TestRender_RawHTMLEscapeGate_EveryLayout is the security half: DX-AUD-08's
// three-conjunct gate (locked AND raw_html_reviewed AND module allowlisted) is
// layout-blind, so EVERY layout must escape a claim's raw_html whenever any one
// of the three fails. This is what fails if a partial ever swaps {{mockupHTML .}}
// for a bare {{.RawHTML}} — that bypass renders live markup on all four rows
// below, not just the trusted one.
func TestRender_RawHTMLEscapeGate_EveryLayout(t *testing.T) {
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

	gates := []struct {
		name     string
		cfg      *config.Config
		status   model.Status
		reviewed bool
		trusted  bool
	}{
		{name: "locked+reviewed+allowlisted-is-live", cfg: allowCfg, status: model.StatusLocked, reviewed: true, trusted: true},
		{name: "draft-is-escaped", cfg: allowCfg, status: model.StatusDraft, reviewed: true},
		{name: "unreviewed-is-escaped", cfg: allowCfg, status: model.StatusLocked, reviewed: false},
		{name: "module-not-allowlisted-is-escaped", cfg: noAllowCfg, status: model.StatusLocked, reviewed: true},
		{name: "draft+unreviewed+not-allowlisted-is-escaped", cfg: noAllowCfg, status: model.StatusDraft, reviewed: false},
	}

	for _, layout := range rawHTMLLayouts {
		for _, g := range gates {
			t.Run(string(layout)+"/"+g.name, func(t *testing.T) {
				claim := rawHTMLLayoutClaim(layout, g.status, g.reviewed, rawHTMLSample)
				sec := rawHTMLRender(t, claim, g.cfg)

				if !strings.Contains(sec, rawHTMLWrapper) {
					t.Fatalf("layout %s renders no %s wrapper:\n%s", layout, rawHTMLWrapper, sec)
				}
				hasLive := strings.Contains(sec, rawHTMLLive)
				hasEscaped := strings.Contains(sec, rawHTMLEscaped)
				if g.trusted {
					if !hasLive {
						t.Fatalf("layout %s: trusted raw_html was escaped, expected live markup:\n%s", layout, sec)
					}
					return
				}
				if hasLive {
					t.Fatalf("layout %s LEAKED live raw_html for an untrusted claim (%s) — the escape gate is bypassed on this partial:\n%s", layout, g.name, sec)
				}
				if !hasEscaped {
					t.Fatalf("layout %s: expected HTML-escaped raw_html, found neither live nor escaped markup:\n%s", layout, sec)
				}
			})
		}
	}
}

// TestRender_RawHTMLAbsent_NoWrapper guards the {{if .RawHTML}} guard itself:
// a claim with no raw_html must emit no .claim-mockup-body wrapper on any
// layout, so the block stays an attachment rather than an empty box every
// ordinary claim now carries.
func TestRender_RawHTMLAbsent_NoWrapper(t *testing.T) {
	cfg := &config.Config{
		Modules:       []string{"widget"},
		Facets:        []string{"internals"},
		MockupModules: []string{"widget"},
	}

	for _, layout := range rawHTMLLayouts {
		t.Run(string(layout), func(t *testing.T) {
			sec := rawHTMLRender(t, rawHTMLLayoutClaim(layout, model.StatusLocked, true, ""), cfg)
			if strings.Contains(sec, rawHTMLWrapper) {
				t.Fatalf("layout %s emits an empty %s wrapper for a claim with no raw_html:\n%s", layout, rawHTMLWrapper, sec)
			}
		})
	}
}
