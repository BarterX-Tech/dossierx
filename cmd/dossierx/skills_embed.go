// skills_embed.go wires the embedded skills/ directory into the CLI as
// "dossierx skills export [dir]" — the command a bootstrap agent runs once, in
// a project that has the dossierx BINARY but not this repository, to install the
// guide that teaches it how to operate DossierX.
//
// # WHY THIS IS NOT JUST A FILE COPY
//
// The skill bundles under skills/ are ONE markdown source, but every agent
// harness reads its instructions from a different place and in a different
// shape: Claude Code loads .claude/skills/<name>/SKILL.md on demand, Codex reads
// a single always-on AGENTS.md at the repo root, and Pi, Kimi, an editor plugin
// or a human want one self-contained document. Maintaining three copies of the
// same rules is how they drift, and a drifted rule about locked claims is worse
// than no rule. So this file DERIVES the other two forms from the same embedded
// bytes at export time:
//
//	.claude/skills/dossierx*/SKILL.md      verbatim, frontmatter included
//	AGENTS.md                              a marker-delimited section, idempotent
//	docs/dossierx-agent-guide.md           every bundle concatenated, self-contained
//
// Detection, not creation: the SKILL.md tree is written when a .claude/
// directory already exists (or when the caller names a directory explicitly),
// and the AGENTS.md section is maintained only when AGENTS.md already exists —
// creating either unasked would be this command inventing a harness the project
// does not use. The generic guide is ALWAYS written, because it is the form that
// needs no harness at all and is therefore the only one guaranteed to be read.
//
// The positional <dir> argument is unchanged from v0.2.x and still means exactly
// what it always did: write the skill bundles THERE. It is optional now, which
// is what makes the detection path reachable.
//
// The actual //go:embed directive lives in
// github.com/BarterX-Tech/dossierx/skills (see skills/embed.go), not here.
// An embed pattern must not contain ".." path elements, so a file under
// cmd/dossierx/ cannot embed the repo-root skills/ directory directly.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	dxskills "github.com/BarterX-Tech/dossierx/skills"
)

// The three names this command writes, and the markers that make the AGENTS.md
// form idempotent.
//
// The markers are HTML comments so they are invisible in every markdown renderer
// while still being exact, greppable anchors. Re-running the export replaces
// everything between them and touches nothing else in the file, which is the
// whole point: AGENTS.md belongs to the project, not to DossierX, and a project
// will have its own instructions above and below this section.
// agentGuidePath is BOTH the path Form 3 writes the guide to under a project
// root AND the href the Form 2 AGENTS.md section links it by, and that is what
// makes the link resolve rather than a coincidence worth re-checking. Form 2 is
// written only when root != "" (see exportSkillForms), and in exactly that case
// Form 3's target is filepath.Join(root, agentGuidePath) while AGENTS.md itself
// is filepath.Join(root, agentsFileName) — same root, so this repo-relative href
// is correct from AGENTS.md every time the section exists at all. The rootless
// export, where the guide lands beside the bundles as
// <explicitDir>/dossierx-agent-guide.md instead, writes NO AGENTS.md section, so
// no link to the moved guide is ever emitted. One constant serving both jobs is
// deliberate: two constants could disagree, and a wrong link here points a
// client's always-on instructions at a file that is not there.
const (
	agentsFileName  = "AGENTS.md"
	agentGuidePath  = "docs/dossierx-agent-guide.md"
	claudeSkillsDir = ".claude/skills"

	agentsBeginMarker = "<!-- BEGIN dossierx skills -->"
	agentsEndMarker   = "<!-- END dossierx skills -->"
)

// newSkillsCmd is the "dossierx skills" command group.
func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install the embedded DossierX agent skills (dossierx router, claims, comments, build-order, code-links) in whatever form this repo's harness reads",
	}
	cmd.AddCommand(newSkillsExportCmd())
	return commandGroup(cmd)
}

// skillsExportForm is one written form in the export's machine payload. The
// per-form breakdown exists because "did my harness get its copy?" is the only
// question a bootstrap agent has after running this, and a flat file list makes
// it guess: .claude/skills/dossierx/SKILL.md and docs/dossierx-agent-guide.md
// are the same content for two different readers, and only the Harness field
// says which reader each one serves.
type skillsExportForm struct {
	Harness string   `json:"harness"`
	Form    string   `json:"form"`
	Path    string   `json:"path"`
	Action  string   `json:"action"`
	Written []string `json:"written,omitempty"`
}

// skillsExportData is "dossierx skills export"'s machine payload.
//
// TargetDir, Written and Count predate the multi-form export and keep their
// v0.2.x meanings (Written is every path this run wrote, Count its length), so
// a consumer that only ever read those three still works. Skipped names the
// forms that were NOT written and why, because "nothing happened for Codex" and
// "this repo has no AGENTS.md, so there was nothing to update" are very
// different answers and only one of them is a reason to look for a bug.
type skillsExportData struct {
	TargetDir   string             `json:"target_dir"`
	ProjectRoot string             `json:"project_root,omitempty"`
	Forms       []skillsExportForm `json:"forms"`
	Skipped     []string           `json:"skipped"`
	Written     []string           `json:"written"`
	Count       int                `json:"count"`
}

func newSkillsExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export [dir]",
		Short: "Write the embedded skills in every form this repo uses: a SKILL.md tree, an idempotent AGENTS.md section, and a self-contained agent guide",
		Args:  cobra.MaximumNArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			explicitDir := ""
			if len(args) == 1 {
				explicitDir = args[0]
			}
			data, err := exportSkillForms(dxskills.FS, explicitDir, skillsExportRoot())
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "skills export: %w", err)
			}
			return cmdResult{
				Data: data,
				Text: func() { writeSkillsExportText(cmd.OutOrStdout(), data) },
			}, nil
		}),
	}
}

// skillsExportRoot is the directory the AGENTS.md section and the generic guide
// are written relative to: the project root, i.e. the directory holding
// project.config.yaml.
//
// It returns "" rather than an error when there is no project, because "export
// the bundles into this directory" is a legitimate call in a repo DossierX has
// not been set up in yet — an agent may install the guides to read before any
// project exists. The documented BOOTSTRAP sequence, though, creates the config
// FIRST and exports after (skills/dossierx/SKILL.md), precisely because only a
// rooted export maintains the AGENTS.md section and places the guide at
// docs/dossierx-agent-guide.md. An unrooted export writes the bundles and the
// guide into the named directory and leaves the repo alone; it never guesses
// where a project root might be.
//
// Every failure mode collapses to the same answer for that reason — a missing
// config, an unreadable cwd, a --config naming a file that is not there — because
// they all mean "no project root", and none of them is fatal to this one command.
func skillsExportRoot() string {
	path, err := resolveConfigPath()
	if err != nil {
		return ""
	}
	// An explicit --config may name a file that does not exist; that is not a
	// project root, and treating it as one would create docs/ next to a path the
	// caller only imagined.
	if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return filepath.Dir(abs)
}

// exportSkillForms writes every applicable form and reports what it did.
//
// The resolution order is deliberate and is the entire contract of this command:
//
//	skill tree   explicitDir if given; else <root>/.claude/skills if .claude
//	             exists; else skipped (no harness of that shape here)
//	AGENTS.md    <root>/AGENTS.md, only if it already exists
//	guide        <root>/docs/dossierx-agent-guide.md, or <explicitDir>/ when
//	             there is no project root — always written, somewhere
//
// The one hard error is having nowhere to write anything at all: no directory
// argument AND no project root. Succeeding silently there would tell a bootstrap
// agent the guide is installed when nothing was.
func exportSkillForms(embedded fs.FS, explicitDir, root string) (skillsExportData, error) {
	data := skillsExportData{
		ProjectRoot: root,
		Forms:       []skillsExportForm{},
		Skipped:     []string{},
		Written:     []string{},
	}
	if explicitDir == "" && root == "" {
		return data, errors.New("no directory given and no project.config.yaml found, so there is nowhere to install the skills; pass a directory (e.g. \"dossierx skills export .claude/skills\") or run this from inside the project")
	}

	// --- Form 1: the SKILL.md tree, verbatim. ---
	treeDir := explicitDir
	if treeDir == "" {
		if _, err := os.Stat(filepath.Join(root, ".claude")); err == nil {
			treeDir = filepath.Join(root, claudeSkillsDir)
		}
	}
	if treeDir == "" {
		data.Skipped = append(data.Skipped, "claude-code skill tree: no .claude/ directory in "+root+" and no directory argument given")
	} else {
		written, err := exportSkills(embedded, treeDir)
		if err != nil {
			return data, err
		}
		data.TargetDir = treeDir
		data.Written = append(data.Written, written...)
		data.Forms = append(data.Forms, skillsExportForm{
			Harness: "claude-code", Form: "skill-tree", Path: treeDir,
			Action: "written", Written: written,
		})
	}

	// --- Form 2: the AGENTS.md section, only into a file that already exists. ---
	if root != "" {
		agentsPath := filepath.Join(root, agentsFileName)
		if existing, err := os.ReadFile(agentsPath); err == nil {
			section, buildErr := buildAgentsSection(embedded)
			if buildErr != nil {
				return data, buildErr
			}
			updated := spliceAgentsSection(string(existing), section)
			action := "unchanged"
			if updated != string(existing) {
				if err := os.WriteFile(agentsPath, []byte(updated), 0o644); err != nil {
					return data, fmt.Errorf("write %s: %w", agentsPath, err)
				}
				action = "updated"
				data.Written = append(data.Written, agentsPath)
			}
			data.Forms = append(data.Forms, skillsExportForm{
				Harness: "codex", Form: "agents-md-section", Path: agentsPath, Action: action,
			})
		} else {
			data.Skipped = append(data.Skipped, "codex AGENTS.md section: no "+agentsPath+" to update (this command does not create one)")
		}
	}

	// --- Form 3: the generic guide, always. ---
	guidePath := filepath.Join(root, filepath.FromSlash(agentGuidePath))
	if root == "" {
		guidePath = filepath.Join(explicitDir, filepath.Base(agentGuidePath))
	}
	guide, err := buildAgentGuide(embedded)
	if err != nil {
		return data, err
	}
	if err := os.MkdirAll(filepath.Dir(guidePath), 0o755); err != nil {
		return data, fmt.Errorf("create dir for %s: %w", guidePath, err)
	}
	if err := os.WriteFile(guidePath, []byte(guide), 0o644); err != nil {
		return data, fmt.Errorf("write %s: %w", guidePath, err)
	}
	data.Written = append(data.Written, guidePath)
	data.Forms = append(data.Forms, skillsExportForm{
		Harness: "any", Form: "generic-guide", Path: guidePath, Action: "written",
	})

	data.Count = len(data.Written)
	return data, nil
}

// writeSkillsExportText renders the export for a human reading the terminal.
// Per-file lines first (a bootstrap agent pastes these into chat as evidence
// that the install happened), then one line per form naming the harness it
// serves and one line per skipped form saying why it was skipped — because "no
// AGENTS.md here" is information, and silence about it is not.
func writeSkillsExportText(out io.Writer, data skillsExportData) {
	for _, p := range data.Written {
		fmt.Fprintf(out, "skills export: wrote %s\n", p)
	}
	for _, f := range data.Forms {
		fmt.Fprintf(out, "skills export: %s (%s) -> %s [%s]\n", f.Form, f.Harness, f.Path, f.Action)
	}
	for _, s := range data.Skipped {
		fmt.Fprintf(out, "skills export: skipped %s\n", s)
	}
	fmt.Fprintf(out, "skills export: wrote %d file(s)\n", data.Count)
}

// exportSkills walks every file in embedded and writes it under targetDir,
// preserving the embedded path layout (e.g. dossierx-claims/SKILL.md).
// Parent directories are created as needed; existing files are
// overwritten. Returns the list of paths written, in walk order.
func exportSkills(embedded fs.FS, targetDir string) ([]string, error) {
	var written []string
	err := fs.WalkDir(embedded, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(embedded, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		outPath := filepath.Join(targetDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", outPath, err)
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		written = append(written, outPath)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// skillDoc is one parsed bundle: its frontmatter identity plus the markdown
// below the frontmatter. The derived forms need the two apart — frontmatter is
// Claude Code's loading metadata and is meaningless (worse: it renders as a
// stray table) inside AGENTS.md or a standalone document, while Description is
// exactly the "load this when…" sentence the other forms need in their index.
type skillDoc struct {
	Name        string
	Description string
	Body        string
}

// loadSkillDocs reads and parses every bundle in skills.Order — the declared
// reading order, not the order a directory walk happens to return.
//
// It cross-checks the list against the embedded directories and fails when they
// disagree, in BOTH directions: a name in Order with no directory is a typo that
// would otherwise silently drop a skill from the guide, and a directory absent
// from Order is a new bundle whose place in the reading order nobody decided. The
// error names the offender rather than quietly exporting four of five skills,
// because a guide missing a section reads as complete.
func loadSkillDocs(embedded fs.FS) ([]skillDoc, error) {
	entries, err := fs.ReadDir(embedded, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills: %w", err)
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			onDisk[e.Name()] = true
		}
	}
	for _, name := range dxskills.Order {
		if !onDisk[name] {
			return nil, fmt.Errorf("skills.Order names %q, which is not an embedded directory", name)
		}
		delete(onDisk, name)
	}
	if len(onDisk) > 0 {
		leftover := make([]string, 0, len(onDisk))
		for name := range onDisk {
			leftover = append(leftover, name)
		}
		sort.Strings(leftover)
		return nil, fmt.Errorf("embedded skill(s) %s are not placed in skills.Order; decide where in the reading order they belong", strings.Join(leftover, ", "))
	}

	docs := make([]skillDoc, 0, len(dxskills.Order))
	for _, name := range dxskills.Order {
		raw, err := fs.ReadFile(embedded, name+"/SKILL.md")
		if err != nil {
			return nil, fmt.Errorf("read embedded %s/SKILL.md: %w", name, err)
		}
		doc, err := parseSkillDoc(raw)
		if err != nil {
			return nil, fmt.Errorf("%s/SKILL.md: %w", name, err)
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

// parseSkillDoc splits a SKILL.md into its YAML frontmatter and its body.
//
// The frontmatter is parsed with yaml.v3 rather than scanned for "name:"
// because the description is a folded (">-") multi-line scalar, and a line-based
// scan would either truncate it at the first newline or have to reimplement YAML
// folding. Frontmatter is required: a bundle without it cannot be loaded by
// Claude Code either, so failing loudly here catches the mistake at export time
// instead of leaving an unloadable file on a user's disk.
func parseSkillDoc(raw []byte) (skillDoc, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return skillDoc{}, errors.New("missing YAML frontmatter (expected a leading \"---\" line)")
	}
	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return skillDoc{}, errors.New("unterminated YAML frontmatter (no closing \"---\" line)")
	}
	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end+1]), &front); err != nil {
		return skillDoc{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if front.Name == "" {
		return skillDoc{}, errors.New("frontmatter has no name")
	}
	body := strings.TrimLeft(rest[end+len("\n---\n"):], "\n")
	return skillDoc{
		Name:        front.Name,
		Description: strings.Join(strings.Fields(front.Description), " "),
		Body:        strings.TrimRight(body, "\n") + "\n",
	}, nil
}

// buildAgentGuide renders the harness-independent document: every bundle, in
// full, in one file, with an index.
//
// "Self-contained" is the requirement it is built to. A harness that supports
// nothing but a path to a markdown file gets the same rules as Claude Code
// does, with no cross-file references to resolve — so the [[wikilinks]] the
// bundles use between each other are rewritten to in-document anchors, and each
// section is preceded by an explicit HTML anchor rather than relying on whatever
// heading-slug algorithm the reader's renderer happens to use.
func buildAgentGuide(embedded fs.FS) (string, error) {
	docs, err := loadSkillDocs(embedded)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("# DossierX — agent guide\n\n")
	b.WriteString("Generated by `dossierx skills export` from the skill bundles embedded in the DossierX\n")
	b.WriteString("binary. Do not edit this file: re-run the export to refresh it.\n\n")
	b.WriteString("This is the harness-independent form. It contains every DossierX skill in full, so it\n")
	b.WriteString("needs no plugin, no frontmatter support and no skill loader — read the first section\n")
	b.WriteString("always, and the section the first one sends you to.\n\n")
	for _, doc := range docs {
		fmt.Fprintf(&b, "- [`%s`](#%s) — %s\n", doc.Name, doc.Name, doc.Description)
	}
	names := docNames(docs)
	for _, doc := range docs {
		fmt.Fprintf(&b, "\n---\n\n<a id=\"%s\"></a>\n\n%s", doc.Name, rewriteSkillLinks(doc.Body, "", names))
	}
	return b.String(), nil
}

// buildAgentsSection renders the always-on form: the ROUTER's body, plus an
// index pointing at the guide for everything else.
//
// It carries the router and only the router on purpose. An AGENTS.md section is
// loaded on every single turn in every single conversation in the repo, whether
// or not DossierX is what the agent is working on, so the budget is far tighter
// than a skill file's — and the router is precisely the part that must always be
// resident (the contract, the two roles, the rules that never bend, and where to
// read the rest). Inlining all five would pay for four skills' worth of context
// on every unrelated turn.
func buildAgentsSection(embedded fs.FS) (string, error) {
	docs, err := loadSkillDocs(embedded)
	if err != nil {
		return "", err
	}
	if len(docs) == 0 || docs[0].Name != dxskills.RouterName {
		return "", fmt.Errorf("embedded skills: expected %q first, cannot build the AGENTS.md section", dxskills.RouterName)
	}

	var b strings.Builder
	b.WriteString(agentsBeginMarker + "\n")
	b.WriteString("<!-- Generated by \"dossierx skills export\". Everything between these two markers is\n")
	b.WriteString("     regenerated on each export; edit the skills, not this. Text outside them is left\n")
	b.WriteString("     untouched. -->\n\n")
	b.WriteString(rewriteSkillLinks(docs[0].Body, agentGuidePath, docNames(docs)))
	b.WriteString("\nThe companion guides are in [`" + agentGuidePath + "`](" + agentGuidePath + "):\n\n")
	for _, doc := range docs[1:] {
		fmt.Fprintf(&b, "- [`%s`](%s#%s) — %s\n", doc.Name, agentGuidePath, doc.Name, doc.Description)
	}
	b.WriteString("\n" + agentsEndMarker + "\n")
	return b.String(), nil
}

// spliceAgentsSection returns existing with section substituted for whatever
// currently sits between the markers, or appended if the markers are absent.
//
// Idempotence is the property that matters: running the export twice must
// produce byte-identical files, and running it on a hand-trimmed AGENTS.md must
// not stack a second copy. Both fall out of splicing on the markers rather than
// appending unconditionally. A file containing a BEGIN with no END is treated as
// having no section (the malformed remnant is left in place and a fresh section
// appended) rather than swallowing the entire tail of someone's instructions.
func spliceAgentsSection(existing, section string) string {
	begin := strings.Index(existing, agentsBeginMarker)
	if begin >= 0 {
		if end := strings.Index(existing[begin:], agentsEndMarker); end >= 0 {
			tail := begin + end + len(agentsEndMarker)
			// Keep whatever followed the old section's closing marker verbatim,
			// minus the newline the section itself now supplies.
			return existing[:begin] + section + strings.TrimPrefix(existing[tail:], "\n")
		}
	}
	prefix := existing
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	if prefix != "" {
		prefix += "\n"
	}
	return prefix + section
}

// siblingSkillHref is how a bundle SPELLS a cross-reference to another bundle in
// its own source: a link relative to its own directory in the exported tree.
//
// The spelling is chosen for the one form nothing rewrites. exportSkills copies
// the bundles out byte for byte, preserving the embedded layout, so
// <dir>/dossierx-claims/SKILL.md is always one "../" away from
// <dir>/dossierx-code-links/SKILL.md — in the export, and in this repository,
// where .claude/skills/* symlinks straight onto skills/*. Anything that has to be
// rewritten to resolve is wrong in that form by construction, which is what the
// old [[wikilink]] spelling was: it reached the other two forms as a real link
// and reached a client's agent as the four characters "[[" and "]]".
func siblingSkillHref(name string) string {
	return "../" + name + "/SKILL.md"
}

// rewriteSkillLinks retargets those sibling links for the two DERIVED forms,
// which are single documents and have no sibling files to reach.
//
// names comes from the bundles that were actually loaded, so a link naming a
// bundle that does not exist is left exactly as the author wrote it — visible,
// and therefore fixable — rather than retargeted at an anchor that goes nowhere.
// prefix is the file the anchors live in: "" when the target is the same document
// (the guide), and the guide's path when the body is being inlined somewhere else
// (AGENTS.md).
func rewriteSkillLinks(body, prefix string, names []string) string {
	out := body
	for _, name := range names {
		out = strings.ReplaceAll(out, "]("+siblingSkillHref(name)+")", "]("+prefix+"#"+name+")")
	}
	return out
}

// docNames is the loaded bundles' names, in load order.
func docNames(docs []skillDoc) []string {
	names := make([]string, 0, len(docs))
	for _, d := range docs {
		names = append(names, d.Name)
	}
	return names
}
