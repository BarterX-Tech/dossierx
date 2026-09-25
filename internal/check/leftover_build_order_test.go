// leftover_build_order_test.go owns the one contract the removed build-order
// product left behind: a project upgraded from v0.7.20 or earlier may still
// carry build/build-order/<module>.json files and lock-ledger rows with
// "subject": "build-order" (key "build-order:<module>"). Nothing reads them any
// more, and nothing may refuse over them — no finding, no next step, and check
// does not fail, whether the worktree or the index is judged.
//
// The fixtures write both leftovers as raw bytes: the product has no writer for
// either, so the only honest way to produce them is the way an old release left
// them on disk.
package check_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// leftoverArtifact is a locked build order exactly as v0.7.20 wrote one.
const leftoverArtifact = `{"module":"widget","locked":true,"phases":[{"name":"schema","claims":["widget.contract.a"]}]}`

// writeLeftoverArtifact drops raw bytes at build/build-order/<module>.json.
func writeLeftoverArtifact(t *testing.T, cfg *config.Config, module, raw string) {
	t.Helper()
	dir := filepath.Join(cfg.Dir(), config.DefaultBuildDir, "build-order")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir leftover dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, module+".json"), []byte(raw), 0o644); err != nil {
		t.Fatalf("write leftover artifact: %v", err)
	}
}

// writeLeftoverLedgerRow adds a raw "build-order:<module>" row to the lock
// store, beside whatever claim records the fixture armed, signed over
// leftoverArtifact as the old lock verb signed it. extra fields (a release)
// are merged into the row.
func writeLeftoverLedgerRow(t *testing.T, cfg *config.Config, module string, extra map[string]any) {
	t.Helper()
	path := cfg.LockStorePath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock store: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse lock store: %v", err)
	}
	ledger, ok := doc["ledger"].(map[string]any)
	if !ok || ledger == nil {
		ledger = map[string]any{}
	}
	sum := sha256.Sum256([]byte(leftoverArtifact))
	row := map[string]any{
		"subject": "build-order",
		"hash":    hex.EncodeToString(sum[:]),
		"at":      "2026-07-01T00:00:00Z",
		"actor":   "fixture",
		"reason":  "locked by v0.7.20",
	}
	for k, v := range extra {
		row[k] = v
	}
	ledger["build-order:"+module] = row
	doc["ledger"] = ledger
	edited, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal lock store: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write lock store: %v", err)
	}
	// A row the store refused to decode would make every case pass for the
	// wrong reason: the leftover has to be THERE and still be ignored.
	store, err := lock.LoadStore(path)
	if err != nil {
		t.Fatalf("fixture precondition: the store must still load with the leftover row: %v", err)
	}
	if r, ok := store.Record("build-order:" + module); !ok || r.Subject != "build-order" {
		t.Fatalf("fixture precondition: the leftover row did not survive decoding, got %+v (present=%v)", r, ok)
	}
}

// buildOrderLeaks returns every rule id, lint name or next step in res that
// still speaks about a build order.
func buildOrderLeaks(res check.Result) []string {
	var leaks []string
	for _, f := range res.LedgerFindings {
		if strings.HasPrefix(f.Rule, "build-order") {
			leaks = append(leaks, "ledger rule "+f.Rule)
		}
	}
	for _, f := range res.LintFindings {
		if strings.HasPrefix(f.LintName, "build-order") {
			leaks = append(leaks, "lint "+f.LintName)
		}
	}
	for _, h := range res.NextSteps {
		low := strings.ToLower(h)
		if strings.Contains(low, "build-order") || strings.Contains(low, "build order") {
			leaks = append(leaks, "next step "+h)
		}
	}
	return leaks
}

// Every leftover shape an old project can hold — the honest one, each tamper
// the removed gate used to catch, and the pre-ledger store whose only locked
// thing is a build order — is ignored by check, in the worktree and in the
// index alike.
func TestLeftoverBuildOrderArtifactsAreIgnored(t *testing.T) {
	released := map[string]any{
		"released_at":     "2026-07-02T00:00:00Z",
		"released_by":     "fixture",
		"released_reason": "re-proposing",
	}
	cases := []struct {
		name     string
		claim    string         // claims/a.yaml
		artifact string         // "" writes no artifact
		module   string         // the leftover's module; "" is widget
		row      bool           // a standing build-order ledger row
		rowExtra map[string]any // merged into the row
		preLedge bool           // downgrade the store to pre-ledger
	}{
		{name: "locked artifact with a standing row", claim: lockedClaim("widget.contract.a"), artifact: leftoverArtifact, row: true},
		{name: "hand-edited artifact", claim: lockedClaim("widget.contract.a"), artifact: `{"module":"widget","locked":true,"phases":[],"excluded":["widget.contract.smuggled"]}`, row: true},
		{name: "ledger row deleted", claim: lockedClaim("widget.contract.a"), artifact: leftoverArtifact},
		{name: "artifact deleted, row standing", claim: lockedClaim("widget.contract.a"), row: true},
		{name: "module no longer in the config", claim: lockedClaim("widget.contract.a"), artifact: leftoverArtifact, module: "retired", row: true},
		{name: "released row", claim: lockedClaim("widget.contract.a"), row: true, rowExtra: released},
		{name: "locked:false artifact under a standing row", claim: lockedClaim("widget.contract.a"), artifact: `{"module":"widget","locked":false,"phases":[]}`, row: true},
		{name: "corrupt artifact", claim: lockedClaim("widget.contract.a"), artifact: `{ "module": "widget", "locked": tr`, row: true},
		{name: "pre-ledger store whose only locked thing is a build order", claim: draftClaim("widget.contract.a"), artifact: leftoverArtifact, preLedge: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := project(t, baseConfig, map[string]string{"claims/a.yaml": tc.claim})
			module := tc.module
			if module == "" {
				module = "widget"
			}
			if tc.artifact != "" {
				writeLeftoverArtifact(t, cfg, module, tc.artifact)
			}
			if tc.row {
				writeLeftoverLedgerRow(t, cfg, module, tc.rowExtra)
			}
			if tc.preLedge {
				downgradeLockStore(t, cfg, false)
			}
			claims := reload(t, cfg)

			assertIgnored := func(mode string, res check.Result) {
				t.Helper()
				if len(res.LedgerFindings) != 0 {
					t.Fatalf("%s: leftover build-order state must produce no ledger finding, got %v", mode, rulesOf(res.LedgerFindings))
				}
				if n := len(filterErrors(res.LintFindings)); n != 0 {
					t.Fatalf("%s: leftover build-order state must not fail lint, got %v", mode, res.LintFindings)
				}
				if leaks := buildOrderLeaks(res); len(leaks) != 0 {
					t.Fatalf("%s: check still speaks about build orders: %v", mode, leaks)
				}
			}

			assertIgnored("status", check.Status(claims, cfg))

			gitRepo(t, cfg.Dir())
			git(t, cfg.Dir(), "add", "-A")
			git(t, cfg.Dir(), "commit", "-qm", "an upgraded project with leftovers")
			sp, err := check.Staged(cfg)
			if err != nil {
				t.Fatalf("--staged: %v", err)
			}
			assertIgnored("--staged", check.StatusStaged(sp, cfg))

			res, err := check.Run(claims, cfg)
			if err != nil {
				t.Fatalf("run: leftover build-order state must not fail check: %v", err)
			}
			assertIgnored("run", res)
		})
	}
}

// filterErrors keeps the error-severity lint findings, the ones that fail check.
func filterErrors(findings []lint.Finding) []lint.Finding {
	var out []lint.Finding
	for _, f := range findings {
		if f.Severity != lint.SeverityWarning {
			out = append(out, f)
		}
	}
	return out
}
