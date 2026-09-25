// claim_caps_harness_test.go is the NIT-11 integration test for the claim
// harness (NIT-28). Each cap ships its own lint and lock refusal on its own
// ticket (NIT-8 summary and body, NIT-14 claims per module, NIT-6/NIT-20
// rests-on-target). This test proves they hold together at the two gates an
// agent actually hits, `check` and `claim lock`, through the built binary and
// one static fixture: testdata/fixture-coverage/lifecycle/claim-caps.
//
// The fixture runs on the engine defaults (10 claims per module, 200-char
// summary, 2,000-char body+steps+rows) with no override in its config, and
// holds one claim per refusal:
//
//	caps.contract.no-summary    summary-required
//	caps.contract.long-summary  summary-oversize (201 characters)
//	caps.contract.long-body     body-oversize (body 1,000 + steps 1,100:
//	                            each surface is under the cap, the sum is not)
//	crowded.contract.c00..c10   module-claim-cap (11 claims in one module)
//	caps.contract.foreign       rests-on-target (cites other.internals.secret)
//
// plus caps.contract.control, which is inside every cap and must lock, so a
// refusal above is about the claim and not about the project.
//
// Oversize is a hard refusal: check exits 1 and claim lock exits 1 with
// lint_failed and writes nothing. A refusal that stops firing here is a bug
// in the owning rule, not something to relax in this test.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type capsFinding struct {
	Lint     string `json:"lint"`
	ClaimID  string `json:"claim_id"`
	Severity string `json:"severity"`
}

// claimCapsCases is every refusal the harness owes, keyed by the claim that
// must carry it.
var claimCapsCases = []struct {
	claim, lint string
}{
	{"caps.contract.no-summary", "summary-required"},
	{"caps.contract.long-summary", "summary-oversize"},
	{"caps.contract.long-body", "body-oversize"},
	{"crowded.contract.c00", "module-claim-cap"},
	{"crowded.contract.c10", "module-claim-cap"},
	{"caps.contract.foreign", "rests-on-target"},
}

// claimCapsFixture copies the static fixture into a temp directory, because
// check and claim lock write the ledger and claim files. The committed
// config is asserted to carry no cap override, so an edit cannot quietly turn
// this into a test of custom limits.
func claimCapsFixture(t *testing.T) (root, cfgPath string) {
	t.Helper()
	src := filepath.Join(lifecycleFixturesRoot(t), "claim-caps")
	cfg, err := os.ReadFile(filepath.Join(src, "project.config.yaml"))
	if err != nil {
		t.Fatalf("read claim-caps config: %v", err)
	}
	for _, key := range []string{"max_claims_per_module:", "max_claim_body_chars:", "max_claim_summary_chars:"} {
		if bytes.Contains(cfg, []byte("\n"+key)) {
			t.Fatalf("the claim-caps fixture must run on the defaults, but its config sets %s", key)
		}
	}
	root = filepath.Join(t.TempDir(), "claim-caps")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	copyTree(t, src, root)
	return root, filepath.Join(root, "project.config.yaml")
}

func capsCheckFindings(t *testing.T, root, cfgPath string, args ...string) []capsFinding {
	t.Helper()
	all := append([]string{"--config", cfgPath, "check"}, args...)
	stdout, stderr, code := run(t, root, append(all, "--format", "json")...)
	if code != 1 {
		t.Fatalf("check %v: want exit 1 on cap violations, got %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
	}
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			LintFindings []capsFinding `json:"lint_findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("check %v: parse envelope: %v\n%s", args, err, stdout)
	}
	if env.OK {
		t.Fatalf("check %v: envelope says ok on cap violations", args)
	}
	return env.Data.LintFindings
}

func hasCapsFinding(findings []capsFinding, lint, claim string) bool {
	for _, f := range findings {
		if f.Lint == lint && f.ClaimID == claim && f.Severity == "error" {
			return true
		}
	}
	return false
}

func TestClaimCaps_CheckAndLockRefuseEveryCap(t *testing.T) {
	root, cfgPath := claimCapsFixture(t)

	for _, mode := range [][]string{{"--validate"}, nil} {
		findings := capsCheckFindings(t, root, cfgPath, mode...)
		for _, c := range claimCapsCases {
			if !hasCapsFinding(findings, c.lint, c.claim) {
				t.Errorf("check %v: want %s on %s, got %+v", mode, c.lint, c.claim, findings)
			}
		}
		for i := 0; i <= 10; i++ {
			id := fmt.Sprintf("crowded.contract.c%02d", i)
			if !hasCapsFinding(findings, "module-claim-cap", id) {
				t.Errorf("check %v: every claim in the over-cap module carries module-claim-cap, missing %s", mode, id)
			}
		}
		for _, f := range findings {
			if f.ClaimID == "caps.contract.control" && f.Severity == "error" {
				t.Errorf("check %v: the control claim must be clean, got %+v", mode, f)
			}
		}
	}

	for _, c := range claimCapsCases {
		t.Run(c.lint+"/"+c.claim, func(t *testing.T) {
			claimPath := capsClaimPath(t, root, c.claim)
			before, err := os.ReadFile(claimPath)
			if err != nil {
				t.Fatal(err)
			}
			storePath := filepath.Join(root, "build", "ledger", "lock-store.json")
			storeBefore, err := os.ReadFile(storePath)
			if err != nil {
				t.Fatal(err)
			}

			preview, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", c.claim, "--reason", "harness", "--dry-run", "--format", "json")
			if code != 0 {
				t.Fatalf("dry-run exited %d: %s %s", code, preview, stderr)
			}
			var dry struct {
				Data struct {
					Blocked  bool   `json:"blocked"`
					Snapshot string `json:"snapshot"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(preview), &dry); err != nil {
				t.Fatalf("parse dry-run: %v\n%s", err, preview)
			}
			if !dry.Data.Blocked || !strings.Contains(preview, "lint:"+c.lint) {
				t.Fatalf("dry-run must be blocked on lint:%s, got %s", c.lint, preview)
			}

			stdout, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", c.claim, "--reason", "harness", "--proposal", dry.Data.Snapshot, "--format", "json")
			if code != 1 {
				t.Fatalf("claim lock %s: want exit 1, got %d\nstdout: %s\nstderr: %s", c.claim, code, stdout, stderr)
			}
			var env struct {
				OK    bool `json:"ok"`
				Error *struct {
					Code    string `json:"code"`
					Details struct {
						LintFindings []capsFinding `json:"lint_findings"`
					} `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(stdout), &env); err != nil {
				t.Fatalf("parse lock envelope: %v\n%s", err, stdout)
			}
			if env.OK || env.Error == nil || env.Error.Code != "lint_failed" {
				t.Fatalf("claim lock %s: want lint_failed, got %s", c.claim, stdout)
			}
			if !hasCapsFinding(env.Error.Details.LintFindings, c.lint, c.claim) {
				t.Fatalf("claim lock %s: refusal must name %s, got %+v", c.claim, c.lint, env.Error.Details.LintFindings)
			}

			after, err := os.ReadFile(claimPath)
			if err != nil {
				t.Fatal(err)
			}
			storeAfter, err := os.ReadFile(storePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !bytes.Equal(storeBefore, storeAfter) {
				t.Fatalf("a refused lock must write nothing: claim changed=%v store changed=%v", !bytes.Equal(before, after), !bytes.Equal(storeBefore, storeAfter))
			}
		})
	}

	mustLock(t, root, cfgPath, "caps.contract.control")
	stdout, _, code := run(t, root, "--config", cfgPath, "claim", "show", "caps.contract.control", "--format", "json")
	if code != 0 || !strings.Contains(stdout, `"status": "locked"`) {
		t.Fatalf("the control claim must be locked after its lock, exit %d:\n%s", code, stdout)
	}
}

// capsClaimPath maps a fixture claim id to its file: the fixture names each
// file after its module and slug.
func capsClaimPath(t *testing.T, root, id string) string {
	t.Helper()
	parts := strings.Split(id, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected claim id %q", id)
	}
	p := filepath.Join(root, "claims", parts[0]+"-"+strings.TrimPrefix(parts[2], "c")+".yaml")
	if parts[0] != "crowded" {
		p = filepath.Join(root, "claims", parts[0]+"-"+parts[2]+".yaml")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("claim file for %s: %v", id, err)
	}
	return p
}
