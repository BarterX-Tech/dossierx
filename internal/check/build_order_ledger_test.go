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

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// lockBuildOrder proposes and locks module's build order and records its
// approval in the ledger, mirroring cmd/dossierx's own two steps (buildorder.Lock
// then lock.RecordBuildOrderApproval over a sha256 of the artifact's JSON).
func lockBuildOrder(t *testing.T, cfg *config.Config, claims []model.Claim, module string) *buildorder.Artifact {
	t.Helper()

	path := buildorder.ArtifactPath(cfg, module)
	proposed, err := buildorder.Propose(claims, cfg, module)
	if err != nil {
		t.Fatalf("propose %s: %v", module, err)
	}
	if err := buildorder.WriteArtifact(proposed, path); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	artifact, err := buildorder.Lock(path, claims, cfg)
	if err != nil {
		t.Fatalf("lock build order: %v", err)
	}

	storePath := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	sum := sha256.Sum256(raw)
	lock.RecordBuildOrderApproval(store, module, hex.EncodeToString(sum[:]),
		lock.Approval{Actor: "fixture", Reason: "order approved"})
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	return artifact
}

// orderedClaim is a locked claim carrying the build_role a build order needs.
func orderedClaim(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"build_role: behavior\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
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
	lockBuildOrder(t, cfg, claims, "widget")

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
	lockBuildOrder(t, cfg, claims, "widget")

	path := buildorder.ArtifactPath(cfg, "widget")
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
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderContentDrift) {
		t.Fatalf("expected %s, got %v", check.RuleBuildOrderContentDrift, rulesOf(res.LedgerFindings))
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
	lockBuildOrder(t, cfg, claims, "widget")

	storePath := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	store, err := lock.LoadStore(storePath)
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	delete(store.Ledger, lock.BuildOrderLedgerKey("widget"))
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerMissing) {
		t.Fatalf("expected %s, got %v", check.RuleBuildOrderLedgerMissing, rulesOf(res.LedgerFindings))
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
	artifact, err := buildorder.Propose(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if err := buildorder.WriteArtifact(artifact, buildorder.ArtifactPath(cfg, "widget")); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("an unlocked build order must not be audited, got %v", rulesOf(res.LedgerFindings))
	}
}

// Deleting the artifact must not be QUIETER than editing it.
//
// Every forward rule starts from the file, so removing the file removed the
// module from the gate's evidence set entirely: the standing record stayed in
// the ledger, `check` reported nothing, and `rm .build-order.widget.json` was a
// strictly better attack than the hand edit the gate was built to catch. This is
// the reverse sweep, the same shape as lock-ledger-abandoned for claims.
func TestBuildOrderGate_DeletingTheArtifactIsLedgerAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")

	if err := os.Remove(buildorder.ArtifactPath(cfg, "widget")); err != nil {
		t.Fatalf("remove artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerAbandoned) {
		t.Fatalf("expected %s after the locked artifact was deleted, got %v",
			check.RuleBuildOrderLedgerAbandoned, rulesOf(res.LedgerFindings))
	}
}

// Dropping the module from project.config.yaml is the same act by another route:
// the gate iterates cfg.Modules, so a module removed from the config takes its
// locked build order out of the audit with it while the approval still stands.
func TestBuildOrderGate_DroppingTheModuleFromConfigIsLedgerAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")

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
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerAbandoned) {
		t.Fatalf("expected %s when the module left the config, got %v",
			check.RuleBuildOrderLedgerAbandoned, rulesOf(res.LedgerFindings))
	}
}

// A RELEASED record is a human's decision on the record, so the sweep stays
// quiet for it — the same rule lock-ledger-abandoned follows for an unlocked,
// then deleted, claim.
func TestBuildOrderGate_ReleasedRecordIsNotAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")

	storePath := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
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
	if err := os.Remove(buildorder.ArtifactPath(cfg, "widget")); err != nil {
		t.Fatalf("remove artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerAbandoned) {
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
	lockBuildOrder(t, cfg, claims, "widget")

	path := buildorder.ArtifactPath(cfg, "widget")
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
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerOrphan) {
		t.Fatalf("expected %s, got %v", check.RuleBuildOrderLedgerOrphan, rulesOf(res.LedgerFindings))
	}
}

// The honest re-propose window must stay silent: propose overwrites the locked
// artifact with a fresh unlocked one, and the record still stands until the lock
// that follows. A gate that failed here would refuse every commit between the
// two halves of the documented flow.
func TestBuildOrderGate_ReProposedArtifactIsNotAbandoned(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	lockBuildOrder(t, cfg, claims, "widget")

	reproposed, err := buildorder.Propose(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("re-propose: %v", err)
	}
	if err := buildorder.WriteArtifact(reproposed, buildorder.ArtifactPath(cfg, "widget")); err != nil {
		t.Fatalf("write re-proposed artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if len(res.LedgerFindings) != 0 {
		t.Fatalf("a re-proposed (unlocked) artifact must not be audited at all, got %v", rulesOf(res.LedgerFindings))
	}
}

// A CORRUPT artifact was audited by NO rule, which made destroying the approved
// implementation sequence quieter than deleting it. Reproduced as three states
// on one project: a valid locked artifact under a standing record -> ok:true;
// `rm .build-order.widget.json` -> ok:false ['build-order-ledger-abandoned'];
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
	lockBuildOrder(t, cfg, claims, "widget")

	path := buildorder.ArtifactPath(cfg, "widget")
	if err := os.WriteFile(path, []byte(`{ "module": "widget", "locked": tr`), 0o644); err != nil {
		t.Fatalf("truncate artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if !hasRule(res.LedgerFindings, check.RuleBuildOrderUnreadable) {
		t.Fatalf("a corrupt build-order artifact must be reported, got %v", rulesOf(res.LedgerFindings))
	}
	// It must not ALSO be reported as deleted: the file is right there, and
	// "restore it from version control" is the recovery for a different state.
	if hasRule(res.LedgerFindings, check.RuleBuildOrderLedgerAbandoned) {
		t.Fatalf("a corrupt artifact is not evidence of deletion, got %v", rulesOf(res.LedgerFindings))
	}
	// And the run fails, rather than exiting 0 over a destroyed sequence.
	if _, err := check.Run(claims, cfg); err == nil {
		t.Fatalf("expected the gate to fail the run on a corrupt build-order artifact")
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
	lockBuildOrder(t, cfg, claims, "widget")

	// Precondition: a fresh locked order is silent, in the hint and in the data.
	res := check.Status(claims, cfg)
	if len(res.BuildOrders) != 1 || res.BuildOrders[0].Stale {
		t.Fatalf("fixture precondition: the locked order must start fresh, got %+v", res.BuildOrders)
	}

	// The sanctioned change: the claim's content moves (unlock -> edit -> lock,
	// compressed here to the edit the approval path would have written).
	path := filepath.Join(cfg.ClaimsDir, "a.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	edited := strings.Replace(string(raw), "a locked claim.", "a locked claim, re-approved.", 1)
	if edited == string(raw) {
		t.Fatalf("fixture precondition: the edit did not apply")
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("rewrite claim: %v", err)
	}
	claims = reload(t, cfg)
	armLedger(t, cfg, claims) // the re-lock's approval record

	res = check.Status(claims, cfg)
	if len(res.BuildOrders) != 1 || !res.BuildOrders[0].Stale {
		t.Fatalf("expected the build order reported stale in the data, got %+v", res.BuildOrders)
	}
	if got := res.BuildOrders[0].StaleIDs; len(got) != 1 || got[0] != "widget.contract.a" {
		t.Fatalf("expected the changed claim named, got %v", got)
	}

	var hint string
	for _, h := range res.NextSteps {
		if strings.Contains(h, "stale") {
			hint = h
		}
	}
	if hint == "" {
		t.Fatalf("expected a stale build-order next step, got %v", res.NextSteps)
	}
	if !strings.Contains(hint, "build-order propose --module widget") ||
		!strings.Contains(hint, "build-order lock --module widget") {
		t.Fatalf("the hint must name the two commands that fix it, got %q", hint)
	}
}

// The other silent state: an artifact that exists and was never locked — an
// abandoned propose->lock flow, which produced next_steps: null forever.
func TestNextSteps_UnlockedBuildOrderIsReported(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
	})
	proposed, err := buildorder.Propose(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if err := buildorder.WriteArtifact(proposed, buildorder.ArtifactPath(cfg, "widget")); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	res := check.Status(claims, cfg)
	if len(res.BuildOrders) != 1 || res.BuildOrders[0].Locked {
		t.Fatalf("expected one unlocked build order reported, got %+v", res.BuildOrders)
	}
	found := false
	for _, h := range res.NextSteps {
		if strings.Contains(h, "never locked") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a hint for the abandoned propose->lock flow, got %v", res.NextSteps)
	}
}
