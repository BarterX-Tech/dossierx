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

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/loader"
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
		_, _ = io.WriteString(w, renderErrorPage(err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", cspValue)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(htmlBytes)
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
// status strip can show lint health and open-comment counts. It drives
// internal/check.Status — the MEMORY-ONLY sibling of check.Run — so this read
// endpoint computes the same lint partition, open-comment counts, and
// next-steps advisory WITHOUT any of Run's disk writes: it never truncates
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
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	claims, err := loader.LoadClaims(s.cfg.ClaimsDir)
	if err != nil {
		s.writeInternal(w, fmt.Errorf("load claims: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, statusToDTO(check.Status(claims, s.cfg)))
}

// ---------------------------------------------------------------------
// Mutating endpoints — all delegate to internal/comments
// ---------------------------------------------------------------------

// handleAddThread: POST /api/claims/{id}/comments.
func (s *Server) handleAddThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req bodyRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_actor")
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
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_actor")
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
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_actor")
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
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	actor, err := actorFromString(req.As)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_actor")
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
		writeError(w, http.StatusBadRequest, "invalid_actor")
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
// before the sentinel, could miss a `dossierx flag` that committed concurrently
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
	_ = enc.Encode(v)
}

// writeError writes a structured {"error":"<code>"} body with the given status.
// code is a stable snake_case token the client can branch on (thread_not_found,
// reply_not_found, ...), never a raw message.
func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
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
// file change (loader's optimistic concurrency sentinel) 409 claim_file_changed;
// a store-bricking / non-round-trippable body 400 unsafe_body (BOTH the input
// pre-check's comments.ErrUnsafeBody AND the loader's save-time backstop
// loader.ErrClaimNotRoundTrippable — never a 500 that leaks the internal yaml
// round-trip text); anything unmatched is a 500. It also fields mutatingDeps'
// setup errors (its callers route them here), whose store-load failures fall
// through to the 500 default unchanged.
func (s *Server) writeOpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errReadOnly):
		writeError(w, http.StatusForbidden, "read_only")
	case errors.Is(err, comments.ErrClaimNotFound):
		writeError(w, http.StatusNotFound, "claim_not_found")
	case errors.Is(err, comments.ErrThreadNotFound):
		writeError(w, http.StatusNotFound, "thread_not_found")
	case errors.Is(err, comments.ErrReplyNotFound):
		writeError(w, http.StatusNotFound, "reply_not_found")
	case errors.Is(err, comments.ErrBannerClaim):
		writeError(w, http.StatusUnprocessableEntity, "banner_claim")
	case errors.Is(err, comments.ErrEmptyBody):
		writeError(w, http.StatusBadRequest, "empty_body")
	case errors.Is(err, comments.ErrUnsafeBody), errors.Is(err, loader.ErrClaimNotRoundTrippable):
		writeError(w, http.StatusBadRequest, "unsafe_body")
	case errors.Is(err, comments.ErrInvalidActor):
		writeError(w, http.StatusBadRequest, "invalid_actor")
	case errors.Is(err, comments.ErrRightsDenied):
		writeError(w, http.StatusForbidden, "rights_denied")
	case errors.Is(err, comments.ErrThreadResolved):
		writeError(w, http.StatusConflict, "thread_resolved")
	case errors.Is(err, comments.ErrThreadOpen):
		writeError(w, http.StatusConflict, "thread_open")
	case errors.Is(err, loader.ErrClaimFileChanged):
		writeError(w, http.StatusConflict, "claim_file_changed")
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
// partition, open-comment counts per module, and the next-step hints — the same
// fields check.Result carries for the terminal reporter.
type statusDTO struct {
	OK           bool           `json:"ok"`
	LintErrors   []findingDTO   `json:"lint_errors"`
	LintWarnings []findingDTO   `json:"lint_warnings"`
	OpenComments map[string]int `json:"open_comments"`
	NextSteps    []string       `json:"next_steps"`
}

func statusToDTO(res check.Result) statusDTO {
	open := res.OpenComments
	if open == nil {
		open = map[string]int{}
	}
	next := res.NextSteps
	if next == nil {
		next = []string{}
	}
	return statusDTO{
		OK:           res.OK,
		LintErrors:   findingsToDTO(res.LintErrors),
		LintWarnings: findingsToDTO(res.LintWarnings),
		OpenComments: open,
		NextSteps:    next,
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
