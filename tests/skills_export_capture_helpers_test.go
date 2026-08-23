// skills_export_capture_helpers_test.go captures what `dossierx skills export`
// actually WRITES, not what skills/*/SKILL.md says. The transform is the thing
// that matters: the bundles are go:embed-ed into the binary and installed into
// OTHER people's repositories, where they are unfixable after the tag. This
// repository's own checked-in `.claude/skills/` symlinks are the narrower,
// separate thing.
//
// Things are named, not numbered. An earlier version of this header called this
// "Surface 12's capture", and two sibling files carried the same style of
// label; nothing defined that numbering, so every one of those labels pointed a
// reader at the wrong entry. Positions also move the moment something is
// inserted, which is why the fix is to cite
// the `name:` key rather than to correct the arithmetic.
//
// WHY A CAPTURE AND NOT A SOURCE READ. cmd/dossierx/skills_embed.go's
// buildAgentGuide and buildAgentsSection both run rewriteWikilinks over the
// bundle bodies and splice/concatenate them into two documents that appear in
// no SKILL.md at all: docs/dossierx-agent-guide.md, and a marker-delimited
// section spliced into the client's own AGENTS.md — the file that file's own
// comment says "is loaded on every single turn in every single conversation
// in the repo". A gate that reads skills/*/SKILL.md is auditing the source;
// this captures the transform's OUTPUT, the bytes a client's repository
// actually receives, so a prose-auditing agent can be handed exactly that.
//
// It runs the REAL, COMPILED "dossierx" binary end to end (reusing
// cli_test.go's binPath/run harness) rather than calling
// buildAgentGuide/buildAgentsSection/exportSkills directly — those are
// unexported in cmd/dossierx and, more to the point, calling them directly
// would be auditing the source's OWN idea of its output, the exact
// distinction this capture exists to avoid collapsing. The capture reads
// every file back off disk the way a client's agent would.
//
// This is a _test.go file, not a plain package file, because it is built on
// top of cli_test.go's binPath/run/TestMain harness — those are themselves
// _test.go-only (a *testing.T threads through every call here) — so a helper
// built on them can only ever be reusable from within `go test`, the same way
// every other capture/audit helper in this package already is.
package tests

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The exported AGENTS.md section's marker comments. These are literal copies
// of cmd/dossierx/skills_embed.go's unexported agentsBeginMarker/
// agentsEndMarker constants — this package cannot import cmd/dossierx's
// unexported identifiers, so the two copies must be kept in sync by hand. A
// drift is not silent: extractAgentsSection below fails loudly (rather than
// mis-extracting) the moment a generated AGENTS.md no longer contains these
// exact strings, which is exactly the failure mode a hand-copied constant
// needs.
const (
	skillsExportAgentsBeginMarker = "<!-- BEGIN dossierx skills -->"
	skillsExportAgentsEndMarker   = "<!-- END dossierx skills -->"

	// skillsExportAgentGuidePath is a literal copy of
	// cmd/dossierx/skills_embed.go's unexported agentGuidePath constant, for
	// the same reason as the two markers above: it is the exact link target
	// buildAgentsSection writes for each companion's index bullet
	// ("- [`name`](docs/dossierx-agent-guide.md#name) — description"), and
	// TestCaptureSkillsExport_AllThreeFormsPresent needs that exact string to
	// tell "the companion was indexed" apart from "the companion's name
	// merely appears somewhere in the router's own prose".
	skillsExportAgentGuidePath = "docs/dossierx-agent-guide.md"
)

// SkillsExportCapture is export-output.json: everything a client's repository
// receives from one `dossierx skills export` run, in the form the release
// gate hands to a prose-auditing agent instead of skills/*/SKILL.md.
type SkillsExportCapture struct {
	// SkillTree is the verbatim SKILL.md tree (Form 1), keyed by its path
	// relative to the export target directory (e.g.
	// "dossierx-claims/SKILL.md"). This form is copied byte for byte from the
	// embedded bundles with NO wikilink rewrite, so it is the baseline the
	// other two forms are a transform OF.
	SkillTree map[string]string `json:"skill_tree"`

	// AgentGuide is docs/dossierx-agent-guide.md in full (Form 3): every
	// bundle concatenated, wikilinks rewritten to in-document anchors. Always
	// present — this form is written unconditionally.
	AgentGuide string `json:"agent_guide"`

	// AgentsMDSection is exactly the text between the BEGIN/END markers
	// spliced into the client's AGENTS.md (Form 2) — the router's body, with
	// its wikilinks rewritten to point at AgentGuide, plus the companion
	// index. Empty when the fixture's AGENTS.md did not exist (this
	// capture's fixture always creates one, so in practice it never is).
	AgentsMDSection string `json:"agents_md_section"`
}

// captureSkillsExport runs `dossierx skills export` against projectRoot —
// which must already have a .claude/ directory and an AGENTS.md, so all
// three forms actually fire, per exportSkillForms's detection rules in
// cmd/dossierx/skills_embed.go — and reads every written file back.
func captureSkillsExport(t *testing.T, projectRoot string) SkillsExportCapture {
	t.Helper()

	stdout, stderr, code := run(t, projectRoot, "skills", "export")
	if code != 0 {
		t.Fatalf("dossierx skills export: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	treeDir := filepath.Join(projectRoot, ".claude", "skills")
	tree := map[string]string{}
	if err := filepath.WalkDir(treeDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(treeDir, path)
		if relErr != nil {
			return relErr
		}
		tree[filepath.ToSlash(rel)] = string(data)
		return nil
	}); err != nil {
		t.Fatalf("read exported skill tree under %s: %v", treeDir, err)
	}
	if len(tree) == 0 {
		t.Fatalf("captured no files under %s; skills export did not write the tree the fixture expected", treeDir)
	}

	guidePath := filepath.Join(projectRoot, "docs", "dossierx-agent-guide.md")
	guide, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("read exported agent guide %s: %v", guidePath, err)
	}

	agentsPath := filepath.Join(projectRoot, "AGENTS.md")
	agentsMD, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read exported %s: %v", agentsPath, err)
	}
	section, err := extractAgentsSection(string(agentsMD))
	if err != nil {
		t.Fatalf("%s: %v", agentsPath, err)
	}

	return SkillsExportCapture{
		SkillTree:       tree,
		AgentGuide:      string(guide),
		AgentsMDSection: section,
	}
}

// extractAgentsSection returns exactly the marker-delimited block spliceAgentsSection
// (cmd/dossierx/skills_embed.go) writes into AGENTS.md, markers included.
func extractAgentsSection(agentsMD string) (string, error) {
	begin := strings.Index(agentsMD, skillsExportAgentsBeginMarker)
	if begin < 0 {
		return "", fmt.Errorf("no %q marker found; either skills export did not run or the marker text has drifted from cmd/dossierx/skills_embed.go", skillsExportAgentsBeginMarker)
	}
	endMarkerAt := strings.Index(agentsMD[begin:], skillsExportAgentsEndMarker)
	if endMarkerAt < 0 {
		return "", fmt.Errorf("no %q marker found after the BEGIN marker; either skills export did not run or the marker text has drifted from cmd/dossierx/skills_embed.go", skillsExportAgentsEndMarker)
	}
	end := begin + endMarkerAt + len(skillsExportAgentsEndMarker)
	return agentsMD[begin:end], nil
}

// firstBodyLine returns the first non-blank line of raw's markdown body,
// after its YAML frontmatter. raw is a verbatim SkillTree entry (Form 1,
// untouched by rewriteWikilinks), so this is exactly the heading line a
// companion's SKILL.md opens with — used as a distinctive fingerprint for
// "this companion's body was inlined in full" that doesn't require
// hardcoding each companion's heading text by hand and re-syncing it every
// time a SKILL.md is edited.
func firstBodyLine(t *testing.T, raw string) string {
	t.Helper()
	const openDelim = "---\n"
	if !strings.HasPrefix(raw, openDelim) {
		t.Fatalf("expected the raw SKILL.md to start with %q, got:\n%s", openDelim, raw)
	}
	rest := raw[len(openDelim):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		t.Fatalf("expected a closing frontmatter delimiter in:\n%s", raw)
	}
	body := rest[end+len("\n---\n"):]
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	t.Fatalf("companion body has no content lines after its frontmatter:\n%s", raw)
	return ""
}

// sortedSkillTreeKeys is a small helper so tests and any future consumer can
// report SkillTree's contents in a stable order (map iteration is not).
func sortedSkillTreeKeys(tree map[string]string) []string {
	keys := make([]string, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
