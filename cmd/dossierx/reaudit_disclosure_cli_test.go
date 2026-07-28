// reaudit_disclosure_cli_test.go pins the CLI half of the reaudit disclosure
// contract: internal/reaudit computes Diff.ResultingBody and Diff.SideEffects
// precisely so a human is never asked to approve a body they were not shown,
// and those two fields have to survive the trip through cmd/dossierx or the
// engine's honesty is decorative.
//
// The defect this file exists for: reauditData declared neither field, so a
// confirmed flag-sourced reaudit on a multi-line claim deleted every line the
// flag never mentioned, and all three preview surfaces (--dry-run, the bare
// preview, the confirm envelope) said nothing about it. data.body carried the
// RENDERED diff — <mark style=...> spans — which is not what Apply writes, so
// even an agent reading the payload closely could not see the loss.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// disclosureFixture writes a project holding one LOCKED claim whose body is
// three lines, and returns the config path and the claim's path.
//
// Three lines, and a flag that names only the first, is the whole point: the
// rendered diff shows one red span and one green span — a phrase-level edit —
// while Apply replaces the entire body. Two lines vanish that nothing in the
// diff mentions.
func disclosureFixture(t *testing.T) (cfgPath, claimPath string) {
	t.Helper()
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath = filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(parityConfig), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	claimPath = filepath.Join(claimsDir, "main.yaml")
	claim := "id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\nbuild_role: behavior\n" +
		"body: |\n" +
		"  the retry policy allows two attempts.\n" +
		"  Backoff is exponential, starting at 200ms.\n" +
		"  A dead-letter queue receives whatever still fails.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}
	armLedgerFixture(t, cfgPath)
	return cfgPath, claimPath
}

// armDisclosureFlag flags the fixture claim, naming ONLY its first line.
func armDisclosureFlag(t *testing.T, cfgPath string) {
	t.Helper()
	if _, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", "widget.contract.main",
		"--claim-says", "the retry policy allows two attempts.",
		"--now-does", "the retry policy allows five attempts.",
		"--reason", "the code was changed to five"); err != nil {
		t.Fatalf("claim flag: %v", err)
	}
}

// A confirmed `claim reaudit` REPLACES the body with --now-does; that is
// deliberate (see ProposeFlagDiff) and is not what this test is about. What it
// is about is that the human agreeing to it has to be able to SEE it, in every
// surface the agent could show them, before the write happens.
func TestReauditDisclosesTheWholeBodyItWillWrite(t *testing.T) {
	cfgPath, claimPath := disclosureFixture(t)
	armDisclosureFlag(t, cfgPath)

	// 1. The machine-readable preview.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
	joined := strings.Join(dr.SideEffects, "\n")
	if !strings.Contains(joined, "replaces the claim's entire body") {
		t.Fatalf("the dry run must disclose the whole-body replacement in side_effects, got %#v", dr.SideEffects)
	}
	if !strings.Contains(joined, "2 other line(s)") {
		t.Fatalf("the dry run must say HOW MUCH of the current body is dropped, got %#v", dr.SideEffects)
	}
	resulting, ok := dr.Proposed["resulting_body"].(string)
	if !ok || !strings.Contains(resulting, "five attempts") {
		t.Fatalf("the dry run must propose the EXACT body a confirm would write, got %#v", dr.Proposed)
	}
	if strings.Contains(resulting, "dead-letter queue") {
		t.Fatalf("the proposed resulting body must be what Apply writes, not the current body: %q", resulting)
	}

	// 2. The bare preview envelope — the surface the skills tell an agent to
	//    show its human before asking for the approving words.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
	if err != nil {
		t.Fatalf("reaudit preview: %v", err)
	}
	var preview reauditData
	envData(t, env, &preview)
	assertDisclosed(t, "preview", preview)

	// 3. The confirm envelope — same proposal, applied. An agent that showed
	//    the human the preview and then re-shows what happened must find the
	//    same two fields, or the record of what was approved is incomplete.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main",
		"--confirm", "--reason", "the human read the resulting body and said yes")
	if err != nil {
		t.Fatalf("reaudit --confirm: %v", err)
	}
	var applied reauditData
	envData(t, env, &applied)
	if !applied.Applied {
		t.Fatalf("expected the confirm envelope to report applied, got %+v", applied)
	}
	assertDisclosed(t, "confirm", applied)

	// And the disclosure was true: the file on disk is exactly resulting_body.
	onDisk, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if strings.Contains(string(onDisk), "dead-letter queue") {
		t.Fatalf("fixture precondition changed: the confirm was supposed to drop the other lines:\n%s", onDisk)
	}
	if !strings.Contains(string(onDisk), strings.TrimSpace(applied.ResultingBody)) {
		t.Fatalf("resulting_body must be the text Apply actually wrote, got %q for:\n%s", applied.ResultingBody, onDisk)
	}
}

// assertDisclosed is the per-envelope half of the contract, run against both
// the preview and the confirm payload because they carry the SAME proposal and
// an agent may show either one.
func assertDisclosed(t *testing.T, surface string, d reauditData) {
	t.Helper()
	if d.ResultingBody == "" {
		t.Fatalf("%s: the envelope must carry resulting_body — the exact text a confirm writes: %+v", surface, d)
	}
	if !strings.Contains(d.ResultingBody, "five attempts") {
		t.Fatalf("%s: resulting_body must be the collapsed body, got %q", surface, d.ResultingBody)
	}
	if strings.Contains(d.ResultingBody, "<mark") {
		t.Fatalf("%s: resulting_body is plain text, not the rendered diff, got %q", surface, d.ResultingBody)
	}
	if len(d.SideEffects) == 0 {
		t.Fatalf("%s: the envelope must carry side_effects naming the whole-body replacement: %+v", surface, d)
	}
	if !strings.Contains(strings.Join(d.SideEffects, "\n"), "2 other line(s)") {
		t.Fatalf("%s: side_effects must say how much of the body is dropped, got %#v", surface, d.SideEffects)
	}
	// The rendered diff is still carried, under a name that says what it is.
	if !strings.Contains(d.BodyDiffHTML, "<mark") {
		t.Fatalf("%s: body_diff_html must still carry the rendered diff, got %q", surface, d.BodyDiffHTML)
	}
}

// TestReauditTextRendersTheResultingBodyAndTheLoss pins the human surface.
//
// --format text is what an agent pastes into chat when it asks for the yes, so
// a disclosure that exists only in JSON is a disclosure the human never reads.
func TestReauditTextRendersTheResultingBodyAndTheLoss(t *testing.T) {
	cfgPath, _ := disclosureFixture(t)
	armDisclosureFlag(t, cfgPath)

	out, _, err := execCLI(t, "--config", cfgPath, "claim", "reaudit", "widget.contract.main")
	if err != nil {
		t.Fatalf("reaudit preview: %v", err)
	}
	if !strings.Contains(out, "resulting body") {
		t.Fatalf("the text preview must show the body a confirm would write, got:\n%s", out)
	}
	if !strings.Contains(out, "the retry policy allows five attempts.") {
		t.Fatalf("the text preview must print the resulting body itself, got:\n%s", out)
	}
	if !strings.Contains(out, "replaces the claim's entire body") {
		t.Fatalf("the text preview must name the side effect, got:\n%s", out)
	}
}
