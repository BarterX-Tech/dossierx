package render

// The engine-owned type layer: the five inlined faces, the serif token that
// has no theme key, and the nine graph colours the design names.
//
// Every assertion here is about the RENDERED document or the embedded source,
// not about the constants in engine_fonts.go. A face named in a constant and
// never emitted, or emitted without the weight descriptor a variable face
// needs, reads exactly like a working one from the Go side and shows up only
// as a reader on a fallback face.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
)

// TestEngineFacesAreInlinedInTheRenderedViewer pins the five @font-face rules
// a default viewer carries.
//
// The weight descriptor is checked, not just the family: a variable face
// declared without its range collapses to a single instance, and the three
// static IBM Plex Mono faces declared under one family without distinct
// weights would leave the browser synthesising bold from 400 — both of which
// render as "the font loaded" to any check that only looks for the family.
func TestEngineFacesAreInlinedInTheRenderedViewer(t *testing.T) {
	out, err := Render(&catalog.Catalog{}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{
		`@font-face{font-family:"Inter";src:url(data:font/woff2;base64,`,
		`@font-face{font-family:"Source Serif 4";src:url(data:font/woff2;base64,`,
		`@font-face{font-family:"IBM Plex Mono";src:url(data:font/woff2;base64,`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the rendered viewer does not carry %q; the face is not inlined "+
				"and a reader falls through to the next family in the stack", want)
		}
	}

	// Inter and Source Serif 4 are variable and carry a RANGE; the three Plex
	// Mono faces are static and carry one weight each.
	for _, want := range []string{
		`format("woff2");font-weight:100 900;font-style:normal;font-display:swap}`,
		`format("woff2");font-weight:200 900;font-style:normal;font-display:swap}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no @font-face in the rendered viewer ends with %q; a variable "+
				"face without its weight range is one instance, not an axis", want)
		}
	}

	plex := regexp.MustCompile(`@font-face\{font-family:"IBM Plex Mono";src:url\(data:font/woff2;base64,[A-Za-z0-9+/=]+\) format\("woff2"\);font-weight:(\d+);`)
	var weights []string
	for _, m := range plex.FindAllStringSubmatch(out, -1) {
		weights = append(weights, m[1])
	}
	if got, want := strings.Join(weights, ","), "400,500,600"; got != want {
		t.Errorf("the rendered viewer carries IBM Plex Mono at weights [%s], want [%s]. "+
			"No variable Plex Mono is published, so the three the design uses ship as "+
			"three static faces under one family; a missing one is synthesised by the "+
			"browser and does not look like the face.", got, want)
	}

	// The faces these replaced. Geist surviving anywhere in the document means
	// either a stale @font-face or a stack still naming a family no longer
	// inlined — both silently put a reader on a system face.
	if strings.Contains(out, "Geist") {
		i := strings.Index(out, "Geist")
		lo := i - 120
		if lo < 0 {
			lo = 0
		}
		t.Errorf("the rendered viewer still names Geist at byte %d: ...%s...", i, out[lo:i+40])
	}
}

// TestFontSerifIsDeclaredAndConsumed is the whole reason the serif token was
// allowed to be added at all.
//
// --font-serif is engine-owned and outside ThemeTokenAllowlist, so
// TestEveryAllowlistedTokenHasAConsumer — which iterates the allowlist — will
// never see it. A family token with no consumer is indistinguishable from a
// working one until someone opens the viewer, which is exactly the class of
// defect the allowlist's own consumer check exists to prevent.
func TestFontSerifIsDeclaredAndConsumed(t *testing.T) {
	css := embeddedCSS(t, styleTemplatePath)

	if !regexp.MustCompile(`(?m)^\s*--font-serif:\s*\S`).MatchString(css) {
		t.Fatal("style.css declares no --font-serif; the serif role has no token")
	}
	if !strings.Contains(css, `"Source Serif 4"`) {
		t.Error(`--font-serif does not name "Source Serif 4", the family the engine inlines; ` +
			"the stack resolves to a system serif and the inlined face is dead weight")
	}

	reads := consumerPattern("font-serif").FindAllString(stripComments(css), -1)
	if len(reads) == 0 {
		t.Fatal("no rule in style.css reads var(--font-serif); the token is declared " +
			"and nothing is set in it, which renders identically to not having it")
	}

	// And the read must WIN, not merely exist. `.claim-body` is declared twice
	// in style.css (once here, once for width/margin in the narrow layout), so
	// the last font-family declaration on that selector is the one a reader
	// gets. Same model as TestConsumerReadWinsCascade.
	decls := parseDecls(stripComments(css))
	var matches []cssDecl
	for _, d := range decls {
		if d.selector == ".claim-body" && d.property == "font-family" {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("no `font-family` declaration on `.claim-body` in style.css. The serif "+
			"consumer is somewhere else now (reads found: %v); re-point this guard at "+
			"whatever selector carries claim body prose rather than deleting it", reads)
	}
	if last := matches[len(matches)-1]; !strings.Contains(last.text, "var(--font-serif") {
		t.Errorf("the LAST `font-family` declaration on `.claim-body` is %q, which does "+
			"not read --font-serif; the read is shadowed and claim prose renders sans",
			last.text)
	}
}

// TestGraphDesignColourNamesFeedTheRamp pins the join between the design's
// colour vocabulary and the ramp the pane actually draws with.
//
// The nine names are a declaration layer, not a second source of truth: if a
// --dxg-* slot stops reading its --color-*graph-* twin, the design name
// becomes a value nothing paints, which is the state Paper's own dark set was
// in (tokens.md, Disagreements 1 and 2) and the reason this layer exists.
func TestGraphDesignColourNamesFeedTheRamp(t *testing.T) {
	css := stripComments(embeddedCSS(t, graphCSSTemplatePath))

	// slot -> the design name it must read in each of graph.css's two blocks.
	// Spot-checked on facet-1 per the phase brief, then widened to all nine,
	// because a pair that works for slot 1 and was never wired for the other
	// eight is the failure this would otherwise miss.
	for _, slot := range []string{
		"facet-1", "facet-2", "facet-3", "facet-4", "facet-5",
		"other", "cycle", "halo", "governed",
	} {
		dxg := "--dxg-" + slot
		if slot == "other" {
			dxg = "--dxg-facet-other"
		}
		for _, name := range []string{"--color-dark-graph-" + slot, "--color-graph-" + slot} {
			if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `:\s*#[0-9A-Fa-f]{6};`).MatchString(css) {
				t.Errorf("graph.css does not declare %s as a hex literal", name)
			}
			if !strings.Contains(css, dxg+": var("+name+");") {
				t.Errorf("graph.css does not read %s into %s; the design name is declared "+
					"and nothing paints with it", name, dxg)
			}
		}
	}

	// Vacuity guard: a name shaped like the others but absent must not match,
	// or the two checks above accept anything.
	const absent = "--color-graph-facet-99"
	if strings.Contains(css, absent) {
		t.Fatalf("graph.css contains %q; pick a slot number the ramp does not have", absent)
	}
}
