package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestCheck_MissingAndValid(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	claims := []model.Claim{{ID: "widget.contract.bound", Facet: "contract", Module: "widget"}}

	got := Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "missing required") {
		t.Fatalf("missing: %+v", got)
	}

	if err := writeMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	if got := Check(claims, cfg); len(got) != 0 {
		t.Fatalf("valid empty-surface: %+v", got)
	}
}

func TestCheck_CapsAndContractSurface(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	claims := []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget"},
		{ID: "widget.internals.store", Facet: "internals", Module: "widget"},
		{ID: "other.contract.api", Facet: "contract", Module: "other"},
	}

	oversize := strings.Repeat("x", MaxFileBytes+1)
	write(t, dir, "widget/manifest.yaml", "summary: why\nprovides: []\ndepends_on: []\n# "+oversize+"\n")
	got := Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "bytes") {
		t.Fatalf("oversize: %+v", got)
	}

	long := strings.Repeat("w", MaxSummaryRunes+1)
	write(t, dir, "widget/manifest.yaml", "summary: "+long+"\nprovides: []\ndepends_on: []\n")
	got = Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "characters") {
		t.Fatalf("summary cap: %+v", got)
	}

	write(t, dir, "widget/manifest.yaml", "summary: why this module exists for neighbors.\nprovides:\n  - widget.internals.store\ndepends_on: []\n")
	got = Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "contract-surface") {
		t.Fatalf("internals provide: %+v", got)
	}

	write(t, dir, "widget/manifest.yaml", "summary: why this module exists for neighbors.\nprovides:\n  - other.contract.api\ndepends_on: []\n")
	got = Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "belongs to module") {
		t.Fatalf("foreign provide: %+v", got)
	}

	write(t, dir, "widget/manifest.yaml", "summary: why this module exists for neighbors.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - other.contract.api\n")
	if got := Check(claims, cfg); len(got) != 0 {
		t.Fatalf("valid surface lists: %+v", got)
	}

	write(t, dir, "widget/contract/manifest.yaml", "summary: misplaced\nprovides: []\ndepends_on: []\n")
	got = Check(claims, cfg)
	if !hasMsg(got, "not a module manifest") {
		t.Fatalf("misplaced: %+v", got)
	}
}

func TestCheck_StagedTreeNotWorktree(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Modules:   []string{"widget"},
		ClaimsDir: dir,
		ManifestTree: map[string][]byte{
			"widget/manifest.yaml": minimalYAML("widget"),
		},
	}
	// Worktree has no file; overlay must be enough.
	if got := Check(nil, cfg); len(got) != 0 {
		t.Fatalf("overlay: %+v", got)
	}

	write(t, dir, "widget/manifest.yaml", "summary: worktree only — should be ignored\nprovides: []\ndepends_on: []\n")
	cfg.ManifestTree = map[string][]byte{}
	got := Check(nil, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "missing required") {
		t.Fatalf("empty overlay must ignore worktree: %+v", got)
	}
}

func TestCheck_UnknownFieldAndSecondDoc(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	write(t, dir, "widget/manifest.yaml", "summary: why\nprovides: []\ndepends_on: []\nessay: no\n")
	got := Check(nil, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "not a valid") {
		t.Fatalf("unknown field: %+v", got)
	}
	write(t, dir, "widget/manifest.yaml", "summary: why\nprovides: []\ndepends_on: []\n---\nsummary: two\n")
	got = Check(nil, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "more than one YAML document") {
		t.Fatalf("second doc: %+v", got)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasMsg(fs []Finding, sub string) bool {
	for _, f := range fs {
		if strings.Contains(f.Message, sub) {
			return true
		}
	}
	return false
}

// The decisions recorded on Linear NIT-7 (2026-09-24): provides is the
// module's export list, depends_on may only name what a provider exports,
// never this module's own ids, never a module-less id; the claim new stub
// fails until drafted; manifest show gives check's verdict.

func TestCheck_ExportRuleAndDependsOnShape(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
	claims := []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget"},
		{ID: "widget.contract.other", Facet: "contract", Module: "widget"},
		{ID: "lock.contract.store", Facet: "contract", Module: "lock"},
		{ID: "lock.contract.hidden", Facet: "contract", Module: "lock"},
		{ID: "project.roof", Facet: "", Module: ""},
	}
	write(t, dir, "lock/manifest.yaml", "summary: the lock store.\nprovides:\n  - lock.contract.store\ndepends_on: []\n")

	// Self-dependency.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - widget.contract.other\n")
	got := Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "from this module") {
		t.Fatalf("self-dependency: %+v", got)
	}

	// A module-less id (a project claim) is a rests_on target, never a manifest id.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides: []\ndepends_on:\n  - project.roof\n")
	got = Check(claims, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "not a module claim") {
		t.Fatalf("module-less id: %+v", got)
	}

	// The export rule: lock.contract.hidden exists and is contract, but lock
	// does not list it in provides.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides: []\ndepends_on:\n  - lock.contract.hidden\n")
	got = Check(claims, cfg)
	if len(got) != 1 || got[0].Module != "widget" || !strings.Contains(got[0].Message, "does not list in provides") {
		t.Fatalf("export rule: %+v", got)
	}

	// Exported: clean. A cycle between the two modules is legal.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - lock.contract.store\n")
	write(t, dir, "lock/manifest.yaml", "summary: the lock store.\nprovides:\n  - lock.contract.store\ndepends_on:\n  - widget.contract.bound\n")
	if got := Check(claims, cfg); len(got) != 0 {
		t.Fatalf("exported + cycle must be clean: %+v", got)
	}

	// A provider with no manifest carries its own finding; the dependant is
	// not blamed a second time for a rule nobody can evaluate.
	if err := os.Remove(filepath.Join(dir, "lock", "manifest.yaml")); err != nil {
		t.Fatal(err)
	}
	got = Check(claims, cfg)
	if len(got) != 1 || got[0].Module != "lock" || !strings.Contains(got[0].Message, "missing required") {
		t.Fatalf("missing provider blamed once: %+v", got)
	}
}

func TestStubIsRefusedUntilDrafted(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteStub(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	got := Check(nil, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, "empty summary") || !strings.Contains(got[0].Message, DraftCommand("widget")) {
		t.Fatalf("stub must fail and name the drafting command: %+v", got)
	}
	// The missing-file finding names the same command: one recovery, not two.
	if err := os.Remove(filepath.Join(dir, "widget", "manifest.yaml")); err != nil {
		t.Fatal(err)
	}
	got = Check(nil, cfg)
	if len(got) != 1 || !strings.Contains(got[0].Message, DraftCommand("widget")) {
		t.Fatalf("missing must name the drafting command: %+v", got)
	}
	// The test seed stays valid: fixtures are not stubs.
	if err := writeMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	if got := Check(nil, cfg); len(got) != 0 {
		t.Fatalf("minimal seed must pass: %+v", got)
	}
}

func TestShowReportsCheckVerdict(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
	claims := []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "bound"},
		{ID: "lock.contract.hidden", Facet: "contract", Module: "lock", Status: model.StatusDraft, Body: "hidden"},
	}
	write(t, dir, "lock/manifest.yaml", "summary: the lock store.\nprovides: []\ndepends_on: []\n")

	// Missing: the verdict is check's finding, and the isolation view with
	// its draft hints is still assembled — that is the adoption path.
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Findings) != 1 || !strings.Contains(view.Findings[0].Message, "missing required") {
		t.Fatalf("missing via check: %+v", view.Findings)
	}
	if view.Isolation == nil || len(view.Isolation.DraftHints.SuggestedProvides) != 1 {
		t.Fatalf("draft hints while missing: %+v", view.Isolation)
	}

	// The cross-module export rule reaches manifest show too.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides: []\ndepends_on:\n  - lock.contract.hidden\n")
	view, err = Show(claims, cfg, "widget", ShowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Findings) != 1 || !strings.Contains(view.Findings[0].Message, "does not list in provides") {
		t.Fatalf("export rule via show: %+v", view.Findings)
	}
	if len(view.DependsOn) != 1 {
		t.Fatalf("the file is still printed beside its findings: %+v", view)
	}
	// Another module's defect is not this module's verdict.
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides: []\ndepends_on: []\n")
	write(t, dir, "lock/manifest.yaml", "summary: \"\"\nprovides: []\ndepends_on: []\n")
	view, err = Show(claims, cfg, "widget", ShowOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Findings) != 0 {
		t.Fatalf("lock's stub is not widget's finding: %+v", view.Findings)
	}
}
