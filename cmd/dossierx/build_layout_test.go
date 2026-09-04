// build_layout_test.go covers the build/ layout at the command level: the
// legacy-root refusal every verb gives, the store-gitignored finding and the
// store_gitignored refusal, the dry-run precondition that predicts it, and the
// gitignore_check non-verdict the read-only modes report.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/layout"
)

// blGit runs git in dir with an isolated configuration.
func blGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func blGitInit(t *testing.T, dir string) {
	t.Helper()
	blGit(t, dir, "init", "-q", "-b", "main")
	blGit(t, dir, "config", "user.email", "fixture@example.invalid")
	blGit(t, dir, "config", "user.name", "fixture")
}

func blWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// blLegacyProject is icWriteFixtureProject plus the legacy root files.
func blLegacyProject(t *testing.T, root string, legacy map[string]string) (cfgPath string) {
	t.Helper()
	cfgPath, _ = icWriteFixtureProject(t, root, "widget")
	for rel, body := range legacy {
		blWrite(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return cfgPath
}

func blDetailsMoves(t *testing.T, env cliout.Envelope) []map[string]any {
	t.Helper()
	details, ok := env.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("error.details = %#v, want an object with moves", env.Error.Details)
	}
	raw, err := json.Marshal(details["moves"])
	if err != nil {
		t.Fatalf("error.details.moves does not re-encode: %v", err)
	}
	var moves []map[string]any
	if err := json.Unmarshal(raw, &moves); err != nil {
		t.Fatalf("error.details.moves does not decode: %v", err)
	}
	return moves
}

func blRecoveryLines(msg string) []string {
	var lines []string
	for _, l := range strings.Split(msg, "\n") {
		if strings.HasPrefix(l, "  ") {
			lines = append(lines, strings.TrimPrefix(l, "  "))
		}
	}
	return lines
}

// TestLegacyLayoutRefusesEveryVerbWithGitMvLines: a legacy root layout
// refuses check, check --validate, claim list and build-order status alike
// with layout_legacy, the exact lines from (d), and under --format json one
// details.moves entry per printed move line in the same order — and then
// EXECUTES the printed block in a committed copy and requires exit 0.
func TestLegacyLayoutRefusesEveryVerbWithGitMvLines(t *testing.T) {
	root := t.TempDir()
	if layout.InWorkTree(root) {
		t.Fatalf("%s sits inside a git work tree; the no-git half cannot be judged here", root)
	}
	cfgPath := blLegacyProject(t, root, map[string]string{
		".dossierx-lock-store.json":     `{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`,
		".dossierx-comment-digest.json": `{"version":1,"digests":{}}`,
		".catalog.json":                 "{}",
		"viewer/index.html":             "<html>",
	})
	wantLines := []string{
		"mkdir -p build/ledger",
		"mv .dossierx-lock-store.json build/ledger/lock-store.json",
		"mv .dossierx-comment-digest.json build/ledger/comment-digest.json",
		"rm -f .catalog.json",
		"rm -f viewer/index.html",
	}
	for _, verb := range [][]string{
		{"check"},
		{"check", "--validate"},
		{"claim", "list"},
		{"build-order", "status", "--module", "widget"},
	} {
		t.Run(strings.Join(verb, " "), func(t *testing.T) {
			env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, verb...)...)
			if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLayoutLegacy {
				t.Fatalf("expected error.code layout_legacy, got err=%v env=%+v", err, env)
			}
			if got := blRecoveryLines(env.Error.Message); strings.Join(got, "\n") != strings.Join(wantLines, "\n") {
				t.Fatalf("recovery lines:\n got %q\nwant %q", got, wantLines)
			}
			moves := blDetailsMoves(t, env)
			if len(moves) != len(wantLines)-1 {
				t.Fatalf("details.moves has %d entries, want %d", len(moves), len(wantLines)-1)
			}
			for i, m := range moves {
				from, fromIsString := m["from"].(string)
				tracked, trackedIsBool := m["tracked"].(bool)
				if _, hasTo := m["to"]; !hasTo || !fromIsString || !trackedIsBool || !strings.Contains(wantLines[i+1], from) || tracked {
					t.Fatalf("details.moves[%d] = %v does not match printed line %q (untracked outside a work tree)", i, m, wantLines[i+1])
				}
			}
		})
	}

	// The executable half: a committed copy where the files ARE tracked.
	t.Run("the printed block runs to the end in a committed copy", func(t *testing.T) {
		repo := t.TempDir()
		cfgPath := blLegacyProject(t, repo, map[string]string{
			".dossierx-lock-store.json":     `{"version":2,"hashes":{},"locked_at":{},"ledger":{}}`,
			".dossierx-comment-digest.json": `{"version":1,"digests":{}}`,
			".catalog.json":                 "{}",
			"viewer/index.html":             "<html>",
		})
		blGitInit(t, repo)
		blGit(t, repo, "add", "-A")
		blGit(t, repo, "commit", "-qm", "baseline")
		env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLayoutLegacy {
			t.Fatalf("expected layout_legacy, got err=%v env=%+v", err, env)
		}
		lines := blRecoveryLines(env.Error.Message)
		want := []string{
			"mkdir -p build/ledger",
			"git mv .dossierx-lock-store.json build/ledger/lock-store.json",
			"git mv .dossierx-comment-digest.json build/ledger/comment-digest.json",
			"git rm --cached --ignore-unmatch .catalog.json && rm -f .catalog.json",
			"git rm --cached --ignore-unmatch viewer/index.html && rm -f viewer/index.html",
		}
		if strings.Join(lines, "\n") != strings.Join(want, "\n") {
			t.Fatalf("recovery lines:\n got %q\nwant %q", lines, want)
		}
		// Run the first three lines alone: still refused, naming only the
		// catalog and the viewer. `sh` is resolved on PATH, not /bin/sh: the
		// windows CI leg has no /bin/sh and runs the block in Git for
		// Windows' sh.exe, as tests/readme_setup_replay_test.go does.
		partial := exec.Command("sh", "-c", "set -e\n"+strings.Join(lines[:3], "\n")+"\n")
		partial.Dir = repo
		if out, err := partial.CombinedOutput(); err != nil {
			t.Fatalf("the first three lines failed: %v\n%s", err, out)
		}
		env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLayoutLegacy {
			t.Fatalf("after the moves alone, the catalog and viewer must still refuse: err=%v env=%+v", err, env)
		}
		if got := blRecoveryLines(env.Error.Message); strings.Join(got, "\n") != strings.Join(want[3:], "\n") {
			t.Fatalf("after the moves the block must name only the catalog and the viewer, got %q", got)
		}
		rest := exec.Command("sh", "-c", "set -e\n"+strings.Join(lines[3:], "\n")+"\n")
		rest.Dir = repo
		if out, err := rest.CombinedOutput(); err != nil {
			t.Fatalf("the removal lines failed: %v\n%s", err, out)
		}
		status := blGit(t, repo, "status", "--porcelain")
		for _, w := range []string{"R  .dossierx-lock-store.json -> build/ledger/lock-store.json", "R  .dossierx-comment-digest.json -> build/ledger/comment-digest.json", "D  .catalog.json", "D  viewer/index.html"} {
			if !strings.Contains(status, w) {
				t.Fatalf("expected %q in git status, got:\n%s", w, status)
			}
		}
		env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
		if err != nil || !env.OK {
			t.Fatalf("after the block, check --validate must pass: err=%v env=%+v", err, env)
		}
		data, err := json.Marshal(env.Data)
		if err != nil {
			t.Fatalf("re-encode data: %v", err)
		}
		for _, prefix := range []string{"lock-ledger-", "comment-digest-", "build-order-"} {
			if strings.Contains(string(data), `"rule":"`+prefix) {
				t.Fatalf("no ledger finding may follow a pure move (signatures hash bytes, not paths): %s", data)
			}
		}
	})
}

// blIgnoredRepo is a git-inited project whose .gitignore says build/, with
// one draft claim.
func blIgnoredRepo(t *testing.T) (root, cfgPath string) {
	t.Helper()
	root = t.TempDir()
	cfgPath, _ = icWriteFixtureProject(t, root, "widget")
	blGitInit(t, root)
	blWrite(t, filepath.Join(root, ".gitignore"), "build/\n")
	return root, cfgPath
}

// blData re-encodes an envelope's data section into dst; the envelope was
// decoded into an any, and the tests read typed fields off it.
func blData(t *testing.T, env cliout.Envelope, dst any) {
	t.Helper()
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-encode data: %v", err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode data: %v", err)
	}
}

func blRules(t *testing.T, env cliout.Envelope) []string {
	t.Helper()
	var data struct {
		LedgerFindings []struct{ Rule string } `json:"ledger_findings"`
	}
	blData(t, env, &data)
	var rules []string
	for _, f := range data.LedgerFindings {
		rules = append(rules, f.Rule)
	}
	return rules
}

func blGitignoreCheck(t *testing.T, env cliout.Envelope) (string, bool) {
	t.Helper()
	var data map[string]any
	blData(t, env, &data)
	v, ok := data["gitignore_check"].(string)
	return v, ok
}

// TestStoreGitignoredIsAnErrorFindingAndARefusal: with build/ ignored, check
// exits 1 with a store-gitignored finding and claim lock refuses; with lint
// red as well the finding is still reported; with the lock store force-added
// the read-only modes carry the ignored-but-tracked warning unchanged in exit.
func TestStoreGitignoredIsAnErrorFindingAndARefusal(t *testing.T) {
	t.Run("finding and refusal", func(t *testing.T) {
		_, cfgPath := blIgnoredRepo(t)
		env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
		if err == nil || env.OK {
			t.Fatalf("check on an ignored build/ must exit non-zero: env=%+v", env)
		}
		if rules := blRules(t, env); len(rules) == 0 || rules[0] != "store-gitignored" {
			t.Fatalf("expected a store-gitignored ledger finding, got %v", rules)
		}
		if _, present := blGitignoreCheck(t, env); present {
			t.Fatalf("a verdict was reached, so gitignore_check must be absent: %+v", env.Data)
		}
		env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeStoreGitignored {
			t.Fatalf("claim lock must refuse with store_gitignored, got err=%v env=%+v", err, env)
		}
		if !strings.Contains(env.Error.Message, "!build/code-links/*") || !strings.Contains(env.Error.Message, "build_dir") {
			t.Fatalf("the refusal must carry the replacement block and the build_dir recovery: %s", env.Error.Message)
		}
	})

	t.Run("lint red still reports the finding", func(t *testing.T) {
		root, cfgPath := blIgnoredRepo(t)
		blWrite(t, filepath.Join(root, "claims", "dangling.yaml"), "id: widget.contract.dangling\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  rests on nothing that exists.\ngoverned_by:\n  type: none\n  reason: fixture\nrests_on:\n  - widget.contract.nowhere\n")
		env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
			t.Fatalf("expected lint_failed, got err=%v env=%+v", err, env)
		}
		rules := blRules(t, env)
		found := false
		for _, r := range rules {
			if r == "store-gitignored" {
				found = true
			}
		}
		if !found {
			t.Fatalf("a lint error stops the pipeline, not the guard: ledger_findings must still carry store-gitignored, got %v", rules)
		}
		if _, present := blGitignoreCheck(t, env); present {
			t.Fatalf("a verdict was reached, so gitignore_check must be absent: %+v", env.Data)
		}
	})

	t.Run("force-added ledger is a warning on every read-only mode", func(t *testing.T) {
		// The recommended block plus ONE extra pattern that matches only the
		// lock store, which is then force-added: the one ignored path is
		// tracked, so the verdict is "warning, nothing else" and the exit
		// status is the clean one — which is what "unchanged" has to mean for
		// the row to be measurable. (With a bare build/ the untracked siblings
		// are findings in their own right and decide the exit status alone;
		// that split is pinned in internal/check's gitignore_test.go.)
		root, cfgPath := blIgnoredRepo(t)
		blWrite(t, filepath.Join(root, ".gitignore"), "")
		if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved"); err != nil {
			t.Fatalf("seed lock: %v", err)
		}
		blGit(t, root, "add", "-A")
		blGit(t, root, "commit", "-qm", "seed")
		blWrite(t, filepath.Join(root, ".gitignore"), layout.RecommendedGitignore+"\nbuild/ledger/lock-store.json\n")
		blGit(t, root, "add", "-A")
		blGit(t, root, "commit", "-qm", "ignore the tracked ledger")
		for _, verb := range [][]string{{"check"}, {"check", "--validate"}, {"check", "--staged"}} {
			env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, verb...)...)
			if err != nil || !env.OK {
				t.Fatalf("%v: an ignored-but-tracked ledger must not change the exit status: err=%v env=%+v", verb, err, env)
			}
			if rules := blRules(t, env); len(rules) != 0 {
				t.Fatalf("%v: an ignored-but-tracked ledger is not a finding, got %v", verb, rules)
			}
			found := false
			for _, w := range env.Warnings {
				if strings.HasPrefix(w, "build/ledger/lock-store.json is in the repository but matched by .gitignore pattern") {
					found = true
				}
			}
			if !found {
				t.Fatalf("%v: expected the ignored-but-tracked warning in warnings[], got %v", verb, env.Warnings)
			}
		}
		// And claim lock succeeds with the warning in its envelope.
		blWrite(t, filepath.Join(root, "claims", "two.yaml"), "id: widget.contract.two\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  a second claim.\ngoverned_by:\n  type: none\n  reason: fixture\n")
		env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.two", "--reason", "approved")
		if err != nil || !env.OK {
			t.Fatalf("claim lock over a force-added ledger must succeed, got err=%v env=%+v", err, env)
		}
		found := false
		for _, w := range env.Warnings {
			if strings.HasPrefix(w, "build/ledger/lock-store.json is in the repository but matched by .gitignore pattern") {
				found = true
			}
		}
		if !found {
			t.Fatalf("claim lock must carry the ignored-but-tracked warning, got %v", env.Warnings)
		}
	})
}

// TestDryRun_StoreGitignoredIsAFailingPrecondition: each single-id approval
// verb's --dry-run reports stores_are_tracked ok:false where the real run
// refuses with store_gitignored; a policy-enabled batch preview reports the
// same failing store precondition before the real run refuses.
func TestDryRun_StoreGitignoredIsAFailingPrecondition(t *testing.T) {
	// A project locked and flagged BEFORE build/ is ignored, so reaudit and
	// flag have a claim in the state they require.
	seed := func(t *testing.T) (root, cfgPath string) {
		t.Helper()
		root = t.TempDir()
		cfgPath, _ = icWriteFixtureProject(t, root, "widget")
		blWrite(t, filepath.Join(root, "claims", "one.yaml"), "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: schema\nbody: |\n  one.\ngoverned_by:\n  type: none\n  reason: fixture\n")
		blWrite(t, filepath.Join(root, "claims", "overview.yaml"), "id: widget.contract.overview\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: orientation\nbody: |\n  fixture claim.\ngoverned_by:\n  type: none\n  reason: fixture\n")
		blGitInit(t, root)
		for _, id := range []string{"widget.contract.one", "widget.contract.overview"} {
			if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil {
				t.Fatalf("seed lock %s: %v", id, err)
			}
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", "widget.contract.one", "--claim-says", "one", "--now-does", "two", "--reason", "changed"); err != nil {
			t.Fatalf("seed flag: %v", err)
		}
		if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
			t.Fatalf("seed propose: %v", err)
		}
		blWrite(t, filepath.Join(root, ".gitignore"), "build/\n")
		return root, cfgPath
	}
	refusedAndBlocked := func(t *testing.T, cfgPath string, dry, real []string) {
		t.Helper()
		dr := dryRunOf(t, append([]string{"--config", cfgPath}, dry...)...)
		if !dr.Blocked || !hasPrecondition(dr, "stores_are_tracked", false) {
			t.Fatalf("%v must preview blocked with stores_are_tracked ok:false, got %+v", dry, dr)
		}
		env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, real...)...)
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeStoreGitignored {
			t.Fatalf("%v must refuse with store_gitignored, got err=%v env=%+v", real, err, env)
		}
	}
	t.Run("claim lock", func(t *testing.T) {
		root, cfgPath := seed(t)
		blWrite(t, filepath.Join(root, "claims", "three.yaml"), "id: widget.contract.three\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: schema\nbody: |\n  three.\ngoverned_by:\n  type: none\n  reason: fixture\n")
		refusedAndBlocked(t, cfgPath,
			[]string{"claim", "lock", "widget.contract.three", "--dry-run", "--reason", "ok"},
			[]string{"claim", "lock", "widget.contract.three", "--reason", "ok"})
	})
	t.Run("claim flag", func(t *testing.T) {
		_, cfgPath := seed(t)
		refusedAndBlocked(t, cfgPath,
			[]string{"claim", "flag", "widget.contract.overview", "--dry-run", "--claim-says", "a", "--now-does", "b", "--reason", "c"},
			[]string{"claim", "flag", "widget.contract.overview", "--claim-says", "a", "--now-does", "b", "--reason", "c"})
	})
	t.Run("claim reaudit --confirm", func(t *testing.T) {
		_, cfgPath := seed(t)
		refusedAndBlocked(t, cfgPath,
			[]string{"claim", "reaudit", "widget.contract.one", "--dry-run", "--reason", "ok"},
			[]string{"claim", "reaudit", "widget.contract.one", "--confirm", "--reason", "ok"})
	})
	t.Run("build-order lock", func(t *testing.T) {
		_, cfgPath := seed(t)
		refusedAndBlocked(t, cfgPath,
			[]string{"build-order", "lock", "--module", "widget", "--dry-run", "--reason", "ok"},
			[]string{"build-order", "lock", "--module", "widget", "--reason", "ok"})
	})
	t.Run("batch claim lock preview and refusal before the sentinel", func(t *testing.T) {
		root, cfgPath := seed(t)
		for _, id := range []string{"a", "b"} {
			blWrite(t, filepath.Join(root, "claims", id+".yaml"), "id: widget.contract."+id+"\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbuild_role: schema\nbody: |\n  "+id+".\ngoverned_by:\n  type: none\n  reason: fixture\n")
		}
		dr := dryRunOf(t, "--config", cfgPath, "claim", "lock", "widget.contract.a", "widget.contract.b", "--dry-run", "--reason", "ok")
		if !dr.Blocked || !hasPrecondition(dr, "stores_are_tracked", false) {
			t.Fatalf("the batch dry-run must report stores_are_tracked ok:false, got %+v", dr)
		}
		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.a", "widget.contract.b", "--reason", "ok")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeStoreGitignored {
			t.Fatalf("batch lock must refuse with store_gitignored, got err=%v env=%+v", err, env)
		}
		if _, statErr := os.Stat(filepath.Join(root, "build", "ledger", "claims.lock")); !os.IsNotExist(statErr) {
			t.Fatalf("the refusal must come before the claims sentinel is acquired (stat err=%v)", statErr)
		}
	})
}

// TestCLI_CheckReportsGitignoreCheckWhenTheGuardCannotApply: the read-only
// modes report data.gitignore_check for each non-verdict and exit 0, while
// claim lock refuses on the one that means "git could not answer".
func TestCLI_CheckReportsGitignoreCheckWhenTheGuardCannotApply(t *testing.T) {
	t.Run("not a work tree", func(t *testing.T) {
		root := t.TempDir()
		if layout.InWorkTree(root) {
			t.Fatalf("%s sits inside a git work tree; this row cannot be judged here", root)
		}
		cfgPath, _ := icWriteFixtureProject(t, root, "widget")
		env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
		if err != nil {
			t.Fatalf("check --validate: %v", err)
		}
		if got, _ := blGitignoreCheck(t, env); got != "not a work tree" {
			t.Fatalf("data.gitignore_check = %q, want %q", got, "not a work tree")
		}
	})
	t.Run("outside the work tree", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "repo")
		if err := os.MkdirAll(filepath.Join(repo, "claims"), 0o755); err != nil {
			t.Fatal(err)
		}
		blWrite(t, filepath.Join(repo, "project.config.yaml"), "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nbuild_dir: ../out\n")
		blWrite(t, filepath.Join(repo, "claims", "overview.yaml"), "id: widget.contract.overview\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  fixture.\ngoverned_by:\n  type: none\n  reason: fixture\n")
		blGitInit(t, repo)
		env, _, err := execCLIJSON(t, "--config", filepath.Join(repo, "project.config.yaml"), "check", "--validate")
		if err != nil {
			t.Fatalf("check --validate: %v", err)
		}
		if got, _ := blGitignoreCheck(t, env); got != "outside the work tree" {
			t.Fatalf("data.gitignore_check = %q, want %q", got, "outside the work tree")
		}
		found := false
		for _, w := range env.Warnings {
			if strings.Contains(w, "build_dir resolves outside the repository") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected the outside-the-repository warning, got %v", env.Warnings)
		}
	})
	t.Run("git not available", func(t *testing.T) {
		root := t.TempDir()
		cfgPath, _ := icWriteFixtureProject(t, root, "widget")
		blGitInit(t, root)
		t.Setenv("PATH", t.TempDir())
		env, _, err := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
		if err != nil {
			t.Fatalf("check --validate must exit 0 where git cannot answer: %v", err)
		}
		if got, _ := blGitignoreCheck(t, env); got != "git not available" {
			t.Fatalf("data.gitignore_check = %q, want %q", got, "git not available")
		}
		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved")
		if err == nil || env.Error == nil || env.Error.Code != cliout.CodeStoreGitignored {
			t.Fatalf("claim lock must refuse where git cannot answer, got err=%v env=%+v", err, env)
		}
		if !strings.Contains(env.Error.Message, "cannot tell whether build/ledger/lock-store.json is ignored") {
			t.Fatalf("the refusal must carry the verbatim text: %s", env.Error.Message)
		}
	})
}

// TestCheckOnAFreshProjectWritesOnlyUnderBuild is WP1's fresh-project
// acceptance criterion in executable form: testdata/fixture-basic's INPUTS
// (its config and claims, never its committed build/) copied to a temp
// directory outside any work tree, one `dossierx check`, exit 0, and the
// project then holds exactly project.config.yaml, the claims, build/.gitignore,
// build/catalog/catalog.json, build/viewer/index.html and
// build/ledger/comment-digest.json — nothing beginning with "." at the root
// except the config, and no viewer/ directory.
func TestCheckOnAFreshProjectWritesOnlyUnderBuild(t *testing.T) {
	root := t.TempDir()
	if layout.InWorkTree(root) {
		t.Fatalf("%s sits inside a git work tree; the fresh-project row cannot be judged here", root)
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixture-basic"))
	if err != nil {
		t.Fatal(err)
	}
	copied := 0
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel == "build" {
				return filepath.SkipDir
			}
			return nil
		}
		body, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		blWrite(t, filepath.Join(root, rel), string(body))
		copied++
		return nil
	})
	if err != nil {
		t.Fatalf("copy fixture inputs: %v", err)
	}
	if copied < 2 {
		t.Fatalf("copied %d file(s) from %s; the fixture's config and claims were expected", copied, src)
	}

	cfgPath := filepath.Join(root, "project.config.yaml")
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("check on a fresh project must exit 0: err=%v env=%+v stderr=%s", err, env, stderr)
	}

	var got []string
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel == "claims" {
				return filepath.SkipDir
			}
			return nil
		}
		got = append(got, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("list project: %v", err)
	}
	sort.Strings(got)
	want := []string{
		"build/.gitignore",
		"build/catalog/catalog.json",
		"build/ledger/comment-digest.json",
		"build/viewer/index.html",
		"project.config.yaml",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("files outside claims/ after check:\n got %q\nwant %q", got, want)
	}
}

// TestCheckWarnsWhenAStyleOverrideIsInForceBesideALockedOrder pins the one
// render-side warning check carries: the Build order tab's diagram colours,
// its overflow rules and its sticky module strip live in the engine's
// style.css (the .bo-* rules), and a project that overrides style.css and
// locks an order would otherwise get mermaid's base-theme lavender nodes and
// a page that scrolls sideways with nothing said. Present on check with the
// override and a locked order; absent once the artifact is gone or the file
// removed. Render has no warnings channel, which is why it is check's line.
func TestCheckWarnsWhenAStyleOverrideIsInForceBesideALockedOrder(t *testing.T) {
	root := t.TempDir()
	cfgPath, _, _ := restsOnPairProject(t, root)
	cfgRaw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := os.WriteFile(cfgPath, append(cfgRaw, []byte("viewer:\n  template_overrides: tmpl\n")...), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	tmplDir := filepath.Join(root, "tmpl")
	if err := os.MkdirAll(tmplDir, 0o755); err != nil {
		t.Fatalf("mkdir tmpl: %v", err)
	}
	stylePath := filepath.Join(tmplDir, "style.css")
	if err := os.WriteFile(stylePath, []byte("body { color: black; }\n"), 0o644); err != nil {
		t.Fatalf("write style.css: %v", err)
	}
	const want = "viewer.template_overrides/style.css is in force: the Build order tab's diagram colours and overflow rules come from the engine's style.css and are not supplied by the override; copy the .bo-* rules into it"
	hasWarning := func(env cliout.Envelope) bool {
		for _, w := range env.Warnings {
			if w == want {
				return true
			}
		}
		return false
	}

	// Override in force, no locked order: no warning.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check before any lock: %v", err)
	}
	if hasWarning(env) {
		t.Fatalf("the warning must not fire with no locked order; got %v", env.Warnings)
	}

	for _, id := range []string{"widget.contract.alpha", "widget.contract.beta"} {
		if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed"); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	// Proposed but not locked: still no warning (the tab renders only locked
	// orders, so there is nothing the override could be mis-painting yet).
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check after propose: %v", err)
	}
	if hasWarning(env) {
		t.Fatalf("the warning must not fire for a proposed-but-unlocked order; got %v", env.Warnings)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check with a locked order and a style override: %v", err)
	}
	if !env.OK || !hasWarning(env) {
		t.Fatalf("expected ok:true with the style-override warning, got ok=%v warnings=%v", env.OK, env.Warnings)
	}
	// --validate carries it too: the override is in force for whatever the
	// next check renders, and a CI reader learns it here.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil {
		t.Fatalf("check --validate: %v", err)
	}
	if !hasWarning(env) {
		t.Fatalf("expected the warning on --validate, got %v", env.Warnings)
	}

	// The file removed: absent.
	if err := os.Remove(stylePath); err != nil {
		t.Fatalf("remove style.css: %v", err)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check after removing the override: %v", err)
	}
	if hasWarning(env) {
		t.Fatalf("the warning must vanish with the override file; got %v", env.Warnings)
	}
}

// TestCheckWarnsAndStillRendersWhenALockedOrderCannotBeDrawn pins the
// per-module half of the Build order tab's load-error policy at the CLI: a
// locked artifact whose stored phase name the engine does not know (a
// hand-edit; the ledger's content-drift finding fails the run for it) costs
// that module its tab entry and ONE warnings[] line naming the module and
// the reason — the viewer is still written, with the module's ordinary
// claims in it. Before this, the same edit failed Render and no viewer was
// written for any module.
func TestCheckWarnsAndStillRendersWhenALockedOrderCannotBeDrawn(t *testing.T) {
	root := t.TempDir()
	cfgPath, _, _ := restsOnPairProject(t, root)
	for _, id := range []string{"widget.contract.alpha", "widget.contract.beta"} {
		if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "reviewed"); err != nil {
			t.Fatalf("claim lock %s: %v", id, err)
		}
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("build-order propose: %v", err)
	}
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "approved"); err != nil {
		t.Fatalf("build-order lock: %v", err)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil {
		t.Fatalf("check with a sound locked order: %v", err)
	}
	for _, w := range env.Warnings {
		if strings.Contains(w, "is not drawn in the viewer's Build order tab") {
			t.Fatalf("a sound locked order must not warn; got %q", w)
		}
	}
	viewer := filepath.Join(root, "build", "viewer", "index.html")
	before, err := os.ReadFile(viewer)
	if err != nil {
		t.Fatalf("read viewer: %v", err)
	}
	if !strings.Contains(string(before), `id="dossierx-build-order-widget"`) {
		t.Fatal("the sound order must render widget's tab entry")
	}

	artifactPath := filepath.Join(root, "build", "build-order", "widget.json")
	tamper(t, artifactPath, `"phase": "schema"`, `"phase": "Schema"`)
	if err := os.Remove(viewer); err != nil {
		t.Fatalf("remove viewer: %v", err)
	}

	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil {
		t.Fatal("check must fail on the hand-edited artifact (content drift)")
	}
	var found []string
	for _, w := range env.Warnings {
		if strings.Contains(w, "is not drawn in the viewer's Build order tab") {
			found = append(found, w)
		}
	}
	if len(found) != 1 {
		t.Fatalf("warnings[] carries %d build-order lines, want exactly 1: %v", len(found), env.Warnings)
	}
	for _, want := range []string{`build order for module "widget"`, `"Schema" is not a phase`, "dossierx build-order propose --module widget"} {
		if !strings.Contains(found[0], want) {
			t.Errorf("warning %q lacks %q", found[0], want)
		}
	}
	after, err := os.ReadFile(viewer)
	if err != nil {
		t.Fatalf("the viewer must still be written when one module's order cannot be drawn: %v", err)
	}
	if strings.Contains(string(after), `id="dossierx-build-order-widget"`) {
		t.Error("the undrawable order must not appear in the tab")
	}
	if !strings.Contains(string(after), "widget.contract.alpha") {
		t.Error("widget's ordinary claims must still render")
	}
}
