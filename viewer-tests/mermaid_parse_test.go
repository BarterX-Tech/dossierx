package viewertests

// EVERY FLOWCHART "build-order show --format mermaid" EXPORTS PARSES IN THE
// RENDERER THE VIEWER SHIPS.
//
// The export's whole use is to be pasted into a pull request or a document
// that renders mermaid, so the only evidence that matters is the vendored
// parser accepting the text. A live editor is neither offline nor
// repeatable; this test loads a static viewer rendered from a project with a
// locked order (which is what puts window.mermaid on the page), exports the
// project's own orders through the CLI, splits each export on blank lines
// the way (f) says a consumer does, and feeds every chunk to mermaid.parse.
//
// Two vacuity guards. A chunk with no "flowchart TD" line is a FAILED parse,
// not a skipped one: the export prints nothing for an empty phase and for
// the excluded block precisely so this rule can be strict. And a deliberately
// malformed text must be REJECTED, or "everything parsed" is what a parser
// that accepts anything says too.
//
// One documented input from outside: DOSSIERX_TEST_MERMAID_DIR, a directory
// of *.mmd exports (the reference client's, in the acceptance procedure).
// When set, every file there is split and parsed as well; the test fails if
// the directory holds no .mmd file or any file is empty (a redirect creates
// the file before the command runs), and logs how many it parsed.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// mermaidParseVerdict runs mermaid.parse over text in the loaded page and
// returns "ok" or "rejected: <message>".
func mermaidParseVerdict(t *testing.T, ctx context.Context, text string) string {
	t.Helper()
	enc, err := json.Marshal(text)
	if err != nil {
		t.Fatalf("encode chunk: %v", err)
	}
	expr := `(async function(){ try { await window.mermaid.parse(` + string(enc) + `); return 'ok'; } catch (e) { return 'rejected: ' + String(e && e.message ? e.message : e); } })()`
	var verdict string
	runCDP(t, ctx, chromedp.Evaluate(expr, &verdict, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	return verdict
}

// splitFlowcharts splits one export on blank lines and requires every chunk
// to carry a "flowchart TD" line.
func splitFlowcharts(t *testing.T, source, text string) []string {
	t.Helper()
	var chunks []string
	for _, raw := range strings.Split(text, "\n\n") {
		chunk := strings.TrimSpace(raw)
		if chunk == "" {
			continue
		}
		hasKeyword := false
		for _, line := range strings.Split(chunk, "\n") {
			if strings.TrimSpace(line) == "flowchart TD" {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			t.Fatalf("%s: a chunk with no \"flowchart TD\" line is not a diagram, and the export emits nothing for a phase without one:\n%s", source, chunk)
		}
		chunks = append(chunks, chunk+"\n")
	}
	if len(chunks) == 0 {
		t.Fatalf("%s: the export holds no flowchart at all", source)
	}
	return chunks
}

func TestMermaidParsesEveryExportedFlowchart(t *testing.T) {
	p := newBuildOrderProject(t)
	ctx, pe, _ := staticBuildOrderTab(t, p)

	// The vacuity guard first: a parser that accepts these accepts anything.
	for _, bad := range []string{
		"this is not a diagram\n",
		"flowchart TD\n  a[\"unterminated --> b\n",
	} {
		if v := mermaidParseVerdict(t, ctx, bad); v == "ok" {
			t.Fatalf("mermaid.parse accepted a malformed text, so every acceptance below is meaningless:\n%s", bad)
		}
	}

	// Each locked module's export. widget has three non-empty phases, gadget
	// and wide one each — the same counts the tab renders as svgs.
	want := map[string]int{"widget": widgetSVGs, "gadget": gadgetSVGs, "wide": 1}
	parsed := 0
	for _, m := range []string{"widget", "gadget", "wide"} {
		out := p.run("build-order", "show", "--module", m, "--format", "mermaid")
		chunks := splitFlowcharts(t, "module "+m, out)
		if len(chunks) != want[m] {
			t.Errorf("module %s: export holds %d flowchart(s), want %d (one per non-empty phase)", m, len(chunks), want[m])
		}
		for i, c := range chunks {
			if v := mermaidParseVerdict(t, ctx, c); v != "ok" {
				t.Errorf("module %s, flowchart %d: %s\n%s", m, i+1, v, c)
			}
			parsed++
		}
	}
	t.Logf("parsed %d flowchart(s) exported from the local project", parsed)

	if dir := os.Getenv("DOSSIERX_TEST_MERMAID_DIR"); dir != "" {
		files, err := filepath.Glob(filepath.Join(dir, "*.mmd"))
		if err != nil {
			t.Fatalf("DOSSIERX_TEST_MERMAID_DIR %q: %v", dir, err)
		}
		if len(files) == 0 {
			t.Fatalf("DOSSIERX_TEST_MERMAID_DIR %q holds no .mmd file; nothing would be parsed", dir)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			if strings.TrimSpace(string(b)) == "" {
				t.Fatalf("%s is empty: a redirect created it before the export ran, or the export printed nothing", f)
			}
			for i, c := range splitFlowcharts(t, f, string(b)) {
				if v := mermaidParseVerdict(t, ctx, c); v != "ok" {
					t.Errorf("%s, flowchart %d: %s\n%s", f, i+1, v, c)
				}
			}
		}
		t.Logf("parsed %d file(s) from DOSSIERX_TEST_MERMAID_DIR", len(files))
	}
	assertNoPageErrors(t, ctx, pe)
}
