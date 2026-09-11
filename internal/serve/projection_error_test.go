package serve_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestStatusKeepsMalformedShellFailureInRenderDomain(t *testing.T) {
	cfg := baseConfig + "viewer:\n  template_overrides: overrides\nconformance:\n  observations: observations.json\n"
	_, base, _ := startServer(t, cfg, map[string]string{
		"claims/one.yaml":      draftClaim("widget.contract.one") + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://one\n      expectation:\n        shape: set\n        value: [ready]\n",
		"observations.json":    `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://one","shape":"set","value":["ready"]}]}`,
		"overrides/shell.html": "{{end}}",
	})
	resp, body := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d cache=%q body=%s", resp.StatusCode, resp.Header.Get("Cache-Control"), body)
	}
	var status struct {
		OK               bool   `json:"ok"`
		ConformanceError string `json:"conformance_error"`
		RenderError      string `json:"render_error"`
		FailurePhase     string `json:"failure_phase"`
		ErrorCode        string `json:"error_code"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if status.OK || status.ConformanceError != "" || status.RenderError == "" || status.FailurePhase != "render" || status.ErrorCode != "write_failed" {
		t.Fatalf("status projection=%+v body=%s", status, body)
	}
	if strings.Contains(status.RenderError, "conformance input") {
		t.Fatalf("render failure used conformance prose: %q", status.RenderError)
	}
}
