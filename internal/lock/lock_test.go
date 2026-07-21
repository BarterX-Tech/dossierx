package lock

import (
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
// error-severity findings — matching "docs lint"/"docs check"'s own
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
// "docs lint"/"docs check"'s own pass/fail semantics: a claim with only
// warning-severity findings against it (e.g. the real "orphan" lint) must
// still be lockable, exactly as "docs lint" would exit 0 for it. Before
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
	if store.Hashes[dep.ID] != ContentHash(dep) {
		t.Fatalf("expected store to record dependency baseline hash")
	}
}

func TestDependencyChangeFlipsToReviewPendingNeverDraft(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "original body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	store := &Store{Hashes: map[string]string{dep.ID: ContentHash(dep)}, path: t.TempDir() + "/store.json"}

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
	store := &Store{Hashes: map[string]string{dep.ID: ContentHash(dep)}, path: t.TempDir() + "/store.json"}

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

	store := &Store{Hashes: map[string]string{dep.ID: "stale-hash"}, path: t.TempDir() + "/store.json"}
	claims := []model.Claim{claim, dep}

	got := ClearReviewPending(claim, claims, store)

	if got.ReviewPending {
		t.Fatalf("expected review_pending cleared")
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status to remain locked, got %q", got.Status)
	}
	if store.Hashes[dep.ID] != ContentHash(dep) {
		t.Fatalf("expected store baseline hash refreshed to current dependency content")
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

	store.Hashes["widget.contract.dep"] = "abc123"
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore (reload): %v", err)
	}
	if reloaded.Hashes["widget.contract.dep"] != "abc123" {
		t.Fatalf("expected reloaded store to contain saved hash, got %v", reloaded.Hashes)
	}
}
