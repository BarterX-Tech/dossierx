package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/approvaledit"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
)

var homeCardRE = regexp.MustCompile(`<a class="home-card" data-kind="([a-z]+)" href="#([^"]+)">\s*<span class="home-card__count">(\d+)</span>`)

// homeCards reads the rendered Home page's "Waiting on you" cards as
// kind -> {count, target}, in document order.
func homeCards(t *testing.T, out string) (cards map[string][2]string, order []string) {
	t.Helper()
	cards = map[string][2]string{}
	for _, m := range homeCardRE.FindAllStringSubmatch(out, -1) {
		cards[m[1]] = [2]string{m[3], m[2]}
		order = append(order, m[1])
	}
	return cards, order
}

// TestRender_HomeWaitingOnYou pins what the Home page (NIT-196) tells the
// human is waiting on them, read from the finished document. Each card
// counts one kind of thing from its own source, a kind at zero is not
// rendered, and each card leads to a claim of its kind.
func TestRender_HomeWaitingOnYou(t *testing.T) {
	cfg := projectTestConfig(t)

	edited := projectTestClaim("widget", "edited", model.StatusDraft)
	upstream := projectTestClaim("widget", "upstream", model.StatusLocked)
	direct := projectTestClaim("gateway", "direct", model.StatusLocked)
	// Live readiness marks this one review_pending too, but only because of
	// its own open thread: it belongs on the thread card, not "to re-read".
	threaded := projectTestClaim("gateway", "threaded", model.StatusLocked)
	threaded.Comments = []model.Comment{openComment("c-aaaaaa", "why?"), openComment("c-bbbbbb", "and?")}
	quiet := projectTestClaim("gateway", "quiet", model.StatusLocked)
	// A draft whose dependency changed has no approval to re-read, and a
	// locked claim with only a ledger-integrity cause is the status strip's,
	// not "something it rests on changed": neither is to re-read.
	draftDep := projectTestClaim("gateway", "draftdep", model.StatusDraft)
	drifted := projectTestClaim("gateway", "drifted", model.StatusLocked)

	cat, err := catalog.Build([]model.Claim{edited, upstream, direct, threaded, quiet, draftDep, drifted}, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	cause := func(k readiness.CauseKind) readiness.Assessment {
		return readiness.Assessment{ReviewPending: true, ReviewCauses: []readiness.Cause{{Kind: k}}}
	}
	cat.SetReadiness(map[string]readiness.Assessment{
		edited.ID:   cause(readiness.CauseUnapprovedEdit),
		upstream.ID: cause(readiness.CauseUpstreamDependencyReview),
		direct.ID:   cause(readiness.CauseDirectDependencyChange),
		threaded.ID: cause(readiness.CauseOwnThread),
		draftDep.ID: cause(readiness.CauseDirectDependencyChange),
		drifted.ID:  cause(readiness.CauseApprovalContentDrift),
	})
	cat.SetApprovedEdits(map[string]approvaledit.Change{edited.ID: {ClaimID: edited.ID}})

	out, err := Render(cat, cfg)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	cards, order := homeCards(t, out)

	want := []struct{ kind, count, target string }{
		{"edited", "1", edited.ID},
		{"review", "2", direct.ID},
		{"thread", "2", threaded.ID},
		{"draft", "2", draftDep.ID},
	}
	if got := strings.Join(order, ","); got != "edited,review,thread,draft" {
		t.Fatalf("cards in order %q, want edited,review,thread,draft", got)
	}
	for _, w := range want {
		got := cards[w.kind]
		if got[0] != w.count || got[1] != w.target {
			t.Errorf("%s card = count %s -> #%s, want count %s -> #%s", w.kind, got[0], got[1], w.count, w.target)
		}
	}
	if !strings.Contains(out, "7 items across 4 kinds") {
		t.Errorf("header summary missing \"7 items across 4 kinds\"")
	}
	if strings.Contains(out, `class="home-clear"`) {
		t.Errorf("the all-clear line rendered while cards are waiting")
	}
}

// TestRender_HomeEmptyStates pins the design review's empty states: with
// nothing waiting the card row is one "Nothing waiting on you" line, and a
// project with no constitution.yaml has no Constitution tile, while one with
// it shows the file's counts and the roof gate's lock state (the lock
// store's verdict, not the file's status line). The sidebar's Constitution
// entry shows while there is the file or a project claim.
func TestRender_HomeEmptyStates(t *testing.T) {
	cfg := projectTestConfig(t)
	claims := []model.Claim{
		projectTestClaim("widget", "a", model.StatusLocked),
		projectTestClaim("gateway", "b", model.StatusLocked),
	}

	out := renderProject(t, cfg, claims)
	if cards, _ := homeCards(t, out); len(cards) != 0 {
		t.Fatalf("cards rendered with nothing waiting: %v", cards)
	}
	if !strings.Contains(out, "<strong>Nothing waiting on you</strong>") {
		t.Errorf("the all-clear line is missing")
	}
	if strings.Contains(out, `data-tile="constitution"`) {
		t.Errorf("Constitution tile rendered for a project with no constitution.yaml")
	}
	if strings.Contains(out, `class="sec-tab site-nav__item constitution-tab"`) {
		t.Errorf("Constitution sidebar entry rendered with no constitution.yaml and no project claims")
	}
	if !strings.Contains(out, `data-tile="modules" href="#widget"`) ||
		!strings.Contains(out, "2 claims · 2 locked</span>") {
		t.Errorf("Modules tile missing, or not leading to the first module with 2 of 2 locked")
	}

	// A project claim with no constitution.yaml keeps the sidebar entry: its
	// page is where project claims live. The tile still needs the file.
	scope := model.Claim{ID: "project.scope", Scope: model.ScopeProject, Status: model.StatusDraft,
		Layout: model.LayoutCard, Body: "scope", RestsOn: model.RestsNone("test fixture")}
	out = renderProject(t, cfg, append(append([]model.Claim(nil), claims...), scope))
	if !strings.Contains(out, `class="sec-tab site-nav__item constitution-tab"`) {
		t.Errorf("Constitution sidebar entry missing for a project claim with no constitution.yaml")
	}
	if strings.Contains(out, `<a class="home-tile" data-tile="constitution"`) {
		t.Errorf("Constitution tile rendered with no constitution.yaml")
	}

	const roof = "status: locked\ninvariants:\n  - slug: one\n    body: One rule.\n  - slug: two\n    body: Two rules.\ndecisions:\n  - slug: d\n    body: A decision.\n"
	writeFile(t, cfg.ConstitutionPath(), roof)

	// "status: locked" with no lock record is the gate's "unrecorded": never
	// approved, so neither the tile nor the sidebar may say Locked.
	out = renderProject(t, cfg, claims)
	if tile := between(t, out, `<a class="home-tile" data-tile="constitution"`, `</a>`); !strings.Contains(tile, `data-state="unrecorded">Not locked`) {
		t.Errorf("an unrecorded roof must read Not locked:\n%s", tile)
	}
	if strings.Contains(out, `class="dx-icon dx-icon--lock site-nav__lock"`) {
		t.Errorf("the sidebar shows a lock for a roof with no lock record")
	}
	// The Constitution page's own meter reads the same verdict, so the two
	// pages of one viewer never disagree about the roof.
	if !strings.Contains(out, "6 of 800 words · not locked</p>") {
		t.Errorf("the Constitution meter must name the gate's verdict for an unrecorded roof:\n%s", between(t, out, `<p class="constitution-meter">`, `</p>`))
	}

	// Record the lock the way constitution lock does, then edit the file:
	// the gate says edited, and so must Home.
	f, err := constitution.Load(cfg.ConstitutionPath())
	if err != nil {
		t.Fatalf("load roof: %v", err)
	}
	store, err := lock.LoadStore(cfg.LockStorePath())
	if err != nil {
		t.Fatalf("load store: %v", err)
	}
	store.Constitution = &constitution.LockRecord{Hash: constitution.Hash(f), Reason: "test"}
	if err := store.Save(); err != nil {
		t.Fatalf("save store: %v", err)
	}
	out = renderProject(t, cfg, claims)
	if !strings.Contains(out, `class="dx-icon dx-icon--lock site-nav__lock"`) {
		t.Errorf("the sidebar shows no lock for a locked roof")
	}
	writeFile(t, cfg.ConstitutionPath(), strings.Replace(roof, "One rule.", "One rule, edited.", 1))
	edited := renderProject(t, cfg, claims)
	if tile := between(t, edited, `<a class="home-tile" data-tile="constitution"`, `</a>`); !strings.Contains(tile, `data-state="edited">Edited since lock`) {
		t.Errorf("a roof edited after its lock must not read Locked:\n%s", tile)
	}
	if strings.Contains(edited, `class="dx-icon dx-icon--lock site-nav__lock"`) {
		t.Errorf("the sidebar shows a lock for a roof edited after its lock")
	}
	if !strings.Contains(out, `class="sec-tab site-nav__item constitution-tab"`) {
		t.Errorf("Constitution sidebar entry missing once constitution.yaml exists")
	}
	tile := between(t, out, `<a class="home-tile" data-tile="constitution"`, `</a>`)
	for _, want := range []string{
		`data-state="locked">Locked`,
		`<span class="home-stat__count">2</span><span class="home-stat__label">invariants</span>`,
		`<span class="home-stat__count">1</span><span class="home-stat__label">decision</span>`,
		`<span class="home-stat__count">0</span><span class="home-stat__label">glossary terms</span>`,
	} {
		if !strings.Contains(tile, want) {
			t.Errorf("Constitution tile missing %q:\n%s", want, tile)
		}
	}
}

// TestRender_HomeCardsNameABoundedNumberOfClaims: a card's line names at
// most three claims and counts the rest, so Home stays the same size on a
// corpus of any size.
func TestRender_HomeCardsNameABoundedNumberOfClaims(t *testing.T) {
	cfg := projectTestConfig(t)
	var claims []model.Claim
	for i := 0; i < 7; i++ {
		c := projectTestClaim("widget", "t"+strconv.Itoa(i), model.StatusLocked)
		c.Comments = []model.Comment{openComment("c-"+strconv.Itoa(100000+i), "q")}
		claims = append(claims, c)
	}
	out := renderProject(t, cfg, claims)
	card := between(t, out, `<a class="home-card" data-kind="thread"`, `</a>`)
	if !strings.Contains(card, "On T0, T1, T2 and 4 more.") {
		t.Errorf("thread card does not name three claims and count four more:\n%s", card)
	}
}

// TestRender_HomeSizeIsIndependentOfCorpusSize pins the graph-safety bound
// docs/graph-safety/nit-196-home.md records: with every claim on every card
// (a draft or a locked claim with a dependency change, each with an open
// thread, across five modules), the Home section grows only by the digits of
// its counts, and the draft card names at most three modules.
func TestRender_HomeSizeIsIndependentOfCorpusSize(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets: [contract, internals]\nmodules: [m1, m2, m3, m4, m5]\nclaims_dir: claims\n")
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	homeBytes := func(n int) (int, string) {
		var claims []model.Claim
		ra := map[string]readiness.Assessment{}
		for i := 0; i < n; i++ {
			st := model.StatusDraft
			if i%2 == 0 {
				st = model.StatusLocked
			}
			c := projectTestClaim("m"+strconv.Itoa(1+i%5), "c"+strconv.Itoa(100000+i), st)
			c.Comments = []model.Comment{openComment("c-"+strconv.Itoa(100000+i), "q")}
			claims = append(claims, c)
			ra[c.ID] = readiness.Assessment{ReviewPending: true, ReviewCauses: []readiness.Cause{{Kind: readiness.CauseUpstreamDependencyReview}}}
		}
		cat, err := catalog.Build(claims, cfg)
		if err != nil {
			t.Fatalf("catalog.Build: %v", err)
		}
		cat.SetReadiness(ra)
		out, err := Render(cat, cfg)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		home := between(t, out, `<section class="module-section home-section"`, `</main>`)
		return len(home), home
	}
	small, _ := homeBytes(10)
	large, home := homeBytes(2000)
	if large-small > 64 {
		t.Errorf("Home grew %d bytes from 10 to 2,000 claims (%d -> %d); it must grow only by count digits", large-small, small, large)
	}
	if !strings.Contains(home, "and 2 more modules.") {
		t.Errorf("draft card does not name three modules and count the other two")
	}
}

func between(t *testing.T, s, start, end string) string {
	t.Helper()
	i := strings.Index(s, start)
	if i < 0 {
		t.Fatalf("%q not found in the rendered document", start)
	}
	j := strings.Index(s[i:], end)
	if j < 0 {
		t.Fatalf("%q not found after %q", end, start)
	}
	return s[i : i+j]
}
