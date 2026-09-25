package manifest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func viewerFixtureClaims() []model.Claim {
	return []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "bound"},
		{ID: "lock.contract.store", Facet: "contract", Module: "lock", Status: model.StatusDraft, Body: "store"},
	}
}

// findingsFor is check's verdict filtered exactly the way manifest show
// filters it — the reference the viewer projection must equal.
func findingsFor(claims []model.Claim, cfg *config.Config, module string) []Finding {
	var out []Finding
	for _, f := range Check(claims, cfg) {
		if f.Module == module || f.Module == "" {
			out = append(out, f)
		}
	}
	return out
}

func TestViewerHealthyCarriesTheFileAndNoFindings(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
	widgetYAML := "summary: widget <b>boundary</b>.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - lock.contract.store\n"
	write(t, dir, "widget/manifest.yaml", widgetYAML)
	write(t, dir, "lock/manifest.yaml", "summary: lock store.\nprovides:\n  - lock.contract.store\ndepends_on: []\n")

	views := Viewer(viewerFixtureClaims(), cfg)
	if len(views) != 2 {
		t.Fatalf("want one view per configured module, got %d", len(views))
	}
	v := views["widget"]
	if !v.Healthy() || len(v.Findings) != 0 {
		t.Fatalf("healthy manifest reported findings: %+v", v.Findings)
	}
	if v.Path != "widget/manifest.yaml" || v.Command != "dossierx manifest show widget --isolation" {
		t.Fatalf("path/command: %q %q", v.Path, v.Command)
	}
	if v.Raw != widgetYAML {
		t.Fatalf("raw text must be the file verbatim, got %q", v.Raw)
	}
	if v.Manifest.Summary != "widget <b>boundary</b>." ||
		!reflect.DeepEqual(v.Manifest.Provides, []string{"widget.contract.bound"}) ||
		!reflect.DeepEqual(v.Manifest.DependsOn, []string{"lock.contract.store"}) {
		t.Fatalf("decoded manifest: %+v", v.Manifest)
	}
}

// Every broken shape the ticket names must carry check's own finding text and
// nothing of the file: never a half-decoded summary or raw bytes.
func TestViewerRefusalsAreChecksFindingsVerbatim(t *testing.T) {
	cases := []struct {
		name   string
		widget string // "" = file missing
		want   string // substring of the finding
	}{
		{"missing", "", "is missing required widget/manifest.yaml"},
		{"oversize", "summary: x\n#" + strings.Repeat("x", MaxFileBytes) + "\n", "module manifests must be at most 4096 bytes"},
		{"malformed", "summary: [unclosed\n", "is not a valid module manifest"},
		{"empty summary", "summary: \"\"\nprovides: []\ndepends_on: []\n", "has an empty summary"},
		{"depends on unexported id", "summary: widget.\nprovides: []\ndepends_on:\n  - lock.contract.store\n", "does not list in provides"},
		{"depends on unknown id", "summary: widget.\nprovides: []\ndepends_on:\n  - lock.contract.nope\n", "names unknown claim lock.contract.nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
			write(t, dir, "lock/manifest.yaml", "summary: lock store.\nprovides: []\ndepends_on: []\n")
			if tc.widget != "" {
				write(t, dir, "widget/manifest.yaml", tc.widget)
			}
			claims := viewerFixtureClaims()
			v := Viewer(claims, cfg)["widget"]
			if v.Healthy() {
				t.Fatal("a broken manifest must not project as healthy")
			}
			if !reflect.DeepEqual(v.Findings, findingsFor(claims, cfg, "widget")) {
				t.Fatalf("viewer findings differ from check's:\n viewer %+v\n check  %+v", v.Findings, findingsFor(claims, cfg, "widget"))
			}
			if !strings.Contains(v.Findings[0].Message, tc.want) {
				t.Fatalf("finding %q does not contain %q", v.Findings[0].Message, tc.want)
			}
			if v.Raw != "" || v.Manifest.Summary != "" || len(v.Manifest.Provides)+len(v.Manifest.DependsOn) != 0 {
				t.Fatalf("a refused manifest carried content: %+v", v)
			}
			if v.Command != DraftCommand("widget") {
				t.Fatalf("command: %q", v.Command)
			}
			if !Viewer(claims, cfg)["lock"].Healthy() {
				t.Fatalf("a broken widget manifest must not refuse lock: %+v", Viewer(claims, cfg)["lock"].Findings)
			}
		})
	}
}

// A misplaced manifest is a project-wide finding; manifest show carries it
// for every module, and so does the viewer.
func TestViewerCarriesProjectWideFindings(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	write(t, dir, "widget/manifest.yaml", "summary: widget.\nprovides: []\ndepends_on: []\n")
	write(t, dir, "stray/manifest.yaml", "summary: stray.\n")
	v := Viewer(nil, cfg)["widget"]
	if v.Healthy() || !strings.Contains(v.Findings[0].Message, "stray/manifest.yaml is not a module manifest") {
		t.Fatalf("project-wide finding missing: %+v", v.Findings)
	}
}

func TestViewerNilConfig(t *testing.T) {
	if Viewer(nil, nil) != nil {
		t.Fatal("nil config must project nothing")
	}
}
