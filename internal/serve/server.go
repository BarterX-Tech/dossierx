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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
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
	// 'unsafe-inline' are required because the viewer ships everything inline
	// and nothing external, ever: three <style> blocks (graph.css, style.css,
	// the project theme override), a <script type="application/json"> graph
	// payload, the viewer IIFE, and graph-core.js / graph-ui.js. connect-src
	// 'self' lets the same-origin /api/* endpoints be reached while blocking
	// the exfiltration half of any injected script — load-bearing now for two
	// independent callers, the IIFE (comments, /api/fragment) and graph-ui.js
	// (/api/graph).
	//
	// img-src 'self' IS THE PHASE-D ADDITION, and the exact token matters more
	// than the fact of it. serve is the only human surface, so a claim-body
	// image that cannot load here is the feature not working — but the two
	// cheap ways to make one load are "img-src *" and "img-src data:", and both
	// hand every injected or authored byte on the page an outbound channel: an
	// <img> whose src names an attacker's host exfiltrates by loading, with no
	// fetch and no script involved, which is exactly what connect-src 'self'
	// was chosen to prevent. 'self' re-allows
	// exactly one thing — an image from this origin, which means the
	// allowlisted /claim-assets/ route in claim_assets.go and nothing else.
	// The rest of the policy is unchanged; in particular the comment above
	// about "no external assets, ever" still holds, because 'self' is not
	// external.
	// font-src data: IS THE THEME ADDITION, and like img-src the exact token
	// is the argument. A project theme inlines its own font faces as
	// base64 data: URLs inside the same <style> block the tokens live in
	// (internal/render's themeOverrideCSS), so with default-src 'none' and no
	// font-src at all every one of them is blocked and the viewer silently
	// falls back to a system face — the feature not working, with nothing but
	// a console entry to say so. data: is the narrowest re-allowance that
	// makes it work: it permits exactly the bytes already in the document and
	// no host, so it opens no outbound channel, which is the property
	// connect-src 'self' and img-src 'self' were chosen for. In particular it
	// is NOT 'self' and NOT https:, either of which would let an injected
	// @font-face fetch from somewhere. The "no external assets, ever" comment
	// above still holds: a data: URL is not external.
	//
	// assetCSPValue (claim_assets.go) is unaffected. That policy governs image
	// RESPONSES, which are never documents and load no fonts; it stays
	// "default-src 'none'; sandbox".
	cspValue = "default-src 'none'; img-src 'self'; font-src data:; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'"

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

	// theme is the project's viewer theme, resolved ONCE in New and reused by
	// every rebuild; themeErr is New's resolve failure, held rather than
	// returned (New has no error) and turned into the render error page by
	// renderViewer. See New for why re-resolving per rebuild is wrong.
	theme    *config.ResolvedTheme
	themeErr error

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

	// assets is the computed claim-image allowlist and assetsMu guards it; both
	// are read from handler goroutines. See claim_assets.go, which owns the
	// whole structure — nil means "not built yet", never "no images".
	//
	// assetsCheckedAt is when a freshness check was last ATTEMPTED, and
	// assetsConfirmedAt is when one last SUCCEEDED — when the index was rebuilt,
	// or its fingerprint re-matched the tree. They are two clocks on purpose:
	// the first bounds how often the check runs (it costs a stat-walk of the
	// whole tree, and a page fires one asset request per image, so it is
	// amortised over one watcher tick rather than paid per request); the second
	// bounds how long an index that CANNOT be re-verified may keep authorising
	// requests, because an unparseable claim file makes every check fail and
	// "we tried recently" is then true forever while "it is still correct" is
	// not. claimAssets and keepAssets carry both arguments. assetScans counts
	// the walks purely so a test can prove the amortisation still holds (see
	// AssetTreeScans).
	//
	// It is deliberately NOT invalidated from onChange either: that callback
	// fires on the watcher's NARROWER fingerprint, which cannot see a claim in a
	// dot-directory or one whose filename contains ".tmp-", so wiring it here
	// would be right for most claims and silently wrong for those.
	assetsMu          sync.Mutex
	assets            *assetIndex
	assetsCheckedAt   time.Time
	assetsConfirmedAt time.Time
	assetScans        atomic.Int64
}

// New builds a Server for cfg. version is reported verbatim by GET /api/ping
// (the reachability probe the viewer uses to decide whether the write controls
// mount); the caller resolves it (cmd/dossierx passes resolveVersionInfo's
// value). New does not bind a port — call Listen next.
func New(cfg *config.Config, version string) *Server {
	// The theme is resolved ONCE, here, and every rebuild for the life of
	// this process emits the result. A long-running server that re-read the
	// theme file and its fonts on each rebuild would be reading them while
	// an editor is halfway through writing one, and would answer two requests
	// a second apart with two different viewers for a project whose config
	// never changed. Changing a theme means restarting serve, which is
	// documented; a resolve failure is held and surfaced by renderViewer as
	// the error page, rather than making the server refuse to start with a
	// message nobody is looking at a terminal to read.
	rt, themeErr := config.ResolveTheme(cfg, os.ReadFile)
	s := &Server{
		theme:            rt,
		themeErr:         themeErr,
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

// AssetTreeScans is the number of times the claim-image allowlist has stat-walked
// the claims tree to verify its own freshness. It exists for the one test that
// can catch a return to per-request walking — the walk is O(claims) and a page
// fires one asset request per image, so N requests against an unchanged tree
// must produce a bounded number of scans, never N. Production code has no
// reason to call it.
func (s *Server) AssetTreeScans() int64 { return s.assetScans.Load() }

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

// assertOutputsOutsideClaimsTree is serve's startup guardrail: the build
// directory — where every output the engine writes now lives — MUST sit
// outside the watched claims tree. If it were inside, a render/catalog/store
// write would look like a claim change and drive the watcher in an endless
// re-render loop. config.DecodeConfig already refuses that containment at
// load time, so this is the belt to that brace; it checks the one directory
// rather than three files because the directory is what config guarantees.
func (s *Server) assertOutputsOutsideClaimsTree() error {
	root := s.cfg.ClaimsDir
	if isInsideDir(root, s.cfg.BuildDirPath()) {
		return fmt.Errorf("serve: the build directory (%s) is inside the watched claims_dir (%s); set build_dir or claims_dir so the engine's outputs sit outside it", s.cfg.BuildDirPath(), root)
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
	// The only route that reads a file off disk, and the only non-API route
	// besides the root document. It answers from a computed allowlist rather
	// than from the filesystem — see claim_assets.go for the whole argument.
	mux.HandleFunc(assetRoutePattern, s.handleClaimAsset)
	mux.HandleFunc("GET /api/ping", s.handlePing)
	mux.HandleFunc("GET /api/fragment", s.handleFragment)
	mux.HandleFunc("GET /api/comments", s.handleListComments)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/graph", s.handleGraph)
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
// writes build/viewer/index.html or build/catalog/catalog.json — those are "dossierx render" /
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
	cat.SetReadiness(s.readinessFor(claims))
	if s.themeErr != nil {
		return nil, fmt.Errorf("serve: theme: %w", s.themeErr)
	}
	html, err := render.RenderWithTheme(cat, s.cfg, s.theme)
	if err != nil {
		return nil, fmt.Errorf("serve: render: %w", err)
	}
	return []byte(html), nil
}

func (s *Server) readinessFor(claims []model.Claim) map[string]readiness.Assessment {
	store, _ := lock.LoadStore(s.storePath())
	flags, _ := reaudit.LoadFlagStore(s.flagStorePath())
	return readiness.Compute(claims, store, flags)
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

// storePath and flagStorePath resolve the lock-store and flag-store files
// under the build directory (cfg.LockStorePath / cfg.FlagStorePath), matching
// cmd/dossierx and internal/check. serve reads both (never under their own
// sentinel) only to build the comment Deps, whose review_pending
// recomputation consults real drift/flag state.
func (s *Server) storePath() string {
	return s.cfg.LockStorePath()
}

func (s *Server) flagStorePath() string {
	return s.cfg.FlagStorePath()
}
