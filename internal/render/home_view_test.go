package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/approvaledit"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
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

	cat, err := catalog.Build([]model.Claim{edited, upstream, direct, threaded, quiet}, cfg)
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
		{"draft", "1", edited.ID},
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
	if !strings.Contains(out, "6 items across 4 kinds") {
		t.Errorf("header summary missing \"6 items across 4 kinds\"")
	}
	if strings.Contains(out, `class="home-clear"`) {
		t.Errorf("the all-clear line rendered while cards are waiting")
	}
}

// TestRender_HomeEmptyStates pins the design review's empty states: with
// nothing waiting the card row is one "Nothing waiting on you" line, and a
// project with no constitution.yaml has no Constitution tile, while one with
// it shows the file's counts and lock state. The sidebar's Constitution
// entry follows the file (a project claim alone also keeps it, see
// project_claims_test.go's fixture, which has one).
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

	writeFile(t, cfg.ConstitutionPath(),
		"status: locked\ninvariants:\n  - slug: one\n    body: One rule.\n  - slug: two\n    body: Two rules.\ndecisions:\n  - slug: d\n    body: A decision.\n")
	out = renderProject(t, cfg, claims)
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
