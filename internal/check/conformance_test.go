package check_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/layout"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

func readConformanceFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRunConformanceWritesAgreementThenRemovesStaleStatus(t *testing.T) {
	claimPath := "claims/state.yaml"
	nonePath := "claims/declared-none.yaml"
	plain := draftClaim("widget.contract.state")
	plainNone := draftClaim("widget.contract.declared-none")
	withDeclaration := plain + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [blocked, ready]\n"
	cfg, claims := project(t, baseConfig+"conformance:\n  observations: observations.json\n", map[string]string{
		claimPath:           withDeclaration,
		nonePath:            plainNone + "embodiment:\n  mode: none\n  reason: documentation-only neutral fixture\n",
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready","paused"]}]}`,
	})
	res, err := check.Run(claims, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conformance == nil || res.Conformance.Summary.Mismatch != 1 || res.Conformance.Summary.DeclaredNone != 1 || res.ConformancePath == "" {
		t.Fatalf("conformance result = %+v", res.Conformance)
	}
	statusRaw, err := os.ReadFile(cfg.ConformanceStatusPath())
	if err != nil || !strings.Contains(string(statusRaw), `"missing": [`) || !strings.Contains(string(statusRaw), `"extra": [`) {
		t.Fatalf("status = %s, err=%v", statusRaw, err)
	}
	var statusReport conformance.Report
	if err := json.Unmarshal(statusRaw, &statusReport); err != nil || !reflect.DeepEqual(statusReport, *res.Conformance) {
		t.Fatalf("status disagrees with in-memory result: err=%v\nstatus=%+v\nresult=%+v", err, statusReport, res.Conformance)
	}
	catalogRaw := readConformanceFile(t, cfg.CatalogPath())
	viewerRaw := readConformanceFile(t, cfg.ViewerPath())
	for name, raw := range map[string][]byte{"status": statusRaw, "catalog": catalogRaw, "viewer": viewerRaw} {
		if !strings.Contains(string(raw), "blocked") || !strings.Contains(string(raw), "paused") || !strings.Contains(string(raw), "documentation-only neutral fixture") {
			t.Fatalf("%s did not project exact difference", name)
		}
	}
	var catalogDoc struct {
		Claims []struct {
			ID          string              `json:"id"`
			Conformance *conformance.Result `json:"conformance"`
		} `json:"claims"`
	}
	if err := json.Unmarshal(catalogRaw, &catalogDoc); err != nil {
		t.Fatal(err)
	}
	catalogResults := map[string]conformance.Result{}
	for _, entry := range catalogDoc.Claims {
		if entry.Conformance != nil {
			catalogResults[entry.ID] = *entry.Conformance
		}
	}
	for _, result := range statusReport.Results {
		if got, ok := catalogResults[result.ClaimID]; !ok || !reflect.DeepEqual(got, result) {
			t.Fatalf("catalog disagrees for %s: %+v vs %+v", result.ClaimID, got, result)
		}
	}

	if err := os.WriteFile(filepath.Join(cfg.Dir(), claimPath), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir(), nonePath), []byte(plainNone), 0o644); err != nil {
		t.Fatal(err)
	}
	claims, err = loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatal(err)
	}
	res, err = check.Run(claims, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Conformance != nil || res.ConformancePath != "" {
		t.Fatalf("zero-declaration run retained conformance: %+v", res.Conformance)
	}
	if _, err := os.Stat(cfg.ConformanceStatusPath()); !os.IsNotExist(err) {
		t.Fatalf("stale status still exists: %v", err)
	}
	gitignoreRaw, err := os.ReadFile(cfg.BuildGitignorePath())
	if err != nil || string(gitignoreRaw) != layout.BuildGitignoreContent {
		t.Fatalf("zero-declaration transition did not restore historical build/.gitignore bytes: %q err=%v", gitignoreRaw, err)
	}
	catalogRaw = readConformanceFile(t, cfg.CatalogPath())
	viewerRaw = readConformanceFile(t, cfg.ViewerPath())
	if strings.Contains(string(catalogRaw), `"conformance"`) || strings.Contains(string(viewerRaw), "claim-conformance") {
		t.Fatal("zero-declaration outputs retained a conformance projection")
	}
}

func TestConformanceNeverMutatesClaimLedgerOrExistingReadiness(t *testing.T) {
	claimPath := "claims/state.yaml"
	declaration := "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"
	cfg, claims := project(t, baseConfig+"conformance:\n  observations: observations.json\n", map[string]string{
		claimPath:           lockedClaim("widget.contract.state") + declaration,
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`,
	})
	claimFile := filepath.Join(cfg.Dir(), claimPath)
	claimBefore := readConformanceFile(t, claimFile)
	ledgerBefore := readConformanceFile(t, cfg.LockStorePath())
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatal(err)
	}
	readyMatched := catalogReadiness(t, cfg)
	if err := os.WriteFile(cfg.Conformance.Observations, []byte(`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["blocked"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatal(err)
	}
	readyMismatch := catalogReadiness(t, cfg)
	claimAfter := readConformanceFile(t, claimFile)
	ledgerAfter := readConformanceFile(t, cfg.LockStorePath())
	if !bytes.Equal(claimBefore, claimAfter) || !bytes.Equal(ledgerBefore, ledgerAfter) {
		t.Fatal("conformance evaluation mutated a claim or lock ledger")
	}
	if !bytes.Equal(readyMatched, readyMismatch) {
		t.Fatalf("observation result changed existing readiness:\nmatched=%s\nmismatch=%s", readyMatched, readyMismatch)
	}
}

func TestRunNoDeclarationDoesNotTouchUnknownConformancePath(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/plain.yaml": draftClaim("widget.contract.plain"),
	})
	if err := os.MkdirAll(filepath.Dir(cfg.ConformanceStatusPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("not a dossierx conformance artifact")
	if err := os.WriteFile(cfg.ConformanceStatusPath(), sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.BuildGitignorePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	spoofedIgnore := strings.Replace(layout.BuildGitignoreContent, "catalog/\n", "catalog/\nconformance/\n", 1)
	if err := os.WriteFile(cfg.BuildGitignorePath(), []byte(spoofedIgnore), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfg.ConformanceStatusPath())
	if err != nil || !bytes.Equal(got, sentinel) {
		t.Fatalf("never-opted path changed: %q err=%v", got, err)
	}
}

func TestRunOptOutCleanupFailureKeepsMarkerForRetry(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/plain.yaml": draftClaim("widget.contract.plain"),
	})
	if err := layout.MarkConformanceStatusOwned(cfg); err != nil {
		t.Fatal(err)
	}
	statusPath := cfg.ConformanceStatusPath()
	if err := os.MkdirAll(statusPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(statusPath, "stale"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err == nil {
		t.Fatal("expected stale non-empty directory cleanup to fail")
	}
	enabled, err := layout.ConformanceStatusOwned(cfg)
	if err != nil || !enabled {
		t.Fatalf("transition marker lost after failed cleanup: enabled=%v err=%v", enabled, err)
	}
	if err := os.RemoveAll(statusPath); err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := os.Stat(statusPath); !os.IsNotExist(err) {
		t.Fatalf("stale status remains after retry: %v", err)
	}
	enabled, err = layout.ConformanceStatusOwned(cfg)
	if err != nil || enabled {
		t.Fatalf("transition marker not downgraded after retry: enabled=%v err=%v", enabled, err)
	}
}

func TestRunOptOutUsesOwnershipMarkerWithCustomGitignore(t *testing.T) {
	claimPath := "claims/state.yaml"
	plain := draftClaim("widget.contract.state")
	declared := plain + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"
	cfg, claims := project(t, baseConfig+"conformance:\n  observations: observations.json\n", map[string]string{
		claimPath:           declared,
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`,
	})
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatal(err)
	}
	owned, err := layout.ConformanceStatusOwned(cfg)
	if err != nil || !owned {
		t.Fatalf("generated status ownership = %v, err=%v", owned, err)
	}
	custom := []byte("# project policy\n*.scratch\n")
	if err := os.WriteFile(cfg.BuildGitignorePath(), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir(), claimPath), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	claims, err = loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(cfg.ConformanceStatusPath()); !os.IsNotExist(err) {
		t.Fatalf("owned stale status survived custom gitignore: %v", err)
	}
	if owned, err := layout.ConformanceStatusOwned(cfg); err != nil || owned {
		t.Fatalf("ownership marker survived cleanup: owned=%v err=%v", owned, err)
	}
	if got, err := os.ReadFile(cfg.BuildGitignorePath()); err != nil || !bytes.Equal(got, custom) {
		t.Fatalf("custom gitignore changed: %q err=%v", got, err)
	}
}

func TestRunOwnershipConflictIsPreflightedAndLifecycleRetriesCleanly(t *testing.T) {
	claimPath := "claims/state.yaml"
	plain := draftClaim("widget.contract.state")
	declared := plain + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n"
	cfg, claims := project(t, baseConfig+"conformance:\n  observations: observations.json\n", map[string]string{
		claimPath:           declared,
		"observations.json": `{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://state","shape":"set","value":["ready"]}]}`,
	})
	for path, value := range map[string][]byte{
		cfg.CatalogPath():           []byte("prior catalog"),
		cfg.ConformanceStatusPath(): nil,
		cfg.ViewerPath():            []byte("prior viewer"),
	} {
		if value == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, value, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	marker := layout.ConformanceOwnershipPath(cfg)
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := check.Run(claims, cfg)
	if err == nil || res.ConformanceFailurePhase != "conformance" || res.CatalogPath != "" || res.RenderPath != "" || res.ConformancePath != "" {
		t.Fatalf("conflict result=%+v err=%v", res, err)
	}
	for path, want := range map[string]string{cfg.CatalogPath(): "prior catalog", cfg.ViewerPath(): "prior viewer"} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != want {
			t.Fatalf("preflight replaced %s: %q err=%v", path, got, readErr)
		}
	}
	if err := os.RemoveAll(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("opted-in retry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfg.Dir(), claimPath), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	claims, err = loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("opt-out retry: %v", err)
	}
	if _, err := os.Stat(cfg.ConformanceStatusPath()); !os.IsNotExist(err) {
		t.Fatalf("stale status remains after recovered lifecycle: %v", err)
	}
	if owned, err := layout.ConformanceStatusOwned(cfg); err != nil || owned {
		t.Fatalf("ownership remains after recovered lifecycle: owned=%v err=%v", owned, err)
	}
}

func TestConformanceObservationParentSymlinkIntoBuildIsUncheckable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	cfg, claims := project(t, baseConfig+"conformance:\n  observations: input-alias/observations.json\n", map[string]string{
		"claims/state.yaml": draftClaim("widget.contract.state") + "embodiment:\n  mode: compare\n  checks:\n    - id: state\n      adapter: neutral/v1\n      target: widget://state\n      expectation:\n        shape: set\n        value: [ready]\n",
	})
	if err := os.MkdirAll(cfg.BuildDirPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.BuildDirPath(), "observations.json"), []byte(`{"format_version":1,"observations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cfg.BuildDirPath(), filepath.Join(cfg.Dir(), "input-alias")); err != nil {
		t.Fatal(err)
	}
	res := check.Status(claims, cfg)
	if res.Conformance == nil || res.Conformance.Results[0].Checks[0].State != conformance.StateUncheckable || res.Conformance.Results[0].Checks[0].Reason != "observation input is unavailable" {
		t.Fatalf("symlinked observation result = %+v", res.Conformance)
	}
}

func catalogReadiness(t *testing.T, cfg *config.Config) json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(cfg.CatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Claims []struct {
			Readiness json.RawMessage `json:"readiness"`
		} `json:"claims"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Claims) != 1 {
		t.Fatalf("catalog: claims=%d err=%v", len(doc.Claims), err)
	}
	return append(json.RawMessage(nil), doc.Claims[0].Readiness...)
}
