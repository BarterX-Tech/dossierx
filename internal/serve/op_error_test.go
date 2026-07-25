package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// TestWriteOpError_NotRoundTrippableMapsTo400 pins the serve-side backstop for the
// store-bricking sentinel: should loader.ErrClaimNotRoundTrippable ever reach the
// serve boundary (a body that slipped the input pre-check, or any future
// divergence between the pre-check and the save-time guard), it must map to a
// CLEAN 400 unsafe_body — the SAME code the input pre-check yields — never the
// prior 500 that leaked the raw internal "did not round-trip byte-exact" text in
// the JSON body.
func TestWriteOpError_NotRoundTrippableMapsTo400(t *testing.T) {
	rec := httptest.NewRecorder()
	s := &Server{}

	// Wrapped exactly the way loader.SaveClaimIfUnchanged returns it up the stack.
	s.writeOpError(rec, fmt.Errorf(`loader: claim "x": body did not round-trip byte-exact: %w`, loader.ErrClaimNotRoundTrippable))

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
	// The JSON body must carry ONLY the stable code, never the leaked internal text.
	if got := rec.Body.String(); strings.Contains(got, "round-trip") || strings.Contains(got, "store-bricking") {
		t.Fatalf("internal round-trip detail leaked into the error body: %s", got)
	}
}
