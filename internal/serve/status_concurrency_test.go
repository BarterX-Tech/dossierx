package serve_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestStatusConcurrentGETAndHEADCoalesceAndRemainFresh(t *testing.T) {
	files := map[string]string{
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["old"]}]}`,
	}
	large := make([]byte, 192<<10)
	for i := range large {
		large[i] = 'x'
	}
	for i := 0; i < 32; i++ {
		claim := fmt.Sprintf("id: widget.contract.c%02d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: %s\ngoverned_by:\n  type: none\n  reason: fixture\n", i, large)
		if i == 0 {
			claim += "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [new]\n"
		}
		files[fmt.Sprintf("claims/c%02d.yaml", i)] = claim
	}
	cfg := baseConfig + "conformance:\n  observations: observations.json\n"
	srv, base, root := startServer(t, cfg, files)

	const requests = 32
	start := make(chan struct{})
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			method := http.MethodGet
			if i%2 == 1 {
				method = http.MethodHead
			}
			resp, body := do(t, method, base+"/api/status", "")
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("%s status=%d body=%s", method, resp.StatusCode, body)
				return
			}
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				errs <- fmt.Errorf("%s Cache-Control=%q", method, got)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	runtime.ReadMemStats(&after)
	for err := range errs {
		t.Error(err)
	}
	if runs := srv.StatusRuns(); runs > 2 {
		t.Fatalf("%d concurrent status polls performed %d full projections; want at most 2", requests, runs)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 768<<20 {
		t.Fatalf("coalesced status burst allocated %d bytes", allocated)
	}

	// A later request starts a later batch and must observe the changed input.
	updated := `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["new"]}]}`
	if err := os.WriteFile(filepath.Join(root, "observations.json"), []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, body := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("fresh GET status=%d cache=%q body=%s", resp.StatusCode, resp.Header.Get("Cache-Control"), body)
	}
	var status struct {
		Conformance struct {
			Results []struct {
				Checks []struct {
					State string `json:"state"`
				} `json:"checks"`
			} `json:"results"`
		} `json:"conformance"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Conformance.Results) != 1 || status.Conformance.Results[0].Checks[0].State != "matched" {
		t.Fatalf("fresh status did not observe changed snapshot: %s", body)
	}
}
