package serve_test

import (
	"encoding/json"
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

			for _, path := range []string{"/", "/api/graph"} {
				resp, body := do(t, http.MethodGet, base+path, "")
				if resp.StatusCode != http.StatusInternalServerError {
					t.Fatalf("GET %s with malformed %s: got %d, want 500 (body=%s)", path, tc.name, resp.StatusCode, body)
				}
				if !strings.Contains(string(body), "readiness") {
					t.Fatalf("GET %s with malformed %s did not expose readiness failure: %s", path, tc.name, body)
				}
			}
			resp, body := do(t, http.MethodGet, base+"/api/status", "")
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET /api/status with malformed %s: got %d body=%s", tc.name, resp.StatusCode, body)
			}
			var status struct {
				OK             bool              `json:"ok"`
				CatalogError   string            `json:"catalog_error"`
				FailurePhase   string            `json:"failure_phase"`
				ErrorCode      string            `json:"error_code"`
				LedgerFindings []json.RawMessage `json:"ledger_findings"`
			}
			if err := json.Unmarshal(body, &status); err != nil {
				t.Fatal(err)
			}
			if tc.file == "lock-store.json" {
				if status.OK || status.CatalogError != "" || status.FailurePhase != "ledger" || status.ErrorCode != "integrity_failed" || len(status.LedgerFindings) == 0 {
					t.Fatalf("lock-store status projection=%+v body=%s", status, body)
				}
			} else if status.OK || status.CatalogError == "" || status.FailurePhase != "catalog" || status.ErrorCode != "write_failed" {
				t.Fatalf("flag-store status projection=%+v body=%s", status, body)
			}
		})
	}
}
