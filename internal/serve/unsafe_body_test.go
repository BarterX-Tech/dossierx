package serve_test

import (
	"net/http"
	"testing"
)

// A store-bricking leading-whitespace body must be refused at the HTTP boundary
// with a clean 400 unsafe_body — the same clean rejection the CLI gives — and
// must leave every claim file byte-identical (never the bricking bytes on disk).
// This exercises the shared internal/comments body validation THROUGH the serve
// handlers and its error-code mapping.
func TestUnsafeBody_HTTPRejectedWithoutBricking(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"add-thread", http.MethodPost, "/api/claims/widget.contract.one/comments", `{"body":"\ncontract says X"}`},
		{"reply", http.MethodPost, "/api/claims/widget.contract.locked/comments/c-aaaaaa/replies", `{"body":"\n0"}`},
		{"edit-root", http.MethodPatch, "/api/claims/widget.contract.locked/comments/c-aaaaaa", `{"body":"\t\nreal content"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, base, root := startServer(t, baseConfig, standardFiles())
			before := snapshotClaims(t, root)

			resp, data := do(t, tc.method, base+tc.path, tc.body, allowedMutating(base)...)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("%s unsafe body: got %d, want 400 (body=%s)", tc.name, resp.StatusCode, data)
			}
			assertErrorCode(t, data, "unsafe_body")

			// The bricking bytes must never have touched disk.
			assertClaimsUnchanged(t, before, root)
			// And the whole project must still list (i.e. still loads) afterwards.
			if resp2, _ := do(t, http.MethodGet, base+"/api/comments", ""); resp2.StatusCode != http.StatusOK {
				t.Fatalf("%s: GET /api/comments after rejected write = %d, want 200 (dir may be bricked)", tc.name, resp2.StatusCode)
			}
		})
	}
}
