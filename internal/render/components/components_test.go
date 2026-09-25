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

func TestDefaultPartialsHaveNoStructuralTrailingWhitespace(t *testing.T) {
	partials, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, layout := range []model.Layout{
		model.LayoutCard, model.LayoutTable, model.LayoutList,
		model.LayoutSteps, model.LayoutTree, model.LayoutBanner,
		model.LayoutMockup,
	} {
		t.Run(string(layout), func(t *testing.T) {
			claim := model.Claim{ID: "widget.contract.empty", Module: "widget", Facet: "contract", Layout: layout, Status: model.StatusDraft}
			var buf bytes.Buffer
			if err := partials[layout].Execute(&buf, claim); err != nil {
				t.Fatalf("execute %s: %v", layout, err)
			}
			for lineNo, line := range strings.Split(buf.String(), "\n") {
				if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
					t.Fatalf("default %s partial has trailing whitespace at line %d", layout, lineNo+1)
				}
			}
		})
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
	// A claim with no relationships or sources still has a stable footer: its
	// zero states use plain language, not empty dropdowns or numeral counts.
	c := model.Claim{Facet: "contract"}
	got := string(edgesHTML(c))
	if !strings.Contains(got, `<div class="claim-footer">`) {
		t.Fatalf("edgesHTML for an edgeless claim must still open the footer strip, got: %s", got)
	}
	for _, want := range []string{
		`<span class="claim-footer-chip-label">No relationships</span>`,
		`<span class="claim-footer-chip-label">No sources</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q, got: %s", want, got)
		}
	}
	if strings.Contains(got, "0 sources") {
		t.Fatalf("zero sources must read \"No sources\", never the numeral form, got: %s", got)
	}

	// facet/module are deliberately never rendered — the surrounding page (tab
	// + nav) already conveys them. Proven against a claim that DOES emit a
	// footer, since the empty case above can no longer discriminate: every
	// substring is trivially absent from "".
	// 05 §4.6/R09.1: the footer is now a <div class="claim-footer"> strip
	// wrapping a <details class="claim-links"> door, whose panel carries the
	// "extra" facts (migrated_from among them) in a
	// <ul class="claim-edges claim-edges-extra">.
	withEdge := model.Claim{Facet: "contract", Module: "widget", MigratedFrom: "docs/tabs/widget.html"}
	got = string(edgesHTML(withEdge))
	if !strings.Contains(got, `<div class="claim-footer">`) || !strings.Contains(got, `class="claim-edges`) {
		t.Fatalf("a claim with one edge must still render the footer strip and its edges ul, got: %s", got)
	}
	for _, absent := range []string{"claim-facet", "facet:", "claim-module", "module:", "claim-rests-on", "claim-review-pending"} {
		if strings.Contains(got, absent) {
			t.Errorf("edgesHTML should omit %q, got: %s", absent, got)
		}
	}
}

// ---------------------------------------------------------------------
// v0.4.1: the footer is a collapsed <details class="claim-links"> whose
// <summary> digests it as "N links - N files - N drifted".
// ---------------------------------------------------------------------

// TestEdgesHTMLWithLinks_SummaryCountsAndFormat walks the frozen count table
// for the RELATIONSHIPS chip — docs/design/screens/05-claim-one-expansion-at-
// a-time.md §6's "a noun and a count, never a score" (R-F.1), now
// "N relationship"/"N relationships" rather than the pre-redesign "N links".
// links counts what the reader FINDS ON EXPANDING — one per id inside the
// three R09.4 direction blocks and the "extra" migrated_from/
// review_pending rows — never the linked files or sources, which are their
// own doors (§6: "files and drifted do not appear in the strip"; sources is
// its own door per R09.5) and no longer ride in this chip's count at all.
//
// "rests_on: none" counts ZERO. It is a stated absence, not a link.
//
// RETRY RE-PIN (fix-list item 10; 05 §8 item 5). Counting it used to put a
// "1 relationship" chip under every edgeless claim in every real project
// (rests_on is mandatory) — the comment that used to sit here also said
// it "made the no-footer case unreachable", treating that as evidence the
// count was right. It was backwards: the no-footer case was itself the bug
// this retry removes (05 §8 item 5's "an entirely edgeless, sourceless,
// checkless claim still shows four zeros", not no footer at all). Every
// wantChip below is now the real "0 relationships"/"1 relationship" text —
// there is no longer a sentinel for "no footer", because there is no longer
// a no-footer case for this chip to reach.
//
// The count is singular at exactly 1 — "1 relationship" — and plural
// everywhere else, including 0.
func TestEdgesHTMLWithLinks_SummaryCountsAndFormat(t *testing.T) {
	cases := []struct {
		name       string
		claim      model.Claim
		files      []implink.ViewFile
		dependedBy []string
		wantChip   string
	}{
		{
			name: "restson_dependedby_no_files",
			claim: model.Claim{
				Module: "widget", Facet: "contract",
				RestsOn: model.RestsOnIDs("widget.contract.b", "widget.contract.c"),
			},
			dependedBy: []string{"widget.internals.d", "widget.internals.e"},
			wantChip:   "4 relationships",
		},
		{
			// Linked files no longer add to the relationships count at all —
			// they render as rows inside the panel but are not their own
			// term in this chip (§6).
			name: "restson3_one_clean_file",
			claim: model.Claim{
				Module: "widget", Facet: "contract",
				RestsOn: model.RestsOnIDs("widget.contract.a", "widget.contract.b", "widget.contract.c"),
			},
			files:    []implink.ViewFile{{File: "a.go"}},
			wantChip: "3 relationships",
		},
		{
			name: "restson3_two_files_one_drifted",
			claim: model.Claim{
				Module: "widget", Facet: "contract",
				RestsOn: model.RestsOnIDs("widget.contract.a", "widget.contract.b", "widget.contract.c"),
			},
			files:    []implink.ViewFile{{File: "a.go", Drifted: true}, {File: "b.go"}},
			wantChip: "3 relationships",
		},
		{
			// One relationship, singular.
			name:     "only_migrated_from",
			claim:    model.Claim{Facet: "contract", MigratedFrom: "docs/tabs/widget.html"},
			wantChip: "1 relationship",
		},
		{
			// A file with no relationships still opens the footer (the
			// implemented-in row rides inside the relationships panel), but
			// the chip itself reads zero.
			name:     "only_one_clean_file",
			claim:    model.Claim{Facet: "contract"},
			files:    []implink.ViewFile{{File: "a.go"}},
			wantChip: "0 relationships",
		},
		{
			name:     "one_drifted_file",
			claim:    model.Claim{Facet: "contract"},
			files:    []implink.ViewFile{{File: "a.go", Drifted: true}},
			wantChip: "0 relationships",
		},
		{
			// rests_on: none counts ZERO — a stated absence, not a
			// relationship — so a claim carrying nothing else still renders
			// the strip (RETRY: the strip no longer suppresses at all-zero),
			// with the chip reading the numeral "0 relationships". Its reason
			// is not a relationship either.
			name:     "rests_on_none_only",
			claim:    model.Claim{Facet: "contract", RestsOn: model.RestsNone("fixture")},
			wantChip: "0 relationships",
		},
		{
			// One linked file is enough to disclose something, so the footer
			// comes back — and the rests_on: none row rides along inside
			// it while still counting zero.
			name:     "rests_on_none_plus_one_file",
			claim:    model.Claim{Facet: "contract", RestsOn: model.RestsNone("fixture")},
			files:    []implink.ViewFile{{File: "a.go"}},
			wantChip: "0 relationships",
		},
		{
			// One named rests_on target is one relationship.
			name: "one_rests_on",
			claim: model.Claim{
				Module: "widget", Facet: "contract",
				RestsOn: model.RestsOnIDs("widget.contract.a"),
			},
			wantChip: "1 relationship",
		},
		{
			name:     "review_pending_only",
			claim:    model.Claim{Facet: "contract", Status: model.StatusLocked, ReviewPending: true},
			wantChip: "1 relationship",
		},
		{
			// An entirely edgeless, sourceless claim keeps the fixed footer but
			// uses its plain, non-dropdown zero state.
			name:     "nothing_at_all",
			claim:    model.Claim{Facet: "contract"},
			wantChip: "No relationships",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(EdgesHTMLWithLinks(tc.claim, tc.files, tc.dependedBy, nil))

			want := `<span class="claim-footer-chip-label">` + tc.wantChip + `</span>`
			if !strings.Contains(got, want) {
				t.Fatalf("expected the relationships chip %q, got: %s", want, got)
			}
			// A global net: "1 relationships" is what a naive pluraliser
			// applied blindly would produce.
			if strings.Contains(got, "1 relationships") {
				t.Errorf("the relationships chip must be pluralised per count; found \"1 relationships\" in: %s", got)
			}
		})
	}
}

// TestEdgesHTMLWithLinks_DetailsWrapperSeams pins the strip's outer shape
// (05 §4.6, R09.1/R09.4): a <div class="claim-footer"> wrapping a
// <details class="claim-links" name="claim-footer-<id>"> door, whose
// RESTS ON direction block carries the rests_on row.
//
// RETRY RE-PIN (verifier fix-list item 2, THIRD retry; 05 §2 "nothing else
// on the card moves"). A door's panel used to be its <details>'s own child,
// closed together in one `</div></details>`. `display: contents` on the
// <details> — an intermediate draft's way of promoting the panel and the
// summary chip into independent flex items of .claim-footer, so the chip
// stays in the strip row while only the panel wraps onto its own row below
// — measurably breaks in the tested engine (see style.css's
// `.claim-links, .claim-sources` reset for the two probed defects). The
// panel is therefore now components.EdgesHTMLWithLinks' own SIBLING <div>
// immediately following `</details>`, never its child, which is what
// actually keeps every door's <summary> chip an ordinary flex item of the
// strip regardless of open state.
func TestEdgesHTMLWithLinks_DetailsWrapperSeams(t *testing.T) {
	c := model.Claim{
		ID: "widget.contract.self", Module: "widget", Facet: "contract",
		RestsOn: model.RestsOnIDs("doctrine.hub.retries"),
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))

	if !strings.HasPrefix(got, `<div class="claim-footer"><details class="claim-links" name="claim-footer-widget.contract.self">`) {
		t.Fatalf("expected the strip/door prologue with the per-claim accordion name, got: %s", got)
	}
	if !strings.Contains(got, `</summary></details><div class="claim-footer-panel claim-links-panel">`) {
		t.Fatalf("the relationships door must close immediately after its summary, with its panel as the very next sibling, got: %s", got)
	}
	if !strings.HasSuffix(got, `</button></span></div>`) {
		t.Fatalf("the strip must end with its footer comment slot, got: %s", got)
	}
	// The rests_on row sits inside the RELATIONSHIPS panel's direction block.
	if !strings.Contains(got, `<a class="claim-ref" href="#doctrine.hub.retries"`) {
		t.Fatalf("the rests_on row must survive the move into the strip, got: %s", got)
	}
}

// TestEdgesHTMLWithLinks_WorkedExampleExactBytes is the design's canonical
// footer, asserted as one exact string rather than a pile of Contains checks.
//
// RETRY RE-PIN (fix-list items 8, 9, 10): three shape changes since this
// string was last pinned. (1) DEPENDS ON now carries its mono direction
// count (05 §4.10; 07a §6 pins "DEPENDS ON · 1"/"DEPENDED ON BY · 1").
// (2) every
// relationship row's inline `claim-ref-prefix` is gone, replaced by an
// unconditional `claim-relationship-meta` column inside a
// `claim-relationship-line2` wrapper — 05 §4.10 counts the target's
// `module · facet` as its own column, never text folded into the title (see
// writeRelationshipMeta; contrast writeIDListItems' extras below, which
// still use the elided prefix because they have no meta column of their
// own). (3) the sources door now always renders, even at zero — 05 §8 item
// 5 / 07a §6's "No sources" — so the worked example's tail is no longer the
// relationships door's own close.
//
// RETRY RE-PIN, THIRD retry (verifier fix-list item 2; see
// TestEdgesHTMLWithLinks_DetailsWrapperSeams's doc comment for the measured
// display:contents browser bug this works around). Each door's panel is now
// written as a sibling <div> immediately after its own `</details>`, not as
// that <details>'s child, so the relationships door closes right after its
// summary and the sources door likewise closes right after ITS summary,
// each followed immediately by its own `.claim-footer-panel` sibling.
func TestEdgesHTMLWithLinks_WorkedExampleExactBytes(t *testing.T) {
	c := model.Claim{
		ID: "widget.contract.retry-policy", Module: "widget", Facet: "contract",
		Status: model.StatusLocked, ReviewPending: true,
		RestsOn: model.RestsOnIDs("widget.contract.retry-budget", "platform.http.client"),
	}
	files := []implink.ViewFile{
		{File: "internal/http/retry.go", Symbol: "Do", Drifted: true},
		{File: "internal/http/backoff.go"},
	}
	// relationships = 2 (rests_on ids) + 1 (review_pending) = 3.
	want := `<div class="claim-footer">` +
		`<details class="claim-links" name="claim-footer-widget.contract.retry-policy">` +
		`<summary class="claim-footer-chip claim-footer-chip--relationships"><span class="claim-footer-chip-label">3 relationships</span><svg class="claim-footer__chevron" aria-hidden="true" viewBox="0 0 24 24" fill="none"><path d="m6 9 6 6 6-6"/></svg></summary></details>` +
		`<div class="claim-footer-panel claim-links-panel">` +
		`<div class="claim-footer-panel-head"><span class="claim-footer-eyebrow">RELATIONSHIPS</span><span class="claim-footer-rule" aria-hidden="true"></span><span class="claim-footer-panel-count">3</span></div>` +
		`<div class="claim-relationship-direction"><div class="claim-relationship-direction-head"><span class="claim-relationship-arrow" aria-hidden="true">↓</span><span class="claim-relationship-direction-label">RESTS ON</span><span class="claim-relationship-direction-count">2</span></div><ul class="claim-edges claim-relationship-list">` +
		`<li class="claim-rests-on claim-relationship"><a class="claim-ref" href="#widget.contract.retry-budget" data-claim-id="widget.contract.retry-budget" title="widget.contract.retry-budget"><span class="claim-ref-label">Retry Budget</span></a><span class="claim-relationship-line2"><span class="claim-relationship-meta">Widget · Contract</span></span></li>` +
		`<li class="claim-rests-on claim-relationship"><a class="claim-ref" href="#platform.http.client" data-claim-id="platform.http.client" title="platform.http.client"><span class="claim-ref-label">Client</span></a><span class="claim-relationship-line2"><span class="claim-relationship-meta">Platform · Http</span></span></li>` +
		`</ul></div>` +
		`<ul class="claim-edges claim-edges-extra">` +
		`<li class="claim-review-pending claim-relationship-extra">review_pending</li>` +
		`<li class="claim-implemented-in claim-relationship-extra">implemented in: <code>internal/http/retry.go#Do</code> <span class="pill pw">drifted</span></li>` +
		`<li class="claim-implemented-in claim-relationship-extra">implemented in: <code>internal/http/backoff.go</code></li>` +
		`</ul></div>` +
		`<span class="claim-footer-chip claim-footer-chip--sources claim-footer-chip--empty"><span class="claim-footer-chip-label">No sources</span></span>` +
		`<!--dossierx-claim-footer-slot-->` + string(CommentChipHTML(c)) + `</div>`

	if got := string(EdgesHTMLWithLinks(c, files, nil, nil)); got != want {
		t.Fatalf("worked-example footer mismatch\n want: %s\n got:  %s", want, got)
	}
}

// TestEdgesHTMLWithLinks_OpenAttribute pins that there is NO server-written
// auto-open signal on the relationships door, in the exact cases that used to
// produce one. A drifted linked file and a locked+review_pending claim each
// used to open it, OR'd; Paper node Z7-0 says the relationships door "Never
// opens by default", and the reading view (board 3B-0) draws all four footer
// doors closed, so both signals were removed. Those states remain visible in
// the claim's status pill and in the readiness door.
//
// The one surviving open path is a deep link to the claim: a CSS :target rule
// in style.css, deliberately undetectable from here because a URL fragment is
// never sent to the server. That is reader-initiated, not a default.
func TestEdgesHTMLWithLinks_OpenAttribute(t *testing.T) {

	cases := []struct {
		name     string
		claim    model.Claim
		files    []implink.ViewFile
		wantOpen bool
	}{
		{
			name:     "neither_signal",
			claim:    model.Claim{Facet: "contract", Status: model.StatusLocked, MigratedFrom: "docs/x.html"},
			files:    []implink.ViewFile{{File: "a.go"}},
			wantOpen: false,
		},
		{
			name:     "drifted_file_alone",
			claim:    model.Claim{Facet: "contract", Status: model.StatusLocked},
			files:    []implink.ViewFile{{File: "a.go"}, {File: "b.go", Drifted: true}},
			wantOpen: false,
		},
		{
			name:     "locked_review_pending_alone",
			claim:    model.Claim{Facet: "contract", Status: model.StatusLocked, ReviewPending: true},
			wantOpen: false,
		},
		{
			name:     "both_signals",
			claim:    model.Claim{Facet: "contract", Status: model.StatusLocked, ReviewPending: true},
			files:    []implink.ViewFile{{File: "a.go", Drifted: true}},
			wantOpen: false,
		},
		{
			// ReviewPending is only meaningful on a locked claim; a draft
			// carrying it renders no review_pending row and must not open.
			name:     "draft_review_pending_does_not_open",
			claim:    model.Claim{Facet: "contract", Status: model.StatusDraft, ReviewPending: true, MigratedFrom: "docs/x.html"},
			wantOpen: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(EdgesHTMLWithLinks(tc.claim, tc.files, nil, nil))
			doorTag := `<details class="claim-links" name="claim-footer-` + tc.claim.ID + `"`
			wantTag := doorTag + `>`
			if tc.wantOpen {
				wantTag = doorTag + ` open>`
			}
			if !strings.Contains(got, wantTag) {
				t.Fatalf("expected the relationships door tag %q, got: %s", wantTag, got)
			}
			// The bare boolean form is the contract: never open="", never
			// open="open", and never an open="false" for the negative case —
			// a non-qualifying footer carries no open attribute at all.
			for _, forbidden := range []string{`open=""`, `open="open"`, `open="true"`, `open="false"`} {
				if strings.Contains(got, forbidden) {
					t.Errorf("the open attribute must be a bare boolean, found %q in: %s", forbidden, got)
				}
			}
		})
	}
}

// The rests_on: none ROW itself — its class, its reason, and the inline
// markdown ceiling on that reason — is asserted on a claim that also carries a
// second edge, because "none" alone does not count as a link (see
// TestEdgesHTMLWithLinks_RestsOnNoneAloneStillRendersTheStrip). MigratedFrom
// is the cheapest edge that opens the disclosure: one flat <li>, no nested id
// list, no claim-ref markup to confuse a Contains check on the reason.
func TestEdgesHTML_RestsOnNoneWithReason(t *testing.T) {
	c := model.Claim{
		Facet:        "contract",
		MigratedFrom: "docs/tabs/widget.html",
		RestsOn:      model.RestsNone("fixture <claim>"),
	}
	got := string(edgesHTML(c))
	if !strings.Contains(got, `rests-on-none`) {
		t.Fatalf("expected the rests-on-none class, got: %s", got)
	}
	if !strings.Contains(got, `>none<span class="claim-rests-on-reason"> — `) || !strings.Contains(got, `fixture &lt;claim&gt;`) {
		t.Fatalf("expected the stated-absence 'none' plus an HTML-escaped reason, got: %s", got)
	}
}

// TestEdgesHTML_RestsOnNoneReasonRoutesThroughInlineMarkdown covers the
// change routing the rests_on: none Reason through markdown.RenderInline instead of a
// bare html.EscapeString: Reason is hand-written prose that routinely names
// a claim id or a path, so it should be able to carry a code span or a
// link (the INLINE ceiling — no block constructs), the same subset every
// other prose field already gets via the "markdown"/"cell" funcs.
func TestEdgesHTML_RestsOnNoneReasonRoutesThroughInlineMarkdown(t *testing.T) {
	c := model.Claim{
		Facet:        "contract",
		MigratedFrom: "docs/tabs/widget.html", // opens the footer; see the note above.
		RestsOn:      model.RestsNone("see `widget.contract.retry-policy` for the real gate"),
	}
	got := string(edgesHTML(c))
	if !strings.Contains(got, "<code>widget.contract.retry-policy</code>") {
		t.Fatalf("expected the code span in Reason to render as <code>, got: %s", got)
	}
	if strings.Contains(got, "`widget.contract.retry-policy`") {
		t.Fatalf("Reason's backticks should not survive as literal text, got: %s", got)
	}
}

// TestEdgesHTML_RestsOnNoneReasonHostileHTMLStillEscaped guards the
// INLINE ceiling: RenderInline still HTML-escapes anything that isn't one
// of its recognized inline constructs, so a Reason is never a vector for
// raw markup even after the switch away from a bare html.EscapeString.
func TestEdgesHTML_RestsOnNoneReasonHostileHTMLStillEscaped(t *testing.T) {
	c := model.Claim{
		Facet:        "contract",
		MigratedFrom: "docs/tabs/widget.html", // opens the footer; see the note above.
		RestsOn:      model.RestsNone(`<script>alert(1)</script>`),
	}
	got := string(edgesHTML(c))
	if !strings.Contains(got, "rests-on-none") {
		t.Fatalf("this test only proves anything if the none row rendered at all, got: %s", got)
	}
	if strings.Contains(got, "<script>") {
		t.Fatalf("hostile HTML in Reason leaked unescaped: %s", got)
	}
}

// TestEdgesHTMLWithLinks_RestsOnNoneAloneStillRendersTheStrip is the RETRY
// re-pin (fix-list item 10) of what this test used to call the "newly-
// reachable zero-footer case". "rests_on: none" is a stated absence, not
// a link, so a claim whose only footer content is that row has no edges and
// no files — and, before this retry, that all-zero state suppressed the
// WHOLE footer, reason and none row included.
//
// That was the exact defect the retry's verifier evidence names ("852
// footers vs 290 sources chips; no footer at all for an edgeless claim"):
// the stated-absence row is mandatory when nothing is named, so this
// all-zero shape is not a rare edge case, it is the FIRST state most real
// projects' claims are in, and it was rendering no strip at all — the
// none row (with its author-written Reason) simply vanished. 05 §8
// item 5 is explicit that this state still shows "four zeros and the
// comment count", so the strip, the RELATIONSHIPS door and the none
// row (with its reason) now all render — only the numeral is zero.
func TestEdgesHTMLWithLinks_RestsOnNoneAloneStillRendersTheStrip(t *testing.T) {
	c := model.Claim{
		ID: "widget.contract.ungoverned", Module: "widget", Facet: "contract",
		RestsOn: model.RestsNone("no doctrine hub covers retries yet"),
	}

	got := string(EdgesHTMLWithLinks(c, nil, nil, nil))
	if !strings.Contains(got, `<div class="claim-footer">`) {
		t.Fatalf("a claim whose only footer content is rests_on: none must still render the strip, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">0 relationships</span>`) {
		t.Fatalf("expected the zero-relationships chip, got: %s", got)
	}
	if !strings.Contains(got, `<li class="claim-rests-on rests-on-none claim-relationship-none">none<span class="claim-rests-on-reason"> — no doctrine hub covers retries yet</span></li>`) {
		t.Fatalf("expected the rests_on none row with its reason, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-footer-chip-label">No sources</span>`) {
		t.Fatalf("expected the zero-sources chip, got: %s", got)
	}

	// One linked file changes nothing about whether the row or the strip
	// render — both already do — only the relationships panel gains its
	// implemented-in row alongside the unchanged "none" row.
	withFile := string(EdgesHTMLWithLinks(c, []implink.ViewFile{{File: "a.go"}}, nil, nil))
	if !strings.Contains(withFile, `<span class="claim-footer-chip-label">0 relationships</span>`) {
		t.Fatalf("expected a 0-relationships chip once something is disclosable, got: %s", withFile)
	}
	if !strings.Contains(withFile, `<li class="claim-rests-on rests-on-none claim-relationship-none">none<span class="claim-rests-on-reason"> — no doctrine hub covers retries yet</span></li>`) {
		t.Fatalf("the rests_on none row must ride along inside a footer that is emitted, got: %s", withFile)
	}
}

func TestEdgesHTML_FullClaimAllFields(t *testing.T) {
	c := model.Claim{
		Facet:         "contract",
		Module:        "widget",
		RestsOn:       model.RestsOnIDs("widget.contract.a", "widget.contract.b", "widget.contract.c"),
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
		`claim-rests-on`,
		`href="#widget.contract.a"`,
		`href="#widget.contract.b"`,
		`href="#widget.contract.c"`,
		`migrated_from: docs/tabs/widget.html`,
		`claim-review-pending`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("edgesHTML missing %q, got: %s", want, got)
		}
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
	if !strings.Contains(got, `<span class="claim-relationship-direction-label">DEPENDED ON BY</span>`) {
		t.Fatalf("expected a DEPENDED ON BY direction header, got: %s", got)
	}
	if !strings.Contains(got, `<li class="claim-depended-by claim-relationship"><a class="claim-ref" href="#widget.internals.a" data-claim-id="widget.internals.a" title="widget.internals.a">`) {
		t.Fatalf("expected each depended-by id rendered as its own relationship row, got: %s", got)
	}
	if strings.Count(got, `class="claim-depended-by claim-relationship"`) != 2 {
		t.Fatalf("expected one relationship row per depended-by id, got: %s", got)
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

// TestEdgesHTMLWithLinks_TargetPill_DraftTarget covers 05 §4.10's
// always-on lifecycle badge for a fixed-direction relationship row (R-I.2),
// which SUPERSEDES the old actionable-only .pill for RESTS ON/DEPENDED ON
// BY rows specifically: writeRelationshipRow passes a nil
// targetStatuses to the shared writeClaimRef so the old pill never doubles
// up beside the new dot+badge. targetPillHTML's original actionable-only
// contract still holds for the "extra" rows below.
func TestEdgesHTMLWithLinks_TargetPill_DraftTarget(t *testing.T) {
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: model.RestsOnIDs("widget.contract.a")}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusDraft},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if !strings.Contains(got, `<span class="claim-relationship-dot claim-relationship-dot--draft" aria-hidden="true"></span>`) {
		t.Fatalf("expected a draft lifecycle dot on a draft target, got: %s", got)
	}
	if !strings.Contains(got, `<span class="claim-relationship-badge claim-relationship-badge--draft">DRAFT</span>`) {
		t.Fatalf("expected a draft lifecycle badge on a draft target, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_LockedReviewPendingTarget(t *testing.T) {
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: model.RestsOnIDs("widget.contract.a")}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusLocked, ReviewPending: true},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	// A locked-but-review_pending target still reads "locked" in the
	// relationship badge — lifecycleModifier's doc comment states the
	// review_pending distinction is the target's own status pill's job, not
	// this row's.
	if !strings.Contains(got, `<span class="claim-relationship-badge claim-relationship-badge--locked">REVIEW PENDING</span>`) {
		t.Fatalf("expected a locked-hued badge reading REVIEW PENDING on a locked+review_pending target, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_HealthyLockedTargetGetsNoPill(t *testing.T) {
	// R-I.2: a healthy locked target now gets the LOCKED badge (the row's
	// lifecycle fact, not an alert) rather than nothing — the opposite of
	// the pre-redesign .pill's actionable-only rule for this row shape.
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: model.RestsOnIDs("widget.contract.a")}
	statuses := map[string]TargetStatus{
		"widget.contract.a": {Status: model.StatusLocked},
	}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if !strings.Contains(got, `<span class="claim-relationship-badge claim-relationship-badge--locked">LOCKED</span>`) {
		t.Fatalf("expected a healthy locked target to still carry the LOCKED lifecycle badge, got: %s", got)
	}
	if strings.Contains(got, `class="pill`) {
		t.Fatalf("a fixed-direction relationship row must never carry the old actionable-only .pill, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_UnknownTargetGetsNoPill(t *testing.T) {
	// A target id not present in the lookup at all (e.g. an unlinted or
	// otherwise unresolvable id) must not panic on the nil-map read and
	// must render no dot and no badge, same as an empty/nil statuses map.
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: model.RestsOnIDs("widget.contract.a")}
	statuses := map[string]TargetStatus{"widget.contract.b": {Status: model.StatusDraft}}
	got := string(EdgesHTMLWithLinks(c, nil, nil, statuses))
	if strings.Contains(got, `claim-relationship-dot`) || strings.Contains(got, `claim-relationship-badge`) {
		t.Fatalf("a target absent from the lookup must get no dot and no badge, got: %s", got)
	}
}

func TestEdgesHTMLWithLinks_TargetPill_NilStatuses_DegradesToNoPill(t *testing.T) {
	// The default funcMap binding (edgesHTML) always calls
	// EdgesHTMLWithLinks with a nil targetStatuses map — this is the
	// degrade-under-the-default-binding contract the task requires.
	c := model.Claim{Facet: "contract", Module: "widget", RestsOn: model.RestsOnIDs("widget.contract.a")}
	got := string(edgesHTML(c))
	if strings.Contains(got, `class="pill`) {
		t.Fatalf("edgesHTML (the default binding) must never render a target pill, got: %s", got)
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
//
// RETRY RE-PIN (fix-list item 9; 05 §4.10's four columns, R-I.2). RestsOn
// renders through writeRelationshipRow — one of R09.4's three fixed
// directions — which no longer calls writeClaimRef with the elided,
// context-relative `claim-ref-prefix` at all (showPrefix=false): the
// target's own `module · facet` is now an UNCONDITIONAL meta column
// (writeRelationshipMeta) instead, shown on every row regardless of how far
// the target is from the reader's own context. The three-tier elision this
// test used to pin is real, but it now lives in writeClaimRef's OTHER
// caller, writeIDListItems (migrated_from-adjacent extras) — see
// TestEdgesHTML_ElisionTiers_ExtrasStillElide below for that half.
func TestEdgesHTML_ElisionTiers(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.self",
		Module: "widget",
		Facet:  "contract",
		RestsOn: model.RestsOnIDs(
			"widget.contract.retry-policy",  // same module + facet
			"widget.internals.retry-buffer", // same module, other facet
			"ledger.contract.spend-cap",     // other module entirely
		),
	}
	got := string(edgesHTML(c))

	for _, want := range []string{
		// No claim-ref-prefix span on any row any more — the title is bare
		// in all three cases, same-context or not.
		`title="widget.contract.retry-policy"><span class="claim-ref-label">Retry Policy</span>`,
		`title="widget.internals.retry-buffer"><span class="claim-ref-label">Retry Buffer</span>`,
		`title="ledger.contract.spend-cap"><span class="claim-ref-label">Spend Cap</span>`,
		// The meta column instead names the TARGET's own module/facet,
		// unconditionally — including the same-module-and-facet case, which
		// the old prefix elided into nothing.
		`<span class="claim-relationship-meta">Widget · Contract</span>`,
		`<span class="claim-relationship-meta">Widget · Internals</span>`,
		`<span class="claim-relationship-meta">Ledger · Contract</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the edges footer, got: %s", want, got)
		}
	}

	// No prefix spans at all: the elided inline prefix is exclusively
	// writeIDListItems' territory now (migrated_from-adjacent
	// extras), never a fixed-direction relationship row's.
	if strings.Contains(got, "claim-ref-prefix") {
		t.Errorf("a fixed-direction relationship row must carry no inline prefix span, got: %s", got)
	}
	// All three targets get their own meta column, tiered elision or not.
	if n := strings.Count(got, "claim-relationship-meta"); n != 3 {
		t.Errorf("expected 3 meta columns (one per RestsOn target), got %d in: %s", n, got)
	}
}

// TestEdgesHTML_RestsOnUsesMetaColumn not prefix elision: rests_on rows
// carry module/facet in claim-relationship-meta, not claim-ref-prefix.
func TestEdgesHTML_RestsOnUsesMetaColumn(t *testing.T) {
	c := model.Claim{
		ID:     "widget.contract.self",
		Module: "widget",
		Facet:  "contract",
		RestsOn: model.RestsOnIDs(
			"widget.contract.retry-policy",
			"widget.internals.retry-buffer",
			"ledger.contract.spend-cap"),
	}
	got := string(edgesHTML(c))

	for _, want := range []string{
		`title="widget.contract.retry-policy"><span class="claim-ref-label">Retry Policy</span>`,
		`title="widget.internals.retry-buffer"><span class="claim-ref-label">Retry Buffer</span>`,
		`title="ledger.contract.spend-cap"><span class="claim-ref-label">Spend Cap</span>`,
		`Widget · Internals`,
		`Ledger · Contract`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the edges footer, got: %s", want, got)
		}
	}
	if strings.Contains(got, "claim-ref-prefix") {
		t.Errorf("rests_on rows must not use claim-ref-prefix, got: %s", got)
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
		RestsOn: model.RestsOnIDs("widget.contract.four.segments", "loose-id"),
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
		RestsOn: model.RestsOnIDs(unshaped, shaped),
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
		// Shaped: the label span and, RETRY RE-PIN (fix-list item 9), the
		// unconditional meta column (writeRelationshipMeta) both get their
		// DisplayCase'd segments escaped. RestsOn renders through
		// writeRelationshipRow, which no longer draws the elided
		// claim-ref-prefix at all (showPrefix=false).
		`<span class="claim-ref-label">Slug&amp;x</span>`,
		`<span class="claim-relationship-meta">&lt;img Src=x Onerror=&#34;alert(1)&#34;&gt; · &#39;facet</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected escaped %q in the footer, got: %s", want, got)
		}
	}
}

// TestPartialHeadings_LabelIDAndKeepMachineIDReachable is C4/item 5 across all
// seven layout partials at once — banner.html included, the one partial with no
// edges footer for the rest of this work to reach. The heading shows the label;
// data-claim-id and the title tooltip keep the id typeable for "dossierx claim
// lock <id>", greppable in the rendered HTML, and reachable from the viewer JS.
//
// v0.4.1 flexes that head: the label and its status pill are wrapped in a
// <span class="label"> and the comment chip's slot follows as a sibling, so CSS
// can push the chip to the far edge without the pill going with it. SIX of the
// seven partials take the new shape; BANNER KEEPS THE OLD FLAT LINE, because
// banner carries no chip (it has no edges footer and no comment surface at all)
// and therefore has nothing to flex against. A rewrite that applies the wrapped
// prefix uniformly is wrong, and one that relaxes the assertion until banner
// passes stops proving the shape at all.
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

	// The whole head, byte for byte, for the six non-banner layouts: the
	// opening <div class="k"> tag is unchanged and the title/pill/id move
	// inside <span class="label"> with the one space between the title and the
	// pill preserved. The comment slot belongs to the footer, not the head.
	//
	// v0.4.2 (docs/design/screens/07-claim-boundary-no-embodiment.md §4.3/§4.4,
	// LANES.md's L3 section) wraps the title in its own <span class="k-title">
	// and adds a visible <span class="k-id"> mono id line as a third child of
	// .label, alongside the existing pill — the "claim id/slug mono line" L3
	// owns. The LOCKED pill also gains the padlock glyph StatusIconHTML emits
	// for every status: closed for "ps"/"pw" (an approval is on record), open
	// for "pv" (see components.go's StatusIconHTML doc comment) — this claim
	// is StatusLocked, so it takes the closed glyph exercised below.
	const wantHead = `<div class="k" data-claim-id="widget.contract.retry-policy" title="widget.contract.retry-policy">` +
		`<span class="label"><span class="k-title">Retry Policy</span> <span class="pill ps">` +
		`<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock"/></svg>Locked</span>` +
		`<span class="k-id">widget.contract.retry-policy</span></span></div>`

	// banner's flat head, unchanged from before v0.4.1 except the same
	// padlock glyph every other LOCKED chip now carries — banner keeps no
	// .label wrapper and therefore no .k-title/.k-id split (it has no edges
	// footer or comment surface to flex the head against).
	const wantBannerHead = `<div class="k" data-claim-id="widget.contract.retry-policy" title="widget.contract.retry-policy">` +
		`Retry Policy <span class="pill ps">` +
		`<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock"/></svg>Locked</span></div>`

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

			want := wantHead
			if layout == model.LayoutBanner {
				want = wantBannerHead
			}
			if !strings.Contains(got, want) {
				t.Fatalf("expected the labeled, id-bearing heading\n want: %s\n got:  %s", want, got)
			}
			if layout == model.LayoutBanner && strings.Contains(got, "claim-comments-slot") {
				t.Errorf("banner must carry no comment chip slot, got: %s", got)
			}
			if layout != model.LayoutBanner {
				footerIdx := strings.Index(got, `<div class="claim-footer">`)
				chipIdx := strings.Index(got, `claim-comments-slot`)
				if footerIdx < 0 || chipIdx < footerIdx {
					t.Errorf("the comment slot must be in the footer, not the heading, got: %s", got)
				}
			}

			// The heading label is the BARE label — a claim's own module and
			// facet are the page the reader is standing on.
			if strings.Contains(got, `>Widget · Contract › Retry Policy`) {
				t.Errorf("a claim's own heading must not carry a prefix, got: %s", got)
			}
			// The root element's id= is untouched: it is what a #hash deep link
			// and stripDuplicateClaimIDs both key off. The head's new spans add
			// no second ` id="` for stripDuplicateClaimIDs to hit by mistake.
			if !strings.Contains(got, ` id="widget.contract.retry-policy"`) {
				t.Errorf("the root element must keep its id attribute, got: %s", got)
			}
			if n := strings.Count(got, ` id="`); n != 1 {
				t.Errorf("exactly one ` id=\"` attribute must appear (the root section's), got %d in: %s", n, got)
			}
		})
	}
}

// A partial heading is ordinary auto-escaping template context — claimLabel
// returns a plain string, not template.HTML — so a hostile id must come back
// escaped by html/template itself, with no hand-escaping in the partial. This
// pins that no future edit "helpfully" wraps claimLabel's output in a
// template.HTML the way the edges footer legitimately has to.
//
// The head now also carries the comment chip, and the chip is the OTHER kind:
// CommentChipHTML returns template.HTML from a FuncMap, which bypasses
// html/template's automatic escaping entirely, so its data-claim-id is escaped
// BY HAND with html.EscapeString or not at all. Both kinds now interpolate the
// same hostile id into the same <div class="k">, which is why this test — the
// one that looks at the head — is where the chip's hand-escaping is proven.
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
	// The chip's hand-escaped attribute, in the head, byte for byte. Asserting
	// the literal (rather than recomputing html.EscapeString here) is the point:
	// a future edit that drops the hand-escaping would still pass a test that
	// escapes its own expectation the same way the code does.
	const wantChip = `<span class="claim-comments-slot" hidden><button type="button" class="comment-chip comment-chip--empty" ` +
		`data-claim-id="x&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;" aria-controls="commentsPanel"`
	if !strings.Contains(got, wantChip) {
		t.Fatalf("expected the chip's data-claim-id hand-escaped in the head\n want: %s\n got:  %s", wantChip, got)
	}
	// And nothing anywhere closed out of that attribute.
	if strings.Contains(got, `data-claim-id="x">`) {
		t.Fatalf("the chip's data-claim-id broke out of its attribute: %s", got)
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
