// staged_project_claims_test.go pins that `check --staged` reads the
// PROJECT-CLAIMS STORE (config project_claims_dir, default project-claims/)
// out of the git index exactly as it reads claims_dir — and that on a project
// which uses project claims the two enforcing modes, --staged and --validate,
// reach the same verdict on the same bytes.
//
// The defect this file was written for: Staged enumerated claims from the
// claims_dir pathspec alone, so a module claim resting on project.<slug> was
// `dangling` from the hook and every locked project claim's approval was
// `lock-ledger-abandoned`, on a tree plain check accepted. A project that
// adopted project claims could not commit through the pre-commit hook at all.
// The register of properties staged.go defends for claims_dir — index only,
// never the worktree; the config from the index, so an unstaged decoy line
// cannot repoint the store; a store absent from the index is an empty store;
// a store outside the work tree is one no commit can carry — must hold for
// the second store too, and each is asserted below.
package check_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// projectClaimsConfig is baseConfig plus an explicit project_claims_dir, so a
// fixture can repoint the store without a substitution.
func projectClaimsConfig(dir string) string {
	return baseConfig + "project_claims_dir: " + dir + "\n"
}

// lockedProjectClaim is a locked scope: project claim resting on nothing.
func lockedProjectClaim(id string) string {
	return "id: " + id + "\nscope: project\nstatus: locked\nlayout: card\n" +
		"body: |\n  a locked project claim.\n" +
		"rests_on:\n  none: true\n  reason: fixture\n"
}

// draftProjectClaimOn is a draft scope: project claim resting on target.
func draftProjectClaimOn(id, target string) string {
	return "id: " + id + "\nscope: project\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft project claim.\n" +
		"rests_on:\n  - " + target + "\n"
}

// lockedClaimOn is lockedClaim resting on target instead of on nothing.
func lockedClaimOn(id, target string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"rests_on:\n  - " + target + "\n"
}

// draftClaimOn is draftClaim resting on target instead of on nothing.
func draftClaimOn(id, target string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"rests_on:\n  - " + target + "\n"
}

// writeProjectFiles writes cfgBody as project.config.yaml under root plus every
// file in files, loads the config, and arms the roof and the ledger for every
// locked claim in BOTH stores — the fixture equivalent of "a human approved
// these", said for the project store as well as the module store. It does NOT
// touch git; callers decide what is staged.
func writeProjectFiles(t *testing.T, root, cfgBody string, files map[string]string) *config.Config {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", root, err)
	}
	writeFixtureFile(t, filepath.Join(root, config.FileName), cfgBody)
	for rel, body := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		writeFixtureFile(t, abs, body)
	}
	cfg, err := config.LoadConfig(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	armConstitution(t, cfg)
	armLedger(t, cfg, loadAllFixtureClaims(t, cfg))
	return cfg
}

// loadAllFixtureClaims is loader.LoadAll — BOTH stores, as plain check loads
// them — with the test's error handling.
func loadAllFixtureClaims(t *testing.T, cfg *config.Config) []model.Claim {
	t.Helper()
	claims, err := loader.LoadAll(cfg)
	if err != nil {
		t.Fatalf("load all claims: %v", err)
	}
	return claims
}

// projectClaimsParityFixture is a committed, fully-armed project that USES
// project claims in every shape the store allows: a locked project claim
// (project.scope), a locked module claim resting on it
// (widget.contract.overview -> project.scope), and a locked project claim
// resting on that module *.contract.* claim (project.retention ->
// widget.contract.overview). Every approval is on the ledger, the roof is
// locked, and the repository has a sibling directory outside the work tree
// for the out-of-tree repoint.
func projectClaimsParityFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(root, "outside-project-claims"), 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
		"claims/overview.yaml":          lockedClaimOn("widget.contract.overview", "project.scope"),
		"project-claims/scope.yaml":     lockedProjectClaim("project.scope"),
		"project-claims/retention.yaml": lockedProjectClaimOn("project.retention", "widget.contract.overview"),
		"archive/NOTES.md":              "an ordinary tracked directory with no claims in it\n",
		// A tracked non-claim file keeps claims_dir in existence when the
		// matrix deletes its only claim: loader.LoadClaims treats a missing
		// claims_dir as a hard error, and this file is what lets the
		// "module claim deleted" row measure the gate rather than the load.
		"claims/README.md": "module claims live here\n",
	})
	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	if got := worktreeVerdict(t, cfg); len(got) != 0 {
		t.Fatalf("fixture precondition: the honest project must be silent under --validate, got %v", got)
	}
	return cfg
}

// lockedProjectClaimOn is lockedProjectClaim resting on target.
func lockedProjectClaimOn(id, target string) string {
	return "id: " + id + "\nscope: project\nstatus: locked\nlayout: card\n" +
		"body: |\n  a locked project claim.\n" +
		"rests_on:\n  - " + target + "\n"
}

// verdictOf flattens EVERYTHING the two enforcing callers read — every lint
// finding (name, claim, severity, message) and every ledger finding (rule,
// claim, message) — into one list in report order, one "|"-separated line
// per finding, so the comparison is over the whole verdict and not over the
// ledger rules alone. `dangling` is a lint finding, and it was the one the
// hook got wrong. The separator lets an assertion name a finding by its
// kind, rule and claim ("ledger|lock-content-drift|project.scope|") without
// matching a claim id that merely appears in another finding's prose.
func verdictOf(res check.Result) []string {
	var out []string
	for _, f := range res.LintFindings {
		out = append(out, fmt.Sprintf("lint|%s|%s|%s|%s", f.LintName, f.ClaimID, f.Severity, f.Message))
	}
	for _, f := range res.LedgerFindings {
		out = append(out, fmt.Sprintf("ledger|%s|%s|%s", f.Rule, f.ClaimID, f.Message))
	}
	return out
}

// worktreeVerdict is what `check` and `check --validate` report on cfg's
// working tree: both stores through loader.LoadAll.
func worktreeVerdict(t *testing.T, cfg *config.Config) []string {
	t.Helper()
	return verdictOf(check.Status(loadAllFixtureClaims(t, cfg), cfg))
}

// stagedVerdict is what `check --staged` reports on cfg's index.
func stagedVerdict(t *testing.T, cfg *config.Config) ([]string, check.StagedProject) {
	t.Helper()
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	return verdictOf(check.StatusStaged(sp, cfg)), sp
}

// indexAsWorktree materialises the INDEX of cfg's repository into a fresh
// directory (git checkout-index) and loads the config from there, so plain
// check can be run over exactly the tree the commit would contain. It is how
// a "what would plain check say on the same tree" assertion is made honest
// when the index and the working tree deliberately differ.
func indexAsWorktree(t *testing.T, cfg *config.Config) *config.Config {
	t.Helper()
	dir := t.TempDir()
	git(t, cfg.Dir(), "checkout-index", "-a", "-f", "--prefix="+dir+string(filepath.Separator))
	rel, err := filepath.Rel(cfg.Dir(), configSourcePath(cfg))
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	loaded, err := config.LoadConfig(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("load the index's config: %v", err)
	}
	return loaded
}

func configSourcePath(cfg *config.Config) string {
	if p := cfg.Path(); p != "" {
		return p
	}
	return filepath.Join(cfg.Dir(), config.FileName)
}

func sameVerdict(t *testing.T, what string, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%s: the two modes disagree on one tree:\n--staged:\n  %s\n--validate:\n  %s",
			what, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func hasVerdict(v []string, substr string) bool {
	for _, line := range v {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}

// verdictAbout reports whether any finding is ABOUT id — carries it as its
// claim id — as opposed to naming it inside another claim's message.
func verdictAbout(v []string, id string) bool {
	return hasVerdict(v, "|"+id+"|")
}

// THE DEFECT, in the smallest tree that shows it: the honest, fully-locked
// project-claims fixture, committed, must be as silent under --staged as it is
// under --validate. Before the fix --staged reported `dangling` on the module
// claim and `lock-ledger-abandoned` on both project claims.
func TestStaged_ProjectClaimsAreReadFromTheIndex(t *testing.T) {
	cfg := projectClaimsParityFixture(t)
	got, sp := stagedVerdict(t, cfg)
	if len(got) != 0 {
		t.Fatalf("--staged refuses the honest project-claims fixture that --validate accepts:\n  %s", strings.Join(got, "\n  "))
	}
	ids := make([]string, 0, len(sp.Claims))
	for _, c := range sp.Claims {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "widget.contract.overview,project.retention,project.scope" {
		t.Fatalf("the registry must hold both stores merged and sorted by path, got %v", ids)
	}
	for _, c := range sp.Claims {
		if !strings.HasPrefix(c.ID, "project.") {
			continue
		}
		if filepath.Base(filepath.Dir(c.SourcePath)) != "project-claims" {
			t.Fatalf("a project claim's SourcePath must be its WORKING-TREE path under project_claims_dir, got %s", c.SourcePath)
		}
	}
}

// THE MATRIX, for the project store: one committed fixture, a table of
// tampers, and for each the same demand made of claims_dir in
// staged_parity_test.go — --staged and --validate must report the same
// findings, lint and ledger both, and --staged must never take the exit-0
// escape hatch on a tree --validate refuses.
func TestStaged_AgreesWithValidateOnAProjectWithProjectClaims(t *testing.T) {
	cases := []struct {
		name   string
		tamper func(t *testing.T, cfg *config.Config) *config.Config
		// want is a substring every row's verdict must carry (both modes), or
		// "" for a tree both must accept.
		want string
	}{
		{
			name:   "the honest tree",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config { return cfg },
		},
		{
			name: "a locked project claim's body rewritten",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				rewrite(t, filepath.Join(cfg.ProjectClaimsDirPath(), "scope.yaml"), "a locked project claim.", "a locked project claim, quietly rewritten.")
				return cfg
			},
			want: "ledger|" + lock.RuleLockContentDrift + "|project.scope|",
		},
		{
			name: "a locked project claim deleted, its approval left standing",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", "project-claims/scope.yaml")
				return cfg
			},
			want: "ledger|" + lock.RuleLockLedgerAbandoned + "|project.scope|",
		},
		{
			name: "the module claim resting on a project claim deleted, its approval left standing",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", "claims/overview.yaml")
				return cfg
			},
			want: "ledger|" + lock.RuleLockLedgerAbandoned + "|widget.contract.overview|",
		},
		{
			name: "project_claims_dir repointed at an empty directory INSIDE the work tree",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				return repointProjectClaimsDir(t, cfg, "archive")
			},
			want: "ledger|" + lock.RuleLockLedgerAbandoned + "|project.scope|",
		},
		{
			name: "project_claims_dir repointed OUTSIDE the work tree, ledger left standing",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				return repointProjectClaimsDir(t, cfg, "../outside-project-claims")
			},
			want: "ledger|" + lock.RuleLockLedgerAbandoned + "|project.scope|",
		},
		{
			name: "a duplicate id across the two stores",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				writeFixtureFile(t, filepath.Join(cfg.ProjectClaimsDirPath(), "scope-copy.yaml"), lockedProjectClaim("project.scope"))
				return cfg
			},
			want: "lint|ambiguous|project.scope|",
		},
		{
			name: "a draft project claim added beside the locked ones",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				writeFixtureFile(t, filepath.Join(cfg.ProjectClaimsDirPath(), "extra.yaml"), draftProjectClaimOn("project.extra", "project.scope"))
				return cfg
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := projectClaimsParityFixture(t)
			tampered := tc.tamper(t, cfg)
			git(t, cfg.Dir(), "add", "-A")

			want := worktreeVerdict(t, tampered)
			if tc.want == "" {
				if len(want) != 0 {
					t.Fatalf("control precondition: --validate must accept this tree, got %v", want)
				}
			} else if !hasVerdict(want, tc.want) {
				t.Fatalf("control precondition: --validate must refuse this tree with %q, got %v", tc.want, want)
			}

			got, _ := stagedVerdict(t, tampered)
			sameVerdict(t, tc.name, got, want)
		})
	}
}

// rewrite substitutes old with new in the file at path, failing if nothing
// changed.
func rewrite(t *testing.T, path, old, new string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	edited := strings.Replace(string(raw), old, new, 1)
	if edited == string(raw) {
		t.Fatalf("fixture precondition: the substitution %q did not apply to %s", old, path)
	}
	writeFixtureFile(t, path, edited)
}

// repointProjectClaimsDir rewrites project_claims_dir in the working tree's
// project.config.yaml and returns the config as it now reads.
func repointProjectClaimsDir(t *testing.T, cfg *config.Config, dir string) *config.Config {
	t.Helper()
	path := filepath.Join(cfg.Dir(), config.FileName)
	writeFixtureFile(t, path, projectClaimsConfig(dir))
	reloaded, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("reload repointed config: %v", err)
	}
	return reloaded
}

// A PROJECT WITHOUT A PROJECT-CLAIMS STORE — the common case — is an EMPTY
// store, not an error, exactly as loader.LoadProjectClaims treats a directory
// that does not exist. And a store that exists on disk but is NOT IN THE INDEX
// is equally empty to the gate: the commit does not carry it.
func TestStaged_ProjectClaimsStoreAbsentFromTheIndexIsAnEmptyStore(t *testing.T) {
	cfg := stagedFixture(t)
	if _, err := os.Stat(cfg.ProjectClaimsDirPath()); !os.IsNotExist(err) {
		t.Fatalf("fixture precondition: no project-claims store on disk, stat err=%v", err)
	}
	want := worktreeVerdict(t, cfg)
	got, sp := stagedVerdict(t, cfg)
	sameVerdict(t, "no project-claims store", got, want)
	if len(sp.Claims) != 1 {
		t.Fatalf("a project with no project-claims store must be judged on its module claims alone: %d claim(s)", len(sp.Claims))
	}

	// Now the store appears on disk, untracked. The index is unchanged, so
	// the verdict must be too — and the registry must not grow.
	if err := os.MkdirAll(cfg.ProjectClaimsDirPath(), 0o755); err != nil {
		t.Fatalf("mkdir project-claims: %v", err)
	}
	writeFixtureFile(t, filepath.Join(cfg.ProjectClaimsDirPath(), "scope.yaml"), lockedProjectClaim("project.scope"))
	got, sp = stagedVerdict(t, cfg)
	sameVerdict(t, "untracked project-claims store", got, want)
	if len(sp.Claims) != 1 {
		t.Fatalf("an UNTRACKED project-claims store is not part of the commit and must not be judged: %d claim(s)", len(sp.Claims))
	}
}

// PARTIAL STAGING. A project claim and the module claim resting on it are two
// files, and an author can stage one without the other. The gate judges the
// index, so the verdict is what plain check would give ON THE TREE THE INDEX
// HOLDS — measured here by checking out the index into a fresh directory and
// running plain check there — not what the working tree happens to show.
func TestStaged_PartiallyStagedProjectClaimsAreJudgedAsTheIndexTree(t *testing.T) {
	t.Run("the project claim is not staged, its module dependent is", func(t *testing.T) {
		cfg := writeProjectFiles(t, filepath.Join(t.TempDir(), "repo"), baseConfig, map[string]string{
			"claims/overview.yaml":      draftClaimOn("widget.contract.overview", "project.scope"),
			"project-claims/scope.yaml": draftProjectClaimOn("project.scope", "widget.contract.overview"),
		})
		gitRepo(t, cfg.Dir())
		git(t, cfg.Dir(), "add", config.FileName, "constitution.yaml", "build", "claims")

		got, _ := stagedVerdict(t, cfg)
		if !hasVerdict(got, "lint|dangling|widget.contract.overview|") {
			t.Fatalf("the index holds the module claim without the project claim it rests on; --staged must report dangling on it, got %v", got)
		}
		if verdictAbout(got, "project.scope") {
			t.Fatalf("an unstaged project claim is not in the commit and must not be judged, got %v", got)
		}
		sameVerdict(t, "project claim unstaged", got, worktreeVerdict(t, indexAsWorktree(t, cfg)))
	})

	t.Run("the module claim is not staged, the project claim resting on it is", func(t *testing.T) {
		cfg := writeProjectFiles(t, filepath.Join(t.TempDir(), "repo"), baseConfig, map[string]string{
			"claims/anchor.yaml":            draftClaim("widget.contract.anchor"),
			"claims/overview.yaml":          draftClaimOn("widget.contract.overview", "project.scope"),
			"project-claims/scope.yaml":     lockedProjectClaim("project.scope"),
			"project-claims/retention.yaml": draftProjectClaimOn("project.retention", "widget.contract.overview"),
		})
		gitRepo(t, cfg.Dir())
		git(t, cfg.Dir(), "add", config.FileName, "constitution.yaml", "build", "project-claims", "claims/anchor.yaml")

		got, _ := stagedVerdict(t, cfg)
		if !hasVerdict(got, "lint|dangling|project.retention|") {
			t.Fatalf("the index holds the project claim without the module claim it rests on; --staged must report dangling on it, got %v", got)
		}
		if verdictAbout(got, "widget.contract.overview") {
			t.Fatalf("an unstaged module claim is not in the commit and must not be judged, got %v", got)
		}
		sameVerdict(t, "module claim unstaged", got, worktreeVerdict(t, indexAsWorktree(t, cfg)))
	})
}

// A TAMPERED LOCKED PROJECT CLAIM IN THE INDEX IS REFUSED, the way a tampered
// module claim is: the verdict follows the index and not the working tree in
// both directions — a clean index under a tampered worktree passes, a
// tampered index under a restored worktree is refused.
func TestStaged_TamperedLockedProjectClaimInTheIndexIsRefused(t *testing.T) {
	cfg := projectClaimsParityFixture(t)
	claimFile := filepath.Join(cfg.ProjectClaimsDirPath(), "scope.yaml")
	original, err := os.ReadFile(claimFile)
	if err != nil {
		t.Fatalf("read fixture claim: %v", err)
	}

	// Worktree tampered, index clean -> the commit is fine, and the body the
	// gate linted is the index's.
	rewrite(t, claimFile, "a locked project claim.", "a locked project claim, quietly rewritten.")
	got, sp := stagedVerdict(t, cfg)
	if len(got) != 0 {
		t.Fatalf("a tampered WORKTREE with a clean index must pass: got %v", got)
	}
	if strings.Join(sp.FromIndex, ",") != "project-claims/scope.yaml" {
		t.Fatalf("FromIndex must name the project claim whose index content differs from the worktree, got %v", sp.FromIndex)
	}
	for _, c := range sp.Claims {
		if c.ID == "project.scope" && !strings.Contains(c.Body, "a locked project claim.\n") {
			t.Fatalf("expected the index's body, got %q", c.Body)
		}
	}

	// Stage the tamper and restore the worktree -> the commit is refused,
	// and nothing on disk says why.
	git(t, cfg.Dir(), "add", "project-claims/scope.yaml")
	writeFixtureFile(t, claimFile, string(original))
	got, _ = stagedVerdict(t, cfg)
	if !hasVerdict(got, "ledger|"+lock.RuleLockContentDrift+"|project.scope|") {
		t.Fatalf("staging the tampered project-claim blob must be refused as lock-content-drift: got %v", got)
	}
}

// THE DECOY, for the second store. project_claims_dir is read from the config
// THE INDEX HOLDS, so an unstaged edit pointing it at an empty decoy directory
// changes nothing: the gate still audits project-claims/ and still refuses the
// tampered locked project claim the commit carries — the same trap
// TestStaged_ConfigComesFromTheIndex closes for claims_dir.
func TestStaged_ProjectClaimsDirComesFromTheIndex(t *testing.T) {
	cfg := projectClaimsParityFixture(t)
	rewrite(t, filepath.Join(cfg.ProjectClaimsDirPath(), "scope.yaml"), "a locked project claim.", "a locked project claim, quietly rewritten.")
	git(t, cfg.Dir(), "add", "project-claims/scope.yaml")

	// The unstaged redirect. The decoy must exist, or the redirect would fail
	// for an uninteresting reason rather than being followed.
	if err := os.MkdirAll(filepath.Join(cfg.Dir(), "decoy"), 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	decoyed := repointProjectClaimsDir(t, cfg, "decoy")
	if filepath.Base(decoyed.ProjectClaimsDirPath()) != "decoy" {
		t.Fatalf("fixture precondition: the worktree config must now say project_claims_dir: decoy")
	}

	sp, err := check.Staged(decoyed)
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	if !sp.ConfigFromIndex {
		t.Fatalf("the config must have come from the index")
	}
	if got := filepath.Base(sp.Config.ProjectClaimsDirPath()); got != "project-claims" {
		t.Fatalf("the index still says project_claims_dir: project-claims; the gate resolved %q", got)
	}
	if len(sp.Claims) != 3 {
		t.Fatalf("the unstaged redirect emptied the project store: %d claim(s)", len(sp.Claims))
	}
	if !hasRule(check.StatusStaged(sp, decoyed).LedgerFindings, lock.RuleLockContentDrift) {
		t.Fatalf("an unstaged project_claims_dir edit disarmed the gate")
	}
}

// A project_claims_dir INSIDE claims_dir is refused by config.DecodeConfig on
// every path, and the staged path reaches that refusal through the STAGED
// config — a hard error, never a fallback to the worktree copy and never a
// silent empty store.
func TestStaged_ProjectClaimsDirInsideClaimsDirIsRefusedFromTheIndex(t *testing.T) {
	cfg := projectClaimsParityFixture(t)
	writeFixtureFile(t, filepath.Join(cfg.Dir(), config.FileName), projectClaimsConfig("claims/project"))
	git(t, cfg.Dir(), "add", config.FileName)
	// The worktree config is restored to the honest one, so the only copy
	// that says claims/project is the staged one.
	writeFixtureFile(t, filepath.Join(cfg.Dir(), config.FileName), baseConfig)

	_, err := check.Staged(cfg)
	if err == nil || !strings.Contains(err.Error(), "project_claims_dir") {
		t.Fatalf("a staged config placing project_claims_dir inside claims_dir must fail to load with the config's own containment error, got %v", err)
	}
}
