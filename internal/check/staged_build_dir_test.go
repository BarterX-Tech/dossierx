// staged_build_dir_test.go pins how `check --staged` reads the build/ layout
// out of the index: the legacy-root refusal it shares with every other verb,
// the build-order artifact judged from its index copy, the base-name collision
// materializeIndexFile closes, and the decode-confirmed store match that keeps
// an unrelated repository file named lock-store.json from being a refusal.
//
// These rows live beside TestStaged_AgreesWithValidateOnAMatrixOfTamperedTrees
// rather than inside it because they need fixtures the matrix's parityFixture
// cannot provide: a build order needs a FULLY locked module with no open
// thread (buildorder.Propose's completeness gates), while the parity fixture
// holds a commented draft on purpose.
package check_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// orderedClaimIn is orderedClaim for a module other than widget.
func orderedClaimIn(module, id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: " + module + "\nstatus: locked\nlayout: card\n" +
		"build_role: behavior\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
}

// committedBuildOrderFixture is a project whose one module is fully locked,
// with its build order locked and recorded, everything committed.
func committedBuildOrderFixture(t *testing.T, cfgBody string, files map[string]string, modules ...string) *config.Config {
	t.Helper()
	cfg, claims := project(t, cfgBody, files)
	for _, m := range modules {
		lockBuildOrder(t, cfg, claims, m)
	}
	gitRepo(t, cfg.Dir())
	git(t, cfg.Dir(), "add", "-A")
	git(t, cfg.Dir(), "commit", "-qm", "fixture")
	if rules := validateRules(t, cfg); len(rules) != 0 {
		t.Fatalf("fixture precondition: the honest project must be silent under --validate, got %v", rules)
	}
	return cfg
}

// tamperArtifact hand-edits module's build-order artifact on disk.
func tamperArtifact(t *testing.T, cfg *config.Config, module string) {
	t.Helper()
	path := cfg.BuildOrderPath(module)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse artifact: %v", err)
	}
	doc["excluded"] = []string{module + ".contract.smuggled"}
	edited, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal edited artifact: %v", err)
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("write edited artifact: %v", err)
	}
}

// A commit that still carries a legacy root file refuses with layout_legacy —
// the same refusal every other verb gives on the working-tree form — even when
// the working tree itself has already been migrated.
func TestStaged_LegacyRootFilesInTheIndexRefuseWithLayoutLegacy(t *testing.T) {
	cfg := stagedFixture(t)
	root := cfg.Dir()
	// The commit carries the ledger at the ROOT: copy the real store there and
	// stage that copy alongside the config, then move the worktree on.
	store, err := os.ReadFile(cfg.LockStorePath())
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dossierx-lock-store.json"), store, 0o644); err != nil {
		t.Fatalf("write legacy copy: %v", err)
	}
	git(t, root, "add", ".dossierx-lock-store.json")
	git(t, root, "commit", "-qm", "the ledger at the root")
	if err := os.Remove(filepath.Join(root, ".dossierx-lock-store.json")); err != nil {
		t.Fatalf("remove worktree copy: %v", err)
	}

	_, err = check.Staged(cfg)
	if err == nil {
		t.Fatalf("an index holding .dossierx-lock-store.json must refuse, got a verdict")
	}
	if errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("this must not be the exit-0 escape hatch: %v", err)
	}
	ce := cliout.As(err)
	if ce == nil || ce.Code != cliout.CodeLayoutLegacy {
		t.Fatalf("expected error.code layout_legacy, got %v", err)
	}
	if !strings.Contains(ce.Message, "git mv .dossierx-lock-store.json build/ledger/lock-store.json") {
		t.Fatalf("the refusal must print the git mv line for the tracked file, got:\n%s", ce.Message)
	}
}

// A locked build order under build/build-order/ is judged from its INDEX copy
// by --staged and from disk by --validate, and the two agree: honest is
// silent, a staged tamper is build-order-content-drift in both. A tamper left
// in the worktree only is refused by --validate (the worktree gate) and is,
// by design, invisible to --staged, which reads the commit — the direction
// TestStaged_VerdictFollowsTheIndexNotTheWorktree pins for claims.
func TestStaged_BuildOrderUnderBuildDirIsJudgedByBothModes(t *testing.T) {
	files := map[string]string{"claims/a.yaml": orderedClaim("widget.contract.a")}

	t.Run("honest", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, baseConfig, files, "widget")
		if _, err := os.Stat(filepath.Join(cfg.Dir(), "build", "build-order", "widget.json")); err != nil {
			t.Fatalf("the artifact must sit under build/build-order/: %v", err)
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped || len(got) != 0 {
			t.Fatalf("--staged on the honest tree: skipped=%v rules=%v", skipped, got)
		}
	})

	t.Run("staged tamper", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, baseConfig, files, "widget")
		tamperArtifact(t, cfg, "widget")
		git(t, cfg.Dir(), "add", "-A")
		want := validateRules(t, cfg)
		if !hasName(want, check.RuleBuildOrderContentDrift) {
			t.Fatalf("control precondition: --validate must refuse the tamper as %s, got %v", check.RuleBuildOrderContentDrift, want)
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped {
			t.Fatalf("--staged took the escape hatch on a tree --validate refuses with %v", want)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("the two modes disagree:\n--staged:   %v\n--validate: %v", got, want)
		}
	})

	t.Run("worktree-only tamper", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, baseConfig, files, "widget")
		tamperArtifact(t, cfg, "widget")
		if !hasName(validateRules(t, cfg), check.RuleBuildOrderContentDrift) {
			t.Fatalf("the worktree gate must refuse a hand-edited artifact under build/build-order/")
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped || len(got) != 0 {
			t.Fatalf("--staged judges the commit, whose artifact is untouched: skipped=%v rules=%v", skipped, got)
		}
	})
}

// A module literally named "lock-store" has a build-order artifact whose BASE
// NAME is the ledger's (build/build-order/lock-store.json beside
// build/ledger/lock-store.json). Both must survive materialisation into the
// temp directory and both must be judged: a tampered artifact is
// build-order-content-drift and a deleted ledger is lock-ledger-absent, in
// --staged as in --validate. A base-name copy would overwrite one with the
// other and one of the two rows would go quiet.
func TestStaged_ModuleNamedLockStoreSurvivesMaterialisation(t *testing.T) {
	cfgBody := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - lock-store\nclaims_dir: claims\n"
	files := map[string]string{
		"claims/a.yaml": orderedClaim("widget.contract.a"),
		"claims/b.yaml": orderedClaimIn("lock-store", "lock-store.contract.b"),
	}

	t.Run("honest", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, cfgBody, files, "widget", "lock-store")
		if filepath.Base(cfg.BuildOrderPath("lock-store")) != filepath.Base(cfg.LockStorePath()) {
			t.Fatalf("fixture precondition: the two base names must collide, got %q and %q", cfg.BuildOrderPath("lock-store"), cfg.LockStorePath())
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped || len(got) != 0 {
			t.Fatalf("--staged on the honest tree: skipped=%v rules=%v", skipped, got)
		}
	})

	t.Run("the artifact tampered", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, cfgBody, files, "widget", "lock-store")
		tamperArtifact(t, cfg, "lock-store")
		git(t, cfg.Dir(), "add", "-A")
		want := validateRules(t, cfg)
		if !hasName(want, check.RuleBuildOrderContentDrift) {
			t.Fatalf("control precondition: --validate must refuse as %s, got %v", check.RuleBuildOrderContentDrift, want)
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("the artifact named like the ledger was not judged from the index:\n--staged:   %v (skipped=%v)\n--validate: %v", got, skipped, want)
		}
	})

	t.Run("the ledger deleted", func(t *testing.T) {
		cfg := committedBuildOrderFixture(t, cfgBody, files, "widget", "lock-store")
		git(t, cfg.Dir(), "rm", "-q", config.LockStoreDisplayPath)
		want := validateRules(t, cfg)
		if !hasName(want, lock.RuleLockLedgerAbsent) {
			t.Fatalf("control precondition: --validate must refuse as %s, got %v", lock.RuleLockLedgerAbsent, want)
		}
		got, skipped := stagedRulesOrSkipped(t, cfg)
		if skipped || !hasName(got, lock.RuleLockLedgerAbsent) {
			t.Fatalf("the ledger must be judged absent from the index, not read from the artifact that shares its name: got %v (skipped=%v)", got, skipped)
		}
	})
}

// An untracked config whose build_dir points somewhere else does not decide
// where --staged looks: the index holds a ledger that decodes strictly as one,
// so the run is refused with ErrUntrackedConfig, never judged against the
// worktree's redirection and never the exit-0 escape hatch.
func TestStaged_UntrackedConfigWithAnotherBuildDirStillRefusesOverAStagedLedger(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	root := cfg.Dir()
	gitRepo(t, root)
	git(t, root, "add", "claims", config.LockStoreDisplayPath)
	git(t, root, "commit", "-qm", "ledger and claims, no config")

	redirected := baseConfig + "build_dir: elsewhere\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(redirected), 0o644); err != nil {
		t.Fatalf("write redirected config: %v", err)
	}
	cfg2, err := config.LoadConfig(filepath.Join(root, "project.config.yaml"))
	if err != nil {
		t.Fatalf("load redirected config: %v", err)
	}
	_, err = check.Staged(cfg2)
	if err == nil {
		t.Fatalf("an untracked config must not be used to judge a tracked ledger")
	}
	if errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("this must not be the exit-0 escape hatch: %v", err)
	}
	if !errors.Is(err, check.ErrUntrackedConfig) {
		t.Fatalf("expected ErrUntrackedConfig, got %v", err)
	}
	if !strings.Contains(err.Error(), config.LockStoreDisplayPath) {
		t.Fatalf("the refusal must name the staged store it found, got %v", err)
	}
}

// A staged LEGACY-named ledger with no tracked config is a refusal too — a
// half-migrated commit is never "nothing to evaluate".
func TestStaged_StagedLegacyLedgerWithNoTrackedConfigRefuses(t *testing.T) {
	cfg, _ := project(t, baseConfig, map[string]string{
		"claims/locked.yaml": lockedClaim("widget.contract.locked"),
	})
	root := cfg.Dir()
	store, err := os.ReadFile(cfg.LockStorePath())
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".dossierx-lock-store.json"), store, 0o644); err != nil {
		t.Fatalf("write legacy copy: %v", err)
	}
	gitRepo(t, root)
	git(t, root, "add", ".dossierx-lock-store.json")

	_, err = check.Staged(cfg)
	if err == nil {
		t.Fatalf("a staged legacy ledger with no tracked config must refuse")
	}
	if errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("this must not be the exit-0 escape hatch: %v", err)
	}
	if ce := cliout.As(err); (ce == nil || ce.Code != cliout.CodeLayoutLegacy) && !errors.Is(err, check.ErrUntrackedConfig) {
		t.Fatalf("expected layout_legacy or ErrUntrackedConfig, got %v", err)
	}
}

// A sibling directory in the same repository holding its own viewer/index.html
// is not this project's legacy layout: the index listing is scoped to the
// project's own subtree.
func TestStaged_SiblingProjectsViewerDoesNotRefuseThisProject(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	for _, d := range []string{filepath.Join(docs, "claims"), filepath.Join(root, "other", "viewer")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFixtureFile(t, filepath.Join(docs, config.FileName), baseConfig)
	writeFixtureFile(t, filepath.Join(docs, "claims", "locked.yaml"), lockedClaim("widget.contract.locked"))
	writeFixtureFile(t, filepath.Join(root, "other", "viewer", "index.html"), "<html>a sibling project's viewer</html>")
	cfg, err := config.LoadConfig(filepath.Join(docs, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	armLedger(t, cfg, loadFixtureClaims(t, cfg))
	gitRepo(t, root)
	git(t, root, "add", "-A")
	git(t, root, "commit", "-qm", "two projects")

	got, skipped := stagedRulesOrSkipped(t, cfg)
	if skipped || len(got) != 0 {
		t.Fatalf("the sibling's viewer must not touch this project's verdict: skipped=%v rules=%v", skipped, got)
	}
}

// A tracked lock-store.json that does NOT decode strictly as the engine's
// ledger is not judgeable content: with no tracked config, --staged takes the
// same path an index with no dossierx content takes, rather than refusing with
// ErrUntrackedConfig naming an unrelated file. The positive is beside it: the
// same name holding a real store IS judgeable content.
func TestStaged_UnrelatedLockStoreJSONIsNotJudgeableContent(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"empty object", "{}"},
		{"unrelated json", `{"unrelated": true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := project(t, baseConfig, map[string]string{
				"claims/one.yaml": draftClaim("widget.contract.one"),
			})
			root := cfg.Dir()
			gitRepo(t, root)
			writeFixtureFile(t, filepath.Join(root, "lock-store.json"), tc.body)
			git(t, root, "add", "lock-store.json")
			git(t, root, "commit", "-qm", "an unrelated file")

			sp, err := check.Staged(cfg)
			if err != nil {
				t.Fatalf("an unrelated lock-store.json is not dossierx content; expected the empty-index path, got %v", err)
			}
			if sp.ConfigFromIndex || len(sp.Claims) != 0 {
				t.Fatalf("nothing dossierx is staged, so nothing is judged; got config-from-index=%v claims=%d", sp.ConfigFromIndex, len(sp.Claims))
			}
		})
	}

	t.Run("a real store at the same name is judgeable", func(t *testing.T) {
		cfg, _ := project(t, baseConfig, map[string]string{
			"claims/locked.yaml": lockedClaim("widget.contract.locked"),
		})
		root := cfg.Dir()
		store, err := os.ReadFile(cfg.LockStorePath())
		if err != nil {
			t.Fatalf("read store: %v", err)
		}
		gitRepo(t, root)
		if err := os.MkdirAll(filepath.Join(root, "archive"), 0o755); err != nil {
			t.Fatalf("mkdir archive: %v", err)
		}
		writeFixtureFile(t, filepath.Join(root, "archive", "lock-store.json"), string(store))
		git(t, root, "add", "archive/lock-store.json")
		git(t, root, "commit", "-qm", "a real store, elsewhere")

		_, err = check.Staged(cfg)
		if !errors.Is(err, check.ErrUntrackedConfig) {
			t.Fatalf("a strictly-decoding store is dossierx content wherever it sits; expected ErrUntrackedConfig, got %v", err)
		}
	})
}
