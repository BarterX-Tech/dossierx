package render

import (
	"html"
	"html/template"
	"path"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// A feature (NIT-201, Paper board B4) is an ordinary brief in
// briefs/features/. It gets the brief page (NIT-197) and, in place of the
// Rests on list, a "Made of" list: the claims its rests_on names, grouped by
// the module each lives in. The sidebar lists features under their own
// Features entry, not in the Briefs tree, and Home carries a Features tile.
//
// A feature shows the brief's own state and nothing else: no built,
// specified or conformance state is read or drawn (decided 29 Sep). Its row
// mark and tile word are the brief's lock and review state (NIT-205's
// Evaluation, the payload's lock_state, review_pending and open_threads;
// briefMark), in the slot a brief row's mark takes. The page's pill and
// the lock date stay the brief page's (NIT-197, NIT-199).

// FeatureDetail is what a feature page adds to a brief page.
type FeatureDetail struct {
	// Groups are the Made of list: one per module in the order its first
	// claim appears in rests_on, rows in rests_on order within it. An id
	// that names no claim is in a last group of its own.
	Groups []MadeOfGroup
	// Count is len(rests_on); Locked is how many of them name a locked
	// claim; Modules is how many modules they fall in.
	Count   int
	Locked  int
	Modules int
	// State is the word Home's tile shows for the feature: locked, draft,
	// or, from the brief's review state (NIT-205), edited or review.
	State string
}

// MadeOfGroup is one module's rows in a feature's Made of list.
type MadeOfGroup struct {
	Label string
	// Rows are relationship rows (components.BriefRelationRowsHTML), the
	// writer NIT-197's Rests on list uses.
	Rows template.HTML
}

// madeOfUnknownLabel heads the rows whose id names no claim. check reports
// each as brief-rests-on-unknown; the page shows the id as written.
const madeOfUnknownLabel = "Not a claim"

// maxMetaModules is how many module names the meta line spells out before
// it counts the rest, so a feature resting on many modules keeps a
// one-line meta.
const maxMetaModules = 3

// Label is the Made of head on a wide screen: "Made of · 6 claims in 3
// modules".
func (f FeatureDetail) Label() string {
	s := "Made of · " + claimCount(f.Count)
	if f.Modules > 0 {
		s += " in " + countNoun(f.Modules, "module")
	}
	return s
}

// ShortLabel is the Made of head on a phone and the "On this page" row:
// "Made of · 6 claims".
func (f FeatureDetail) ShortLabel() string { return "Made of · " + claimCount(f.Count) }

// lockedPhrase is "all locked" only when every rests_on id names a locked
// claim, and "M of N locked" otherwise.
func (f FeatureDetail) lockedPhrase() string {
	if f.Count > 0 && f.Locked == f.Count {
		return "all locked"
	}
	return strconv.Itoa(f.Locked) + " of " + strconv.Itoa(f.Count) + " locked"
}

// madeOfClaim is what the Made of list reads of one claim.
type madeOfClaim struct {
	module string
	locked bool
}

// madeOfIndex maps each claim id to its module label and whether it is
// locked. A project claim's module is "Project", as the Home draft card
// names it.
func madeOfIndex(cat *catalog.Catalog) map[string]madeOfClaim {
	out := map[string]madeOfClaim{}
	if cat == nil {
		return out
	}
	for _, c := range cat.Claims {
		label := "Project"
		if !c.IsProjectClaim() {
			label = components.DisplayCase(c.Module)
			if label == "" {
				label = components.DisplayCase(ungroupedModuleName)
			}
		}
		out[c.ID] = madeOfClaim{module: label, locked: c.Status == model.StatusLocked}
	}
	return out
}

// buildFeatureDetail groups rests_on by module. O(R) for R rests_on ids.
func buildFeatureDetail(restsOn []string, index map[string]madeOfClaim, statuses map[string]components.TargetStatus) *FeatureDetail {
	f := &FeatureDetail{Count: len(restsOn)}
	byLabel := map[string][]string{}
	var order []string
	var unknown []string
	for _, id := range restsOn {
		c, ok := index[id]
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		if c.locked {
			f.Locked++
		}
		if _, seen := byLabel[c.module]; !seen {
			order = append(order, c.module)
		}
		byLabel[c.module] = append(byLabel[c.module], id)
	}
	f.Modules = len(order)
	for _, label := range order {
		f.Groups = append(f.Groups, MadeOfGroup{Label: label, Rows: components.BriefRelationRowsHTML(byLabel[label], statuses)})
	}
	if len(unknown) > 0 {
		f.Groups = append(f.Groups, MadeOfGroup{Label: madeOfUnknownLabel, Rows: components.BriefRelationRowsHTML(unknown, statuses)})
	}
	return f
}

// featureMeta is the wide meta line: "Rests on 6 claims in Expenses,
// Splitting and Settlements, all locked", then the words and images the brief
// page already counts. The lock date the board leads with ("Locked 26 Sep
// 2026 · rests on …") is the brief lock record's (NIT-199) and is left out
// until it exists, as the brief page leaves it out; the line starts at
// "Rests on" instead.
func featureMeta(f *FeatureDetail, modules []string, tail string) string {
	s := "Rests on " + claimCount(f.Count)
	if len(modules) > 0 {
		s += " in " + joinModules(modules)
	}
	if f.Count > 0 {
		s += ", " + f.lockedPhrase()
	}
	return s + " · " + tail
}

// featureMetaShort is the phone's: "6 claims, all locked".
func featureMetaShort(f *FeatureDetail) string {
	if f.Count == 0 {
		return "No claims yet"
	}
	return claimCount(f.Count) + ", " + f.lockedPhrase()
}

// joinModules spells module names as a sentence: "A", "A and B", "A, B and
// C", and past maxMetaModules "A, B, C and 2 more modules".
func joinModules(names []string) string {
	if len(names) > maxMetaModules {
		rest := len(names) - maxMetaModules
		return strings.Join(names[:maxMetaModules], ", ") + " and " + countNoun(rest, "more module")
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// FeaturesTile is Home's Features tile (Paper B0): each feature and its
// state, and on a phone one line counting them.
type FeaturesTile struct {
	Count   int
	FirstID string
	Rows    []FeatureTileRow
	// Line is the phone row's "5 · 3 locked · 2 draft".
	Line string
}

// FeatureTileRow is one feature on the tile.
type FeatureTileRow struct {
	ID    string
	Title string
	// State is the word the row shows: FeatureDetail.State.
	State string
}

// FeaturesTile summarises the features for Home. Zero Count hides the tile.
func (v BriefsView) FeaturesTile() FeaturesTile {
	var t FeaturesTile
	counts := map[string]int{}
	for _, p := range v.Features {
		state := p.Status
		if p.Feature != nil && p.Feature.State != "" {
			state = p.Feature.State
		}
		t.Rows = append(t.Rows, FeatureTileRow{ID: p.ID, Title: p.NavLabel, State: state})
		counts[state]++
	}
	t.Count = len(t.Rows)
	if t.Count == 0 {
		return t
	}
	t.FirstID = t.Rows[0].ID
	parts := []string{strconv.Itoa(t.Count)}
	for _, state := range []string{"locked", "edited", "review", "draft"} {
		if n := counts[state]; n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+state)
		}
	}
	t.Line = strings.Join(parts, " · ")
	return t
}

// briefLinkTargets maps every brief's path to its page id, the anchor the
// payload carries for it.
func briefLinkTargets(set *briefs.Set, rendered map[string]renderedBrief) map[string]string {
	out := make(map[string]string, len(set.Briefs))
	for _, b := range set.Briefs {
		out[b.Path] = rendered[b.ID].anchor
	}
	return out
}

// resolveBriefLinks points each link in a rendered brief body at the brief
// page it names. A brief links to another the way markdown does, by path:
// relative to its own file ("../voice/talking-about-money.md",
// "sibling.md") or from the project root, as check names a brief
// ("briefs/voice/talking-about-money.md", with or without a leading "/").
// Either form that names a brief in this project becomes "#<anchor>", the
// page's id, so the viewer's own hash routing opens it; any other href is
// left as written. A "#fragment" or "?query" after the path is dropped: a
// brief page has one address.
//
// The renderer writes a link as <a href="..."> with the url HTML-escaped and
// escapes every "<" of author text (markdown.go), so `<a href="` occurs in its
// output only at a link it wrote. The replacement is an id drawn from
// [a-z0-9-_], which needs no escaping. O(body bytes).
func resolveBriefLinks(body, briefPath string, targets map[string]string) string {
	const open = `<a href="`
	if !strings.Contains(body, open) {
		return body
	}
	dir := path.Dir(briefPath)
	var b strings.Builder
	b.Grow(len(body))
	rest := body
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		start := i + len(open)
		end := strings.IndexByte(rest[start:], '"')
		if end < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:start])
		href := rest[start : start+end]
		if anchor, ok := briefLinkAnchor(html.UnescapeString(href), dir, targets); ok {
			b.WriteString("#" + anchor)
		} else {
			b.WriteString(href)
		}
		rest = rest[start+end:]
	}
}

func briefLinkAnchor(href, dir string, targets map[string]string) (string, bool) {
	if cut := strings.IndexAny(href, "#?"); cut >= 0 {
		href = href[:cut]
	}
	if href == "" || !strings.HasSuffix(href, ".md") || strings.Contains(href, ":") {
		return "", false
	}
	if !strings.HasPrefix(href, "/") {
		if anchor, ok := targets[path.Join(dir, href)]; ok {
			return anchor, true
		}
	}
	anchor, ok := targets[path.Clean(strings.TrimPrefix(href, "/"))]
	return anchor, ok
}
