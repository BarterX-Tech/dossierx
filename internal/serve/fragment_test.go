package serve_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/serve"
)

// =============================================================================
// GET /api/fragment — the two subtrees the live client swaps on reload.
// =============================================================================

// fragmentResp mirrors the endpoint's JSON shape for decoding.
type fragmentResp struct {
	Nav     string `json:"nav"`
	Content string `json:"content"`
}

// TestFragment_ReturnsBothSubtrees is the core Phase-5a contract: GET
// /api/fragment answers 200 with both the <nav id="nav"> sidebar subtree and the
// <main class="content-area"> claim-body subtree, each as a self-contained outer
// element, and each is a verbatim slice of the SAME render GET / produces (they
// share the single-flight pipeline, so the two views can never disagree).
func TestFragment_ReturnsBothSubtrees(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resp, data := do(t, http.MethodGet, base+"/api/fragment", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/fragment: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("fragment content-type %q, want application/json", ct)
	}
	assertNoCORS(t, resp, "GET /api/fragment")

	var frag fragmentResp
	if err := json.Unmarshal(data, &frag); err != nil {
		t.Fatalf("decode fragment: %v (body=%s)", err, data)
	}

	// nav subtree: the whole <nav id="nav">…</nav>, and nothing from <main>.
	if !strings.HasPrefix(frag.Nav, `<nav id="nav">`) {
		t.Fatalf("nav subtree must start with <nav id=\"nav\">:\n%s", frag.Nav)
	}
	if !strings.HasSuffix(frag.Nav, "</nav>") {
		t.Fatalf("nav subtree must end at </nav>:\n%s", frag.Nav)
	}
	if strings.Contains(frag.Nav, "<main") {
		t.Fatalf("nav subtree leaked <main> markup (the sidebar lock must ride the nav, not the body):\n%s", frag.Nav)
	}

	// content subtree: the whole <main class="content-area">…</main>, carrying
	// the claim, and not the nav.
	if !strings.HasPrefix(frag.Content, `<main class="content-area">`) {
		t.Fatalf("content subtree must start with <main class=\"content-area\">:\n%s", frag.Content)
	}
	if !strings.HasSuffix(frag.Content, "</main>") {
		t.Fatalf("content subtree must end at </main>:\n%s", tail(frag.Content))
	}
	if strings.Contains(frag.Content, `<nav id="nav">`) {
		t.Fatalf("content subtree leaked the nav:\n%s", frag.Content)
	}
	if !strings.Contains(frag.Content, "widget.contract.one") {
		t.Fatalf("content subtree missing the rendered claim:\n%s", frag.Content)
	}

	// Both subtrees are verbatim slices of the full page (same render).
	_, page := do(t, http.MethodGet, base+"/", "")
	if !strings.Contains(string(page), frag.Nav) {
		t.Fatalf("nav subtree is not a substring of GET / — the fragment and page renders disagree")
	}
	if !strings.Contains(string(page), frag.Content) {
		t.Fatalf("content subtree is not a substring of GET / — the fragment and page renders disagree")
	}
}

// TestFragment_AdmissionGated proves /api/fragment sits behind the same
// admission middleware as every other route: a DNS-rebinding request (a Host
// that is not this server) is rejected 421 before the handler runs.
func TestFragment_AdmissionGated(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resp, _ := do(t, http.MethodGet, base+"/api/fragment", "", setHost("evil.com"))
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Fatalf("GET /api/fragment with Host: evil.com: got %d, want 421", resp.StatusCode)
	}
	assertNoCORS(t, resp, "rebinding GET /api/fragment")

	// Cross-site fetch metadata is rejected too (belt to the Host check).
	resp, _ = do(t, http.MethodGet, base+"/api/fragment", "", setHeader("Sec-Fetch-Site", "cross-site"))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET /api/fragment with Sec-Fetch-Site: cross-site: got %d, want 403", resp.StatusCode)
	}
}

// tail returns the last stretch of s for readable failure messages.
func tail(s string) string {
	const n = 160
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// =============================================================================
// Override degradation — read-only + startup WARNING for a hook-less shell.
// =============================================================================

// marklessShell is a custom shell.html override that keeps the two swap anchors
// but DROPS the dossierx-viewer-runtime marker — the shape of an older copied
// template. serve must detect the missing marker and run read-only.
const marklessShell = `<!doctype html><html lang="en"><head>` +
	`<meta charset="utf-8"><title>degraded</title></head>` +
	`<body class="degraded-no-runtime">` +
	`<nav id="nav">{{range .ModuleGroups}}<button class="sec-tab">{{.ModuleLabel}}</button>{{end}}</nav>` +
	`<main class="content-area">{{range .ModuleGroups}}{{range .Facets}}{{range .Claims}}{{.}}{{end}}{{end}}{{end}}</main>` +
	`</body></html>`

// overrideConfig points the viewer at an override dir alongside the config.
const overrideConfig = baseConfig + "viewer:\n  template_overrides: viewer-override\n"

// syncBuf is a mutex-guarded buffer: the startup WARNING is written from Serve's
// goroutine and read from the test goroutine, so the two accesses must be
// synchronized to stay race-clean.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startServerWarn is startServer with the startup warning redirected into a
// capturable buffer (via SetWarnWriter, before Serve). Because the warning is
// written at the very start of Serve — before the listener accepts anything —
// any request that gets a response proves the warning is already final, so a
// test can read the buffer right after its first round-trip with no extra sync.
func startServerWarn(t *testing.T, cfgBody string, files map[string]string) (srv *serve.Server, base, root string, warn *syncBuf) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "project.config.yaml"), cfgBody)
	for rel, content := range files {
		writeFile(t, filepath.Join(root, rel), content)
	}
	cfg, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	srv = serve.New(cfg, testVersion)
	warn = &syncBuf{}
	srv.SetWarnWriter(warn)
	if err := srv.Listen(0); err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		srv.Serve(ctx) //nolint:errcheck // test server; Serve returns ErrServerClosed on cancel
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return srv, fmt.Sprintf("http://127.0.0.1:%d", srv.Port()), root, warn
}

// TestViewerDegradation_HookLessOverride: a shell.html override lacking the
// runtime marker makes serve (a) warn once at startup, (b) refuse HTTP comment
// writes with 403 read_only leaving claim files untouched, while (c) reads —
// GET / and GET /api/fragment — keep working (read-ONLY, not offline).
func TestViewerDegradation_HookLessOverride(t *testing.T) {
	files := standardFiles()
	files["viewer-override/shell.html"] = marklessShell

	srv, base, root, warn := startServerWarn(t, overrideConfig, files)

	// (b) A fully-allowed mutating request is refused 403 read_only, no write.
	before := snapshotClaims(t, root)
	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		`{"body":"should be refused"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("degraded POST: got %d, want 403 (body=%s)", resp.StatusCode, data)
	}
	assertErrorCode(t, data, "read_only")
	assertClaimsUnchanged(t, before, root)

	// (a) The startup WARNING fired (buffer is final now the POST has answered).
	if w := warn.String(); !strings.Contains(w, "WARNING") || !strings.Contains(w, "READ-ONLY") {
		t.Fatalf("degradation warning did not fire for a hook-less override; warnw=%q", w)
	}
	if !srv.ReadOnly() {
		t.Fatalf("serve should be in read-only mode for a hook-less override")
	}

	// (c) Reads still work: the page and the fragment both render (200).
	if r, _ := do(t, http.MethodGet, base+"/", ""); r.StatusCode != http.StatusOK {
		t.Fatalf("degraded GET /: got %d, want 200", r.StatusCode)
	}
	fResp, fData := do(t, http.MethodGet, base+"/api/fragment", "")
	if fResp.StatusCode != http.StatusOK {
		t.Fatalf("degraded GET /api/fragment: got %d, want 200 (body=%s)", fResp.StatusCode, fData)
	}
	var frag fragmentResp
	if err := json.Unmarshal(fData, &frag); err != nil {
		t.Fatalf("decode degraded fragment: %v", err)
	}
	if !strings.HasPrefix(frag.Nav, `<nav id="nav">`) || !strings.HasPrefix(frag.Content, `<main class="content-area">`) {
		t.Fatalf("degraded fragment lost a swap anchor:\nnav=%s\ncontent=%s", frag.Nav, frag.Content)
	}
}

// TestViewerDegradation_DefaultTemplateStaysLive is the negative control: with
// the embedded default shell (marker present) serve is NOT read-only, prints no
// warning, and accepts a comment write normally.
func TestViewerDegradation_DefaultTemplateStaysLive(t *testing.T) {
	srv, base, _, warn := startServerWarn(t, baseConfig, standardFiles())

	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		`{"body":"live and well"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("default-template POST: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	if srv.ReadOnly() {
		t.Fatalf("default template must not put serve in read-only mode")
	}
	if w := warn.String(); strings.TrimSpace(w) != "" {
		t.Fatalf("default template must emit no degradation warning, got: %q", w)
	}
}
