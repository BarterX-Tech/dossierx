// mockup_gate_test.go covers the ONE path in this engine that emits author
// bytes into the browser unescaped, as "dossierx serve" reaches it.
//
// "dossierx check" protects that path with the raw-html-scope lint: a hostile
// raw_html payload fails the lint step, loudly, and the pipeline never renders.
// serve has no lint step — it loads, builds and renders — so the human's very
// next move after a failing check ("let me go LOOK at that claim") was the one
// that executed the payload, same-origin, on the port that owns the comment
// write API.
package serve_test

import (
	"net/http"
	"strings"
	"testing"
)

// viewerHTML fetches GET / — the rendered viewer document — and fails the test
// on anything but a 200.
func viewerHTML(t *testing.T, base string) string {
	t.Helper()
	res, raw := do(t, http.MethodGet, base+"/", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d, body %s", res.StatusCode, raw)
	}
	return string(raw)
}

const mockupConfig = "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
	"mockup_modules:\n  - widget\n"

// hostileMockup is a claim typed by hand into the repo: status: locked and
// raw_html_reviewed: true are plain YAML fields, and `status` sits on
// LockedClaimHash's deny-list, so neither the lint gate nor the content-drift
// rule stands between these bytes and the render.
const hostileMockup = "id: widget.contract.mock\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: mockup\n" +
	"raw_html_reviewed: true\n" +
	"raw_html: '<script>alert(1)</script><img src=x onerror=\"fetch(1)\">'\n" +
	"body: |\n  a mockup claim.\n" +
	"governed_by:\n  type: none\n  reason: fixture\n"

// approvedMockup passes every part of the gate: allowlisted module, locked,
// reviewed, and markup inside the permitted tag/attribute/class vocabulary.
const approvedMockup = "id: widget.contract.ok\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: mockup\n" +
	"raw_html_reviewed: true\n" +
	"raw_html: '<div class=\"gcp-row\">approved markup</div>'\n" +
	"body: |\n  an approved mockup.\n" +
	"governed_by:\n  type: none\n  reason: fixture\n"

func TestServe_UngatedRawHTMLIsEscaped(t *testing.T) {
	_, base, _ := startServer(t, mockupConfig, map[string]string{
		"claims/mock.yaml": hostileMockup,
	})

	body := viewerHTML(t, base)

	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("serve emitted the payload verbatim: a script in raw_html runs same-origin on the comment API's own port")
	}
	if strings.Contains(body, `onerror="fetch(1)"`) {
		t.Fatalf("serve emitted a live event-handler attribute from raw_html")
	}
	// Disarmed, not hidden: the human opened the viewer precisely to read the
	// disputed claim, so the markup must still be there — escaped.
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatalf("expected the payload to be rendered as escaped text so the reviewer can read it, got:\n%s", body)
	}
}

// The gate must not disarm a mockup that PASSES it, or the one feature it
// guards would never work.
func TestServe_ApprovedRawHTMLStaysLive(t *testing.T) {
	_, base, _ := startServer(t, mockupConfig, map[string]string{
		"claims/ok.yaml": approvedMockup,
	})

	body := viewerHTML(t, base)
	if !strings.Contains(body, `<div class="gcp-row">approved markup</div>`) {
		t.Fatalf("an approved, allowlisted, reviewed mockup must still render live markup")
	}
}

// A hostile claim must not take its NEIGHBOURS' markup down with it: the gate
// disarms per claim, exactly as its findings are per claim.
func TestServe_OneUngatedMockupDoesNotDisarmTheOthers(t *testing.T) {
	_, base, _ := startServer(t, mockupConfig, map[string]string{
		"claims/mock.yaml": hostileMockup,
		"claims/ok.yaml":   approvedMockup,
	})

	body := viewerHTML(t, base)
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Fatalf("serve emitted the hostile payload verbatim")
	}
	if !strings.Contains(body, `<div class="gcp-row">approved markup</div>`) {
		t.Fatalf("the approved mockup was disarmed by an unrelated claim's finding")
	}
}
