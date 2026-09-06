// check_staged_cli_test.go covers "dossierx check --staged" at the command
// level: the two promises a pre-commit hook depends on (it judges the index,
// and it writes nothing), plus the outside-a-work-tree escape hatch and the
// machine contract a skill branches on.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// stagedGit runs one git command in dir with an isolated configuration, so a
// developer's global git config cannot change what these tests measure.
func stagedGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_DATE=2026-07-26T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-07-26T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// stagedProject writes a project with one claim, locks it through the real
// approval path (so the ledger record is earned, not fabricated), and commits
// everything. It returns the config path, the project root, and the claim file.
func stagedProject(t *testing.T) (cfgPath, root, claimPath string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; --staged degrades to a warning by design")
	}

	root = t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	claimPath = filepath.Join(claimsDir, "one.yaml")
	src := "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: schema\n" +
		"body: |\n  the approved body.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	if _, stderr, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.one", "--reason", "approved in review"); err != nil {
		t.Fatalf("lock: %v (stderr=%s)", err, stderr)
	}

	stagedGit(t, root, "init", "-q", "-b", "main")
	stagedGit(t, root, "config", "user.email", "fixture@example.invalid")
	stagedGit(t, root, "config", "user.name", "fixture")
	stagedGit(t, root, "add", "-A")
	stagedGit(t, root, "commit", "-qm", "fixture")
	return cfgPath, root, claimPath
}

// The read-only promise, asserted the way the task states it: "git status
// --porcelain" must be byte-identical across the command. --staged runs DURING
// a commit; anything it wrote would change the tree it was asked to judge.
func TestCLI_CheckStaged_LeavesGitStatusByteIdentical(t *testing.T) {
	cfgPath, root, _ := stagedProject(t)

	before := stagedGit(t, root, "status", "--porcelain")
	if _, stderr, err := execReviewedCLI(t, "--config", cfgPath, "check", "--staged"); err != nil {
		t.Fatalf("check --staged: %v (stderr=%s)", err, stderr)
	}
	after := stagedGit(t, root, "status", "--porcelain")
	if before != after {
		t.Fatalf("git status --porcelain must be byte-identical across check --staged:\nbefore: %q\nafter:  %q", before, after)
	}

	// And nothing plain "check" writes on every run exists yet either: no
	// catalog, no viewer, no reconciled claim file.
	for _, rel := range []string{"build/catalog/catalog.json", filepath.Join("build", "viewer", "index.html")} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("check --staged wrote %s (stat err=%v); it must write nothing", rel, err)
		}
	}
}

// The verdict follows the INDEX. A hook that got this backwards would pass the
// commit carrying the tampered content and refuse the one that does not.
func TestCLI_CheckStaged_VerdictFollowsTheIndex(t *testing.T) {
	cfgPath, root, claimPath := stagedProject(t)

	original, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	tampered := strings.Replace(string(original), "the approved body.", "a body nobody approved.", 1)
	if tampered == string(original) {
		t.Fatalf("fixture precondition: the tamper substitution did not apply")
	}
	if err := os.WriteFile(claimPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("tamper worktree: %v", err)
	}

	// Worktree tampered, index clean: the commit is fine.
	if out, stderr, err := execReviewedCLI(t, "--config", cfgPath, "check", "--staged"); err != nil {
		t.Fatalf("a tampered WORKTREE with a clean index must pass: %v\nstdout:%s\nstderr:%s", err, out, stderr)
	}

	// Stage the tamper: refused, with the stable integrity code.
	stagedGit(t, root, "add", "claims/one.yaml")
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--staged")
	if err == nil {
		t.Fatalf("staging a tampered locked claim must be refused")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeIntegrityFailed {
		t.Fatalf("expected error.code=%q, got %#v", cliout.CodeIntegrityFailed, env.Error)
	}
	if env.StoppedAt != "ledger" {
		t.Fatalf("expected stopped_at=ledger, got %q", env.StoppedAt)
	}
	out, _, _ := execReviewedCLI(t, "--config", cfgPath, "check", "--staged") //nolint:errcheck // the command under test is EXPECTED to fail; the envelope it still emits is the assertion
	if !strings.Contains(out, "lock-content-drift") {
		t.Fatalf("expected the finding's rule name in the terminal block, got:\n%s", out)
	}
}

// Staging a locked claim WITHOUT its approval record is refused. This is what
// makes reading the ledger out of the index rather than the worktree matter:
// the record exists on disk, it just is not part of this commit.
func TestCLI_CheckStaged_RefusesAClaimCommittedWithoutItsApproval(t *testing.T) {
	cfgPath, root, _ := stagedProject(t)

	second := filepath.Join(root, "claims", "two.yaml")
	src := "id: widget.contract.two\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: schema\n" +
		"body: |\n  a second claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(second, []byte(src), 0o644); err != nil {
		t.Fatalf("write second claim: %v", err)
	}
	if _, stderr, err := execReviewedCLI(t, "--config", cfgPath, "claim", "lock", "widget.contract.two", "--reason", "approved"); err != nil {
		t.Fatalf("lock second: %v (stderr=%s)", err, stderr)
	}

	stagedGit(t, root, "add", "claims/two.yaml") // the ledger update stays unstaged
	out, _, err := execReviewedCLI(t, "--config", cfgPath, "check", "--staged")
	if err == nil {
		t.Fatalf("a locked claim staged without its approval record must be refused, got:\n%s", out)
	}
	if !strings.Contains(out, "lock-ledger-missing") {
		t.Fatalf("expected lock-ledger-missing, got:\n%s", out)
	}

	// BOTH tracked stores, because an approval now establishes comment coverage in
	// the same act that it records the approval (lock.RecordApproval writes the
	// claim's comment digest beside the ledger record). A commit that carries the
	// claim and the ledger but not the digest store leaves a standing approval with
	// no entry, which is comment-digest-missing — the finding that closed the
	// empty-the-map launder. The installed hook's refusal text names all three
	// stores for exactly this reason, so staging one of them is the fixture being
	// out of date, not the gate being strict.
	stagedGit(t, root, "add", "build/ledger/lock-store.json", "build/ledger/comment-digest.json")
	if out, stderr, err := execReviewedCLI(t, "--config", cfgPath, "check", "--staged"); err != nil {
		t.Fatalf("claim + approval staged together must pass: %v\nstdout:%s\nstderr:%s", err, out, stderr)
	}
}

// Outside a git work tree: a warning and exit 0, with data.skipped saying so.
// Failing instead would break CI for any project built from a tarball and would
// teach hook authors to swallow exit codes — the one habit that would disarm
// every other gate in this release.
func TestCLI_CheckStaged_OutsideAWorkTreeWarnsAndSucceeds(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte("schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir, "one.yaml"), []byte(
		"id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"body: |\n  a draft.\n"+
			"governed_by:\n  type: none\n  reason: fixture\n"), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--staged")
	if err != nil {
		t.Fatalf("expected exit 0 outside a work tree, got %v", err)
	}
	if !env.OK {
		t.Fatalf("expected ok=true, got %#v", env)
	}
	data, ok := env.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected an object payload, got %T", env.Data)
	}
	if skipped, _ := data["skipped"].(bool); !skipped { //nolint:errcheck // absent or non-bool both mean "not skipped", which the check handles
		t.Fatalf("expected data.skipped=true so CI can insist, got %#v", data)
	}
	if len(env.Warnings) == 0 || !strings.Contains(strings.Join(env.Warnings, " "), "no git index") {
		t.Fatalf("expected a warning naming the reason, got %#v", env.Warnings)
	}
}

// --validate and --staged answer different questions; combining them is a usage
// error rather than a silent precedence rule, so a hook can never report having
// validated something other than what it validated.
func TestCLI_CheckStaged_RefusesToCombineWithValidate(t *testing.T) {
	cfgPath, _, _ := stagedProject(t)
	_, _, err := execReviewedCLI(t, "--config", cfgPath, "check", "--staged", "--validate")
	if err == nil {
		t.Fatalf("expected --staged --validate to be refused")
	}
	if !strings.Contains(err.Error(), "different questions") {
		t.Fatalf("unexpected error: %v", err)
	}
}
