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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// The five bundles and the order the router presents them in. Spelled out
// rather than derived so that adding or removing a skill is a deliberate edit
// to a test, the same way cmd/dossierx/main_test.go pins the 19-leaf surface.
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

	// Five bundles plus the generic guide, which is always written — with no
	// project root to put it in, it lands beside the bundles.
	if !strings.Contains(stdout, "wrote 6 file(s)") {
		t.Fatalf("expected stdout to report 6 file(s) written, got:\n%s", stdout)
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
	for _, want := range []string{"# House rules", "Be careful.", agentsBeginMarker, agentsEndMarker, "The seven nouns"} {
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
	for _, want := range []string{"The seven nouns, nineteen leaves", "Five rules that never bend", "unlock → fix → lock"} {
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
			t.Errorf("%s names %q, which is not a command in the v0.3.0 surface (dossierx lint/stale/coverage/deps/implink and comment resolve/reopen/edit/delete are gone). If this is prose, write \"DossierX\" with a capital D.", path, strings.TrimSpace(m[0]))
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
func TestSkills_StayWithinTheirLineBudget(t *testing.T) {
	const maxLines = 230

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
		{"dossierx-comments", "agent_can_resolve", "the rights rule is data, not memory"},
		{"dossierx-comments", "you never resolve", "the agent replies and waits"},
		{"dossierx-comments", "inclusive", "the inbox cursor re-reports its boundary second"},
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
