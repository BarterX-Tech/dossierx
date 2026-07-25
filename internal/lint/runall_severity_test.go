package lint

import (
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestRunAll_NormalizesEmptySeverityToError guards DX-AUD-17: roughly a
// dozen lints (dangling, id-shape, ...) build Finding values without setting
// Severity, relying on the zero value being treated as "error". But the zero
// value of Severity is "" (not SeverityError), so a text report printed "[]"
// for the severity and a JSON report emitted Severity:"". RunAll is the one
// place every finding funnels through, so it must fill any empty Severity
// with SeverityError before returning — without each of those ~dozen lint
// files having to set it.
func TestRunAll_NormalizesEmptySeverityToError(t *testing.T) {
	cfg := &config.Config{
		Facets:  []string{"contract"},
		Modules: []string{"widget"},
	}

	claims := []model.Claim{
		// dangling: rests_on an id nothing defines — historically left
		// Severity empty.
		{
			ID:      "widget.contract.dangler",
			Facet:   "contract",
			Module:  "widget",
			Status:  model.StatusDraft,
			Layout:  model.LayoutCard,
			Body:    "x",
			RestsOn: []string{"widget.contract.ghost"},
		},
		// id-shape: an uppercase slug violates the id grammar —
		// historically left Severity empty.
		{
			ID:     "widget.contract.Bad-Slug",
			Facet:  "contract",
			Module: "widget",
			Status: model.StatusDraft,
			Layout: model.LayoutCard,
			Body:   "y",
		},
		// rows-shape: mismatched row columns — already sets SeverityError
		// explicitly; asserted here to confirm the normalization never
		// downgrades an already-correct severity.
		{
			ID:     "widget.contract.tbl",
			Facet:  "contract",
			Module: "widget",
			Status: model.StatusDraft,
			Layout: model.LayoutTable,
			Rows:   []model.Row{{"a": "1"}, {"b": "2"}},
		},
	}

	findings := RunAll(claims, cfg)
	if len(findings) == 0 {
		t.Fatal("expected findings from the fixture, got none")
	}

	byLint := map[string]Severity{}
	for _, f := range findings {
		if f.Severity == "" {
			t.Errorf("finding %+v has an empty Severity; RunAll must normalize it to error", f)
		}
		if f.Severity == SeverityError {
			byLint[f.LintName] = f.Severity
		}
	}

	for _, name := range []string{"dangling", "id-shape", "rows-shape"} {
		if byLint[name] != SeverityError {
			t.Errorf("expected an error-severity %q finding, got severity %q", name, byLint[name])
		}
	}
}
