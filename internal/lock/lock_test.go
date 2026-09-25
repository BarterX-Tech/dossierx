package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// namedLint is a test-only lint.Lint that reports one finding of the given
// severity against claimID, used to exercise the lock policy's lint gate
// without depending on any of the real lint rules.
type namedLint struct {
	name     string
	claimID  string
	severity lint.Severity
}

func (l namedLint) Name() string { return l.name }
func (l namedLint) Check(claims []model.Claim, cfg *config.Config) []lint.Finding {
	return []lint.Finding{{LintName: l.name, ClaimID: l.claimID, Message: "forced finding", Severity: l.severity}}
}

// lockedClaimLint is a test-only lint.Lint that reports every claim whose OWN
// status is locked — a stand-in for the real rules (roll-up, for one) that
// describe a property a claim only has once it is locked.
type lockedClaimLint struct{}

func (lockedClaimLint) Name() string { return "test-locked-claim-lint" }
func (lockedClaimLint) Check(claims []model.Claim, cfg *config.Config) []lint.Finding {
	var out []lint.Finding
	for _, c := range claims {
		if c.Status == model.StatusLocked {
			out = append(out, lint.Finding{LintName: "test-locked-claim-lint", ClaimID: c.ID, Message: "locked"})
		}
	}
	return out
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

// testApproval is the stand-in human approval every lock/unlock in this
// package's tests executes. The ledger write hooks take one by value so a
// caller cannot record an approval without having something to put in it; a
// test that wants to assert the RECORDED actor/reason builds its own.
func testApproval() Approval {
	return Approval{Actor: "test-actor", Reason: "test approval"}
}

// TestLockLintGateRefusesOnlyErrorFindingsAgainstTheCandidate pins the lock
// policy's lint gate: an error-severity finding against the candidate refuses
// it and names the lint, while a warning-severity one does not — matching
// "dossierx lint"/"dossierx check"'s own pass/fail semantics, where warnings
// (e.g. "orphan") are reported but never fail the command.
func TestLockLintGateRefusesOnlyErrorFindingsAgainstTheCandidate(t *testing.T) {
	const id = "widget.contract.overview"
	claims := []model.Claim{{ID: id, Facet: "contract", Module: "widget", Status: model.StatusDraft}}
	store := newStore(t)

	withRegistry(t, namedLint{name: "test-error-lint", claimID: id})
	verdict := requireRefusal(t, claims, id, testConfig(), store, "lint:test-error-lint")
	if len(verdict.LintFindings) != 1 || verdict.LintFindings[0].ClaimID != id {
		t.Fatalf("the refusal must carry the blocking finding, got %+v", verdict.LintFindings)
	}

	withRegistry(t, namedLint{name: "test-warning-lint", claimID: id, severity: lint.SeverityWarning})
	if verdict := verdictFor(t, claims, id, testConfig(), store); !verdict.LocalAdmissible {
		t.Fatalf("a warning-only finding must not refuse the lock, got %v", verdict.Refusals)
	}
}

func TestLockSucceedsWithEmptyLintRegistry(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "dep body"}
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: model.RestsOnIDs(dep.ID)}
	claims := []model.Claim{claim, dep}
	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	got, err := approve(claim, claims, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("approve: unexpected error: %v", err)
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
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}

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
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}
	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: ContentHash(dep)}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}

	claims := []model.Claim{claim, dep}
	out := DetectStale(claims, store)

	for _, c := range out {
		if c.ID == claim.ID && c.ReviewPending {
			t.Fatalf("expected review_pending to stay false when dependency content is unchanged")
		}
	}
}

// TestLockEvaluatesLintsAgainstCandidatesPostLockStatus proves the lock policy
// lints the claim set as it will look once the candidate is locked, not its
// still-draft entry. Lints that key off a claim's own status (roll-up, for one)
// describe a property of the claim once locked; evaluating the pre-lock draft
// would let such a claim lock past the very rule meant to stop it. The stub lint
// fires only on locked claims, so it can refuse the draft candidate only if the
// evaluator substituted its locked form — and it must not reach an unrequested
// draft beside it.
func TestLockEvaluatesLintsAgainstCandidatesPostLockStatus(t *testing.T) {
	withRegistry(t, lockedClaimLint{})

	candidate := model.Claim{ID: "widget.contract.candidate", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	bystander := model.Claim{ID: "widget.contract.bystander", Facet: "contract", Module: "widget", Status: model.StatusDraft}
	claims := []model.Claim{candidate, bystander}

	store := newStore(t)
	requireRefusal(t, claims, candidate.ID, testConfig(), store, "lint:test-locked-claim-lint")
	evaluation := EvaluateSet(claims, []string{candidate.ID}, testConfig(), store)
	if len(evaluation.UnrelatedFindings) != 0 {
		t.Fatalf("only the requested claim is evaluated as locked; the bystander drew %+v", evaluation.UnrelatedFindings)
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
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: model.RestsOnIDs(dep.ID)}

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
	claim := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: model.RestsOnIDs(dep.ID)}
	store := &Store{Version: storeSchemaVersion, Hashes: map[string]map[string]string{claim.ID: {dep.ID: "stale-hash"}}, LockedAt: map[string]string{}, path: t.TempDir() + "/store.json"}
	claims := []model.Claim{claim, dep}

	RefreshBaseline(claim, claims, store)

	if h, ok := store.Baseline(claim.ID, dep.ID); !ok || h != ContentHash(dep) {
		t.Fatalf("expected RefreshBaseline to re-record the dependency baseline to current content")
	}
	if _, ok := store.LockedAt[claim.ID]; !ok {
		t.Fatalf("expected RefreshBaseline to stamp LockedAt")
	}
	// The refreshed baseline is what clears the drift trigger: against it the
	// same dependency content no longer reads as changed.
	settled := claim
	settled.ReviewPending = false
	if out := DetectStale([]model.Claim{settled, dep}, store); out[0].ReviewPending {
		t.Fatalf("a refreshed baseline must clear the drift trigger")
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

	verdict := requireRefusal(t, claims, claim.ID, testConfig(), store, "unresolved_comments")
	if len(verdict.OpenThreads) != 1 || verdict.OpenThreads[0] != "c-aaa111" {
		t.Fatalf("expected the refusal to name the open thread id, got: %v", verdict.OpenThreads)
	}
	if _, ok := store.Record(claim.ID); ok {
		t.Fatalf("a refused lock must not leave a ledger record behind")
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

	got, err := approve(b, claims, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("expected locking B to succeed while unrelated locked A has an open thread, got: %v", err)
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
// test pins. It uses only the lock path/DetectStale/LoadStore (never the store's
// internal representation) so it compiles against, and fails on, the pre-fix
// code too.
func TestPerDependentBaselineNotSharedAcrossDependents(t *testing.T) {
	withRegistry(t) // empty registry: lint always passes

	dep := model.Claim{ID: "widget.contract.dep", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "dep v1"}
	a := model.Claim{ID: "widget.contract.a", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: model.RestsOnIDs(dep.ID)}
	b := model.Claim{ID: "widget.contract.b", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: model.RestsOnIDs(dep.ID)}

	store, err := LoadStore(t.TempDir() + "/store.json")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	// A locks against D v1.
	lockedA, err := approve(a, []model.Claim{dep, a, b}, testConfig(), store, testApproval())
	if err != nil {
		t.Fatalf("lock A: %v", err)
	}

	// D drifts to v2, then B locks against v2.
	dep.Body = "dep v2"
	lockedB, err := approve(b, []model.Claim{dep, lockedA, b}, testConfig(), store, testApproval())
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
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}

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
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}

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
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}
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
	main := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, ReviewPending: true, RestsOn: model.RestsOnIDs(dep.ID)}
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
	draft := model.Claim{ID: "widget.contract.draft", Facet: "contract", Module: "widget", Status: model.StatusDraft, RestsOn: model.RestsOnIDs(dep.ID)}
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
const contentHashNoRawHTML = "b39a06c885236b934cece1487107d58c1feab7b7b641825021f152afcbca9b91"

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

// TestContentHash_SummaryIsHashedOnlyWhenPresent pins NIT-8's summary
// stanza the same way raw_html is pinned: empty keeps the historical
// digest; a present summary is content a dependent must notice.
func TestContentHash_SummaryIsHashedOnlyWhenPresent(t *testing.T) {
	base := model.Claim{
		ID:     "widget.contract.a",
		Facet:  "contract",
		Module: "widget",
		Body:   "the claim body",
	}
	if got := ContentHash(base); got != contentHashNoRawHTML {
		t.Fatalf("empty summary must keep the historical ContentHash:\n got %s\nwant %s", got, contentHashNoRawHTML)
	}
	explicitlyEmpty := base
	explicitlyEmpty.Summary = ""
	if got := ContentHash(explicitlyEmpty); got != contentHashNoRawHTML {
		t.Fatalf("empty summary = %s, want %s", got, contentHashNoRawHTML)
	}
	with := base
	with.Summary = "one line about the claim"
	first := ContentHash(with)
	if first == contentHashNoRawHTML {
		t.Fatalf("gaining a summary must move ContentHash")
	}
	edited := with
	edited.Summary = "a different one line"
	if ContentHash(edited) == first {
		t.Fatalf("editing summary must move ContentHash")
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
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}
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
	dependent := model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Status: model.StatusLocked, RestsOn: model.RestsOnIDs(dep.ID)}
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
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n"
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
// Without the ledger_record_deleted refusal this fails at the first assertion:
// the policy admits the claim, and the approval that follows leaves Audit
// reporting nothing at all.
func TestLockRefusesAClaimWhoseLedgerRecordWasDeleted(t *testing.T) {
	locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

	// THE ATTACK: delete the record, flip to draft, rewrite the body.
	delete(store.Ledger, locked.ID)
	tampered := locked
	tampered.Status = model.StatusDraft
	tampered.Body = "rewritten now that nothing vouches for it"

	requireRefusal(t, []model.Claim{tampered}, tampered.ID, testConfig(), store, "ledger_record_deleted")

	// The refusal wrote nothing: no record was created, so the finding that
	// names the tamper survives. A refusal that still recorded would be the
	// bypass with an error message attached.
	if _, ok := store.Ledger[locked.ID]; ok {
		t.Fatalf("the refused lock still wrote a ledger record; the whole point is that no record is created")
	}
	if !hasRule(Audit([]model.Claim{tampered}, store, nil), RuleLockLedgerDeleted) {
		t.Fatalf("after the refusal the gate must still report %s", RuleLockLedgerDeleted)
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
// covered project, and refusing it would leave a covered project unable to lock
// anything new.
func TestTheDeletedRecordLockGateIsSilentOnTheHonestPaths(t *testing.T) {
	t.Run("unlock then fix then lock", func(t *testing.T) {
		locked, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

		unlocked := Unlock(locked, store, Approval{Actor: "alice", Reason: "needs a correction"})
		unlocked.Body = "the corrected body, which a human is about to approve"

		if _, err := approve(unlocked, []model.Claim{unlocked}, testConfig(), store, Approval{Actor: "alice", Reason: "approved the correction"}); err != nil {
			t.Fatalf("unlock -> fix -> lock must still work; got %v", err)
		}
	})

	t.Run("a claim this engine never locked", func(t *testing.T) {
		_, store := lockedProjectOnDisk(t, model.Claim{ID: "widget.contract.main", Facet: "contract", Module: "widget", Body: "the approved body"})

		fresh := model.Claim{ID: "widget.contract.fresh", Facet: "contract", Module: "widget", Body: "a brand new claim"}
		if _, err := approve(fresh, []model.Claim{fresh}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
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
// Without the comment_digest_unrecorded refusal this fails at the first
// assertion: the policy admits the claim, and the approval that follows gains
// the digest store an entry for the forged block.
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
	if _, err := approve(other, []model.Claim{other}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
		t.Fatalf("seed approve: %v", err)
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

	requireRefusal(t, []model.Claim{forged}, forged.ID, testConfig(), store, "comment_digest_unrecorded")

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
		if _, err := approve(other, []model.Claim{other}, testConfig(), s, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("seed approve: %v", err)
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
		if _, err := approve(fresh, []model.Claim{fresh}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
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

		if _, err := approve(reviewed, []model.Claim{reviewed}, testConfig(), store, Approval{Actor: "alice", Reason: "approved after review"}); err != nil {
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
		if _, err := approve(commented, []model.Claim{commented}, testConfig(), store, Approval{Actor: "alice", Reason: "approved"}); err != nil {
			t.Fatalf("an uncovered project must still be able to lock a commented claim; got %v", err)
		}
	})
}

// ---------------------------------------------------------------------
// rests_on is the only drift baseline (NIT-6)
// ---------------------------------------------------------------------

// TestBaselineDependencyIDsIncludesClaimValuedRestsOn pins the whole of what
// the baseline set is: the claim ids rests_on names — with "none" and an unset
// rests_on excluded, and repeats collapsed deterministically, so a lock records
// one baseline per target however often it is named.
func TestBaselineDependencyIDsIncludesClaimValuedRestsOn(t *testing.T) {
	cases := []struct {
		name  string
		claim model.Claim
		want  []string
	}{
		{
			name:  "rests_on names a claim",
			claim: model.Claim{ID: "child", RestsOn: model.RestsOnIDs("widget.contract.hub")},
			want:  []string{"widget.contract.hub"},
		},
		{
			name:  "rests_on none is not a dependency",
			claim: model.Claim{ID: "child", RestsOn: model.RestsNone("deliberately ungoverned")},
			want:  []string{},
		},
		{
			name:  "rests_on unset is not a dependency",
			claim: model.Claim{ID: "child"},
			want:  []string{},
		},
		{
			name:  "duplicate targets collapse",
			claim: model.Claim{ID: "child", RestsOn: model.RestsOnIDs("widget.contract.hub", "widget.contract.hub")},
			want:  []string{"widget.contract.hub"},
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

// TestRestsOnDriftPropagationIsStaged: flagging a claim whose rests_on target
// changed does not itself flag claims downstream of it. DetectStale compares stored
// baselines against CURRENT content, and the downstream claim's baseline is
// over its dependency's content — which review_pending does not change (see
// ContentHash's field list).
func TestRestsOnDriftPropagationIsStaged(t *testing.T) {
	hub := model.Claim{ID: "widget.contract.hub", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "doctrine v1"}
	child := model.Claim{ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "child", RestsOn: model.RestsOnIDs(hub.ID)}
	downstream := model.Claim{ID: "widget.contract.downstream", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "downstream", RestsOn: model.RestsOnIDs(child.ID)}

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

// TestDanglingRestsOnTargetRecordsNoBaseline: a rests_on id that names no
// claim in the registry has no content to snapshot, so RefreshBaseline records
// no baseline row for it — a row keyed by a missing claim would compare against
// nothing and quietly never drift. The lock policy refuses such a claim
// (missing_dependency), but reaudit --confirm re-baselines an already-locked
// claim whose target may since have gone, so the baseline writer has to hold
// this itself.
func TestDanglingRestsOnTargetRecordsNoBaseline(t *testing.T) {
	child := model.Claim{
		ID: "widget.contract.child", Facet: "contract", Module: "widget", Status: model.StatusLocked, Body: "child",
		RestsOn: model.RestsOnIDs("constitution.invariants.single-roof"),
	}
	store, err := LoadStore(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	RefreshBaseline(child, []model.Claim{child}, store)
	if ids := store.Hashes[child.ID]; len(ids) != 0 {
		t.Fatalf("a dangling rests_on target must not become a baseline; store has %v", ids)
	}
}

// The roof's record (NIT-6) round-trips through Save/LoadStore and through the
// strict DecodeStore the staged gate uses, and a store that never locked its
// roof carries no record at all.
func TestConstitutionRecordRoundTripsThroughTheStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "ledger", "lock-store.json")
	store, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if store.Constitution != nil {
		t.Fatal("a fresh store has no roof record")
	}
	f := &constitution.File{Status: model.StatusLocked, Invariants: []constitution.Entry{{Slug: "one", Body: "one roof"}}}
	rec := LockConstitution(store, f, "the human's words", time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	if rec.Hash != constitution.Hash(f) || rec.Reason != "the human's words" || rec.LockedAt != "2026-09-24T12:00:00Z" {
		t.Fatalf("record = %+v", rec)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}
	again, err := LoadStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.Constitution == nil || *again.Constitution != rec {
		t.Fatalf("record after reload = %+v, want %+v", again.Constitution, rec)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	strict, err := DecodeStore(raw)
	if err != nil {
		t.Fatalf("the strict decoder must accept the constitution key: %v", err)
	}
	if strict.Constitution == nil || strict.Constitution.Hash != rec.Hash {
		t.Fatalf("strict decode dropped the record: %+v", strict.Constitution)
	}
	if constitution.Evaluate(f.SourcePath, f, nil, again.Constitution).State != constitution.StateLocked {
		t.Fatal("the reloaded record must judge the same file locked")
	}
}
