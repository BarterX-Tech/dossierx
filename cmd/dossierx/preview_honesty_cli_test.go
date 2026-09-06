// preview_honesty_cli_test.go covers four smaller round-5 findings that share a
// theme: a surface that answered a question with something other than the truth
// about itself. `serve` failing silently on stdout, `claim new` promising a
// lint-clean claim and writing a lint-error one, `check --validate` hiding
// integrity findings behind an unrelated lint failure, and dry-run preconditions
// whose detail sentence was written for the branch that did not happen.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

// ---------------------------------------------------------------------
// serve answers on the failure path
// ---------------------------------------------------------------------

// serve is the one command exempt from the one-envelope-per-invocation contract,
// and it has a real reason to be: its useful output (the URL) has to appear
// before it blocks. The exemption was applied to the FAILURE path too, where it
// bought nothing — a serve that cannot find a config wrote NOTHING to stdout and
// exited non-zero, which is precisely the outcome the machine contract exists to
// prevent.
func TestServeFailureEmitsAnEnvelope(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nowhere", "project.config.yaml")

	env, _, err := execReviewedCLIJSON(t, "--config", missing, "serve")
	if err == nil {
		t.Fatalf("serve must fail when its config does not exist")
	}
	if env.OK {
		t.Fatalf("the failure envelope must report ok:false, got %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeConfigNotFound {
		t.Fatalf("serve's failure must classify like every other command's, got %+v", env.Error)
	}
	if env.Command != "serve" {
		t.Fatalf("the envelope must name the command that failed, got %q", env.Command)
	}
}

// TestServeFailureUnderTextIsUnchanged: the exemption still holds where it was
// justified. --format text gets the same "Error: <msg>" line it always did, and
// no JSON leaks into a human's terminal.
func TestServeFailureUnderTextIsUnchanged(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nowhere", "project.config.yaml")

	stdout, stderr, err := execReviewedCLI(t, "--config", missing, "serve")
	if err == nil {
		t.Fatalf("serve must fail when its config does not exist")
	}
	if strings.Contains(stdout, "{") {
		t.Fatalf("--format text must not emit an envelope, got: %q", stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("--format text must keep the prose error line, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------
// claim new keeps its own promise in the reserved overview facet
// ---------------------------------------------------------------------

// `claim new`'s help text promises "the claim it writes is shaped to pass the
// lint suite immediately". Under the one RESERVED facet it did the opposite,
// every time: a claim in `overview` IS an orientation note (the facet name is
// what makes it one), orientation-note-shape requires layout: banner, and the
// --layout default is card. The command wrote the file and then reported, in the
// same call, the lint error it had just created.
func TestClaimNewInTheOverviewFacetIsLintClean(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "new", "widget.overview.router",
		"--body", "read the contract claims below in order.",
		"--governed-reason", "an orientation note is not backed by doctrine")
	if err != nil {
		t.Fatalf("claim new: %v", err)
	}
	var data claimNewData
	envData(t, env, &data)
	if data.LintErrorCount != 0 {
		t.Fatalf("claim new promises a lint-clean claim; it wrote one with %d error(s): %+v", data.LintErrorCount, data)
	}
	if data.Layout != "banner" {
		t.Fatalf("an overview-facet claim must default to layout: banner, got %q", data.Layout)
	}

	// The file on disk agrees, and the read-only gate confirms it.
	written, readErr := os.ReadFile(data.Path)
	if readErr != nil {
		t.Fatalf("read the written claim: %v", readErr)
	}
	if !strings.Contains(string(written), "layout: banner") {
		t.Fatalf("the written claim must carry the banner layout:\n%s", written)
	}
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "check", "--validate"); err != nil {
		t.Fatalf("the project must still validate after claim new: %v", err)
	}
}

// TestClaimNewHonoursAnExplicitLayoutInTheOverviewFacet: the default moves, the
// caller's choice does not. A caller who names a layout is making a decision,
// and the lint suite — reported in the same call — is where a wrong one is
// answered.
func TestClaimNewHonoursAnExplicitLayoutInTheOverviewFacet(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	env, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "new", "widget.overview.explicit",
		"--layout", "card",
		"--body", "deliberately not a banner.",
		"--governed-reason", "fixture")
	if err != nil {
		t.Fatalf("claim new: %v", err)
	}
	var data claimNewData
	envData(t, env, &data)
	if data.Layout != "card" {
		t.Fatalf("an explicit --layout must win, got %q", data.Layout)
	}
}

// ---------------------------------------------------------------------
// check --validate reports the ledger gate even when lint failed
// ---------------------------------------------------------------------

// The ledger block sat behind a `if !res.OK` return, and res.OK is lint-driven.
// So a project with one unrelated lint error printed no integrity findings at
// all, while `check` and `check --staged` printed both — the same command, the
// same tree, three different answers about whether a locked claim had been
// tampered with, decided by something unrelated.
func TestValidateTextPrintsLedgerFindingsAlongsideLintErrors(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeCheckFixture(t, root, parityConfig, map[string]string{
		"claims/locked.yaml": "id: widget.contract.locked\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
			"build_role: schema\n" +
			"body: |\n  the approved body.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
	})
	// A hand edit to a LOCKED claim, after the ledger recorded it: the drift the
	// gate exists to catch.
	claimsDir := filepath.Join(root, "claims")
	tamper(t, filepath.Join(claimsDir, "locked.yaml"), "the approved body.", "a body nobody approved.")
	// And, entirely separately, a draft claim that does not lint.
	broken := "id: widget.contract.broken\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  rests on nothing that exists.\n" +
		"rests_on:\n  - widget.contract.does-not-exist\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(filepath.Join(claimsDir, "broken.yaml"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write broken claim: %v", err)
	}

	out, _, err := execReviewedCLI(t, "--config", cfgPath, "check", "--validate")
	if err == nil {
		t.Fatalf("a project with a lint error and a tampered locked claim must fail")
	}
	if !strings.Contains(out, "[lint]") && !strings.Contains(out, "error(s)") {
		t.Fatalf("expected the lint findings reported, got:\n%s", out)
	}
	if !strings.Contains(out, "[ledger]") {
		t.Fatalf("--validate must report ledger findings even when lint failed; check and check --staged both do:\n%s", out)
	}
	if !strings.Contains(out, "widget.contract.locked") {
		t.Fatalf("the ledger block must name the tampered claim:\n%s", out)
	}
}

// ---------------------------------------------------------------------
// a passing precondition's detail describes the pass
// ---------------------------------------------------------------------

// cliout.Precondition.Detail is emitted verbatim on BOTH branches — reporting
// passing gates is the point, since the passes are the evidence — and several
// call sites had written theirs for the failing branch only. The preview a human
// is shown to get a yes therefore printed "[ok] id_is_unused: a claim with this
// id already exists".
func TestDryRunDetailsDescribeTheVerdictTheyStandNextTo(t *testing.T) {
	root := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, root, "widget")

	// claim new, on an id nothing has taken.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "new", "widget.contract.fresh",
		"--body", "a new claim.", "--governed-reason", "fixture")
	assertPassingDetailsDoNotContradict(t, "claim new", dr.Preconditions, map[string]string{
		"id_is_unused":   "already exists",
		"file_is_unused": "already exists",
	})

	// comment add, on a claim that accepts comments with a storable body.
	dr = dryRunOf(t, "--config", cfgPath, "comment", "add", "widget.contract.overview",
		"--as", "agent", "--body", "a perfectly storable body")
	assertPassingDetailsDoNotContradict(t, "comment add", dr.Preconditions, map[string]string{
		"body_is_storable": "cannot be stored",
	})

	// claim flag, on a locked body-only claim: claim_is_body_only passes, and its
	// detail must not read as the structured-layout refusal.
	if _, _, err := execReviewedCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.overview", "--reason", "approved"); err != nil {
		t.Fatalf("lock: %v", err)
	}
	dr = dryRunOf(t, "--config", cfgPath, "claim", "flag", "widget.contract.overview",
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	assertPassingDetailsDoNotContradict(t, "claim flag", dr.Preconditions, map[string]string{
		"claim_is_body_only": "can only rewrite body",
	})
}

// assertPassingDetailsDoNotContradict checks that each named precondition
// passed, and that its detail does not carry the phrase that only makes sense
// when it failed.
func assertPassingDetailsDoNotContradict(t *testing.T, verb string, preconditions []cliout.Precondition, banned map[string]string) {
	t.Helper()
	seen := map[string]bool{}
	for _, p := range preconditions {
		phrase, watched := banned[p.Name]
		if !watched {
			continue
		}
		seen[p.Name] = true
		if !p.OK {
			t.Fatalf("%s: fixture precondition: %q was supposed to pass, got %+v", verb, p.Name, p)
		}
		if strings.Contains(p.Detail, phrase) {
			t.Fatalf("%s: a passing %q reports %q — the detail was written for the failing branch", verb, p.Name, p.Detail)
		}
		if p.Detail == "" {
			t.Fatalf("%s: %q must still explain its verdict", verb, p.Name)
		}
	}
	for name := range banned {
		if !seen[name] {
			t.Fatalf("%s: expected precondition %q in the preview, got %+v", verb, name, preconditions)
		}
	}
}
