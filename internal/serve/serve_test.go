package serve_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/serve"
)

const testVersion = "v-test-1.2.3"

const baseConfig = "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"

// draftClaim is a lint-clean draft card used as the target for "add" (adding a
// thread to a draft never sets review_pending, so the file stays predictable).
func draftClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
}

// lockedClaimWithOpenThread is a locked card carrying one open thread c-aaaaaa,
// the target for reply/resolve/reopen/edit/delete in the admission matrix.
func lockedClaimWithOpenThread(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nreview_pending: true\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-aaaaaa\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: please clarify\n    edited: false\n"
}

// standardFiles is the project every admission test runs against: a draft claim
// to add to, and a locked claim with an open thread to reply/resolve/etc.
func standardFiles() map[string]string {
	return map[string]string{
		"claims/one.yaml":    draftClaim("widget.contract.one"),
		"claims/locked.yaml": lockedClaimWithOpenThread("widget.contract.locked"),
	}
}

// startServer writes a project, starts a real 127.0.0.1 listener serving it,
// and returns the server plus its base URL (http://127.0.0.1:<port>) and the
// project root. A real listener (not httptest) is used so the admission Host/
// Origin checks run against the actual bound port.
func startServer(t *testing.T, cfgBody string, files map[string]string) (srv *serve.Server, base, root string) {
	t.Helper()
	return startServerWatch(t, cfgBody, files, 0, 0)
}

// startServerWatch is startServer with the watcher's cadence exposed. Zero
// intervals keep the production defaults (~500ms/200ms); a short pair drives
// live reload — and, with it, the claim-image allowlist's freshness window,
// which is defined as one watcher tick — on a tight deterministic cycle. Tests
// that assert something BECOMES true after a claim file changes go through this
// rather than sleeping out a real half-second poll.
func startServerWatch(t *testing.T, cfgBody string, files map[string]string, poll, debounce time.Duration) (srv *serve.Server, base, root string) {
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
	if poll > 0 {
		srv.SetWatchIntervals(poll, debounce)
	}
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
	return srv, fmt.Sprintf("http://127.0.0.1:%d", srv.Port()), root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// --- request helpers ---------------------------------------------------------

type reqMod func(*http.Request)

func setHost(h string) reqMod { return func(r *http.Request) { r.Host = h } }
func setHeader(k, v string) reqMod {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

// allowedMutating stamps the Origin and Content-Type a mutating request needs
// to pass admission; individual tests append one "bad" modifier after it to
// exercise a single rejected dimension.
func allowedMutating(base string) []reqMod {
	return []reqMod{
		setHeader("Origin", base),
		setHeader("Content-Type", "application/json"),
	}
}

func do(t *testing.T, method, url, body string, mods ...reqMod) (res *http.Response, raw []byte) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, url, err)
	}
	for _, m := range mods {
		m(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body %s %s: %v", method, url, err)
	}
	return resp, data
}

// --- claim-file byte-identity ------------------------------------------------

func snapshotClaims(t *testing.T, root string) map[string][]byte {
	t.Helper()
	dir := filepath.Join(root, "claims")
	snap := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snap[path] = b
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot claims: %v", err)
	}
	return snap
}

func assertClaimsUnchanged(t *testing.T, before map[string][]byte, root string) {
	t.Helper()
	after := snapshotClaims(t, root)
	if len(before) != len(after) {
		t.Fatalf("claim file set changed: %d before, %d after", len(before), len(after))
	}
	for path, b := range before {
		ab, ok := after[path]
		if !ok {
			t.Fatalf("claim file %s disappeared after a rejected request", path)
		}
		if !bytes.Equal(b, ab) {
			t.Fatalf("claim file %s changed after a REJECTED request:\nbefore:\n%s\nafter:\n%s", path, b, ab)
		}
	}
}

func assertNoCORS(t *testing.T, resp *http.Response, where string) {
	t.Helper()
	if v := resp.Header.Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("%s: unexpected Access-Control-Allow-Origin: %q", where, v)
	}
}

// mutatingEndpoint is one write route, used to sweep the whole admission matrix
// across POST, PATCH, and DELETE.
type mutatingEndpoint struct {
	name    string
	method  string
	path    string
	body    string
	needsCT bool // POST/PATCH require Content-Type; DELETE does not
}

func mutatingEndpoints() []mutatingEndpoint {
	return []mutatingEndpoint{
		{"add", http.MethodPost, "/api/claims/widget.contract.one/comments", `{"body":"x"}`, true},
		{"reply", http.MethodPost, "/api/claims/widget.contract.locked/comments/c-aaaaaa/replies", `{"body":"x"}`, true},
		{"resolve", http.MethodPost, "/api/claims/widget.contract.locked/comments/c-aaaaaa/resolve", `{}`, true},
		{"reopen", http.MethodPost, "/api/claims/widget.contract.locked/comments/c-aaaaaa/reopen", `{}`, true},
		{"edit", http.MethodPatch, "/api/claims/widget.contract.locked/comments/c-aaaaaa", `{"body":"x"}`, true},
		{"delete", http.MethodDelete, "/api/claims/widget.contract.locked/comments/c-aaaaaa", ``, false},
	}
}

// =============================================================================
// (1) Host allowlist — DNS-rebinding defense on EVERY route.
// =============================================================================

func TestAdmission_RebindingHostRejected(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	// GET routes.
	for _, path := range []string{"/", "/api/comments"} {
		before := snapshotClaims(t, root)
		resp, _ := do(t, http.MethodGet, base+path, "", setHost("evil.com"))
		if resp.StatusCode != http.StatusMisdirectedRequest {
			t.Fatalf("GET %s with Host: evil.com: got %d, want 421", path, resp.StatusCode)
		}
		assertNoCORS(t, resp, "rebinding GET "+path)
		assertClaimsUnchanged(t, before, root)
	}

	// Every mutating endpoint: valid Origin/CT, but a rebinding Host. Host is
	// checked first, so all reject 421 and nothing is written.
	for _, ep := range mutatingEndpoints() {
		before := snapshotClaims(t, root)
		mods := append(allowedMutating(base), setHost("evil.com"))
		resp, _ := do(t, ep.method, base+ep.path, ep.body, mods...)
		if resp.StatusCode != http.StatusMisdirectedRequest {
			t.Fatalf("%s %s with Host: evil.com: got %d, want 421", ep.method, ep.name, resp.StatusCode)
		}
		assertNoCORS(t, resp, "rebinding "+ep.name)
		assertClaimsUnchanged(t, before, root)
	}
}

// =============================================================================
// (2) Origin allowlist on mutating methods — null AND absent AND cross rejected.
// =============================================================================

func TestAdmission_OriginRejected(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	cases := []struct {
		name string
		// origin modifiers to apply (in addition to a valid Content-Type).
		originMods []reqMod
	}{
		{"absent", nil}, // no Origin header at all
		{"null", []reqMod{setHeader("Origin", "null")}},
		{"cross", []reqMod{setHeader("Origin", "http://evil.example:6006")}},
	}

	for _, ep := range mutatingEndpoints() {
		for _, tc := range cases {
			before := snapshotClaims(t, root)
			mods := []reqMod{setHeader("Content-Type", "application/json")}
			mods = append(mods, tc.originMods...)
			resp, _ := do(t, ep.method, base+ep.path, ep.body, mods...)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s with %s Origin: got %d, want 403", ep.method, ep.name, tc.name, resp.StatusCode)
			}
			assertNoCORS(t, resp, ep.name+" "+tc.name)
			assertClaimsUnchanged(t, before, root)
		}
	}
}

// =============================================================================
// (3) Content-Type — POST/PATCH require application/json.
// =============================================================================

func TestAdmission_ContentTypeRejected(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	for _, ep := range mutatingEndpoints() {
		if !ep.needsCT {
			continue // DELETE carries no body and needs no Content-Type
		}
		before := snapshotClaims(t, root)
		mods := []reqMod{setHeader("Origin", base), setHeader("Content-Type", "text/plain")}
		resp, _ := do(t, ep.method, base+ep.path, ep.body, mods...)
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("%s %s with text/plain: got %d, want 415", ep.method, ep.name, resp.StatusCode)
		}
		assertNoCORS(t, resp, ep.name+" text/plain")
		assertClaimsUnchanged(t, before, root)
	}
}

// =============================================================================
// (4) Sec-Fetch-Site — cross-site rejected.
// =============================================================================

func TestAdmission_SecFetchSiteRejected(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	// A mutating request that is otherwise fully allowed, rejected purely for
	// its cross-site fetch metadata.
	before := snapshotClaims(t, root)
	mods := append(allowedMutating(base), setHeader("Sec-Fetch-Site", "cross-site"))
	resp, _ := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments", `{"body":"x"}`, mods...)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST add with Sec-Fetch-Site: cross-site: got %d, want 403", resp.StatusCode)
	}
	assertNoCORS(t, resp, "sec-fetch-site POST")
	assertClaimsUnchanged(t, before, root)

	// A GET is gated too when the header is present.
	resp, _ = do(t, http.MethodGet, base+"/", "", setHeader("Sec-Fetch-Site", "cross-site"))
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("GET / with Sec-Fetch-Site: cross-site: got %d, want 403", resp.StatusCode)
	}

	// same-origin and none pass (the header being present must not reject a
	// legitimate same-origin fetch).
	resp, _ = do(t, http.MethodGet, base+"/api/ping", "", setHeader("Sec-Fetch-Site", "same-origin"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ping with Sec-Fetch-Site: same-origin: got %d, want 200", resp.StatusCode)
	}
}

// =============================================================================
// (5) No Access-Control-Allow-Origin on ANY response.
// =============================================================================

func TestAdmission_NoCORSHeaderAnywhere(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	// Successful GETs.
	for _, path := range []string{"/", "/api/ping", "/api/comments", "/api/status"} {
		resp, _ := do(t, http.MethodGet, base+path, "")
		assertNoCORS(t, resp, "GET "+path)
	}
	// A rejected request.
	resp, _ := do(t, http.MethodGet, base+"/", "", setHost("evil.com"))
	assertNoCORS(t, resp, "rejected GET /")
	// A successful mutation.
	resp, _ = do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments", `{"body":"hi"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy POST: got %d, want 200", resp.StatusCode)
	}
	assertNoCORS(t, resp, "happy POST")
	_ = root
}

// GET / renders to memory only: a bare page load must not write the viewer or
// catalog files to disk (those are "dossierx render"/"check"'s output, not the
// serve pipeline's — a truncating per-request write would race and be readable
// half-finished).
func TestRoot_NoDiskWrites(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	for i := 0; i < 3; i++ {
		resp, _ := do(t, http.MethodGet, base+"/", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /: got %d, want 200", resp.StatusCode)
		}
	}
	for _, rel := range []string{filepath.Join("viewer", "index.html"), ".catalog.json"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("GET / wrote %s to disk (stat err=%v); the pipeline must render to memory", rel, err)
		}
	}
}

// GET/HEAD /api/status computes the check Result from memory only. A status
// poll must NOT run check.Run's disk writers (viewer/index.html, .catalog.json)
// or the impl-link Scan that mutates link artifacts: those are the "dossierx
// check" writer, not a read endpoint's job. Because GET and HEAD are safe
// methods that skip the CSRF admission gates, a write-on-read here would let a
// bare, unauthenticated poll truncate the ~65KB viewer and rewrite the catalog
// on every request. This pins that neither file is ever created by the endpoint,
// for BOTH methods, while the endpoint still returns a real status DTO.
func TestStatus_NoDiskWrites(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	// A real poller hits this repeatedly and with both methods.
	for i := 0; i < 3; i++ {
		resp, data := do(t, http.MethodGet, base+"/api/status", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/status: got %d, want 200 (body=%s)", resp.StatusCode, data)
		}
		// The memory-only path still returns a well-formed status DTO.
		var dto struct {
			OK           bool           `json:"ok"`
			OpenComments map[string]int `json:"open_comments"`
			NextSteps    []string       `json:"next_steps"`
		}
		if err := json.Unmarshal(data, &dto); err != nil {
			t.Fatalf("decode status: %v (body=%s)", err, data)
		}

		resp, _ = do(t, http.MethodHead, base+"/api/status", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("HEAD /api/status: got %d, want 200", resp.StatusCode)
		}
	}

	for _, rel := range []string{filepath.Join("viewer", "index.html"), ".catalog.json"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("GET/HEAD /api/status wrote %s to disk (stat err=%v); status must be computed from memory only, never through check.Run's disk writers", rel, err)
		}
	}
}

// =============================================================================
// (6) Happy path accepted end-to-end.
// =============================================================================

func TestHappyPath_AddCommentAccepted(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		`{"body":"hello from the browser"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("happy add: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("happy add: content-type %q, want application/json", ct)
	}
	var out struct {
		ThreadID string `json:"thread_id"`
		Thread   struct {
			ClaimID  string `json:"claim_id"`
			Body     string `json:"body"`
			BodyHTML string `json:"body_html"`
			Status   string `json:"status"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode add response: %v (body=%s)", err, data)
	}
	if !strings.HasPrefix(out.ThreadID, "c-") {
		t.Fatalf("expected a minted thread id, got %q", out.ThreadID)
	}
	if out.Thread.Body != "hello from the browser" {
		t.Fatalf("thread body drift: %q", out.Thread.Body)
	}
	if out.Thread.Status != "open" || out.Thread.ClaimID != "widget.contract.one" {
		t.Fatalf("unexpected thread: %#v", out.Thread)
	}

	// The comment is now discoverable through the list endpoint.
	_, listData := do(t, http.MethodGet, base+"/api/comments", "")
	if !strings.Contains(string(listData), "hello from the browser") {
		t.Fatalf("added comment not found via GET /api/comments: %s", listData)
	}
}

// =============================================================================
// (7) Ping shape.
// =============================================================================

func TestPing_Shape(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())
	resp, data := do(t, http.MethodGet, base+"/api/ping", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("ping content-type %q, want application/json", ct)
	}
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode ping: %v (body=%s)", err, data)
	}
	if out["dossierx"] != "serve" {
		t.Fatalf("ping dossierx=%q, want \"serve\"", out["dossierx"])
	}
	if out["version"] != testVersion {
		t.Fatalf("ping version=%q, want %q", out["version"], testVersion)
	}
}

// =============================================================================
// (8) GET / renders a structurally broken project (never blank) + CSP header.
// =============================================================================

func TestRoot_RendersBrokenProjectsNotBlank(t *testing.T) {
	danglingRef := "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  rests on a ghost.\n" +
		"rests_on:\n  - widget.contract.ghost\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"

	dupA := draftClaim("widget.contract.dup")
	dupB := draftClaim("widget.contract.dup") // same id in a second file

	projects := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"dangling-ref", map[string]string{"claims/one.yaml": danglingRef}, "widget.contract.one"},
		{"duplicate-id", map[string]string{"claims/a.yaml": dupA, "claims/b.yaml": dupB}, "widget.contract.dup"},
	}
	for _, p := range projects {
		t.Run(p.name, func(t *testing.T) {
			_, base, _ := startServer(t, baseConfig, p.files)
			resp, data := do(t, http.MethodGet, base+"/", "")
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET / on %s: got %d, want 200", p.name, resp.StatusCode)
			}
			if csp := resp.Header.Get("Content-Security-Policy"); csp == "" {
				t.Fatalf("GET / on %s: missing CSP header", p.name)
			}
			if len(data) < 500 || !strings.Contains(string(data), p.want) {
				t.Fatalf("GET / on %s produced a blank/incomplete page (%d bytes, want substring %q)", p.name, len(data), p.want)
			}
			if !strings.Contains(strings.ToLower(string(data)), "<!doctype html>") {
				t.Fatalf("GET / on %s: not an HTML document", p.name)
			}
		})
	}
}

// The CSP value is the exact one required for the inline-style/inline-script
// viewer that can still reach same-origin /api/* and load its own claim images.
//
// img-src 'self' WAS ADDED DELIBERATELY at phase D and this assertion was
// updated with it, not around it. serve is the only human surface, so a
// claim-body image had to be able to load here; 'self' is the narrowest token
// that lets one — it re-allows the same-origin /claim-assets/ route
// (claim_assets.go) and nothing else. The two tokens that would also have
// worked, "*" and "data:", each hand every byte on the page an outbound
// channel, which is the exact thing connect-src 'self' is here to deny. See
// TestRoot_CSPDoesNotWidenImgSrc, which is the guard on that.
func TestRoot_CSPValue(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())
	resp, _ := do(t, http.MethodGet, base+"/", "")
	want := "default-src 'none'; img-src 'self'; font-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'"
	if got := resp.Header.Get("Content-Security-Policy"); got != want {
		t.Fatalf("CSP header = %q, want %q", got, want)
	}
}

// TestRoot_CSPDoesNotWidenImgSrc is the guard on the fastest wrong fix. If a
// claim image ever fails to load under serve, the two-character repair is to
// widen img-src, and it would pass every other test in this file. The route in
// claim_assets.go is the repair that is allowed; these tokens are not.
func TestRoot_CSPDoesNotWidenImgSrc(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())
	resp, _ := do(t, http.MethodGet, base+"/", "")
	csp := resp.Header.Get("Content-Security-Policy")
	for _, forbidden := range []string{
		"img-src *", "img-src data:", "img-src https:", "img-src http:",
		"img-src 'unsafe-inline'", "default-src *",
	} {
		if strings.Contains(csp, forbidden) {
			t.Errorf("CSP contains %q; images load from the same-origin /claim-assets/ route, never from a widened img-src: %q", forbidden, csp)
		}
	}
	if !strings.Contains(csp, "img-src 'self'") {
		t.Errorf("CSP has no img-src 'self'; claim-body images cannot load: %q", csp)
	}
}

// The 500 render-error page MUST carry the SAME CSP header the 200 path sets.
// A shell.html override that fails to parse drives handleRoot's error branch,
// which serves a self-contained HTML document — and that document must be
// governed by the identical Content-Security-Policy, so a broken override can
// never downgrade the page to an unprotected one. The override keeps the
// viewer-runtime marker so serve stays writable, isolating this to the
// render-error path rather than read-only degradation.
func TestRoot_RenderErrorHasCSP(t *testing.T) {
	// Invalid template syntax ({{end}} with no matching block) makes
	// render.Render fail at shell-override parse time -> handleRoot 500.
	brokenShell := `<!doctype html><html><head><meta name="dossierx-viewer-runtime"></head>` +
		`<body>{{end}}</body></html>`
	files := map[string]string{
		"claims/one.yaml":            draftClaim("widget.contract.one"),
		"viewer-override/shell.html": brokenShell,
	}
	_, base, _ := startServer(t, overrideConfig, files)

	resp, data := do(t, http.MethodGet, base+"/", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("GET / with a broken shell override: got %d, want 500 (body=%s)", resp.StatusCode, data)
	}
	want := "default-src 'none'; img-src 'self'; font-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'"
	if got := resp.Header.Get("Content-Security-Policy"); got != want {
		t.Fatalf("500 render-error page CSP = %q, want %q (the error branch must set the same CSP as the 200 path)", got, want)
	}
	// Sanity: it is the diagnostic error page, not a blank body.
	if !strings.Contains(string(data), "Viewer render error") {
		t.Fatalf("expected the render-error page body, got: %s", data)
	}
}

// =============================================================================
// (9) XSS: a hostile body is escaped everywhere it is surfaced.
// =============================================================================

func TestXSS_BodyEscapedInResponses(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	const payload = `<img src=x onerror=alert(1)>`
	body, err := json.Marshal(map[string]string{"body": payload})
	if err != nil {
		t.Fatal(err)
	}

	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
		string(body), allowedMutating(base)...)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("XSS add: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	var added struct {
		Thread struct {
			BodyHTML string `json:"body_html"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(data, &added); err != nil {
		t.Fatalf("decode XSS add: %v", err)
	}
	assertEscaped(t, "POST response body_html", added.Thread.BodyHTML)

	// The same escaping holds when the thread is read back from the list.
	_, listData := do(t, http.MethodGet, base+"/api/comments", "")
	var listed struct {
		Comments []struct {
			ClaimID  string `json:"claim_id"`
			BodyHTML string `json:"body_html"`
		} `json:"comments"`
	}
	if err := json.Unmarshal(listData, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, c := range listed.Comments {
		if c.ClaimID == "widget.contract.one" {
			found = true
			assertEscaped(t, "GET /api/comments body_html", c.BodyHTML)
		}
	}
	if !found {
		t.Fatalf("XSS thread not found in list: %s", listData)
	}
	// Belt and braces: the raw wire bytes never carry a live <img onerror=.
	if bytes.Contains(listData, []byte("<img")) {
		t.Fatalf("raw /api/comments bytes contain a live <img tag: %s", listData)
	}
}

func assertEscaped(t *testing.T, where, bodyHTML string) {
	t.Helper()
	if !strings.Contains(bodyHTML, "&lt;img") {
		t.Fatalf("%s: expected escaped &lt;img, got %q", where, bodyHTML)
	}
	if strings.Contains(bodyHTML, "<img") {
		t.Fatalf("%s: live <img markup leaked: %q", where, bodyHTML)
	}
}

// =============================================================================
// (10) Single-flight: N concurrent GET / + a concurrent POST.
// =============================================================================

func TestConcurrency_SingleFlightAndSurvival(t *testing.T) {
	srv, base, _ := startServer(t, baseConfig, standardFiles())

	const n = 30
	var wg sync.WaitGroup
	start := make(chan struct{})

	// N concurrent GET / — all released at once so many land during one render.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resp, data := do(t, http.MethodGet, base+"/", "")
			if resp.StatusCode != http.StatusOK {
				t.Errorf("concurrent GET /: got %d, want 200", resp.StatusCode)
				return
			}
			if !strings.Contains(string(data), "widget.contract.one") {
				t.Errorf("concurrent GET / returned an incomplete document (%d bytes)", len(data))
			}
		}()
	}
	// One concurrent mutation in the same window.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.one/comments",
			`{"body":"survivor"}`, allowedMutating(base)...)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("concurrent POST: got %d, want 200 (body=%s)", resp.StatusCode, data)
		}
	}()

	close(start)
	wg.Wait()

	if runs := srv.RenderRuns(); runs >= n {
		t.Fatalf("single-flight failed: %d renders for %d GET / requests (want fewer)", runs, n)
	}

	// The comment posted during the storm survived.
	_, listData := do(t, http.MethodGet, base+"/api/comments", "")
	if !strings.Contains(string(listData), "survivor") {
		t.Fatalf("comment added during concurrency did not survive: %s", listData)
	}
}

// =============================================================================
// (11) Structured op errors: unknown thread/reply ids -> 404 codes.
// =============================================================================

func TestOpErrors_NotFoundCodes(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	// Reply to a thread id that does not exist.
	resp, data := do(t, http.MethodPost, base+"/api/claims/widget.contract.locked/comments/c-nope99/replies",
		`{"body":"x"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("reply to unknown thread: got %d, want 404 (body=%s)", resp.StatusCode, data)
	}
	assertErrorCode(t, data, "thread_not_found")

	// Edit a reply id that does not exist under a real thread.
	resp, data = do(t, http.MethodPatch, base+"/api/claims/widget.contract.locked/comments/c-aaaaaa?reply=r-nope99",
		`{"body":"x"}`, allowedMutating(base)...)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("edit unknown reply: got %d, want 404 (body=%s)", resp.StatusCode, data)
	}
	assertErrorCode(t, data, "reply_not_found")
}

func assertErrorCode(t *testing.T, data []byte, want string) {
	t.Helper()
	var out struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, data)
	}
	if out.Error != want {
		t.Fatalf("error code = %q, want %q", out.Error, want)
	}
}

// =============================================================================
// (12) GET /api/comments open filter.
// =============================================================================

func TestListComments_OpenFilter(t *testing.T) {
	files := map[string]string{
		"claims/mixed.yaml": "id: widget.contract.mixed\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"body: |\n  mixed threads.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n" +
			"comments:\n" +
			"  - id: c-open01\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: still open\n    edited: false\n" +
			"  - id: c-done01\n    status: resolved\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: all done\n    edited: false\n    resolved_by: human\n    resolved_at: \"2026-07-24T11:00:00Z\"\n",
	}
	_, base, _ := startServer(t, baseConfig, files)

	_, all := do(t, http.MethodGet, base+"/api/comments", "")
	if got := countComments(t, all); got != 2 {
		t.Fatalf("GET /api/comments: %d threads, want 2", got)
	}
	_, openOnly := do(t, http.MethodGet, base+"/api/comments?open=1", "")
	if got := countComments(t, openOnly); got != 1 {
		t.Fatalf("GET /api/comments?open=1: %d threads, want 1", got)
	}
	if !strings.Contains(string(openOnly), "still open") || strings.Contains(string(openOnly), "all done") {
		t.Fatalf("open filter leaked a resolved thread: %s", openOnly)
	}
}

func countComments(t *testing.T, data []byte) int {
	t.Helper()
	var out struct {
		Comments []json.RawMessage `json:"comments"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode comments: %v (body=%s)", err, data)
	}
	return len(out.Comments)
}

// --- theme -------------------------------------------------------------------

// themeServeConfig is a project whose viewer.theme has a per-mode value and one
// inlined font, so the served document exercises every emitted part.
const themeServeConfig = baseConfig +
	"viewer:\n  theme:\n    font-sans: 'Probe, sans-serif'\n" +
	"    light:\n      paper: '#ffffff'\n" +
	"    dark:\n      paper: '#151515'\n" +
	"    fonts:\n      - family: Probe\n        src: fonts/probe.woff2\n"

// probeWoff2 is a file whose first four bytes are the woff2 signature config
// checks. The rule is about the signature, not the font tables.
const probeWoff2 = "wOF2-payload"

// TestRoot_ThemeIsServed is the end-to-end shape check for a themed project:
// the font goes out as a data: URL, the light block covers print, and the dark
// block is scoped to screen so nothing a project set under dark: can reach a
// printed page.
func TestRoot_ThemeIsServed(t *testing.T) {
	files := standardFiles()
	files["fonts/probe.woff2"] = probeWoff2
	_, base, _ := startServer(t, themeServeConfig, files)

	_, body := do(t, http.MethodGet, base+"/", "")
	doc := string(body)
	for _, want := range []string{
		`@font-face{font-family:"Probe";src:url(data:font/woff2;base64,`,
		"@media (prefers-color-scheme: light), print{:root{--paper:#ffffff;}}",
		"@media screen and (prefers-color-scheme: dark){:root{--paper:#151515;}}",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("served document is missing %q", want)
		}
	}
}

// TestRoot_ThemeIsResolvedOnceAtStartup pins the decision in serve.New: the
// theme and its fonts are read once, and every rebuild for the life of the
// process emits that result.
//
// Two things would go wrong without it. A font file being rewritten by a font
// tool or an editor would be read half-written by whichever request arrived
// during the save, so two readers a second apart would get two different
// viewers for a project whose config never changed; and a theme edit would
// take effect on some rebuilds and not others depending on nothing the user
// can see. Changing a theme means restarting serve, which is documented — and
// this test is what makes "restart" a true statement rather than a habit.
func TestRoot_ThemeIsResolvedOnceAtStartup(t *testing.T) {
	files := standardFiles()
	files["fonts/probe.woff2"] = probeWoff2
	_, base, root := startServerWatch(t, themeServeConfig, files, 20*time.Millisecond, 10*time.Millisecond)

	_, before := do(t, http.MethodGet, base+"/", "")
	if !strings.Contains(string(before), "--paper:#151515;") {
		t.Fatalf("served document does not carry the theme's dark value to begin with")
	}

	// Edit the theme's font AND touch a claim, so a rebuild definitely
	// happens: the change under test must survive a real re-render, not
	// merely a cache hit on an idle server.
	writeFile(t, filepath.Join(root, "fonts", "probe.woff2"), "wOF2-edited-after-startup")
	writeFile(t, filepath.Join(root, "claims", "one.yaml"),
		strings.Replace(draftClaim("widget.contract.one"), "a draft claim.", "REBUILT-MARKER.", 1))

	deadline := time.Now().Add(2 * time.Second)
	for {
		_, body := do(t, http.MethodGet, base+"/", "")
		if strings.Contains(string(body), "REBUILT-MARKER") {
			// The served font must still be the bytes New read. Comparing
			// the base64 payload, not a word in the file, because the
			// document is full of prose.
			wantB64 := base64.StdEncoding.EncodeToString([]byte(probeWoff2))
			if !strings.Contains(string(body), wantB64) {
				t.Fatalf("a rebuild dropped or replaced the startup-resolved font; the theme is being re-resolved per rebuild")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the claim edit never reached a rebuild, so this test proved nothing about the theme")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRoot_ThemeFailureIsAnErrorPageNotASilentFallback holds the other half of
// resolving at startup: New has no error return, so a broken theme is held and
// surfaced by the render. A server that started and quietly served an unthemed
// viewer would be the worst of the three options — the reader sees a document
// that looks fine and is not the one the project configured.
func TestRoot_ThemeFailureIsAnErrorPageNotASilentFallback(t *testing.T) {
	files := standardFiles()
	files["fonts/probe.woff2"] = "NOPE-not-a-woff2"
	_, base, _ := startServer(t, themeServeConfig, files)

	resp, body := do(t, http.MethodGet, base+"/", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("GET / with a broken theme = %d, want 500\n%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "theme") {
		t.Errorf("the error page does not mention the theme:\n%s", body)
	}
}
