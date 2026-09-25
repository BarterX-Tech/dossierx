package render

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func writeManifestFile(t *testing.T, claimsDir, module, body string) {
	t.Helper()
	dir := filepath.Join(claimsDir, module)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifest.FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// manifestTabOf builds the module groups the shell renders and returns the
// named module's Manifest tab body, asserting the peer-tab contract on the way.
func manifestTabOf(t *testing.T, claims []model.Claim, cfg *config.Config, module string) string {
	t.Helper()
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	rendered := map[string]template.HTML{}
	for _, c := range claims {
		rendered[c.ID] = template.HTML(`<section class="claim" id="` + c.ID + `"></section>`)
	}
	for _, mg := range buildModuleGroups(buildGroups(cat, cfg, rendered)) {
		if mg.Module != module {
			continue
		}
		var labels []string
		for _, f := range mg.Facets {
			labels = append(labels, f.TabLabel)
		}
		if strings.Join(labels, "|") != "Manifest|Contract|Internals" {
			t.Fatalf("peer tabs = %v, want exactly Manifest|Contract|Internals", labels)
		}
		if mg.FirstFacetID != module+"-contract" {
			t.Fatalf("module opens on %q, want its Contract tab", mg.FirstFacetID)
		}
		if mg.Facets[0].ClaimCount != 0 || len(mg.Facets[0].Claims) != 1 {
			t.Fatalf("Manifest tab must hold exactly one manifest view and no claims: %+v", mg.Facets[0])
		}
		return string(mg.Facets[0].Claims[0])
	}
	t.Fatalf("module %q not rendered", module)
	return ""
}

func manifestTestClaims() []model.Claim {
	return []model.Claim{
		{ID: "widget.contract.bound", Module: "widget", Facet: "contract", Status: model.StatusDraft, Body: "b"},
		{ID: "lock.contract.store", Module: "lock", Facet: "contract", Status: model.StatusDraft, Body: "s"},
	}
}

func TestManifestTab_RefusalShowsChecksFindingsAndCommand(t *testing.T) {
	cases := map[string]string{
		"missing":   "",
		"malformed": "summary: [unclosed\n",
		"oversize":  "summary: x\n#" + strings.Repeat("y", manifest.MaxFileBytes) + "\n",
		"bad deps":  "summary: widget <i>secret</i>.\nprovides: []\ndepends_on:\n  - lock.contract.store\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{Modules: []string{"widget", "lock"}, Facets: []string{"contract", "internals"}, ClaimsDir: dir}
			writeManifestFile(t, dir, "lock", "summary: lock store.\nprovides: []\ndepends_on: []\n")
			if body != "" {
				writeManifestFile(t, dir, "widget", body)
			}
			claims := manifestTestClaims()
			got := manifestTabOf(t, claims, cfg, "widget")

			if !strings.Contains(got, `data-manifest-state="refused"`) {
				t.Fatalf("broken manifest not refused:\n%s", got)
			}
			var n int
			for _, f := range manifest.Check(claims, cfg) {
				if f.Module != "widget" {
					continue
				}
				n++
				want := `<span class="manifest-finding-lint">module-manifest:</span> <span class="manifest-finding-message">` +
					template.HTMLEscapeString(f.Message) + `</span>`
				if !strings.Contains(got, want) {
					t.Errorf("tab lacks check's finding verbatim %q:\n%s", f.Message, got)
				}
			}
			if n == 0 || strings.Count(got, `class="manifest-finding"`) != n {
				t.Fatalf("tab shows %d finding(s), check reports %d", strings.Count(got, `class="manifest-finding"`), n)
			}
			if !strings.Contains(got, `<code class="manifest-command-text">dossierx manifest show widget --isolation</code><button type="button" class="manifest-copy" data-copy-text="dossierx manifest show widget --isolation">Copy</button>`) {
				t.Errorf("tab lacks the copyable draft command:\n%s", got)
			}
			for _, leak := range []string{"manifest-summary", "manifest-raw", "manifest-provides", "secret", "yyyy"} {
				if strings.Contains(got, leak) {
					t.Errorf("refused manifest soft-rendered %q:\n%s", leak, got)
				}
			}
		})
	}
}
