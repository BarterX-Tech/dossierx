package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/readiness"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// HomeView is the viewer's landing page (NIT-196): what is waiting on the
// human, then one tile per section. Every value is read from the catalog,
// the module groups, constitution.yaml and the lock store's constitution
// record, and nothing on it is written by the project except the title and
// eyebrow, which the shell already carries.
//
// Of the brief halves, the Open threads card carries one (NIT-198): a
// brief's open threads count on it beside the claims', since an open thread
// on a brief holds `brief lock` as one on a claim holds `claim lock`. The
// other cards' brief halves and the Briefs tile are not built yet; they are
// hidden, not shown as zero, as in the "no briefs/" state.
//
// The Features tile (NIT-201) is not built here: it is BriefsView's
// FeaturesTile (feature_page.go), read by the shell beside this view, and it
// shows only when briefs/features/ holds a brief.
//
// Cost: two passes over the claims (the cards, and the project-claim count)
// plus one over each claim's comments, one over the module groups, and two
// small file reads for the Constitution tile (constitution.yaml and the lock
// store). Output is bounded independently of corpus size: a card names at
// most homeCardNames claims and a draft card at most homeDraftModules
// modules, each followed by a count of the rest.
type HomeView struct {
	// Cards holds only the kinds with something waiting, in the fixed
	// order edited, re-read, threads, drafts. A kind at zero is absent.
	Cards []HomeCard
	// Items is the sum of every card's Count; Kinds is len(Cards).
	// Summary ("21 items across 4 kinds") and SummaryShort ("21 items") are
	// those two numbers as the header reads them on desktop and on a phone.
	Items        int
	Kinds        int
	Summary      string
	SummaryShort string

	Constitution HomeConstitutionTile
	Modules      HomeModulesTile
}

// HomeCard is one "Waiting on you" card.
type HomeCard struct {
	// Kind is the card's state, used as its data-kind and colour:
	// edited, review, thread or draft.
	Kind  string
	Count int
	Label string
	// ShortLabel is the phone's label where the desktop one does not fit a
	// half-width card; it equals Label otherwise.
	ShortLabel string
	// Detail is the desktop description; Short is the phone one.
	Detail string
	Short  string
	Action string
	// Target is the claim id the action opens (the resolver's hash).
	Target string
}

// HomeConstitutionTile summarises constitution.yaml. Present is false when
// the project has no constitution file, which hides the tile.
//
// State is the roof gate's own verdict (constitution.Evaluate against the
// lock store's record), not the file's status line: a hand-flipped
// "status: locked" with no record, or a locked file edited since, is not
// shown as Locked. Label is the pill's word for it.
type HomeConstitutionTile struct {
	Present bool
	Locked  bool
	State   string
	// Label is the pill's word for State; ShortLabel is the phone row's,
	// where the pill shares a line with the counts; MeterLabel is the
	// Constitution page's meter's, lower case after its word count.
	Label         string
	ShortLabel    string
	MeterLabel    string
	ProjectClaims int
	// Stats are the invariants, decisions and glossary counts in that order,
	// each with its label already made singular or plural; Line is the same
	// three joined for the phone row.
	Stats []HomeStat
	Line  string
}

// HomeStat is one number on a tile and the word under it.
type HomeStat struct {
	Count int
	Label string
}

// HomeModulesTile summarises the module claims (project claims excluded,
// as the sidebar's Modules group excludes them).
type HomeModulesTile struct {
	Modules int
	Claims  int
	// Line is "214 claims · 202 locked · 12 draft", the draft part only
	// when there are drafts.
	Line   string
	Locked int
	Draft  int
	// Percent is Locked/Claims rounded down, 0..100.
	Percent int
	// FirstModuleID is where the tile leads.
	FirstModuleID string
}

const (
	homeCardNames    = 3
	homeDraftModules = 3
)

// homeBriefThread is a brief with open threads, for the Open threads card:
// its title and the page id its link opens.
type homeBriefThread struct {
	title, anchor string
	open          int
}

// homeBriefThreads lists the briefs with open threads, in brief-id order
// (set order), each with its page id from renderBriefs' anchors.
func homeBriefThreads(set *briefs.Set, rendered map[string]renderedBrief) []homeBriefThread {
	if set.Empty() {
		return nil
	}
	var out []homeBriefThread
	for _, b := range set.Briefs {
		if n := b.OpenThreads(); n > 0 {
			out = append(out, homeBriefThread{title: b.Title, anchor: rendered[b.ID].anchor, open: n})
		}
	}
	return out
}

func buildHomeView(cat *catalog.Catalog, cfg *config.Config, modules []ModuleGroup, briefThreads []homeBriefThread) HomeView {
	var view HomeView
	view.Modules = homeModulesTile(modules)
	view.Constitution = homeConstitutionTile(cat, cfg)
	if cat == nil {
		return view
	}

	claims := append([]model.Claim(nil), cat.Claims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID < claims[j].ID })

	var edited, review, threaded, drafts []model.Claim
	threads := 0
	for _, c := range claims {
		if _, ok := cat.ApprovedEdits[c.ID]; ok {
			edited = append(edited, c)
		}
		if c.Status == model.StatusLocked && dependencyChanged(cat.Readiness[c.ID]) {
			review = append(review, c)
		}
		if n := len(c.OpenThreadIDs()); n > 0 {
			threads += n
			threaded = append(threaded, c)
		}
		if c.Status == model.StatusDraft {
			drafts = append(drafts, c)
		}
	}

	if len(edited) > 0 {
		view.Cards = append(view.Cards, HomeCard{
			Kind:       "edited",
			Count:      len(edited),
			Label:      "Edited after approval",
			ShortLabel: "Edited after approval",
			Detail:     countNoun(len(edited), "claim") + ": " + claimNames(edited) + ".",
			Short:      countNoun(len(edited), "claim"),
			Action:     "See changes",
			Target:     edited[0].ID,
		})
	}
	if len(review) > 0 {
		view.Cards = append(view.Cards, HomeCard{
			Kind:       "review",
			Count:      len(review),
			Label:      "To re-read",
			ShortLabel: "To re-read",
			Detail:     reviewDetail(len(review)),
			Short:      countNoun(len(review), "claim"),
			Action:     "Review",
			Target:     review[0].ID,
		})
	}
	for _, b := range briefThreads {
		threads += b.open
	}
	if threads > 0 {
		label := "Open threads"
		if threads == 1 {
			label = "Open thread"
		}
		card := HomeCard{
			Kind:       "thread",
			Count:      threads,
			Label:      label,
			ShortLabel: label,
			Action:     "Open thread",
		}
		var names []string
		for _, c := range threaded {
			names = append(names, components.ClaimLabel(c.ID))
		}
		for _, b := range briefThreads {
			names = append(names, b.title)
		}
		switch {
		case len(briefThreads) == 0:
			card.Detail = "On " + joinNames(names) + ". A claim can't lock while a thread on it is open."
		case len(threaded) == 0:
			card.Detail = "On " + joinNames(names) + ". A brief can't lock while a thread on it is open."
		default:
			card.Detail = "On " + joinNames(names) + ". A claim or brief can't lock while a thread on it is open."
		}
		// The first claim's thread, as before briefs; a brief's page when
		// only briefs have one.
		if len(threaded) > 0 {
			card.Short, card.Target = components.ClaimLabel(threaded[0].ID), threaded[0].ID
		} else {
			card.Short, card.Target = briefThreads[0].title, briefThreads[0].anchor
		}
		view.Cards = append(view.Cards, card)
	}
	if len(drafts) > 0 {
		byModule, order := draftsByModule(drafts)
		view.Cards = append(view.Cards, HomeCard{
			Kind:       "draft",
			Count:      len(drafts),
			Label:      "Draft claims to lock",
			ShortLabel: "Drafts to lock",
			Detail:     draftModulesLine(byModule, order),
			Short:      "In " + countNoun(len(order), "module"),
			Action:     "Review drafts",
			Target:     drafts[0].ID,
		})
	}

	for _, c := range view.Cards {
		view.Items += c.Count
	}
	view.Kinds = len(view.Cards)
	view.Summary = countNoun(view.Items, "item") + " across " + countNoun(view.Kinds, "kind")
	view.SummaryShort = countNoun(view.Items, "item")
	return view
}

func reviewDetail(n int) string {
	if n == 1 {
		return "1 claim: something it rests on changed since approval."
	}
	return countNoun(n, "claim") + ": something they rest on changed since approval."
}

// dependencyChanged reports whether live readiness says a claim must be
// re-read because something it rests on changed: a direct dependency moved
// since its baseline, or the change reached it through an unchanged
// intermediate. It reads the assessment's own causes and never re-derives
// them. The other review causes are counted elsewhere (an unapproved edit is
// the edited card, an open thread the thread card) or are ledger integrity
// findings the status strip owns, so none of them is "to re-read".
func dependencyChanged(a readiness.Assessment) bool {
	for _, cause := range a.ReviewCauses {
		switch cause.Kind {
		case readiness.CauseDirectDependencyChange, readiness.CauseUpstreamDependencyReview:
			return true
		}
	}
	return false
}

func homeModulesTile(modules []ModuleGroup) HomeModulesTile {
	var t HomeModulesTile
	t.Modules = len(modules)
	for _, m := range modules {
		t.Claims += m.ClaimCount
		t.Locked += m.LockedCount
	}
	t.Draft = t.Claims - t.Locked
	if t.Claims > 0 {
		t.Percent = t.Locked * 100 / t.Claims
	}
	t.Line = fmt.Sprintf("%s · %d locked", countNoun(t.Claims, "claim"), t.Locked)
	if t.Draft > 0 {
		t.Line += fmt.Sprintf(" · %d draft", t.Draft)
	}
	if len(modules) > 0 {
		t.FirstModuleID = modules[0].ID
	}
	return t
}

func homeConstitutionTile(cat *catalog.Catalog, cfg *config.Config) HomeConstitutionTile {
	var t HomeConstitutionTile
	if cat != nil {
		for _, c := range cat.Claims {
			if c.IsProjectClaim() {
				t.ProjectClaims++
			}
		}
	}
	if cfg == nil {
		return t
	}
	var rec *constitution.LockRecord
	if store, err := lock.LoadStore(cfg.LockStorePath()); err == nil && store != nil {
		rec = store.Constitution
	}
	f, loadErr := constitution.Load(cfg.ConstitutionPath())
	v := constitution.Evaluate(cfg.ConstitutionPath(), f, loadErr, rec)
	switch v.State {
	case constitution.StateMissing:
		return t
	case constitution.StateLocked:
		t.Label = "Locked"
	case constitution.StateDraft:
		t.Label = "Draft"
	case constitution.StateEdited:
		t.Label, t.ShortLabel = "Edited since lock", "Edited"
	case constitution.StateUnrecorded:
		t.Label, t.ShortLabel = "Not locked", "Pending"
	default:
		t.Label = "Unreadable"
	}
	if t.ShortLabel == "" {
		t.ShortLabel = t.Label
	}
	t.MeterLabel = strings.ToLower(t.Label)
	t.Present = true
	t.State = string(v.State)
	t.Locked = v.State == constitution.StateLocked
	if f == nil {
		return t
	}
	d := constitution.NewDigest(cfg.ConstitutionPath(), f)
	t.Stats = []HomeStat{
		{d.Invariants, plural(d.Invariants, "invariant")},
		{d.Decisions, plural(d.Decisions, "decision")},
		{d.Glossary, plural(d.Glossary, "glossary term")},
	}
	// Non-breaking inside each count and before each separator, so a narrow
	// phone row wraps between counts and never starts a line with "·".
	nb := func(n int, noun string) string { return strings.ReplaceAll(countNoun(n, noun), " ", "\u00a0") }
	t.Line = nb(d.Invariants, "invariant") + "\u00a0· " + nb(d.Decisions, "decision") + "\u00a0· " + nb(d.Glossary, "term")
	return t
}

// claimNames lists the first homeCardNames claims by their readable label,
// then how many more there are.
func claimNames(claims []model.Claim) string {
	names := make([]string, 0, len(claims))
	for _, c := range claims {
		names = append(names, components.ClaimLabel(c.ID))
	}
	return joinNames(names)
}

// joinNames names at most homeCardNames of names, then a count of the rest.
func joinNames(names []string) string {
	shown := names
	if len(shown) > homeCardNames {
		shown = shown[:homeCardNames]
	}
	out := strings.Join(shown, ", ")
	if rest := len(names) - len(shown); rest > 0 {
		out += fmt.Sprintf(" and %d more", rest)
	}
	return out
}

// draftsByModule counts drafts per module label, returning the labels most
// drafts first (ties alphabetical). Project claims count under "Project".
func draftsByModule(drafts []model.Claim) (counts map[string]int, order []string) {
	counts = map[string]int{}
	for _, c := range drafts {
		label := "Project"
		if !c.IsProjectClaim() {
			label = displayCase(c.Module)
			if label == "" {
				label = displayCase(ungroupedModuleName)
			}
		}
		counts[label]++
	}
	order = make([]string, 0, len(counts))
	for label := range counts {
		order = append(order, label)
	}
	sort.Slice(order, func(i, j int) bool {
		if counts[order[i]] != counts[order[j]] {
			return counts[order[i]] > counts[order[j]]
		}
		return order[i] < order[j]
	})
	return counts, order
}

func draftModulesLine(counts map[string]int, order []string) string {
	var parts []string
	for i, label := range order {
		if i == homeDraftModules {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %d", label, counts[label]))
	}
	out := strings.Join(parts, ", ")
	if rest := len(order) - len(parts); rest > 0 {
		out += fmt.Sprintf(" and %d more %s", rest, plural(rest, "module"))
	}
	return out + "."
}

func countNoun(n int, noun string) string {
	return fmt.Sprintf("%d %s", n, plural(n, noun))
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}
