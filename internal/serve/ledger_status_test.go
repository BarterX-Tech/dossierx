package serve_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ledgerStatusDTO is the shape a status poller actually decodes. It is spelled
// out here rather than reusing serve's unexported statusDTO because the WIRE
// CONTRACT is what regressed: the strip is fed by JSON field names, and a test
// that shared the Go struct would keep passing if the field were renamed on the
// wire.
type ledgerStatusDTO struct {
	OK             bool `json:"ok"`
	LedgerFindings []struct {
		Rule    string `json:"rule"`
		ClaimID string `json:"claim_id"`
		Message string `json:"message"`
	} `json:"ledger_findings"`
}

// GET /api/status must SURFACE the lock-ledger gate's verdict.
//
// This is the defect the viewer made dangerous. check.Status has always
// evaluated the ledger gate and filled Result.LedgerFindings — it REPORTS where
// check.Run REFUSES, precisely so the page keeps rendering for a human who
// needs to read the tampered claim. But serve's status DTO carried exactly five
// fields and dropped that slice on the floor, so the one surface a human ever
// looks at answered a tampered project with `ok: true` and no findings at all.
// The viewer is not a secondary read-out of the CLI; for a reviewer it IS the
// project, and "the lock ledger says this locked claim was never approved" is
// the single most important thing it can say.
//
// The fixture is the standard project, which contains a LOCKED claim
// (widget.contract.locked) and no lock store whatsoever — the exact state
// lock.Audit calls lock-ledger-missing (the claim's status was set outside the
// approval path) alongside the project-scoped lock-ledger-absent.
func TestStatus_SurfacesLedgerFindings(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resp, data := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	var dto ledgerStatusDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("decode status: %v (body=%s)", err, data)
	}

	if len(dto.LedgerFindings) == 0 {
		t.Fatalf("GET /api/status reported no ledger findings for a project whose locked claim has no ledger record; the tamper is invisible on the only surface a human reads (body=%s)", data)
	}
	var found bool
	for _, f := range dto.LedgerFindings {
		if f.Rule == "lock-ledger-missing" && f.ClaimID == "widget.contract.locked" {
			found = true
		}
		if f.Rule == "" || f.Message == "" {
			t.Fatalf("ledger finding %+v is missing its rule/message; the strip renders both verbatim", f)
		}
	}
	if !found {
		t.Fatalf("expected a lock-ledger-missing finding for widget.contract.locked, got %+v", dto.LedgerFindings)
	}

	// And the headline must agree with the body. `ok: true` next to a populated
	// ledger_findings array is worse than the omission was: it tells the strip
	// to render a green project while the gate is refusing.
	if dto.OK {
		t.Fatalf("GET /api/status reported ok:true with %d ledger finding(s); the status headline must fail closed on an integrity finding (body=%s)", len(dto.LedgerFindings), data)
	}
}

// ledger_findings is always an ARRAY on the wire, never null, on the same terms
// lint_errors/next_steps already are: the strip ranges over it without a null
// test, and a `null` there is an unhandled TypeError in the viewer rather than
// a clean "no findings".
func TestStatus_LedgerFindingsNeverNull(t *testing.T) {
	// A project with only a draft claim: nothing locked, so the gate is silent.
	_, base, _ := startServer(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})

	resp, data := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode status: %v (body=%s)", err, data)
	}
	got, ok := raw["ledger_findings"]
	if !ok {
		t.Fatalf("status DTO has no ledger_findings field at all (body=%s)", data)
	}
	if string(got) != "[]" {
		t.Fatalf("ledger_findings on a clean project = %s, want []; the strip must be able to range over it without a null test", got)
	}
}

// The status poll stays READ-ONLY with respect to the lock store and the comment
// digest store. Surfacing the gate's verdict must not tempt the endpoint into
// creating, adopting or repairing either store: "re-record whatever the files
// say now" is exactly what an attacker wants an integrity check to do, and GET
// is CSRF-exempt, so a bare unauthenticated poll would be the trigger.
func TestStatus_DoesNotWriteLedgerStores(t *testing.T) {
	_, base, root := startServer(t, baseConfig, standardFiles())

	for i := 0; i < 3; i++ {
		if resp, data := do(t, http.MethodGet, base+"/api/status", ""); resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/status: got %d, want 200 (body=%s)", resp.StatusCode, data)
		}
	}

	for _, rel := range []string{".dossierx-lock-store.json", ".dossierx-comment-digest.json"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("GET /api/status created %s (stat err=%v); the ledger gate is a pure read and serve must never adopt a store on a poll", rel, err)
		}
	}
}

// The served viewer must actually CONSUME the field. A status DTO nobody reads
// is the same invisible tamper in a different file, and the viewer is the only
// surface the human has: the shell has to poll /api/status and present
// ledger_findings the way it presents lint errors.
//
// This asserts on the SERVED document (not the template on disk) because that
// is what a browser receives, and it pins the three things the wiring cannot
// work without: the strip element, the fetch, and the field name.
func TestViewer_ShellConsumesLedgerFindings(t *testing.T) {
	_, base, _ := startServer(t, baseConfig, standardFiles())

	resp, data := do(t, http.MethodGet, base+"/", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: got %d, want 200", resp.StatusCode)
	}
	doc := string(data)
	for _, want := range []string{
		`id="statusStrip"`, // the strip element itself
		`'/api/status'`,    // the poll that feeds it
		`ledger_findings`,  // the field the defect dropped
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("served viewer does not contain %q; the status DTO's ledger findings reach no human surface", want)
		}
	}
}
