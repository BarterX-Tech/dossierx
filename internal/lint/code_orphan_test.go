package lint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// testConfig builds a *config.Config carrying the module/facet vocabulary
// these tests' fixture claim ids use (module "widget", facets "contract"
// and "internals"), via the real config.LoadConfig path rather than
// constructing the struct by hand (its dir field is unexported).
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "project.config.yaml")
	body := "schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return cfg
}

func TestCodeOrphan(t *testing.T) {
	cfg := testConfig(t)

	cases := []struct {
		name    string
		claims  []model.Claim
		wantErr bool
	}{
		{
			name: "passing: code block references a real claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Body: "See ```widget.internals.fields``` for the schema."},
				{ID: "widget.internals.fields"},
			},
			wantErr: false,
		},
		{
			name: "passing: no fenced code at all",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Body: "widget.internals.missing is only mentioned in prose."},
			},
			wantErr: false,
		},
		{
			name: "failing: code block references a nonexistent claim",
			claims: []model.Claim{
				{ID: "widget.contract.overview", Body: "```\nwidget.internals.missing\n```"},
			},
			wantErr: true,
		},
	}

	l := codeOrphanLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check(tc.claims, cfg)
			if got := len(findings) > 0; got != tc.wantErr {
				t.Fatalf("findings = %+v, wantErr=%v", findings, tc.wantErr)
			}
			for _, f := range findings {
				if f.Severity != SeverityError {
					t.Errorf("Severity = %q, want error", f.Severity)
				}
			}
		})
	}
}
