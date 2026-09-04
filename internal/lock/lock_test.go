package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
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

// testApproval is the stand-in human approval every Lock/Unlock in this
// package's tests executes. The ledger write hooks take one by value so a
// caller cannot record an approval without having something to put in it; a
// test that wants to assert the RECORDED actor/reason builds its own.
func testApproval() Approval {
	return Approval{Actor: "test-actor", Reason: "test approval"}
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

	got, err := Lock(claim, claims, testConfig(), store, testApproval())
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

	got, err := Lock(claim, claims, testConfig(), store, testApproval())
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

	got, err := Lock(claim, claims, testConfig(), store, testApproval())
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

	_, err = Lock(child, claims, testConfigWithDoctrine(), store, testApproval())
	if err == nil {
		t.Fatalf("expected Lock to be refused: doctrine hub is not yet locked")
	}

	// Once the hub is locked, locking the child should succeed.
	hub.Status = model.StatusLocked
	claims = []model.Claim{hub, child}
	got, err := Lock(child, claims, testConfigWithDoctrine(), store, testApproval())
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
	got, err := Lock(child, claims, testConfig(), store, testApproval())
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

	if _, err := Lock(candidate, claims, testConfig(), store, testApproval()); err == nil {
		t.Fatalf("expected Lock to be refused: candidate would rest_on a still-draft target once locked")
	}

	// Locking the target first, then the candidate, must succeed.
	target.Status = model.StatusLocked
	claims = []model.Claim{target, candidate}
	got, err := Lock(candidate, claims, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("expected Lock to succeed once target is locked, got: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}
}

func TestUnlockAlwaysAllowed(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.main", Status: model.StatusLocked, ReviewPending: true}
	got := Unlock(claim, nil, testApproval())
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

// TestRefreshBaselineRefreshesHashesWithoutTouchingReviewPending pins the
// ClearReviewPending split: RefreshBaseline does the re-baseline + LockedAt
// stamp half and NOTHING to the claim's ReviewPending — that verdict is the
// caller's to compute, so a claim with an independent open-comment-thread
// trigger can stay review_pending after a confirmed reaudit re-baselines it.
func TestRefreshBaselineRefreshesHashesWithoutTouchingReviewPending(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "v2 body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: []string{dep.ID}}
	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: "stale-hash"}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	claims := []model.Claim{claim, dep}

	RefreshBaseline(claim, claims, store)

	if h, ok := store.Baseline(claim.ID, dep.ID); !ok || h != ContentHash(dep) {
		t.Fatalf("expected RefreshBaseline to re-record the dependency baseline to current content")
	}
	if _, ok := store.LockedAt[claim.ID]; !ok {
		t.Fatalf("expected RefreshBaseline to stamp LockedAt")
	}
}

// TestLockRefusedOnOpenCommentThread is the comment lock gate: a claim cannot
// transition draft -> locked while it carries an unresolved comment thread, and
// the refusal names the open thread id(s). The empty registry isolates this
// gate from the (warning-only) comments-unresolved lint.
func TestLockRefusedOnOpenCommentThread(t *testing.T) {
	withRegistry(t)

	claim := model.Claim{
		ID: "widget.contract.overview", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Comments: []model.Comment{{ID: "c-aaa111", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "clarify"}},
	}
	claims := []model.Claim{claim}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := Lock(claim, claims, testConfig(), store, testApproval())
	if err == nil {
		t.Fatalf("expected Lock to be refused for a claim with an open comment thread")
	}
	if !strings.Contains(err.Error(), "c-aaa111") {
		t.Fatalf("expected the refusal to name the open thread id, got: %v", err)
	}
	if got.Status != model.StatusDraft {
		t.Fatalf("expected claim to remain draft on refused lock, got %q", got.Status)
	}
}

// TestLockAllowedWhenUnrelatedLockedClaimHasOpenThread proves the gate is
// CANDIDATE-scoped: locking a clean claim B succeeds even though an unrelated
// already-locked claim A in the same project carries an open thread. A
// project-wide open-thread check would freeze all locking; this one must not.
func TestLockAllowedWhenUnrelatedLockedClaimHasOpenThread(t *testing.T) {
	withRegistry(t)

	a := model.Claim{
		ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true,
		Comments: []model.Comment{{ID: "c-aaa111", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "clarify"}},
	}
	b := model.Claim{ID: "widget.contract.b", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	claims := []model.Claim{a, b}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := Lock(b, claims, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("expected Lock of B to succeed while unrelated locked A has an open thread, got: %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected B to be locked, got %q", got.Status)
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
	lockedA, err := Lock(a, []model.Claim{dep, a, b}, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("lock A: %v", err)
	}

	// D drifts to v2, then B locks against v2.
	dep.Body = "dep v2"
	lockedB, err := Lock(b, []model.Claim{dep, lockedA, b}, testConfig(), store, testApproval())
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
	// LoadStore reports the version the file EARNED, not the one the next Save
	// would like to write: a legacy store loads as version 0 and is stamped
	// forward only by a migration that actually runs (MigrateLegacyStore /
	// CrossPreLedger). Stamping here instead is what used to make a downgraded
	// version field repair itself on the next ordinary write — see LoadStore.
	// (MigrateLegacyStore's own re-arm, and the version it then stamps, are
	// asserted by TestMigrateLegacyStore*; this test is about LoadStore alone,
	// which is why it does not run the migration here.)
	if store.Version != 0 {
		t.Fatalf("expected legacy store to keep its on-disk version 0 until a migration raises it, got %d", store.Version)
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

// TestMigrateLegacyStoreReArmsBaselines is the DEFERRED-1 regression: after
// LoadStore drops a legacy store's un-attributable flat hashes, every
// already-locked claim is left with no baseline, so DX-AUD-09 drift detection
// is down for the project's existing locks. MigrateLegacyStore must re-arm each
// locked claim's per-dependent baseline from CURRENT dependency content:
// DetectStale immediately after reports NO drift (bar a), while a dependency
// edited AFTER migration flips its dependent to review_pending (bar b).
func TestMigrateLegacyStoreReArmsBaselines(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}

	// Hand-write a legacy flat store: no "version", flat map[depID]hash whose
	// value no longer matches dep's content.
	path := t.TempDir() + "/store.json"
	legacy := `{
  "hashes": {
    "widget.contract.dep": "stale-legacy-hash"
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
		t.Fatalf("LoadStore: %v", err)
	}
	if len(store.Hashes) != 0 {
		t.Fatalf("precondition: expected LoadStore to drop the legacy flat hashes, got %v", store.Hashes)
	}

	claims := []model.Claim{main, dep}
	if changed := MigrateLegacyStore(store, claims); !changed {
		t.Fatalf("expected MigrateLegacyStore to re-arm baselines and report changed=true")
	}
	// It stamps the version IT earned — the per-dependent baseline schema — and
	// NOT the current one. Stamping the ledger schema here would take this
	// schema-0 store to "ledger-covered" with no ledger record in it, so every
	// locked claim would read as covered-but-unrecorded and the one-time adoption
	// would never be offered.
	if store.Version != nestedHashSchemaVersion {
		t.Fatalf("expected store stamped schema version %d, got %d", nestedHashSchemaVersion, store.Version)
	}
	if h, ok := store.Baseline(main.ID, dep.ID); !ok || h != ContentHash(dep) {
		t.Fatalf("expected per-dependent baseline Hashes[%s][%s] re-armed to current dep content", main.ID, dep.ID)
	}

	// (a) No spurious drift immediately after migration.
	for _, c := range DetectStale(claims, store) {
		if c.ID == main.ID && c.ReviewPending {
			t.Fatalf("expected NO drift immediately after migration (current == re-armed baseline)")
		}
	}

	// (b) A dependency edited AFTER migration flips its dependent stale.
	dep.Body = "dep v2"
	var flipped bool
	for _, c := range DetectStale([]model.Claim{main, dep}, store) {
		if c.ID == main.ID {
			flipped = c.ReviewPending
		}
	}
	if !flipped {
		t.Fatalf("expected main to flip review_pending after its dependency was edited post-migration")
	}
}

// TestMigrateLegacyStoreIdempotent proves the migration runs once (bar d):
// after the first re-arm populates baselines, a second call is a no-op
// (changed=false) that leaves the recorded baselines untouched.
func TestMigrateLegacyStoreIdempotent(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}
	claims := []model.Claim{main, dep}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}

	if changed := MigrateLegacyStore(store, claims); !changed {
		t.Fatalf("first MigrateLegacyStore should re-arm and report changed=true")
	}
	first, ok := store.Baseline(main.ID, dep.ID)
	if !ok {
		t.Fatalf("expected a baseline recorded on first migration")
	}

	if changed := MigrateLegacyStore(store, claims); changed {
		t.Fatalf("second MigrateLegacyStore must be a no-op (changed=false) once baselines are present")
	}
	if again, _ := store.Baseline(main.ID, dep.ID); again != first {
		t.Fatalf("second migration must not alter an existing baseline")
	}
}

// TestMigrateLegacyStorePreservesExistingReviewPending is correctness bar (c):
// a claim already review_pending before the upgrade stays so. Migration re-arms
// the baseline to current content (so DetectStale sees no NEW drift), but
// DetectStale only ever SETS review_pending, never clears it, so a pre-existing
// flag — which lives in the claim YAML, never the store — survives untouched.
func TestMigrateLegacyStorePreservesExistingReviewPending(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: []string{dep.ID}}
	claims := []model.Claim{main, dep}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	MigrateLegacyStore(store, claims)

	var got model.Claim
	for _, c := range DetectStale(claims, store) {
		if c.ID == main.ID {
			got = c
		}
	}
	if !got.ReviewPending {
		t.Fatalf("expected a pre-upgrade review_pending claim to stay review_pending after migration")
	}
}

// TestMigrateLegacyStoreSkipsDraftClaims proves migration re-arms baselines
// only for LOCKED claims: DetectStale only inspects locked claims, so a draft
// claim's dependencies are irrelevant and must not be recorded.
func TestMigrateLegacyStoreSkipsDraftClaims(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: []string{dep.ID}}
	claims := []model.Claim{draft, dep}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	changed := MigrateLegacyStore(store, claims)

	if _, ok := store.Baseline(draft.ID, dep.ID); ok {
		t.Fatalf("expected no baseline recorded for a draft claim")
	}
	if changed {
		t.Fatalf("expected changed=false when no locked claim has a dependency to re-arm")
	}
}

// TestContentHash_ExcludesComments is the Blocking #6 regression: ContentHash
// hashes an explicit content allowlist that does NOT include Comments, so
// every comment op leaves a claim's content hash byte-identical. This proves
// the exclusion by construction — there is intentionally no comment-exclusion
// code anywhere; this test is what guards it.
func TestContentHash_ExcludesComments(t *testing.T) {
	base := model.Claim{
		ID:     "widget.contract.a",
		Facet:  "contract",
		Module: "widget",
		Status: model.StatusLocked,
		Body:   "the claim body",
	}
	want := ContentHash(base)

	// Every mutation a comment op can make: add a thread, add a reply,
	// resolve, reopen, edit, and set review_pending — none may change the hash.
	withComments := base
	withComments.Comments = []model.Comment{
		{
			ID:         "c-8f3a2b",
			Status:     model.CommentStatusResolved,
			Author:     model.CommentRoleHuman,
			Created:    "2026-07-24T10:12:00Z",
			Body:       "hostile: colons: --- and \"quotes\"",
			Edited:     true,
			Replies:    []model.Reply{{ID: "r-4c9e11", Author: model.CommentRoleAgent, Created: "2026-07-24T10:40:00Z", Body: "reply body", Edited: false}},
			ResolvedBy: model.CommentRoleHuman,
			ResolvedAt: "2026-07-24T11:02:00Z",
			ReopenedBy: model.CommentRoleAgent,
			ReopenedAt: "2026-07-24T11:10:00Z",
		},
	}
	withComments.ReviewPending = true
	if got := ContentHash(withComments); got != want {
		t.Fatalf("ContentHash changed when comments/review_pending were added:\n got %s\nwant %s", got, want)
	}
}

// contentHashNoRawHTML is the ContentHash of the claim built by
// TestContentHash_RawHTMLIsHashedOnlyWhenPresent, captured from the code as it
// stood BEFORE raw_html joined the allowlist. It is written out as a literal
// rather than recomputed so it cannot drift along with the implementation: it
// is the only thing standing between a future edit to ContentHash's field list
// and every consuming project's recorded baselines mismatching at once.
const contentHashNoRawHTML = "5f8766204a1f739054b452d5871bc51e917260a23e961481e48a66c5e3e4e4d2"

// TestContentHash_RawHTMLIsHashedOnlyWhenPresent pins both halves of the
// conditional in ContentHash's raw_html stanza, because each half guards a
// different failure:
//
//   - A claim WITHOUT raw_html must hash exactly as it did before raw_html was
//     added to the list. If raw_html were appended unconditionally, every claim
//     in every project would re-hash on upgrade and the first run would flip the
//     whole graph to review_pending — migration-shaped churn from a patch
//     release. The frozen constant is what detects that.
//
//   - A claim WITH raw_html must re-hash when that raw_html is edited. Since
//     v0.4.1 raw_html is an attachment legal on any layout, so it can sit on a
//     rule-bearing claim other claims rest_on; if the hash did not move, editing
//     the attachment would change what a reader sees while leaving every
//     dependent unflagged.
func TestContentHash_RawHTMLIsHashedOnlyWhenPresent(t *testing.T) {
	base := model.Claim{
		ID:     "widget.contract.a",
		Facet:  "contract",
		Module: "widget",
		Body:   "the claim body",
	}
	if got := ContentHash(base); got != contentHashNoRawHTML {
		t.Fatalf("ContentHash of a claim with no raw_html moved:\n got %s\nwant %s\n"+
			"raw_html must only be hashed when non-empty; hashing it unconditionally\n"+
			"re-hashes every claim in every existing project", got, contentHashNoRawHTML)
	}

	// An empty raw_html is the same claim as no raw_html: the zero value of an
	// omitempty field is what every pre-v0.4.1 claim on disk loads as, so it
	// must take the untouched path and not merely happen to.
	explicitlyEmpty := base
	explicitlyEmpty.RawHTML = ""
	if got := ContentHash(explicitlyEmpty); got != contentHashNoRawHTML {
		t.Fatalf("ContentHash of a claim with an empty raw_html = %s, want the unchanged %s", got, contentHashNoRawHTML)
	}

	// Gaining raw_html moves the hash...
	withRaw := base
	withRaw.RawHTML = "<div class=\"mock\">before</div>"
	first := ContentHash(withRaw)
	if first == contentHashNoRawHTML {
		t.Fatalf("ContentHash did not move when the claim gained raw_html: still %s\n"+
			"a dependent would never be flagged for an attachment it can see", first)
	}

	// ...and so does editing it, which is the case v0.4.1 actually introduces:
	// raw_html on a rule-bearing claim that other claims rest_on.
	edited := withRaw
	edited.RawHTML = "<div class=\"mock\">after</div>"
	if second := ContentHash(edited); second == first {
		t.Fatalf("ContentHash did not move when raw_html was edited: still %s", second)
	}

	// raw_html is content, not bookkeeping: it must not disturb the exclusions
	// TestContentHash_ExcludesComments pins. Same raw_html, different comment
	// state, same hash.
	noisy := withRaw
	noisy.ReviewPending = true
	noisy.Comments = []model.Comment{{ID: "c-8f3a2b", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-24T10:12:00Z", Body: "q"}}
	if got := ContentHash(noisy); got != first {
		t.Fatalf("ContentHash of a raw_html-bearing claim changed with comments/review_pending:\n got %s\nwant %s", got, first)
	}
}

// TestDetectStale_RawHTMLEditOnDependencyFlipsTheDependent is the reason FIX 1
// exists, stated end to end rather than at the hash: a locked claim that rests
// on another claim must be flipped to review_pending when that dependency's
// raw_html attachment is edited. Before v0.4.1 raw_html could only sit on a
// layout: mockup illustration with no inbound edges, so this path was
// unreachable; now the attachment is legal on a rule-bearing claim, and this is
// the case that would otherwise be silent.
func TestDetectStale_RawHTMLEditOnDependencyFlipsTheDependent(t *testing.T) {
	dep := model.Claim{
		ID:      "widget.contract.dep",
		Facet:   "contract",
		Module:  "widget",
		Status:  model.StatusLocked,
		Body:    "dep body",
		RawHTML: "<div>before</div>",
	}
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}
	claims := []model.Claim{dependent, dep}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	store.recordBaseline(dependent.ID, dep.ID, ContentHash(dep))

	// Only the attachment changes — body, rows, steps and edges all stand.
	claims[1].RawHTML = "<div>after</div>"

	out := DetectStale(claims, store)
	var flipped bool
	for _, c := range out {
		if c.ID == dependent.ID {
			flipped = c.ReviewPending
		}
	}
	if !flipped {
		t.Fatalf("dependent was NOT flipped to review_pending after its dependency's raw_html was edited")
	}
}

// TestDetectStale_CommentOnDependencyDoesNotFlip proves a locked dependent
// claim is NOT flipped to review_pending merely because a claim it rests on
// gained a comment thread: since ContentHash excludes Comments, the dependency
// baseline still matches after the comment is added.
func TestDetectStale_CommentOnDependencyDoesNotFlip(t *testing.T) {
	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep body"}
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: []string{dep.ID}}
	claims := []model.Claim{dependent, dep}

	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	store.recordBaseline(dependent.ID, dep.ID, ContentHash(dep))

	// The dependency gains an open comment thread.
	claims[1].Comments = []model.Comment{{ID: "c-000001", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-07-24T10:12:00Z", Body: "q"}}

	out := DetectStale(claims, store)
	for _, c := range out {
		if c.ID == dependent.ID && c.ReviewPending {
			t.Fatalf("dependent claim was flipped to review_pending by a comment on its dependency")
		}
	}
}

// TestClaimsSentinelPath_OutsideClaimsDir proves the claims sentinel lives
// under cfg.Dir() and outside claims_dir, and that AcquireClaimsLock creates
// then removes the .lock file.
func TestClaimsSentinelPath_OutsideClaimsDir(t *testing.T) {
	root := t.TempDir()
	claimsDir := root + "/claims"
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(root+"/project.config.yaml", []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(root + "/project.config.yaml")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	base := ClaimsSentinelPath(cfg)
	if want := filepath.Join(cfg.Dir(), "build", "ledger", "claims"); base != want || base != cfg.ClaimsSentinelPath() {
		t.Fatalf("ClaimsSentinelPath = %q, want %q (cfg.ClaimsSentinelPath = %q)", base, want, cfg.ClaimsSentinelPath())
	}
	if strings.HasPrefix(base, cfg.ClaimsDir+string(filepath.Separator)) {
		t.Fatalf("claims sentinel %q must live OUTSIDE claims_dir %q", base, cfg.ClaimsDir)
	}

	release, err := AcquireClaimsLock(cfg)
	if err != nil {
		t.Fatalf("AcquireClaimsLock: %v", err)
	}
	if _, err := os.Stat(base + ".lock"); err != nil {
		t.Fatalf("expected sentinel lock file to exist while held: %v", err)
	}
	release()
	if _, err := os.Stat(base + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("expected sentinel lock file to be removed after release, stat err = %v", err)
	}
}

// THE LAST STEP OF THE DELETED-RECORD BYPASS, which is the one that made the
// other three invisible.
//
// internal/lock's audit reports the intermediate state correctly
// (lock-ledger-deleted, asserted in audit_test.go, and it fires whether the
// claim reads locked or draft). But reporting is not refusing, and the sequence
// does not stop there. Verified against the binary before this gate existed:
//
//	delete the claim's key from "ledger" in build/ledger/lock-store.json
//	edit "status: locked" -> "status: draft"     check: lock-ledger-deleted, exit 1
//	rewrite the body                             check: lock-ledger-deleted, exit 1
//	dossierx claim lock <id> --reason "..."      exit 0 — a FRESH record, over the
//	                                             rewritten body
//	dossierx check                               exit 0, ZERO findings, permanently
//
// The finding that named the tamper at every step vanished at the last one,
// because RecordApproval wrote the record whose absence was the evidence. The
// audit rule's own message ends "do NOT re-lock, which would record whatever the
// claim says NOW as approved" — this test is what stops the tool from doing it.
//
// Without the gate in Lock this fails at the first assertion: the lock returns
// nil, and Audit then reports nothing at all.
func TestLockRefusesAClaimWhoseLedgerRecordWasDeleted(t *testing.T) {
	locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

	// THE ATTACK: delete the record, flip to draft, rewrite the body.
	delete(store.Ledger, locked.ID)
	tampered := locked
	tampered.Status = model.StatusDraft
	tampered.Body = "rewritten now that nothing vouches for it"

	_, err := Lock(tampered, []model.Claim{tampered}, testConfig(), store, Approval{Actor: "mallory", Reason: "reads exactly like a human's approval"})
	if !errors.Is(err, ErrLedgerRecordDeleted) {
		t.Fatalf("re-locking a claim whose ledger record was deleted must be refused with ErrLedgerRecordDeleted; got %v", err)
	}

	// The refusal wrote nothing: no record was created, so the finding that
	// names the tamper survives. A refusal that still recorded would be the
	// bypass with an error message attached.
	if _, ok := store.Ledger[locked.ID]; ok {
		t.Fatalf("the refused lock still wrote a ledger record; the whole point is that no record is created")
	}
	if !hasRule(Audit([]model.Claim{tampered}, store, nil), RuleLockLedgerDeleted) {
		t.Fatalf("after the refusal the gate must still report %s", RuleLockLedgerDeleted)
	}

	// The recovery must be a RESTORE. Naming unlock here would be actively
	// wrong: unlocking accepts the attacker's edit and asks a human to sign it.
	if !strings.Contains(err.Error(), "Restore build/ledger/lock-store.json from version control") {
		t.Errorf("the refusal must name the restore as the recovery, got %q", err)
	}
}

// The gate must not touch the two shapes that look similar and are honest,
// because a refusal that fires on correct work is worked around rather than
// obeyed.
//
// unlock -> fix -> lock: unlock RELEASES the record rather than deleting it, so
// a record still exists and this gate never sees the claim. That path is the one
// every other refusal in this package points at, and it has to stay open.
//
// A claim this engine never locked: no locked_at, no dependency baselines, so
// engineLocked is false. This is the ordinary first lock of a new claim in a
// covered project, and it is the case the pre-existing comment in Lock worried
// about when it declined to refuse on a missing record at all.
func TestTheDeletedRecordLockGateIsSilentOnTheHonestPaths(t *testing.T) {
	t.Run("unlock then fix then lock", func(t *testing.T) {
		locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

		unlocked := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a correction"})
		unlocked.Body = "the corrected body, which a human is about to approve"

		if _, err := Lock(unlocked, []model.Claim{unlocked}, testConfig(), store, Approval{Actor: "alice", Reason: "approved the correction"}); err != nil {
			t.Fatalf("unlock -> fix -> lock must still work; got %v", err)
		}
	})

	t.Run("a claim this engine never locked", func(t *testing.T) {
		_, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

		fresh := model.Claim{ID: "widget.contract.fresh", Facet: "contract", Module: "widget", Body: "a brand new claim"}
		if _, err := Lock(fresh, []model.Claim{fresh}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("the first lock of a new claim must work in a covered project; got %v", err)
		}
	})
}

// THE DELETED DIGEST KEY, closed at the command that used to launder it.
//
// The audit reports this state (RuleCommentDigestUnrecorded) and the comment ops
// refuse to write under it (internal/comments' checkCommentDigest). Neither
// closes it, because the laundering step is `claim lock`: RecordApproval records
// the claim's comment digest in the same act as the approval, unconditionally,
// so on a claim whose entry was deleted it MANUFACTURES one from whatever the
// block says at that moment.
//
// Verified against the binary before this gate existed, on a fully covered
// project: a human's open thread blocks the lock; forge `status: resolved` in
// the YAML and drop that one key from "digests"; `dossierx check` correctly
// reports comment-digest-unrecorded; `dossierx claim lock` then exits 0 AND
// writes an entry certifying the forged block; and every check from then on
// exits 0 with zero findings. The human's objection is gone and the record says
// the review was clean.
//
// Without the gate this fails at the first assertion (the lock returns nil) and
// again at the second (the digest store gains an entry for the forged block).
func TestLockRefusesAClaimWhoseCommentDigestEntryWasDeleted(t *testing.T) {
	withRegistry(t)

	dir := t.TempDir()
	storePath := filepath.Join(dir, "store.json")
	store, err := LoadStore(storePath)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	// A covered project: a real lock on an unrelated claim creates the store at
	// the ledger schema, and records that claim's (empty) comment digest beside
	// it — which is what makes the digest store PRESENT, so comment-digest-absent
	// is not the finding here.
	other := model.Claim{ID: "widget.contract.other", Facet: "contract", Module: "widget", Body: "unrelated"}
	if _, err := Lock(other, []model.Claim{other}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
		t.Fatalf("seed Lock: %v", err)
	}
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	store, err = LoadStore(storePath)
	if err != nil {
		t.Fatalf("LoadStore (reopen): %v", err)
	}
	if !store.LedgerCovered() {
		t.Fatalf("fixture precondition: the project must read as ledger-covered")
	}

	digestPath := digest.StorePathBeside(storePath)
	digests, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("digest.LoadStore: %v", err)
	}
	if !digests.FileExists() {
		t.Fatalf("fixture precondition: a lock must have created the comment digest store beside the lock store")
	}

	// The claim under attack: it CARRIES a thread, forged as resolved, and has
	// no entry in the digest store — the key having been dropped.
	forged := model.Claim{
		ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body",
		Comments: []model.Comment{{
			ID: "c-aaa111", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
			Created: "2026-07-27T10:00:00Z", Body: "this is wrong, please fix",
		}},
	}
	if _, known := digests.Digest(forged.ID); known {
		t.Fatalf("fixture precondition: the claim under attack must have no digest entry")
	}

	_, lockErr := Lock(forged, []model.Claim{forged}, testConfig(), store, Approval{Actor: "mallory", Reason: "the thread reads resolved"})
	if !errors.Is(lockErr, ErrCommentDigestUnrecorded) {
		t.Fatalf("locking a claim whose comment digest entry was deleted must be refused with ErrCommentDigestUnrecorded; got %v", lockErr)
	}

	// And the refusal wrote NOTHING. An entry here would be the launder with an
	// error message attached: the finding would be cleared for every later run.
	after, err := digest.LoadStore(digestPath)
	if err != nil {
		t.Fatalf("digest.LoadStore (reopen): %v", err)
	}
	if _, known := after.Digest(forged.ID); known {
		t.Fatalf("the refused lock still recorded a digest for the forged comment block")
	}
	if _, ok := store.Ledger[forged.ID]; ok {
		t.Fatalf("the refused lock still wrote a ledger record")
	}

	// The recovery is a restore, and it must not name a comment op or a re-lock:
	// both record whatever the claim says now as the review history.
	if !strings.Contains(lockErr.Error(), "Restore build/ledger/comment-digest.json from version control") {
		t.Errorf("the refusal must name the restore as the recovery, got %q", lockErr)
	}
}

// The same gate must be silent on the shapes where the evidence is honestly
// absent, or it refuses correct work — each of these is one of the three
// silences RuleCommentDigestUnrecorded documents.
func TestTheUnrecordedDigestLockGateIsSilentWhereEvidenceIsHonestlyAbsent(t *testing.T) {
	withRegistry(t)

	covered := func(t *testing.T) (*Store, string) {
		t.Helper()
		dir := t.TempDir()
		storePath := filepath.Join(dir, "store.json")
		s, err := LoadStore(storePath)
		if err != nil {
			t.Fatalf("LoadStore: %v", err)
		}
		other := model.Claim{ID: "widget.contract.other", Facet: "contract", Module: "widget", Body: "unrelated"}
		if _, err := Lock(other, []model.Claim{other}, testConfig(), s, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("seed Lock: %v", err)
		}
		if err := s.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
		reopened, err := LoadStore(storePath)
		if err != nil {
			t.Fatalf("LoadStore (reopen): %v", err)
		}
		return reopened, storePath
	}

	// A threadless claim: an entry is what a comment op CREATES, not something
	// locking requires. This is the ordinary first lock of a claim nobody has
	// commented on, and it must not be refused.
	t.Run("threadless claim in a covered project", func(t *testing.T) {
		store, _ := covered(t)
		fresh := model.Claim{ID: "widget.contract.fresh", Facet: "contract", Module: "widget", Body: "no threads here"}
		if _, err := Lock(fresh, []model.Claim{fresh}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("a claim with no comment threads must lock normally; got %v", err)
		}
	})

	// A claim WITH threads that HAS its entry: the honest post-review lock, and
	// the one this gate most has to leave alone.
	t.Run("claim whose threads are recorded", func(t *testing.T) {
		store, storePath := covered(t)
		reviewed := model.Claim{
			ID: "widget.contract.reviewed", Facet: "contract", Module: "widget", Body: "reviewed and agreed",
			Comments: []model.Comment{{
				ID: "c-bbb222", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
				Created: "2026-07-27T10:00:00Z", Body: "looks right now",
			}},
		}
		// Record it the way a comment op would have.
		digests, err := digest.LoadStore(digest.StorePathBeside(storePath))
		if err != nil {
			t.Fatalf("digest.LoadStore: %v", err)
		}
		digests.Record(reviewed)
		if err := digests.Save(); err != nil {
			t.Fatalf("digest Save: %v", err)
		}

		if _, err := Lock(reviewed, []model.Claim{reviewed}, testConfig(), store, Approval{Actor: "alice", Reason: "approved after review"}); err != nil {
			t.Fatalf("a claim whose threads are recorded must lock normally; got %v", err)
		}
	})

	// An UNCOVERED project. Its threads predate the digest store entirely, and
	// the project-scoped adoption finding is what speaks to it — refusing every
	// commented claim here would block a v0.2.x project from locking anything.
	t.Run("uncovered project", func(t *testing.T) {
		store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
		if err != nil {
			t.Fatalf("LoadStore: %v", err)
		}
		commented := model.Claim{
			ID: "widget.contract.legacy", Facet: "contract", Module: "widget", Body: "old project",
			Comments: []model.Comment{{
				ID: "c-ccc333", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman,
				Created: "2026-07-27T10:00:00Z", Body: "from before the digest store",
			}},
		}
		if _, err := Lock(commented, []model.Claim{commented}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("an uncovered project must still be able to lock a commented claim; got %v", err)
		}
	})
}

// ---------------------------------------------------------------------
// governed_by is a DRIFT dependency (#21)
// ---------------------------------------------------------------------

// TestBaselineDependencyIDsIncludesAClaimValuedGovernedBy pins the whole of
// what the baseline set is: mirrors, rests_on, and a governed_by.type that
// names a claim — with "none" and the empty string excluded by the same guard
// internal/lint/dangling.go uses, and repeats collapsed deterministically.
func TestBaselineDependencyIDsIncludesAClaimValuedGovernedBy(t *testing.T) {
	cases := []struct {
		name  string
		claim model.Claim
		want  []string
	}{
		{
			name:  "governed_by names a claim",
			claim: model.Claim{ID: "child", Governed: model.Governed{Type: "widget.doctrine.hub"}},
			want:  []string{"widget.doctrine.hub"},
		},
		{
			name:  "governed_by none is not a dependency",
			claim: model.Claim{ID: "child", Governed: model.Governed{Type: "none", Reason: "deliberately ungoverned"}},
			want:  []string{},
		},
		{
			name:  "governed_by unset is not a dependency",
			claim: model.Claim{ID: "child"},
			want:  []string{},
		},
		{
			name: "all three edge types, in order",
			claim: model.Claim{
				ID: "child", Mirrors: []string{"m"}, RestsOn: []string{"r"},
				Governed: model.Governed{Type: "widget.doctrine.hub"},
			},
			want: []string{"m", "r", "widget.doctrine.hub"},
		},
		{
			// The reason dedupeStable is required rather than incidental: a
			// claim may reach the same target through two edge types, and the
			// baseline table must not depend on which edge was walked first.
			name: "the same target through two edges is one dependency",
			claim: model.Claim{
				ID: "two-edge", RestsOn: []string{"widget.doctrine.hub"},
				Governed: model.Governed{Type: "widget.doctrine.hub"},
			},
			want: []string{"widget.doctrine.hub"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BaselineDependencyIDs(tc.claim)
			if len(got) != len(tc.want) {
				t.Fatalf("BaselineDependencyIDs = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("BaselineDependencyIDs = %v, want %v (order is part of the contract)", got, tc.want)
				}
			}
		})
	}
}

// TestLockRecordsAGovernanceBaseline is the bug in #21 at its source: before
// the fix a claim-valued governed_by.type never became an approved baseline, so
// there was nothing for DetectStale to compare against and editing the
// governing doctrine claim moved nothing to review_pending.
func TestLockRecordsAGovernanceBaseline(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "the governing doctrine"}
	// governed_by is the ONLY edge: no mirrors, no rests_on naming the hub.
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "child", Governed: model.Governed{Type: hub.ID}}
	claims := []model.Claim{hub, child}

	store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if _, err := Lock(child, claims, testConfig(), store, testApproval()); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	got, known := store.Baseline(child.ID, hub.ID)
	if !known {
		t.Fatalf("locking a claim with a claim-valued governed_by.type must record the governor's content hash as a per-dependent baseline; store has %v", store.Hashes)
	}
	if got != ContentHash(hub) {
		t.Fatalf("governance baseline = %q, want the governor's current ContentHash %q", got, ContentHash(hub))
	}

	// And the drift half: edit the governor's comparable content and the
	// directly governed locked claim flips to review_pending.
	hub.Body = "the governing doctrine, reworded"
	locked := child
	locked.Status = model.StatusLocked
	out := DetectStale([]model.Claim{hub, locked}, store)
	for _, c := range out {
		if c.ID == child.ID && !c.ReviewPending {
			t.Fatalf("expected review_pending true after the governor's content changed")
		}
	}
}

// TestGovernanceDriftPropagationIsStaged: flagging a directly governed claim
// does not itself flag claims downstream of it. DetectStale compares stored
// baselines against CURRENT content, and the downstream claim's baseline is
// over its dependency's content — which review_pending does not change (see
// ContentHash's field list).
func TestGovernanceDriftPropagationIsStaged(t *testing.T) {
	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "doctrine v1"}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "child", Governed: model.Governed{Type: hub.ID}}
	downstream := model.Claim{ID: "widget.contract.downstream", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "downstream", RestsOn: []string{child.ID}}

	store := &Store{
		Version: storeSchemaVersion,
		Hashes: map[string]map[string]string{
			child.ID:      {hub.ID: ContentHash(hub)},
			downstream.ID: {child.ID: ContentHash(child)},
		},
		LockedAt: map[string]string{},
		path:     filepath.Join(t.TempDir(), "store.json"),
	}

	hub.Body = "doctrine v2"
	out := DetectStale([]model.Claim{hub, child, downstream}, store)
	for _, c := range out {
		switch c.ID {
		case child.ID:
			if !c.ReviewPending {
				t.Fatalf("the directly governed claim must be flagged")
			}
		case downstream.ID:
			if c.ReviewPending {
				t.Fatalf("propagation is staged: a claim resting on a newly-flagged claim must not be flagged in the same pass")
			}
		}
	}
}

// TestHubGatingIgnoresGovernedBy is the behavioural half of "hub gating is
// byte-for-byte unchanged" (D-6, branch (a)): governance is a DRIFT edge, not a
// GATING edge. A child naming an UNLOCKED doctrine-facet claim only through
// governed_by.type still locks. Widening dependencyIDs instead of adding
// BaselineDependencyIDs is exactly what this test refuses, and it is a refusal
// documented in internal/lint/governed_cycle.go and FORMAT.md.
func TestHubGatingIgnoresGovernedBy(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusDraft, Body: "still draft"}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "child", Governed: model.Governed{Type: hub.ID}}
	claims := []model.Claim{hub, child}

	store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	got, err := Lock(child, claims, testConfigWithDoctrine(), store, testApproval())
	if err != nil {
		t.Fatalf("a claim naming an unlocked doctrine claim ONLY through governed_by must still lock; got %v", err)
	}
	if got.Status != model.StatusLocked {
		t.Fatalf("expected status locked, got %q", got.Status)
	}

	// The contrast, in the same test so the two can never drift apart: name the
	// same unlocked hub through rests_on and the lock IS refused.
	gated := child
	gated.ID = "widget.contract.gated"
	gated.RestsOn = []string{hub.ID}
	if _, err := Lock(gated, []model.Claim{hub, gated}, testConfigWithDoctrine(), store, testApproval()); err == nil {
		t.Fatalf("rests_on on an unlocked doctrine claim must still be a lock refusal")
	}
}

// TestRefreshBaselineRefreshesTheGovernanceBaseline is the reaudit half: a
// confirmed reaudit re-snapshots the governor, so the drift trigger clears.
func TestRefreshBaselineRefreshesTheGovernanceBaseline(t *testing.T) {
	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "doctrine v1"}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "child", Governed: model.Governed{Type: hub.ID}}
	store := &Store{
		Version:  storeSchemaVersion,
		Hashes:   map[string]map[string]string{child.ID: {hub.ID: ContentHash(hub)}},
		LockedAt: map[string]string{},
		path:     filepath.Join(t.TempDir(), "store.json"),
	}

	hub.Body = "doctrine v2"
	RefreshBaseline(child, []model.Claim{hub, child}, store)

	got, known := store.Baseline(child.ID, hub.ID)
	if !known || got != ContentHash(hub) {
		t.Fatalf("governance baseline after RefreshBaseline = %q (known=%v), want the governor's new ContentHash %q", got, known, ContentHash(hub))
	}
	if out := DetectStale([]model.Claim{hub, child}, store); out[1].ReviewPending {
		t.Fatalf("a refreshed governance baseline must clear the drift trigger")
	}
}

// TestGovernedByNoneCreatesNoBaseline: "none" is a sentinel, not a claim id. It
// must never become a baseline key — a store row keyed "none" would compare
// against a claim that cannot exist and quietly do nothing forever.
func TestGovernedByNoneCreatesNoBaseline(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	claim := model.Claim{
		ID: "widget.contract.ungoverned", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Body: "ungoverned", Governed: model.Governed{Type: "none", Reason: "deliberately ungoverned"},
	}
	store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if _, err := Lock(claim, []model.Claim{claim}, testConfig(), store, testApproval()); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, known := store.Baseline(claim.ID, "none"); known {
		t.Fatalf("governed_by.type: none must create no baseline; store has %v", store.Hashes)
	}
}

// TestTwoEdgeDependencyRecordsExactlyOneBaseline is dedupeStable at the store
// level: rests_on X plus governed_by X is one dependency, recorded once.
func TestTwoEdgeDependencyRecordsExactlyOneBaseline(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	hub := model.Claim{ID: "widget.doctrine.hub", Facet: "doctrine", Module: "widget", Status: model.StatusLocked, Body: "doctrine"}
	twoEdge := model.Claim{
		ID: "widget.contract.two-edge", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "two edges",
		RestsOn: []string{hub.ID}, Governed: model.Governed{Type: hub.ID},
	}
	store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if _, err := Lock(twoEdge, []model.Claim{hub, twoEdge}, testConfig(), store, testApproval()); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if n := len(store.Hashes[twoEdge.ID]); n != 1 {
		t.Fatalf("a target reached through two edge types must produce exactly one baseline entry, got %d: %v", n, store.Hashes[twoEdge.ID])
	}
}
