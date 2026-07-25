package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
