package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/graph"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// ---------------------------------------------------------------------
// GET / — the viewer, rendered from memory
// ---------------------------------------------------------------------

// handleRoot serves the viewer for the CURRENT claims. It ALWAYS returns 200
// with the rendered document when rendering succeeds — it never gates the page
// on lint (duplicate ids, dangling refs, unlocked tags all still render; those
// are data for the terminal / status endpoint, not a reason to blank the page).
// The single genuine failure is a template/override parse error from
// render.Render, which becomes a self-contained 500 HTML page (not a bare
// string) plus a stderr line, so a broken override is diagnosable in the tab.
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	htmlBytes, err := s.pipe.get(r.Context())
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Client went away mid-render; nothing to send.
			return
		}
		fmt.Fprintf(os.Stderr, "serve: GET /: render failed: %v\n", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The 500 error page is a self-contained HTML document too, so it MUST
		// carry the same CSP as the 200 path — a broken override must never
		// downgrade the page to one served without the policy.
		w.Header().Set("Content-Security-Policy", cspValue)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, renderErrorPage(err)) //nolint:errcheck // headers already sent; a client write error mid-response is unrecoverable
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", cspValue)
	w.WriteHeader(http.StatusOK)
	w.Write(htmlBytes) //nolint:errcheck // headers already sent; a client write error mid-response is unrecoverable
}

// renderErrorPage is the minimal, self-contained 500 body shown when the viewer
// cannot render. It embeds only inline styles and escapes the error text.
func renderErrorPage(err error) string {
	return `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>dossierx serve — render error</title></head>` +
		`<body style="font-family:system-ui,sans-serif;max-width:44rem;margin:3rem auto;padding:0 1rem;line-height:1.5">` +
		`<h1>Viewer render error</h1>` +
		`<p>The claims viewer could not be rendered. Fix the problem below and reload.</p>` +
		`<pre style="white-space:pre-wrap;background:#f4f4f5;color:#111;padding:1rem;border-radius:8px;overflow:auto">` +
		html.EscapeString(err.Error()) +
		`</pre></body></html>`
}

// ---------------------------------------------------------------------
// GET /api/ping — reachability probe target
// ---------------------------------------------------------------------

// handlePing answers the viewer's "is a live serve behind this page?" probe.
// The exact shape is a contract: the client requires body.dossierx === "serve"
// (plus a JSON content type) before it mounts the write controls, so a static
// file:// viewer — whose relative fetch cannot reach any server — correctly
// stays read-only.
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"dossierx": "serve",
		"version":  s.version,
	})
}

// ---------------------------------------------------------------------
// GET /api/fragment — the two subtrees the live client swaps on reload
// ---------------------------------------------------------------------

// fragmentDTO carries the two viewer subtrees the SSE client re-fetches and
// swaps in place after a "changed" event: Nav is the <nav id="nav"> sidebar
// module list (which holds the per-module 🔒 lock state, so it lives OUTSIDE
// .content-area and must be swapped alongside the body), and Content is the
// <main class="content-area"> claim body. Each value is the element's full outer
// HTML, so the client can replace outerHTML and keep the #nav / .content-area
// selectors valid, then re-run initViewer() against the fresh DOM.
type fragmentDTO struct {
	Nav     string `json:"nav"`
	Content string `json:"content"`
}

// handleFragment serves the nav + content-area subtrees from the SAME
// single-flight in-memory render as GET / (pipe.get) — never a disk read/write —
// so a fragment fetch and a full page load coalesce onto one render and always
// agree. The two subtrees are sliced out server-side rather than shipping the
// whole document for the client to DOMParser-slice, purely for testability: the
// endpoint's contract is exactly "these two elements," which an httptest can
// assert without a browser. If the effective shell dropped either anchor (a
// custom override), that is a 500 with a diagnostic rather than a half fragment.
func (s *Server) handleFragment(w http.ResponseWriter, r *http.Request) {
	htmlBytes, err := s.pipe.get(r.Context())
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Client went away mid-render; nothing to send.
			return
		}
		s.writeInternal(w, fmt.Errorf("serve: GET /api/fragment: render failed: %w", err))
		return
	}
	doc := string(htmlBytes)
	nav, okNav := extractElement(doc, `<nav id="nav">`, "nav")
	content, okContent := extractElement(doc, `<main class="content-area">`, "main")
	if !okNav || !okContent {
		s.writeInternal(w, fmt.Errorf("serve: GET /api/fragment: shell is missing a swap anchor (nav=%t content=%t); a shell.html override must keep <nav id=\"nav\"> and <main class=\"content-area\">", okNav, okContent))
		return
	}
	writeJSON(w, http.StatusOK, fragmentDTO{Nav: nav, Content: content})
}

// extractElement returns the outer HTML of the first element in doc that begins
// with openTag (a literal opening tag such as `<nav id="nav">`), spanning that
// opening tag through its matching `</name>` close. It depth-counts same-named
// open/close tags, so a hypothetical nested <name> cannot end the slice early;
// the default shell nests neither <nav> nor <main>, and claim bodies are
// markdown-escaped (a literal "<main>" in a body renders as "&lt;main&gt;"), so
// in practice the first close is the match and the counter is defensive. It
// returns ("", false) when openTag is absent or no balanced close is found.
func extractElement(doc, openTag, name string) (string, bool) {
	start := strings.Index(doc, openTag)
	if start < 0 {
		return "", false
	}
	openPrefix := "<" + name
	closeTag := "</" + name + ">"
	depth := 1
	i := start + len(openTag)
	for depth > 0 {
		rel := strings.Index(doc[i:], closeTag)
		if rel < 0 {
			return "", false // unbalanced: no matching close
		}
		closeAt := i + rel
		if openAt := indexTagStart(doc, openPrefix, i); openAt >= 0 && openAt < closeAt {
			depth++
			i = openAt + len(openPrefix)
			continue
		}
		depth--
		i = closeAt + len(closeTag)
	}
	return doc[start:i], true
}

// indexTagStart returns the index of the next real opening tag beginning with
// openPrefix (e.g. "<nav") at or after from — one whose following byte is a tag
// boundary — so "<nav" cannot spuriously match a longer element name. It returns
// -1 when none remain.
func indexTagStart(doc, openPrefix string, from int) int {
	for i := from; i < len(doc); {
		rel := strings.Index(doc[i:], openPrefix)
		if rel < 0 {
			return -1
		}
		idx := i + rel
		after := idx + len(openPrefix)
		if after < len(doc) && isTagBoundary(doc[after]) {
			return idx
		}
		i = idx + len(openPrefix)
	}
	return -1
}

// isTagBoundary reports whether c can immediately follow an element name in a
// start tag: a space, the tag close, a self-close slash, or ASCII whitespace.
func isTagBoundary(c byte) bool {
	switch c {
	case ' ', '>', '/', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------
// GET /api/comments[?open=1] — list threads across all claims
// ---------------------------------------------------------------------

// handleListComments returns every comment thread in the project (each carrying
// its claim id and both raw body and server-rendered body_html), optionally
// filtered to open threads with ?open=1. body_html is produced only by
// markdown.Render, the one safe renderer, so a hostile body is inert here.
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("load claims: %w", err))
		return
	}
	openOnly := r.URL.Query().Get("open") == "1"

	out := []commentDTO{}
	for _, c := range claims {
		for _, cm := range c.Comments {
			if openOnly && cm.Status != model.CommentStatusOpen {
				continue
			}
			out = append(out, commentToDTO(c.ID, cm))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": out})
}

// ---------------------------------------------------------------------
// GET /api/status — structured check result for the status strip
// ---------------------------------------------------------------------

// handleStatus returns the check pipeline's Result as JSON so the viewer's
// status strip can show lint health, LOCK-LEDGER INTEGRITY, and open-comment
// counts. It drives internal/check.Status — the MEMORY-ONLY sibling of
// check.Run — so this read endpoint computes the same lint partition, the same
// ledger-gate verdict, the same open-comment counts, and the same next-steps
// advisory WITHOUT any of Run's disk writes: it never truncates
// viewer/index.html or .catalog.json (os.WriteFile) and never runs the
// per-request impl-link Scan that mutates link artifacts. That matters because
// GET and HEAD are safe methods that skip the CSRF admission gates, so a
// write-on-read here would let a bare, unauthenticated poll rewrite the viewer
// on every request and race the GET / render pipeline mid-write; the disk
// writers belong to "dossierx check" / serve startup, never a read handler.
// Claims are loaded fresh but NOT reconciled (a GET must not rewrite claim
// files); the lint partition and OpenComments are exact regardless, and
// NextSteps is best-effort. A lint failure is data to display, not a reason to
// fail the endpoint.
//
// The ledger gate is read-only in the strongest sense the feature has: it opens
// the lock store and the comment digest store and compares, and it never
// creates, adopts or repairs either one. That is the property that makes it
// safe to hang off a CSRF-exempt GET at all — an endpoint that "fixed" the
// ledger on a poll would record whatever the files say NOW as approved, which
// is the outcome a tamperer wants. Nothing here changes that; the endpoint only
// stops HIDING what the gate found (see statusToDTO).
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("load claims: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, statusToDTO(check.Status(claims, s.cfg)))
}

// ---------------------------------------------------------------------
// GET /api/graph — a freshly built claims-graph payload
// ---------------------------------------------------------------------

// handleGraph returns the claims-graph payload for the CURRENT claims. It is
// the graph analogue of handleStatus: load claims fresh, build the catalog the
// same way renderViewer does, compute, write. Nothing else.
//
// WHY IT EXISTS. The graph payload is inlined into the viewer document in a
// <script type="application/json"> block that sits OUTSIDE both subtrees an
// SSE fragment swap replaces, so a live session never re-receives it. A claim
// edited mid-session updates the reading view and not the graph. Rather than
// pretend otherwise, the pane states its payload's generation time and — only
// against a live serve — offers a refresh button, which this endpoint answers.
//
// WHY IT IS SAFE ON A CSRF-EXEMPT GET. It writes nothing. GET and HEAD skip
// the admission gates that guard the mutating routes, so a read handler that
// touched disk would let a bare unauthenticated poll rewrite the project and
// race the render pipeline mid-write. This one never calls os.WriteFile, never
// reconciles a claim file, and never runs the impl-link scan that mutates link
// artifacts — exactly the argument handleStatus's doc comment already makes.
//
// WHY IT DOES NOT GO THROUGH writeJSON. The payload has exactly ONE encoder,
// graph.Encode, and therefore exactly one escaping rule to keep correct.
// encoding/json's DEFAULT HTML escaping is the only thing standing between an
// author-authored claim label and a </script> breakout in the inline block, and
// under serve no lint has run to constrain what an author wrote. Routing these
// bytes through a second marshaller would create a second place for that rule
// to be got wrong, and the two encodings would drift silently — which is why
// a test asserts these bytes are byte-identical to the inline block's, modulo
// generated_at.
//
// WHY IT DOES NOT USE graphPayloadJSON's CACHE SEAM. That seam exists to avoid
// re-deriving a payload nothing asked to change. This endpoint's entire
// contract is that something did.
//
// WHY IT DOES NOT LINT. Neither does any other serve path — serve loads,
// builds and renders. A corpus with dangling edges genuinely can reach this
// handler; graph.Build drops those edges and counts them into
// dropped.unresolved_edges, and the pane shows a notice rather than silently
// drawing a smaller graph than the data describes.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("load claims: %w", err))
		return
	}
	// The same normalisation renderViewer applies, so the endpoint and the
	// inline block always describe the same corpus.
	cat, err := catalog.Build(disarmUngatedMockups(claims, s.cfg), s.cfg)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("build catalog: %w", err))
		return
	}

	p := graph.Build(cat, s.cfg)
	// graph.Build reads no clock — it leaves GeneratedAt empty and every
	// caller stamps it. Here the value is request time, because freshness is
	// the whole point; on the render path it is the render's own timestamp, so
	// the payload agrees with line 1 and the sidebar footer to the instant.
	p.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	body, err := graph.Encode(p)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("encode graph payload: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// no-store, not no-cache: a cached graph payload is a stale answer to the
	// one question this endpoint exists to answer.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(body) //nolint:errcheck // headers already sent; a client write error mid-response is unrecoverable
}

// ---------------------------------------------------------------------
// Mutating endpoints — all delegate to internal/comments
// ---------------------------------------------------------------------

// handleAddThread: POST /api/claims/{id}/comments.
func (s *Server) handleAddThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req bodyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeBadRequest)
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
		return
	}
	deps, err := s.mutatingDeps()
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	claim, tid, err := deps.Add(id, actor, req.Body)
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	dto, _ := findThreadDTO(claim, tid)
	writeJSON(w, http.StatusOK, map[string]any{"thread_id": tid, "thread": dto})
}

// handleReply: POST /api/claims/{id}/comments/{tid}/replies.
func (s *Server) handleReply(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tid := r.PathValue("tid")
	var req bodyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeBadRequest)
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
		return
	}
	deps, err := s.mutatingDeps()
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	claim, rid, err := deps.Reply(id, tid, actor, req.Body)
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	dto, _ := findThreadDTO(claim, tid)
	writeJSON(w, http.StatusOK, map[string]any{"reply_id": rid, "thread": dto})
}

// handleResolve: POST /api/claims/{id}/comments/{tid}/resolve.
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	s.threadStateChange(w, r, func(deps *comments.Deps, id, tid string, actor model.CommentRole) (model.Claim, error) {
		return deps.Resolve(id, tid, actor)
	})
}

// handleReopen: POST /api/claims/{id}/comments/{tid}/reopen.
func (s *Server) handleReopen(w http.ResponseWriter, r *http.Request) {
	s.threadStateChange(w, r, func(deps *comments.Deps, id, tid string, actor model.CommentRole) (model.Claim, error) {
		return deps.Reopen(id, tid, actor)
	})
}

// threadStateChange is the shared resolve/reopen skeleton: both take an
// optional {"as":...} body (default human, the browser composer's actor),
// mutate the named thread, and return it. An empty JSON body is fine — resolve
// and reopen carry no content.
func (s *Server) threadStateChange(w http.ResponseWriter, r *http.Request, op func(*comments.Deps, string, string, model.CommentRole) (model.Claim, error)) {
	id := r.PathValue("id")
	tid := r.PathValue("tid")
	var req bodyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeBadRequest)
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
		return
	}
	deps, err := s.mutatingDeps()
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	claim, err := op(deps, id, tid, actor)
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	dto, _ := findThreadDTO(claim, tid)
	writeJSON(w, http.StatusOK, map[string]any{"thread": dto})
}

// handleEdit: PATCH /api/claims/{id}/comments/{tid}[?reply=<rid>].
func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tid := r.PathValue("tid")
	replyID := r.URL.Query().Get("reply")
	var req bodyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeBadRequest)
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
		return
	}
	deps, err := s.mutatingDeps()
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	claim, err := deps.Edit(id, tid, replyID, actor, req.Body)
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	dto, _ := findThreadDTO(claim, tid)
	writeJSON(w, http.StatusOK, map[string]any{"thread": dto})
}

// handleDelete: DELETE /api/claims/{id}/comments/{tid}[?reply=<rid>]. DELETE
// carries no JSON body (admission does not require a Content-Type for it), so
// the optional actor rides a ?as= query param, defaulting to human.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tid := r.PathValue("tid")
	replyID := r.URL.Query().Get("reply")
	actor, err := actorFromString(r.URL.Query().Get("as"))
	if err != nil {
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
		return
	}
	deps, err := s.mutatingDeps()
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	claim, err := deps.Delete(id, tid, replyID, actor)
	if err != nil {
		s.writeOpError(w, err)
		return
	}
	if replyID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "thread_id": tid})
		return
	}
	dto, _ := findThreadDTO(claim, tid)
	writeJSON(w, http.StatusOK, map[string]any{"deleted_reply": replyID, "thread": dto})
}

// ---------------------------------------------------------------------
// Comment Deps + shared request/response plumbing
// ---------------------------------------------------------------------

// errReadOnly is returned by mutatingDeps when serve is running read-only (the
// effective shell.html lacks the live-viewer runtime marker; see
// Server.applyViewerRuntimeMode). writeOpError maps it to 403 read_only, so a
// hand-crafted write is refused with a clear code even though the degraded viewer
// would never send one itself. The CLI comment path is unaffected.
var errReadOnly = errors.New("serve: read-only (viewer runtime unavailable)")

// mutatingDeps builds the comments.Deps a mutating op runs against, mirroring
// cmd/dossierx's mutatingCommentDeps: the lock- and flag-store are supplied as
// PATHS, not pre-loaded snapshots, so each op re-reads them fresh inside the
// claims sentinel before recomputing review_pending — a snapshot loaded here,
// before the sentinel, could miss a `dossierx claim flag` that committed concurrently
// and orphan it with review_pending:false. Claims is left unset — every mutating
// op re-reads claims fresh inside the claims lock. In read-only mode (degraded
// viewer) it short-circuits with errReadOnly before touching any store.
func (s *Server) mutatingDeps() (*comments.Deps, error) {
	if s.readOnly.Load() {
		return nil, errReadOnly
	}
	return &comments.Deps{
		Cfg:           s.cfg,
		LockStorePath: s.storePath(),
		FlagStorePath: s.flagStorePath(),
	}, nil
}

// bodyRequest is the JSON body a mutating comment request may carry: the
// message body (required by add/reply/edit; ignored by resolve/reopen) and an
// optional actor role.
type bodyRequest struct {
	As   string `json:"as"`
	Body string `json:"body"`
}

// decodeJSONBody decodes an optional JSON body. An empty body (EOF) is not an
// error — resolve/reopen legitimately send none — leaving the target zero. A
// present-but-malformed body is an error the caller maps to 400.
func decodeJSONBody(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

// actorFromString validates the requested role. An empty value defaults to
// human — the browser composer's fixed actor — so the common case (a reviewer
// clicking in the viewer) needs no explicit role, while an explicit "agent" is
// still honored.
func actorFromString(as string) (model.CommentRole, error) {
	switch as {
	case "":
		return model.CommentRoleHuman, nil
	case string(model.CommentRoleHuman), string(model.CommentRoleAgent):
		return model.CommentRole(as), nil
	default:
		return "", fmt.Errorf("serve: invalid actor %q", as)
	}
}

// writeJSON encodes v as an indented JSON response with the given status.
// encoding/json's default HTML escaping stays on, so any <, >, & inside a
// body_html value is written as a \u00xx escape in the wire bytes — belt to the
// markdown renderer's braces.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck // status + headers already written; a mid-response encode error is unrecoverable
}

// writeError writes a structured {"error":"<code>"} body with the given status.
// code is a stable snake_case token the client can branch on (thread_not_found,
// reply_not_found, ...), never a raw message.
//
// The codes themselves now live in internal/cliout, shared with the CLI's
// output envelope, so the browser and the terminal answer the same question
// with the same word. The WIRE FORMAT here is unchanged — a bare
// {"error":"<code>"} object, not an envelope — because the viewer's fetch()
// calls parse it as it is, and reshaping a working API to match a new one would
// be churn for its own sake.
func writeError(w http.ResponseWriter, status int, code cliout.Code) {
	writeJSON(w, status, map[string]string{"error": string(code)})
}

// writeInternal logs err to stderr and returns a 500 with the error text. This
// is a local single-user tool, so surfacing the message aids debugging in the
// terminal running serve; it is only reached by genuinely unexpected failures
// (store load, encode) that admission and the op error mapping do not cover.
func (s *Server) writeInternal(w http.ResponseWriter, err error) {
	fmt.Fprintf(os.Stderr, "serve: internal error: %v\n", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// writeOpError maps an internal/comments op error to an HTTP status + stable
// error code. Read-only mode becomes 403 read_only; unknown thread/reply ids
// become 404 thread_not_found / reply_not_found; a rights denial 403; a
// wrong-state op (reply to resolved, double resolve/reopen) 409; an out-of-band
// file change (loader's optimistic concurrency sentinel) 409 claim_file_changed.
// The two store-safety failures are kept DISTINCT: the input pre-check's
// comments.ErrUnsafeBody (the caller's SUPPLIED body cannot be stored) is 400
// unsafe_body, while the loader's save-time backstop
// loader.ErrClaimNotRoundTrippable (the WHOLE claim's STORED bytes will not
// re-serialize — usually a pre-existing, hand-edited body, NOT the caller's input,
// so 400 would misattribute fault) is 422 claim_not_serializable. Both carry ONLY
// their stable code — never a 500 that leaks the internal yaml round-trip text.
// Anything unmatched is a 500. It also fields mutatingDeps' setup errors (its
// callers route them here), whose store-load failures fall through to the 500
// default unchanged.
func (s *Server) writeOpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errReadOnly):
		writeError(w, http.StatusForbidden, cliout.CodeReadOnly)
	case errors.Is(err, comments.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, cliout.CodeClaimNotFound)
	case errors.Is(err, comments.ErrThreadNotFound):
		writeError(w, http.StatusNotFound, cliout.CodeThreadNotFound)
	case errors.Is(err, comments.ErrReplyNotFound):
		writeError(w, http.StatusNotFound, cliout.CodeReplyNotFound)
	case errors.Is(err, comments.ErrBannerClaim):
		writeError(w, http.StatusUnprocessableEntity, cliout.CodeBannerClaim)
	case errors.Is(err, comments.ErrEmptyBody):
		writeError(w, http.StatusBadRequest, cliout.CodeEmptyBody)
	case errors.Is(err, comments.ErrUnsafeBody):
		// The caller's SUPPLIED body cannot be stored: their input IS at fault.
		writeError(w, http.StatusBadRequest, cliout.CodeUnsafeBody)
	case errors.Is(err, loader.ErrClaimNotRoundTrippable):
		// The whole claim's STORED bytes will not re-serialize (usually a
		// pre-existing, hand-edited body) — NOT the caller's input. Distinct code,
		// and honest that this is not a 400-class bad request.
		writeError(w, http.StatusUnprocessableEntity, cliout.CodeClaimNotSerializable)
	case errors.Is(err, comments.ErrInvalidActor):
		writeError(w, http.StatusBadRequest, cliout.CodeInvalidActor)
	case errors.Is(err, comments.ErrRightsDenied):
		writeError(w, http.StatusForbidden, cliout.CodeRightsDenied)
	case errors.Is(err, comments.ErrThreadResolved):
		writeError(w, http.StatusConflict, cliout.CodeThreadResolved)
	case errors.Is(err, comments.ErrThreadOpen):
		writeError(w, http.StatusConflict, cliout.CodeThreadOpen)
	case errors.Is(err, loader.ErrClaimFileChanged):
		writeError(w, http.StatusConflict, cliout.CodeClaimFileChanged)
	case errors.Is(err, comments.ErrCommentDigestDrift):
		// 409: the STORED comment block disagrees with the integrity record, so
		// this write would launder a hand edit. Same family as
		// claim_file_changed — the server's picture of the file is not the
		// file — but not the same cause, and the recovery is version control
		// rather than a reload-and-retry, so it keeps its own code.
		writeError(w, http.StatusConflict, cliout.CodeCommentDigestDrift)
	case errors.Is(err, comments.ErrCommentDigestUnavailable):
		// 503: the integrity store the write path depends on cannot be opened,
		// so the op was refused BEFORE anything was written. It is a service
		// condition rather than a fault in the request — the same request will
		// succeed once the store is restored — and it must not fall through to
		// the 500 default: an unclassified error is what a client retries, and
		// retrying used to be the thing that appended the same thread again.
		writeError(w, http.StatusServiceUnavailable, cliout.CodeCommentDigestUnavailable)
	default:
		s.writeInternal(w, err)
	}
}

// ---------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------

// replyDTO is one reply in a JSON response: the raw Body (for edit-textarea
// prefill) plus BodyHTML (server-rendered, the only thing the client assigns to
// innerHTML).
type replyDTO struct {
	ID       string `json:"id"`
	Author   string `json:"author"`
	Created  string `json:"created"`
	Body     string `json:"body"`
	BodyHTML string `json:"body_html"`
	Edited   bool   `json:"edited"`
}

// commentDTO is one thread in a JSON response, carrying its owning claim id and
// the same raw+rendered body pairing as replyDTO.
type commentDTO struct {
	ClaimID    string     `json:"claim_id"`
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	Author     string     `json:"author"`
	Created    string     `json:"created"`
	Body       string     `json:"body"`
	BodyHTML   string     `json:"body_html"`
	Edited     bool       `json:"edited"`
	Replies    []replyDTO `json:"replies"`
	ResolvedBy string     `json:"resolved_by,omitempty"`
	ResolvedAt string     `json:"resolved_at,omitempty"`
	ReopenedBy string     `json:"reopened_by,omitempty"`
	ReopenedAt string     `json:"reopened_at,omitempty"`
}

// commentToDTO renders cm (on claimID) to its wire form, producing body_html
// for the thread root and every reply via markdown.Render — the sole markdown
// renderer, which escapes hostile HTML so an <img onerror=...> body arrives as
// inert &lt;img text, never live markup.
func commentToDTO(claimID string, cm model.Comment) commentDTO {
	replies := make([]replyDTO, 0, len(cm.Replies))
	for _, rp := range cm.Replies {
		replies = append(replies, replyDTO{
			ID:       rp.ID,
			Author:   string(rp.Author),
			Created:  rp.Created,
			Body:     rp.Body,
			BodyHTML: string(markdown.Render(rp.Body)),
			Edited:   rp.Edited,
		})
	}
	return commentDTO{
		ClaimID:    claimID,
		ID:         cm.ID,
		Status:     cm.Status,
		Author:     string(cm.Author),
		Created:    cm.Created,
		Body:       cm.Body,
		BodyHTML:   string(markdown.Render(cm.Body)),
		Edited:     cm.Edited,
		Replies:    replies,
		ResolvedBy: string(cm.ResolvedBy),
		ResolvedAt: cm.ResolvedAt,
		ReopenedBy: string(cm.ReopenedBy),
		ReopenedAt: cm.ReopenedAt,
	}
}

// findThreadDTO returns the wire form of thread tid on claim, if present. It is
// absent only after a whole-thread delete (the caller returns a deleted marker
// instead).
func findThreadDTO(claim model.Claim, tid string) (commentDTO, bool) {
	for _, cm := range claim.Comments {
		if cm.ID == tid {
			return commentToDTO(claim.ID, cm), true
		}
	}
	return commentDTO{}, false
}

// findingDTO is one lint finding in the status response.
type findingDTO struct {
	Lint     string `json:"lint"`
	ClaimID  string `json:"claim_id"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// statusDTO is the status strip's data: overall OK, the lint error/warning
// partition, the lock-ledger gate's findings, open-comment counts per module,
// and the next-step hints — the same fields check.Result carries for the
// terminal reporter.
type statusDTO struct {
	OK           bool           `json:"ok"`
	LintErrors   []findingDTO   `json:"lint_errors"`
	LintWarnings []findingDTO   `json:"lint_warnings"`
	OpenComments map[string]int `json:"open_comments"`
	NextSteps    []string       `json:"next_steps"`

	// LedgerFindings is the lock-ledger gate's verdict, and it is on this DTO
	// for the reason the whole gate exists: v0.3.0's promise is that a LOCKED
	// claim cannot change without an approval record, and the VIEWER is the only
	// surface the human who relies on that promise ever looks at. check.Status
	// has always computed this slice — it REPORTS where check.Run REFUSES, so
	// that a tampered project still renders the claim under dispute instead of
	// going dark exactly when someone needs to read it — but the strip's DTO
	// dropped it, so serve answered a tampered project with `ok: true` and five
	// clean fields. The agent saw the finding in its JSON envelope; the human saw
	// a green page. That asymmetry is the failure mode the ledger is meant to
	// close, reintroduced one layer up.
	//
	// It is NOT folded into lint_errors and does not gain a severity, matching
	// cmd/dossierx's checkData: a lint finding is advice about how a claim is
	// written, a ledger finding is a refusal about whether it was approved, and
	// the strip presents the two differently because a reader must not have to
	// tell them apart by squinting at a rule name. lock.Finding already carries
	// snake_case JSON tags (it is written to the same machine contract this
	// endpoint is), so unlike lint.Finding it needs no projection type — reusing
	// it keeps the browser's field names and the CLI envelope's the same strings.
	//
	// Always an array, never null, on the same terms as lint_errors and
	// next_steps: the client ranges over it without a null test.
	LedgerFindings []lock.Finding `json:"ledger_findings"`

	// BuildOrders is every module's build-order state with staleness recomputed
	// live, beside the ledger findings and for the same reason: a locked build
	// order is the second class of locked artifact in a project, and a human
	// reading the viewer had no way to see that the approved implementation
	// sequence had gone stale under the claims they were reading. It is an array,
	// never null.
	BuildOrders []check.BuildOrderReport `json:"build_orders"`
}

// statusToDTO projects a check.Result into the strip's wire form.
//
// OK is the one field that is NOT copied through. check.Result.OK is
// lint-driven on purpose (see check.Result.LedgerFindings: Status must keep
// reporting a project whose ledger is in dispute, so a ledger finding may not
// blank the page), but `ok` on this endpoint is the strip's HEADLINE — the
// single boolean a reader takes as "is this project sound?" — and a green
// headline sitting on top of a populated ledger_findings array is a worse lie
// than the omission was. So the two are conjoined here, at the presentation
// seam, where failing closed costs nothing: the page still renders, every claim
// is still readable, and the strip says why it is red. That is also exactly the
// verdict the CLI reaches from the same Result (checkStoppedAt maps a non-empty
// LedgerFindings to stopped_at "ledger" with ok:false), so a human reading the
// viewer and an agent reading the envelope are never told different things.
//
// This projection is a pure read: it neither loads nor writes the lock and
// comment-digest stores (check.Status already read them, read-only), because
// "re-record whatever the files say now" is precisely what an attacker would
// want an integrity check to do — and GET is CSRF-exempt, so a bare
// unauthenticated poll would be the trigger.
func statusToDTO(res check.Result) statusDTO {
	open := res.OpenComments
	if open == nil {
		open = map[string]int{}
	}
	next := res.NextSteps
	if next == nil {
		next = []string{}
	}
	ledger := res.LedgerFindings
	if ledger == nil {
		ledger = []lock.Finding{}
	}
	orders := res.BuildOrders
	if orders == nil {
		orders = []check.BuildOrderReport{}
	}
	return statusDTO{
		OK:             res.OK && len(ledger) == 0,
		LintErrors:     findingsToDTO(res.LintErrors),
		LintWarnings:   findingsToDTO(res.LintWarnings),
		OpenComments:   open,
		NextSteps:      next,
		LedgerFindings: ledger,
		BuildOrders:    orders,
	}
}

func findingsToDTO(findings []lint.Finding) []findingDTO {
	out := make([]findingDTO, 0, len(findings))
	for _, f := range findings {
		out = append(out, findingDTO{
			Lint:     f.LintName,
			ClaimID:  f.ClaimID,
			Message:  f.Message,
			Severity: string(f.Severity),
		})
	}
	return out
}
