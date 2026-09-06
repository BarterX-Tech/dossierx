package serve_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedReadinessStoresFailAllServedSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
	}{
		{name: "lock store", file: "lock-store.json"},
		{name: "flag store", file: "flag-store.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, base, root := startServer(t, baseConfig, map[string]string{
				"claims/one.yaml": draftClaim("widget.contract.one"),
			})
			writeFile(t, filepath.Join(root, "build", "ledger", tc.file), "{")

			for _, path := range []string{"/", "/api/status", "/api/graph"} {
				resp, body := do(t, http.MethodGet, base+path, "")
				if resp.StatusCode != http.StatusInternalServerError {
					t.Fatalf("GET %s with malformed %s: got %d, want 500 (body=%s)", path, tc.name, resp.StatusCode, body)
				}
				if !strings.Contains(string(body), "readiness") {
					t.Fatalf("GET %s with malformed %s did not expose readiness failure: %s", path, tc.name, body)
				}
			}
		})
	}
}
