package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestRenderConformanceProjectionAndEscaping(t *testing.T) {
	claim := claimFor(model.LayoutCard)
	cat, err := catalog.Build([]model.Claim{claim}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cat.SetConformance(&conformance.Report{Results: []conformance.Result{{
		ClaimID: claim.ID, Mode: model.EmbodimentModeCompare, ImplementationReady: false,
		Checks: []conformance.CheckResult{
			{
				ID: `public-values<&"`, Adapter: `<script>alert("adapter")</script>`, Target: `target<&>`, Shape: model.ExpectationShapeSet,
				State: conformance.StateMismatch, Expected: []string{"ready", `<img src=x onerror=alert(1)>`}, Observed: []string{"paused"},
				Missing: []string{"ready"}, Extra: []string{"paused"}, Reason: "expected and observed sets differ",
			},
			{
				ID: "schema-version", Adapter: "schema/v1", Target: "schema://widget", Shape: model.ExpectationShapeScalar,
				State: conformance.StateMismatch, Expected: "3", Observed: "4", Reason: "expected and observed scalars differ",
			},
		},
	}}})
	out, err := Render(cat, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Implementation conformance", "public-values", "schema-version", "mismatch", "shape", "scalar", "missing", "ready", "extra", "paused", "Ready here means every declared check"} {
		if !strings.Contains(out, want) {
			t.Fatalf("viewer missing %q", want)
		}
	}
	if !strings.Contains(out, "dossierx-conformance-status-freshness") {
		t.Fatal("conformance viewer omitted the status freshness guard")
	}
	for _, unsafe := range []string{`public-values<&"`, `<script>alert("adapter")</script>`, `<img src=x onerror=alert(1)>`} {
		if strings.Contains(out, unsafe) {
			t.Fatalf("viewer emitted unescaped project value %q", unsafe)
		}
	}
	scalarStart := strings.Index(out, `data-check-id="schema-version"`)
	if scalarStart < 0 {
		t.Fatal("viewer omitted scalar check")
	}
	scalarEnd := strings.Index(out[scalarStart:], `</article>`)
	if scalarEnd < 0 {
		t.Fatal("viewer emitted an unterminated scalar check")
	}
	scalar := out[scalarStart : scalarStart+scalarEnd]
	if !strings.Contains(scalar, `<span>expected:</span> <code>3</code>`) || !strings.Contains(scalar, `<span>observed:</span> <code>4</code>`) || strings.Contains(scalar, `<span>missing:</span>`) || strings.Contains(scalar, `<span>extra:</span>`) {
		t.Fatalf("scalar check did not preserve scalar-only detail fields: %s", scalar)
	}
}

func TestRenderNoConformanceIsExactNoOp(t *testing.T) {
	claim := claimFor(model.LayoutCard)
	cat, err := catalog.Build([]model.Claim{claim}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before, err := Render(cat, nil)
	if err != nil {
		t.Fatal(err)
	}
	cat.SetConformance(nil)
	after, err := Render(cat, nil)
	if err != nil {
		t.Fatal(err)
	}
	normalizeTime := func(value string) string {
		value = regexp.MustCompile(`at \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`).ReplaceAllString(value, "at <time>")
		return regexp.MustCompile(`Generated \d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC`).ReplaceAllString(value, "Generated <time>")
	}
	if normalizeTime(before) != normalizeTime(after) || strings.Contains(after, "claim-conformance") || strings.Contains(after, "dossierx-conformance-status-freshness") {
		t.Fatal("an omitted conformance projection changed viewer bytes")
	}
}

func TestRenderConformanceAllLayoutsAndStates(t *testing.T) {
	layouts := []model.Layout{model.LayoutCard, model.LayoutTable, model.LayoutList, model.LayoutSteps, model.LayoutTree, model.LayoutBanner, model.LayoutMockup}
	states := []conformance.State{conformance.StateMatched, conformance.StateOwed, conformance.StateMismatch, conformance.StateUncheckable}
	claims := make([]model.Claim, 0, len(layouts))
	results := make([]conformance.Result, 0, len(layouts))
	for i, layout := range layouts {
		claim := claimFor(layout)
		claims = append(claims, claim)
		state := states[i%len(states)]
		result := conformance.Result{ClaimID: claim.ID, Mode: model.EmbodimentModeCompare, ImplementationReady: state == conformance.StateMatched, Checks: []conformance.CheckResult{{
			ID: "current-value", Shape: model.ExpectationShapeScalar, State: state, ImplementationReady: state == conformance.StateMatched, Expected: "ready",
		}}}
		if i == len(layouts)-1 {
			result = conformance.Result{ClaimID: claim.ID, Mode: model.EmbodimentModeNone, ImplementationReady: true, Reason: "no software embodiment"}
		}
		results = append(results, result)
	}
	cat, err := catalog.Build(claims, nil)
	if err != nil {
		t.Fatal(err)
	}
	cat.SetConformance(&conformance.Report{Results: results})
	out, err := Render(cat, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, layout := range layouts {
		if !strings.Contains(out, `class="claim claim-`+string(layout)) {
			t.Fatalf("layout %s did not render", layout)
		}
	}
	for _, state := range []string{"matched", "owed", "mismatch", "uncheckable", "declared_none"} {
		if !strings.Contains(out, `data-conformance-state="`+state+`"`) {
			t.Fatalf("state %s did not render", state)
		}
	}
	if got, want := strings.Count(out, `class="claim-conformance"`), len(layouts); got != want {
		t.Fatalf("claim conformance panels = %d, want %d", got, want)
	}
	if got, want := strings.Count(out, `class="claim-conformance-check"`), len(layouts)-1; got != want {
		t.Fatalf("check panels = %d, want %d", got, want)
	}
}

func TestRenderConformanceSurvivesComponentOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card.html"), []byte(`<article class="project-card-override">{{.ID}}</article>`), 0o644); err != nil {
		t.Fatal(err)
	}
	claim := claimFor(model.LayoutCard)
	cat, err := catalog.Build([]model.Claim{claim}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cat.SetConformance(&conformance.Report{Results: []conformance.Result{{
		ClaimID: claim.ID, Mode: model.EmbodimentModeCompare, ImplementationReady: true,
		Checks: []conformance.CheckResult{{ID: "current-value", Shape: model.ExpectationShapeScalar, State: conformance.StateMatched, ImplementationReady: true, Expected: "ready", Observed: "ready"}},
	}}})
	out, err := Render(cat, &config.Config{Viewer: config.Viewer{TemplateOverrides: dir}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `project-card-override`) || !strings.Contains(out, `data-check-id="current-value"`) || !strings.Contains(out, `data-conformance-state="matched"`) || !strings.Contains(out, `data-implementation-ready="true"`) {
		t.Fatalf("component override hid engine conformance projection: %s", out)
	}
}
