package serve_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeReportsConformanceCapacityCauseAndRecovers(t *testing.T) {
	files := make(map[string]string)
	for i := 0; i < 64; i++ {
		files[fmt.Sprintf("claims/c%03d.yaml", i)] = fmt.Sprintf("id: widget.contract.c%03d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: capacity fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://shared\n      expectation:\n        shape: set\n        value: [expected]\n", i)
	}
	large := strings.Repeat("x", (1<<20)+(4<<10))
	files["observations.json"] = `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["` + large + `"]}]}`
	cfg := baseConfig + "conformance:\n  observations: observations.json\n"
	_, base, root := startServer(t, cfg, files)

	resp, raw := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
	}
	var status struct {
		OK               bool   `json:"ok"`
		ConformanceError string `json:"conformance_error"`
		ConformanceCode  string `json:"conformance_code"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if status.OK || status.ConformanceCode != "conformance_capacity_exceeded" || !strings.Contains(status.ConformanceError, "capacity") {
		t.Fatalf("status projection=%+v body=%s", status, raw)
	}
	if resp, _ := do(t, http.MethodGet, base+"/", ""); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("oversized root status=%d, want 500", resp.StatusCode)
	}

	small := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["expected"]}]}`
	if err := os.WriteFile(filepath.Join(root, "observations.json"), []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, raw = do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("recovered status=%d body=%s", resp.StatusCode, raw)
	}
	status = struct {
		OK               bool   `json:"ok"`
		ConformanceError string `json:"conformance_error"`
		ConformanceCode  string `json:"conformance_code"`
	}{}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if !status.OK || status.ConformanceError != "" || status.ConformanceCode != "" {
		t.Fatalf("recovered status=%+v", status)
	}
	if resp, body := do(t, http.MethodGet, base+"/", ""); resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "claim-conformance") {
		t.Fatalf("recovered root status=%d", resp.StatusCode)
	}
}

func TestServeConformanceCapacityPrecedesInvalidThemeLikeEveryCheckMode(t *testing.T) {
	files := make(map[string]string)
	for i := 0; i < 64; i++ {
		files[fmt.Sprintf("claims/c%03d.yaml", i)] = fmt.Sprintf("id: widget.contract.c%03d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: capacity fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://shared\n      expectation:\n        shape: set\n        value: [expected]\n", i)
	}
	large := strings.Repeat("x", (1<<20)+(4<<10))
	files["observations.json"] = `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["` + large + `"]}]}`
	cfg := baseConfig + "viewer:\n  theme:\n    preset: does-not-exist\nconformance:\n  observations: observations.json\n"
	_, base, _ := startServer(t, cfg, files)
	resp, raw := do(t, http.MethodGet, base+"/api/status", "")
	var status struct {
		OK           bool   `json:"ok"`
		ThemeError   string `json:"theme_error"`
		FailurePhase string `json:"failure_phase"`
		ErrorCode    string `json:"error_code"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || status.OK || status.ErrorCode != "conformance_capacity_exceeded" || status.FailurePhase != "conformance" || status.ThemeError != "" {
		t.Fatalf("status=%d projection=%+v body-prefix=%q", resp.StatusCode, status, string(raw[:min(len(raw), 512)]))
	}
}

func TestServeLintPrecedesConformanceCapacityLikeEveryCheckMode(t *testing.T) {
	files := make(map[string]string)
	for i := 0; i < 64; i++ {
		restsOn := ""
		if i == 0 {
			restsOn = "rests_on:\n  - widget.contract.missing\n"
		}
		files[fmt.Sprintf("claims/c%03d.yaml", i)] = fmt.Sprintf("id: widget.contract.c%03d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: capacity fixture\n%[2]sgoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://shared\n      expectation:\n        shape: set\n        value: [expected]\n", i, restsOn)
	}
	large := strings.Repeat("x", (1<<20)+(4<<10))
	files["observations.json"] = `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared","shape":"set","value":["` + large + `"]}]}`
	cfg := baseConfig + "conformance:\n  observations: observations.json\n"
	_, base, _ := startServer(t, cfg, files)

	resp, raw := do(t, http.MethodGet, base+"/api/status", "")
	var status struct {
		OK              bool   `json:"ok"`
		LintErrors      []any  `json:"lint_errors"`
		ConformanceCode string `json:"conformance_code"`
		FailurePhase    string `json:"failure_phase"`
		ErrorCode       string `json:"error_code"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || status.OK || len(status.LintErrors) != 1 || status.ErrorCode != "lint_failed" || status.FailurePhase != "lint" {
		t.Fatalf("status=%d projection=%+v body-prefix=%q", resp.StatusCode, status, string(raw[:min(len(raw), 512)]))
	}
	if status.ConformanceCode != "conformance_capacity_exceeded" {
		t.Fatalf("secondary conformance diagnosis disappeared: %+v", status)
	}
}

func TestServeViewerFacetMultiplicityRefusesBeforeUnboundedConstructionAndRecovers(t *testing.T) {
	const facetCount = 600
	files := make(map[string]string, facetCount+1)
	facets := make([]string, facetCount)
	for i := range facets {
		facet := fmt.Sprintf("f%03d", i)
		facets[i] = facet
		files[fmt.Sprintf("claims/f%03d.yaml", i)] = fmt.Sprintf("id: widget.%s.one\nfacet: %s\nmodule: widget\nstatus: draft\nlayout: card\nbody: facet fixture\ngoverned_by:\n  type: none\n  reason: fixture\n", facet, facet)
	}
	overviewPath := "claims/overview.yaml"
	overview := func(reason string) string {
		return "id: widget.overview.capacity\nfacet: overview\nmodule: widget\nstatus: draft\nkind: orientation-note\nlayout: banner\nbody: overview fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: none\n  reason: \"" + reason + "\"\n"
	}
	files[overviewPath] = overview(strings.Repeat("x", 128<<10))
	cfg := "schema_version: 1\nfacets: [" + strings.Join(facets, ", ") + "]\nmodules: [widget]\nclaims_dir: claims\n"
	_, base, root := startServer(t, cfg, files)

	resp, raw := do(t, http.MethodGet, base+"/api/status", "")
	var capacityStatus struct {
		OK               bool   `json:"ok"`
		ConformanceError string `json:"conformance_error"`
		ConformanceCode  string `json:"conformance_code"`
	}
	if err := json.Unmarshal(raw, &capacityStatus); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || capacityStatus.ConformanceCode != "conformance_capacity_exceeded" {
		t.Fatalf("status=%d capacity=%+v body-prefix=%q", resp.StatusCode, capacityStatus, string(raw[:min(len(raw), 512)]))
	}
	if resp, _ := do(t, http.MethodGet, base+"/", ""); resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("oversized viewer status=%d", resp.StatusCode)
	}

	if err := os.WriteFile(filepath.Join(root, overviewPath), []byte(overview("small")), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, raw = do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK || strings.Contains(string(raw), "conformance_capacity_exceeded") {
		t.Fatalf("recovered status=%d body=%s", resp.StatusCode, raw)
	}
	if resp, _ := do(t, http.MethodGet, base+"/", ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("recovered viewer status=%d", resp.StatusCode)
	}
}
