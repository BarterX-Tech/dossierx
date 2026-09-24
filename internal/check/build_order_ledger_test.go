// build_order_ledger_test.go covers the BUILD-ORDER half of the lock-ledger
// gate — the half that shipped write-only.
//
// "dossierx build-order lock" already wrote a ledger record for the artifact it
// froze, and FORMAT.md already called a locked build order a locked artifact
// inside the same gate as a locked claim. Nothing read the record back:
// lock.Audit filters on Subject == SubjectClaim, so build-order records were the
// only records in the ledger no rule could ever fire on. A hand-edited
// .build-order.<module>.json — the implementation sequence an agent then follows
// — was exactly as invisible as it had been before the ledger existed.
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
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// writeLeftoverBuildOrder drops a leftover artifact and optional ledger row
// so tests can prove they do not refuse check.
func writeLeftoverBuildOrder(t *testing.T, cfg *config.Config, module string, record bool) []byte {
	t.Helper()
	dir := filepath.Join(cfg.Dir(), config.DefaultBuildDir, config.BuildOrderDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir leftover dir: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"module": module,
		"locked": true,
		"phases": []any{map[string]any{"name": "schema", "claims": []any{}}},
	})
	if err != nil {
		t.Fatalf("marshal leftover: %v", err)
	}
	path := filepath.Join(dir, module+".json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write leftover: %v", err)
	}
	if !record {
		return raw
	}
	storePath := filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	sum := sha256.Sum256(raw)
	lock.RecordBuildOrderApproval(store, module, hex.EncodeToString(sum[:]),
		lock.Approval{Actor: "fixture", Reason: "leftover"})
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	return raw
}

func leftoverPath(cfg *config.Config, module string) string {
	return filepath.Join(cfg.Dir(), config.DefaultBuildDir, config.BuildOrderDirName, module+".json")
}

// lockBuildOrder writes a leftover artifact plus ledger row. Other leftover
// tests still call this name; the product no longer has a lock verb.
func lockBuildOrder(t *testing.T, cfg *config.Config, _ []model.Claim, module string) {
	t.Helper()
	writeLeftoverBuildOrder(t, cfg, module, true)
}

// orderedClaim is a locked claim carrying the build_role a build order needs.
func orderedClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"build_role: behavior\n" +
		"body: |\n  a locked claim.\n"
}

// A properly locked build order must be SILENT. This is the assertion that
// makes the other two safe to ship: a gate that fires on correct state is a
// gate people turn off, and every project with a locked build order would have
// started failing check the moment these rules landed if the signature this
// gate computes disagreed by one byte with the signature the writer records.
func TestBuildOrderGate_HonestLockedOrderIsSilent(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	res := check.Status(claims, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("an honestly locked build order must produce no findings, got %v", rulesOf(res.LedgerFindings))
	}
}

// Hand-editing the frozen artifact reorders what an agent builds without
// touching a single claim. Before this rule, nothing in the engine noticed.
func TestBuildOrderGate_HandEditedArtifactIsContentDrift(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	path := leftoverPath(cfg, "widget")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	doc["excluded"] = []string{"widget.contract.smuggled"}
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal edited artifact: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write edited artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-content-drift") {
		t.Fatalf("leftover build-order artifacts must not refuse, got %v", rulesOf(res.LedgerFindings))
	}
}

// Deleting the record to clear the drift finding must be LOUDER, not quieter.
// A gate that only catches EDITED approvals is bypassed by removing the
// approval, which is the same reasoning lock-ledger-missing exists on for
// claims.
func TestBuildOrderGate_DeletingTheRecordIsLedgerMissing(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	storePath := filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	delete(store.Ledger, lock.BuildOrderLedgerKey("widget"))
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-ledger-missing") {
		t.Fatalf("leftover build-order artifacts must not refuse, got %v", rulesOf(res.LedgerFindings))
	}
}

// An artifact that has only been PROPOSED is not audited. It is a working
// document that "build-order propose" overwrites freely and that nobody has
// approved, so demanding a record for it would refuse every commit between
// propose and lock.
func TestBuildOrderGate_ProposedButUnlockedIsNotAudited(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", false)

	res := check.Status(claims, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("an unlocked build order must not be audited, got %v", rulesOf(res.LedgerFindings))
	}
}

// Deleting the artifact must not be QUIETER than editing it.
//
// Every forward rule starts from the file, so removing the file removed the
// module from the gate's evidence set entirely: the standing record stayed in
// the ledger, `check` reported nothing, and `rm build/build-order/widget.json` was a
// strictly better attack than the hand edit the gate was built to catch. This is
// the reverse sweep, the same shape as lock-ledger-abandoned for claims.
func TestBuildOrderGate_DeletingTheArtifactIsLedgerAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	if err := os.Remove(leftoverPath(cfg, "widget")); err != nil {
		t.Fatalf("remove artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-ledger-abandoned") {
		t.Fatalf("leftover build-order artifacts must not refuse after delete, got %v",
			rulesOf(res.LedgerFindings))
	}
}

// Dropping the module from project.config.yaml is the same act by another route:
// the gate iterates cfg.Modules, so a module removed from the config takes its
// locked build order out of the audit with it while the approval still stands.
func TestBuildOrderGate_DroppingTheModuleFromConfigIsLedgerAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	// The same project, re-read with a config that no longer declares the
	// module. The artifact and the record are both still on disk.
	if err := os.WriteFile(filepath.Join(cfg.Dir(), "project.config.yaml"),
		[]byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - other\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("rewrite config: %v", err)
	}
	narrowed, err := config.LoadConfig(filepath.Join(cfg.Dir(), "project.config.yaml"))
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	res := check.Status(claims, narrowed)
	if hasRule(res.LedgerFindings, "build-order-ledger-abandoned") {
		t.Fatalf("leftover build-order artifacts must not refuse when the module left the config, got %v",
			rulesOf(res.LedgerFindings))
	}
}

// A RELEASED record is a human's decision on the record, so the sweep stays
// quiet for it — the same rule lock-ledger-abandoned follows for an unlocked,
// then deleted, claim.
func TestBuildOrderGate_ReleasedRecordIsNotAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	storePath := filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	if !lock.ReleaseBuildOrderApproval(store, "widget", lock.Approval{Actor: "fixture", Reason: "re-proposing"}) {
		t.Fatalf("expected a build-order record to release")
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	if err := os.Remove(leftoverPath(cfg, "widget")); err != nil {
		t.Fatalf("remove artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-ledger-abandoned") {
		t.Fatalf("a released record must not be reported abandoned, got %v", rulesOf(res.LedgerFindings))
	}
}

// One boolean, in the audited file, used to disarm every rule above.
//
// Set "locked": false in .build-order.<module>.json and change nothing else: the
// forward rules skip it (an unlocked artifact is a proposal nobody approved) and
// the reverse sweep skips it too (a present artifact is the forward loop's
// business), so the approved implementation sequence left the gate's evidence set
// entirely while staying exactly where it was for an agent to read. check
// reported ok, with the unreleased ledger record still standing beside it.
func TestBuildOrderGate_HandFlippedLockedFalseIsLedgerOrphan(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	path := leftoverPath(cfg, "widget")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if doc["locked"] != true {
		t.Fatalf("precondition: the artifact should be locked, got %v", doc["locked"])
	}
	doc["locked"] = false
	flipped, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal flipped artifact: %v", err)
	}
	if err := os.WriteFile(path, flipped, 0o644); err != nil {
		t.Fatalf("write flipped artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-ledger-orphan") {
		t.Fatalf("expected %s, got %v", "build-order-ledger-orphan", rulesOf(res.LedgerFindings))
	}
}

// The honest re-propose window must stay silent: propose overwrites the locked
// artifact with a fresh unlocked one, and the commits between that and the lock
// which follows must not be refused.
//
// What makes the window identifiable is the RELEASE. propose writes two things —
// the artifact and the release of the approval the artifact just destroyed — so
// the honest window is an unlocked artifact under a RELEASED record, and a hand
// edit is an unlocked artifact under a standing one. This test performs both
// writes, which is what the propose command does; TestBuildOrderGate_
// FlagFlipWithAContentEditIsLedgerOrphan is the same fixture minus the release.
func TestBuildOrderGate_ReProposedArtifactIsNotAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	writeLeftoverBuildOrder(t, cfg, "widget", false)
	storePath := filepath.Join(cfg.Dir(), "build", "ledger", "lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	if !lock.ReleaseBuildOrderApproval(store, "widget",
		lock.Approval{Actor: "fixture", Reason: "superseded by propose"}) {
		t.Fatal("precondition: there should have been a standing record to release")
	}
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}

	res := check.Status(claims, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("a re-proposed (unlocked) artifact whose record was released must not be audited at all, got %v", rulesOf(res.LedgerFindings))
	}
}

// The attack the orphan rule's old exactness let through, and the reason that
// exception is gone.
//
// The predicate used to be "the artifact re-signs to its record once the locked
// flag is put back" — which identifies a LONE flag flip, and by construction
// cannot identify a flip made together with a content edit, because a content
// edit re-signs to something else. So gutting the approved implementation
// sequence AND clearing its locked flag in one edit was strictly quieter than
// clearing the flag alone: check reported ok:true, exit 0, and even offered
// "dossierx build-order lock" as a next step over a sequence nobody approved.
//
// Reproduced end to end before the fix on a real project: propose + lock a
// widget order, empty the schema phase and set "locked": false in the same
// write, then `dossierx check --validate` -> "OK (read-only: nothing written)",
// exit 0, zero ledger findings.
func TestBuildOrderGate_FlagFlipWithAContentEditIsLedgerOrphan(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	path := leftoverPath(cfg, "widget")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	if doc["locked"] != true {
		t.Fatalf("precondition: the artifact should be locked, got %v", doc["locked"])
	}
	// Both edits in one write: the flag AND the sequence itself.
	doc["locked"] = false
	phases, ok := doc["phases"].([]any)
	if !ok || len(phases) == 0 {
		t.Fatalf("precondition: expected phases in the artifact, got %v", doc["phases"])
	}
	first, ok := phases[0].(map[string]any)
	if !ok {
		t.Fatalf("precondition: expected a phase object, got %T", phases[0])
	}
	first["claims"] = []any{}
	tampered, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal tampered artifact: %v", err)
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatalf("write tampered artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-ledger-orphan") {
		t.Fatalf("expected %s for a flag flip made together with a content edit, got %v",
			"build-order-ledger-orphan", rulesOf(res.LedgerFindings))
	}
}

// A CORRUPT artifact was audited by NO rule, which made destroying the approved
// implementation sequence quieter than deleting it. Reproduced as three states
// on one project: a valid locked artifact under a standing record -> ok:true;
// `rm build/build-order/widget.json` -> ok:false ['build-order-ledger-abandoned'];
// truncate the same file mid-token -> ok:true, zero findings, exit 0.
//
// collectBuildOrderStates set Unreadable and left Present false, so the forward
// loop skipped it (it audits present artifacts) and the reverse sweep skipped it
// too (an unreadable file is not evidence of deletion). The flag's doc comment
// deferred the case to "check's own build-order reporting", which did not exist.
func TestBuildOrderGate_CorruptArtifactIsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	path := leftoverPath(cfg, "widget")
	if err := os.WriteFile(path, []byte(`{ "module": "widget", "locked": tr`), 0o644); err != nil {
		t.Fatalf("truncate artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, "build-order-unreadable") {
		t.Fatalf("a corrupt build-order artifact must be reported, got %v", rulesOf(res.LedgerFindings))
	}
	// It must not ALSO be reported as deleted: the file is right there, and
	// "restore it from version control" is the recovery for a different state.
	if hasRule(res.LedgerFindings, "build-order-ledger-abandoned") {
		t.Fatalf("a corrupt artifact is not evidence of deletion, got %v", rulesOf(res.LedgerFindings))
	}
	if _, err := check.Run(claims, cfg); err != nil {
		t.Fatalf("a leftover corrupt build-order artifact must not refuse check: %v", err)
	}
}

// check reported NOTHING about a build order, and a locked one going stale is
// the ordinary outcome of the fully sanctioned lifecycle: unlock a covered
// claim, edit it, re-lock it. Reproduced end to end — `build-order status` said
// stale:true with stale_ids, while `dossierx check` and `dossierx check
// --staged` (the pre-commit hook and CI path) both said ok:true, exit 0, with
// next_steps mentioning only the dependent claim's review_pending. The
// build-order skill tells an agent to act "whenever a locked build order reports
// stale"; the loop command never reported it.
func TestNextSteps_StaleLockedBuildOrderIsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", true)

	res := check.Status(claims, cfg)
	if len(res.BuildOrders) != 0 {
		t.Fatalf("leftover build orders are not a check surface, got %+v", res.BuildOrders)
	}

	// The sanctioned change: the claim's build_role moves (unlock -> edit ->
	// lock, compressed here to the edit the approval path would have written).
	// A derivation input, not prose: since issue #58 a body edit alone leaves
	// the order current.
	path := filepath.Join(cfg.ClaimsDir, "a.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	edited := strings.Replace(string(raw), "build_role: behavior\n", "build_role: schema\n", 1)
	if edited == string(raw) {
		t.Fatalf("fixture precondition: the edit did not apply")
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	claims = reload(t, cfg)
	armLedger(t, cfg, claims) // the re-lock's approval record

	res = check.Status(claims, cfg)
	if len(res.BuildOrders) != 0 {
		t.Fatalf("leftover build orders are not a check surface, got %+v", res.BuildOrders)
	}
	for _, h := range res.NextSteps {
		if strings.Contains(h, "build-order") || strings.Contains(h, "stale") {
			t.Fatalf("check must not require a stale build-order recovery, got %v", res.NextSteps)
		}
	}
}

// The other silent state: an artifact that exists and was never locked — an
// abandoned propose->lock flow, which produced next_steps: null forever.
func TestNextSteps_UnlockedBuildOrderIsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	writeLeftoverBuildOrder(t, cfg, "widget", false)

	res := check.Status(claims, cfg)
	if len(res.BuildOrders) != 0 {
		t.Fatalf("leftover build orders are not a check surface, got %+v", res.BuildOrders)
	}
	for _, h := range res.NextSteps {
		if strings.Contains(h, "build-order") || strings.Contains(h, "never locked") {
			t.Fatalf("check must not require locking a leftover proposal, got %v", res.NextSteps)
		}
	}
}
