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
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
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

	// httpSrv is built in New (its handler closes over this Server, so the
	// admission middleware reads port at request time). ln and port are set by
	// Listen. port is atomic because the admission middleware reads it from
	// handler goroutines while Listen writes it from the caller's goroutine.
	httpSrv *http.Server
	ln      net.Listener
	port    atomic.Int64
}

// New builds a Server for cfg. version is reported verbatim by GET /api/ping
// (the reachability probe the viewer uses to decide whether the write controls
// mount); the caller resolves it (cmd/dossierx passes resolveVersionInfo's
// value). New does not bind a port — call Listen next.
func New(cfg *config.Config, version string) *Server {
	s := &Server{cfg: cfg, version: version}
	s.pipe = newPipeline(s.renderViewer)
	s.httpSrv = &http.Server{
		Handler: s.admission(s.routes()),
		// ReadHeaderTimeout guards against a slow-loris client dribbling
		// request headers; the body is separately capped by MaxBytesReader in
		// the admission middleware. WriteTimeout is deliberately 0: a future
		// SSE stream (Phase 5) is a long-lived response that a write deadline
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
	errCh := make(chan error, 1)
	go func() { errCh <- s.httpSrv.Serve(s.ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutCtx)
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
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
	mux.HandleFunc("GET /api/comments", s.handleListComments)
	mux.HandleFunc("GET /api/status", s.handleStatus)
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
	cat, err := catalog.Build(claims, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("serve: build catalog: %w", err)
	}
	html, err := render.Render(cat, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("serve: render: %w", err)
	}
	return []byte(html), nil
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
