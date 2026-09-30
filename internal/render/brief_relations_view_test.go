package render

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// briefRelationsProject writes a project directory holding briefs and returns
// its config (loaded from disk, so cfg.Dir() anchors the internal-source
// paths the drift lint reads) and the briefs Load reads from it.
func briefRelationsProject(t testing.TB, files map[string]string) (*config.Config, *briefs.Set) {
	t.Helper()
	dir := t.TempDir()
	files["project.config.yaml"] = "schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\n"
	files["claims/.keep"] = ""
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.LoadConfig(filepath.Join(dir, "project.config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg, briefs.Load(cfg)
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// claimSection returns one claim card's rendered <section>.
func claimSection(t *testing.T, page, id string) string {
	t.Helper()
	start := strings.Index(page, `<section class="claim claim-card card" id="`+id+`"`)
	if start < 0 {
		t.Fatalf("no card for %s", id)
	}
	end := strings.Index(page[start:], "</section>")
	return page[start : start+end]
}

var briefRowRE = regexp.MustCompile(`<li class="claim-brief claim-relationship[^"]*"[^>]*>.*?</li>`)

// TestRenderWith_BriefsInAClaimsRelationships is the NIT-202 contract at the
// render boundary: a claim a brief's rests_on names, or whose internal source
// pins a brief, gets a derived BRIEFS group in its relationships panel —
// explained-by rows sorted by brief id, cited rows with their pin or "pin out
// of date" exactly when source-internal-drift reports the source — and the
// relationships count includes those rows. A guidance brief (resting on
// nothing, cited by nothing) appears on no card; a rests_on id that names no
// claim adds nothing; a source path in odd casing is not a brief path; and a
// claim with no brief row renders no group at all.
func TestRenderWith_BriefsInAClaimsRelationships(t *testing.T) {
	rounding := "---\nsummary: How other apps round.\n---\n# Rounding in other apps\n"
	stale := "---\nsummary: Stale evidence.\nstatus: locked\n---\n# Old research\n"
	twice := "---\nsummary: Cited by two sources.\n---\n# Cited twice\n"
	drifted := sha("twice, as it read when the second source pinned it")
	cfg, set := briefRelationsProject(t, map[string]string{
		// Explaining briefs, written so the tree's walk order (decisions,
		// features) differs from nothing; the ghost id names no claim.
		"briefs/features/split-a-bill.md": "---\nsummary: Split a bill.\nstatus: locked\nrests_on: [widget.contract.a]\n---\n# Split a bill\n",
		"briefs/decisions/round-once.md":  "---\nsummary: Round once.\nrests_on: [widget.contract.ghost, widget.contract.a]\n---\n# Balances round to the cent, once\n",
		"briefs/guidance/voice.md":        "---\nsummary: How we write.\nstatus: locked\n---\n# Voice\n",
		"briefs/research/rounding.md":     rounding,
		"briefs/research/stale.md":        stale,
		"briefs/research/twice.md":        twice,
		"briefs/research/pane-sketch.svg": "<svg/>",
		"briefs/research/uses-sketch.md":  "---\nsummary: Uses the sketch.\n---\n![s](pane-sketch.svg)\n",
		"notes/elsewhere.md":              "outside briefs",
	})
	// A brief's pin is its content hash, the one brief lock signs (NIT-198),
	// not the file's sha256.
	pin := func(id string) string {
		b, ok := set.Lookup(id)
		if !ok {
			t.Fatalf("no brief %s", id)
		}
		return b.LockHash
	}
	internal := func(ref int, path, sum string) model.Source {
		return model.Source{Ref: ref, Kind: model.SourceKindInternal, Title: "src " + path, Path: path, SHA256: sum}
	}
	cat := &catalog.Catalog{Claims: []model.Claim{
		{
			ID: "widget.contract.a", Module: "widget", Facet: "contract", Layout: model.LayoutCard,
			Status: model.StatusLocked, Body: "[1] [2] [3] [4] [5] [6] [7] [8] [9]",
			// A rests_on target and (claim c) a dependent, so the BRIEFS
			// group has both fixed directions to come after.
			RestsOn: model.RestsOn{IDs: []string{"widget.contract.b"}},
			Sources: []model.Source{
				internal(1, "briefs/research/stale.md", sha("what the brief said at pin time")),
				internal(2, "./briefs/research/rounding.md", pin("research.rounding")),
				internal(3, "briefs/research/gone.md", sha("x")),
				internal(4, "Briefs/Research/Rounding.md", sha(rounding)),
				internal(5, "notes/elsewhere.md", sha("outside briefs")),
				internal(6, "briefs/research/pane-sketch.svg", sha("<svg/>")),
				{Ref: 7, Kind: model.SourceKindExternal, Title: "ext", URL: "https://example.test/briefs/research/rounding.md", AccessedOn: "2026-01-02"},
				// One brief, two sources: the first pin holds, the second
				// has drifted. One row, out of date, hovering the drifted pin.
				internal(8, "briefs/research/twice.md", pin("research.twice")),
				internal(9, "briefs/research/twice.md", drifted),
			},
		},
		{ID: "widget.contract.b", Module: "widget", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "plain"},
		{ID: "widget.contract.c", Module: "widget", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "plain",
			RestsOn: model.RestsOn{IDs: []string{"widget.contract.a"}}},
	}}
	at := time.Unix(1_700_000_000, 0).UTC()
	page, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, 0)
	if err != nil {
		t.Fatal(err)
	}
	again, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, at, 0)
	if err != nil || again != page {
		t.Fatal("the derived rows must render identically on every pass")
	}

	a := claimSection(t, page, "widget.contract.a")
	if strings.Count(a, "claim-relationship-direction--briefs") != 1 {
		t.Fatalf("claim a must carry exactly one BRIEFS group:\n%s", a)
	}
	// 1 rests_on + 1 dependent + 2 explaining + 4 cited (gone, rounding,
	// stale, twice) rows: the count includes the six brief rows.
	if !strings.Contains(a, `<span class="claim-footer-chip-label">8 relationships</span>`) ||
		!strings.Contains(a, `<span class="claim-relationship-direction-label">BRIEFS</span><span class="claim-relationship-direction-count">6</span>`) {
		t.Fatalf("the relationships count must include the brief rows:\n%s", a)
	}
	restsOn, dependedBy, briefsAt := strings.Index(a, ">RESTS ON<"), strings.Index(a, ">DEPENDED ON BY<"), strings.Index(a, "claim-relationship-direction--briefs")
	if restsOn < 0 || dependedBy < 0 || !(restsOn < dependedBy && dependedBy < briefsAt) {
		t.Fatalf("BRIEFS must come after RESTS ON and DEPENDED ON BY (at %d, %d, %d)", restsOn, dependedBy, briefsAt)
	}
	if strings.Index(a, "claim-brief-subgroup--explained") > strings.Index(a, "claim-brief-subgroup--cited") {
		t.Fatal("Explained by comes before Cited as evidence")
	}

	rows := briefRowRE.FindAllString(a, -1)
	want := []struct{ contains, lacks []string }{
		{contains: []string{`data-brief-id="decisions.round-once"`, `href="#brief-decisions-round-once"`, `>Balances round to the cent, once</a>`,
			`<span class="claim-brief-folder">Decisions</span>`, `<span class="claim-brief-state"></span>`, `claim-relationship-badge--draft">DRAFT<`}},
		{contains: []string{`data-brief-id="features.split-a-bill"`, `href="#brief-features-split-a-bill"`, `claim-relationship-badge--locked">LOCKED<`,
			`claim-relationship-dot--locked`}},
		{contains: []string{`title="no brief at this path">src briefs/research/gone.md</span>`, `claim-brief--pin-out-of-date`,
			`claim-brief-state--drift`, `>pin out of date</span>`}, lacks: []string{"href=", "claim-relationship-badge"}},
		{contains: []string{`data-brief-id="research.rounding"`, `href="#brief-research-rounding"`, `claim-brief-state--pin`,
			`>pinned ` + pin("research.rounding")[:12] + `</span>`, `claim-relationship-badge--draft">DRAFT<`}, lacks: []string{"pin-out-of-date"}},
		{contains: []string{`data-brief-id="research.stale"`, `claim-brief--pin-out-of-date`, `>pin out of date</span>`,
			`claim-relationship-badge--locked">LOCKED<`}},
		{contains: []string{`data-brief-id="research.twice"`, `claim-brief--pin-out-of-date`,
			`<span class="claim-brief-state claim-brief-state--drift" title="` + drifted + `">`}, lacks: []string{"pinned "}},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d brief rows, want %d:\n%s", len(rows), len(want), strings.Join(rows, "\n"))
	}
	for i, w := range want {
		for _, s := range w.contains {
			if !strings.Contains(rows[i], s) {
				t.Errorf("row %d lacks %q:\n%s", i, s, rows[i])
			}
		}
		for _, s := range w.lacks {
			if strings.Contains(rows[i], s) {
				t.Errorf("row %d must not carry %q:\n%s", i, s, rows[i])
			}
		}
	}
	for _, never := range []string{"guidance.voice", "research.uses-sketch", "pane-sketch", "Rounding.md", "elsewhere", "example.test/briefs"} {
		if strings.Contains(strings.Join(rows, ""), never) {
			t.Errorf("no brief row may come from %q", never)
		}
	}
	// The Sources panel keeps listing every source as it did.
	if strings.Count(a, `<li class="claim-source"`) != 9 {
		t.Errorf("the Sources panel must still list all nine sources")
	}

	if b := claimSection(t, page, "widget.contract.b"); strings.Contains(b, "claim-brief") {
		t.Fatalf("a claim no brief names or is cited by renders no BRIEFS group:\n%s", b)
	}
	if strings.Contains(page, `<section class="claim claim-card card" id="widget.contract.ghost"`) {
		t.Fatal("a rests_on id naming no claim must not conjure a card")
	}

	// A project holding no brief shows no group, even for a source whose path
	// looks like a brief's (FORMAT.md: no brief, no brief-derived byte).
	none, err := renderBoundedAt(cat, cfg, Extras{Briefs: briefs.FromFiles(cfg, nil)}, at, 0)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(claimSection(t, none, "widget.contract.a"), "claim-brief") {
		t.Fatal("with no brief in the project, no card may draw a BRIEFS group")
	}
}

// TestSourceDriftCheck_MemoKeysOnRecordID guards the drift memo's key
// (NIT-202 F10): the verdict depends on the path, the record_id and the
// recorded sha256, so two sources sharing a path and a pin but naming
// different records must each get their own verdict. A key without record_id
// would hand the second source the first one's cached "holds".
func TestSourceDriftCheck_MemoKeysOnRecordID(t *testing.T) {
	lineA := `{"id":"a","text":"first"}`
	cfg, _ := briefRelationsProject(t, map[string]string{
		"evidence/records.jsonl": lineA + "\n" + `{"id":"b","text":"second"}` + "\n",
	})
	drift := sourceDriftCheck(cfg)
	src := model.Source{Ref: 1, Kind: model.SourceKindInternal, Title: "r", Path: "evidence/records.jsonl", RecordID: "a", SHA256: sha(lineA)}
	if drift("widget.contract.a", src) {
		t.Fatal("record a's pin holds")
	}
	src.RecordID = "b"
	if !drift("widget.contract.a", src) {
		t.Fatal("record b does not hash to record a's pin, so its pin is out of date")
	}
}

// TestRenderWith_BriefRowsLinkToTheirOwnSection is NIT-202 audit F2: two
// briefs can spell the same brief-<folder>-<slug> (api/design-notes and
// api-design/notes), and the brief pages give the later one a -2 suffix. Each
// BRIEFS row must link to the section that holds its own brief — the row
// takes its anchor from the pages' map, never a recomputed plain id.
func TestRenderWith_BriefRowsLinkToTheirOwnSection(t *testing.T) {
	cfg, set := briefRelationsProject(t, map[string]string{
		"briefs/api/design-notes.md": "---\nsummary: One spelling.\nrests_on: [widget.contract.a]\n---\n# Api design notes\n",
		"briefs/api-design/notes.md": "---\nsummary: The other spelling.\nrests_on: [widget.contract.a]\n---\n# Api-design notes\n",
	})
	cat := &catalog.Catalog{Claims: []model.Claim{{
		ID: "widget.contract.a", Module: "widget", Facet: "contract", Layout: model.LayoutCard, Status: model.StatusDraft, Body: "a",
	}}}
	page, err := renderBoundedAt(cat, cfg, Extras{Briefs: set}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	rowLink := regexp.MustCompile(`<a class="claim-brief-ref" href="#([^"]+)" title="([^"]+)">`)
	links := rowLink.FindAllStringSubmatch(claimSection(t, page, "widget.contract.a"), -1)
	if len(links) != 2 {
		t.Fatalf("want two linked brief rows, got %d", len(links))
	}
	seen := map[string]bool{}
	for _, l := range links {
		anchor, path := l[1], l[2]
		if seen[anchor] {
			t.Fatalf("two rows link to the same section %q", anchor)
		}
		seen[anchor] = true
		section := `<section class="module-section brief-section" id="` + anchor + `" hidden data-brief-path="` + path + `"`
		if !strings.Contains(page, section) {
			t.Errorf("row for %s links to #%s, which is not that brief's section", path, anchor)
		}
	}
	if !seen["brief-api-design-notes"] || !seen["brief-api-design-notes-2"] {
		t.Fatalf("want the plain id and its -2 suffix, got %v", seen)
	}
}
