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
// path, a confirmed reaudit and plain check refuse CONSTITUTION_NOT_LOCKED while the constitution
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
	state, ok := details["state"].(string)
	if !ok {
		t.Fatalf("error.details.state must be a string, got %#v", details["state"])
	}
	return state
}

// takeJSON keeps errcheck honest on helpers that return (envelope, stderr, err)
// when the test cares about the envelope only. The three results are the only
// arguments so Go will unpack them.
func takeJSON(env cliout.Envelope, stderr string, err error) cliout.Envelope {
	if err != nil {
		return env
	}
	if strings.TrimSpace(stderr) == "" {
		return env
	}
	return env
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
		{"claim", "new", "widget.contract.third", "--summary", "Fixture claim used by the engine test corpus.", "--body", "drafted under an unlocked roof", "--rests-on", "widget.contract.overview"},
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
	env = takeJSON(execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go"))
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
	env = takeJSON(execCLIJSON(t, "--config", cfgPath, "check", "--validate"))
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
	env = takeJSON(execCLIJSON(t, "--config", cfgPath, "check"))
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || roofState(t, env) != string(constitution.StateEdited) {
		t.Fatalf("an edited roof must refuse with state edited, got %+v", env.Error)
	}
	env = takeJSON(execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go"))
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
	env := takeJSON(execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "too long"))
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionOverCap {
		t.Fatalf("constitution lock over the cap must refuse %s, got %+v", cliout.CodeConstitutionOverCap, env.Error)
	}
	env = takeJSON(execCLIJSON(t, "--config", cfgPath, "check"))
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionOverCap {
		t.Fatalf("check over the cap must refuse %s, got %+v", cliout.CodeConstitutionOverCap, env.Error)
	}
	env = takeJSON(execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "go"))
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
	internals := "id: widget.internals.detail\nsummary: An internal fact.\nfacet: internals\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  an internal fact.\n" +
		"rests_on:\n  - widget.contract.overview\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "detail.yaml"), []byte(internals), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "new", "project.retention", "--summary", "Fixture claim used by the engine test corpus.", "--body", "Data is kept for thirty days.\nLonger on request.", "--rests-on", "widget.contract.overview")
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
	env = takeJSON(execCLIJSON(t, "--config", cfgPath, "check", "--validate"))
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
	env := takeJSON(execCLIJSON(t, "--config", cfgPath, "check", "--validate"))
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
		t.Fatalf("--validate reads the worktree and must see the edit: %+v", env.Error)
	}
	env, _, err = execCLIJSON(t, "--config", cfgPath, "check", "--staged")
	if err != nil || !env.OK {
		t.Fatalf("--staged reads the index, where the roof is still locked: %v %+v", err, env.Error)
	}

	// Stage the edit alone: now the commit carries an edited roof.
	stagedGit(t, root, "add", "constitution.yaml")
	env = takeJSON(execCLIJSON(t, "--config", cfgPath, "check", "--staged"))
	if env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked || env.StoppedAt != "constitution" {
		t.Fatalf("--staged must refuse the staged edit at constitution: %+v stopped_at=%q", env.Error, env.StoppedAt)
	}
	var data checkData
	envData(t, env, &data)
	if !data.Staged || !data.ReadOnly {
		t.Fatalf("staged data = %+v", data)
	}
}

// reviewPendingFixture is icWriteFixtureProject with its one claim locked and
// then flagged: the locked+review_pending state a reaudit is about. The roof
// is locked when this returns; the caller unroofs it.
func reviewPendingFixture(t *testing.T) (root, cfgPath, claimPath, id string) {
	t.Helper()
	root = t.TempDir()
	cfgPath, claimPath = icWriteFixtureProject(t, root, "widget")
	id = "widget.contract.overview"
	if env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--reason", "approved"); err != nil || !env.OK {
		t.Fatalf("lock: %v %+v", err, env.Error)
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "it retries twice", "--now-does", "it retries five times", "--reason", "code changed")
	if err != nil || !env.OK {
		t.Fatalf("flag: %v %+v", err, env.Error)
	}
	return root, cfgPath, claimPath, id
}

// A confirmed reaudit writes an approval to the lock ledger, so it is module
// work and refuses at the roof gate exactly as `claim lock` does: the same
// code, the same details.state, the same hint, from the same helper. The bare
// preview and the dry run stay open — the dry run names the roof as a failing
// precondition — and nothing is written by a refused confirm.
func TestClaimReauditConfirmRefusesWhileTheRoofIsNotLocked(t *testing.T) {
	root, cfgPath, claimPath, id := reviewPendingFixture(t)
	roof := filepath.Join(root, "constitution.yaml")
	second := "id: widget.contract.second\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a second draft, for the lock refusal to compare against.\n" +
		"rests_on:\n  - widget.contract.overview\n"
	if err := os.WriteFile(filepath.Join(root, "claims", "second.yaml"), []byte(second), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "review_pending: true") {
		t.Fatalf("fixture claim must be review_pending:\n%s", before)
	}
	storeBefore, err := os.ReadFile(storePathForTest(t, cfgPath))
	if err != nil {
		t.Fatal(err)
	}

	refused := func(state constitution.State) {
		t.Helper()
		env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "re-read, still true")
		if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeConstitutionNotLocked {
			t.Fatalf("reaudit --confirm under a %s roof must refuse %s, got %+v", state, cliout.CodeConstitutionNotLocked, env.Error)
		}
		if got := roofState(t, env); got != string(state) {
			t.Fatalf("details.state = %q, want %s", got, state)
		}
		if env.Error.Hint == "" || !strings.HasPrefix(env.Error.Message, "reaudit: refused") {
			t.Fatalf("the refusal must name the verb and carry the recovery hint: %+v", env.Error)
		}
		// The same refusal `claim lock` makes, from the same helper: code,
		// state and hint agree; only the verb prefix differs.
		lockEnv := takeJSON(execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.second", "--reason", "go"))
		if lockEnv.Error == nil || lockEnv.Error.Code != env.Error.Code || roofState(t, lockEnv) != roofState(t, env) || lockEnv.Error.Hint != env.Error.Hint {
			t.Fatalf("reaudit and lock must refuse identically under a %s roof:\nreaudit=%+v\nlock=%+v", state, env.Error, lockEnv.Error)
		}
		if strings.TrimPrefix(lockEnv.Error.Message, "lock:") != strings.TrimPrefix(env.Error.Message, "reaudit:") {
			t.Fatalf("the two refusals must differ only in the verb:\nreaudit=%q\nlock=%q", env.Error.Message, lockEnv.Error.Message)
		}

		// A refused confirm writes nothing: the claim and the ledger are as
		// they were, and the flag still waits.
		after, err := os.ReadFile(claimPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatalf("a refused reaudit must not touch the claim:\n%s", after)
		}
		storeAfter, err := os.ReadFile(storePathForTest(t, cfgPath))
		if err != nil {
			t.Fatal(err)
		}
		if string(storeAfter) != string(storeBefore) {
			t.Fatalf("a refused reaudit must not touch the ledger:\n%s", storeAfter)
		}

		// The preview stays open: it is how the agent shows the human what a
		// confirm would write.
		env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id)
		if err != nil || !env.OK {
			t.Fatalf("a bare reaudit is a preview and must keep working under a %s roof: %v %+v", state, err, env.Error)
		}
		var preview reauditData
		envData(t, env, &preview)
		if preview.Applied || preview.Trigger != "flag" || preview.ResultingBody == "" {
			t.Fatalf("preview = %+v", preview)
		}

		// The dry run names the roof and is blocked by it.
		dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "re-read, still true")
		found := false
		for _, pc := range dr.Preconditions {
			if pc.Name == "constitution_locked" {
				found = true
				if pc.OK {
					t.Fatalf("constitution_locked must fail under a %s roof: %+v", state, pc)
				}
			}
		}
		if !found || !dr.Blocked {
			t.Fatalf("the dry run must name constitution_locked and be blocked: %+v", dr)
		}
	}

	// (a) missing.
	if err := os.Remove(roof); err != nil {
		t.Fatal(err)
	}
	refused(constitution.StateMissing)

	// (b) draft.
	if err := os.WriteFile(roof, []byte(fixtureConstitutionYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	refused(constitution.StateDraft)

	// (c) edited after its lock.
	lockFixtureConstitution(t, cfgPath)
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
	storeBefore, err = os.ReadFile(storePathForTest(t, cfgPath))
	if err != nil {
		t.Fatal(err)
	}
	refused(constitution.StateEdited)

	// Locked by the human: the same confirm now applies.
	if env, _, err := execCLIJSON(t, "--config", cfgPath, "constitution", "lock", "--reason", "re-read and approved"); err != nil || !env.OK {
		t.Fatalf("re-lock: %v %+v", err, env.Error)
	}
	dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "re-read, still true")
	for _, pc := range dr.Preconditions {
		if pc.Name == "constitution_locked" && !pc.OK {
			t.Fatalf("constitution_locked must hold under a locked roof: %+v", pc)
		}
	}
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", id, "--confirm", "--reason", "re-read, still true")
	if err != nil || !env.OK {
		t.Fatalf("reaudit --confirm under a locked roof must apply: %v %+v", err, env.Error)
	}
	var applied reauditData
	envData(t, env, &applied)
	if !applied.Applied || applied.ReviewPending {
		t.Fatalf("a confirmed reaudit must apply and clear review_pending, got %+v", applied)
	}
	after, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "review_pending: true") || !strings.Contains(string(after), "it retries five times") {
		t.Fatalf("the confirm must have written the flagged wording and cleared review_pending:\n%s", after)
	}
}
