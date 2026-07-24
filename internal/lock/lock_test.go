package lock

import (
	"os"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// failingLint is a test-only lint.Lint that always reports one finding,
// used to exercise Lock's lint gate without depending on any of the real
// (later-phase) lint rules.
type failingLint struct{}

func (failingLint) Name() string { return "test-failing-lint" }
func (failingLint) Check(claims []model.Claim, cfg *config.Config) []lint.Finding {
	return []lint.Finding{{LintName: "test-failing-lint", ClaimID: "any", Message: "forced failure"}}
}

// warningOnlyLint is a test-only lint.Lint that always reports a single
// warning-severity finding, used to prove Lock's gate only refuses on
// error-severity findings — matching "dossierx lint"/"dossierx check"'s own
// pass/fail semantics (see reportLintFindings in cmd/dossierx/main.go) —
// rather than refusing on any finding at all regardless of severity.
type warningOnlyLint struct{}

func (warningOnlyLint) Name() string { return "test-warning-only-lint" }
func (warningOnlyLint) Check(claims []model.Claim, cfg *config.Config) []lint.Finding {
	return []lint.Finding{{LintName: "test-warning-only-lint", ClaimID: "any", Message: "advisory only", Severity: lint.SeverityWarning}}
}

func withRegistry(t *testing.T, lints ...lint.Lint) {
	t.Helper()
	orig := lint.Registry
	lint.Registry = lints
	t.Cleanup(func() { lint.Registry = orig })
}

func testConfig() *config.Config {
	return &config.Config{
		SchemaVersion: config.CurrentSchemaVersion,
		Facets:        []string{"contract", "internals"},
		Modules:       []string{"widget"},
		ClaimsDir:     "claims",
	}
}

func testConfigWithDoctrine() *config.Config {
	cfg := testConfig()
	cfg.Facets = append(cfg.Facets, "doctrine")
	cfg.DoctrineFacet = "doctrine"
	return cfg
}

func TestLockFailsOnLintError(t *testing.T) {
	withRegistry(t, failingLint{})

	claim := model.Claim{ID: "widget.contract.overview", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	claims := []model.Claim{claim}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := Lock(claim, claims, testConfig(), store)
	if err == nil {
		t.Fatalf("expected Lock to be refused when lint has findings, got nil error")
	}
	if got.Status != model.StatusDraft {
		t.Fatalf("expected claim to remain draft on refused lock, got status %q", got.Status)
	}
}

// TestLockSucceedsWithOnlyWarningFindings proves Lock's lint gate mirrors
// "dossierx lint"/"dossierx check"'s own pass/fail semantics: a claim with only
// warning-severity findings against it (e.g. the real "orphan" lint) must
// still be lockable, exactly as "dossierx lint" would exit 0 for it. Before
// the fix, Lock refused on len(findings) > 0 regardless of severity, so
// this test fails against that code (any warning-only finding blocked
// every lock forever).
func TestLockSucceedsWithOnlyWarningFindings(t *testing.T) {
	withRegistry(t, warningOnlyLint{})

	claim := model.Claim{ID: "widget.contract.overview", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	claims := []model.Claim{claim}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := Lock(claim, claims, testConfig(), store)
	if err != nil {
		t.Fatalf("expected Lock to succeed with only warning-severity findings, got error: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected claim status locked, got %q", got.Status)
	}
}

func TestLockSucceedsWithEmptyLintRegistry(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "dep body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{dep.ID}}
	claims := []model.Claim{claim, dep}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := Lock(claim, claims, testConfig(), store)
	if err != nil {
		t.Fatalf("Lock: unexpected error: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}
	if got.ReviewPending {
		t.Fatalf("expected review_pending false on fresh lock")
	}
	if h, ok := store.Baseline(claim.ID, dep.ID); !ok || h != ContentHash(dep) {
		t.Fatalf("expected store to record dependency baseline hash under the dependent claim's own id")
	}
}

func TestDependencyChangeFlipsToReviewPendingNeverDraft(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "original body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: ContentHash(dep)}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}

	// Dependency content changes underneath the locked claim.
	dep.Body = "changed body"
	claims := []model.Claim{claim, dep}

	out := DetectStale(claims, store)
	if len(out) != 2 {
		t.Fatalf("expected 2 claims back, got %d", len(out))
	}

	var updated model.Claim
	for _, c := range out {
		if c.ID == claim.ID {
			updated = c
		}
	}

	if updated.Status != model.StatusLocked {
		t.Fatalf("expected status to remain locked, got %q (must never auto-revert to draft)", updated.Status)
	}
	if !updated.ReviewPending {
		t.Fatalf("expected review_pending true after dependency content changed")
	}
}

func TestDetectStaleLeavesUnaffectedClaimsAlone(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "stable"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}
	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: ContentHash(dep)}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}

	claims := []model.Claim{claim, dep}
	out := DetectStale(claims, store)

	for _, c := range out {
		if c.ID == claim.ID && c.ReviewPending {
			t.Fatalf("expected review_pending to stay false when dependency content is unchanged")
		}
	}
}

func TestHubGatingBlocksWhenConfigured(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusDraft}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{hub.ID}}
	claims := []model.Claim{hub, child}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	_, err = Lock(child, claims, testConfigWithDoctrine(), store)
	if err == nil {
		t.Fatalf("expected Lock to be refused: doctrine hub is not yet locked")
	}

	// Once the hub is locked, locking the child should succeed.
	hub.Status = model.StatusLocked
	claims = []model.Claim{hub, child}
	got, err := Lock(child, claims, testConfigWithDoctrine(), store)
	if err != nil {
		t.Fatalf("expected Lock to succeed once doctrine hub is locked, got: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}
}

func TestHubGatingSkippedWhenNotConfigured(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "internals", Module: "widget", Status: model.StatusDraft}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{hub.ID}}
	claims := []model.Claim{hub, child}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// cfg has no DoctrineFacet set at all: hub-gating must not run, so this
	// lock succeeds even though "hub" (which isn't even a doctrine claim
	// here) is still draft.
	got, err := Lock(child, claims, testConfig(), store)
	if err != nil {
		t.Fatalf("expected Lock to succeed when hub-gating is not configured, got: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}
}

// TestLockEvaluatesLintsAgainstCandidatesPostLockStatus proves Lock lints
// against the claim as it will look once locked, not against its still-
// draft entry in the input claims slice. Using the real RestOnLockedLint
// (which only fires for a claim whose OWN status is already locked): if
// Lock evaluated lint against claim's pre-lock (draft) status, this lint
// could never see the candidate as locked and would never block it, no
// matter what its rests_on target's status was — silently defeating the
// whole point of rest-on-locked. Commenting out the withLockedCandidate
// substitution in Lock makes this test fail.
func TestLockEvaluatesLintsAgainstCandidatesPostLockStatus(t *testing.T) {
	withRegistry(t, lint.RestOnLockedLint{})

	target := model.Claim{ID: "widget.contract.target", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	candidate := model.Claim{ID: "widget.contract.candidate", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{target.ID}}
	claims := []model.Claim{target, candidate}

	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	if _, err := Lock(candidate, claims, testConfig(), store); err == nil {
		t.Fatalf("expected Lock to be refused: candidate would rest_on a still-draft target once locked")
	}

	// Locking the target first, then the candidate, must succeed.
	target.Status = model.StatusLocked
	claims = []model.Claim{target, candidate}
	got, err := Lock(candidate, claims, testConfig(), store)
	if err != nil {
		t.Fatalf("expected Lock to succeed once target is locked, got: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}
}

func TestUnlockAlwaysAllowed(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true}
	got := Unlock(claim)
	if got.Status != model.StatusDraft {
		t.Fatalf("expected status draft after Unlock, got %q", got.Status)
	}
	if got.ReviewPending {
		t.Fatalf("expected review_pending cleared after Unlock")
	}
}

func TestClearReviewPendingRefreshesHashesAndKeepsLocked(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "v2 body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: []string{dep.ID}}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: "stale-hash"}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	claims := []model.Claim{claim, dep}

	got := ClearReviewPending(claim, claims, store)

	if got.ReviewPending {
		t.Fatalf("expected review_pending cleared")
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status to remain locked, got %q", got.Status)
	}
	if h, ok := store.Baseline(claim.ID, dep.ID); !ok || h != ContentHash(dep) {
		t.Fatalf("expected store baseline hash refreshed to current dependency content")
	}
}

// TestPerDependentBaselineNotSharedAcrossDependents is the DX-AUD-09
// regression: two locked claims A and B both rest_on the same dependency D.
// A locks against D's v1 content; D then drifts to v2; B locks against v2.
// Because baselines are keyed PER DEPENDENT, B's lock must NOT overwrite A's
// baseline for D, so a later DetectStale still flips A (whose recorded D
// content is stale) while leaving B (which baselined against the current D)
// alone. Under the old shared-key store (store.Hashes[depID] alone) B's lock
// clobbered the single D baseline and A never flipped — the masked bug this
// test pins. It uses only Lock/DetectStale/LoadStore (never the store's
// internal representation) so it compiles against, and fails on, the pre-fix
// code too.
func TestPerDependentBaselineNotSharedAcrossDependents(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	a := model.Claim{ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{dep.ID}}
	b := model.Claim{ID: "widget.contract.b", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{dep.ID}}

	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// A locks against D v1.
	lockedA, err := Lock(a, []model.Claim{dep, a, b}, testConfig(), store)
	if err != nil {
		t.Fatalf("lock A: %v", err)
	}

	// D drifts to v2, then B locks against v2.
	dep.Body = "dep v2"
	lockedB, err := Lock(b, []model.Claim{dep, lockedA, b}, testConfig(), store)
	if err != nil {
		t.Fatalf("lock B: %v", err)
	}

	out := DetectStale([]model.Claim{dep, lockedA, lockedB}, store)
	var gotA, gotB model.Claim
	for _, c := range out {
		switch c.ID {
		case a.ID:
			gotA = c
		case b.ID:
			gotB = c
		}
	}
	if !gotA.ReviewPending {
		t.Fatalf("expected A to flip review_pending: its shared dependency drifted after A locked, and B's later lock must not have overwritten A's baseline")
	}
	if gotB.ReviewPending {
		t.Fatalf("expected B to stay clean: it baselined against the current dependency content")
	}
}

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	path := t.TempDir() + "/store.json"
	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (missing file): %v", err)
	}
	if len(store.Hashes) != 0 {
		t.Fatalf("expected empty store for missing file")
	}
	if store.Version != storeSchemaVersion {
		t.Fatalf("expected a fresh store to carry the current schema version %d, got %d", storeSchemaVersion, store.Version)
	}

	store.recordBaseline("widget.contract.main", "widget.contract.dep", "abc123")
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if reloaded.Version != storeSchemaVersion {
		t.Fatalf("expected reloaded store to carry the current schema version, got %d", reloaded.Version)
	}
	if h, ok := reloaded.Baseline("widget.contract.main", "widget.contract.dep"); !ok || h != "abc123" {
		t.Fatalf("expected reloaded store to contain saved per-dependent hash, got %v", reloaded.Hashes)
	}
}

// TestLoadStoreMigratesLegacyFlatFormat is the DX-AUD-09 migration
// regression: an existing (pre-versioning) store file carries no "version"
// field and a legacy flat map[depID]hash. LoadStore must not crash on it, must
// present it as an already-migrated current-version store, must DROP the
// legacy flat hashes (they can't be safely re-keyed per-dependent), and must
// preserve locked_at. A subsequent DetectStale must therefore report NO
// spurious review_pending for a dependent whose (legacy-recorded) dependency
// has since drifted — the safe outcome per LoadStore's migration doc.
func TestLoadStoreMigratesLegacyFlatFormat(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "dep v2 (already drifted from what the legacy store recorded)"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	// Hand-write a legacy flat-format store: no "version", "hashes" keyed by
	// dependency id alone, with a hash that no longer matches dep's content.
	path := t.TempDir() + "/store.json"
	legacy := `{
  "hashes": {
    "widget.contract.dep": "legacy-hash-recorded-at-mains-lock"
  },
  "locked_at": {
    "widget.contract.main": "2020-01-01T00:00:00Z"
  }
}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy store: %v", err)
	}

	store, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore must not error on a legacy flat store: %v", err)
	}
	if store.Version != storeSchemaVersion {
		t.Fatalf("expected legacy store migrated to current schema version %d, got %d", storeSchemaVersion, store.Version)
	}
	if len(store.Hashes) != 0 {
		t.Fatalf("expected legacy flat hashes dropped on migration, got %v", store.Hashes)
	}
	if store.LockedAt["widget.contract.main"] == "" {
		t.Fatalf("expected locked_at preserved across migration, got %v", store.LockedAt)
	}

	out := DetectStale([]model.Claim{main, dep}, store)
	for _, c := range out {
		if c.ID == main.ID && c.ReviewPending {
			t.Fatalf("expected NO spurious review_pending after migrating a legacy store (baseline was dropped; the claim re-baselines on its next lock)")
		}
	}
}
