// parent_scope_test.go covers the two things the scope guard has to derive from
// the PARENT COMMIT rather than from the commit it is judging.
//
// history_test.go pins the guard's original shape: a store that DISAPPEARED, and
// a claims_dir that MOVED, both read at the paths the CURRENT commit uses. Both
// of those questions were asked in a form the commit under judgement gets to
// answer for itself, and each one has a one-commit reply:
//
//   - "is the store still tracked?" is answered yes by a store that is still
//     there and has been EMPTIED of its standing records. (RULE A)
//   - "what was the parent's claims_dir?" is looked up at the CURRENT config's
//     path, so MOVING project.config.yaml makes the lookup miss and the parent
//     contribute nothing at all. (RULE B)
//
// Every test here is paired with the honest edit that reaches the same shape, and
// the honest one has to stay silent — a gate that fires on correct state is the
// outage that implicit grandfathering existed to prevent.
package check_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// emptyLedger rewrites the lock store IN PLACE with its "ledger" map emptied and
// every other byte of the document left as it was — the edit an attacker makes
// instead of `git rm`, because it leaves a tracked, well-formed, current-version
// store behind for every presence check to be satisfied by.
func emptyLedger(t *testing.T, cfg *config.Config) {
	t.Helper()
	path := filepath.Join(cfg.Dir(), ".dossierx-lock-store.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("empty ledger: read store: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("empty ledger: parse store: %v", err)
	}
	if _, ok := doc["ledger"]; !ok {
		t.Fatalf("fixture precondition: the store has no ledger to empty:\n%s", raw)
	}
	doc["ledger"] = json.RawMessage(`{}`)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("empty ledger: encode store: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("empty ledger: write store: %v", err)
	}
}

// relocateConfig git-mv's project.config.yaml into dir (repository-relative, ""
// for the root), rewrites claims_dir to claimsDir, and returns the config as it
// now reads from its NEW home — which is the config the CLI would load after the
// move, and therefore the one the gate is handed.
func relocateConfig(t *testing.T, cfg *config.Config, dir, claimsDir string) *config.Config {
	t.Helper()
	root := cfg.Dir()
	if dir != "" && dir != "." {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	dst := filepath.Join(dir, config.FileName)
	git(t, root, "mv", config.FileName, dst)

	abs := filepath.Join(root, dst)
	if err := os.WriteFile(abs, []byte(configWithClaimsDir(claimsDir)), 0o644); err != nil {
		t.Fatalf("write relocated config: %v", err)
	}
	reloaded, err := config.LoadConfig(abs)
	if err != nil {
		t.Fatalf("reload relocated config: %v", err)
	}
	return reloaded
}

// carryStores git-mv's both integrity stores into dir, which is what a project
// moving its config does — the stores are resolved beside the config, so a move
// that leaves them behind is a move that loses them.
func carryStores(t *testing.T, root, dir string) {
	t.Helper()
	for _, name := range []string{".dossierx-lock-store.json", digest.StoreFileName} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			continue
		}
		git(t, root, "mv", name, filepath.Join(dir, name))
	}
}

// ---------------------------------------------------------------------
// RULE A — the store's CONTENT, parent vs commit
// ---------------------------------------------------------------------

// THE EMPTIED LEDGER. Deleting .dossierx-lock-store.json is refused; emptying it
// was not, because the guard asked whether the path was still TRACKED and a
// store that has been emptied of its standing records is still tracked.
//
// The reproduction is one commit and it needs no config edit at all, so there is
// nothing in the diff for a reviewer to notice: `git mv` the claim files out of
// claims_dir (which stays exactly as it is), and overwrite the ledger's map with
// {}. Afterwards the registry is EMPTY, so every forward rule has no claim to
// name; the store file is present and current-version, so the presence rule is
// satisfied; and the reverse sweep that would have called the standing record
// abandoned has no record left to walk. The claim is still tracked, still says
// status: locked, and is audited by nothing.
func TestStaged_RefusesAnEmptiedLedgerWhoseClaimsAreStillTracked(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	git(t, root, "mv", filepath.Join("claims", "locked.yaml"), filepath.Join("archive", "locked.yaml"))
	emptyLedger(t, cfg)
	git(t, root, "add", "-A")

	rules, res := stagedRules(t, cfg)
	if !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("emptying the ledger while its claims stay tracked must be refused exactly like deleting it: got %v", rules)
	}

	joined := strings.Join(messagesOf(res.LedgerFindings), "\n")
	for _, want := range []string{"widget.contract.locked", "git checkout"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("the refusal must name the dropped approval and the recovery (%q missing) — got:\n%s", want, joined)
		}
	}
}

// The same edit WITHOUT the move: the ledger is emptied and the claim stays
// where it is. This one was already refused (lock-ledger-missing had a claim to
// name), and it must STILL be refused after RULE A, by something — the point of
// the pairing is that the two shapes are the same event and neither is quiet.
func TestStaged_RefusesAnEmptiedLedgerWithTheClaimsInPlace(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	emptyLedger(t, cfg)
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, cfg)
	if len(rules) == 0 {
		t.Fatalf("emptying the ledger under a locked claim must be refused")
	}
}

// THE HONEST UNLOCK. unlock -> fix -> lock is the sanctioned way to change a
// locked claim, and its middle state is exactly the shape RULE A looks for: a
// claim the parent held a standing record for, whose record is no longer
// standing in the commit under judgement.
//
// What separates it from the tamper is that an unlock RELEASES the record rather
// than removing it (lock.ReleaseApproval), so the evidence of the approval — and
// of its withdrawal — is still in the file. RULE A therefore triggers on records
// that are GONE, never on records that are released, and this is what says so.
func TestStaged_UnlockFixLockStaysSilent(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()
	claimFile := filepath.Join(root, "claims", "locked.yaml")

	// STEP 1 — unlock: the claim becomes draft and its record is released.
	store, err := lock.LoadStore(filepath.Join(root, ".dossierx-lock-store.json"))
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("fixture precondition: expected one claim, got %d", len(claims))
	}
	unlocked := lock.Unlock(claims[0], store, lock.Approval{Actor: "fixture", Reason: "the human asked"})
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	if err := os.WriteFile(claimFile, []byte(draftClaim(unlocked.ID)), 0o644); err != nil {
		t.Fatalf("write unlocked claim: %v", err)
	}
	git(t, root, "add", "-A")

	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("an honest unlock must be silent: got %v", rules)
	}
	git(t, root, "commit", "-qm", "unlock the claim")

	// STEP 2 — fix and re-lock: a NEW approval, standing again.
	relocked := unlocked
	relocked.Status = model.StatusLocked
	relocked.Body = "a locked claim, revised through the approval path.\n"
	if err := os.WriteFile(claimFile, []byte(lockedClaim(relocked.ID)), 0o644); err != nil {
		t.Fatalf("write relocked claim: %v", err)
	}
	fixed, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("reload claims: %v", err)
	}
	store, err = lock.LoadStore(filepath.Join(root, ".dossierx-lock-store.json"))
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	lock.RecordApproval(store, fixed[0], lock.Approval{Actor: "fixture", Reason: "the human approved the fix"})
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	git(t, root, "add", "-A")

	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("unlock -> fix -> lock must be silent at every step: got %v", rules)
	}
}

// A PRE-LEDGER PROJECT RUNNING `migrate --adopt` ONCE adds records and removes
// none, so RULE A — which reads the records the PARENT held — has nothing to
// compare and nothing to say. This is the upgrade path every v0.2.x project
// takes, and refusing it would make the release unshippable.
func TestStaged_AdoptionCommitStaysSilentUnderRuleA(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "claims", config.FileName)
	git(t, root, "commit", "-qm", "pre-adoption: claims, no stores")

	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("load claims: %v", err)
	}
	armDigests(t, cfg, claims)
	git(t, root, "add", "-A")

	if rules, _ := stagedRules(t, cfg); len(rules) != 0 {
		t.Fatalf("the one-time adoption commit must pass: got %v", rules)
	}
}

// ---------------------------------------------------------------------
// RULE B — the parent's scope, from the PARENT's own tree
// ---------------------------------------------------------------------

// MOVING THE CONFIG collapsed the scope with no file deleted and no claims_dir
// edit visible against the parent: the parent's config was looked up at the
// CURRENT config's path, so after `git mv project.config.yaml docs/` the lookup
// missed, the parent contributed nothing, and the scope came from the new config
// alone. The claim files stay exactly where they are and stop being audited.
func TestStaged_RefusesAConfigMoveThatStrandsTheClaims(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	// The config moves into docs/ and points at the innocent tracked directory
	// from there. Both stores travel with it, so nothing is missing anywhere.
	moved := relocateConfig(t, cfg, "docs", "../archive")
	carryStores(t, root, "docs")
	git(t, root, "add", "-A")

	rules, res := stagedRules(t, moved)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("moving the config must not hide a repointed claims_dir: got %v", rules)
	}
	joined := strings.Join(messagesOf(res.LedgerFindings), "\n")
	if !strings.Contains(joined, "claims/locked.yaml") {
		t.Fatalf("the refusal must name the stranded claim — got:\n%s", joined)
	}
}

// The same move with the LEDGER LEFT BEHIND. The stores are resolved beside the
// config, so after the move the gate looks for docs/.dossierx-lock-store.json —
// a path the parent never had either. Asking "did the parent carry the store at
// the path this commit uses?" answers no for a store that was there all along,
// which is a deletion the presence rule could not see.
func TestStaged_RefusesAConfigMoveThatLeavesTheLedgerBehind(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	moved := relocateConfig(t, cfg, "docs", "../claims")
	git(t, root, "rm", "-q", ".dossierx-lock-store.json")
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, moved)
	if !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("a config move that drops the lock ledger must be refused: got %v", rules)
	}
}

// THE WHOLE COLLAPSE IN ONE COMMIT, and the reason both halves of RULE B are
// one change: move the config, point claims_dir from its new home at the
// innocent tracked directory, and leave both stores where they were. Nothing is
// deleted from the repository, the claims_dir LINE is not edited into anything
// suspicious, and every path the gate reads — claims_dir, the lock ledger, the
// comment digest store — is now looked up somewhere the parent never had one. On
// the shipped binary this commit produced zero findings and exit 0.
func TestStaged_RefusesTheWholeConfigMoveCollapse(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	moved := relocateConfig(t, cfg, "docs", "../archive")
	git(t, root, "add", "-A")

	rules, _ := stagedRules(t, moved)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("the stranded claims must be reported: got %v", rules)
	}
	if !contains(rules, check.RuleIntegrityStoreRemoved) {
		t.Fatalf("the ledger left behind by the config move must be reported: got %v", rules)
	}
}

// TWO PROJECTS MOVING AT ONCE is the one genuinely ambiguous state, and the
// answer is an ADVISORY rather than a guess in either direction. Guessing which
// vanished config was ours would compare this project's claims_dir against a
// stranger's and refuse honest work; guessing that neither was ours is the
// silence this whole file exists to end. "This could not be compared" is what a
// shallow checkout is told, for the same reason.
func TestStaged_TwoProjectsMovingAtOnceIsAnAdvisoryNotAGuess(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	other := filepath.Join(root, "other")
	if err := os.MkdirAll(filepath.Join(other, "spec"), 0o755); err != nil {
		t.Fatalf("mkdir other: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, config.FileName), []byte(configWithClaimsDir("spec")), 0o644); err != nil {
		t.Fatalf("write other config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "spec", "draft.yaml"), []byte(draftClaim("widget.contract.other")), 0o644); err != nil {
		t.Fatalf("write other claim: %v", err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "a second project in the same repository")

	// BOTH configs move in one commit, so neither is where it was and neither
	// can be told from the other.
	if err := os.MkdirAll(filepath.Join(root, "elsewhere"), 0o755); err != nil {
		t.Fatalf("mkdir elsewhere: %v", err)
	}
	git(t, root, "mv", filepath.Join("other", config.FileName), filepath.Join("elsewhere", config.FileName))
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	git(t, root, "mv", "claims", filepath.Join("docs", "claims"))
	moved := relocateConfig(t, cfg, "docs", "claims")
	carryStores(t, root, "docs")
	git(t, root, "add", "-A")

	sp, err := check.Staged(moved)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	res := check.StatusStaged(sp, moved)
	if rules := rulesOf(res.LedgerFindings); contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("an ambiguous parent must not be guessed into a refusal: got %v", rules)
	}
	said := false
	for _, h := range res.NextSteps {
		if strings.Contains(h, "could NOT identify") {
			said = true
		}
	}
	if !said {
		t.Fatalf("an ambiguous parent must SAY the comparison did not happen; next_steps was %v", res.NextSteps)
	}
}

// THE SANCTIONED CONFIG MOVE, which has to stay silent or the guard is a trap:
// the config, the claims and both stores all move together and claims_dir is
// adjusted to keep pointing at the same files.
func TestStaged_ConfigMoveThatCarriesEverythingIsAccepted(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	git(t, root, "mv", "claims", filepath.Join("docs", "claims"))
	moved := relocateConfig(t, cfg, "docs", "claims")
	carryStores(t, root, "docs")
	git(t, root, "add", "-A")

	rules, res := stagedRules(t, moved)
	if len(rules) != 0 {
		t.Fatalf("a config move that carries everything with it must be accepted: got %v", rules)
	}
	if len(res.LintErrors) != 0 {
		t.Fatalf("unexpected lint errors after a sanctioned config move: %v", res.LintErrors)
	}

	sp, err := check.Staged(moved)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if len(sp.Claims) != 1 {
		t.Fatalf("after the move the gate must still see the claim, got %d", len(sp.Claims))
	}
}

// A SECOND PROJECT IN THE SAME REPOSITORY must not be mistaken for this one's
// parent config. The parent is located by finding the config that VANISHED from
// its old path, so a config that is still exactly where it was belongs to
// somebody else and is never read as ours.
func TestStaged_ASecondProjectsConfigIsNotMistakenForTheParent(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	// A second, unrelated dossierx project living in the same repository.
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(filepath.Join(other, "spec"), 0o755); err != nil {
		t.Fatalf("mkdir other: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, config.FileName), []byte(configWithClaimsDir("spec")), 0o644); err != nil {
		t.Fatalf("write other config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "spec", "draft.yaml"), []byte(draftClaim("widget.contract.other")), 0o644); err != nil {
		t.Fatalf("write other claim: %v", err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "a second project in the same repository")

	// Our project moves, honestly, carrying everything.
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	git(t, root, "mv", "claims", filepath.Join("docs", "claims"))
	moved := relocateConfig(t, cfg, "docs", "claims")
	carryStores(t, root, "docs")
	git(t, root, "add", "-A")

	if rules, _ := stagedRules(t, moved); len(rules) != 0 {
		t.Fatalf("another project's config must not be read as this one's parent: got %v", rules)
	}
}

// ---------------------------------------------------------------------
// RULE B, the escape hatch — claims_dir pointed OUT of the repository
// ---------------------------------------------------------------------

// POINTING claims_dir OUTSIDE THE WORK TREE reached the exit-0 escape hatch
// before the comparison ran: "claims_dir is outside the git work tree, so no
// commit can carry it" is true of the NEW value and says nothing about the old
// one. The parent audited claims that are still tracked, still locked, and now
// judged by nothing at all — a strictly larger collapse than any in-repository
// repoint, reported as "nothing to evaluate".
func TestStaged_RefusesClaimsDirEscapingTheWorkTree(t *testing.T) {
	cfg := scopeFixture(t)
	root := cfg.Dir()

	escaped := repoint(t, cfg, "../outside-claims")
	git(t, root, "add", "-A")

	sp, err := check.Staged(escaped)
	if err != nil {
		t.Fatalf("pointing claims_dir out of the repository must be a refusal, not an error: %v", err)
	}
	res := check.StatusStaged(sp, escaped)
	rules := rulesOf(res.LedgerFindings)
	if !contains(rules, check.RuleClaimsScopeNarrowed) {
		t.Fatalf("claims_dir leaving the repository must be refused: got %v", rules)
	}
	joined := strings.Join(messagesOf(res.LedgerFindings), "\n")
	if !strings.Contains(joined, "claims/locked.yaml") {
		t.Fatalf("the refusal must name the claims left behind — got:\n%s", joined)
	}
}

// A PROJECT WHOSE claims_dir HAS ALWAYS BEEN OUTSIDE the work tree is not
// collapsing anything: no commit ever carried those claims, so there is nothing
// for the gate to have lost. It stays the exit-0 escape hatch, which is what
// keeps "run check --staged in CI" working for a checkout that is not a
// repository's whole story.
func TestStaged_AlwaysOutsideClaimsDirIsStillTheEscapeHatch(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "repo")
	outer := filepath.Join(root, "outside-claims")
	for _, d := range []string{inner, outer} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	cfgPath := filepath.Join(inner, config.FileName)
	if err := os.WriteFile(cfgPath, []byte(configWithClaimsDir("../outside-claims")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outer, "draft.yaml"), []byte(draftClaim("widget.contract.one")), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	gitRepo(t, inner)
	git(t, inner, "add", "-A")
	git(t, inner, "commit", "-qm", "claims have always lived outside the repository")

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, err := check.Staged(cfg); !errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("a claims_dir that was never inside the repository must stay the exit-0 escape hatch, got %v", err)
	}
}

// ---------------------------------------------------------------------
// the three entry points, on every tampered tree above
// ---------------------------------------------------------------------

// CHECK, CHECK --VALIDATE AND CHECK --STAGED MUST AGREE, in the one direction
// that can be true of them (see TestStaged_IsNeverLaxerThanTheWorktreeGate for
// why it is directional and not symmetric): whatever the one-tree gate reports,
// --staged reports too.
//
// It is asserted over every tamper this file adds, because a scope collapse is
// precisely the shape that makes the one-tree gate go quiet — so each of these
// trees is a chance for --staged to have gone quiet WITH it.
func TestStaged_AgreesWithTheWorktreeGateOnTheParentScopeTampers(t *testing.T) {
	tampers := map[string]func(t *testing.T) *config.Config{
		"emptied ledger, claims moved out of scope": func(t *testing.T) *config.Config {
			cfg := scopeFixture(t)
			git(t, cfg.Dir(), "mv", filepath.Join("claims", "locked.yaml"), filepath.Join("archive", "locked.yaml"))
			emptyLedger(t, cfg)
			git(t, cfg.Dir(), "add", "-A")
			return cfg
		},
		"emptied ledger, claims in place": func(t *testing.T) *config.Config {
			cfg := scopeFixture(t)
			emptyLedger(t, cfg)
			git(t, cfg.Dir(), "add", "-A")
			return cfg
		},
		"config moved, claims stranded": func(t *testing.T) *config.Config {
			cfg := scopeFixture(t)
			moved := relocateConfig(t, cfg, "docs", "../archive")
			carryStores(t, cfg.Dir(), "docs")
			git(t, cfg.Dir(), "add", "-A")
			return moved
		},
		"config moved, ledger left behind": func(t *testing.T) *config.Config {
			cfg := scopeFixture(t)
			moved := relocateConfig(t, cfg, "docs", "../claims")
			git(t, cfg.Dir(), "rm", "-q", ".dossierx-lock-store.json")
			git(t, cfg.Dir(), "add", "-A")
			return moved
		},
	}

	for name, build := range tampers {
		t.Run(name, func(t *testing.T) {
			cfg := build(t)

			claims, err := loader.LoadClaims(cfg.ClaimsDir)
			if err != nil {
				t.Fatalf("load claims: %v", err)
			}
			worktree := rulesOf(check.Status(claims, cfg).LedgerFindings)
			staged, _ := stagedRules(t, cfg)

			if len(staged) == 0 {
				t.Fatalf("--staged must refuse this tree; it reported nothing")
			}
			for _, want := range worktree {
				if !contains(staged, want) {
					t.Fatalf("check --staged is LAXER than the worktree gate: missing %q\n--staged:   %v\n--validate: %v", want, staged, worktree)
				}
			}
		})
	}
}
