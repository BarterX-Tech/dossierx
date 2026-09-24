package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
)

// The roof gate (NIT-6 / NIT-26), end to end through the CLI: every claim-lock
// path and plain check refuse CONSTITUTION_NOT_LOCKED while the constitution
// is missing, draft, unrecorded or edited after its lock; the read-only modes
// carry it as an error finding; the drafting and reading verbs keep working;
// and `constitution lock` records the hash, re-locks an edited roof and
// refuses an unchanged one.

// unroofedProject is icWriteFixtureProject WITHOUT the roof: the fixture the
// gate exists to refuse.
func unroofedProject(t *testing.T) (root, cfgPath, claimPath string) {
	t.Helper()
	root = t.TempDir()
	cfgPath, claimPath = icWriteFixtureProject(t, root, "widget")
	for _, rel := range []string{"constitution.yaml", filepath.Join("build", "ledger")} {
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			t.Fatalf("unroof %s: %v", rel, err)
		}
	}
	return root, cfgPath, claimPath
}

func roofState(t *testing.T, env cliout.Envelope) string {
	t.Helper()
	details, ok := env.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("error.details must carry the verdict, got %#v", env.Error.Details)
	}
	state, _ := details["state"].(string)
	return state
}

func TestClaimLockRefusesEveryPathWhileTheRoofIsNotLocked(t *testing.T) {
	root, cfgPath, _ := unroofedProject(t)
	second := "id: widget.contract.second\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a second draft.\n" +
		"rests_on:\n  - widget.contract.overview\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "second.yaml"), []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}

	// Policy v1 is what a fresh project runs on: the set evaluator path.
	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go")
	if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("policy lock on an unroofed project must refuse %s, got %+v", cliout.CodeConstitutionNotLocked, env.Error)
	}
	if got := roofState(t, env); got != string(constitution.StateMissing) {
		t.Fatalf("details.state = %q, want missing", got)
	}
	if env.Error.Hint == "" {
		t.Fatal("the refusal must carry the recovery hint")
	}
	// Batch shape through the same evaluator.
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "widget.contract.second", "--reason", "go")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("batch lock must refuse %s, got %+v", cliout.CodeConstitutionNotLocked, env.Error)
	}
	// The preview says so too, as a failing precondition, and stays ok:true.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--dry-run")
	if err != nil || !env.OK {
		t.Fatalf("dry run must answer, got %v %+v", err, env)
	}
	var preview policyLockPreviewData
	envData(t, env, &preview)
	found := false
	for _, pc := range preview.Preconditions {
		if pc.Name == "constitution_locked" {
			found = true
			if pc.OK {
				t.Fatalf("constitution_locked must fail on an unroofed project: %+v", pc)
			}
		}
	}
	if !found || !preview.Blocked {
		t.Fatalf("the preview must name constitution_locked and be blocked: %+v", preview)
	}

	// Drafting and reading keep working.
	for _, args := range [][]string{
		{"claim", "show", "widget.contract.overview"},
		{"claim", "list"},
		{"claim", "new", "widget.contract.third", "--body", "drafted under an unlocked roof", "--rests-on", "widget.contract.overview"},
		{"constitution", "show"},
	} {
		if env, _, err := execCLIJSON(t, append([]string{"--config", cfgPath}, args...)...); err != nil || !env.OK {
			t.Fatalf("%v must keep working while the roof is unlocked: %v %+v", args, err, env.Error)
		}
	}

	// A draft roof is refused the same way, naming the state.
	if err := os.WriteFile(filepath.Join(root, "constitution.yaml"), []byte(fixtureConstitutionYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, _ = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || roofState(t, env) != string(constitution.StateDraft) {
		t.Fatalf("a draft roof must refuse with state draft, got %+v", env.Error)
	}

	// Locked by the human: the same lock now goes through.
	lockFixtureConstitution(t, cfgPath)
	env, _, err = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go")
	if err != nil || !env.OK {
		t.Fatalf("lock under a locked roof must succeed, got %v %+v", err, env.Error)
	}
}

func TestPlainCheckRefusesAtTheRoofGateAfterRegeneratingTheViewer(t *testing.T) {
	root, cfgPath, _ := unroofedProject(t)
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("check on an unroofed project must refuse %s, got %+v", cliout.CodeConstitutionNotLocked, env.Error)
	}
	if env.StoppedAt != "constitution" {
		t.Fatalf("stopped_at = %q, want constitution (a gate after the projections, not a lint stop)", env.StoppedAt)
	}
	for _, artifact := range []string{filepath.Join(root, "build", "catalog", "catalog.json"), filepath.Join(root, "build", "viewer", "index.html")} {
		if _, statErr := os.Stat(artifact); statErr != nil {
			t.Fatalf("the roof gate must not take the viewer offline: %s missing (%v)", artifact, statErr)
		}
	}
	var data checkData
	envData(t, env, &data)
	var roof []lintFindingData
	for _, f := range data.LintFindings {
		if f.Lint == "constitution-not-locked" {
			roof = append(roof, f)
		}
	}
	if len(roof) != 1 || roof[0].Severity != "error" || roof[0].ClaimID != "constitution" {
		t.Fatalf("expected one error-severity constitution-not-locked finding, got %+v", data.LintFindings)
	}
	if data.LintErrorCount != 1 {
		t.Fatalf("lint_error_count = %d, want 1 (the roof's finding counts)", data.LintErrorCount)
	}

	// --validate reports the same finding and the same code, at the same
	// position relative to the ledger (after it), and writes nothing.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || env.StoppedAt != "constitution" {
		t.Fatalf("check --validate must refuse %s at constitution, got %+v stopped_at=%q", cliout.CodeConstitutionNotLocked, env.Error, env.StoppedAt)
	}
	envData(t, env, &data)
	if !data.ReadOnly {
		t.Fatal("--validate must stay read-only")
	}

	// A claim lint error still wins the lint step: the roof is not the cause
	// of a dangling target.
	if err := os.WriteFile(filepath.Join(root, "claims", "dangling.yaml"), []byte("id: widget.contract.dangling\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  rests on nothing that exists.\nrests_on:\n  - widget.contract.ghost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, _ = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if env.Error == nil || env.Error.Code != cliout.CodeLintFailed || env.StoppedAt != "lint" {
		t.Fatalf("a claim lint error must still stop at lint with lint_failed, got %+v stopped_at=%q", env.Error, env.StoppedAt)
	}
}

func TestEditedAfterLockStopsWorkUntilReLockedAndUnchangedIsAlreadyLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	roof := filepath.Join(root, "constitution.yaml")

	// Locked and unchanged: a second lock is refused, nothing to sign.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "again")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeAlreadyLocked {
		t.Fatalf("re-locking an unchanged roof must refuse already_locked, got %+v", env.Error)
	}

	// The human edits the locked file by hand. check judges it edited: the
	// stored hash and the file's hash disagree.
	raw, err := os.ReadFile(roof)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "One roof", "One roof, amended", 1)
	if edited == string(raw) {
		t.Fatalf("fixture roof lacks the title to edit:\n%s", raw)
	}
	if err := os.WriteFile(roof, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, _ = execCLIJSON(t, "--config", cfgPath, "check")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || roofState(t, env) != string(constitution.StateEdited) {
		t.Fatalf("an edited roof must refuse with state edited, got %+v", env.Error)
	}
	env, _, _ = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("claim lock under an edited roof must refuse, got %+v", env.Error)
	}

	// constitution show tells the truth about it: words, hash, and the verdict.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "constitution", "show")
	if err != nil || !env.OK {
		t.Fatalf("constitution show must work on an edited roof: %v %+v", err, env.Error)
	}
	var show constitutionShowData
	envData(t, env, &show)
	if show.Lock.State != constitution.StateEdited || show.Lock.Hash == show.Lock.StoredHash || show.Text == "" || !strings.Contains(show.Text, "One roof, amended") {
		t.Fatalf("show = %+v", show)
	}
	if show.Digest.Words == 0 || show.Digest.WordCap != constitution.WordCap || len(show.Section) != 3 {
		t.Fatalf("digest/sections = %+v / %d", show.Digest, len(show.Section))
	}
	out, _, err := execCLI(t, "--config", cfgPath, "constitution", "show")
	if err != nil || !strings.Contains(out, "# Invariants") || !strings.Contains(out, "One roof, amended") || !strings.Contains(out, "edited after its lock") {
		t.Fatalf("text mode must print the words and the lock state, got: %s", out)
	}

	// Re-lock records the new hash; work resumes.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "re-read and approved")
	if err != nil || !env.OK {
		t.Fatalf("re-locking an edited roof must succeed: %v %+v", err, env.Error)
	}
	var lockData constitutionLockData
	envData(t, env, &lockData)
	if !lockData.Relock || lockData.Record.Reason != "re-read and approved" || lockData.Record.Hash != show.Lock.Hash {
		t.Fatalf("lock data = %+v", lockData)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("check must pass once the roof is re-locked: %v %+v", err, env.Error)
	}
}

func TestOverCapRefusesLockAndCheckAndNearCapWarns(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	roof := filepath.Join(root, "constitution.yaml")
	write := func(words int) {
		t.Helper()
		body := "status: draft\ninvariants:\n  - slug: long\n    title: Long\n    body: " + strings.Repeat("word ", words-1) + "\n"
		if err := os.WriteFile(roof, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(constitution.WordCap + 1)
	env, _, _ := execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "too long")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionOverCap {
		t.Fatalf("constitution lock over the cap must refuse %s, got %+v", cliout.CodeConstitutionOverCap, env.Error)
	}
	env, _, _ = execCLIJSON(t, "--config", cfgPath, "check")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionOverCap {
		t.Fatalf("check over the cap must refuse %s, got %+v", cliout.CodeConstitutionOverCap, env.Error)
	}
	env, _, _ = execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionOverCap {
		t.Fatalf("claim lock over the cap must refuse %s, got %+v", cliout.CodeConstitutionOverCap, env.Error)
	}

	write(constitution.NearCap)
	if env, _, err := execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "near the cap is fine"); err != nil || !env.OK {
		t.Fatalf("near-cap roof must lock: %v %+v", err, env.Error)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "check")
	if err != nil || !env.OK {
		t.Fatalf("check at near-cap must pass: %v %+v", err, env.Error)
	}
	warned := false
	for _, w := range env.Warnings {
		if strings.Contains(w, "constitution-near-cap") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected a constitution-near-cap warning, got %v", env.Warnings)
	}
}

func TestProjectClaimsLiveInTheirOwnStoreAndNeverRestOnInternals(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")
	internals := "id: widget.internals.detail\nfacet: internals\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  an internal fact.\n" +
		"rests_on:\n  - widget.contract.overview\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "detail.yaml"), []byte(internals), 0o644); err != nil {
		t.Fatal(err)
	}
	// icWriteFixtureProject's config declares only the contract facet.
	cfg, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte(strings.Replace(string(cfg), "facets:\n  - contract\n", "facets:\n  - contract\n  - internals\n", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "new", "project.retention", "--body", "Data is kept for thirty days.\nLonger on request.", "--rests-on", "widget.contract.overview")
	if err != nil || !env.OK {
		t.Fatalf("claim new project.<slug>: %v %+v", err, env.Error)
	}
	path := filepath.Join(root, "project-claims", "retention.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("project claim must be written under project-claims/: %v", err)
	}
	for _, want := range []string{"id: project.retention", "scope: project"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("project claim file lacks %q:\n%s", want, raw)
		}
	}
	for _, absent := range []string{"module:", "facet:"} {
		if strings.Contains(string(raw), absent) {
			t.Fatalf("project claim must carry no %s:\n%s", absent, raw)
		}
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if err != nil || !env.OK {
		t.Fatalf("a project claim resting on a contract claim is clean: %v %+v", err, env.Error)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "show", "project.retention")
	if err != nil || !env.OK {
		t.Fatalf("claim show project.<slug>: %v %+v", err, env.Error)
	}

	// Internals are never a project claim's target.
	if err := os.WriteFile(path, []byte(strings.Replace(string(raw), "widget.contract.overview", "widget.internals.detail", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, _ = execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	var data checkData
	envData(t, env, &data)
	hit := false
	for _, f := range data.LintFindings {
		if f.Lint == "rests-on-target" && f.ClaimID == "project.retention" {
			hit = true
		}
	}
	if env.OK || !hit {
		t.Fatalf("expected rests-on-target on the project claim, got %+v / %+v", env.Error, data.LintFindings)
	}
}

// --staged judges the INDEX's roof and record, not the worktree's: a
// constitution edited but not staged is not what the commit carries.
func TestCheckStagedJudgesTheIndexsRoof(t *testing.T) {
	cfgPath, root, _ := stagedProject(t)
	roof := filepath.Join(root, "constitution.yaml")
	raw, err := os.ReadFile(roof)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "One roof", "One roof, amended", 1)
	if edited == string(raw) {
		t.Fatalf("fixture roof lacks the title to edit:\n%s", raw)
	}
	if err := os.WriteFile(roof, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Worktree: edited. Index: the locked roof the fixture committed.
	env, _, _ := execCLIJSON(t, "--config", cfgPath, "check", "--validate")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("--validate reads the worktree and must see the edit: %+v", env.Error)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--staged")
	if err != nil || !env.OK {
		t.Fatalf("--staged reads the index, where the roof is still locked: %v %+v", err, env.Error)
	}

	// Stage the edit alone: now the commit carries an edited roof.
	stagedGit(t, root, "add", "constitution.yaml")
	env, _, _ = execCLIJSON(t, "--config", cfgPath, "check", "--staged")
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || env.StoppedAt != "constitution" {
		t.Fatalf("--staged must refuse the staged edit at constitution: %+v stopped_at=%q", env.Error, env.StoppedAt)
	}
	var data checkData
	envData(t, env, &data)
	if !data.Staged || !data.ReadOnly {
		t.Fatalf("staged data = %+v", data)
	}
}
