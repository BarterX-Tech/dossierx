// lifecycle_audit_cli_test.go covers the DX-AUD-10 and DX-AUD-11 lifecycle
// integrity fixes end to end, in-process via execCLI (mirroring
// cli_inprocess_test.go's style):
//
//	DX-AUD-10: "dossierx unlock" must delete the claim's pending-flag store
//	           entry, so a stale pre-unlock flag can never survive a relock
//	           and be silently applied by a later drift-triggered reaudit.
//	DX-AUD-11: "dossierx flag" must refuse a structured (table/steps/mockup)
//	           claim, whose rows/steps/raw-HTML a body-only flag-sourced
//	           reaudit would leave stale while clearing review_pending.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeLayoutClaim writes one locked claim with an explicit layout and the
// given extra YAML body lines (rows/steps/raw_html), under claimsDir.
func writeLayoutClaim(t *testing.T, claimsDir, id, layout, extra string) string {
	t.Helper()
	path := filepath.Join(claimsDir, strings.ReplaceAll(id, ".", "_")+".yaml")
	src := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: " + layout + "\n" +
		"body: |\n  a " + layout + " claim.\n" + extra +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", id, err)
	}
	return path
}

// TestCLI_Unlock_ClearsPendingFlag is the DX-AUD-10 regression: flag a locked
// claim, unlock it (which must drop the pending flag), relock it, then drift a
// dependency so a normal reaudit is triggered. Because the pre-unlock flag was
// cleared, the confirmed reaudit must NOT resurrect the stale --now-does and
// overwrite the body with it.
func TestCLI_Unlock_ClearsPendingFlag(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	depPath := filepath.Join(claimsDir, "dep.yaml")
	dep := "id: widget.contract.dep\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  dependency claim, v1.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(depPath, []byte(dep), 0o644); err != nil {
		t.Fatalf("write dep: %v", err)
	}
	mainPath := filepath.Join(claimsDir, "main.yaml")
	mainClaim := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\n" +
		"body: |\n  the real, correct main body.\n" +
		"rests_on:\n  - widget.contract.dep\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(mainPath, []byte(mainClaim), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}

	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.dep", "--reason", "test fixture"); err != nil {
		t.Fatalf("lock dep: %v", err)
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); err != nil {
		t.Fatalf("lock main: %v", err)
	}

	const stale = "STALE-NOW-DOES-MUST-NOT-APPLY"
	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "flag", "widget.contract.main",
		"--claim-says", "the real, correct main body.", "--now-does", stale, "--reason", "flagged before unlock"); err != nil {
		t.Fatalf("flag main: %v", err)
	}

	flagStorePath := filepath.Join(root, ".dossierx-flag-store.json")
	if raw, err := os.ReadFile(flagStorePath); err != nil {
		t.Fatalf("read flag store after flag: %v", err)
	} else if !strings.Contains(string(raw), "widget.contract.main") {
		t.Fatalf("expected the flag store to carry an entry for main after flag, got:\n%s", raw)
	}

	// Unlock must delete the pending flag entry for this claim.
	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture"); err != nil {
		t.Fatalf("unlock main: %v", err)
	}
	if raw, err := os.ReadFile(flagStorePath); err != nil {
		t.Fatalf("read flag store after unlock: %v", err)
	} else if strings.Contains(string(raw), "widget.contract.main") {
		t.Fatalf("expected unlock to delete main's pending flag entry, but it survived:\n%s", raw)
	}

	// Relock, then drift the dependency so a normal (dependency-drift)
	// reaudit is triggered on main.
	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); err != nil {
		t.Fatalf("relock main: %v", err)
	}
	depOnDisk, err := os.ReadFile(depPath)
	if err != nil {
		t.Fatalf("read dep: %v", err)
	}
	drifted := strings.Replace(string(depOnDisk), "dependency claim, v1.", "dependency claim, v2.", 1)
	if err := os.WriteFile(depPath, []byte(drifted), 0o644); err != nil {
		t.Fatalf("rewrite dep: %v", err)
	}
	// The in-place edit of a LOCKED dependency stands in for unlock -> edit ->
	// lock; re-record its approval so the ledger gate sees an approved change.
	armLedgerFixture(t, cfgPath)
	if _, _, err := execCLI(t, "--config", cfgPath, "check"); err != nil {
		t.Fatalf("check: %v", err)
	}

	mainPending, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if !strings.Contains(string(mainPending), "review_pending: true") {
		t.Fatalf("expected main flagged review_pending by the dependency drift, got:\n%s", mainPending)
	}

	// Confirmed reaudit: with the pre-unlock flag gone, this is a plain
	// dependency-drift (no-change stub) reaudit — it must NOT apply the stale
	// --now-does over the body.
	if _, _, err := execCLI(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main", "--confirm", "--reason", "test fixture"); err != nil {
		t.Fatalf("reaudit --confirm: %v", err)
	}
	mainAfter, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("read main: %v", err)
	}
	if strings.Contains(string(mainAfter), stale) {
		t.Fatalf("expected the stale pre-unlock --now-does to NEVER be applied after unlock/relock/drift, but it was:\n%s", mainAfter)
	}
	if !strings.Contains(string(mainAfter), "the real, correct main body.") {
		t.Fatalf("expected the real body preserved through a no-change reaudit, got:\n%s", mainAfter)
	}
}

// TestCLI_Unlock_TolerantOfFlagStore is the v0.1.2 regression for the
// deferred DX-AUD-10 follow-up: the DX-AUD-10 fix made unlock unconditionally
// load the flag store to clear a pending entry, which turned an unrelated
// broken flag-store file into a hard unlock failure. But unlock is the
// recovery escape hatch ("get this claim back to draft so I can fix things")
// and must never be blocked by that. So: an absent store means nothing to
// clear (proceed silently); an unparseable store is warned about on stderr
// and skipped (unlock still succeeds, exit 0); a valid store holding this
// claim's flag still has that entry removed (the original DX-AUD-10 behavior).
func TestCLI_Unlock_TolerantOfFlagStore(t *testing.T) {
	writeProject := func(t *testing.T) (root, cfgPath, claimPath string) {
		t.Helper()
		root = t.TempDir()
		claimsDir := filepath.Join(root, "claims")
		if err := os.MkdirAll(claimsDir, 0o755); err != nil {
			t.Fatalf("mkdir claims: %v", err)
		}
		cfgPath = filepath.Join(root, "project.config.yaml")
		if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		claimPath = writeLockedFixtureClaim(t, claimsDir, "widget.contract.main", "widget", "the real body")
		return root, cfgPath, claimPath
	}
	assertDraft := func(t *testing.T, claimPath string) {
		t.Helper()
		after, err := os.ReadFile(claimPath)
		if err != nil {
			t.Fatalf("read claim: %v", err)
		}
		if !strings.Contains(string(after), "status: draft") {
			t.Fatalf("expected the claim reverted to draft after unlock, got:\n%s", after)
		}
	}

	t.Run("absent flag store proceeds silently", func(t *testing.T) {
		root, cfgPath, claimPath := writeProject(t)
		flagStorePath := filepath.Join(root, ".dossierx-flag-store.json")
		if _, err := os.Stat(flagStorePath); !os.IsNotExist(err) {
			t.Fatalf("precondition: expected no flag store on disk, stat err=%v", err)
		}
		_, stderr, err := execCLI(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture")
		if err != nil {
			t.Fatalf("unlock with an absent flag store must succeed, got: %v", err)
		}
		if strings.Contains(stderr, "warning") {
			t.Fatalf("expected no warning when the flag store is simply absent, got stderr:\n%s", stderr)
		}
		assertDraft(t, claimPath)
	})

	t.Run("unparseable flag store warns and proceeds", func(t *testing.T) {
		root, cfgPath, claimPath := writeProject(t)
		flagStorePath := filepath.Join(root, ".dossierx-flag-store.json")
		if err := os.WriteFile(flagStorePath, []byte("{ this is not valid json"), 0o644); err != nil {
			t.Fatalf("write corrupt flag store: %v", err)
		}
		_, stderr, err := execCLI(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture")
		if err != nil {
			t.Fatalf("unlock must still succeed with a corrupt flag store, got: %v", err)
		}
		if !strings.Contains(stderr, "warning") || !strings.Contains(stderr, "widget.contract.main") {
			t.Fatalf("expected a stderr warning naming the claim whose flag was not cleared, got:\n%s", stderr)
		}
		assertDraft(t, claimPath)
	})

	t.Run("valid flag store has this claim's entry removed", func(t *testing.T) {
		root, cfgPath, claimPath := writeProject(t)
		flagStorePath := filepath.Join(root, ".dossierx-flag-store.json")
		valid := `{"flags":{` +
			`"widget.contract.main":{"claim_says":"a","now_does":"b","reason":"c","flagged_at":"2026-01-01T00:00:00Z"},` +
			`"widget.contract.other":{"claim_says":"x","now_does":"y","reason":"z","flagged_at":"2026-01-01T00:00:00Z"}}}`
		if err := os.WriteFile(flagStorePath, []byte(valid), 0o644); err != nil {
			t.Fatalf("write valid flag store: %v", err)
		}
		if _, _, err := execCLI(t, "--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture"); err != nil {
			t.Fatalf("unlock: %v", err)
		}
		raw, err := os.ReadFile(flagStorePath)
		if err != nil {
			t.Fatalf("read flag store after unlock: %v", err)
		}
		if strings.Contains(string(raw), "widget.contract.main") {
			t.Fatalf("expected unlock to remove main's flag entry, got:\n%s", raw)
		}
		if !strings.Contains(string(raw), "widget.contract.other") {
			t.Fatalf("expected unlock to leave other claims' flag entries intact, got:\n%s", raw)
		}
		assertDraft(t, claimPath)
	})
}

// TestCLI_Flag_RefusesStructuredLayouts is the DX-AUD-11 regression: flagging
// a table/steps/mockup claim must be refused (a body-only flag reaudit cannot
// update its rows/steps/raw-HTML), leaving review_pending untouched; a card
// claim is still flaggable.
func TestCLI_Flag_RefusesStructuredLayouts(t *testing.T) {
	structured := []struct {
		name, id, layout, extra string
	}{
		{"table", "widget.contract.tbl", "table", "rows:\n  - name: alpha\n    value: one\n"},
		{"steps", "widget.contract.stp", "steps", "steps:\n  - first step\n  - second step\n"},
		{"mockup", "widget.contract.mck", "mockup", "raw_html: \"<div>mock</div>\"\nraw_html_reviewed: true\n"},
	}
	for _, tc := range structured {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			claimsDir := filepath.Join(root, "claims")
			if err := os.MkdirAll(claimsDir, 0o755); err != nil {
				t.Fatalf("mkdir claims: %v", err)
			}
			cfgPath := filepath.Join(root, "project.config.yaml")
			if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}
			claimPath := writeLayoutClaim(t, claimsDir, tc.id, tc.layout, tc.extra)

			_, stderr, err := execCLI(t, "--config", cfgPath, "claim", "flag", tc.id,
				"--claim-says", "a", "--now-does", "b", "--reason", "c")
			if err == nil {
				t.Fatalf("expected flag to be refused for a %s-layout claim", tc.layout)
			}
			if !strings.Contains(stderr, "unlock") {
				t.Fatalf("expected the refusal to direct the user to unlock -> edit -> relock, got stderr: %s", stderr)
			}
			after, readErr := os.ReadFile(claimPath)
			if readErr != nil {
				t.Fatalf("read claim: %v", readErr)
			}
			if strings.Contains(string(after), "review_pending: true") {
				t.Fatalf("expected review_pending untouched on a refused flag, got:\n%s", after)
			}
		})
	}

	// A card (body-only) claim is still flaggable.
	t.Run("card still works", func(t *testing.T) {
		root := t.TempDir()
		claimsDir := filepath.Join(root, "claims")
		if err := os.MkdirAll(claimsDir, 0o755); err != nil {
			t.Fatalf("mkdir claims: %v", err)
		}
		cfgPath := filepath.Join(root, "project.config.yaml")
		if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		claimPath := writeLockedFixtureClaim(t, claimsDir, "widget.contract.card", "widget", "a plain card body")

		if _, _, err := execCLI(t, "--config", cfgPath, "claim", "flag", "widget.contract.card",
			"--claim-says", "a", "--now-does", "b", "--reason", "c"); err != nil {
			t.Fatalf("expected flag to succeed on a card (body-only) claim: %v", err)
		}
		after, err := os.ReadFile(claimPath)
		if err != nil {
			t.Fatalf("read claim: %v", err)
		}
		if !strings.Contains(string(after), "review_pending: true") {
			t.Fatalf("expected a flagged card claim to carry review_pending: true, got:\n%s", after)
		}
	})
}
