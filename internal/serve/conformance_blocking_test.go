package serve_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestStatusConformanceBlockingPolicyStateMatrixRemainsAvailableAndTruthful(t *testing.T) {
	compare := draftClaim("widget.contract.state") + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"
	none := draftClaim("widget.contract.state") + "embodiment:\n  mode: none\n  reason: neutral documentation-only fixture\n"
	states := []struct {
		name         string
		claim        string
		observations string
		state        string
		nonmatched   bool
	}{
		{name: "matched", claim: compare, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`, state: "matched"},
		{name: "owed", claim: compare, observations: `{"format_version":1,"observations":[]}`, state: "owed", nonmatched: true},
		{name: "mismatch", claim: compare, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`, state: "mismatch", nonmatched: true},
		{name: "uncheckable", claim: compare, observations: `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","error":{"code":"adapter_failed","message":"neutral fixture failure"}}]}`, state: "uncheckable", nonmatched: true},
		{name: "declared-none", claim: none, observations: `{"format_version":1,"observations":[]}`},
	}
	policies := []struct {
		name    string
		line    string
		enabled bool
	}{
		{name: "omitted"},
		{name: "explicit-false", line: "  blocking: false\n"},
		{name: "true", line: "  blocking: true\n", enabled: true},
	}
	for _, policy := range policies {
		for _, state := range states {
			t.Run(policy.name+"/"+state.name, func(t *testing.T) {
				cfg := baseConfig + "conformance:\n  observations: observations.json\n" + policy.line
				_, base, _ := startServer(t, cfg, map[string]string{
					"claims/state.yaml": state.claim,
					"observations.json": state.observations,
				})
				resp, body := do(t, http.MethodGet, base+"/api/status", "")
				if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" {
					t.Fatalf("status=%d cache=%q body=%s", resp.StatusCode, resp.Header.Get("Cache-Control"), body)
				}
				var status struct {
					OK                         bool `json:"ok"`
					ConformanceBlockingEnabled bool `json:"conformance_blocking_enabled"`
					ConformanceBlockingChecks  int  `json:"conformance_blocking_checks"`
					Conformance                struct {
						Results []struct {
							Checks []struct {
								State string `json:"state"`
							} `json:"checks"`
						} `json:"results"`
					} `json:"conformance"`
					ErrorCode    string `json:"error_code"`
					FailurePhase string `json:"failure_phase"`
				}
				if err := json.Unmarshal(body, &status); err != nil {
					t.Fatal(err)
				}
				blocked := policy.enabled && state.nonmatched
				wantCount := 0
				wantCode, wantPhase := "", ""
				if blocked {
					wantCount = 1
					wantCode, wantPhase = "conformance_failed", "conformance"
				}
				if status.OK == blocked || status.ConformanceBlockingEnabled != policy.enabled || status.ConformanceBlockingChecks != wantCount || status.ErrorCode != wantCode || status.FailurePhase != wantPhase || len(status.Conformance.Results) != 1 {
					t.Fatalf("status policy/state mismatch: %+v body=%s", status, body)
				}
				if state.state != "" && (len(status.Conformance.Results[0].Checks) != 1 || status.Conformance.Results[0].Checks[0].State != state.state) {
					t.Fatalf("status did not carry %s check: %+v body=%s", state.state, status, body)
				}
				if state.state == "" && len(status.Conformance.Results[0].Checks) != 0 {
					t.Fatalf("declared-none unexpectedly carried checks: %+v", status)
				}
			})
		}
	}
}

func TestStatusReportsBlockingConformanceAlongsidePrimaryLedgerFailure(t *testing.T) {
	cfg := baseConfig + "conformance:\n  observations: observations.json\n  blocking: true\n"
	locked := "id: widget.contract.state\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\nbody: neutral unrecorded fixture\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"
	_, base, _ := startServer(t, cfg, map[string]string{
		"claims/state.yaml": locked,
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`,
	})

	resp, body := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var status struct {
		OK                         bool `json:"ok"`
		ConformanceBlockingEnabled bool `json:"conformance_blocking_enabled"`
		ConformanceBlockingChecks  int  `json:"conformance_blocking_checks"`
		LedgerFindings             []struct {
			Rule string `json:"rule"`
		} `json:"ledger_findings"`
		ErrorCode    string `json:"error_code"`
		FailurePhase string `json:"failure_phase"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if status.OK || status.ErrorCode != "integrity_failed" || status.FailurePhase != "ledger" || len(status.LedgerFindings) == 0 || !status.ConformanceBlockingEnabled || status.ConformanceBlockingChecks != 1 {
		t.Fatalf("status lost integrity precedence or conformance evidence: %+v body=%s", status, body)
	}
}

func TestStatusBlockingCountsMixedChecksAndStaysAvailable(t *testing.T) {
	cfg := baseConfig + "conformance:\n  observations: observations.json\n  blocking: true\n"
	claim := draftClaim("widget.contract.state") + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n    - id: policy\n      adapter: neutral/v1\n      target: widget://policy\n      expectation:\n        shape: scalar\n        value: enabled\n"
	_, base, _ := startServer(t, cfg, map[string]string{
		"claims/state.yaml": claim,
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]},{"adapter":"neutral/v1","target":"widget://policy","shape":"scalar","value":"disabled"}]}`,
	})

	resp, body := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	var status struct {
		OK                        bool `json:"ok"`
		ConformanceBlockingChecks int  `json:"conformance_blocking_checks"`
		Conformance               struct {
			Summary struct {
				Checks   int `json:"checks"`
				Matched  int `json:"matched"`
				Mismatch int `json:"mismatch"`
			} `json:"summary"`
		} `json:"conformance"`
		ErrorCode    string `json:"error_code"`
		FailurePhase string `json:"failure_phase"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if status.OK || status.ConformanceBlockingChecks != 1 || status.Conformance.Summary.Checks != 2 || status.Conformance.Summary.Matched != 1 || status.Conformance.Summary.Mismatch != 1 || status.ErrorCode != "conformance_failed" || status.FailurePhase != "conformance" {
		t.Fatalf("mixed status = %+v body=%s", status, body)
	}
}
