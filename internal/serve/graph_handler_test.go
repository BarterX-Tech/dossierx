package serve_test

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------
// GET /api/graph — the refresh button's data source
//
// The endpoint exists because a live session's SSE fragment swap replaces
// only <nav id="nav"> and <main class="content-area">. The graph payload
// block sits outside both, so it is never re-delivered: a claim edited
// mid-session updates the reading view and NOT the graph. The pane's answer
// is to state its payload's age and offer a button that fetches a fresh one.
// This file pins the three properties that button depends on.
// ---------------------------------------------------------------------

const graphPayloadOpenTag = `<script type="application/json" id="dossierx-graph">`

// graphGeneratedAt matches the one field that legitimately differs between
// two encodings of the same corpus. Everything else must be byte-identical,
// which is the whole point of there being exactly one encoder.
var graphGeneratedAt = regexp.MustCompile(`"generated_at":"[^"]*"`)

func normalizeGraphStamp(s string) string {
	return graphGeneratedAt.ReplaceAllString(s, `"generated_at":"STAMP"`)
}

// inlineGraphPayload pulls the payload block's text out of a rendered viewer
// document. The block's contents are graph.Encode's bytes verbatim —
// html/template applies no escaping at all in an application/json script
// context — so no unescaping step belongs here, and adding one would hide the
// very breakout this design guards against.
func inlineGraphPayload(t *testing.T, doc string) string {
	t.Helper()
	start := strings.Index(doc, graphPayloadOpenTag)
	if start < 0 {
		t.Fatalf("rendered document carries no %s block", graphPayloadOpenTag)
	}
	rest := doc[start+len(graphPayloadOpenTag):]
	end := strings.Index(rest, "</script>")
	if end < 0 {
		t.Fatalf("payload block is never closed")
	}
	return rest[:end]
}

// snapshotTree lists every path under root with its size, so a "wrote
// nothing" assertion can name what appeared rather than just failing.
func snapshotTree(t *testing.T, root string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = info.Size()
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

func diffTrees(before, after map[string]int64) []string {
	var changes []string
	for path, size := range after {
		prev, existed := before[path]
		switch {
		case !existed:
			changes = append(changes, "created "+path)
		case prev != size:
			changes = append(changes, "rewrote "+path)
		}
	}
	for path := range before {
		if _, still := after[path]; !still {
			changes = append(changes, "deleted "+path)
		}
	}
	sort.Strings(changes)
	return changes
}

// TestGraphEndpoint covers the response contract and the safety property that
// lets this endpoint hang off a CSRF-exempt GET at all: it writes nothing.
func TestGraphEndpoint(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	// Settle anything the server writes at startup before measuring, so the
	// snapshot isolates THIS request.
	res, _ := do(t, http.MethodGet, base+"/", "")
	res.Body.Close()
	before := snapshotTree(t, root)

	res, body := do(t, http.MethodGet, base+"/api/graph", "")
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/graph = %d, want 200\n%s", res.StatusCode, body)
	}
	if got := res.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json; charset=utf-8")
	}
	if got := res.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want %q — the endpoint's entire contract is freshness", got, "no-store")
	}

	var payload struct {
		Schema      int    `json:"schema"`
		GeneratedAt string `json:"generated_at"`
		Nodes       []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body does not parse as JSON: %v\n%s", err, body)
	}
	if payload.Schema != 1 {
		t.Errorf("schema = %d, want 1", payload.Schema)
	}
	if payload.GeneratedAt == "" {
		t.Errorf("generated_at is empty; the handler must stamp it (graph.Build deliberately does not)")
	}
	if len(payload.Nodes) != 2 {
		t.Errorf("payload carries %d nodes, want 2 (the project's claims)", len(payload.Nodes))
	}

	if changes := diffTrees(before, snapshotTree(t, root)); len(changes) > 0 {
		t.Errorf("GET /api/graph modified the project directory: %v\n"+
			"A read handler must never write viewer/index.html or .catalog.json: GET is CSRF-exempt, "+
			"so a write-on-read lets a bare unauthenticated poll rewrite the project and race the render pipeline.", changes)
	}

	// A HEAD reaches the same handler (Go's ServeMux routes HEAD to a GET
	// pattern) and must be just as inert.
	beforeHead := snapshotTree(t, root)
	resHead, _ := do(t, http.MethodHead, base+"/api/graph", "")
	resHead.Body.Close()
	if resHead.StatusCode != http.StatusOK {
		t.Errorf("HEAD /api/graph = %d, want 200", resHead.StatusCode)
	}
	if changes := diffTrees(beforeHead, snapshotTree(t, root)); len(changes) > 0 {
		t.Errorf("HEAD /api/graph modified the project directory: %v", changes)
	}
}

// TestGraphEndpointMatchesInlinePayload is the one-encoder assertion. If the
// endpoint ever grew its own marshalling — a json.Encoder, a writeJSON detour,
// a re-marshal to "tidy" the escaped output — this is what catches it, because
// there would then be two escaping rules to keep correct instead of one.
func TestGraphEndpointMatchesInlinePayload(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resDoc, docBytes := do(t, http.MethodGet, base+"/", "")
	resDoc.Body.Close()
	if resDoc.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", resDoc.StatusCode)
	}
	inline := inlineGraphPayload(t, string(docBytes))

	resAPI, apiBytes := do(t, http.MethodGet, base+"/api/graph", "")
	resAPI.Body.Close()
	if resAPI.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/graph = %d, want 200\n%s", resAPI.StatusCode, apiBytes)
	}

	gotInline := normalizeGraphStamp(inline)
	gotAPI := normalizeGraphStamp(string(apiBytes))

	if !strings.Contains(gotInline, `"generated_at":"STAMP"`) {
		t.Fatalf("the inline payload carries no generated_at to normalize; this comparison would be vacuous:\n%.300s", inline)
	}
	if gotInline != gotAPI {
		t.Errorf("the endpoint's bytes differ from the inline block's for the same corpus.\ninline: %.500s\napi:    %.500s", gotInline, gotAPI)
	}
}

// TestGraphEndpointReflectsAnEditTheFragmentSwapDoesNot is the endpoint's
// reason for existing, stated as a test: the inline block is delivered once,
// the endpoint is delivered on demand, and after a claim is added only the
// second one knows about it.
func TestGraphEndpointReflectsAnEditTheFragmentSwapDoesNot(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	res, docBytes := do(t, http.MethodGet, base+"/", "")
	res.Body.Close()
	inlineNodes := len(nodeIDs(t, inlineGraphPayload(t, string(docBytes))))

	if err := os.WriteFile(filepath.Join(root, "claims", "three.yaml"),
		[]byte(draftClaim("widget.contract.three")), 0o644); err != nil {
		t.Fatalf("write third claim: %v", err)
	}

	resAPI, apiBytes := do(t, http.MethodGet, base+"/api/graph", "")
	resAPI.Body.Close()
	apiNodes := len(nodeIDs(t, string(apiBytes)))

	if apiNodes != inlineNodes+1 {
		t.Errorf("after adding a claim the endpoint reports %d nodes; the inline block reported %d, so the endpoint should report %d.\n"+
			"The handler must load claims fresh on every request — that is the whole contract the refresh button rests on.",
			apiNodes, inlineNodes, inlineNodes+1)
	}
}

func nodeIDs(t *testing.T, payload string) []string {
	t.Helper()
	var p struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(payload)), &p); err != nil {
		t.Fatalf("payload does not parse: %v\n%.300s", err, payload)
	}
	out := make([]string, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		out = append(out, n.ID)
	}
	return out
}
