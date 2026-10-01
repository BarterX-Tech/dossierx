package render

import (
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// BriefsIndex is the "All briefs" page (NIT-203, Paper B6): every non-feature
// folder, each brief's title, summary and one lock/review pill, and the
// totals against the effective caps. Features stay off the groups (they
// have their own entry) while the total against the project cap still
// counts them.
type BriefsIndex struct {
	Total    int
	TotalCap int
	Features int
	// OverTotal is Total > TotalCap; check already refuses brief-total-cap.
	OverTotal bool
	// Headline is "11 of 60, 5 are features" (or "6 of 60" with none).
	Headline string
	// Locked, Review, Edited and Threads are the non-feature briefs' states,
	// from the same briefMark / Evaluation the sidebar reads. Threads is the
	// open-thread count, matching Home's Open threads card.
	Locked, Review, Edited, Threads int
	States                          string
	Folders                         []BriefsIndexFolder
}

// BriefsIndexFolder is one folder on the index.
type BriefsIndexFolder struct {
	Name, Label string
	Count, Cap  int
	// Pair is "N of 12" with the same grouping the headline uses.
	Pair string
	Over bool
	// Open is true when a brief in the folder is edited, review-pending or
	// carrying an open thread; a quiet folder starts collapsed.
	Open bool
	// Marks are the folder's distinct briefMark values, in legend order, for
	// the collapsed summary's state dots.
	Marks []string
	Pages []BriefPageView
}

// BriefsTile is Home's Briefs tile (Paper B0 / B6): folders other than
// features/, and the inclusive total against the project cap.
type BriefsTile struct {
	Count    int
	Headline string
	// FeaturesNote is "5 are features" when features/ holds any; empty otherwise.
	FeaturesNote string
	OverTotal    bool
	Line         string
	Folders      []BriefsTileFolder
}

// BriefsTileFolder is one folder row on the tile.
type BriefsTileFolder struct {
	Label string
	Count string
	Over  bool
}

// BriefsTile summarises the non-feature folders for Home. Zero Count hides
// the tile, as FeaturesTile hides when features/ is empty.
func (v BriefsView) BriefsTile() BriefsTile {
	idx := v.Index
	var t BriefsTile
	t.Count = idx.Total - idx.Features
	if t.Count <= 0 {
		return t
	}
	t.Headline = capPair(idx.Total, idx.TotalCap)
	t.OverTotal = idx.OverTotal
	if idx.Features > 0 {
		t.FeaturesNote = featuresAre(idx.Features)
	}
	for _, f := range idx.Folders {
		t.Folders = append(t.Folders, BriefsTileFolder{
			Label: f.Label,
			Count: capPair(f.Count, f.Cap),
			Over:  f.Over,
		})
	}
	parts := []string{strconv.Itoa(t.Count)}
	for _, pair := range []struct {
		n    int
		word string
	}{
		{idx.Locked, "locked"},
		{idx.Edited, "edited"},
		{idx.Review, "review"},
		{idx.Threads, "open threads"},
	} {
		if pair.n > 0 {
			if pair.word == "open threads" && pair.n == 1 {
				parts = append(parts, "1 open thread")
				continue
			}
			parts = append(parts, strconv.Itoa(pair.n)+" "+pair.word)
		}
	}
	t.Line = strings.Join(parts, " · ")
	return t
}

func capPair(n, cap int) string {
	return groupDigits(n) + " of " + groupDigits(cap)
}

func featuresAre(n int) string {
	if n == 1 {
		return "1 is a feature"
	}
	return strconv.Itoa(n) + " are features"
}

func briefsCapHeadline(total, cap, features int) string {
	h := capPair(total, cap)
	if features > 0 {
		return h + ", " + featuresAre(features)
	}
	return h
}

func briefsStatesLine(locked, review, edited, threads int) string {
	var parts []string
	if locked > 0 {
		parts = append(parts, strconv.Itoa(locked)+" locked")
	}
	if review > 0 {
		parts = append(parts, strconv.Itoa(review)+" review pending")
	}
	if edited > 0 {
		parts = append(parts, strconv.Itoa(edited)+" edited since approval")
	}
	if threads > 0 {
		parts = append(parts, openThreadsLabel(threads))
	}
	return strings.Join(parts, " · ")
}

// markOrder is the sidebar legend's order, used for a collapsed folder's dots.
var markOrder = []string{"edited", "review", "thread", "draft", "locked"}

func folderMarks(pages []BriefPageView) []string {
	seen := map[string]bool{}
	for _, p := range pages {
		if p.Mark != "" {
			seen[p.Mark] = true
		}
	}
	var out []string
	for _, m := range markOrder {
		if seen[m] {
			out = append(out, m)
		}
	}
	return out
}

func folderIsOpen(pages []BriefPageView) bool {
	for _, p := range pages {
		if p.Mark == "edited" || p.Mark == "review" || p.Mark == "thread" {
			return true
		}
	}
	return false
}

func buildBriefsIndex(set *briefs.Set, folders []BriefFolderView, features int) BriefsIndex {
	caps := config.BriefCaps{Total: config.DefaultMaxBriefs, PerFolder: config.DefaultMaxBriefsPerFolder}
	total := features
	if set != nil {
		caps = set.Caps
		total = len(set.Briefs)
	}
	idx := BriefsIndex{
		Total:     total,
		TotalCap:  caps.Total,
		Features:  features,
		OverTotal: total > caps.Total,
		Headline:  briefsCapHeadline(total, caps.Total, features),
	}
	for _, f := range folders {
		folder := BriefsIndexFolder{
			Name:  f.Name,
			Label: f.Label,
			Count: f.Count,
			Cap:   caps.PerFolder,
			Pair:  capPair(f.Count, caps.PerFolder),
			Over:  f.Count > caps.PerFolder,
			Open:  folderIsOpen(f.Pages),
			Marks: folderMarks(f.Pages),
			Pages: f.Pages,
		}
		idx.Folders = append(idx.Folders, folder)
		for _, p := range f.Pages {
			switch p.Mark {
			case "edited":
				idx.Edited++
			case "review":
				idx.Review++
			case "locked":
				idx.Locked++
			}
			idx.Threads += p.OpenThreads
		}
	}
	idx.States = briefsStatesLine(idx.Locked, idx.Review, idx.Edited, idx.Threads)
	return idx
}
