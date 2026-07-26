package serve_test

import (
	"net/http"
	"strings"
	"testing"
)

// poisonStoredBodyClaim hand-authors a claim whose STORED prose body is a
// tab-led first content line: yaml.v3 v3.0.1 emits it as a block scalar it cannot
// re-parse, so the WHOLE claim will not round-trip on the next save. The
// double-quoted flow scalar loads cleanly (the exact state a user reaches by
// hand-editing a claim YAML), yet ANY mutating op that re-saves the claim trips
// the loader's store-bricking guard (loader.ErrClaimNotRoundTrippable). It
// carries an open thread (c-poison1) and a resolved thread (c-poison2) so every
// verb has a valid target, and the poison lives in the CLAIM body so even a
// whole-thread delete still re-saves the bad bytes.
func poisonStoredBodyClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: \"\\tstored prose yaml cannot round-trip\\nsecond line\"\n" +
		"governed_by:\n  type: none\n  reason: fixture poison claim\n" +
		"comments:\n" +
		"  - id: c-poison1\n    status: open\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: open thread on a poison claim\n    edited: false\n" +
		"  - id: c-poison2\n    status: resolved\n    author: human\n    created: \"2026-07-24T10:00:00Z\"\n    body: resolved thread on a poison claim\n    edited: false\n    resolved_by: human\n    resolved_at: \"2026-07-24T11:00:00Z\"\n"
}

func poisonFiles() map[string]string {
	return map[string]string{
		"claims/poison.yaml": poisonStoredBodyClaim("widget.contract.poison"),
	}
}

// A mutating op on a claim whose PRE-EXISTING stored body will not round-trip is
// NOT the caller's input at fault — so it must NOT collapse into 400 unsafe_body
// (whose de-indent-your-body guidance is wrong here) and must NEVER be the prior
// 500 that leaked the raw internal yaml/round-trip text. Across ALL six mutating
// verbs — the body-less resolve/reopen/delete AS WELL AS add/reply/edit — it maps
// to a DISTINCT 422 claim_not_serializable carrying only the stable code, and the
// claim file is never bricked (bytes unchanged, the dir still loads).
func TestClaimNotSerializable_HTTP422NoLeakNoBrick(t *testing.T) {
	surfaces := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"add", http.MethodPost, "/api/claims/widget.contract.poison/comments", `{"body":"a fine new thread"}`},
		{"reply", http.MethodPost, "/api/claims/widget.contract.poison/comments/c-poison1/replies", `{"body":"a fine reply"}`},
		{"resolve", http.MethodPost, "/api/claims/widget.contract.poison/comments/c-poison1/resolve", `{}`},
		{"reopen", http.MethodPost, "/api/claims/widget.contract.poison/comments/c-poison2/reopen", `{}`},
		{"edit", http.MethodPatch, "/api/claims/widget.contract.poison/comments/c-poison1", `{"body":"a fine edit"}`},
		{"delete", http.MethodDelete, "/api/claims/widget.contract.poison/comments/c-poison1", ``},
	}
	for _, s := range surfaces {
		t.Run(s.name, func(t *testing.T) {
			_, base, root := startServer(t, baseConfig, poisonFiles())
			before := snapshotClaims(t, root)

			// Precondition: the poison claim LOADS clean (list is 200) — the failure
			// is at save time, not load time.
			if resp, _ := do(t, http.MethodGet, base+"/api/comments", ""); resp.StatusCode != http.StatusOK {
				t.Fatalf("precondition: poison claim should load clean, GET /api/comments = %d", resp.StatusCode)
			}

			resp, data := do(t, s.method, base+s.path, s.body, allowedMutating(base)...)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("%s stored-body: got %d, want 422 (not 400 unsafe_body, not 500) (body=%s)", s.name, resp.StatusCode, data)
			}
			assertErrorCode(t, data, "claim_not_serializable")

			// The JSON body carries ONLY the stable code — never leaked internal text,
			// never the wrong unsafe_body code.
			for _, leak := range []string{"round-trip", "round trip", "store-bricking", "loader:", "yaml:", "block scalar", "tab character", "did not re-parse", "unsafe_body", "de-indent"} {
				if strings.Contains(string(data), leak) {
					t.Fatalf("%s: leaked internal/wrong text %q in error body: %s", s.name, leak, data)
				}
			}

			// Never bricked: bytes unchanged and the project still loads.
			assertClaimsUnchanged(t, before, root)
			if resp2, _ := do(t, http.MethodGet, base+"/api/comments", ""); resp2.StatusCode != http.StatusOK {
				t.Fatalf("%s: GET /api/comments after refused op = %d, want 200 (dir may be bricked)", s.name, resp2.StatusCode)
			}
		})
	}
}
