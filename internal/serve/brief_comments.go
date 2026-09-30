package serve

import (
	"errors"
	"net/http"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// brief_comments.go is comment threads on BRIEFS in the served viewer
// (NIT-198): the routes under /api/briefs/{id}/comments, each the twin of a
// claim route, calling the Brief* methods internal/comments added for them
// (NIT-205). The human opens, reads and resolves a thread on a brief the way
// they do on a claim; nothing here locks, restores or confirms a brief.
//
// WHAT IS SHARED WITH THE CLAIM ROUTES, AND WHY THAT IS THE WHOLE POINT.
// Admission (Host, Origin, Sec-Fetch-Site, Content-Type, the body cap) wraps
// every route in New, so these are admitted exactly as the claim writes are.
// The actor is read by the same actorFromString (default human, an explicit
// "agent" honoured), and the rights rule is the one thread operation both
// kinds run in internal/comments: an agent cannot resolve the human's thread
// here either. Read-only serve refuses through the same mutatingDeps. Every
// error the ops share maps through writeOpError to the claim routes' codes.
//
// WHAT DIFFERS.
//   - The subject is a brief, addressed by its slash-free id, <folder>.<slug>.
//     A mux {id} segment cannot carry a path's slash, and internal/briefs'
//     Lookup would otherwise also accept a path; briefRef refuses any ref with
//     a slash or backslash, so the route names a brief by its id only and one
//     spelling reaches one brief.
//   - A brief id can equal a claim id (a project claim is two segments, as a
//     brief id is). The two never meet: a claim route resolves only claims and
//     a brief route only briefs, and the viewer keys each by its own attribute
//     (data-claim-id, data-brief-id).
//   - There is no project-wide GET for briefs: GET /api/comments stays the
//     claim list, byte for byte, and a brief's threads are read from
//     GET /api/briefs/{id}/comments[?open=1].
//   - A thread in the response carries brief_id and path, not claim_id.

// briefCommentDTO is one thread on a brief in a JSON response.
type briefCommentDTO struct {
	BriefID string `json:"brief_id"`
	Path    string `json:"path"`
	threadDTO
}

func briefThreadToDTO(b briefs.Brief, cm model.Comment) briefCommentDTO {
	return briefCommentDTO{BriefID: b.ID, Path: b.Path, threadDTO: threadToDTO(cm)}
}

func findBriefThreadDTO(b briefs.Brief, tid string) (briefCommentDTO, bool) {
	for _, cm := range b.Comments {
		if cm.ID == tid {
			return briefThreadToDTO(b, cm), true
		}
	}
	return briefCommentDTO{}, false
}

// errBriefRefNotAnID is a brief route given a path (or anything with a
// separator) where a brief id belongs.
var errBriefRefNotAnID = errors.New("serve: a brief route takes the brief's <folder>.<slug> id, not its path")

// briefRef is the {id} segment, refused when it is not id-shaped: it holds a
// slash (an escaped %2F reaches here decoded) or a backslash, the separators
// a path has and an id never does.
func briefRef(r *http.Request) (string, error) {
	id := r.PathValue("id")
	if id == "" || strings.ContainsAny(id, `/\`) {
		return "", errBriefRefNotAnID
	}
	return id, nil
}

// writeBriefOpError is writeOpError with the brief-only errors first: a ref
// that names no brief is 404 brief_not_found (the CLI's code for the same
// miss), and a brief file edited between the op's read and its write is 409
// claim_file_changed, the code the CLI's brief comment verbs give it. Every
// other error is one the claim ops share and maps as it does for a claim.
func (s *Server) writeBriefOpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errBriefRefNotAnID), errors.Is(err, comments.ErrBriefNotFound):
		writeError(w, http.StatusNotFound, cliout.CodeBriefNotFound)
	case errors.Is(err, comments.ErrBriefFileChanged):
		writeError(w, http.StatusConflict, cliout.CodeClaimFileChanged)
	default:
		s.writeOpError(w, err)
	}
}

// handleListBriefComments: GET /api/briefs/{id}/comments[?open=1]. A read, so
// it takes no lock and runs in read-only serve too, as GET /api/comments does.
func (s *Server) handleListBriefComments(w http.ResponseWriter, r *http.Request) {
	id, err := briefRef(r)
	if err != nil {
		s.writeBriefOpError(w, err)
		return
	}
	deps := &comments.Deps{Cfg: s.cfg}
	b, threads, err := deps.BriefList(id, r.URL.Query().Get("open") == "1")
	if err != nil {
		s.writeBriefOpError(w, err)
		return
	}
	out := make([]briefCommentDTO, 0, len(threads))
	for _, cm := range threads {
		out = append(out, briefThreadToDTO(b, cm))
	}
	writeJSON(w, http.StatusOK, map[string]any{"comments": out})
}

// briefWrite is the skeleton every brief write shares: the id, the optional
// JSON body, the actor, the mutating deps, then op. withBody is false for
// DELETE, whose actor rides ?as= as the claim route's does.
func (s *Server) briefWrite(w http.ResponseWriter, r *http.Request, withBody bool, op func(deps *comments.Deps, id string, actor model.CommentRole, body string)) {
	id, err := briefRef(r)
	if err != nil {
		s.writeBriefOpError(w, err)
		return
	}
	var req bodyRequest
	if withBody {
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, cliout.CodeBadRequest)
			return
		}
	} else {
		req.As = r.URL.Query().Get("as")
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
	op(deps, id, actor, req.Body)
}

// handleBriefAddThread: POST /api/briefs/{id}/comments.
func (s *Server) handleBriefAddThread(w http.ResponseWriter, r *http.Request) {
	s.briefWrite(w, r, true, func(deps *comments.Deps, id string, actor model.CommentRole, body string) {
		b, tid, err := deps.BriefAdd(id, actor, body)
		if err != nil {
			s.writeBriefOpError(w, err)
			return
		}
		dto, _ := findBriefThreadDTO(b, tid)
		writeJSON(w, http.StatusOK, map[string]any{"thread_id": tid, "thread": dto})
	})
}

// handleBriefReply: POST /api/briefs/{id}/comments/{tid}/replies.
func (s *Server) handleBriefReply(w http.ResponseWriter, r *http.Request) {
	tid := r.PathValue("tid")
	s.briefWrite(w, r, true, func(deps *comments.Deps, id string, actor model.CommentRole, body string) {
		b, rid, err := deps.BriefReply(id, tid, actor, body)
		if err != nil {
			s.writeBriefOpError(w, err)
			return
		}
		dto, _ := findBriefThreadDTO(b, tid)
		writeJSON(w, http.StatusOK, map[string]any{"reply_id": rid, "thread": dto})
	})
}

// handleBriefResolve: POST /api/briefs/{id}/comments/{tid}/resolve. The human
// resolves; an agent resolving the human's thread is 403 rights_denied.
func (s *Server) handleBriefResolve(w http.ResponseWriter, r *http.Request) {
	s.briefStateChange(w, r, (*comments.Deps).BriefResolve)
}

// handleBriefReopen: POST /api/briefs/{id}/comments/{tid}/reopen.
func (s *Server) handleBriefReopen(w http.ResponseWriter, r *http.Request) {
	s.briefStateChange(w, r, (*comments.Deps).BriefReopen)
}

func (s *Server) briefStateChange(w http.ResponseWriter, r *http.Request, op func(*comments.Deps, string, string, model.CommentRole) (briefs.Brief, error)) {
	tid := r.PathValue("tid")
	s.briefWrite(w, r, true, func(deps *comments.Deps, id string, actor model.CommentRole, _ string) {
		b, err := op(deps, id, tid, actor)
		if err != nil {
			s.writeBriefOpError(w, err)
			return
		}
		dto, _ := findBriefThreadDTO(b, tid)
		writeJSON(w, http.StatusOK, map[string]any{"thread": dto})
	})
}

// handleBriefEdit: PATCH /api/briefs/{id}/comments/{tid}[?reply=<rid>].
func (s *Server) handleBriefEdit(w http.ResponseWriter, r *http.Request) {
	tid := r.PathValue("tid")
	replyID := r.URL.Query().Get("reply")
	s.briefWrite(w, r, true, func(deps *comments.Deps, id string, actor model.CommentRole, body string) {
		b, err := deps.BriefEdit(id, tid, replyID, actor, body)
		if err != nil {
			s.writeBriefOpError(w, err)
			return
		}
		dto, _ := findBriefThreadDTO(b, tid)
		writeJSON(w, http.StatusOK, map[string]any{"thread": dto})
	})
}

// handleBriefDelete: DELETE /api/briefs/{id}/comments/{tid}[?reply=<rid>][&as=].
func (s *Server) handleBriefDelete(w http.ResponseWriter, r *http.Request) {
	tid := r.PathValue("tid")
	replyID := r.URL.Query().Get("reply")
	s.briefWrite(w, r, false, func(deps *comments.Deps, id string, actor model.CommentRole, _ string) {
		b, err := deps.BriefDelete(id, tid, replyID, actor)
		if err != nil {
			s.writeBriefOpError(w, err)
			return
		}
		if replyID == "" {
			writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "thread_id": tid})
			return
		}
		dto, _ := findBriefThreadDTO(b, tid)
		writeJSON(w, http.StatusOK, map[string]any{"deleted_reply": replyID, "thread": dto})
	})
}
