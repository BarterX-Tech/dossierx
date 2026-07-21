package components

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/implink"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestLoad_Defaults(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, layout := range []model.Layout{
		model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutSteps, model.LayoutTree, model.LayoutBanner,
		model.LayoutMockup,
	} {
		if _, ok := partials[layout]; !ok {
			t.Errorf("Load(\"\") missing partial for layout %q", layout)
		}
	}
}

func TestLoad_OverrideDirMissingEntirely(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := Load(missing)
	if err == nil {
		t.Fatalf("Load: expected error for missing override directory, got nil")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("Load error should name the missing override path %q, got: %v", missing, err)
	}
}

func TestLoad_OverrideDirNotADirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Fatalf("Load: expected error when override path is a file, not a directory")
	}
}

func TestLoad_OverridePartialFallsBackWhenMissing(t *testing.T) {
	dir := t.TempDir()
	// Only override table.html.
	if err := os.WriteFile(filepath.Join(dir, "table.html"), []byte(`<section class="override-table">{{.ID}}</section>`), 0o644); err != nil {
		t.Fatal(err)
	}

	partials, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var buf bytes.Buffer
	claim := model.Claim{ID: "widget.contract.x", Status: model.StatusDraft}
	if err := partials[model.LayoutTable].Execute(&buf, claim); err != nil {
		t.Fatalf("execute table partial: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "override-table") {
		t.Errorf("table partial should be the override, got: %s", got)
	}

	buf.Reset()
	if err := partials[model.LayoutCard].Execute(&buf, claim); err != nil {
		t.Fatalf("execute card partial: %v", err)
	}
	if got := buf.String(); strings.Contains(got, "override-table") {
		t.Errorf("card partial should be the embedded default, got: %s", got)
	}
}

// TestTreePartial_PillClassMatchesSharedHelper renders tree.html's default
// partial for every Status/ReviewPending combination the pill can take and
// asserts the class it emits matches pillClass's output for the same pair.
// tree.html computes its pill class with its own inline template
// conditional (see the comment atop tree.html) rather than calling the
// shared pillClass helper, so nothing else guarantees the two stay in sync;
// this locks in the one case — locked+ReviewPending ("pw") — most likely to
// silently diverge if either is edited independently, alongside the other
// two reachable states for context.
func TestTreePartial_PillClassMatchesSharedHelper(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tree, ok := partials[model.LayoutTree]
	if !ok {
		t.Fatalf("Load(\"\") missing partial for layout %q", model.LayoutTree)
	}

	cases := []struct {
		name          string
		status        model.Status
		reviewPending bool
	}{
		{"draft", model.StatusDraft, false},
		{"locked", model.StatusLocked, false},
		{"locked_review_pending", model.StatusLocked, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claim := model.Claim{
				ID:            "widget.contract.x",
				Status:        tc.status,
				ReviewPending: tc.reviewPending,
			}

			var buf bytes.Buffer
			if err := tree.Execute(&buf, claim); err != nil {
				t.Fatalf("execute tree partial: %v", err)
			}

			want := pillClass(tc.status, tc.reviewPending)
			wantClass := `class="pill ` + want + `"`
			if got := buf.String(); !strings.Contains(got, wantClass) {
				t.Errorf("tree.html pill class = %q, want it to contain %q (pillClass helper says %q); rendered: %s",
					got, wantClass, want, got)
			}
		})
	}
}

func TestDefaultPartials_PillClassMatchesPillClassHelper(t *testing.T) {
	// Every layout partial's claim-head pill must be driven by the shared
	// pillClass helper rather than a reimplemented inline conditional, so
	// all layouts (including tree, which used to hand-roll this) agree on
	// the same status/review_pending -> pill-class mapping.
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := []struct {
		name          string
		status        model.Status
		reviewPending bool
		wantClass     string
	}{
		{"draft", model.StatusDraft, false, "pv"},
		{"locked", model.StatusLocked, false, "ps"},
		{"locked_review_pending", model.StatusLocked, true, "pw"},
	}

	for layout, tmpl := range partials {
		if layout == model.LayoutBanner {
			continue // banner has no claim-head pill.
		}
		for _, tc := range cases {
			t.Run(string(layout)+"/"+tc.name, func(t *testing.T) {
				claim := model.Claim{
					ID:            "widget.contract.x",
					Status:        tc.status,
					ReviewPending: tc.reviewPending,
				}
				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, claim); err != nil {
					t.Fatalf("execute %s partial: %v", layout, err)
				}
				wantPill := `class="pill ` + tc.wantClass + `"`
				if got := buf.String(); !strings.Contains(got, wantPill) {
					t.Errorf("%s partial pill class = %q, want it to contain %q", layout, got, wantPill)
				}
			})
		}
	}
}

func TestRowKeys_SortedUnion(t *testing.T) {
	rows := []model.Row{
		{"b": 1, "a": 2},
		{"c": 3, "a": 4},
	}
	got := rowKeys(rows)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("rowKeys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rowKeys = %v, want %v", got, want)
		}
	}
}

// ---------------------------------------------------------------------
// rowKeys' authored-order path (RowColumns != nil): distinct from
// TestRowKeys_SortedUnion above, which only exercises the hand-built-Row
// fallback (RowColumns == nil, alphabetical).
// ---------------------------------------------------------------------

func TestRowKeys_PrefersAuthoredOrderOverAlphabetical(t *testing.T) {
	rows := []model.Row{
		decodeRowForTest(t, "zeta: 1\nalpha: 2\n"),
		decodeRowForTest(t, "alpha: 3\nmiddle: 4\n"), // "middle" is new, appended after zeta/alpha.
	}
	got := rowKeys(rows)
	want := []string{"zeta", "alpha", "middle"}
	if len(got) != len(want) {
		t.Fatalf("rowKeys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rowKeys = %v, want %v", got, want)
		}
	}
}

func decodeRowForTest(t *testing.T, doc string) model.Row {
	t.Helper()
	var r model.Row
	if err := yaml.Unmarshal([]byte(doc), &r); err != nil {
		t.Fatalf("decode row: %v", err)
	}
	return r
}

// ---------------------------------------------------------------------
// inc, colClass
// ---------------------------------------------------------------------

func TestInc(t *testing.T) {
	if got := inc(0); got != 1 {
		t.Errorf("inc(0) = %d, want 1", got)
	}
	if got := inc(41); got != 42 {
		t.Errorf("inc(41) = %d, want 42", got)
	}
}

func TestColClass_KnownAndUnknownAndCaseInsensitive(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"key", "key"},
		{"Field", "key"},
		{"TYPE", "ty"},
		{"enum", "en"},
		{"Example", "ex"},
		{"examples", "ex"},
		{"cloud_default", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := colClass(tc.key); got != tc.want {
			t.Errorf("colClass(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------
// edgesHTML
// ---------------------------------------------------------------------

func TestEdgesHTML_MinimalClaimOmitsFacetModuleAndEmptyFields(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	got := string(edgesHTML(c))
	if !strings.Contains(got, `<ul class="claim-edges">`) {
		t.Fatalf("edgesHTML missing wrapper ul, got: %s", got)
	}
	// facet/module are deliberately never rendered here — the surrounding
	// page (tab + nav) already conveys them — so a bare Facet-only claim
	// with nothing else set should render an effectively empty edges div.
	for _, absent := range []string{"claim-facet", "facet:", "claim-module", "module:", "claim-governed", "claim-mirrors", "claim-rests-on", "claim-migrated", "claim-review-pending"} {
		if strings.Contains(got, absent) {
			t.Errorf("edgesHTML for a minimal claim should omit %q, got: %s", absent, got)
		}
	}
}

func TestEdgesHTML_GovernedNoneWithReason(t *testing.T) {
	c := model.Claim{
		Facet:    "contract",
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: "fixture <claim>"},
	}
	got := string(edgesHTML(c))
	if !strings.Contains(got, `governed-none`) {
		t.Fatalf("expected the governed-none class, got: %s", got)
	}
	if !strings.Contains(got, `governed_by: none`) || !strings.Contains(got, `fixture &lt;claim&gt;`) {
		t.Fatalf("expected governed_by: none plus an HTML-escaped reason, got: %s", got)
	}
}

func TestEdgesHTML_GovernedByDoctrineClaim(t *testing.T) {
	c := model.Claim{
		Facet:    "contract",
		Governed: model.Governed{Type: "widget.doctrine.hub"},
	}
	got := string(edgesHTML(c))
	if strings.Contains(got, "governed-none") {
		t.Fatalf("a doctrine-governed claim must not carry the governed-none class, got: %s", got)
	}
	if !strings.Contains(got, `governed_by: <a href="#widget.doctrine.hub">widget.doctrine.hub</a>`) {
		t.Fatalf("expected governed_by to link to the doctrine claim by hash, got: %s", got)
	}
}

func TestEdgesHTML_FullClaimAllFields(t *testing.T) {
	c := model.Claim{
		Facet:         "contract",
		Module:        "widget",
		Governed:      model.Governed{Type: string(model.GovernedNone), Reason: "fixture"},
		Mirrors:       []string{"widget.contract.a", "widget.contract.b"},
		RestsOn:       []string{"widget.contract.c"},
		MigratedFrom:  "docs/tabs/widget.html",
		Status:        model.StatusLocked,
		ReviewPending: true,
	}
	got := string(edgesHTML(c))
	for _, absent := range []string{"claim-facet", "facet:", "claim-module", "module:"} {
		if strings.Contains(got, absent) {
			t.Errorf("edgesHTML should never render facet/module, got %q in: %s", absent, got)
		}
	}
	for _, want := range []string{
		`claim-mirrors`,
		`href="#widget.contract.a"`,
		`href="#widget.contract.b"`,
		`claim-rests-on`,
		`href="#widget.contract.c"`,
		`migrated_from: docs/tabs/widget.html`,
		`claim-review-pending`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("edgesHTML missing %q, got: %s", want, got)
		}
	}
	// Two mirror ids each get their own bulleted <li>.
	if !strings.Contains(got, `<li><a href="#widget.contract.a">widget.contract.a</a></li><li><a href="#widget.contract.b">widget.contract.b</a></li>`) {
		t.Fatalf("expected multiple ids in an edge list rendered as separate <li> bullets, got: %s", got)
	}
}

func TestEdgesHTML_ReviewPendingOnlyShownWhenLocked(t *testing.T) {
	// ReviewPending is only ever meaningful on a locked claim (see
	// model.Claim.ReviewPending's doc comment); edgesHTML's guard reflects
	// that even if a draft claim somehow carried ReviewPending=true.
	c := model.Claim{Facet: "contract", Status: model.StatusDraft, ReviewPending: true}
	got := string(edgesHTML(c))
	if strings.Contains(got, "claim-review-pending") {
		t.Fatalf("edgesHTML should not show review_pending for a draft claim, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// EdgesHTMLWithLinks
// ---------------------------------------------------------------------

func TestEdgesHTMLWithLinks_NilFiles_MatchesPlainEdgesHTML(t *testing.T) {
	c := model.Claim{Facet: "contract", Status: model.StatusLocked}
	if got, want := string(EdgesHTMLWithLinks(c, nil, nil)), string(edgesHTML(c)); got != want {
		t.Fatalf("EdgesHTMLWithLinks(c, nil, nil) = %q, want it to match edgesHTML(c) = %q", got, want)
	}
}

func TestEdgesHTMLWithLinks_RendersFileAndSymbol(t *testing.T) {
	c := model.Claim{Facet: "contract", Status: model.StatusLocked}
	files := []implink.ViewFile{{File: "internal/widget/run.go", Symbol: "Run"}}
	got := string(EdgesHTMLWithLinks(c, files, nil))
	if !strings.Contains(got, "implemented in") {
		t.Fatalf("expected an 'implemented in' line, got: %s", got)
	}
	if !strings.Contains(got, "<code>internal/widget/run.go#Run</code>") {
		t.Fatalf("expected file#symbol rendered together in a <code> span, got: %s", got)
	}
	if strings.Contains(got, "drifted") {
		t.Fatalf("expected no drifted pill for a non-drifted file, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_NoSymbol_OmitsHash(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	files := []implink.ViewFile{{File: "internal/widget/run.go"}}
	got := string(EdgesHTMLWithLinks(c, files, nil))
	if !strings.Contains(got, "<code>internal/widget/run.go</code>") {
		t.Fatalf("expected a bare file path with no trailing '#' when Symbol is empty, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_DriftedFile_GetsWarnPill(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	files := []implink.ViewFile{{File: "internal/widget/run.go", Drifted: true}}
	got := string(EdgesHTMLWithLinks(c, files, nil))
	if !strings.Contains(got, `<span class="pill pw">drifted</span>`) {
		t.Fatalf("expected the shared warn pill (.pill.pw) on a drifted file, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_MultipleFiles_OneLinePerFile(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	files := []implink.ViewFile{
		{File: "a.go", Symbol: "A"},
		{File: "b.go", Symbol: "B"},
	}
	got := string(EdgesHTMLWithLinks(c, files, nil))
	if strings.Count(got, "claim-implemented-in") != 2 {
		t.Fatalf("expected one implemented-in line per linked file, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_DependedBy_RendersLinkedList(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	got := string(EdgesHTMLWithLinks(c, nil, []string{"widget.internals.a", "widget.internals.b"}))
	if !strings.Contains(got, "depended on by") {
		t.Fatalf("expected a 'depended on by' line, got: %s", got)
	}
	if !strings.Contains(got, `<li><a href="#widget.internals.a">widget.internals.a</a></li>`) {
		t.Fatalf("expected each depended-by id rendered as its own <li>, got: %s", got)
	}
	if strings.Count(got, "<li>") != 2 {
		t.Fatalf("expected one <li> per depended-by id, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_NilDependedBy_OmitsLine(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	got := string(EdgesHTMLWithLinks(c, nil, nil))
	if strings.Contains(got, "depended on by") {
		t.Fatalf("expected no 'depended on by' line when dependedBy is empty, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// loadOne / OverrideFile: the override-present-but-invalid and
// permission-error branches TestLoad_* above doesn't reach.
// ---------------------------------------------------------------------

func TestLoad_OverridePartialWithBadTemplateSyntaxFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "card.html"), []byte(`{{.Unclosed`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatalf("Load: expected an error for an override partial with invalid template syntax")
	}
	if !strings.Contains(err.Error(), "card.html") {
		t.Errorf("expected the error to name the offending override file, got: %v", err)
	}
}

func TestOverrideFile_EmptyDirReturnsNotFoundNoError(t *testing.T) {
	data, found, err := OverrideFile("", "card.html")
	if err != nil || found || data != nil {
		t.Fatalf("OverrideFile(\"\", ...) = (%v, %v, %v), want (nil, false, nil)", data, found, err)
	}
}

func TestOverrideFile_UnreadableFileIsHardError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "card.html")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) }) // let t.TempDir() clean up.

	if os.Getuid() == 0 {
		t.Skip("running as root: file permissions do not block reads")
	}

	_, _, err := OverrideFile(dir, "card.html")
	if err == nil {
		t.Fatalf("OverrideFile: expected a hard error for an unreadable override file")
	}
	if !strings.Contains(err.Error(), "card.html") {
		t.Errorf("expected the error to name the file, got: %v", err)
	}
}
