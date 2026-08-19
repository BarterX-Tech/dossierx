// bootstrap_test.go executes skills/dossierx/SKILL.md's "Bootstrap — setting
// DossierX up in a repo" sequence, in the documented order ("in this order —
// steps 2 and 3 are not interchangeable"), against a repository that already
// has an AGENTS.md — the exact repository the export's Codex form exists for.
//
// THE DEFECT THIS DETECTS. Step 2 runs `dossierx skills export .claude/skills`
// BEFORE step 3 creates project.config.yaml. The export resolves its project
// root from the config; with no config there is no root, and by the export's
// own contract a rootless export writes NO AGENTS.md section (it cannot find
// the file) and drops the generic guide beside the bundles instead of at
// docs/dossierx-agent-guide.md. Nothing later in the bootstrap re-runs the
// export, so a repo bootstrapped by the book ends with:
//
//   - an AGENTS.md that never received the dossierx section — the always-on
//     harness this repo actually uses stays uninstructed, forever, silently;
//   - no docs/dossierx-agent-guide.md under the project root, so the section's
//     documented companion (and the harness-independent form "guaranteed to be
//     read") is not where anything expects it.
//
// The export exits 0 both times a reader might look, so no step of the
// bootstrap ever fails — the defect is only visible in the terminal state,
// which is what this scenario asserts: after the whole documented sequence,
// the AGENTS.md section and the agent guide exist under the project root.
// It asserts the postcondition, not the mechanism, so it goes green whether
// the fix is reordering the steps, re-running the export after step 3, or
// teaching the export to find the root some other way.
package procedures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrap_ExportOrderStillReachesAgentsMDAndTheGuide(t *testing.T) {
	f := newBareFixture(t)

	requireDocAnchor(t, "skills/dossierx/SKILL.md",
		"**in this order** — steps 2 and 3 are not interchangeable")
	requireDocAnchor(t, "skills/dossierx/SKILL.md",
		"`dossierx skills export .claude/skills` — or whichever directory this harness reads")

	// The repo the bootstrap is run in: it has its own AGENTS.md, with its own
	// instructions, before DossierX arrives. This is the population the export's
	// "maintained only when AGENTS.md already exists" rule was written for.
	agentsPath := filepath.Join(f.root, "AGENTS.md")
	preexisting := "# this repo\n\nHouse instructions that predate DossierX.\n"
	if err := os.WriteFile(agentsPath, []byte(preexisting), 0o644); err != nil {
		t.Fatalf("write pre-existing AGENTS.md: %v", err)
	}

	f.Plan("bootstrap export order (dossierx)",
		"dossierx version",
		"dossierx skills export .claude/skills",
		"write project.config.yaml and claims/ (step 3: the config the human confirmed)",
		"copy scripts/ci/dossierx-check.yml into .github/workflows/ (step 4's \"no\" branch: CI is the authority; the documented fetch from the release path is substituted with this checkout's copy to stay off the network)",
		"step 5 skipped, and said out loud: the project was created at step 3, so there is no pre-ledger store to cross",
		"dossierx check --format text",
	)

	// Step 1's executable half: the binary is installed (the harness built it)
	// and `dossierx version` answers.
	version := f.Run("dossierx version", nil)
	f.DocumentedSuccess(version, "bootstrap step 1: dossierx version")

	// Step 2, exactly as documented and exactly where documented: before the
	// config exists. The skill itself warns that at step 2 "the config does not
	// exist until step 3" — and still schedules the export here.
	export := f.Run("dossierx skills export .claude/skills", nil)
	f.DocumentedSuccess(export, "bootstrap step 2: the export the skill orders before the config exists")

	// Step 3: propose and write the config. The human's confirmation of the
	// facet list is judgement this harness cannot enact (see the package
	// comment); the write is the step's executable half.
	f.Enact("write project.config.yaml and claims/ (step 3: the config the human confirmed)", func() {
		f.WriteProjectConfig()
	})

	// Step 4, "no" branch: no hook, add the CI workflow instead. The document
	// fetches the template from the pinned release URL; a hermetic suite
	// substitutes the same file from the tree under test, and says so in the
	// plan entry where the substitution is visible to every reader.
	f.Enact("copy scripts/ci/dossierx-check.yml into .github/workflows/ (step 4's \"no\" branch: CI is the authority; the documented fetch from the release path is substituted with this checkout's copy to stay off the network)", func() {
		src := filepath.Join(repoRoot(t), "scripts", "ci", "dossierx-check.yml")
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read the CI template this step installs: %v", err)
		}
		dst := filepath.Join(f.root, ".github", "workflows")
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dst, err)
		}
		if err := os.WriteFile(filepath.Join(dst, "dossierx-check.yml"), raw, 0o644); err != nil {
			t.Fatalf("write CI workflow: %v", err)
		}
	})

	// Step 5 is conditional on a pre-ledger store, and the skill says exactly
	// what to do when the condition is false: "Skip it on a project you created
	// at step 3, and say you skipped it." The skip is recorded, not silent.
	f.Enact("step 5 skipped, and said out loud: the project was created at step 3, so there is no pre-ledger store to cross", nil)

	// Step 6, verbatim including --format text: "Run `dossierx check --format
	// text` and show them the output exiting 0." Text mode prints prose, not an
	// envelope, so the documented outcome here is the exit status alone.
	check := f.Run("dossierx check --format text", nil)
	f.DocumentedSuccess(check, "bootstrap step 6: dossierx check --format text, exiting 0")

	// Steps 7–8 are things to TELL the human (commit the stores; run serve);
	// they change no state this scenario could assert on.

	// THE TERMINAL POSTCONDITION — what a by-the-book bootstrap must leave
	// behind in a repo that has an AGENTS.md. Two independent findings, each
	// reported on its own so the report says which half is missing:
	//
	// (a) The AGENTS.md section. Asserted as "the file changed from its
	//     pre-existing bytes" rather than by grepping for the marker comment,
	//     because the marker's exact spelling belongs to the export (an
	//     implementation constant this suite must not re-type), while "the
	//     export maintains a section in an AGENTS.md that already exists" is
	//     the documented contract — and an unchanged file cannot have had a
	//     section maintained in it.
	after, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md after the bootstrap: %v", err)
	}
	if string(after) == preexisting {
		t.Errorf("FINDING — after the whole documented bootstrap, the pre-existing AGENTS.md is byte-for-byte untouched: the export ran at step 2 with no project root, wrote no section, and nothing in the sequence ever exports again. The always-on harness this repo uses never learns DossierX exists.")
	}

	// (b) The guide, at the path the AGENTS.md section links it by and the only
	//     path the export's own contract writes it to when a root exists.
	guide := filepath.Join(f.root, "docs", "dossierx-agent-guide.md")
	if _, err := os.Stat(guide); err != nil {
		t.Errorf("FINDING — after the whole documented bootstrap, %s does not exist under the project root (%v): the rootless step-2 export dropped the guide beside the skill bundles instead, where nothing documented ever looks for it.", filepath.Join("docs", "dossierx-agent-guide.md"), err)
	}
}
