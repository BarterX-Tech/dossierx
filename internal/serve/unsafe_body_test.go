package serve_test

import (
	"net/http"
	"strings"
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

// TestUnsafeBody_ContentFirstLine_HTTPCleanNoLeak covers the class the old
// leading-whitespace heuristic MISSED: a body whose FIRST line is real CONTENT
// that begins with a TAB ("\tcode\nmore"). The space-indented half of this class
// became storable in v0.4.0 (T6) once the loader emitted at SetIndent(2).
// Under the old code these slipped past validation, the op ran, and the loader
// guard produced a 500 that LEAKED the raw internal yaml/round-trip error in the
// JSON body. They must now be a clean 400 unsafe_body across add/reply/edit, with
// the JSON body carrying only the stable code — no leaked internal text — and no
// claim file bricked.
func TestUnsafeBody_ContentFirstLine_HTTPCleanNoLeak(t *testing.T) {
	// JSON string escapes: \t and \n are literal here (backtick raw strings); the
	// server's JSON decoder turns them into an actual tab/newline body. Both cases
	// are tab-led first CONTENT lines, differing in the continuation.
	bodies := []struct {
		name string
		json string
	}{
		{"tab-led", `{"body":"\tcode line\nmore"}`},
		{"tab-led-multiline", `{"body":"\tone\n\ttwo"}`},
	}
	surfaces := []struct {
		name   string
		method string
		path   string
	}{
		{"add", http.MethodPost, "/api/claims/widget.contract.one/comments"},
		{"reply", http.MethodPost, "/api/claims/widget.contract.locked/comments/c-aaaaaa/replies"},
		{"edit-root", http.MethodPatch, "/api/claims/widget.contract.locked/comments/c-aaaaaa"},
	}
	for _, b := range bodies {
		for _, s := range surfaces {
			t.Run(b.name+"/"+s.name, func(t *testing.T) {
				_, base, root := startServer(t, baseConfig, standardFiles())
				before := snapshotClaims(t, root)

				resp, data := do(t, s.method, base+s.path, b.json, allowedMutating(base)...)
				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("%s %s: got %d, want 400 (body=%s)", b.name, s.name, resp.StatusCode, data)
				}
				assertErrorCode(t, data, "unsafe_body")

				// The JSON body must carry ONLY the stable code — never the internal
				// round-trip / yaml / loader detail a 500 would have leaked.
				for _, leak := range []string{"round-trip", "round trip", "store-bricking", "loader:", "yaml:", "block scalar", "tab character"} {
					if strings.Contains(string(data), leak) {
						t.Fatalf("%s %s: leaked internal text %q in error body: %s", b.name, s.name, leak, data)
					}
				}

				// Never bricked: bytes unchanged and the project still loads.
				assertClaimsUnchanged(t, before, root)
				if resp2, _ := do(t, http.MethodGet, base+"/api/comments", ""); resp2.StatusCode != http.StatusOK {
					t.Fatalf("%s %s: GET /api/comments after rejected write = %d, want 200 (dir may be bricked)", b.name, s.name, resp2.StatusCode)
				}
			})
		}
	}
}
