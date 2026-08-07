// skills_export_capture_test.go covers skills_export_capture.go: that the
// capture actually reads back what `dossierx skills export` writes, and that
// the wikilink transform did its job — no raw "[[name]]" survives into either
// derived document.
//
// TestCaptureSkillsExport_G1Capture is the reusable entry point: run with
// -skills-export-capture-out set, it writes export-output.json for a gate
// stage to hand a prose-auditing agent.
package tests

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawWikilinkPattern matches ANY unresolved "[[text]]" wikilink, not just the
// five bundle names this repo happens to embed today. This is deliberately
// broader than "does the bundle's own name still appear in brackets": a
// rewritten link never contains the bare "[[" + "]]" bracket pair at all
// (rewriteWikilinks turns it into "[`name`](...)", cmd/dossierx/skills_embed.go),
// so a needle built from `wantSkillsExportNames` can only ever fire if
// rewriteWikilinks is deleted outright — it is structurally blind to the
// realistic failure the function's own doc comment names: "a link to a
// bundle that does not exist is left as literal [[text]]" (skills_embed.go's
// rewriteWikilinks). A renamed or typo'd cross-reference produces exactly
// that literal, un-bracketed-by-name text, which the name-only needle never
// looks for. See TestCaptureSkillsExport_NoRawWikilinksSurvive's doc comment
// for the three concrete mutations this closes.
var rawWikilinkPattern = regexp.MustCompile(`\[\[[^\]\n]+\]\]`)

var skillsExportCaptureOut = flag.String("skills-export-capture-out", "", "write the captured `dossierx skills export` output (surfaces.yaml's `agent-skills` surface) to this path as export-output.json")

// The five bundles this repo ships, in their embedded directory names. Kept
// as a local literal (rather than importing skills.Order from package tests,
// which has no dependency on the skills module) so this test has no coupling
// beyond what captureSkillsExport already has to the compiled binary's
// observable behavior.
var wantSkillsExportNames = []string{
	"dossierx",
	"dossierx-claims",
	"dossierx-comments",
	"dossierx-build-order",
	"dossierx-code-links",
}

// newSkillsExportFixture builds a project with every harness `skills export`
// detects: a .claude/ directory (so the verbatim tree is written) and an
// existing AGENTS.md (so the spliced section is written), plus a
// project.config.yaml so the generic guide lands at its documented path
// rather than beside an explicit directory argument.
func newSkillsExportFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixtureProject(t, root, "skillscapture")
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# House rules\n\nBe careful.\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	return root
}

func TestCaptureSkillsExport_AllThreeFormsPresent(t *testing.T) {
	root := newSkillsExportFixture(t)
	capture := captureSkillsExport(t, root)

	for _, name := range wantSkillsExportNames {
		key := name + "/SKILL.md"
		body, ok := capture.SkillTree[key]
		if !ok {
			t.Fatalf("skill tree capture missing %q; got keys %v", key, sortedSkillTreeKeys(capture.SkillTree))
		}
		if !strings.Contains(body, "name: "+name) {
			t.Errorf("%s: expected frontmatter \"name: %s\", got:\n%s", key, name, body)
		}
	}

	for _, name := range wantSkillsExportNames {
		if !strings.Contains(capture.AgentGuide, `<a id="`+name+`"></a>`) {
			t.Errorf("agent guide capture missing an anchor for %s", name)
		}
	}
	// No leaked YAML frontmatter: buildAgentGuide always opens with a fixed
	// "# DossierX — agent guide" preamble (cmd/dossierx/skills_embed.go), so
	// checking HasPrefix(TrimSpace(guide), "---") can never observe a real
	// leak — it tests the preamble, not any bundle's body. The place a leak
	// would actually show up is right after each bundle's own anchor: that is
	// where parseSkillDoc's stripped Body gets concatenated in, so if
	// parseSkillDoc ever stopped stripping frontmatter, the untouched
	// "---\nname: ...\n" block would appear there instead of the bundle's
	// real first heading.
	for _, name := range wantSkillsExportNames {
		anchor := "<a id=\"" + name + "\"></a>\n\n"
		idx := strings.Index(capture.AgentGuide, anchor)
		if idx < 0 {
			// NOT "already reported by the anchor-presence check above": that
			// check above uses a SHORTER needle (`<a id="name"></a>`, no
			// trailing blank line), so a formatting change that removes just
			// the blank line after the anchor would pass the presence check
			// while this longer needle stops matching — and this is the ONLY
			// place that actually looks for leaked frontmatter, so silently
			// skipping it here would disable the one guard against exactly
			// that regression. Fail loudly instead: a check that cannot run
			// is a failure, never a silent pass (CLAUDE.md).
			t.Fatalf("%s: could not find anchor+blank-line %q in agent guide capture; the frontmatter-leak check below cannot run. Got:\n%s", name, anchor, capture.AgentGuide)
		}
		remainder := capture.AgentGuide[idx+len(anchor):]
		if strings.HasPrefix(remainder, "---") {
			t.Errorf("%s: agent guide capture has leaked YAML frontmatter right after its anchor, got:\n%s", name, remainder[:min(200, len(remainder))])
		}
	}

	if !strings.HasPrefix(capture.AgentsMDSection, skillsExportAgentsBeginMarker) {
		t.Errorf("AGENTS.md section capture does not start with the BEGIN marker, got:\n%s", capture.AgentsMDSection)
	}
	if !strings.HasSuffix(strings.TrimRight(capture.AgentsMDSection, "\n"), skillsExportAgentsEndMarker) {
		t.Errorf("AGENTS.md section capture does not end with the END marker, got:\n%s", capture.AgentsMDSection)
	}
	// The router only: the AGENTS.md section budget is tight (loaded every
	// turn), so it must carry the router and point at the guide for the rest
	// rather than inlining every companion skill.
	//
	// A bare strings.Contains(section, name) here is vacuous: the router's
	// OWN body already names every companion skill in its prose (it links to
	// them — see the [[wikilink]] cross-references in skills/dossierx/SKILL.md
	// buildAgentsSection rewrites), so that assertion passes whether or not
	// the index loop that follows the router body ever runs, and it ALSO
	// passes if a regression inlined every companion's body in full instead
	// of indexing it. Two more specific checks close both gaps:
	//   - the exact link buildAgentsSection's index loop writes
	//     ("](docs/dossierx-agent-guide.md#name)") must be present, which
	//     only that loop produces (the router's own body has no such
	//     literal — it uses [[wikilink]] syntax, and the router itself never
	//     links to a companion "the other direction", per
	//     TestCaptureSkillsExport_NoRawWikilinksSurvive's comment) — this
	//     catches the index loop being skipped or emptied;
	//   - the companion's own opening heading (its first body line, verbatim
	//     from SkillTree) must be ABSENT — this catches the companion being
	//     inlined in full instead of merely indexed.
	for _, name := range wantSkillsExportNames[1:] {
		wantLink := "](" + skillsExportAgentGuidePath + "#" + name + ")"
		if !strings.Contains(capture.AgentsMDSection, wantLink) {
			t.Errorf("AGENTS.md section capture should index companion skill %s as a link into the guide (%q), got:\n%s", name, wantLink, capture.AgentsMDSection)
		}

		raw, ok := capture.SkillTree[name+"/SKILL.md"]
		if !ok {
			t.Fatalf("missing skill tree capture for %s; cannot derive its distinctive heading", name)
		}
		heading := firstBodyLine(t, raw)
		if strings.Contains(capture.AgentsMDSection, heading) {
			t.Errorf("AGENTS.md section capture contains companion %s's own opening heading %q — it should be indexed only, not inlined in full (budget is tight, see buildAgentsSection's doc comment), got:\n%s", name, heading, capture.AgentsMDSection)
		}
	}
}

// The whole reason this capture exists rather than a source read: no raw
// "[[text]]" wikilink syntax may survive into either derived document. A
// literal bracket pair reaching a client's AGENTS.md or agent guide means
// rewriteWikilinks silently failed to resolve a real cross-reference.
//
// This asserts against rawWikilinkPattern (ANY "[[...]]" pair), not against
// the five known bundle names. The finding this closes: rewriteWikilinks
// (cmd/dossierx/skills_embed.go) unconditionally rewrites every name in
// `names` — the loaded bundles — so a needle built from those same names can
// only ever find something if rewriteWikilinks is deleted outright. The
// realistic failure the function's own doc comment names is the opposite
// shape: "a link to a bundle that does not exist is left as literal
// [[text]]" — a typo'd or renamed cross-reference, which by definition is
// NOT one of the names being rewritten and so never matches a name-scoped
// needle. Three mutations, each confirmed to leave a name-scoped assertion
// green while a raw bracket pair reaches AgentGuide, prove this needle is the
// one that has to be broad:
//   - inserting "See [[dossierx-claimz]]" (a typo) into a bundle's SKILL.md;
//   - inserting "See [[claims-router]]" (a plausible but nonexistent name);
//   - renaming the loaded name "dossierx-build-order" to
//     "build-order-skill" everywhere EXCEPT inside an existing
//     "[[dossierx-build-order]]" reference, simulating a rename regression
//     that leaves one cross-reference stale.
//
// All three leave a literal "[[" + "]]" pair in AgentGuide; none of them is
// reachable through wantSkillsExportNames because none of the three needles
// is a name rewriteWikilinks was ever going to rewrite in the first place.
func TestCaptureSkillsExport_NoRawWikilinksSurvive(t *testing.T) {
	root := newSkillsExportFixture(t)
	capture := captureSkillsExport(t, root)

	if m := rawWikilinkPattern.FindString(capture.AgentGuide); m != "" {
		t.Errorf("agent guide capture still contains raw wikilink %q; rewriteWikilinks did not resolve it", m)
	}

	// The verbatim tree is the one form that is NOT rewritten (Form 1 is a
	// byte-for-byte copy of the embedded bundle) — so if the companion skills'
	// SKILL.md sources contained no wikilink at all, the assertion above
	// would be passing over nothing. Every companion links back to the
	// router as "[[dossierx]]" (the router itself carries none the other
	// direction, which is why this checks a companion rather than
	// "dossierx/SKILL.md").
	if !strings.Contains(capture.SkillTree["dossierx-claims/SKILL.md"], "[[dossierx]]") {
		t.Fatalf("expected the verbatim dossierx-claims/SKILL.md to still contain a raw [[dossierx]] wikilink (proving Form 1 is untouched and the rewrite assertion above is exercising something real), got:\n%s", capture.SkillTree["dossierx-claims/SKILL.md"])
	}

	// The AGENTS.md section carries the ROUTER's body only (see
	// buildAgentsSection's doc comment), and the router's own SKILL.md has no
	// [[wikilink]] of its own to rewrite — so the meaningful place to prove
	// the rewrite ran is the guide (checked above), not this section. What IS
	// worth pinning here is that whatever wikilink syntax the router's body
	// might one day gain does not leak through unrewritten either.
	if m := rawWikilinkPattern.FindString(capture.AgentsMDSection); m != "" {
		t.Errorf("AGENTS.md section capture contains raw wikilink %q; rewriteWikilinks did not resolve it", m)
	}
}

// TestCaptureSkillsExport_G1Capture is how a gate stage actually uses this
// capture: invoked as
//
//	go test ./tests -run TestCaptureSkillsExport_G1Capture \
//	  -args -skills-export-capture-out=/path/to/export-output.json
//
// it runs the export against a fresh fixture and writes the full capture to
// -skills-export-capture-out. With the flag unset (the default `go test`
// invocation, and every CI run of this suite) it writes nothing — the
// capture's correctness is the two tests above's job, not this one's.
//
// The check below keys off PRESENCE (flag.CommandLine.Visit), not VALUE
// (*skillsExportCaptureOut == ""): a driver invoking
// `-skills-export-capture-out=$OUT` with OUT unset expands to the flag being
// given an empty value, which `== ""` cannot tell apart from the flag never
// having been passed at all — the same gap
// TestPredictReleaseNotesForRange_G1Capture had (tests/release_notes_predict_test.go),
// fixed the same way here. See
// TestCaptureSkillsExport_G1Capture_RequiresNonEmptyValueWhenFlagGiven for
// the regression test.
func TestCaptureSkillsExport_G1Capture(t *testing.T) {
	var outGiven bool
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == "skills-export-capture-out" {
			outGiven = true
		}
	})
	if !outGiven {
		t.Skip("no -skills-export-capture-out given; this test is a capture entry point, not a correctness check (see TestCaptureSkillsExport_AllThreeFormsPresent for that)")
	}
	if *skillsExportCaptureOut == "" {
		// -skills-export-capture-out WAS passed but its value is empty — the
		// unset-shell-variable hazard. Silently skipping here means the
		// export-output.json a gate stage hands to a prose-auditing agent
		// never gets written, while `go test` still exits 0.
		t.Fatalf("-skills-export-capture-out was given but is empty (e.g. a driver expanding an unset shell variable); a skip is a failure, not a pass")
	}

	root := newSkillsExportFixture(t)
	capture := captureSkillsExport(t, root)

	data, err := json.MarshalIndent(capture, "", "  ")
	if err != nil {
		t.Fatalf("marshal capture: %v", err)
	}
	if err := os.WriteFile(*skillsExportCaptureOut, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", *skillsExportCaptureOut, err)
	}
	t.Logf("wrote skills export capture to %s", *skillsExportCaptureOut)
}

// TestCaptureSkillsExport_G1Capture_RequiresNonEmptyValueWhenFlagGiven is the
// regression test for the same class of BLOCKING finding fixed in
// tests/release_notes_predict_test.go: -skills-export-capture-out given on
// the command line but with an EMPTY value (a driver expanding an unset
// shell variable) must fail loudly, not be silently indistinguishable from
// the flag never having been passed at all. The flag package's process-global
// *flag.String var means this can only be exercised by actually invoking
// `go test -args ...` in a subprocess — a plain call from within this same
// test binary only ever observes the flags it itself was started with.
func TestCaptureSkillsExport_G1Capture_RequiresNonEmptyValueWhenFlagGiven(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := repoRoot(t)

	runG1Capture := func(t *testing.T, args ...string) (stdout string, exitCode int) {
		t.Helper()
		full := append([]string{"test", "./tests", "-run", "^TestCaptureSkillsExport_G1Capture$", "-count=1", "-v", "-args"}, args...)
		cmd := exec.Command("go", full...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				t.Fatalf("go %s: %v\n%s", strings.Join(full, " "), err, out)
			}
		}
		return string(out), code
	}

	t.Run("flag given empty must fail, not skip", func(t *testing.T) {
		out, code := runG1Capture(t, "-skills-export-capture-out=")
		if code == 0 {
			t.Fatalf("expected a non-zero exit when -skills-export-capture-out is given but empty, got exit 0 — export-output.json would silently not be written while `go test` still passes:\n%s", out)
		}
		if strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to FAIL, not SKIP, got:\n%s", out)
		}
		if !strings.Contains(out, "-skills-export-capture-out was given but is empty") {
			t.Errorf("expected the fatal message naming the empty -skills-export-capture-out, got:\n%s", out)
		}
	})

	t.Run("flag not given at all still skips cleanly", func(t *testing.T) {
		// Negative control: the fix must not turn the ordinary, flagless
		// `go test ./tests` invocation (every CI run of this suite) into a
		// failure.
		out, code := runG1Capture(t)
		if code != 0 {
			t.Fatalf("expected exit 0 when the flag is not given at all, got exit %d:\n%s", code, out)
		}
		if !strings.Contains(out, "--- SKIP") {
			t.Errorf("expected the test to SKIP when the flag is not given, got:\n%s", out)
		}
	})

	t.Run("flag given a real path still succeeds and writes the file (positive control)", func(t *testing.T) {
		// Negative control: a mutation that made the empty-value check fire
		// unconditionally (regardless of *skillsExportCaptureOut's actual
		// value) would pass both tests above and still break every real G1
		// run.
		outPath := filepath.Join(t.TempDir(), "export-output.json")
		out, code := runG1Capture(t, "-skills-export-capture-out="+outPath)
		if code != 0 {
			t.Fatalf("expected exit 0 for a real -skills-export-capture-out path, got exit %d:\n%s", code, out)
		}
		if _, err := os.Stat(outPath); err != nil {
			t.Fatalf("expected -skills-export-capture-out to be written to %s, got: %v\n%s", outPath, err, out)
		}
	})
}
