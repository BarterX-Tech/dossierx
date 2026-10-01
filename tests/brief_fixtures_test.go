// brief_fixtures_test.go is lint_fixtures_test.go for the brief rule set
// (internal/briefs, NIT-204): every brief rule fires through the real CLI
// ("dossierx check --validate") against one synthetic project per rule under
// testdata/fixture-coverage/brief/<rule-name>/, and each fixture trips its own
// rule and nothing else.
//
// The brief rules are a registry of their own (briefs.Rules), beside
// lint.Registry and never inside it, so they get a corpus of their own too: the
// claim-rule corpus asserts one directory per lint.Registry entry, and a brief
// fixture there would break that count for a rule the claim registry does not
// have. The count here is read from briefs.Rules for the same reason the claim
// test reads lint.Registry — a registered rule with no fixture, or a fixture for
// a rule nobody registered, is red without anyone updating a number.
//
// The corpus also holds one fixture per INTEGRITY finding of a brief's lock
// (briefs.IntegrityRules: content drift, unrecorded, orphan, abandoned), which
// ride in ledger_findings rather than lint_findings: each fails check with its
// own ledger finding and nothing else, lint or ledger.
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/lint"
)

func TestBriefRuleCoverageFixtures(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "testdata", "fixture-coverage", "brief"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	registered := map[string]lint.Severity{}
	for _, r := range briefs.Rules {
		registered[r.Name] = r.Severity
	}
	integrity := map[string]bool{}
	for _, r := range briefs.IntegrityRules {
		integrity[r] = true
	}
	if len(entries) != len(registered)+len(integrity) {
		t.Fatalf("expected exactly one brief fixture directory per brief rule and brief integrity finding (%d + %d), found %d: %v", len(registered), len(integrity), len(entries), entries)
	}

	for _, e := range entries {
		rule := e.Name()
		if integrity[rule] && e.IsDir() {
			t.Run(rule, func(t *testing.T) {
				dir := filepath.Join(root, rule)
				stdout, stderr, code := run(t, dir, "--config", filepath.Join(dir, "project.config.yaml"), "--format", "json", "check", "--validate")
				var env struct {
					Data struct {
						LintFindings   []lintFinding `json:"lint_findings"`
						LedgerFindings []struct {
							Rule    string `json:"rule"`
							ClaimID string `json:"claim_id"`
						} `json:"ledger_findings"`
					} `json:"data"`
					Error *struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal([]byte(stdout), &env); err != nil {
					t.Fatalf("not an envelope: %v\n%s\n%s", err, stdout, stderr)
				}
				if code != 1 || env.Error == nil || env.Error.Code != "integrity_failed" {
					t.Fatalf("expected integrity_failed at exit 1, got exit %d %+v", code, env.Error)
				}
				if len(env.Data.LintFindings) != 0 || len(env.Data.LedgerFindings) != 1 || env.Data.LedgerFindings[0].Rule != rule {
					t.Fatalf("fixture %q must trip only its own ledger finding; lint %+v ledger %+v", rule, env.Data.LintFindings, env.Data.LedgerFindings)
				}
			})
			continue
		}
		severity, ok := registered[rule]
		if !e.IsDir() || !ok {
			t.Errorf("%s is not a directory named for a registered brief rule", rule)
			continue
		}
		t.Run(rule, func(t *testing.T) {
			dir := filepath.Join(root, rule)
			findings, code := runValidateFindings(t, dir, filepath.Join(dir, "project.config.yaml"))
			// A warning reports and passes; every other brief rule is final.
			wantExit := 1
			if severity == lint.SeverityWarning {
				wantExit = 0
			}
			if code != wantExit {
				t.Fatalf("expected exit %d, got %d (findings: %+v)", wantExit, code, findings)
			}
			fired := false
			for _, f := range findings {
				if f.LintName != rule {
					t.Errorf("fixture %q must trip only its own rule; also got %s on %s: %s", rule, f.LintName, f.ClaimID, f.Message)
					continue
				}
				fired = true
				if f.Severity != string(severity) {
					t.Errorf("%s reported severity %q, registered as %q", rule, f.Severity, severity)
				}
			}
			if !fired {
				t.Fatalf("expected %q to fire at least once; findings: %+v", rule, findings)
			}
		})
	}
}
