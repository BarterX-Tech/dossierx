package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/projectclaims"
)

// summary200 is a claim summary at the default 200-character cap.
var summary200 = strings.Repeat("s", 199) + "."

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func moduleClaim(module string, i int, summary string) model.Claim {
	return model.Claim{
		ID: fmt.Sprintf("%s.contract.c%02d", module, i), Facet: "contract", Module: module,
		Status: model.StatusDraft, Summary: summary, Body: strings.Repeat("b", 2000),
	}
}

func projectClaim(i int, summary string) model.Claim {
	return model.Claim{ID: fmt.Sprintf("project.p%03d", i), Scope: model.ScopeProject, Status: model.StatusDraft, Summary: summary}
}

func TestShowIsolationEmitsAuthoredSummariesVerbatim(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	// Past the old 80-rune clip, and a body whose first line says something
	// else: the view must carry the authored summary, never the body.
	long := "Retries are capped at three attempts with jittered backoff, and the caller owns idempotency of every retried request."
	claims := []model.Claim{{
		ID: "widget.contract.retry", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Summary: long + "\n", Body: "First body line that is not the summary.\nSecond line.",
	}}
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true})
	if err != nil {
		t.Fatal(err)
	}
	got := view.Isolation.Claims[0].Summary
	if got != long {
		t.Fatalf("summary = %q, want the authored summary %q", got, long)
	}
	blob := mustJSON(t, view.Isolation)
	if strings.Contains(string(blob), "First body line") {
		t.Fatalf("isolation view leaked a claim body: %s", blob)
	}
	if len(view.Isolation.DraftHints.SuggestedProvides) != 1 {
		t.Fatalf("draft hints: %+v", view.Isolation.DraftHints)
	}
}

func TestShowIsolationCarriesConstitutionAndProjectIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "claims")
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir, Constitution: filepath.Join(root, "constitution.yaml")}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.Constitution, []byte("status: draft\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: Every module reads this sentence.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	claims := []model.Claim{
		moduleClaim("widget", 1, "Widget summary."),
		projectClaim(2, "Second project rule."),
		projectClaim(1, "First project rule."),
	}
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true, ConstitutionState: "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view.Isolation.Shared.ConstitutionText, "Every module reads this sentence.") {
		t.Fatalf("constitution text: %q", view.Isolation.Shared.ConstitutionText)
	}
	want := []projectclaims.Entry{
		{ID: "project.p001", Summary: "First project rule.", Status: "draft"},
		{ID: "project.p002", Summary: "Second project rule.", Status: "draft"},
	}
	if fmt.Sprint(view.Isolation.Shared.ProjectClaims) != fmt.Sprint(want) {
		t.Fatalf("project claims index = %+v", view.Isolation.Shared.ProjectClaims)
	}
	if len(view.Isolation.Claims) != 1 {
		t.Fatalf("project claims are not module claims: %+v", view.Isolation.Claims)
	}
	d := view.ConstitutionDigest
	if !d.Present || d.State != "draft" || d.Invariants != 1 || d.Hash == "" {
		t.Fatalf("constitution digest: %+v", d)
	}
}

// The budget report is the view's own bytes: shared is the shared object,
// module is everything else, and together they are the whole view.
func TestIsolationBudgetMeasuresTheEmittedView(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	var claims []model.Claim
	for i := 1; i <= 10; i++ {
		claims = append(claims, moduleClaim("widget", i, summary200))
	}
	for i := 1; i <= 5; i++ {
		claims = append(claims, projectClaim(i, summary200))
	}
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true})
	if err != nil {
		t.Fatalf("ten 200-character claims must fit the module budget: %v", err)
	}
	whole := mustJSON(t, view.Isolation)
	shared := mustJSON(t, view.Isolation.Shared)
	b := view.IsolationBudget
	if b.SharedBytes != len(shared) || b.SharedBytes+b.ModuleBytes != len(whole) {
		t.Fatalf("budget %+v does not add up to the view (%d shared, %d whole)", b, len(shared), len(whole))
	}
	if b.SharedBudget+b.ModuleBudget != MaxIsolationBytes || b.SharedBudget != SharedBudgetBytes {
		t.Fatalf("budgets: %+v", b)
	}
}

// A max_claims_per_module override well above 10 is an accepted refusal: it
// names the module, the module budget, and the claim count that explains it.
func TestShowIsolationRefusesModuleOverflowNamingTheModule(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	var claims []model.Claim
	for i := 1; i <= 25; i++ {
		claims = append(claims, moduleClaim("widget", i, summary200))
	}
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true})
	if !IsIsolationOversize(err) {
		t.Fatalf("want module-budget refusal, got %v", err)
	}
	for _, want := range []string{`module "widget"`, fmt.Sprint(ModuleBudgetBytes), "25 claim summaries"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("refusal %q does not say %q", err, want)
		}
	}
	if view.Isolation != nil {
		t.Fatal("a refused view must not be emitted")
	}
	details := IsolationOversizeDetails(err)
	if details["module"] != "widget" || details["claims"] != 25 {
		t.Fatalf("details: %+v", details)
	}
}

// Shared text never refuses a module's view: check owns that budget.
func TestShowIsolationNeverRefusesForSharedText(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	claims := []model.Claim{moduleClaim("widget", 1, "Widget summary.")}
	for i := 1; i <= 60; i++ {
		claims = append(claims, projectClaim(i, summary200))
	}
	view, err := Show(claims, cfg, "widget", ShowOptions{Isolation: true})
	if err != nil {
		t.Fatalf("shared overflow must not refuse the module view: %v", err)
	}
	if view.IsolationBudget.SharedBytes <= SharedBudgetBytes {
		t.Fatalf("fixture should be over the shared budget: %+v", view.IsolationBudget)
	}
}

// CheckSharedBudget's running size is the marshalled size at every prefix,
// so the claim it blames is exactly the first one whose line crosses.
func TestCheckSharedBudgetBlamesTheCrossingClaim(t *testing.T) {
	sc := SharedContext{ConstitutionText: strings.Repeat("c", 3000), ProjectClaims: []projectclaims.Entry{}}
	var crossing string
	for i := 1; i <= 60; i++ {
		sc.ProjectClaims = append(sc.ProjectClaims, projectclaims.Entry{ID: fmt.Sprintf("project.p%03d", i), Summary: summary200, Status: "draft"})
		blob := mustJSON(t, sc)
		if crossing == "" && len(blob) > SharedBudgetBytes {
			crossing = sc.ProjectClaims[i-1].ID
		}
		over, err := CheckSharedBudget(sc)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case len(blob) <= SharedBudgetBytes && over != nil:
			t.Fatalf("%d claims, %d bytes: reported over: %+v", i, len(blob), over)
		case len(blob) > SharedBudgetBytes && (over == nil || over.ClaimID != crossing || over.Bytes != len(blob)):
			t.Fatalf("%d claims, %d bytes: got %+v, want %s", i, len(blob), over, crossing)
		}
	}
	if crossing == "" {
		t.Fatal("fixture never crossed the budget")
	}

	over, err := CheckSharedBudget(SharedContext{ConstitutionText: strings.Repeat("c", SharedBudgetBytes)})
	if err != nil || over == nil || over.ClaimID != "" {
		t.Fatalf("constitution alone over: %+v, %v", over, err)
	}
}

func TestListAndIntegration(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	if err := WriteMinimal(dir, "lock"); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "widget/manifest.yaml", "summary: widget boundary.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - lock.contract.store\n")
	write(t, dir, "lock/manifest.yaml", "summary: lock store.\nprovides:\n  - lock.contract.store\ndepends_on: []\n")
	claims := []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "bound"},
		{ID: "lock.contract.store", Facet: "contract", Module: "lock", Status: model.StatusDraft, Body: "store"},
	}
	view, err := Show(claims, cfg, "widget", ShowOptions{Integration: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Findings) != 0 {
		t.Fatalf("exported dependency must be clean: %+v", view.Findings)
	}
	if view.Integration == nil || len(view.Integration.Neighbors) != 1 || view.Integration.Neighbors[0].Module != "lock" {
		t.Fatalf("neighbors: %+v", view.Integration)
	}
	if len(view.Integration.Edges) != 1 || view.Integration.Edges[0].ToModule != "lock" {
		t.Fatalf("edges must be membership, not a graph walk: %+v", view.Integration.Edges)
	}
	cat := List(claims, cfg)
	if len(cat) != 2 {
		t.Fatalf("catalog: %+v", cat)
	}
}
