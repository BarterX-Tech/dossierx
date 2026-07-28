// staged_parity_test.go pins the one property the two enforcing entry points
// have to have: `check --staged` (the pre-commit hook) and `check --validate`
// (CI, and the loop command) must answer the SAME question the same way when
// they are looking at the same bytes.
//
// The asymmetry that matters is one-directional. A --staged run that REFUSES
// where --validate passes is an annoyance an author will notice immediately; a
// --staged run that reports CLEAN where --validate refuses is a hole, and it is
// the worst-shaped hole available, because --staged is the mode that runs at the
// keyboard, in the hook, on the commit that carries the tamper. Whichever of the
// two is laxer is the one an edit travels through.
//
// The defect this file was written for: a claims_dir repointed OUTSIDE the git
// work tree while the lock ledger stayed put took `Staged` down the ErrNoIndex
// escape hatch — "there is nothing here to evaluate", warn, exit 0 — before the
// ledger inputs were ever assembled, so the single-tree ledger rules never ran.
// The identical tree under --validate reported lock-ledger-abandoned. That
// escape hatch is for a run that has NO index content to judge; a standing
// ledger whose records name claims no commit can carry is not that, and it needs
// no history at all to see.
package check_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// parityFixture is a committed, fully-armed project — one LOCKED claim with its
// approval on the ledger, one DRAFT claim carrying a human's open thread with
// its digest recorded — inside a git repository that has a SIBLING directory
// outside the work tree entirely.
//
// The sibling is the point. Everything else in this package's fixtures can only
// repoint claims_dir at another directory inside the repository; the escape
// hatch under test is reached only by a claims_dir that git cannot resolve at
// all, so the fixture has to own a directory git has never heard of.
func parityFixture(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	for _, d := range []string{filepath.Join(repo, "claims"), filepath.Join(root, "outside-claims")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFixtureFile(t, filepath.Join(repo, config.FileName), baseConfig)
	writeFixtureFile(t, filepath.Join(repo, "claims", "locked.yaml"), lockedClaim("widget.contract.locked"))
	writeFixtureFile(t, filepath.Join(repo, "claims", "draft.yaml"), commentedDraftClaim("widget.contract.draft"))

	cfg, err := config.LoadConfig(filepath.Join(repo, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	claims := loadFixtureClaims(t, cfg)
	armLedger(t, cfg, claims)
	armDigests(t, cfg, claims)

	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	// A fixture that is not clean to begin with cannot measure a disagreement.
	if rules := validateRules(t, cfg); len(rules) != 0 {
		t.Fatalf("fixture precondition: the honest project must be silent under --validate, got %v", rules)
	}
	return cfg
}

func writeFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// validateRules is the WORKTREE gate's verdict — what `check` and `check
// --validate` report, and therefore what CI reports.
func validateRules(t *testing.T, cfg *config.Config) []string {
	t.Helper()
	return rulesOf(check.Status(loadFixtureClaims(t, cfg), cfg).LedgerFindings)
}

// stagedRulesOrSkipped is the INDEX gate's verdict, plus whether it took the
// ErrNoIndex escape hatch — which the CLI reports as skipped:true, ok:true,
// exit 0, and which is therefore indistinguishable from a clean run to anything
// downstream that only reads the exit code.
func stagedRulesOrSkipped(t *testing.T, cfg *config.Config) ([]string, bool) {
	t.Helper()
	sp, err := check.Staged(cfg)
	if errors.Is(err, check.ErrNoIndex) {
		return nil, true
	}
	if err != nil {
		t.Fatalf("Staged: %v", err)
	}
	return rulesOf(check.StatusStaged(sp, cfg).LedgerFindings), false
}

// THE DEFECT, on its own, in the smallest tree that shows it.
//
// claims_dir is repointed outside the work tree and the lock ledger is left
// exactly where it is. The ledger's records now name claims that no commit can
// carry — which is lock-ledger-abandoned, a rule that needs one tree and no
// history whatsoever — and `check --validate` says so. `check --staged` used to
// answer "nothing to evaluate", exit 0, on the same bytes.
func TestStaged_OutOfWorkTreeClaimsDirWithAStandingLedgerIsNotSkipped(t *testing.T) {
	cfg := parityFixture(t)
	repointed := repointClaimsDir(t, cfg, "../outside-claims")
	git(t, cfg.Dir(), "add", "-A")

	want := validateRules(t, repointed)
	if !hasName(want, lock.RuleLockLedgerAbandoned) {
		t.Fatalf("control precondition: --validate must already refuse this tree as lock-ledger-abandoned, got %v", want)
	}

	got, skipped := stagedRulesOrSkipped(t, repointed)
	if skipped {
		t.Fatalf("--staged took the ErrNoIndex escape hatch (exit 0) on a tree --validate refuses as %v: the ledger is in the index and its records name claims no commit can carry, which is not \"nothing to evaluate\"", want)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the two modes disagree on one tree:\n--staged:   %v\n--validate: %v", got, want)
	}
}

// THE MATRIX. One committed fixture, a table of tampers, and for each one the
// same demand: --staged and --validate must report the same rules, and --staged
// must not reach for the exit-0 escape hatch on a tree --validate refuses.
//
// The cases are chosen to walk every early return in Staged that can end a run
// before the ledger gate sees anything — the claims_dir pathspec (both the
// in-tree and the out-of-tree form), a missing store, a missing digest store, a
// missing claim — plus the honest tree, because a matrix whose every row refuses
// would also pass if the gate simply refused everything.
func TestStaged_AgreesWithValidateOnAMatrixOfTamperedTrees(t *testing.T) {
	cases := []struct {
		name string
		// tamper edits the working tree and returns the config both gates are
		// then run with (the same config: after `git add -A` the index and the
		// worktree hold identical bytes, so the two are being asked exactly the
		// same question).
		tamper func(t *testing.T, cfg *config.Config) *config.Config
		// wantRule is the rule the tree must be refused with, or "" when the
		// tree is one both modes must accept.
		wantRule string
	}{
		{
			name:   "the honest tree",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config { return cfg },
		},
		{
			name: "a locked claim's body rewritten",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				path := filepath.Join(cfg.ClaimsDir, "locked.yaml")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read locked claim: %v", err)
				}
				edited := strings.Replace(string(raw), "a locked claim.", "a locked claim, quietly rewritten.", 1)
				if edited == string(raw) {
					t.Fatalf("fixture precondition: the tamper substitution did not apply")
				}
				writeFixtureFile(t, path, edited)
				return cfg
			},
			wantRule: lock.RuleLockContentDrift,
		},
		{
			name: "the lock ledger deleted",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", ".dossierx-lock-store.json")
				return cfg
			},
			wantRule: lock.RuleLockLedgerAbsent,
		},
		{
			name: "the comment digest store deleted",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", digest.StoreFileName)
				return cfg
			},
			wantRule: check.RuleCommentDigestAbsent,
		},
		{
			name: "a locked claim deleted, its approval left standing",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", filepath.Join("claims", "locked.yaml"))
				return cfg
			},
			wantRule: lock.RuleLockLedgerAbandoned,
		},
		{
			name: "claims_dir repointed at an empty directory INSIDE the work tree",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				// Tracked and innocent, so the repoint adds nothing to the diff
				// but an edited line: the registry goes empty and every
				// per-claim rule falls silent.
				if err := os.MkdirAll(filepath.Join(cfg.Dir(), "archive"), 0o755); err != nil {
					t.Fatalf("mkdir archive: %v", err)
				}
				writeFixtureFile(t, filepath.Join(cfg.Dir(), "archive", "NOTES.md"), "no claims here\n")
				return repointClaimsDir(t, cfg, "archive")
			},
			wantRule: lock.RuleLockLedgerAbandoned,
		},
		{
			name: "claims_dir repointed OUTSIDE the work tree, ledger left standing",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				return repointClaimsDir(t, cfg, "../outside-claims")
			},
			wantRule: lock.RuleLockLedgerAbandoned,
		},
		{
			// A SYMLINK under claims_dir (git mode 120000). Both modes must
			// ACCEPT it, and the row is here because --staged did not merely
			// disagree — it ABORTED, with the unclassified `internal` code, so
			// the pre-commit hook refused every commit in the repository while
			// --validate read the link and found an ordinary claim. The link
			// points at a DRAFT, which is the honest version of this layout and
			// the one that keeps the two modes genuinely comparable: drafts are
			// free, so no ledger record covers the file either way. The
			// displacement case — a symlink standing where a LOCKED claim was —
			// is the deliberate disagreement pinned in
			// staged_nonregular_test.go, for the same reason as the
			// claims-outside-the-repository test at the bottom of this file.
			name: "a symlink under claims_dir",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				writeFixtureFile(t, filepath.Join(filepath.Dir(cfg.Dir()), "outside-claims", "extra.yaml"), draftClaim("widget.contract.extra"))
				symlinkOrSkip(t, filepath.Join("..", "..", "outside-claims", "extra.yaml"), filepath.Join(cfg.ClaimsDir, "extra.yaml"))
				return cfg
			},
		},
		{
			// A SUBMODULE GITLINK under claims_dir (git mode 160000). Same
			// shape, other mode: the oid is a commit in ANOTHER repository and
			// is not in this one's object store, so `cat-file --batch` answered
			// "missing" and the run died. The vendored repository holds no
			// claim, so there is nothing here for either mode to judge and both
			// must simply carry on.
			name: "a submodule gitlink under claims_dir",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				nestedRepo(t, filepath.Join(cfg.ClaimsDir, "vendored"), map[string]string{
					"NOTES.md": "another repository entirely\n",
				})
				return cfg
			},
		},
		{
			// The collapsed scope: both halves in one change. It is a
			// known, deliberate boundary case (see staged_no_parent_test.go) and both
			// modes must be equally silent about it — a gap the two modes
			// disagreed about would be a defect on top of the gap.
			name: "claims_dir repointed OUTSIDE the work tree AND the ledger deleted",
			tamper: func(t *testing.T, cfg *config.Config) *config.Config {
				git(t, cfg.Dir(), "rm", "-q", ".dossierx-lock-store.json")
				return repointClaimsDir(t, cfg, "../outside-claims")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := parityFixture(t)
			tampered := tc.tamper(t, cfg)
			git(t, cfg.Dir(), "add", "-A")

			want := validateRules(t, tampered)
			if tc.wantRule == "" {
				if len(want) != 0 {
					t.Fatalf("control precondition: --validate must accept this tree, got %v", want)
				}
			} else if !hasName(want, tc.wantRule) {
				t.Fatalf("control precondition: --validate must refuse this tree as %s, got %v", tc.wantRule, want)
			}

			got, skipped := stagedRulesOrSkipped(t, tampered)
			if skipped {
				if len(want) != 0 {
					t.Fatalf("--staged took the ErrNoIndex escape hatch (exit 0, skipped) on a tree --validate refuses with %v", want)
				}
				return
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("the two modes disagree on one tree:\n--staged:   %v\n--validate: %v", got, want)
			}
		})
	}
}

// THE ONE PLACE THE TWO MODES DELIBERATELY DIFFER, pinned as a PASSING test so
// it is measured rather than discovered.
//
// A project whose claims GENUINELY live outside the repository, with its lock
// ledger inside it, is ACCEPTED by --validate (the claims are on disk, they
// match their records, nothing is wrong with them) and REFUSED by --staged as
// lock-ledger-abandoned (no commit can carry those claims, so the approvals in
// the commit cover nothing the commit contains). The disagreement is real and it
// is the price of B1's fix: from the index alone this tree is byte-for-byte the
// tree an attacker produces by repointing claims_dir out of the repository, and
// the index is all --staged is allowed to read. Reading those claims off disk
// instead would restore exactly the worktree-shortcut staged.go's opening
// argument exists to deny.
//
// It is the SAFE direction of the two — a refusal an author sees immediately,
// with `git commit --no-verify` and CI's own --validate run both available,
// against a false clean in the mode that runs in the hook — and the hook already
// refused this layout before the fix, because it refuses a SKIPPED run too (see
// scripts/hook-smoke-test.sh case 19). What changed is which refusal it is: the
// hook's own "the gate could not look at these claims, here is claims_dir" has
// become a ledger finding whose recovery text ("restore the claim file from
// version control") does not fit this layout. That is worth improving in the
// message, and it is not worth buying back with a false clean.
func TestStaged_ClaimsGenuinelyOutsideTheRepositoryAreRefusedWhereValidateAccepts(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	outside := filepath.Join(root, "outside-claims")
	for _, d := range []string{repo, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFixtureFile(t, filepath.Join(repo, config.FileName), claimsDirConfig("../outside-claims"))
	writeFixtureFile(t, filepath.Join(outside, "locked.yaml"), lockedClaim("widget.contract.locked"))

	cfg, err := config.LoadConfig(filepath.Join(repo, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	// The approval is recorded exactly as `claim lock` records it, and it lands
	// in the repository beside the config — which is the whole shape: an index
	// that carries approvals and cannot carry the claims they approve.
	claims := loadFixtureClaims(t, cfg)
	armLedger(t, cfg, claims)

	gitRepo(t, repo)
	git(t, repo, "add", "-A")

	if rules := validateRules(t, cfg); len(rules) != 0 {
		t.Fatalf("--validate reads the claims off disk and must accept this project, got %v", rules)
	}
	got, skipped := stagedRulesOrSkipped(t, cfg)
	if skipped {
		t.Fatalf("the index carries a lock ledger, so this is not \"nothing to evaluate\"; --staged must reach a verdict")
	}
	if !hasName(got, lock.RuleLockLedgerAbandoned) {
		t.Fatalf("--staged judges the commit, which carries approvals for claims it does not contain: expected lock-ledger-abandoned, got %v", got)
	}
}

// The escape hatch still exists, and this is the whole of what is left of it on
// this path: a claims_dir outside the work tree in a project that has NOTHING
// for the ledger gate to judge — no lock ledger, no comment digest store, no
// build-order artifact. There genuinely is no index content to judge, --validate
// says nothing either, and refusing here would break `check --staged` in CI for
// a checkout whose claims legitimately live somewhere else.
func TestStaged_OutOfWorkTreeClaimsDirWithNoLedgerIsStillTheEscapeHatch(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	outside := filepath.Join(root, "outside-claims")
	for _, d := range []string{repo, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	writeFixtureFile(t, filepath.Join(repo, config.FileName), claimsDirConfig("../outside-claims"))
	writeFixtureFile(t, filepath.Join(outside, "draft.yaml"), draftClaim("widget.contract.one"))

	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "claims live outside the repository")

	cfg, err := config.LoadConfig(filepath.Join(repo, config.FileName))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if rules := validateRules(t, cfg); len(rules) != 0 {
		t.Fatalf("control precondition: --validate must accept this tree, got %v", rules)
	}
	if _, err := check.Staged(cfg); !errors.Is(err, check.ErrNoIndex) {
		t.Fatalf("a claims_dir outside the work tree with nothing to judge must stay the exit-0 escape hatch, got %v", err)
	}
}
