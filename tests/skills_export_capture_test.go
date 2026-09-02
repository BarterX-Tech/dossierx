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
// six bundle names this repo happens to embed today. This is deliberately
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

// siblingSkillLinkPattern matches the cross-reference spelling the bundles
// actually use — "](../<name>/SKILL.md)" — capturing <name>.
//
// It has two jobs, and they pull in opposite directions, which is why one
// pattern serves both. In the VERBATIM tree the link must be there and must
// resolve to a bundle that ships. In either DERIVED form it must be gone,
// because "one directory up, then sideways" reaches nothing from a single
// concatenated guide or from a section spliced into a repo-root AGENTS.md.
// Matching on the shape rather than on the six embedded names is deliberate,
// for the same reason rawWikilinkPattern is broad: rewriteSkillLinks
// (cmd/dossierx/skills_embed.go) only retargets names it actually loaded, so a
// renamed or typo'd target — the realistic failure — is by construction not a
// name any name-scoped needle would be looking for.
var siblingSkillLinkPattern = regexp.MustCompile(`\]\(\.\./([^/)\n]+)/SKILL\.md\)`)

var skillsExportCaptureOut = flag.String("skills-export-capture-out", "", "write the captured `dossierx skills export` output (surfaces.yaml's `agent-skills` surface) to this path as export-output.json")

// The six bundles this repo ships, in their embedded directory names. Kept
// as a local literal (rather than importing skills.Order from package tests,
// which has no dependency on the skills module) so this test has no coupling
// beyond what captureSkillsExport already has to the compiled binary's
// observable behavior.
var wantSkillsExportNames = []string{
	"dossierx",
	"dossierx-claims",
	"dossierx-comments",
	"dossierx-theme",
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
	// OWN body already names every companion skill in its prose — its "Which
	// skill to load" table lists all four as plain code spans — so that
	// assertion passes whether or not the index loop that follows the router
	// body ever runs, and it ALSO passes if a regression inlined every
	// companion's body in full instead of indexing it. Two more specific
	// checks close both gaps:
	//   - the exact link buildAgentsSection's index loop writes
	//     ("](docs/dossierx-agent-guide.md#name)") must be present, which
	//     only that loop produces (the router names its companions as plain
	//     code spans and links to none of them, so nothing in its body can
	//     supply this literal) — this catches the index loop being skipped or
	//     emptied;
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
// the six known bundle names. The finding this closes: rewriteWikilinks
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

	// The AGENTS.md section carries the ROUTER's body only (see
	// buildAgentsSection's doc comment), and the router's own SKILL.md has no
	// cross-reference of its own to retarget — so the meaningful place to prove
	// the rewrite ran is the guide (checked above), not this section. What IS
	// worth pinning here is that whatever cross-reference syntax the router's
	// body might one day gain does not leak through unrewritten either.
	if m := rawWikilinkPattern.FindString(capture.AgentsMDSection); m != "" {
		t.Errorf("AGENTS.md section capture contains raw wikilink %q; rewriteSkillLinks did not resolve it", m)
	}

	// The SIBLING-LINK half, which is the shape the bundles actually use. A
	// bundle spells a cross-reference as "](../<name>/SKILL.md)" — a link that
	// resolves in the exported TREE, the one form nothing rewrites. That target
	// is meaningless in either derived form: the guide is a single concatenated
	// document and the AGENTS.md section sits at the client's repo root, and
	// neither has a sibling directory to reach. So a surviving "](../" in either
	// is the same defect a surviving "[[" is, and it is checked the same broad
	// way — on the SHAPE, not on the six names — because rewriteSkillLinks only
	// retargets names it loaded, so a typo'd or renamed target is precisely what
	// a name-scoped needle cannot see.
	for _, form := range []struct{ what, text string }{
		{"agent guide", capture.AgentGuide},
		{"AGENTS.md section", capture.AgentsMDSection},
	} {
		if m := siblingSkillLinkPattern.FindString(form.text); m != "" {
			t.Errorf("%s capture still contains sibling skill link %q, which resolves to nothing in that form; rewriteSkillLinks did not retarget it", form.what, m)
		}
	}

	// Form 1 is a byte-for-byte copy of the embedded bundle, so if the sources
	// carried no cross-reference at all the two assertions above would be
	// passing over nothing. Every companion links back to the router (the router
	// links to none of them the other direction, which is why this reads a
	// companion rather than "dossierx/SKILL.md").
	const routerLink = "](../dossierx/SKILL.md)"
	if !strings.Contains(capture.SkillTree["dossierx-claims/SKILL.md"], routerLink) {
		t.Fatalf("expected the verbatim dossierx-claims/SKILL.md to still contain %s (proving Form 1 is untouched and the rewrite assertions above are exercising something real), got:\n%s", routerLink, capture.SkillTree["dossierx-claims/SKILL.md"])
	}

	// And the point of the spelling: every one of those links has to RESOLVE in
	// the tree a client's agent reads. The tree is keyed by path relative to the
	// export directory, so "<dir>/a/SKILL.md" linking "../b/SKILL.md" resolves
	// exactly when "b/SKILL.md" is a key. A link naming a bundle that does not
	// ship is the failure this closes — it reached a client as a dead link
	// before, and as four literal brackets before that.
	for _, path := range sortedSkillTreeKeys(capture.SkillTree) {
		for _, m := range siblingSkillLinkPattern.FindAllStringSubmatch(capture.SkillTree[path], -1) {
			target := m[1] + "/SKILL.md"
			if _, ok := capture.SkillTree[target]; !ok {
				t.Errorf("exported %s links %q, which resolves to %q — not a file this export writes. Every cross-reference in the verbatim tree must name a bundle that ships, or a client's agent follows it to nothing", path, m[0], target)
			}
		}
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

	data, err := json.MarshalIndent(readableCapture(capture), "", "  ")
	if err != nil {
		t.Fatalf("marshal capture: %v", err)
	}
	if err := os.WriteFile(*skillsExportCaptureOut, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", *skillsExportCaptureOut, err)
	}
	t.Logf("wrote skills export capture to %s", *skillsExportCaptureOut)
}

// skillsExportCaptureDoc is SkillsExportCapture as it is WRITTEN to
// export-output.json: every document is an array of its own lines rather than
// one string.
//
// This is not formatting taste, it is whether the evidence can be read at all.
// json.MarshalIndent indents the OBJECT; it cannot break a string value, so a
// document holding its newlines as "\n" escapes marshals to a single physical
// line however the object around it is indented. The agent guide is every
// bundle concatenated — the last capture put it on one line roughly 25,000
// tokens long, above what any single read returns, and a line-based reader
// cannot open part of a line. That surface's answer recorded, in those words,
// that it read two of the three forms and could not say what was in the third.
// A gate whose own evidence file is unopenable is a gate that reports "could
// not check" for a surface nobody chose to skip.
//
// Splitting on "\n" makes each source line its own JSON element, so the file is
// as readable line by line as the documents it captures. Nothing downstream
// constrains the shape: the gate's freshness key is a digest of whatever bytes
// this writes, so changing them re-keys the bundle and re-reads the surface,
// which is the correct consequence of the evidence changing.
type skillsExportCaptureDoc struct {
	SkillTree       map[string][]string `json:"skill_tree"`
	AgentGuide      []string            `json:"agent_guide"`
	AgentsMDSection []string            `json:"agents_md_section"`
}

// readableCapture converts a capture to its line-per-element written form.
//
// strings.Split on "\n" is exact and lossless: a document ending in a newline
// yields a final empty element, and strings.Join(lines, "\n") reproduces the
// original bytes for every input including the empty string. A reader
// reassembling a document from this file gets what the export wrote, not an
// approximation of it.
func readableCapture(c SkillsExportCapture) skillsExportCaptureDoc {
	doc := skillsExportCaptureDoc{
		SkillTree:       make(map[string][]string, len(c.SkillTree)),
		AgentGuide:      strings.Split(c.AgentGuide, "\n"),
		AgentsMDSection: strings.Split(c.AgentsMDSection, "\n"),
	}
	for path, body := range c.SkillTree {
		doc.SkillTree[path] = strings.Split(body, "\n")
	}
	return doc
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
