// Package serve is the local HTTP server behind "dossierx serve": it renders
// the claims viewer from memory and exposes a same-origin JSON write API for
// the "comments on claims" feature, so a human can review claims in a browser
// while agents drive the CLI.
//
// Security posture. The server binds 127.0.0.1 on a random high port and every
// request passes through the admission middleware (see middleware.go) BEFORE
// any handler runs. That middleware is the whole trust boundary: a Host
// allowlist is the sole DNS-rebinding defense, an Origin allowlist plus a
// Content-Type check plus a Sec-Fetch-Site check gate every mutating method,
// and NO CORS header is ever emitted. The API is unauthenticated by design (a
// single local user), so the admission rules — not credentials — are what stop
// a random web page from driving the comment API.
//
// Locking. serve never mutates claim files itself: every write goes through
// internal/comments, which already owns the project-wide claims sentinel
// (one op = one AcquireClaimsLock -> load -> mutate -> SaveClaim -> release).
// The GET / render pipeline runs lock-free and reads from disk, so a page load
// never blocks (or is blocked by) a concurrent comment write. On SIGINT/SIGTERM
// the command context is cancelled and Serve does a graceful http.Server
// Shutdown, which waits for in-flight handlers so any comment op mid-write runs
// its deferred lock release before the process exits (a bare signal death would
// skip that defer).
package serve

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

const (
	// maxBodyBytes caps a mutating request body. Comment/reply bodies are
	// short prose; a megabyte is orders of magnitude more than any real
	// comment and still small enough that a hostile client cannot exhaust
	// memory through the JSON decoder.
	maxBodyBytes = 1 << 20 // 1 MiB

	// cspValue is the Content-Security-Policy sent with GET /. default-src
	// 'none' denies everything not explicitly re-allowed; style-src/script-src
	// 'unsafe-inline' are required because the viewer ships one inline
	// <style> and one inline IIFE (no external assets, ever); connect-src
	// 'self' lets that IIFE reach the same-origin /api/* endpoints while
	// blocking the exfiltration half of any injected script.
	cspValue = "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'"

	// shutdownGrace bounds how long a graceful shutdown waits for in-flight
	// handlers (and thus their deferred lock releases) before forcing exit.
	shutdownGrace = 5 * time.Second
)

// Server is one "dossierx serve" instance: the viewer render pipeline plus the
// comment JSON API for a single project. It is constructed with New, bound to a
// port with Listen, and driven by Serve. The zero value is not usable — always
// go through New.
type Server struct {
	cfg     *config.Config
	version string

	// pipe serializes viewer renders (single-flight); see pipeline.go.
	pipe *pipeline

	// hub fans a "changed" signal out to every live /api/events subscriber; the
	// watcher drives it on each debounced claim-file change. See sse.go.
	hub *hub

	// closing is closed once when Serve begins a graceful shutdown, so live
	// /api/events handlers return promptly instead of making Shutdown wait out
	// the whole grace period. It is created in New and closed by Serve.
	closing chan struct{}

	// pollInterval and debounceInterval are the watcher's poll cadence and
	// trailing-debounce window (see watcher.go). New sets the production
	// defaults; SetWatchIntervals overrides them for tests before Serve.
	pollInterval     time.Duration
	debounceInterval time.Duration

	// httpSrv is built in New (its handler closes over this Server, so the
	// admission middleware reads port at request time). ln and port are set by
	// Listen. port is atomic because the admission middleware reads it from
	// handler goroutines while Listen writes it from the caller's goroutine.
	httpSrv *http.Server
	ln      net.Listener
	port    atomic.Int64

	// readOnly is set once by Serve at startup when the effective shell.html
	// lacks the live-viewer runtime marker (see render.ShellHasViewerRuntime): a
	// custom override that cannot mount the comment UI or consume SSE re-renders.
	// In that mode the mutating comment endpoints refuse with 403 read_only
	// (writes belong on the CLI, which the degradation does not touch); reads —
	// GET /, /api/fragment, /api/comments, /api/status — still work. It is
	// atomic because handler goroutines read it while Serve writes it, though the
	// write happens-before any request is accepted.
	readOnly atomic.Bool

	// warnw is where the startup viewer-degradation WARNING is written; it
	// defaults to os.Stderr (the terminal running serve) and is redirected by
	// SetWarnWriter in tests. Written once by Serve at startup.
	warnw io.Writer
}

// New builds a Server for cfg. version is reported verbatim by GET /api/ping
// (the reachability probe the viewer uses to decide whether the write controls
// mount); the caller resolves it (cmd/dossierx passes resolveVersionInfo's
// value). New does not bind a port — call Listen next.
func New(cfg *config.Config, version string) *Server {
	s := &Server{
		cfg:              cfg,
		version:          version,
		hub:              newHub(),
		closing:          make(chan struct{}),
		pollInterval:     defaultPollInterval,
		debounceInterval: defaultDebounceInterval,
		warnw:            os.Stderr,
	}
	s.pipe = newPipeline(s.renderViewer)
	s.httpSrv = &http.Server{
		Handler: s.admission(s.routes()),
		// ReadHeaderTimeout guards against a slow-loris client dribbling
		// request headers; the body is separately capped by MaxBytesReader in
		// the admission middleware. WriteTimeout is deliberately 0: the
		// /api/events SSE stream is a long-lived response that a write deadline
		// would silently kill, and a finite response is bounded by the client
		// context instead.
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
	}
	return s
}

// Listen binds the server to 127.0.0.1:port (port 0 selects a random high
// port, the default). It must be called once, before Serve. The bound port is
// what the admission middleware validates every request's Host against, so it
// is recorded here for that check.
func (s *Server) Listen(port int) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("serve: listen on 127.0.0.1:%d: %w", port, err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return fmt.Errorf("serve: unexpected listener address type %T", ln.Addr())
	}
	s.ln = ln
	s.port.Store(int64(tcpAddr.Port))
	return nil
}

// Port returns the bound port (0 before Listen).
func (s *Server) Port() int { return int(s.port.Load()) }

// URL is the absolute base URL to print for the user to open, with a trailing
// slash so it names the root document exactly.
func (s *Server) URL() string { return fmt.Sprintf("http://127.0.0.1:%d/", s.Port()) }

// RenderRuns is the number of viewer renders the pipeline has executed. It
// exists for tests to prove single-flight coalescing (N concurrent GET /
// requests run the pipeline fewer than N times); production code has no reason
// to call it.
func (s *Server) RenderRuns() int64 { return s.pipe.runCount() }

// Serve runs the HTTP server until ctx is cancelled (the command wires ctx to
// SIGINT/SIGTERM), then gracefully drains in-flight requests — letting any
// comment op mid-write run its deferred claims-lock release — before returning.
// It returns nil on a clean shutdown and the listener error otherwise. Listen
// must have been called first.
func (s *Server) Serve(ctx context.Context) error {
	if s.ln == nil {
		return errors.New("serve: Serve called before Listen")
	}
	// Startup guardrail: refuse to run if a render/catalog/store output sits
	// inside the watched claims tree (which would drive the watcher in a loop).
	if err := s.assertOutputsOutsideClaimsTree(); err != nil {
		return err
	}

	// Viewer-runtime degradation: decide read-only vs. live BEFORE accepting any
	// request, so the mode a handler observes is fixed for the process lifetime.
	s.applyViewerRuntimeMode()

	// Capture the claims fingerprint synchronously BEFORE accepting requests, so
	// a change that lands between now and the watcher's first poll is still
	// detected (the baseline reflects the pre-serve state). Then poll in the
	// background until ctx is cancelled, feeding the render pipeline and the SSE
	// hub on each debounced change.
	baseline, err := scanFingerprint(s.cfg.ClaimsDir)
	if err != nil {
		baseline = map[string]fileStamp{}
	}
	w := newWatcher(s.cfg.ClaimsDir, s.pollInterval, s.debounceInterval, s.onChange)
	go w.run(ctx, baseline)

	errCh := make(chan error, 1)
	go func() { errCh <- s.httpSrv.Serve(s.ln) }()

	select {
	case <-ctx.Done():
		// Wake any live /api/events handlers first so Shutdown does not wait out
		// the grace period on a long-lived stream, then drain in-flight requests.
		close(s.closing)
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		s.httpSrv.Shutdown(shutCtx) //nolint:errcheck // graceful shutdown on ctx cancel; we return nil regardless
		return nil
	case err := <-errCh:
		close(s.closing)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// onChange is what the watcher calls once per debounced change burst: refresh
// the in-memory render so the next GET / (or an SSE-driven re-fetch) is already
// warm, and signal every /api/events subscriber to re-fetch (the render and
// /api/status). It writes nothing to disk, so it cannot itself re-trigger the
// watcher.
func (s *Server) onChange() {
	s.pipe.refresh()
	s.hub.broadcast()
}

// HubSize reports the number of live /api/events subscribers. It exists for
// tests to prove a disconnect drops the subscription (a leak -race cannot see);
// production code has no reason to call it.
func (s *Server) HubSize() int { return s.hub.size() }

// SetWatchIntervals overrides the watcher's poll and debounce cadence. It lets
// tests drive live reload on a short, deterministic cycle instead of the
// ~500ms/200ms production defaults; it MUST be called before Serve, which reads
// the values when it starts the watcher. Production code keeps the New defaults
// and never calls this.
func (s *Server) SetWatchIntervals(poll, debounce time.Duration) {
	s.pollInterval = poll
	s.debounceInterval = debounce
}

// ReadOnly reports whether serve is running in read-only mode because the
// effective shell.html lacks the live-viewer runtime marker (see
// applyViewerRuntimeMode). Meaningful only after Serve has started; the mutating
// comment endpoints consult the same flag to refuse writes with 403 read_only.
func (s *Server) ReadOnly() bool { return s.readOnly.Load() }

// SetWarnWriter redirects the startup viewer-degradation WARNING away from
// os.Stderr, so a test can assert whether the warning fired. It MUST be called
// before Serve (which writes the warning during startup); production code keeps
// the New default (os.Stderr) and never calls this.
func (s *Server) SetWarnWriter(w io.Writer) { s.warnw = w }

// applyViewerRuntimeMode inspects the effective shell template once at startup
// and, when it lacks the live-viewer runtime marker (a project ships its own
// shell.html override that predates or omits the comment/SSE hooks), flips the
// server into read-only mode and prints a single-line WARNING to warnw. This is
// the "not silent breakage" contract: rather than serving a viewer whose write
// controls can never mount while quietly accepting HTTP writes it can never
// display, serve says so loudly and points at the CLI. A detection error (a
// genuinely unreadable override, which render.Render will also surface) leaves
// the server writable and notes the failure instead of false-degrading.
func (s *Server) applyViewerRuntimeMode() {
	hasRuntime, err := render.ShellHasViewerRuntime(s.cfg)
	if err != nil {
		fmt.Fprintf(s.warnw, "WARNING: dossierx serve could not check the viewer runtime marker: %v\n", err)
		return
	}
	if hasRuntime {
		return
	}
	s.readOnly.Store(true)
	fmt.Fprintf(s.warnw, "WARNING: this project's shell.html override lacks the DossierX live-viewer runtime (no %q marker); serving READ-ONLY — the browser comment UI cannot mount and HTTP comment writes are disabled. Use the CLI (dossierx comment ...) to review, or restore the marker to your override.\n", render.ViewerRuntimeMarker)
}

// assertOutputsOutsideClaimsTree is serve's startup guardrail: the viewer,
// catalog, and lock-store output paths MUST live outside the watched claims
// tree. If one were inside, a render/catalog/store write would look like a claim
// change and drive the watcher in an endless re-render loop. A violation almost
// always means a misconfigured claims_dir (e.g. "."), so serve refuses to start
// rather than spin.
func (s *Server) assertOutputsOutsideClaimsTree() error {
	root := s.cfg.ClaimsDir
	for _, out := range []struct{ name, path string }{
		{"viewer/index.html", s.renderOutPath()},
		{".catalog.json", s.catalogPath()},
		{"lock store", s.storePath()},
	} {
		if isInsideDir(root, out.path) {
			return fmt.Errorf("serve: %s (%s) is inside the watched claims_dir (%s); move claims_dir so the render/catalog/store outputs sit outside it", out.name, out.path, root)
		}
	}
	return nil
}

// routes wires the endpoint set. It uses the method+wildcard patterns of the
// Go 1.22+ ServeMux: "GET /{$}" matches ONLY the root document (a bare "GET /"
// would greedily catch every unmatched path), and {id}/{tid} bind single path
// segments (claim ids and thread ids never contain a slash). Every route is
// wrapped by the admission middleware in New, so the middleware — not any
// handler — is the first thing every request meets.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleRoot)
	mux.HandleFunc("GET /api/ping", s.handlePing)
	mux.HandleFunc("GET /api/fragment", s.handleFragment)
	mux.HandleFunc("GET /api/comments", s.handleListComments)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("POST /api/claims/{id}/comments", s.handleAddThread)
	mux.HandleFunc("POST /api/claims/{id}/comments/{tid}/replies", s.handleReply)
	mux.HandleFunc("POST /api/claims/{id}/comments/{tid}/resolve", s.handleResolve)
	mux.HandleFunc("POST /api/claims/{id}/comments/{tid}/reopen", s.handleReopen)
	mux.HandleFunc("PATCH /api/claims/{id}/comments/{tid}", s.handleEdit)
	mux.HandleFunc("DELETE /api/claims/{id}/comments/{tid}", s.handleDelete)
	return mux
}

// renderViewer is the pipeline's work function: load the CURRENT claims, build
// the catalog, and render the viewer to a byte slice held in memory. It never
// writes viewer/index.html or .catalog.json — those are "dossierx render" /
// "check"'s files, and a truncating per-request write would be readable
// half-finished. catalog.Build cannot fail structurally (duplicate ids /
// dangling refs are lint's concern and still render), so the only real failure
// here is a template/override parse error from render.Render, which the root
// handler turns into a 500 error page rather than a blank viewer.
func (s *Server) renderViewer() ([]byte, error) {
	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		return nil, fmt.Errorf("serve: load claims: %w", err)
	}
	cat, err := catalog.Build(disarmUngatedMockups(claims, s.cfg), s.cfg)
	if err != nil {
		return nil, fmt.Errorf("serve: build catalog: %w", err)
	}
	html, err := render.Render(cat, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("serve: render: %w", err)
	}
	return []byte(html), nil
}

// disarmUngatedMockups returns claims with RawHTMLReviewed cleared on every
// claim the raw-html mockup gate names, so components.MockupHTML takes its
// ESCAPING branch for that claim instead of emitting its bytes as trusted
// markup. claims itself is never mutated (the caller's slice is a fresh load,
// but a copy keeps that an implementation detail rather than a requirement).
//
// WHY THIS EXISTS HERE. render's unescaped path (components.MockupHTML) opens on
// three of the claim's OWN yaml fields — status: locked, raw_html_reviewed:
// true, and a module in mockup_modules — and `status` is on LockedClaimHash's
// deny-list, so a hand-typed "status: locked" is invisible to the content-drift
// rule too. The lint suite is what normally stands between hostile raw_html and
// that branch: "dossierx check" fails at the lint step and never renders. Serve
// does not lint. It loads, builds and renders, so a claim carrying
// <script>...</script> in raw_html — a claim `check` refuses loudly, exit 1 —
// was served VERBATIM to the reviewer who ran "dossierx serve" to go look at it.
// Same-origin script on the serve port passes the admission middleware's
// Origin/Sec-Fetch-Site checks and can drive every comment mutation endpoint,
// including resolve/reopen, the one authority an agent must never hold.
//
// It DISARMS rather than refuses. The viewer must keep rendering a disputed
// project — that is the same argument the ledger gate is built on (a tampered
// claim costs a project its exit status, not its documentation), and it is
// sharper here, because the human is reading the page precisely in order to see
// the claim that is in dispute. Escaped, they see exactly what the file says.
//
// This is also the only caller of lint.MockupGateFindings. The standalone
// "render"/"catalog" verbs that used to enforce it were retired in v0.3.0, which
// left the function with zero callers and its doc comment asserting a gate that
// no longer ran anywhere.
func disarmUngatedMockups(claims []model.Claim, cfg *config.Config) []model.Claim {
	findings := lint.MockupGateFindings(claims, cfg)
	if len(findings) == 0 {
		return claims
	}
	ungated := make(map[string]bool, len(findings))
	for _, f := range findings {
		ungated[f.ClaimID] = true
	}

	out := make([]model.Claim, len(claims))
	copy(out, claims)
	for i := range out {
		if ungated[out[i].ID] {
			out[i].RawHTMLReviewed = false
		}
	}
	return out
}

// storePath and flagStorePath resolve the lock-store and flag-store files under
// cfg.Dir() (absolute), matching cmd/dossierx and internal/check. serve reads
// both (never under their own sentinel) only to build the comment Deps, whose
// review_pending recomputation consults real drift/flag state.
func (s *Server) storePath() string {
	return filepath.Join(s.cfg.Dir(), ".dossierx-lock-store.json")
}

func (s *Server) flagStorePath() string {
	return filepath.Join(s.cfg.Dir(), ".dossierx-flag-store.json")
}

// renderOutPath and catalogPath resolve "dossierx render"/"check"'s viewer and
// catalog output files under cfg.Dir() (absolute), matching cmd/dossierx and
// internal/check. serve never writes them from the GET / pipeline (which renders
// to memory), but the startup guardrail checks they sit outside the watched
// claims tree so a check-driven write can never loop the watcher.
func (s *Server) renderOutPath() string {
	return filepath.Join(s.cfg.Dir(), "viewer", "index.html")
}

func (s *Server) catalogPath() string {
	return filepath.Join(s.cfg.Dir(), ".catalog.json")
}
