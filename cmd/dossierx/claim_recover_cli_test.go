// claim_recover_cli_test.go drives `dossierx claim recover-approved-content`
// end to end, through the real verbs, in a real git work tree.
//
// The fixture deliberately does not hand-build a ledger. It LOCKS the claim
// with `claim lock`, commits, UNLOCKS it with `claim unlock`, rewrites it, and
// only then strips the `content` key from the record — which is precisely the
// shape a project that locked under v0.7.17 has on disk today. Building the
// record by hand would let the test pass against a ledger the engine never
// writes.
package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// recoverFixture is a git work tree holding a project with one claim that was
// approved, committed, unlocked and rewritten, whose ledger record carries no
// wording. It returns the config path and the approved body.
func recoverFixture(t *testing.T) (cfgPath, approvedBody string) {
	t.Helper()
	return recoverFixtureIn(t, true)
}

// recoverFixtureIn builds the same project with or without a git work tree
// around it. The no-git variant is not a degenerate case: it is a project
// that HAS something to recover and no history to recover it from, which is
// the only state in which the verb's git refusal can be reached at all. With
// nothing eligible the verb never asks git anything, and rightly so — a
// tarball checkout with no approvals to complete must not fail.
func recoverFixtureIn(t *testing.T, useGit bool) (cfgPath, approvedBody string) {
	t.Helper()
	if useGit {
		if _, err := exec.LookPath("git"); err != nil {
			t.Fatal("git is required for approved-content recovery; install git")
		}
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if !useGit {
			return
		}
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init")
	git("config", "user.email", "fixture@example.com")
	git("config", "user.name", "Fixture")

	approvedBody = "The approved wording.\n"
	claim := "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  " + strings.TrimSuffix(approvedBody, "\n") + "\n" +
		""
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(parityConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	claimPath := filepath.Join(root, "claims", "one.yaml")
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock",
		"widget.contract.one", "--reason", "reviewed"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	// The approved revision has to be IN HISTORY for recovery to find it.
	git("add", "-A")
	git("commit", "-m", "approve the claim")

	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "unlock",
		"widget.contract.one", "--reason", "rewording"); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	rewritten := strings.Replace(claim, "The approved wording.", "The current wording.", 1)
	rewritten = strings.Replace(rewritten, "status: locked", "status: draft", 1)
	if err := os.WriteFile(claimPath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	stripLedgerContent(t, cfgPath)
	return cfgPath, approvedBody
}

// stripLedgerContent removes the `content` key from every ledger record,
// turning an engine-written store into the legacy one recovery exists for.
func stripLedgerContent(t *testing.T, cfgPath string) {
	t.Helper()
	path := storePathForTest(t, cfgPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var store map[string]any
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatal(err)
	}
	ledger, isMap := store["ledger"].(map[string]any)
	if !isMap {
		t.Fatal("the lock store has no ledger object")
	}
	if len(ledger) == 0 {
		t.Fatal("fixture ledger is empty; the lock did not record anything")
	}
	stripped := 0
	for _, entry := range ledger {
		record, isRecord := entry.(map[string]any)
		if !isRecord {
			t.Fatalf("ledger entry is not an object: %T", entry)
		}
		if _, ok := record["content"]; ok {
			delete(record, "content")
			stripped++
		}
	}
	if stripped == 0 {
		t.Fatal("fixture ledger carried no content to strip; this test would prove nothing")
	}
	out, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// recoverEnvelope is the command's data, decoded.
type recoverEnvelope struct {
	Eligible        int      `json:"eligible"`
	AlreadyRetained int      `json:"already_retained"`
	Written         []string `json:"written"`
	Recovered       []struct {
		ClaimID   string `json:"claim_id"`
		Commit    string `json:"commit"`
		Revisions int    `json:"revisions"`
	} `json:"recovered"`
	Unrecovered []struct {
		ClaimID string `json:"claim_id"`
		Reason  string `json:"reason"`
	} `json:"unrecovered"`
	Missing []string `json:"missing"`
}

// A dry run must report exactly what the write would do and touch NOTHING.
// This is the product's dry-run rule, and for this verb it is what lets a
// human decide whether to run it at all.
func TestClaimRecoverDryRunWritesNothing(t *testing.T) {
	cfgPath, _ := recoverFixture(t)
	storePath := storePathForTest(t, cfgPath)
	before, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content", "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	var data recoverEnvelope
	envData(t, env, &data)
	if data.Eligible != 1 || len(data.Recovered) != 1 {
		t.Fatalf("want 1 eligible and 1 recovered, got %d/%d", data.Eligible, len(data.Recovered))
	}
	if data.Recovered[0].Commit == "" {
		t.Fatal("a recovered outcome must name the commit it was found in")
	}
	if len(data.Written) != 0 {
		t.Fatalf("a dry run must write nothing, reported %v", data.Written)
	}
	if len(data.Missing) != 1 || data.Missing[0] != "--reason" {
		t.Fatalf("a dry run with no --reason must name it as missing, got %v", data.Missing)
	}

	after, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("a dry run must leave the lock store byte-identical")
	}
}

// The write requires the human's own approving words, like every other verb
// that touches the lock store.
func TestClaimRecoverRequiresAReason(t *testing.T) {
	cfgPath, _ := recoverFixture(t)
	storePath := storePathForTest(t, cfgPath)
	before, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content")
	if err == nil {
		t.Fatal("recovering without --reason must refuse")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeMissingFlag {
		t.Fatalf("want a missing_flag refusal, got %+v", env.Error)
	}
	after, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("a refused run must write nothing")
	}
}

// The write itself: the wording goes onto the record, the approval does not
// move, and the claim file is not touched.
func TestClaimRecoverWritesTheApprovedWordingAndNothingElse(t *testing.T) {
	cfgPath, approvedBody := recoverFixture(t)
	storePath := storePathForTest(t, cfgPath)
	rawBefore, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	claimPath := filepath.Join(filepath.Dir(cfgPath), "claims", "one.yaml")
	claimBefore, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content",
		"--reason", "recovering the wording these approvals signed")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	var data recoverEnvelope
	envData(t, env, &data)
	if len(data.Written) != 1 || data.Written[0] != "widget.contract.one" {
		t.Fatalf("want the one claim written, got %v", data.Written)
	}

	var before, after map[string]any
	if err := json.Unmarshal(rawBefore, &before); err != nil {
		t.Fatal(err)
	}
	rawAfter, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rawAfter, &after); err != nil {
		t.Fatal(err)
	}

	recordBefore := ledgerRecord(t, before, "widget.contract.one")
	recordAfter := ledgerRecord(t, after, "widget.contract.one")
	content, ok := recordAfter["content"].(map[string]any)
	if !ok {
		t.Fatal("the record must carry the recovered wording")
	}
	if got := content["Body"]; got != approvedBody {
		t.Fatalf("recovered body %q, want %q", got, approvedBody)
	}
	// Everything the record said before must still say the same thing. The
	// approval is completed, never rewritten.
	delete(recordAfter, "content")
	for key, want := range recordBefore {
		if got := recordAfter[key]; got != want {
			t.Fatalf("recovery moved %q: %v -> %v", key, want, got)
		}
	}
	if len(recordAfter) != len(recordBefore) {
		t.Fatalf("recovery added or dropped a field: before %v after %v", recordBefore, recordAfter)
	}
	// And the rest of the store is untouched.
	for _, key := range []string{"version", "hashes", "locked_at"} {
		gotBefore, err := json.Marshal(before[key])
		if err != nil {
			t.Fatal(err)
		}
		gotAfter, err := json.Marshal(after[key])
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBefore) != string(gotAfter) {
			t.Fatalf("recovery moved store key %q", key)
		}
	}

	claimAfter, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(claimBefore) != string(claimAfter) {
		t.Fatal("recovery must not touch the claim file")
	}
}

// Running it twice must be safe and must say so: the second run has nothing
// eligible, because the first one retained the wording.
func TestClaimRecoverIsIdempotent(t *testing.T) {
	cfgPath, _ := recoverFixture(t)
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content",
		"--reason", "first run"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	storePath := storePathForTest(t, cfgPath)
	after, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content",
		"--reason", "second run")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	var data recoverEnvelope
	envData(t, env, &data)
	if data.Eligible != 0 || data.AlreadyRetained != 1 {
		t.Fatalf("want 0 eligible and 1 already retained on the second run, got %d/%d",
			data.Eligible, data.AlreadyRetained)
	}
	if len(data.Written) != 0 {
		t.Fatalf("the second run must write nothing, wrote %v", data.Written)
	}
	again, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(again) {
		t.Fatal("a second run with nothing to do must leave the store byte-identical")
	}
}

// Outside a git work tree the verb must refuse with its own code, so an agent
// can tell "git cannot answer" from "nothing was recoverable".
func TestClaimRecoverRefusesWithoutAWorkTree(t *testing.T) {
	cfgPath, _ := recoverFixtureIn(t, false)
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content", "--dry-run")
	if err == nil {
		t.Fatal("outside a work tree the verb must refuse")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeGitUnavailable {
		t.Fatalf("want a git_unavailable refusal, got %+v", env.Error)
	}
}

// ...and with nothing eligible, no work tree is needed. A project that has
// already been swept, or never locked anything, must not start failing
// because it is built from a tarball.
func TestClaimRecoverNeedsNoWorkTreeWhenNothingIsEligible(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/one.yaml": "id: widget.contract.one\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  fixture.\n" +
			"",
	})
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "recover-approved-content", "--dry-run")
	if err != nil {
		t.Fatalf("with nothing eligible the verb must succeed without git: %v", err)
	}
	var data recoverEnvelope
	envData(t, env, &data)
	if data.Eligible != 0 || len(data.Recovered) != 0 {
		t.Fatalf("want nothing eligible, got %d eligible and %d recovered", data.Eligible, len(data.Recovered))
	}
}

// The text form must NAME the claims it could not recover. Those are exactly
// the claims whose readers keep seeing "the approved wording was not kept",
// and a count alone gives nobody a way to find them.
func TestClaimRecoverTextNamesUnrecoveredClaims(t *testing.T) {
	cfgPath, _ := recoverFixture(t)
	// Move the signed hash so no revision can match, without touching
	// anything else about the record.
	storePath := storePathForTest(t, cfgPath)
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	var store map[string]any
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatal(err)
	}
	record := ledgerRecord(t, store, "widget.contract.one")
	record["hash"] = strings.Repeat("0", 64)
	out, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storePath, out, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execCLI(t, "--config", cfgPath, "claim", "recover-approved-content", "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !strings.Contains(stdout, "widget.contract.one") {
		t.Fatalf("the report must name the unrecovered claim:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no revision") {
		t.Fatalf("the report must say why:\n%s", stdout)
	}
	if !strings.Contains(stdout, "keep reporting that the approved wording was not retained") {
		t.Fatalf("the report must say what an unrecovered claim still shows a reader:\n%s", stdout)
	}
}

// ledgerRecord reads one claim's ledger record out of a decoded lock store,
// failing rather than panicking when the shape is not what it expects.
func ledgerRecord(t *testing.T, store map[string]any, claimID string) map[string]any {
	t.Helper()
	ledger, ok := store["ledger"].(map[string]any)
	if !ok {
		t.Fatal("the lock store has no ledger object")
	}
	record, ok := ledger[claimID].(map[string]any)
	if !ok {
		t.Fatalf("the ledger has no record for %s", claimID)
	}
	return record
}
