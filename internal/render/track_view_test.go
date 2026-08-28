// track_view_test.go covers the track page's two contracts: what it renders
// for a project that declares tracks, and — the one that matters to every
// other project — that it renders NOTHING AT ALL for a project that does not.
package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// trackTestConfig writes and loads a minimal project config, optionally
// declaring one track. tracksYAML is spliced in verbatim so a test can state
// the config exactly as an author would write it.
func trackTestConfig(t *testing.T, tracksYAML string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - gateway\nclaims_dir: claims\n" + tracksYAML
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, cfgYAML)

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func trackTestClaim(module, slug string, status model.Status, tracks ...model.TrackRef) model.Claim {
	return model.Claim{
		ID:       module + ".contract." + slug,
		Module:   module,
		Facet:    "contract",
		Status:   status,
		Layout:   model.LayoutCard,
		Body:     slug + " body",
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: "test fixture"},
		Tracks:   tracks,
	}
}

func renderTrackProject(t *testing.T, cfg *config.Config, claims []model.Claim) string {
	t.Helper()
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

const checkoutTrackYAML = "tracks:\n  - id: checkout\n    title: Checkout\n    summary: What a buyer goes through.\n"

// TestRender_NoTracksEmitsNoTrackMarkup is the zero-cost contract, stated as
// MARKUP rather than as bytes. A project that declares no track must not gain a
// nav label, a section or a single guarded element — the whole point of
// guarding the markup on .Tracks and hugging the guard to the preceding action,
// rather than letting an empty range fall through and leave its indentation
// behind.
//
// The engine's stylesheet is deliberately NOT in scope here, and never could
// be: style.css is one embedded document injected whole into every page, so
// every rule it carries ships to every project exactly as graph.css's do. What
// must not vary with a project's config is what that project's own claims
// produce, and that is what these assertions read — element markup, never a
// class name that a rule could also mention.
func TestRender_NoTracksEmitsNoTrackMarkup(t *testing.T) {
	cfg := trackTestConfig(t, "")
	out := renderTrackProject(t, cfg, []model.Claim{trackTestClaim("widget", "main", model.StatusLocked)})

	for _, absent := range []string{
		`<div class="nav-group-label">`,
		`<section class="module-section track-section"`,
		`<header class="track-head">`,
		`<ul class="track-cites">`,
		`id="track-`,
		`data-target="#track-`,
	} {
		if strings.Contains(out, absent) {
			t.Errorf("a track-less project emitted %q", absent)
		}
	}

	// The guards must leave no WHITESPACE behind either, which is the failure
	// mode a class-name assertion cannot see: an unhugged guard emits its own
	// newline and indentation on every render, so every project's page grows a
	// blank line and every committed fixture goes stale for nothing. These are
	// the exact bytes the template produced before tracks existed.
	if !strings.Contains(out, "</details>\n        </div>\n      </nav>") {
		t.Errorf("the sidebar's track guard left markup behind ahead of </nav>")
	}
	if !strings.Contains(out, "</section>\n        \n      \n    </main>") {
		t.Errorf("the content area's track guard left markup behind ahead of </main>")
	}
}

// TestRender_TrackSectionRendersOwnedAndCited is the shape of the page: owned
// claims inline as whole cards, cited claims as pointer rows, and the pointer
// rows carrying no body.
func TestRender_TrackSectionRendersOwnedAndCited(t *testing.T) {
	cfg := trackTestConfig(t, checkoutTrackYAML)
	owned := trackTestClaim("widget", "flow", model.StatusLocked, model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns})
	owned.Body = "the owned body text"
	cited := trackTestClaim("gateway", "settle", model.StatusDraft, model.TrackRef{ID: "checkout"})
	cited.Body = "the cited body text"

	out := renderTrackProject(t, cfg, []model.Claim{owned, cited})

	for _, want := range []string{
		`<button class="sec-tab" data-target="#track-checkout" data-default-target="#track-checkout-claims">Checkout</button>`,
		`<section class="module-section track-section" id="track-checkout" hidden>`,
		`<section class="claim-group" id="track-checkout-claims" hidden>`,
		`<p class="track-summary">What a buyer goes through.</p>`,
		`the owned body text`,
		`<ul class="track-cites">`,
		`data-claim-id="gateway.contract.settle"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in the rendered track page", want)
		}
	}

	// A CITED CLAIM'S BODY APPEARS EXACTLY ONCE ON THE PAGE — in its own
	// module's facet section. A second copy inside the track is the drift this
	// whole tool exists to catch, so this assertion is the one that must not be
	// relaxed into "contains".
	if n := strings.Count(out, "the cited body text"); n != 1 {
		t.Errorf("the cited claim's body appears %d times; a track must reference, never copy", n)
	}
}

// TestRender_OwnedClaimKeepsOneCanonicalID holds the document-validity rule the
// duplicate copy exists inside: a claim id may appear once. The track's inline
// copy is the non-canonical one, so its ids are stripped and the module's copy
// keeps them.
func TestRender_OwnedClaimKeepsOneCanonicalID(t *testing.T) {
	cfg := trackTestConfig(t, checkoutTrackYAML)
	owned := trackTestClaim("widget", "flow", model.StatusLocked, model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns})
	owned.Sources = []model.Source{{Ref: 1, Kind: model.SourceKindExternal, Title: "A page"}}
	owned.Body = "budget is three [1]"

	out := renderTrackProject(t, cfg, []model.Claim{owned})

	if n := strings.Count(out, ` id="widget.contract.flow"`); n != 1 {
		t.Errorf("the claim's root id appears %d times, want exactly 1", n)
	}
	if n := strings.Count(out, ` id="widget.contract.flow-source-1"`); n != 1 {
		t.Errorf("the source row's anchor id appears %d times, want exactly 1", n)
	}
	// The body itself — and its citation marker — still render in both copies:
	// only the ids are duplicates, never the content.
	if n := strings.Count(out, `href="#widget.contract.flow-source-1"`); n != 2 {
		t.Errorf("the citation marker appears %d times, want one per rendered copy", n)
	}
}

// TestTrackCompletion is the pill's whole decision table. It is a pure function
// of the claims' states and it gates nothing — see trackCompletion.
func TestTrackCompletion(t *testing.T) {
	locked := func() model.Claim { return model.Claim{Status: model.StatusLocked} }
	draft := func() model.Claim { return model.Claim{Status: model.StatusDraft} }
	pending := func() model.Claim { return model.Claim{Status: model.StatusLocked, ReviewPending: true} }

	cases := []struct {
		name         string
		owned, cited []model.Claim
		wantClass    string
		wantLabel    string
	}{
		{
			name:      "a declared track with nothing in it is empty, never complete",
			wantClass: "pv",
			wantLabel: "empty · no claims yet",
		},
		{
			name:      "every claim locked, owned and cited alike",
			owned:     []model.Claim{locked()},
			cited:     []model.Claim{locked(), locked()},
			wantClass: "ps",
			wantLabel: "complete · 3 / 3 locked",
		},
		{
			name:      "a cited draft keeps the track incomplete",
			owned:     []model.Claim{locked()},
			cited:     []model.Claim{draft()},
			wantClass: "pv",
			wantLabel: "incomplete · 1 / 2 locked",
		},
		{
			name:      "an owned draft keeps the track incomplete",
			owned:     []model.Claim{draft()},
			cited:     []model.Claim{locked()},
			wantClass: "pv",
			wantLabel: "incomplete · 1 / 2 locked",
		},
		{
			name:      "review_pending counts as locked and is stated separately",
			owned:     []model.Claim{pending()},
			cited:     []model.Claim{locked()},
			wantClass: "pw",
			wantLabel: "complete · 2 / 2 locked · 1 review_pending",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, label := trackCompletion(tc.owned, tc.cited)
			if class != tc.wantClass || label != tc.wantLabel {
				t.Errorf("trackCompletion = (%q, %q), want (%q, %q)", class, label, tc.wantClass, tc.wantLabel)
			}
		})
	}
}

// TestPartitionTrackClaims_OwnershipWins holds the one authoring mistake this
// page could compound: a claim that declares the same track in both roles is
// listed once, as owned. track-multi-owner reports the mistake; the page must
// not print the claim twice while it does.
func TestPartitionTrackClaims_OwnershipWins(t *testing.T) {
	c := trackTestClaim("widget", "flow", model.StatusLocked,
		model.TrackRef{ID: "checkout"},
		model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns},
	)
	owned, cited := partitionTrackClaims(&catalog.Catalog{Claims: []model.Claim{c}}, "checkout")
	if len(owned) != 1 || len(cited) != 0 {
		t.Fatalf("partitionTrackClaims = (%d owned, %d cited), want (1, 0)", len(owned), len(cited))
	}
}

// TestBuildTrackSections_DeclarationOrder pins the ordering rule: config order,
// never sorted, because the order an author declared their tracks in is itself
// information and config.TrackIDs and the CLI already read it that way.
func TestBuildTrackSections_DeclarationOrder(t *testing.T) {
	cfg := trackTestConfig(t, "tracks:\n  - id: zeta\n    title: Zeta\n  - id: alpha\n    title: Alpha\n")
	sections := buildTrackSections(&catalog.Catalog{}, cfg, nil)
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if sections[0].ID != "track-zeta" || sections[1].ID != "track-alpha" {
		t.Errorf("sections came out as %q, %q — want declaration order", sections[0].ID, sections[1].ID)
	}
}
