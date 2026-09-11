package serve_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/conformance"
)

func TestLiveServerPluralConformanceIsBoundedAcrossGraphShapes(t *testing.T) {
	cases := []struct {
		name   string
		layers int
		width  int
	}{
		{name: "chain-24", layers: 24, width: 1},
		{name: "dense-6x4", layers: 6, width: 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, claimCount := liveGraphScaleFiles(tc.layers, tc.width)
			files["observations.json"] = `{"format_version":1,"snapshot":"live-scale","observations":[{"adapter":"neutral/v1","target":"widget://shared/set","shape":"set","value":["waiting","ready"]},{"adapter":"neutral/v1","target":"widget://shared/scalar","shape":"scalar","value":"ready"}]}`
			_, base, _ := startServer(t, baseConfig+"conformance:\n  observations: observations.json\n", files)

			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			statusResponse, statusBody := do(t, http.MethodGet, base+"/api/status", "")
			viewerResponse, viewerBody := do(t, http.MethodGet, base+"/", "")
			elapsed := time.Since(started)
			runtime.ReadMemStats(&after)
			allocated := after.TotalAlloc - before.TotalAlloc
			if statusResponse.StatusCode != http.StatusOK || viewerResponse.StatusCode != http.StatusOK {
				t.Fatalf("status=%d viewer=%d status_body=%s", statusResponse.StatusCode, viewerResponse.StatusCode, statusBody)
			}

			var status struct {
				Conformance struct {
					Summary conformance.Summary `json:"summary"`
					Results []struct {
						Checks []struct {
							State string `json:"state"`
						} `json:"checks"`
					} `json:"results"`
				} `json:"conformance"`
			}
			if err := json.Unmarshal(statusBody, &status); err != nil {
				t.Fatal(err)
			}
			checkCount := claimCount * 2
			wantSummary := conformance.Summary{Declared: claimCount, Checks: checkCount, Matched: checkCount, Ready: claimCount}
			if len(status.Conformance.Results) != claimCount || status.Conformance.Summary != wantSummary {
				t.Fatalf("status results=%d summary=%+v, want results=%d summary=%+v", len(status.Conformance.Results), status.Conformance.Summary, claimCount, wantSummary)
			}
			viewerDeclarations := strings.Count(string(viewerBody), `class="claim-conformance"`)
			viewerChecks := strings.Count(string(viewerBody), `data-check-id="`)
			if viewerDeclarations != claimCount || viewerChecks != checkCount {
				t.Fatalf("viewer declarations=%d checks=%d, want declarations=%d checks=%d", viewerDeclarations, viewerChecks, claimCount, checkCount)
			}
			if len(statusBody) >= conformance.MaxOutputBytes || len(viewerBody) >= conformance.MaxOutputBytes {
				t.Fatalf("status=%d viewer=%d bytes, cap=%d", len(statusBody), len(viewerBody), conformance.MaxOutputBytes)
			}
			if elapsed >= 2*time.Second {
				t.Fatalf("live status+viewer took %s, maximum is under 2s", elapsed)
			}
			if allocated >= 512<<20 {
				t.Fatalf("live status+viewer allocated %d bytes, maximum is under %d", allocated, 512<<20)
			}
			t.Logf("graph=%s claims=%d declarations=%d checks=%d status_bytes=%d viewer_bytes=%d elapsed=%s TotalAlloc=%d", tc.name, claimCount, claimCount, checkCount, len(statusBody), len(viewerBody), elapsed, allocated)
		})
	}
}

func liveGraphScaleFiles(layers, width int) (files map[string]string, claimCount int) {
	files = make(map[string]string, layers*width+1)
	for layer := 0; layer < layers; layer++ {
		for column := 0; column < width; column++ {
			index := layer*width + column
			id := fmt.Sprintf("widget.contract.live-%03d", index)
			var claim strings.Builder
			fmt.Fprintf(&claim, "id: %s\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: bounded live graph fixture\n", id)
			if layer > 0 {
				claim.WriteString("rests_on:\n")
				for previous := 0; previous < width; previous++ {
					fmt.Fprintf(&claim, "  - widget.contract.live-%03d\n", (layer-1)*width+previous)
				}
			}
			claim.WriteString("governed_by:\n  type: none\n  reason: fixture\n")
			claim.WriteString("embodiment:\n  mode: compare\n  checks:\n    - id: set-state\n      adapter: neutral/v1\n      target: widget://shared/set\n      expectation:\n        shape: set\n        value: [ready, waiting]\n    - id: scalar-state\n      adapter: neutral/v1\n      target: widget://shared/scalar\n      expectation:\n        shape: scalar\n        value: ready\n")
			files[fmt.Sprintf("claims/live-%03d.yaml", index)] = claim.String()
		}
	}
	return files, layers * width
}
