// staged_nonregular_test.go pins what `check --staged` does with the two index
// entries that are NOT a regular file: a symlink (git mode 120000) and a
// submodule gitlink (mode 160000).
//
// THE DEFECT. Both aborted the entire run. A symlink's blob is the link TARGET,
// so decodeClaim failed on it ("cannot unmarshal !!str `../../o...`"); a
// gitlink's oid is a commit in ANOTHER repository, normally absent from this
// one's object store, so `git cat-file --batch` answered "<oid> missing" and
// catFile rejected the two-field header. Either way the error reached the CLI
// unclassified, as `internal` — the code internal/cliout/codes.go itself
// documents as "a bug report, not a branch target", and the one code the
// router skill's recovery table has no row for. Through the real pre-commit
// hook that refused EVERY commit in such a repository, printing unlock -> fix
// -> lock advice that cannot possibly apply, while `check` and
// `check --validate` accepted the identical tree.
//
// THE BEHAVIOUR NOW, and it is one behaviour for both modes: the entry is not
// in the registry. That is not a shrug — it is the same thing the stage filter
// beside it means, and it has teeth. An ordinary repository that carries a
// symlink or a vendored submodule under claims_dir is accepted, by both gates.
// One that uses either to DISPLACE a claim the ledger holds an approval for is
// refused as lock-ledger-abandoned, because the approval now covers nothing the
// commit contains — which is exactly true: the commit carries a link, or a
// foreign commit id, at that path.
//
// The displacement pair below is a DELIBERATE, pinned disagreement with
// --validate, in the safe direction, and it is the same one
// staged_parity_test.go already pins for a claims_dir that genuinely lives
// outside the repository: --validate reads through the link (or into the
// submodule's checkout) OFF DISK and is satisfied, and --staged is not allowed
// to read the working tree at all. Closing it from the --staged side would mean
// resolving a link target out of the worktree, which is the assume-unchanged
// bypass staged.go's opening argument exists to deny.
package check_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// symlinkOrSkip creates link -> target, or skips the test where the platform
// will not allow it. Windows needs either developer mode or an elevated process
// to create a symlink, and a CI leg that cannot create one has nothing to say
// about how the gate reads one.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("cannot create a symlink on this platform: %v", err)
		}
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}

// nestedRepo turns dir into a git repository of its own with one commit in it.
//
// That is what a submodule IS as far as the superproject's index is concerned,
// and it is why the fixture does not need `git submodule add`: `git add` in the
// parent records an embedded repository as a gitlink (mode 160000) whose oid is
// a commit in the OTHER repository's object store — an object the parent
// repository does not have, which is precisely the condition that made
// `cat-file --batch` answer "missing".
func nestedRepo(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	gitRepo(t, dir)
	for rel, body := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		writeFixtureFile(t, abs, body)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "the other repository")
}

// indexModes returns the index's mode for each path, so a fixture can prove it
// actually built the entry it claims to be testing. A test that silently staged
// an ordinary 100644 blob would pass for the wrong reason.
func indexModes(t *testing.T, dir string) map[string]string {
	t.Helper()
	modes := map[string]string{}
	for _, line := range strings.Split(git(t, dir, "ls-files", "-s"), "\n") {
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		fields := strings.Fields(line[:tab])
		if len(fields) < 3 {
			continue
		}
		modes[strings.TrimSpace(line[tab+1:])] = fields[0]
	}
	return modes
}

func requireMode(t *testing.T, dir, rel, want string) {
	t.Helper()
	modes := indexModes(t, dir)
	if got := modes[rel]; got != want {
		t.Fatalf("fixture precondition: %s must be staged with mode %s, got %q (index: %v)", rel, want, got, modes)
	}
}

// A SYMLINK under claims_dir must not abort the run, and must not be mistaken
// for a claim: its blob is a path string.
//
// The link points at a perfectly good DRAFT claim outside the repository, which
// is the honest version of this layout — someone sharing one claim file between
// two checkouts. --validate reads through the link and finds a draft (drafts are
// free, so nothing covers it and no rule fires); --staged sees mode 120000 and
// leaves it out of the registry. Both accept, and neither dies.
func TestStaged_ASymlinkUnderClaimsDirDoesNotAbortTheRun(t *testing.T) {
	cfg := parityFixture(t)
	writeFixtureFile(t, filepath.Join(filepath.Dir(cfg.Dir()), "outside-claims", "extra.yaml"), draftClaim("widget.contract.extra"))
	symlinkOrSkip(t, filepath.Join("..", "..", "outside-claims", "extra.yaml"), filepath.Join(cfg.ClaimsDir, "extra.yaml"))
	git(t, cfg.Dir(), "add", "-A")
	requireMode(t, cfg.Dir(), "claims/extra.yaml", "120000")

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("a symlink under claims_dir must not end the run: %v", err)
	}
	for _, c := range sp.Claims {
		if c.ID == "widget.contract.extra" {
			t.Fatalf("the index's copy of a symlink is its target path, not a claim; it must not enter the registry")
		}
	}
	if got := rulesOf(check.StatusStaged(sp, cfg).LedgerFindings); len(got) != 0 {
		t.Fatalf("nothing about this tree is wrong: expected no ledger findings, got %v", got)
	}
	if want := validateRules(t, cfg); len(want) != 0 {
		t.Fatalf("control: --validate must accept this tree too, got %v", want)
	}
}

// The same symlink, used to DISPLACE a claim the ledger has an approval for.
//
// This must not pass unnoticed, and it does not: the commit carries a link at
// claims/locked.yaml, so the approval standing on the ledger covers nothing the
// commit contains, which is lock-ledger-abandoned. --validate reads through the
// link off disk, finds the claim intact, and accepts — the deliberate,
// safe-direction disagreement this file's header explains.
func TestStaged_ASymlinkDisplacingALockedClaimIsRefusedNotAborted(t *testing.T) {
	cfg := parityFixture(t)
	outside := filepath.Join(filepath.Dir(cfg.Dir()), "outside-claims", "locked.yaml")
	locked := filepath.Join(cfg.ClaimsDir, "locked.yaml")
	raw, err := os.ReadFile(locked)
	if err != nil {
		t.Fatalf("read locked claim: %v", err)
	}
	writeFixtureFile(t, outside, string(raw))
	if err := os.Remove(locked); err != nil {
		t.Fatalf("remove locked claim: %v", err)
	}
	symlinkOrSkip(t, filepath.Join("..", "..", "outside-claims", "locked.yaml"), locked)
	git(t, cfg.Dir(), "add", "-A")
	requireMode(t, cfg.Dir(), "claims/locked.yaml", "120000")

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("a symlink under claims_dir must not end the run: %v", err)
	}
	got := rulesOf(check.StatusStaged(sp, cfg).LedgerFindings)
	if !hasName(got, lock.RuleLockLedgerAbandoned) {
		t.Fatalf("the commit carries a link where a locked claim was, so its approval covers nothing in the commit: expected %s, got %v", lock.RuleLockLedgerAbandoned, got)
	}
}

// A SUBMODULE GITLINK under claims_dir must not abort the run either. Its oid
// is a commit in the other repository, which this one does not have, so the
// gate cannot read it and must not try.
//
// The vendored repository deliberately holds no claim at all here, which is the
// ordinary case — a submodule that happens to sit under claims_dir for reasons
// that have nothing to do with claims. Both gates accept.
func TestStaged_ASubmoduleGitlinkUnderClaimsDirDoesNotAbortTheRun(t *testing.T) {
	cfg := parityFixture(t)
	nestedRepo(t, filepath.Join(cfg.ClaimsDir, "vendored"), map[string]string{
		"NOTES.md": "another repository entirely\n",
	})
	git(t, cfg.Dir(), "add", "-A")
	requireMode(t, cfg.Dir(), "claims/vendored", "160000")

	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("a submodule gitlink under claims_dir must not end the run: %v", err)
	}
	if got := rulesOf(check.StatusStaged(sp, cfg).LedgerFindings); len(got) != 0 {
		t.Fatalf("nothing about this tree is wrong: expected no ledger findings, got %v", got)
	}
	if want := validateRules(t, cfg); len(want) != 0 {
		t.Fatalf("control: --validate must accept this tree too, got %v", want)
	}
}

// A submodule that VENDORS the locked claim: the claim file moves into the
// other repository, and this commit carries only the gitlink.
//
// --validate walks into the checked-out submodule, loads the claim and is
// satisfied. --staged cannot: the claim's content is in a tree this commit does
// not carry, so the approval on the ledger covers nothing here and the run is
// refused rather than aborted or waved through.
func TestStaged_ASubmoduleVendoringALockedClaimIsRefusedNotAborted(t *testing.T) {
	cfg := parityFixture(t)
	locked := filepath.Join(cfg.ClaimsDir, "locked.yaml")
	raw, err := os.ReadFile(locked)
	if err != nil {
		t.Fatalf("read locked claim: %v", err)
	}
	if err := os.Remove(locked); err != nil {
		t.Fatalf("remove locked claim: %v", err)
	}
	nestedRepo(t, filepath.Join(cfg.ClaimsDir, "vendored"), map[string]string{"locked.yaml": string(raw)})
	git(t, cfg.Dir(), "add", "-A")
	requireMode(t, cfg.Dir(), "claims/vendored", "160000")

	if want := validateRules(t, cfg); len(want) != 0 {
		t.Fatalf("control: --validate reads the submodule's checkout off disk and must accept this tree, got %v", want)
	}
	sp, err := check.Staged(cfg)
	if err != nil {
		t.Fatalf("a submodule gitlink under claims_dir must not end the run: %v", err)
	}
	got := rulesOf(check.StatusStaged(sp, cfg).LedgerFindings)
	if !hasName(got, lock.RuleLockLedgerAbandoned) {
		t.Fatalf("the claim's content lives in another repository, so this commit's approval covers nothing it carries: expected %s, got %v", lock.RuleLockLedgerAbandoned, got)
	}
}
