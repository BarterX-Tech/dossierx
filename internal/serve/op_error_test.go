package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

// TestWriteOpError_ClaimFileChangedMapsTo409 pins the serve half of the CAS fix:
// the out-of-band-edit conflict internal/comments now surfaces
// (loader.ErrClaimFileChanged, produced by its SaveClaimIfUnchanged write) must
// map to HTTP 409 claim_file_changed at the serve boundary. That wired mapping
// was previously unreachable because the comment ops wrote with plain SaveClaim;
// this asserts writeOpError routes the (wrapped) sentinel to the 409 code a live
// client branches on to prompt a reload.
func TestWriteOpError_ClaimFileChangedMapsTo409(t *testing.T) {
	rec := httptest.NewRecorder()
	s := &Server{}

	// Wrapped exactly the way a comment op returns it up the stack.
	s.writeOpError(rec, fmt.Errorf("comments: mutate: %w", loader.ErrClaimFileChanged))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", rec.Code, rec.Body.Bytes())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.Bytes())
	}
	if body.Error != "claim_file_changed" {
		t.Fatalf("error code = %q, want claim_file_changed", body.Error)
	}
}

// TestWriteOpError_NotRoundTrippableMapsTo422 pins the serve-side backstop for
// the store-bricking sentinel. loader.ErrClaimNotRoundTrippable means the WHOLE
// claim's STORED bytes (often a pre-existing, hand-edited body — NOT the caller's
// input) will not re-serialize; that is a DISTINCT failure from an unsafe supplied
// body, so it must NOT collapse into 400 unsafe_body. It maps to its own
// 422 claim_not_serializable — honest that the caller's input is not at fault —
// with the JSON body carrying ONLY the stable code, never the prior 500 that
// leaked the raw internal "did not round-trip byte-exact" yaml text.
func TestWriteOpError_NotRoundTrippableMapsTo422(t *testing.T) {
	rec := httptest.NewRecorder()
	s := &Server{}

	// Wrapped exactly the way loader.SaveClaimIfUnchanged returns it up the stack.
	s.writeOpError(rec, fmt.Errorf(`loader: claim "x": marshaled YAML does not re-parse (yaml: line 6: found a tab character where an indentation space is expected): %w`, loader.ErrClaimNotRoundTrippable))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body=%s)", rec.Code, rec.Body.Bytes())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.Bytes())
	}
	if body.Error != "claim_not_serializable" {
		t.Fatalf("error code = %q, want claim_not_serializable (body=%s)", body.Error, rec.Body.Bytes())
	}
	// The JSON body must carry ONLY the stable code, never any leaked internal text.
	for _, leak := range []string{"round-trip", "store-bricking", "loader:", "yaml:", "block scalar", "tab character", "did not re-parse", "unsafe_body"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Fatalf("internal detail %q leaked into the error body: %s", leak, rec.Body.String())
		}
	}
}

// TestWriteOpError_UnsafeBodyMapsTo400 pins the OTHER half of the split: the
// supplied-body pre-check's comments.ErrUnsafeBody stays a 400 unsafe_body (the
// caller's input IS at fault here), distinct from the stored-body 422 above.
func TestWriteOpError_UnsafeBodyMapsTo400(t *testing.T) {
	rec := httptest.NewRecorder()
	s := &Server{}

	s.writeOpError(rec, fmt.Errorf("comments: add: %w", comments.ErrUnsafeBody))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.Bytes())
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%s)", err, rec.Body.Bytes())
	}
	if body.Error != "unsafe_body" {
		t.Fatalf("error code = %q, want unsafe_body (body=%s)", body.Error, rec.Body.Bytes())
	}
}

// TestWriteBriefOpError_TheBriefOnlyErrors pins the brief routes' own error
// mapping (NIT-198 F6, F8), each wrapped as the brief ops return it. A brief
// edited between the op's read and its write (comments.ErrBriefFileChanged,
// which only a write racing the op can produce) is the 409 claim_file_changed
// the viewer turns into "the brief changed on disk"; without it the viewer got
// a raw 500. A frontmatter the engine cannot rewrite in place is the file's
// fault, the 422 claim_not_serializable of its claim twin, with only the code
// in the body. Every other error falls through to the claim routes' mapping.
func TestWriteBriefOpError_TheBriefOnlyErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"edited mid-write", fmt.Errorf("%w (briefs/widget/flow.md)", comments.ErrBriefFileChanged), http.StatusConflict, "claim_file_changed"},
		{"frontmatter not rewritable", fmt.Errorf("comments: briefs/widget/bare.md: %w: the file does not open with a --- frontmatter block", briefs.ErrFrontmatterNotRewritable), http.StatusUnprocessableEntity, "claim_not_serializable"},
		{"no such brief", fmt.Errorf("comments: brief %q: %w", "widget.nope", comments.ErrBriefNotFound), http.StatusNotFound, "brief_not_found"},
		{"a shared error", fmt.Errorf("comments: %w", comments.ErrRightsDenied), http.StatusForbidden, "rights_denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			(&Server{}).writeBriefOpError(rec, tc.err)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body=%s)", rec.Code, tc.status, rec.Body.Bytes())
			}
			if got := strings.TrimSpace(rec.Body.String()); got != "{\n  \"error\": \""+tc.code+"\"\n}" {
				t.Fatalf("body = %s, want only the code %s", got, tc.code)
			}
		})
	}
}
