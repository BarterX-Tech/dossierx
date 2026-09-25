// manifest_enforce_cli_test.go pins the NIT-22 enforce contract as decided on
// Linear NIT-7 (2026-09-24): the claim new stub fails until drafted, and a
// module-manifest finding refuses locking every claim of ITS module through
// the lock policy — and never a claim of another module.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
)

const twoModuleConfig = "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\n  - lock\nclaims_dir: claims\n"

func lintFindingMessages(t *testing.T, env cliout.Envelope) []string {
	t.Helper()
	var data struct {
		LintFindings []struct {
			Message string `json:"message"`
		} `json:"lint_findings"`
	}
	envData(t, env, &data)
	out := make([]string, 0, len(data.LintFindings))
	for _, f := range data.LintFindings {
		out = append(out, f.Message)
	}
	return out
}

func TestClaimNewStubFailsCheckUntilDrafted(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(parityConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	lockFixtureConstitution(t, cfgPath)

	// No manifest yet: claim new writes the stub beside the claim.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "new", "widget.contract.bound",
		"--body", "the widget answers within 200ms.", "--summary", "Widget answers within 200ms.", "--rests-on-none-reason", "fixture")
	if err != nil || !env.OK {
		t.Fatalf("claim new: %+v (err=%v)", env, err)
	}
	stubPath := filepath.Join(root, "claims", "widget", "manifest.yaml")
	raw, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatalf("stub not written: %v", err)
	}
	if string(raw) != string(manifest.StubYAML("widget")) {
		t.Fatalf("stub bytes drifted:\n%s", raw)
	}

	// The stub is refused, and the finding names the drafting command.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("stub must fail check --validate: err=%v env=%+v", err, env.Error)
	}
	msgs := lintFindingMessages(t, env)
	found := false
	for _, m := range msgs {
		if strings.Contains(m, "empty summary") && strings.Contains(m, manifest.DraftCommand("widget")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("finding must name the stub and the drafting command, got %v", msgs)
	}

	// manifest show --isolation is the drafting path: exit 1, same finding,
	// and the hints ride in data regardless.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "manifest", "show", "widget", "--isolation")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("show on a stub: err=%v env=%+v", err, env.Error)
	}
	var shown manifestShowData
	envData(t, env, &shown)
	if shown.Isolation == nil || len(shown.Isolation.DraftHints.SuggestedProvides) != 1 ||
		shown.Isolation.DraftHints.SuggestedProvides[0] != "widget.contract.bound" {
		t.Fatalf("draft hints must be assembled while the stub stands: %+v", shown)
	}

	// Drafted: clean.
	if err := os.WriteFile(stubPath, []byte(
		"summary: widget is the public boundary; start with bound.\nprovides:\n  - widget.contract.bound\ndepends_on: []\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil || !env.OK {
		t.Fatalf("drafted manifest must pass: %+v (err=%v)", env, err)
	}
}

// TestClaimNewStubIsPreviewedAndNeverHalfWritten: the manifest stub is a
// second file claim new writes, so --dry-run must name it, and a failure to
// write it must leave no claim on disk behind a write_failed.
func TestClaimNewStubIsPreviewedAndNeverHalfWritten(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(parityConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	lockFixtureConstitution(t, cfgPath)
	newArgs := []string{"--config", cfgPath, "claim", "new", "widget.contract.bound",
		"--body", "the widget answers within 200ms.", "--summary", "Widget answers within 200ms.", "--rests-on-none-reason", "fixture"}
	stubPath := filepath.Join(root, "claims", "widget", "manifest.yaml")
	claimPath := filepath.Join(root, "claims", "widget.contract.bound.yaml")

	env, _, err := execCLIJSON(t, append(newArgs, "--dry-run")...)
	if err != nil || !env.OK {
		t.Fatalf("claim new --dry-run: %+v (err=%v)", env, err)
	}
	var dr cliout.DryRun
	envData(t, env, &dr)
	named := false
	for _, effect := range dr.SideEffects {
		if strings.Contains(effect, "creates "+stubPath) {
			named = true
		}
	}
	if !named {
		t.Fatalf("the dry-run plan must name the manifest stub %s, got %v", stubPath, dr.SideEffects)
	}
	if fileExists(stubPath) || fileExists(claimPath) {
		t.Fatalf("--dry-run must write nothing")
	}

	// claims/widget is a FILE, so the stub's directory cannot be created.
	if err := os.WriteFile(filepath.Join(root, "claims", "widget"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err = execCLIJSON(t, newArgs...)
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeWriteFailed {
		t.Fatalf("a failed stub write must refuse with write_failed: err=%v env=%+v", err, env.Error)
	}
	if fileExists(claimPath) {
		t.Fatalf("a failed stub write must leave no claim behind at %s", claimPath)
	}
}

func TestModuleManifestFindingBlocksOnlyItsOwnModulesLocks(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, twoModuleConfig, map[string]string{
		"claims/bound.yaml": "id: widget.contract.bound\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Widget answers within 200ms.\n" +
			"body: |\n  the widget answers within 200ms.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
		"claims/store.yaml": "id: lock.contract.store\nfacet: contract\nmodule: lock\nstatus: draft\nlayout: card\nsummary: The lock store is append-only.\n" +
			"body: |\n  the lock store is append-only.\n" +
			"rests_on:\n  none: true\n  reason: fixture\n",
	})
	// lock's manifest is the untouched stub; widget's is drafted.
	if err := manifest.WriteStub(filepath.Join(root, "claims"), "lock"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "claims", "widget", "manifest.yaml"), []byte(
		"summary: widget is the public boundary.\nprovides:\n  - widget.contract.bound\ndepends_on: []\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	// A claim of the module whose manifest is a stub cannot lock.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "lock.contract.store", "--reason", "go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("stub module must refuse lock: err=%v env=%+v", err, env.Error)
	}
	if !strings.Contains(env.Error.Message, "refused") {
		t.Fatalf("message: %s", env.Error.Message)
	}

	// A claim of the OTHER module locks: lock's defect is not widget's hostage.
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.bound", "--reason", "go")
	if err != nil || !env.OK {
		t.Fatalf("another module's stub must not block this lock: %+v (err=%v)", env, err)
	}

	// Draft lock's manifest: its claim locks now.
	if err := os.WriteFile(filepath.Join(root, "claims", "lock", "manifest.yaml"), []byte(
		"summary: the lock store.\nprovides:\n  - lock.contract.store\ndepends_on: []\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "lock.contract.store", "--reason", "go")
	if err != nil || !env.OK {
		t.Fatalf("drafted module must lock: %+v (err=%v)", env, err)
	}
}
