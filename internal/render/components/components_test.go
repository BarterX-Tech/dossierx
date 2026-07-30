package components

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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

// TestEdgesHTML_GovernedNoneReasonRoutesThroughInlineMarkdown covers the
// change routing Governed.Reason through markdown.RenderInline instead of a
// bare html.EscapeString: Reason is hand-written prose that routinely names
// a claim id or a path, so it should be able to carry a code span or a
// link (the INLINE ceiling — no block constructs), the same subset every
// other prose field already gets via the "markdown"/"cell" funcs.
func TestEdgesHTML_GovernedNoneReasonRoutesThroughInlineMarkdown(t *testing.T) {
	c := model.Claim{
		Facet: "contract",
		Governed: model.Governed{
			Type:   string(model.GovernedNone),
			Reason: "see `widget.contract.retry-policy` for the real gate",
		},
	}
	got := string(edgesHTML(c))
	if !strings.Contains(got, "<code>widget.contract.retry-policy</code>") {
		t.Fatalf("expected the code span in Reason to render as <code>, got: %s", got)
	}
	if strings.Contains(got, "`widget.contract.retry-policy`") {
		t.Fatalf("Reason's backticks should not survive as literal text, got: %s", got)
	}
}

// TestEdgesHTML_GovernedNoneReasonHostileHTMLStillEscaped guards the
// INLINE ceiling: RenderInline still HTML-escapes anything that isn't one
// of its recognized inline constructs, so a Reason is never a vector for
// raw markup even after the switch away from a bare html.EscapeString.
func TestEdgesHTML_GovernedNoneReasonHostileHTMLStillEscaped(t *testing.T) {
	c := model.Claim{
		Facet:    "contract",
		Governed: model.Governed{Type: string(model.GovernedNone), Reason: `<script>alert(1)</script>`},
	}
	got := string(edgesHTML(c))
	if strings.Contains(got, "<script>") {
		t.Fatalf("hostile HTML in Reason leaked unescaped: %s", got)
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
	// governed_by now routes through the shared writeClaimRef like every other
	// claim-to-claim edge: the hash link is unchanged, the visible text is the
	// derived label, and the machine id rides along in data-claim-id + title.
	// The rendering claim here has no Module and a different Facet, so this is
	// the widest ("Module · Facet › Label") elision tier.
	if !strings.Contains(got, `governed_by: <a class="claim-ref" href="#widget.doctrine.hub" data-claim-id="widget.doctrine.hub" title="widget.doctrine.hub"><span class="claim-ref-prefix">Widget · Doctrine › </span><span class="claim-ref-label">Hub</span></a>`) {
		t.Fatalf("expected governed_by to link to the doctrine claim by hash, labeled and id-bearing, got: %s", got)
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
	// Two mirror ids each get their own bulleted <li>. Both targets share this
	// claim's module AND facet, so they are the bare-label tier: no prefix at
	// all, since "widget.contract." is the tab the reader is already on.
	if !strings.Contains(got, `<li><a class="claim-ref" href="#widget.contract.a" data-claim-id="widget.contract.a" title="widget.contract.a"><span class="claim-ref-label">A</span></a></li><li><a class="claim-ref" href="#widget.contract.b" data-claim-id="widget.contract.b" title="widget.contract.b"><span class="claim-ref-label">B</span></a></li>`) {
		t.Fatalf("expected multiple ids in an edge list rendered as separate <li> bullets, got: %s", got)
	}
	if strings.Contains(got, "claim-ref-prefix") {
		t.Errorf("a same-module, same-facet target must carry no prefix at all, got: %s", got)
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
	if got, want := string(EdgesHTMLWithLinks(c, nil, nil, nil)), string(edgesHTML(c)); got != want {
		t.Fatalf("EdgesHTMLWithLinks(c, nil, nil, nil) = %q, want it to match edgesHTML(c) = %q", got, want)
	}
}

func TestEdgesHTMLWithLinks_RendersFileAndSymbol(t *testing.T) {
	c := model.Claim{Facet: "contract", Status: model.StatusLocked}
	files := []implink.ViewFile{{File: "internal/widget/run.go", Symbol: "Run"}}
	got := string(EdgesHTMLWithLinks(c, files, nil, nil))
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
	got := string(EdgesHTMLWithLinks(c, files, nil, nil))
	if !strings.Contains(got, "<code>internal/widget/run.go</code>") {
		t.Fatalf("expected a bare file path with no trailing '#' when Symbol is empty, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_DriftedFile_GetsWarnPill(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	files := []implink.ViewFile{{File: "internal/widget/run.go", Drifted: true}}
	got := string(EdgesHTMLWithLinks(c, files, nil, nil))
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
	got := string(EdgesHTMLWithLinks(c, files, nil, nil))
	if strings.Count(got, "claim-implemented-in") != 2 {
		t.Fatalf("expected one implemented-in line per linked file, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_DependedBy_RendersLinkedList(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	got := string(EdgesHTMLWithLinks(c, nil, []string{"widget.internals.a", "widget.internals.b"}, nil))
	if !strings.Contains(got, "depended on by") {
		t.Fatalf("expected a 'depended on by' line, got: %s", got)
	}
	if !strings.Contains(got, `<li><a class="claim-ref" href="#widget.internals.a" data-claim-id="widget.internals.a" title="widget.internals.a">`) {
		t.Fatalf("expected each depended-by id rendered as its own <li>, got: %s", got)
	}
	if strings.Count(got, "<li>") != 2 {
		t.Fatalf("expected one <li> per depended-by id, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_NilDependedBy_OmitsLine(t *testing.T) {
	c := model.Claim{Facet: "contract"}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if strings.Contains(got, "depended on by") {
		t.Fatalf("expected no 'depended on by' line when dependedBy is empty, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// C6 target status pills (issue #11's last unshipped piece): a claim-edge
// target gets a small pill after its label ONLY when it is actionable —
// draft, or locked with review_pending — never for a healthy locked target.
// This rides the targetStatuses param EdgesHTMLWithLinks/writeClaimRef added
// and, in production, only ever arrives non-nil through internal/render's
// attachEdgesOverride; the default parse-time funcMap binding always passes
// nil and gets no pill at all, on any target (covered separately below).
// ---------------------------------------------------------------------

func TestEdgesHTMLWithLinks_TargetPill_DraftTarget(t *testing.T) {
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: []string{"widget.contract.a"}}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusDraft},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if !strings.Contains(got, `<span class="pill pv">draft</span>`) {
		t.Fatalf("expected a draft pill (.pill.pv) on a draft target, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_LockedReviewPendingTarget(t *testing.T) {
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: []string{"widget.contract.a"}}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusLocked, ReviewPending: true},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if !strings.Contains(got, `<span class="pill pw">review_pending</span>`) {
		t.Fatalf("expected a warn pill (.pill.pw) reading review_pending on a locked+review_pending target, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_HealthyLockedTargetGetsNoPill(t *testing.T) {
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: []string{"widget.contract.a"}}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusLocked},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if strings.Contains(got, `class="pill`) {
		t.Fatalf("a healthy locked target must get no pill at all, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_UnknownTargetGetsNoPill(t *testing.T) {
	// A target id not present in the lookup at all (e.g. an unlinted or
	// otherwise unresolvable id) must not panic on the nil-map read and
	// must render no pill, same as an empty/nil statuses map.
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: []string{"widget.contract.a"}}
	statuses := map[string]TargetStatus{"widget.contract.b": {Status: model.StatusDraft}}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if strings.Contains(got, `class="pill`) {
		t.Fatalf("a target absent from the lookup must get no pill, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_NilStatuses_DegradesToNoPill(t *testing.T) {
	// The default funcMap binding (edgesHTML) always calls
	// EdgesHTMLWithLinks with a nil targetStatuses map — this is the
	// degrade-under-the-default-binding contract the task requires.
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: []string{"widget.contract.a"}}
	got := string(edgesHTML(c))
	if strings.Contains(got, `class="pill`) {
		t.Fatalf("edgesHTML (the default binding) must never render a target pill, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_OnGovernedByEdge(t *testing.T) {
	// governed_by is a claim-edge target too (it renders through the same
	// writeClaimRef), so an actionable governing claim should also get the
	// pill.
	c := model.Claim{
		Facet:    "contract",
		Module:   "widget",
		Governed: model.Governed{Type: "widget.doctrine.hub"},
	}
	statuses := map[string]TargetStatus{
		"widget.doctrine.hub": {Status: model.StatusDraft},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if !strings.Contains(got, `<span class="pill pv">draft</span>`) {
		t.Fatalf("expected a draft pill on an actionable governed_by target, got: %s", got)
	}
}

// ---------------------------------------------------------------------
// table.html cell rendering (DX-AUD-02): each <td> routes its value through
// the shared inline markdown renderer via the cell helper, so code spans and
// links render (not literal), and a non-string cell value doesn't break
// template execution.
// ---------------------------------------------------------------------

func TestTablePartial_CellRendersInlineMarkdown(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tbl := partials[model.LayoutTable]

	// A single cell carrying a backtick code span, a link, and a literal
	// pipe char — the exact shape BUG-02 rendered literally in every Chitta
	// table claim.
	row := decodeRowForTest(t, "name: id\nnotes: \"use `get()` see [d](http://x/a?b=1&c=2) | end\"\n")
	claim := model.Claim{
		ID:     "widget.contract.t",
		Status: model.StatusLocked,
		Rows:   []model.Row{row},
	}

	var buf bytes.Buffer
	if err := tbl.Execute(&buf, claim); err != nil {
		t.Fatalf("execute table partial: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<code>get()</code>") {
		t.Errorf("expected a rendered <code> span in a table cell, got:\n%s", out)
	}
	if !strings.Contains(out, `<a href="http://x/a?b=1&amp;c=2">d</a>`) {
		t.Errorf("expected a rendered anchor in a table cell, got:\n%s", out)
	}
	if strings.Contains(out, "`") {
		t.Errorf("literal backtick token leaked into table output (cell not routed through the inline renderer):\n%s", out)
	}
	if !strings.Contains(out, "| end") {
		t.Errorf("expected the literal pipe char preserved in the cell, got:\n%s", out)
	}
}

func TestTablePartial_RejectedSchemeCellRendersInert(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tbl := partials[model.LayoutTable]

	row := decodeRowForTest(t, "name: id\nnotes: \"[click](javascript:alert(1))\"\n")
	claim := model.Claim{
		ID:     "widget.contract.j",
		Status: model.StatusLocked,
		Rows:   []model.Row{row},
	}

	var buf bytes.Buffer
	if err := tbl.Execute(&buf, claim); err != nil {
		t.Fatalf("execute table partial: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "<a ") {
		t.Errorf("a javascript: link in a table cell must not become an anchor, got:\n%s", out)
	}
	if !strings.Contains(out, "[click](javascript:alert(1))") {
		t.Errorf("expected the rejected link rendered as inert literal text, got:\n%s", out)
	}
}

func TestTablePartial_NonStringCellDoesNotCrash(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	tbl := partials[model.LayoutTable]

	// int/bool/nil cell values: fmt.Sprint inside the cell helper defuses
	// what would otherwise be a template-execution failure when a
	// string-typed inline renderer receives a non-string cell.
	row := model.Row{"count": 42, "ok": true, "missing": nil}
	claim := model.Claim{
		ID:     "widget.contract.n",
		Status: model.StatusLocked,
		Rows:   []model.Row{row},
	}

	var buf bytes.Buffer
	if err := tbl.Execute(&buf, claim); err != nil {
		t.Fatalf("execute table partial with non-string cells: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "42") {
		t.Errorf("expected int cell rendered as 42, got:\n%s", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected bool cell rendered as true, got:\n%s", out)
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
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits (0o000) don't block reads on Windows's ACL-based permission model")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "card.html")
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Logf("restore permissions: %v", err)
		}
	}) // let t.TempDir() clean up.

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

// ---------------------------------------------------------------------
// Claim-edge labels (issue #11): ClaimLabel / writeClaimRef / the three
// elision tiers / the not-three-segments fallback / escaping.
// ---------------------------------------------------------------------

// TestClaimLabel_DerivesFromSlugOrFallsBackToRawID is the fallback contract in
// full. render never runs the lint suite, so id-shape's module.facet.slug
// guarantee does not hold here: every malformed shape must come back as the
// raw id, byte for byte, with no partial label and no panic.
func TestClaimLabel_DerivesFromSlugOrFallsBackToRawID(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		// Well-shaped: only the slug becomes the label, DisplayCase'd.
		{"widget.contract.retry-policy", "Retry Policy"},
		{"widget.contract.overview", "Overview"},
		{"token-ledger.contract.spend_cap", "Spend Cap"},
		{"widget.contract.a", "A"},
		// Not three segments -> raw id, verbatim.
		{"widget", "widget"},
		{"widget.contract", "widget.contract"},
		{"widget.contract.retry.policy", "widget.contract.retry.policy"},
		{"", ""},
		// Three segments but one is empty -> still raw, since an empty
		// module/facet can't be compared against anything and an empty slug
		// would label the claim with nothing at all.
		{".contract.slug", ".contract.slug"},
		{"widget..slug", "widget..slug"},
		{"widget.contract.", "widget.contract."},
		{"..", ".."},
		// Nothing dot-shaped at all: a draft's placeholder, a path, a sentence.
		{"TODO pick an id", "TODO pick an id"},
	}
	for _, tc := range cases {
		if got := ClaimLabel(tc.id); got != tc.want {
			t.Errorf("ClaimLabel(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

// TestEdgesHTML_ElisionTiers walks one rendering claim's three kinds of
// outgoing edge and pins which prefix each target keeps. This is the heart of
// issue #11: the prefix survives exactly where it distinguishes something the
// reader can't already see from the surrounding tab and nav entry.
func TestEdgesHTML_ElisionTiers(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.self",
		Module: "widget",
		Facet:  "contract",
		RestsOn: []string{
			"widget.contract.retry-policy",  // same module + facet -> bare
			"widget.internals.retry-buffer", // same module, other facet
			"ledger.contract.spend-cap",     // other module entirely
		},
	}
	got := string(edgesHTML(c))

	for _, want := range []string{
		// Tier 1: bare label, no prefix span.
		`title="widget.contract.retry-policy"><span class="claim-ref-label">Retry Policy</span>`,
		// Tier 2: facet only — the module is the nav entry the reader is under.
		`title="widget.internals.retry-buffer"><span class="claim-ref-prefix">Internals › </span><span class="claim-ref-label">Retry Buffer</span>`,
		// Tier 3: module and facet both, joined by the module separator.
		`title="ledger.contract.spend-cap"><span class="claim-ref-prefix">Ledger · Contract › </span><span class="claim-ref-label">Spend Cap</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the edges footer, got: %s", want, got)
		}
	}

	// Exactly two of the three targets are cross-boundary, so exactly two
	// prefixes are rendered — the same-facet one is elided, not merely dimmed.
	if n := strings.Count(got, "claim-ref-prefix"); n != 2 {
		t.Errorf("expected 2 prefix spans (the two cross-boundary targets), got %d in: %s", n, got)
	}
}

// TestEdgesHTML_UnshapedTargetIDRendersRawVerbatim pins the render-side half of
// ClaimLabel's fallback: an edge pointing at an id render can't parse still
// links, still carries the machine id, and shows that id exactly as authored —
// marked claim-ref-raw so style.css can render it as the machine string it is.
func TestEdgesHTML_UnshapedTargetIDRendersRawVerbatim(t *testing.T) {
	c := model.Claim{
		ID:      "widget.contract.self",
		Module:  "widget",
		Facet:   "contract",
		RestsOn: []string{"widget.contract.four.segments", "loose-id"},
	}
	got := string(edgesHTML(c))
	for _, want := range []string{
		`<a class="claim-ref" href="#widget.contract.four.segments" data-claim-id="widget.contract.four.segments" title="widget.contract.four.segments"><span class="claim-ref-label claim-ref-raw">widget.contract.four.segments</span></a>`,
		`<a class="claim-ref" href="#loose-id" data-claim-id="loose-id" title="loose-id"><span class="claim-ref-label claim-ref-raw">loose-id</span></a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected an unshaped id rendered raw and verbatim: %q, got: %s", want, got)
		}
	}
	// A raw id is never given a prefix — there is no trustworthy module/facet
	// to build one from, and a guessed prefix would be a lie about the graph.
	if strings.Contains(got, "claim-ref-prefix") {
		t.Errorf("an unshaped id must carry no prefix, got: %s", got)
	}
}

// TestEdgesHTML_HostileIDIsEscapedInEveryContext is C-L3. EdgesHTMLWithLinks
// hand-escapes because a FuncMap-returned template.HTML bypasses
// html/template's auto-escaping, and the label work added interpolation points
// in three contexts at once: the href, two attribute values (data-claim-id and
// the title tooltip), and element text. An id that fails the shape check flows
// through VERBATIM FROM YAML, so it is the likeliest carrier of a quote or an
// angle bracket — and a shaped-but-hostile id reaches the prefix and label
// spans too, via DisplayCase.
func TestEdgesHTML_HostileIDIsEscapedInEveryContext(t *testing.T) {
	// Unshaped (five segments) hostile id: breaks out of the title attribute
	// with a double quote, then opens a tag.
	unshaped := `a"><script>alert(1)</script>.b.c.d.e`
	// Shaped hostile id: exactly three non-empty segments, so it reaches
	// DisplayCase and both the prefix and the label span.
	shaped := `<img src=x onerror="alert(1)">.'facet.slug&x`

	c := model.Claim{
		ID:      "widget.contract.self",
		Module:  "widget",
		Facet:   "contract",
		RestsOn: []string{unshaped, shaped},
	}
	got := string(edgesHTML(c))

	// Nothing that could open a tag or close an attribute survives anywhere.
	for _, forbidden := range []string{
		`<script>`, `</script>`, `<img `, `onerror="alert`,
		`"><script`, `">alert`,
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("hostile id leaked %q unescaped into the footer: %s", forbidden, got)
		}
	}
	// html.EscapeString covers < > & ' " — the full set, in every context.
	for _, want := range []string{
		// Unshaped: escaped identically in href, data-claim-id, title and text.
		`href="#a&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;.b.c.d.e"`,
		`data-claim-id="a&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;.b.c.d.e"`,
		`title="a&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;.b.c.d.e"`,
		`claim-ref-raw">a&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;.b.c.d.e</span>`,
		// Shaped: the prefix span (module · facet) and the label span both get
		// their DisplayCase'd segments escaped.
		`<span class="claim-ref-prefix">&lt;img Src=x Onerror=&#34;alert(1)&#34;&gt; · &#39;facet › </span>`,
		`<span class="claim-ref-label">Slug&amp;x</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected escaped %q in the footer, got: %s", want, got)
		}
	}
}

// TestClaimEdgeListHTML_MatchesTheSharedFooterMarkup is C5: build_order.html's
// once-independent rests_on rendering and the shared edges footer must now emit
// the same bytes for the same edge, so the two can no longer drift.
func TestClaimEdgeListHTML_MatchesTheSharedFooterMarkup(t *testing.T) {
	ids := []string{"widget.contract.a", "widget.internals.b", "ledger.contract.c"}

	// The footer's version, rendered from a real claim.
	footer := string(edgesHTML(model.Claim{
		ID: "widget.contract.self", Module: "widget", Facet: "contract", RestsOn: ids,
	}))
	// build_order.html's version, which knows only the rendering claim's id.
	list := string(ClaimEdgeListHTML("widget.contract.self", ids))

	if !strings.Contains(footer, list) {
		t.Fatalf("ClaimEdgeListHTML must emit the same markup the shared footer does\n list:   %s\n footer: %s", list, footer)
	}
	if !strings.HasPrefix(list, `<ul class="claim-edge-id-list">`) {
		t.Errorf("expected the shared bulleted list container, got: %s", list)
	}
}

// An unshaped fromID leaves ClaimEdgeListHTML with no module/facet context to
// elide against. Every target then keeps its full prefix: a degraded label,
// never a wrong one, and never a panic.
func TestClaimEdgeListHTML_UnshapedFromIDKeepsEveryPrefix(t *testing.T) {
	list := string(ClaimEdgeListHTML("not-an-id", []string{"widget.contract.a", "widget.contract.b"}))
	if n := strings.Count(list, "claim-ref-prefix"); n != 2 {
		t.Errorf("expected both targets to keep a full prefix when the reader's context is unknown, got %d in: %s", n, list)
	}
	if !strings.Contains(list, `<span class="claim-ref-prefix">Widget · Contract › </span>`) {
		t.Errorf("expected the widest prefix tier, got: %s", list)
	}
}

// TestPartialHeadings_LabelIDAndKeepMachineIDReachable is C4/item 5 across all
// seven layout partials at once — banner.html included, the one partial with no
// edges footer for the rest of this work to reach. The heading shows the label;
// data-claim-id and the title tooltip keep the id typeable for "dossierx claim
// lock <id>", greppable in the rendered HTML, and reachable from the viewer JS.
func TestPartialHeadings_LabelIDAndKeepMachineIDReachable(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	claim := model.Claim{
		ID:     "widget.contract.retry-policy",
		Module: "widget",
		Facet:  "contract",
		Status: model.StatusLocked,
		Body:   "prose",
		Rows:   []model.Row{{"key": "k"}},
		Steps:  []string{"one"},
	}

	for _, layout := range []model.Layout{
		model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutSteps, model.LayoutTree, model.LayoutBanner,
		model.LayoutMockup,
	} {
		t.Run(string(layout), func(t *testing.T) {
			var buf bytes.Buffer
			if err := partials[layout].Execute(&buf, claim); err != nil {
				t.Fatalf("execute %q partial: %v", layout, err)
			}
			got := buf.String()
			want := `<div class="k" data-claim-id="widget.contract.retry-policy" title="widget.contract.retry-policy">Retry Policy `
			if !strings.Contains(got, want) {
				t.Fatalf("expected the labeled, id-bearing heading %q, got: %s", want, got)
			}
			// The heading label is the BARE label — a claim's own module and
			// facet are the page the reader is standing on.
			if strings.Contains(got, `>Widget · Contract › Retry Policy`) {
				t.Errorf("a claim's own heading must not carry a prefix, got: %s", got)
			}
			// The root element's id= is untouched: it is what a #hash deep link
			// and stripOverviewIDs both key off.
			if !strings.Contains(got, ` id="widget.contract.retry-policy"`) {
				t.Errorf("the root element must keep its id attribute, got: %s", got)
			}
		})
	}
}

// A partial heading is ordinary auto-escaping template context — claimLabel
// returns a plain string, not template.HTML — so a hostile id must come back
// escaped by html/template itself, with no hand-escaping in the partial. This
// pins that no future edit "helpfully" wraps claimLabel's output in a
// template.HTML the way the edges footer legitimately has to.
func TestPartialHeadings_HostileIDIsAutoEscaped(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	claim := model.Claim{
		ID:     `x"><script>alert(1)</script>`,
		Status: model.StatusDraft,
		Body:   "prose",
	}
	var buf bytes.Buffer
	if err := partials[model.LayoutCard].Execute(&buf, claim); err != nil {
		t.Fatalf("execute card partial: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "<script>") || strings.Contains(got, "</script>") {
		t.Fatalf("hostile id leaked into a partial heading unescaped: %s", got)
	}
	// Not three segments, so the heading shows the raw id — escaped.
	if !strings.Contains(got, `alert(1)`) {
		t.Fatalf("expected the raw id shown (escaped) in the heading, got: %s", got)
	}
}

// DisplayCase moved here from internal/render (where it was the unexported
// displayCase driving module/facet nav labels) so ClaimLabel could share the
// one implementation. Its behavior must not have changed in the move: render's
// nav labels and a card's claim label both depend on it.
func TestDisplayCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"token-ledger", "Token Ledger"},
		{"token_ledger", "Token Ledger"},
		{"contract", "Contract"},
		{"retry policy", "Retry Policy"},
		{"a-b_c d", "A B C D"},
		{"", ""},
		{"-", ""},
		{"--a--", "A"},
		{"ALREADY-CAPS", "ALREADY CAPS"},
		{"9lives", "9lives"},
	}
	for _, tc := range cases {
		if got := DisplayCase(tc.in); got != tc.want {
			t.Errorf("DisplayCase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
