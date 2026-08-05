package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ---------------------------------------------------------------------
// Every assertion in this file is made against RENDERED OUTPUT, never
// against a Go value. That is the whole reason the file exists.
//
// The three client files are injected as html/template typed values —
// template.CSS for graph.css, template.JS for graph-core.js, graph-ui.js
// and the JSON payload. If any of those four injection sites were typed as
// a plain string instead, html/template would contextually escape the value
// into a quoted JS string literal (or, in the <style> block, replace it
// wholesale with ZgotmplZ) and emit NO error at build time, render time or
// test time. The pane would simply never initialize.
//
// A test asserting on the shellData field — "GraphCoreJS is the bytes of
// graph-core.js" — passes identically in both worlds, because the Go value
// IS correct in both worlds; only what reaches the document differs. So the
// discriminator used below is a byte sequence that CANNOT survive JS-string
// escaping: a source line together with the newlines around it. html/template
// JSON-marshals a plain string into a `"...\n..."` literal, turning every
// real newline into a two-byte \n escape, so a match on "\n<line>\n" proves
// the value arrived as typed, unescaped template.JS.
// ---------------------------------------------------------------------

// graphInjectionWitnesses maps each client file to a line that must appear in
// the rendered document surrounded by its real newlines. They are ordinary
// source lines, deliberately not markers added for the test: a marker could be
// kept alive by a copy-paste while the real injection died.
var graphInjectionWitnesses = map[string]string{
	"graph.css":     "\n#dxgPane {\n",
	"graph-core.js": "\n  root.dossierxGraphCore = api;\n",
	"graph-ui.js":   "\n  var PANE_ID = 'dxgPane';\n",
}

const (
	graphPayloadOpenTag = `<script type="application/json" id="dossierx-graph">`
	graphPaneOpenTag    = `<section id="dxgPane"`
	graphTriggerAttr    = "data-dxg-open"
)

// graphRenderedDoc renders a small, ordinary project and returns the document.
func graphRenderedDoc(t *testing.T) string {
	t.Helper()
	claims := []model.Claim{
		groupedClaim("widget.contract.one", "widget", "contract", model.StatusDraft),
		groupedClaim("widget.contract.two", "widget", "contract", model.StatusLocked),
	}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

// graphPayloadText returns the raw text between the payload block's open tag
// and its closing </script>, failing the test if the block is missing.
func graphPayloadText(t *testing.T, doc string) string {
	t.Helper()
	start := strings.Index(doc, graphPayloadOpenTag)
	if start < 0 {
		t.Fatalf("rendered document has no %s block", graphPayloadOpenTag)
	}
	rest := doc[start+len(graphPayloadOpenTag):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		t.Fatalf("payload block is never closed")
	}
	return rest[:end]
}

// graphOuterHTML returns the outer HTML of the first element beginning with
// openTag, depth-counting same-named tags so a nested element cannot end the
// slice early. It is a local copy of the same walk internal/serve uses for the
// fragment anchors — render must not import serve, and a structural claim
// ("outside div.layout") deserves a structural check rather than an index
// comparison that a later edit could satisfy by accident.
func graphOuterHTML(t *testing.T, doc, openTag, name string) string {
	t.Helper()
	start := strings.Index(doc, openTag)
	if start < 0 {
		t.Fatalf("document has no %s", openTag)
	}
	openPrefix := "<" + name
	closeTag := "</" + name + ">"
	depth := 1
	i := start + len(openTag)
	for depth > 0 {
		rel := strings.Index(doc[i:], closeTag)
		if rel < 0 {
			t.Fatalf("%s is never closed", openTag)
		}
		closeAt := i + rel
		openAt := strings.Index(doc[i:], openPrefix)
		if openAt >= 0 && i+openAt < closeAt {
			depth++
			i = i + openAt + len(openPrefix)
			continue
		}
		depth--
		i = closeAt + len(closeTag)
	}
	return doc[start:i]
}

// TestGraphClientFilesReachTheDocumentVerbatim is the load-bearing typing
// test: it proves GraphCSS is template.CSS and GraphCoreJS / GraphUIJS are
// template.JS by looking for bytes that plain-string escaping would destroy.
func TestGraphClientFilesReachTheDocumentVerbatim(t *testing.T) {
	doc := graphRenderedDoc(t)

	for file, witness := range graphInjectionWitnesses {
		if !strings.Contains(doc, witness) {
			t.Errorf("%s did not reach the document unescaped: %q is absent.\n"+
				"A plain string at this injection site is escaped into a quoted literal with no error anywhere; "+
				"the field must be template.JS (scripts, payload) or template.CSS (stylesheet).", file, witness)
		}
	}

	// ZgotmplZ is html/template's marker for a value it refused to emit in a
	// CSS/URL context — the exact symptom of GraphCSS being a plain string.
	if strings.Contains(doc, "ZgotmplZ") {
		t.Errorf("rendered document contains ZgotmplZ: html/template rejected an injected value, almost certainly an untyped GraphCSS")
	}
}

// TestGraphPayloadBlockParsesAsJSON proves the payload block's contents are
// JSON a browser can read, not a JS string literal containing JSON.
func TestGraphPayloadBlockParsesAsJSON(t *testing.T) {
	doc := graphRenderedDoc(t)
	text := strings.TrimSpace(graphPayloadText(t, doc))

	if strings.HasPrefix(text, `"`) {
		t.Fatalf("payload block starts with a quote — the value was escaped into a JS string literal instead of emitted as JSON:\n%.200s", text)
	}

	var payload struct {
		Schema      int    `json:"schema"`
		GeneratedAt string `json:"generated_at"`
		Nodes       []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload block does not parse as JSON: %v\n%.400s", err, text)
	}
	if payload.Schema != 1 {
		t.Errorf("payload schema = %d, want 1", payload.Schema)
	}
	if payload.GeneratedAt == "" {
		t.Errorf("payload generated_at is empty; the render path must stamp it (graph.Build deliberately does not)")
	}
	if len(payload.Nodes) != 2 {
		t.Fatalf("payload carries %d nodes, want 2 (the rendered project's claims)", len(payload.Nodes))
	}
}

// TestGraphPayloadEscapesAHostileFacet is the XSS assertion, made where it
// matters: in the rendered document, not in the encoder's unit test.
func TestGraphPayloadEscapesAHostileFacet(t *testing.T) {
	const hostile = `</script><img src=x>`

	claims := []model.Claim{groupedClaim("widget.contract."+hostile, "widget", hostile, model.StatusDraft)}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	out, err := Render(cat, &config.Config{Modules: []string{"widget"}, Facets: []string{hostile}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	text := graphPayloadText(t, out)
	if strings.Contains(text, "</script>") {
		t.Fatalf("payload block contains a literal </script> before its own closing tag — the block breaks out and everything after it parses as HTML:\n%.400s", text)
	}
	if !strings.Contains(text, `\u003c/script`) {
		t.Fatalf("payload block does not carry the escaped form \\u003c/script; the hostile string was dropped rather than escaped, which would make this test pass for the wrong reason:\n%.400s", text)
	}

	// Lossless: the pane's detail panel shows the author their real facet name.
	var payload struct {
		Nodes []struct {
			Facet string `json:"facet"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &payload); err != nil {
		t.Fatalf("payload does not parse: %v", err)
	}
	if len(payload.Nodes) != 1 || payload.Nodes[0].Facet != hostile {
		t.Fatalf("hostile facet did not round-trip through the payload block: %#v", payload.Nodes)
	}
}

// TestGraphBlockOrder pins document order (design section 8): graph.css is the
// FIRST <style> block so the two existing style-ordering tests keep passing
// untouched, and the payload precedes graph-core.js, which precedes
// graph-ui.js.
func TestGraphBlockOrder(t *testing.T) {
	doc := graphRenderedDoc(t)

	firstStyle := strings.Index(doc, "<style>")
	graphCSS := strings.Index(doc, graphInjectionWitnesses["graph.css"])
	// A line unique to style.css — graph.css contains no .sec-tab rule, and
	// cannot: every selector in it is #dxg / .dxg- prefixed.
	baseCSS := strings.Index(doc, "\n.sec-tab {\n")
	if firstStyle < 0 || graphCSS < 0 || baseCSS < 0 {
		t.Fatalf("style landmarks missing: firstStyle=%d graphCSS=%d baseCSS=%d", firstStyle, graphCSS, baseCSS)
	}
	if graphCSS < firstStyle {
		t.Fatalf("graph.css lands before the first <style> tag; that is not a style block at all")
	}
	if graphCSS > baseCSS {
		t.Errorf("graph.css is not the FIRST style block (graph at %d, base stylesheet at %d).\n"+
			"Placed last or in the middle it breaks TestRender_ThemeCSSInjectedAfterBaseCSS, which locates the LAST block with strings.LastIndex.", graphCSS, baseCSS)
	}

	payload := strings.Index(doc, graphPayloadOpenTag)
	core := strings.Index(doc, graphInjectionWitnesses["graph-core.js"])
	ui := strings.Index(doc, graphInjectionWitnesses["graph-ui.js"])
	if payload < 0 || core < 0 || ui < 0 {
		t.Fatalf("script landmarks missing: payload=%d core=%d ui=%d", payload, core, ui)
	}
	if !(payload < core && core < ui) {
		t.Errorf("script order is payload=%d core=%d ui=%d; want payload < graph-core.js < graph-ui.js", payload, core, ui)
	}
}

// TestGraphPaneMountsOutsideLayout is design section 1.1 made mechanical: an
// SSE fragment swap replaces <main class="content-area"> and <nav id="nav">,
// both inside div.layout. A pane root inside that subtree would be destroyed
// mid-session together with its zoom, pan, filters and expanded-group set.
func TestGraphPaneMountsOutsideLayout(t *testing.T) {
	doc := graphRenderedDoc(t)

	if !strings.Contains(doc, graphPaneOpenTag) {
		t.Fatalf("rendered document has no %s pane root", graphPaneOpenTag)
	}
	layout := graphOuterHTML(t, doc, `<div class="layout">`, "div")
	if strings.Contains(layout, graphPaneOpenTag) {
		t.Fatalf("the graph pane root mounts INSIDE div.layout; a fragment swap would destroy it and every piece of client-only state it holds")
	}

	strip := strings.Index(doc, `<section id="statusStrip"`)
	pane := strings.Index(doc, graphPaneOpenTag)
	if strip < 0 {
		t.Fatalf("rendered document has no #statusStrip to be a sibling of")
	}
	if pane < strip {
		t.Errorf("the pane root is emitted before #statusStrip (pane=%d strip=%d); design section 8 puts it after the strip, ahead of the script blocks", pane, strip)
	}

	if !strings.Contains(doc, graphPaneOpenTag+" hidden>") {
		t.Errorf("the pane root must ship hidden — the pane is inert until first opened")
	}
}

// TestGraphTriggerIsNotASecTab protects the one class name that would make the
// trigger also switch modules: shell.html's delegated click handler matches
// e.target.closest('.sec-tab').
func TestGraphTriggerIsNotASecTab(t *testing.T) {
	doc := graphRenderedDoc(t)

	nav := graphOuterHTML(t, doc, `<nav id="nav">`, "nav")
	if !strings.Contains(nav, graphTriggerAttr) {
		t.Fatalf("<nav id=\"nav\"> carries no %s trigger:\n%s", graphTriggerAttr, nav)
	}

	// Isolate the single start tag carrying the attribute.
	attrAt := strings.Index(nav, graphTriggerAttr)
	tagStart := strings.LastIndex(nav[:attrAt], "<")
	tagEnd := strings.Index(nav[attrAt:], ">")
	if tagStart < 0 || tagEnd < 0 {
		t.Fatalf("could not isolate the trigger's start tag in:\n%s", nav)
	}
	tag := nav[tagStart : attrAt+tagEnd+1]

	if strings.Contains(tag, "sec-tab") {
		t.Errorf("the graph trigger carries class sec-tab: %s\n"+
			"shell.html's delegated handler matches .sec-tab, so every click would also switch the reading view's module.", tag)
	}
	if !strings.HasPrefix(tag, "<button") {
		t.Errorf("the graph trigger is %q, want a <button>", tag)
	}

	// It must sit above the module tabs, which is where a reader looks first.
	if attrAt > strings.Index(nav, `class="sec-tab"`) && strings.Contains(nav, `class="sec-tab"`) {
		t.Errorf("the graph trigger is emitted after the module tabs; design section 1.2 puts it above them")
	}
}
