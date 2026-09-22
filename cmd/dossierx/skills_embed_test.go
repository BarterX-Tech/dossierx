// skills_embed_test.go covers "dossierx skills export [dir]" (see
// skills_embed.go) plus the content invariants of the embedded skill bundles
// themselves.
//
// The command half runs in-process via execCLI (defined in
// cli_inprocess_test.go) and asserts each of the three harness forms: the
// verbatim SKILL.md tree, the idempotent AGENTS.md section, and the
// always-written self-contained guide.
//
// The content half is the part worth the most. The skills are the ONLY
// documentation an agent operating this CLI ever reads, so a skill naming a
// command that no longer exists is not a docs bug — it is an agent following
// instructions into a "usage" error and then improvising. v0.3.0 renamed or
// deleted eighteen invocations at once, which is exactly the change that leaves
// prose behind, so TestSkills_EveryInvocationNamesARealCommand walks the real
// cobra tree instead of trusting review.
package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// The five bundles and the order the router presents them in. Spelled out
// rather than derived so that adding or removing a skill is a deliberate edit
// to a test, the same way cmd/dossierx/main_test.go pins the leaf surface.
var wantSkillNames = []string{
	"dossierx",
	"dossierx-claims",
	"dossierx-comments",
	"dossierx-build-order",
	"dossierx-code-links",
}

// ---------------------------------------------------------------------
// Form 1 — the SKILL.md tree
// ---------------------------------------------------------------------

func TestCLI_SkillsExport_WritesAllSkillFiles(t *testing.T) {
	targetDir := t.TempDir()

	stdout, stderr, err := execCLI(t, "skills", "export", targetDir)
	if err != nil {
		t.Fatalf("skills export: unexpected error: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	for _, name := range wantSkillNames {
		path := filepath.Join(targetDir, name, "SKILL.md")
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("expected exported file %s to exist: %v", path, readErr)
		}
		if !strings.Contains(string(data), "name: "+name) {
			t.Fatalf("expected %s to contain frontmatter %q, got:\n%s", path, "name: "+name, string(data))
		}
	}

	// Five bundles, their lock file, plus the generic guide, which is always
	// written — with no project root to put it in, it lands beside the bundles.
	if !strings.Contains(stdout, "wrote 7 file(s)") {
		t.Fatalf("expected stdout to report 7 file(s) written, got:\n%s", stdout)
	}
	if _, statErr := os.Stat(filepath.Join(targetDir, skillsLockFile)); statErr != nil {
		t.Fatalf("the lock file must be written beside the bundles: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(targetDir, "dossierx-agent-guide.md")); statErr != nil {
		t.Fatalf("the generic guide must be written even with no project root: %v", statErr)
	}
}

func TestCLI_SkillsExport_OverwritesExistingFiles(t *testing.T) {
	targetDir := t.TempDir()
	stalePath := filepath.Join(targetDir, "dossierx-claims", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if _, _, err := execCLI(t, "skills", "export", targetDir); err != nil {
		t.Fatalf("skills export: unexpected error: %v", err)
	}

	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("read exported file: %v", err)
	}
	if strings.Contains(string(data), "stale content") {
		t.Fatalf("expected stale content to be overwritten, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "name: dossierx-claims") {
		t.Fatalf("expected overwritten file to contain fresh frontmatter, got:\n%s", string(data))
	}
}

// With neither a directory argument nor a project to attach the other two forms
// to, there is nowhere to install anything — and reporting success there would
// tell a bootstrap agent the guide is installed when nothing was written.
func TestCLI_SkillsExport_RefusesWithNowhereToWrite(t *testing.T) {
	empty := t.TempDir()
	missingCfg := filepath.Join(empty, "project.config.yaml")

	if _, _, err := execCLI(t, "--config", missingCfg, "skills", "export"); err == nil {
		t.Fatalf("expected an error when there is no <dir> and no project root, got nil")
	}
}

// ---------------------------------------------------------------------
// Harness detection — which forms a given repo gets
// ---------------------------------------------------------------------

// The bare, no-argument invocation the bootstrap paste block relies on: it finds
// the project itself and writes exactly the forms this repo's harnesses read.
func TestCLI_SkillsExport_DetectsTheHarnessesTheProjectAlreadyHas(t *testing.T) {
	project := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, project, "widget")
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	agentsPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# House rules\n\nBe careful.\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	stdout, _, err := execCLI(t, "--config", cfgPath, "skills", "export")
	if err != nil {
		t.Fatalf("skills export: %v (out: %s)", err, stdout)
	}

	// Claude Code: the tree lands under the .claude/ this repo already had.
	if _, statErr := os.Stat(filepath.Join(project, ".claude", "skills", "dossierx", "SKILL.md")); statErr != nil {
		t.Fatalf("expected the skill tree under .claude/skills: %v", statErr)
	}
	// Codex: the section is spliced into the existing AGENTS.md, and the
	// project's own instructions are left alone.
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	for _, want := range []string{"# House rules", "Be careful.", agentsBeginMarker, agentsEndMarker, "The eight nouns"} {
		if !strings.Contains(string(agents), want) {
			t.Fatalf("expected AGENTS.md to contain %q, got:\n%s", want, string(agents))
		}
	}
	// Anything else: the guide, at its documented path.
	if _, statErr := os.Stat(filepath.Join(project, "docs", "dossierx-agent-guide.md")); statErr != nil {
		t.Fatalf("expected the generic guide at docs/dossierx-agent-guide.md: %v", statErr)
	}
}

// Detection, not creation: a project with no .claude/ and no AGENTS.md gets the
// one form that needs no harness, and is told what was skipped and why.
func TestCLI_SkillsExport_CreatesNoHarnessTheProjectDoesNotUse(t *testing.T) {
	project := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, project, "widget")

	stdout, _, err := execCLI(t, "--config", cfgPath, "skills", "export")
	if err != nil {
		t.Fatalf("skills export: %v (out: %s)", err, stdout)
	}

	if _, statErr := os.Stat(filepath.Join(project, ".claude")); statErr == nil {
		t.Fatalf("skills export must not create a .claude/ directory the project did not have")
	}
	if _, statErr := os.Stat(filepath.Join(project, "AGENTS.md")); statErr == nil {
		t.Fatalf("skills export must not create an AGENTS.md the project did not have")
	}
	if _, statErr := os.Stat(filepath.Join(project, "docs", "dossierx-agent-guide.md")); statErr != nil {
		t.Fatalf("the generic guide is always written: %v", statErr)
	}
	if !strings.Contains(stdout, "skipped") {
		t.Fatalf("expected the skipped forms to be reported, got:\n%s", stdout)
	}
}

// ---------------------------------------------------------------------
// Form 2 — AGENTS.md, and the idempotence the whole design rests on
// ---------------------------------------------------------------------

// Re-running the export is the documented way to pick up a new DossierX
// version, so a second run must produce byte-identical bytes and must never
// stack a second copy of the section.
func TestCLI_SkillsExport_AgentsSectionIsIdempotent(t *testing.T) {
	project := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, project, "widget")
	agentsPath := filepath.Join(project, "AGENTS.md")
	original := "# House rules\n\nBe careful.\n\n## Trailer\n\nStill here.\n"
	if err := os.WriteFile(agentsPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	if _, _, err := execCLI(t, "--config", cfgPath, "skills", "export"); err != nil {
		t.Fatalf("first export: %v", err)
	}
	first, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}

	if _, _, err := execCLI(t, "--config", cfgPath, "skills", "export"); err != nil {
		t.Fatalf("second export: %v", err)
	}
	second, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("re-read AGENTS.md: %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("second export changed AGENTS.md; the section is not idempotent.\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if got := strings.Count(string(second), agentsBeginMarker); got != 1 {
		t.Fatalf("expected exactly 1 section marker, got %d:\n%s", got, second)
	}
	if !strings.Contains(string(second), "## Trailer") || !strings.Contains(string(second), "Still here.") {
		t.Fatalf("content after the section must survive the splice, got:\n%s", second)
	}
}

// A hand-edited section is replaced wholesale, not merged: the file is
// generated, and a stale sentence inside the markers is exactly the drift the
// single-source design exists to prevent.
func TestSpliceAgentsSection_ReplacesAStaleSection(t *testing.T) {
	existing := "intro\n\n" + agentsBeginMarker + "\nSTALE: run dossierx lint\n" + agentsEndMarker + "\n\noutro\n"
	got := spliceAgentsSection(existing, agentsBeginMarker+"\nfresh\n"+agentsEndMarker+"\n")

	if strings.Contains(got, "STALE") {
		t.Fatalf("expected the old section body to be replaced, got:\n%s", got)
	}
	for _, want := range []string{"intro", "fresh", "outro"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q to survive, got:\n%s", want, got)
		}
	}
	if got := strings.Count(got, agentsBeginMarker); got != 1 {
		t.Fatalf("expected exactly 1 marker after a replace, got %d", got)
	}
}

// A BEGIN with no END is a truncated file, not a section: swallowing everything
// after it would eat the rest of someone's instructions, so the remnant is left
// alone and a fresh section is appended.
func TestSpliceAgentsSection_LeavesAnUnterminatedRemnantAlone(t *testing.T) {
	existing := "intro\n" + agentsBeginMarker + "\nhalf a section\nimportant tail\n"
	got := spliceAgentsSection(existing, agentsBeginMarker+"\nfresh\n"+agentsEndMarker+"\n")

	if !strings.Contains(got, "important tail") {
		t.Fatalf("an unterminated marker must not swallow the file's tail, got:\n%s", got)
	}
	if !strings.Contains(got, agentsEndMarker) {
		t.Fatalf("expected a well-formed section to be appended, got:\n%s", got)
	}
}

// ---------------------------------------------------------------------
// Form 3 — the generic guide
// ---------------------------------------------------------------------

// "Self-contained" is a testable claim: every bundle in full, no frontmatter to
// confuse a plain markdown reader, and no [[wikilink]] pointing at a file the
// reader does not have.
func TestBuildAgentGuide_IsSelfContained(t *testing.T) {
	guide, err := buildAgentGuide(dxskills.FS)
	if err != nil {
		t.Fatalf("buildAgentGuide: %v", err)
	}

	for _, name := range wantSkillNames {
		if !strings.Contains(guide, `<a id="`+name+`"></a>`) {
			t.Fatalf("expected an anchor for %s in the guide", name)
		}
		if !strings.Contains(guide, "[`"+name+"`](#"+name+")") {
			t.Fatalf("expected the index to link to #%s", name)
		}
	}
	if strings.Contains(guide, "[[dossierx") {
		t.Fatalf("expected every [[wikilink]] rewritten to an in-document anchor, got a raw one")
	}
	if strings.Contains(guide, "\nname: dossierx") {
		t.Fatalf("frontmatter must not leak into the guide")
	}
	// The router's body has to be present in full, not summarized: this is the
	// only form some harnesses will ever read.
	for _, want := range []string{"The eight nouns, twenty-five leaves", "Five rules that never bend", "unlock → fix → lock"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("expected the guide to carry the router's %q section", want)
		}
	}
}

// skills.Order is the declared reading order and this file's wantSkillNames is
// the test's copy of it; they must agree, or every assertion below is being made
// against a set the exporter does not use.
func TestSkillsOrder_MatchesTheExpectedSet(t *testing.T) {
	if len(dxskills.Order) != len(wantSkillNames) {
		t.Fatalf("skills.Order has %d entries, want %d: %v", len(dxskills.Order), len(wantSkillNames), dxskills.Order)
	}
	for i, name := range wantSkillNames {
		if dxskills.Order[i] != name {
			t.Fatalf("skills.Order[%d] = %q, want %q", i, dxskills.Order[i], name)
		}
	}
}

// A bundle added to skills/ without a place in skills.Order must be a loud
// failure, not a section silently missing from the guide — a guide with four of
// five skills in it reads exactly like a complete one.
func TestLoadSkillDocs_RefusesAnUnorderedBundle(t *testing.T) {
	extra := fstest.MapFS{"dossierx-surprise/SKILL.md": &fstest.MapFile{Data: []byte("---\nname: dossierx-surprise\n---\n\nhi\n")}}
	merged := multiFS{dxskills.FS, extra}

	if _, err := loadSkillDocs(merged); err == nil {
		t.Fatalf("expected loadSkillDocs to refuse a bundle missing from skills.Order")
	} else if !strings.Contains(err.Error(), "dossierx-surprise") {
		t.Fatalf("expected the error to name the unordered bundle, got: %v", err)
	}
}

// multiFS is the smallest thing that can present "the real bundles plus one
// more" as a single fs.FS: ReadDir concatenates, everything else falls through to
// whichever member has the file.
type multiFS [2]fs.FS

func (m multiFS) Open(name string) (fs.File, error) {
	f, err := m[0].Open(name)
	if err == nil {
		return f, nil
	}
	return m[1].Open(name)
}

func (m multiFS) ReadDir(name string) ([]fs.DirEntry, error) {
	first, err := fs.ReadDir(m[0], name)
	if err != nil {
		return nil, err
	}
	second, err := fs.ReadDir(m[1], name)
	if err != nil {
		return first, nil //nolint:nilerr // the second member is allowed to have no such directory
	}
	return append(first, second...), nil
}

// The router is loaded always and first, so it must be the first thing in every
// derived form. A document that opens on a companion skill teaches the reader to
// start in the middle.
func TestLoadSkillDocs_PutsTheRouterFirst(t *testing.T) {
	docs, err := loadSkillDocs(dxskills.FS)
	if err != nil {
		t.Fatalf("loadSkillDocs: %v", err)
	}
	if len(docs) != len(wantSkillNames) {
		t.Fatalf("expected %d bundles, got %d", len(wantSkillNames), len(docs))
	}
	if docs[0].Name != dxskills.RouterName {
		t.Fatalf("expected %q first, got %q", dxskills.RouterName, docs[0].Name)
	}
	for _, doc := range docs {
		if doc.Description == "" {
			t.Fatalf("%s has no frontmatter description; the derived forms use it as their index entry", doc.Name)
		}
		if strings.HasPrefix(doc.Body, "---") {
			t.Fatalf("%s: frontmatter was not stripped from the body", doc.Name)
		}
	}
}

// The always-on form carries the router and only the router — see
// buildAgentsSection's doc comment for the context budget that decides this.
func TestBuildAgentsSection_CarriesTheRouterAndPointsAtTheRest(t *testing.T) {
	section, err := buildAgentsSection(dxskills.FS)
	if err != nil {
		t.Fatalf("buildAgentsSection: %v", err)
	}

	if !strings.HasPrefix(section, agentsBeginMarker) || !strings.HasSuffix(section, agentsEndMarker+"\n") {
		t.Fatalf("the section must be marker-delimited on both ends, got:\n%s", section)
	}
	if !strings.Contains(section, "Five rules that never bend") {
		t.Fatalf("expected the router's rules inline in the AGENTS.md section")
	}
	if strings.Contains(section, "Channel B — tag it") {
		t.Fatalf("the always-on section must not inline the companion skills")
	}
	for _, name := range wantSkillNames[1:] {
		if !strings.Contains(section, agentGuidePath+"#"+name) {
			t.Fatalf("expected the section to point at %s#%s", agentGuidePath, name)
		}
	}
}

// ---------------------------------------------------------------------
// The bundles' own content
// ---------------------------------------------------------------------

// invocationPattern finds "dossierx <word>" and, when a second lowercase word
// follows on the same line, "dossierx <word> <word>".
//
// The convention that makes this reliable — and that any edit to a skill must
// keep — is that a LOWERCASE "dossierx" is only ever used to start a real
// invocation. Prose says "DossierX". So every match this finds is a claim that a
// command exists, and every claim is checked below.
var invocationPattern = regexp.MustCompile(`dossierx ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?`)

func TestSkills_EveryInvocationNamesARealCommand(t *testing.T) {
	root := newRootCmd()

	// resolve reports whether a space-joined command path exists in the real
	// tree. Groups count: "dossierx claim" is a legitimate thing to write.
	resolve := func(path ...string) bool {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == nil {
			return false
		}
		return commandPath(cmd) == strings.Join(path, " ")
	}

	err := fs.WalkDir(dxskills.FS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		raw, readErr := fs.ReadFile(dxskills.FS, path)
		if readErr != nil {
			return readErr
		}
		for _, m := range invocationPattern.FindAllStringSubmatch(string(raw), -1) {
			if m[2] != "" && resolve(m[1], m[2]) {
				continue
			}
			if resolve(m[1]) {
				continue
			}
			t.Errorf("%s names %q, which is not a command in the current surface — eight nouns, and dossierx lint/stale/coverage/deps/implink/migrate and comment resolve/reopen/edit/delete do not exist. If this is prose, write \"DossierX\" with a capital D.", path, strings.TrimSpace(m[0]))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded skills: %v", err)
	}
}

// The budget from the plan, enforced. A skill nobody finishes reading is a skill
// nobody follows, and the failure mode of a long one is silent: the agent skims
// the schema and misses the rule about locked claims at the bottom.
//
// It was 200 through v0.2.x, sized for a six-noun surface with no upgrade path.
// v0.3.0 added a seventh noun and a BREAKING change the router is the only place
// an agent is guaranteed to read about: the one-time `migrate --adopt` every
// pre-v0.3.0 project must run before any gate passes. Raised to 230 as a
// deliberate resize rather than quietly per release: the alternative was cutting
// the adoption section, and an agent that meets `lock-ledger-adoption-required`
// without having read what adoption IS either loops on a gate it cannot clear or
// runs the migration on a project where doing so records tampered bytes as
// approved. The four companion skills are unaffected and all still sit well
// under 200 (claims 200, comments 174, code-links 128, build-order 110), which
// is the check that this is a surface change and not prose creep.
//
// The raise was also justified, at the time, by `--staged`'s parent-commit
// comparison and its two findings. That machinery was removed, and the ceiling
// deliberately was NOT lowered to match: this is a MAXIMUM, the router got
// shorter on its own, and ratcheting a budget down to whatever the current file
// happens to measure turns every honest sentence added later into a test
// failure. Lower it only on a decision that the router should be shorter.
//
// Raised again to 255 in v0.5.0, on the same reasoning that moved it to 230 and
// on no other. `mixed-cycle` is the second BREAKING change in this project's
// history that can fire on a corpus the agent did NOT touch: no edit, no
// content-hash move, nothing in the lock store to explain it. That is the case
// an agent handles worst — its instinct is to hunt for what it broke, find
// nothing, and loop — and the router is the one file every agent is guaranteed
// to have read before it meets the finding. The file was already at 229 of 230,
// so the real choice was "cover it" or "cut something else load-bearing", not
// "keep the budget". The four companions are unaffected and still sit under
// 235 (claims 231, comments 187, code-links 128, build-order 110), which is the
// check that this is a surface change and not prose creep.
// Raised again to 265, on the same reasoning that moved it to 230 and then to
// 255, and on no other. This release adds a NOUN and a SCHEMA FIELD: `track`
// with three read-only leaves, taking the surface from seven nouns and nineteen
// leaves to eight and twenty-two, and `sources` with its citation grammar and
// five lint rules. The router is the one file every agent is guaranteed to read
// before it meets either, and an agent that reaches `track status` or a
// `source-internal-drift` finding without having read what they are is the
// looping case this budget's history is entirely about.
//
// THE PROSE WAS TIGHTENED FIRST, and the raise is what was left. The tracks
// paragraph came down from three lines to two by saying the same thing in fewer
// words. The remaining two lines could only have come out of the ledger-boundary
// paragraph, which is load-bearing, so this is the file's own stated choice:
// "cover it" or "cut something else load-bearing", not "keep the budget".
//
// THE COMPANION CHECK MOVED WITH IT, and it is worth reading rather than
// skipping. Previous raises could point at four companions sitting far under the
// ceiling as evidence that the router alone had grown, which is what tells a
// surface change apart from prose creep. That is no longer the shape: claims is
// at 255, up from 231, because `sources` and `tracks` are claim-authoring
// concerns and its guide had to carry both. The other three did not move at all
// (comments 192, code-links 136, build-order 110). Two guides growing for one
// schema change and three not moving is still the surface signature; four
// growing together would not be, and that is the thing to look for next time.
//
// THE SIXTH BUNDLE CAME AND WENT, and the census is back to five. A
// dossierx-theme bundle was added here at 235 lines, under the ceiling, costing
// the router one line — a row in the companion table — and it was removed again
// with custom themes themselves. It is not in skills.Order and is not exported.
// Nothing was raised for it and nothing had to be lowered when it left, which is
// the budget behaving as a MAXIMUM rather than a ratchet, exactly as the
// `--staged` paragraph above says it should.
//
// CURRENT CENSUS: router 263 of 265, claims 255, comments 206, code-links 136,
// build-order 142. Two of these moved without a raise and for opposite reasons.
// comments went 192 -> 206 for a surface fact the guide was missing outright —
// the four refusals an agent meets when a thread id goes stale or the human
// resolves underneath it — which is a coverage fix, the shape a companion is
// SUPPOSED to grow in. build-order went 110 -> 142 with no such event attached,
// and that is the number to look at next time: a companion growing 30% while
// the others sit still is the shape of prose creep, and it is worth reading
// that file before this budget is argued about again.
//
// THE RAISE TO 272 IS A SURFACE CHANGE, which is the one reason the paragraphs
// above accept. `claim recover-approved-content` is a new leaf, and
// TestRouterSkillNounBlockListsEveryCommandInTheSurface requires the router to
// carry every leaf the binary has — so the router could not stay at 263 and
// stay correct. The verb needed more than its name: it WRITES the lock store,
// so an agent that met it as a bare entry in the noun block would not know that
// it creates no approval, that a revision which does not hash equal is refused,
// or that the human should see the dry run before it runs. That is seven lines,
// and the router went 263 -> 270 for them while no companion moved at all. One
// guide growing for one new leaf, with the other five still, is the surface
// signature this budget exists to distinguish from prose creep. The ceiling is
// 272 rather than 270 so the router is not left with zero headroom, which turns
// the next one-line correction into a budget argument. CENSUS: router 271 of
// 272, claims 255, comments 206, code-links 136, build-order 142. The extra
// line is the `git_unavailable` recovery row for recover-approved-content.
//
// THE RAISE TO 296 IS THE ONE THE BUDGET'S OWN HISTORY DESCRIBES: an agent that
// meets a finding without having read what it is, and loops. Issues #78 and
// #82 recorded the loop: agents reached for `reaudit` on a claim that was not
// pending, for `flag` on a claim that renders from `rows`, for `check
// --validate` after a lock refusal, and for `build-order` when the question
// was whether a feature was done — each a wrong verb that burns an approval.
// The router now carries the two "Which command" tables (fourteen lines) and
// two new refusal rows: `unlinked_claims` from the code-link gate and
// `skills_drift` from `skills export --check`. The retired-verb table was
// compressed from eleven lines to five first, so the raise is what was left
// after the prose was tightened. CENSUS at the raise: router 290 of 296,
// claims 258, comments 212, code-links 164, build-order 147 — every
// companion moved a few lines for the same tables (the shared four-arm
// discriminator in comments and code-links, the read-only track section in
// claims, the status-vs-show line in build-order), which is the surface
// signature: one decision, stated once per guide that owns a piece of it.
//
// NO RAISE FOR #78 PHASE 2. The proof-or-stop rule — admitted evidence only,
// a model's statement in chat is never a certificate — is three lines in the
// router and two in each of the two companions that describe `check`. CENSUS:
// router 294 of 296, claims 260, comments 212, code-links 166, build-order
// 147. The router is now two lines from its ceiling, so the next sentence it
// needs is a budget decision, not a quiet edit.
func TestSkills_StayWithinTheirLineBudget(t *testing.T) {
	const maxLines = 296

	for _, name := range wantSkillNames {
		raw, err := fs.ReadFile(dxskills.FS, name+"/SKILL.md")
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if got := strings.Count(string(raw), "\n"); got > maxLines {
			t.Errorf("%s/SKILL.md is %d lines; the budget is %d", name, got, maxLines)
		}
	}
}

// The rules the release is built around, pinned where an agent will actually
// read them. These are prose assertions on purpose: each one is a sentence that
// has to survive every future rewrite of the skill it lives in, because deleting
// it changes what an agent will do.
func TestSkills_StateTheRulesThatNeverBend(t *testing.T) {
	for _, tc := range []struct {
		skill string
		want  string
		why   string
	}{
		{"dossierx", "unlock → fix → lock", "the approval path, named as the path"},
		{"dossierx", "drift** tool, not the edit tool", "reaudit is not the general edit tool"},
		{"dossierx", "never resolve", "advisory rights"},
		{"dossierx", "score", "resolve the human's words to an id before acting"},
		{"dossierx", "blocked", "a blocked dry run is a successful answer"},
		{"dossierx", "Draft is your workshop", "draft claims are free to author"},
		{"dossierx-claims", "unlock → fix → lock", "the only path through a locked claim"},
		{"dossierx-claims", "not_review_pending", "reaudit refuses a non-drifting claim"},
		{"dossierx-claims", "edit its file freely", "a draft claim needs no ceremony"},
		{"dossierx-code-links", "dossierx-step:", "step tags are scanned with claim tags"},
		{"dossierx-comments", "you never resolve", "the agent replies and waits"},
		{"dossierx-comments", "inclusive", "the inbox cursor re-reports its boundary second"},
		// Issue #82: the decision trees, pinned where the wrong verb is chosen.
		{"dossierx", "Which command", "the flag/unlock/reaudit and track/build-order tables"},
		{"dossierx", "`structured_layout`)", "flag refuses a structured claim; unlock is the path"},
		{"dossierx", "gates nothing and orders nothing", "a track is never a lock gate or a build sequence"},
		{"dossierx", "never a recompute", "build-order show reads the stored artifact"},
		{"dossierx-comments", "structured_layout", "the discriminator's third arm"},
		{"dossierx-code-links", "structured_layout", "the same third arm, same words"},
		{"dossierx-claims", "never a gate, never a build sequence", "track verbs are the read-only axis"},
		{"dossierx-build-order", "never recomputes", "status answers stale; show only renders"},
		// Issue #78 Phase 1A: what a green check proves.
		{"dossierx-code-links", "Linked is not followed", "the gate proves a pointer, not meaning"},
		{"dossierx", "neither proves code links", "--validate and --staged are not sync"},
		{"dossierx", "skills_drift", "a rewritten skill is a refusal, not a soft bump"},
		// Issue #78 Phase 2: proof or stop. Only admitted evidence counts, and
		// a model's statement in chat is never a certificate.
		{"dossierx", "Proof or stop", "chat is not a certificate; stop when the evidence is missing"},
		{"dossierx", "never a certificate", "an agent's 'it is synced' closes no loop"},
		{"dossierx-code-links", "not a certificate and closes nothing", "linked is not followed, and saying so is not evidence"},
		{"dossierx-claims", "an exit code\nyou did not see is one you do not have", "report the envelope, never a belief"},
	} {
		raw, err := fs.ReadFile(dxskills.FS, tc.skill+"/SKILL.md")
		if err != nil {
			t.Fatalf("read %s: %v", tc.skill, err)
		}
		if !strings.Contains(string(raw), tc.want) {
			t.Errorf("%s/SKILL.md no longer states %q (%s)", tc.skill, tc.want, tc.why)
		}
	}
}

// ---------------------------------------------------------------------
// .agents/skills, the lock file and --check (issue #78 Phase 1A)
// ---------------------------------------------------------------------

func TestCLI_SkillsExport_WritesEveryTreeTheRepoAlreadyHas(t *testing.T) {
	project := t.TempDir()
	cfgPath, _ := icWriteFixtureProject(t, project, "widget")
	for _, dir := range []string{".claude", ".agents"} {
		if err := os.MkdirAll(filepath.Join(project, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	stdout, _, err := execCLI(t, "--config", cfgPath, "skills", "export")
	if err != nil {
		t.Fatalf("skills export: %v (out: %s)", err, stdout)
	}
	for _, tree := range []string{claudeSkillsDir, agentsSkillsDir} {
		for _, want := range []string{"dossierx/SKILL.md", skillsLockFile} {
			if _, statErr := os.Stat(filepath.Join(project, filepath.FromSlash(tree), want)); statErr != nil {
				t.Fatalf("expected %s under %s: %v", want, tree, statErr)
			}
		}
	}
	if !strings.Contains(stdout, "skill-tree (agents-skills)") || !strings.Contains(stdout, "skill-tree (claude-code)") {
		t.Fatalf("expected both trees reported as forms, got:\n%s", stdout)
	}

	// The lock names every bundle file with a 64-hex sha256 and this binary's version.
	raw, readErr := os.ReadFile(filepath.Join(project, filepath.FromSlash(agentsSkillsDir), skillsLockFile))
	if readErr != nil {
		t.Fatalf("read lock: %v", readErr)
	}
	var lock skillsLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("decode lock: %v\n%s", err, raw)
	}
	for _, name := range wantSkillNames {
		h, ok := lock.Files[name+"/SKILL.md"]
		if !ok || len(h) != 64 {
			t.Fatalf("lock must carry a sha256 for %s/SKILL.md, got %q (present=%v)", name, h, ok)
		}
	}
	v, _, _ := resolveVersionInfo()
	if lock.Version != v {
		t.Fatalf("lock version = %q, want the binary's %q", lock.Version, v)
	}
}

func TestCLI_SkillsExportCheck_TellsHandEditedFromStaleFromMissing(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "skills")
	if _, _, err := execCLI(t, "skills", "export", targetDir); err != nil {
		t.Fatalf("skills export: %v", err)
	}

	// Clean: every file matches, exit 0.
	env, _, err := execCLIJSON(t, "skills", "export", targetDir, "--check")
	if err != nil || !env.OK {
		t.Fatalf("a fresh export must pass --check, got err=%v env=%+v", err, env)
	}

	// Hand-edit one bundle: it differs from the binary AND from the lock.
	claimsPath := filepath.Join(targetDir, "dossierx-claims", "SKILL.md")
	original, readErr := os.ReadFile(claimsPath)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if err := os.WriteFile(claimsPath, append(original, []byte("\nlocked claims may be edited freely\n")...), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}
	// Make another look stale: rewrite its lock entry to the hash of a
	// different body, then write that body — matches the lock, not the binary.
	lockPath := filepath.Join(targetDir, skillsLockFile)
	rawLock, readErr := os.ReadFile(lockPath)
	if readErr != nil {
		t.Fatalf("read lock: %v", readErr)
	}
	var lock skillsLock
	if err := json.Unmarshal(rawLock, &lock); err != nil {
		t.Fatalf("decode lock: %v", err)
	}
	staleBody := []byte("---\nname: dossierx-build-order\n---\nan older release's guide\n")
	lock.Files["dossierx-build-order/SKILL.md"] = sha256Hex(staleBody)
	encoded, encErr := json.Marshal(lock)
	if encErr != nil {
		t.Fatalf("encode lock: %v", encErr)
	}
	if err := os.WriteFile(lockPath, encoded, 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "dossierx-build-order", "SKILL.md"), staleBody, 0o644); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	// And delete a third.
	if err := os.Remove(filepath.Join(targetDir, "dossierx-comments", "SKILL.md")); err != nil {
		t.Fatalf("remove: %v", err)
	}

	env, _, err = execCLIJSON(t, "skills", "export", targetDir, "--check")
	if err == nil || env.OK || env.Error == nil || env.Error.Code != cliout.CodeSkillsDrift {
		t.Fatalf("expected skills_drift, got err=%v env=%+v", err, env)
	}
	raw, encErr := json.Marshal(env.Data)
	if encErr != nil {
		t.Fatalf("re-marshal data: %v", encErr)
	}
	var data skillsCheckData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode data: %v\n%s", err, raw)
	}
	if len(data.HandEdited) != 1 || !strings.HasSuffix(data.HandEdited[0], "dossierx-claims/SKILL.md") {
		t.Fatalf("hand_edited should name the edited claims skill only, got %+v", data)
	}
	if len(data.Stale) != 1 || !strings.HasSuffix(data.Stale[0], "dossierx-build-order/SKILL.md") {
		t.Fatalf("stale should name the build-order skill only, got %+v", data)
	}
	if len(data.Missing) != 1 || !strings.HasSuffix(data.Missing[0], "dossierx-comments/SKILL.md") {
		t.Fatalf("missing should name the deleted comments skill only, got %+v", data)
	}
	if len(data.NoLock) != 0 {
		t.Fatalf("the lock was present, got no_lock=%v", data.NoLock)
	}
	// The check wrote nothing: the hand edit is still there.
	after, afterErr := os.ReadFile(claimsPath)
	if afterErr != nil {
		t.Fatalf("re-read: %v", afterErr)
	}
	if !strings.Contains(string(after), "edited freely") {
		t.Fatalf("--check must not rewrite files")
	}

	// Text mode names the file and the kind.
	out, _, checkErr := execCLI(t, "skills", "export", targetDir, "--check")
	if checkErr == nil {
		t.Fatalf("text-mode --check must also refuse")
	}
	for _, want := range []string{"hand-edited", "stale", "missing", "3 of 5 file(s) differ"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in text output, got:\n%s", want, out)
		}
	}
}

func TestCLI_SkillsExportCheck_WithoutALockEveryDifferenceIsUnverified(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "skills")
	if _, _, err := execCLI(t, "skills", "export", targetDir); err != nil {
		t.Fatalf("skills export: %v", err)
	}
	if err := os.Remove(filepath.Join(targetDir, skillsLockFile)); err != nil {
		t.Fatalf("remove lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "dossierx", "SKILL.md"), []byte("---\nname: dossierx\n---\nrewritten\n"), 0o644); err != nil {
		t.Fatalf("edit: %v", err)
	}
	env, _, err := execCLIJSON(t, "skills", "export", targetDir, "--check")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeSkillsDrift {
		t.Fatalf("expected skills_drift, got err=%v env=%+v", err, env)
	}
	raw, encErr := json.Marshal(env.Data)
	if encErr != nil {
		t.Fatalf("re-marshal data: %v", encErr)
	}
	var data skillsCheckData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(data.Unverified) != 1 || len(data.HandEdited) != 0 || len(data.Stale) != 0 || len(data.NoLock) != 1 {
		t.Fatalf("without a lock the difference is unverified and the tree is named in no_lock, got %+v", data)
	}
}
