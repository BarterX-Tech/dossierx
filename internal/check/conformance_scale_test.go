package check_test

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
)

func TestConformanceEndToEndGraphScaleBounds(t *testing.T) {
	cases := []struct {
		name   string
		layers int
		width  int
	}{
		{name: "chain-32", layers: 32, width: 1},
		{name: "chain-64", layers: 64, width: 1},
		{name: "chain-128", layers: 128, width: 1},
		{name: "dense-8x5", layers: 8, width: 5},
		{name: "dense-16x5", layers: 16, width: 5},
		{name: "dense-24x5", layers: 24, width: 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, declared, checks := graphScaleFiles(tc.layers, tc.width)
			files["observations.json"] = `{"format_version":1,"snapshot":"scale","observations":[{"adapter":"neutral/v1","target":"widget://shared/set","shape":"set","value":["waiting","ready"]},{"adapter":"neutral/v1","target":"widget://shared/scalar","shape":"scalar","value":"ready"}]}`
			cfg, claims := project(t, baseConfig+"conformance:\n  observations: observations.json\n", files)

			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			res, err := check.Run(claims, cfg)
			elapsed := time.Since(started)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatal(err)
			}
			limit := 2 * time.Second
			allocationLimit := int64(512 << 20)
			if raceBuildEnabled {
				limit = 5 * time.Second
				// The race runtime retains shadow state for every instrumented
				// access. Keep a generous race-only ceiling while preserving the
				// production 512 MiB bound for normal builds and graph proofs.
				allocationLimit = 1 << 30
			}
			if elapsed > limit {
				t.Fatalf("end-to-end run took %s, limit %s", elapsed, limit)
			}
			allocated := after.TotalAlloc - before.TotalAlloc
			if allocated > uint64(allocationLimit) {
				t.Fatalf("end-to-end run allocated %d bytes, limit %d", allocated, allocationLimit)
			}
			if res.Conformance == nil || len(res.Conformance.Results) != declared || res.Conformance.Summary != (conformance.Summary{Declared: declared, DeclaredNone: declared - checks/2, Checks: checks, Matched: checks, Ready: declared}) {
				t.Fatalf("results=%v, want %d", res.Conformance, declared)
			}

			paths := []string{cfg.ConformanceStatusPath(), cfg.CatalogPath(), cfg.ViewerPath()}
			sizes := make([]int, len(paths))
			for i, path := range paths {
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				sizes[i] = len(raw)
				if len(raw) > conformance.MaxOutputBytes {
					t.Fatalf("%s = %d bytes, cap %d", path, len(raw), conformance.MaxOutputBytes)
				}
			}
			statusRaw, err := os.ReadFile(cfg.ConformanceStatusPath())
			if err != nil {
				t.Fatal(err)
			}
			var status conformance.Report
			if err := json.Unmarshal(statusRaw, &status); err != nil || len(status.Results) != declared {
				t.Fatalf("status results=%d err=%v", len(status.Results), err)
			}
			catalogRaw, err := os.ReadFile(cfg.CatalogPath())
			if err != nil {
				t.Fatal(err)
			}
			var catalogDoc struct {
				Claims []struct {
					Conformance *conformance.Result `json:"conformance"`
				} `json:"claims"`
			}
			if err := json.Unmarshal(catalogRaw, &catalogDoc); err != nil {
				t.Fatal(err)
			}
			catalogDeclared, catalogChecks := 0, 0
			for _, claim := range catalogDoc.Claims {
				if claim.Conformance != nil {
					catalogDeclared++
					catalogChecks += len(claim.Conformance.Checks)
				}
			}
			viewerRaw, err := os.ReadFile(cfg.ViewerPath())
			if err != nil {
				t.Fatal(err)
			}
			viewerDeclared := strings.Count(string(viewerRaw), `class="claim-conformance"`)
			viewerChecks := strings.Count(string(viewerRaw), `data-check-id="`)
			if catalogDeclared != declared || catalogChecks != checks || viewerDeclared != declared || viewerChecks != checks || status.Summary.Checks != checks {
				t.Fatalf("agreement: status declarations=%d checks=%d catalog declarations=%d checks=%d viewer declarations=%d checks=%d", declared, status.Summary.Checks, catalogDeclared, catalogChecks, viewerDeclared, viewerChecks)
			}
			t.Logf("claims=%d declarations=%d checks=%d status_bytes=%d catalog_bytes=%d viewer_bytes=%d elapsed=%s TotalAlloc=%d", len(claims), declared, checks, sizes[0], sizes[1], sizes[2], elapsed, allocated)
		})
	}
}

func graphScaleFiles(layers, width int) (files map[string]string, declared, checks int) {
	files = make(map[string]string, layers*width+1)
	for layer := 0; layer < layers; layer++ {
		for column := 0; column < width; column++ {
			index := layer*width + column
			id := fmt.Sprintf("widget.contract.c%03d", index)
			var b strings.Builder
			fmt.Fprintf(&b, "id: %s\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: bounded scale fixture\n", id)
			if layer > 0 {
				b.WriteString("rests_on:\n")
				for previous := 0; previous < width; previous++ {
					fmt.Fprintf(&b, "  - widget.contract.c%03d\n", (layer-1)*width+previous)
				}
			}
			b.WriteString("governed_by:\n  type: none\n  reason: fixture\n")
			switch {
			case index%5 == 0:
				b.WriteString("embodiment:\n  mode: none\n  reason: generated documentation claim\n")
				declared++
			case index%3 == 0:
				b.WriteString("embodiment:\n  mode: compare\n  checks:\n    - id: set-state\n      adapter: neutral/v1\n      target: widget://shared/set\n      expectation:\n        shape: set\n        value: [ready, waiting]\n    - id: scalar-state\n      adapter: neutral/v1\n      target: widget://shared/scalar\n      expectation:\n        shape: scalar\n        value: ready\n")
				declared++
				checks += 2
			}
			files[fmt.Sprintf("claims/c%03d.yaml", index)] = b.String()
		}
	}
	return files, declared, checks
}
