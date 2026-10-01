package render

import (
	"html/template"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/approvaledit"
	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// brief_edit.go is a locked brief whose file moved since its approval
// (NIT-199, Paper B2): the three views of the page — Changes, Approved and
// Current — rendered here, into the page's own section, so a live reload's
// fragment swap carries them exactly as it carries the body.
//
// NOTHING HERE DECIDES A STATE. Whether a brief is edited since approval is
// briefs.Evaluate's answer (LockEdited, the same predicate that raises
// brief-content-drift), and the approved text is the record's own
// (Review.Approved). This file only draws the difference between two strings
// it is handed, with the one diff the claim viewer already uses
// (approvaledit.Passages: textdiff's passages, the words inside a replaced
// pair, each passage through the escaping markdown renderer).

// BriefDiff is the passage-by-passage difference between two brief bodies,
// rendered in document mode with images on the brief's asset path and the
// headings placed under a brief page's title (briefBodyOutline). It is the
// renderer NIT-199's Changes and Current views are drawn from, and the one a
// brief's per-claim redline (NIT-200) reuses: RenderBriefDiff with the two
// wordings, then ChangesHTML or CurrentHTML.
type BriefDiff struct {
	// Hunks are approvaledit.Passages' output: "equal", "remove" or "add",
	// each rendered, the moved words marked.
	Hunks []approvaledit.Hunk
	// Changed counts the passages that differ, a passage and its
	// replacement once: a run of removed passages and the run of added ones
	// that replaces it count as the longer of the two, and a run with nothing
	// facing it counts each passage. A reworded paragraph is one, five
	// reworded paragraphs in a row are five, an added heading and its
	// paragraph are two.
	//
	// It is not approvaledit.Passages' own count, which pairs a removal only
	// with the addition directly after it: textdiff emits a block of rewrites
	// as every removal, then every addition, so five reworded paragraphs read
	// nine there. That count feeds the claim viewer and is left as it is.
	Changed int
}

// RenderBriefDiff diffs before against after, both brief markdown bodies,
// rendering each passage as a brief body renders (markdown.RenderDocument
// with assets as the image prefix; "" renders no image). A body that opens
// with its title heading (markdown.DocumentTitle) loses that heading when it
// is unchanged, as the brief page's own body does (withoutTitleHeading): the
// page header already shows the title.
//
// Cost is approvaledit.Passages': textdiff's bounded LCS over passages
// (textdiff.MaxBlocks a side, else one whole-body replacement) and over the
// words of each replaced pair (textdiff.MaxMarkWords), plus one or two
// renders of each passage.
func RenderBriefDiff(before, after string, assets markdown.AssetPrefix) BriefDiff {
	render := func(src string) string { return string(markdown.RenderDocument(src, assets)) }
	hunks, _ := approvaledit.Passages(endPassage(before), endPassage(after), render)
	if len(hunks) > 0 && hunks[0].Op == "equal" {
		if _, ok := markdown.DocumentTitle(after); ok {
			hunks[0].HTML = withoutTitleHeading(hunks[0].HTML, after)
			if strings.TrimSpace(hunks[0].HTML) == "" {
				hunks = hunks[1:]
			}
		}
	}
	d := BriefDiff{Hunks: hunks}
	runs := d.runs()
	for i := 0; i < len(runs); i++ {
		switch {
		case runs[i].op == "remove" && i+1 < len(runs) && runs[i+1].op == "add":
			d.Changed += max(len(runs[i].hunks), len(runs[i+1].hunks))
			i++
		case runs[i].op != "equal":
			d.Changed += len(runs[i].hunks)
		}
	}
	return d
}

// endPassage ends a body with exactly one blank line. textdiff keeps a
// passage's trailing blank lines on it (so a side's hunks concatenate back to
// the side), which makes a body's LAST passage differ from itself the moment
// anything is appended after it: "text\n" and "text\n\n" are two passages.
// Appending a section is the commonest edit a brief gets, and without this
// every such edit also redlines the unchanged passage above it. A markdown
// body renders the same whatever trails its last line, so nothing a reader
// sees depends on it.
func endPassage(body string) string {
	trimmed := strings.TrimRight(body, " \t\r\n")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n\n"
}

// ChangesHTML is the redline (Paper B2, Changes): each unchanged passage as
// the body draws it, and each changed run as the approved passages, tinted
// and struck, above the current ones — the claim viewer's own passage classes
// (.claim-edit-passage--removed / --added), so the two read as one system.
// Consecutive passages with one op share one block, so a section added whole
// (its heading and its paragraphs) is one ruled block, as the board draws a
// changed paragraph. Every passage is engine-rendered markdown; the
// aria-label names the words that moved, escaped.
func (d BriefDiff) ChangesHTML() template.HTML {
	var b strings.Builder
	for _, run := range d.runs() {
		if run.op == "equal" {
			for _, h := range run.hunks {
				b.WriteString(h.HTML)
			}
			continue
		}
		cls, lead := "claim-edit-passage claim-edit-passage--removed", "Removed since approval"
		if run.op == "add" {
			cls, lead = "claim-edit-passage claim-edit-passage--added", "Added since approval"
		}
		var words []string
		for _, h := range run.hunks {
			words = append(words, h.Changed...)
		}
		b.WriteString(`<div class="` + cls + `"`)
		if len(words) > 0 {
			b.WriteString(` aria-label="` + template.HTMLEscapeString(lead+": "+strings.Join(words, ", ")) + `"`)
		}
		b.WriteString(`>`)
		for _, h := range run.hunks {
			b.WriteString(h.HTML)
		}
		b.WriteString(`</div>`)
	}
	return template.HTML(briefBodyOutline(b.String()))
}

// CurrentHTML is the file as it reads now (Paper B2, Current): the removed
// passages left out, and each run of passages that differs from the approval
// ruled at its left edge with a caption under it — "changed since approval"
// for a run that replaced approved passages, "added since approval" for one
// with nothing facing it.
func (d BriefDiff) CurrentHTML() template.HTML {
	var b strings.Builder
	runs := d.runs()
	for i, run := range runs {
		switch run.op {
		case "remove":
			continue
		case "add":
			caption := "added since approval"
			if i > 0 && runs[i-1].op == "remove" {
				caption = "changed since approval"
			}
			b.WriteString(`<div class="claim-edit-passage claim-edit-passage--current brief-passage--current">`)
			for _, h := range run.hunks {
				b.WriteString(h.HTML)
			}
			b.WriteString(`<p class="brief-passage__caption">` + caption + `</p></div>`)
		default:
			for _, h := range run.hunks {
				b.WriteString(h.HTML)
			}
		}
	}
	return template.HTML(briefBodyOutline(b.String()))
}

// passageRun is consecutive hunks with one op.
type passageRun struct {
	op    string
	hunks []approvaledit.Hunk
}

func (d BriefDiff) runs() []passageRun {
	var out []passageRun
	for _, h := range d.Hunks {
		if n := len(out); n > 0 && out[n-1].op == h.Op {
			out[n-1].hunks = append(out[n-1].hunks, h)
			continue
		}
		out = append(out, passageRun{op: h.Op, hunks: []approvaledit.Hunk{h}})
	}
	return out
}

// BriefEditView is what a brief edited since its approval draws: the banner,
// the three views, and — when the record holds no approved text — the note
// that stands in for them.
type BriefEditView struct {
	// Retained is false when the record carries no approved markdown to
	// compare against; the page then shows the recover note above the
	// current body and offers no switch.
	Retained bool
	// Passages is how many passages of the body differ (BriefDiff.Changed).
	Passages int
	Changes  template.HTML
	Approved template.HTML
	Current  template.HTML
	// SummaryWas is the approved summary when the summary moved; the lede
	// shows the current one.
	SummaryWas string
	// RestsOnAdded and RestsOnRemoved are the rests_on ids the file lists
	// now and the approval did not, and the reverse.
	RestsOnAdded   []string
	RestsOnRemoved []string
	// ImagesChanged, ImagesAdded and ImagesRemoved compare the record's image
	// digests with the brief's own (the record signs each image's sha256
	// beside the markdown): an image whose bytes changed, one referenced now
	// and not at the approval, and one referenced then and not now (or gone).
	ImagesChanged []string
	ImagesAdded   []string
	ImagesRemoved []string
	// WhitespaceOnly is true when the signed text moved (its lock hash no
	// longer matches the approved text's) and yet no passage, summary or
	// rests_on id reads differently: what changed is whitespace the diff
	// does not show, trailing blank lines above all.
	WhitespaceOnly bool
	// HasImages is true when either wording references an image. The record
	// keeps an image's digest, not its bytes, so every view draws the image
	// as it is now, and the page says so.
	HasImages bool
}

// Headline is the banner's count: what differs, in the reader's words.
func (e BriefEditView) Headline() string {
	images := len(e.ImagesChanged) + len(e.ImagesAdded) + len(e.ImagesRemoved)
	switch {
	case !e.Retained:
		return ""
	case e.Passages > 0:
		return countNoun(e.Passages, "passage") + " changed"
	case e.SummaryWas != "":
		return "the summary changed"
	case len(e.RestsOnAdded) > 0 || len(e.RestsOnRemoved) > 0:
		return "rests on changed"
	case images == 1:
		return "an image changed"
	case images > 1:
		return countNoun(images, "image") + " changed"
	case e.WhitespaceOnly:
		return "only whitespace changed"
	default:
		return ""
	}
}

// HeadlineSentence is Headline as the phone banner's opening sentence,
// capitalised and stopped ("2 passages changed. "), or "" when there is none.
func (e BriefEditView) HeadlineSentence() string {
	h := e.Headline()
	if h == "" {
		return ""
	}
	return strings.ToUpper(h[:1]) + h[1:] + ". "
}

// HasOther reports whether anything the redline cannot show moved, which
// the Changes view lists under it.
func (e BriefEditView) HasOther() bool {
	return e.SummaryWas != "" || len(e.RestsOnAdded) > 0 || len(e.RestsOnRemoved) > 0 ||
		len(e.ImagesChanged) > 0 || len(e.ImagesAdded) > 0 || len(e.ImagesRemoved) > 0 || e.WhitespaceOnly
}

// NoPassageDiffers is true when the redline itself is empty, so the list
// under it is everything that moved.
func (e BriefEditView) NoPassageDiffers() bool { return e.Passages == 0 }

// briefEditView builds the edited brief's views from its review, or returns
// nil for any brief that is not edited since approval.
func briefEditView(b briefs.Brief, r briefs.Review) *BriefEditView {
	if r.LockState != briefs.LockEdited {
		return nil
	}
	a := r.Approved
	if a == nil || a.Markdown == "" {
		// A standing record with no approved markdown: nothing to compare
		// against. Every record `brief lock` writes carries it, so this is a
		// store written some other way; the page says so instead of drawing
		// a diff against an empty approval, which would mark every passage
		// as added.
		return &BriefEditView{}
	}
	assets := BriefAssetURLPrefix(b.Folder)
	diff := RenderBriefDiff(a.Markdown, b.Body, assets)
	images := imageStates(b)
	v := &BriefEditView{
		Retained: true,
		Passages: diff.Changed,
		Changes:  template.HTML(drawAbsentImages(string(diff.ChangesHTML()), b.Folder, images)),
		Current:  template.HTML(drawAbsentImages(string(diff.CurrentHTML()), b.Folder, images)),
		Approved: template.HTML(drawAbsentImages(briefBodyOutline(withoutTitleHeading(string(markdown.RenderDocument(a.Markdown, assets)), a.Markdown)), b.Folder, images)),
	}
	if a.Summary != b.Summary {
		v.SummaryWas = a.Summary
	}
	v.RestsOnAdded, v.RestsOnRemoved = idsDiff(a.RestsOn, b.RestsOn)
	v.ImagesChanged, v.ImagesAdded, v.ImagesRemoved = imagesDiff(r.ApprovedImages, b.ImageDigests())
	for _, view := range []template.HTML{v.Changes, v.Approved, v.Current} {
		if strings.Contains(string(view), `<img class="md-img"`) || strings.Contains(string(view), `class="brief-img-absent"`) {
			v.HasImages = true
		}
	}
	textMoved := briefs.LockHash(a.Summary, a.RestsOn, a.Markdown) != b.LockHash
	v.WhitespaceOnly = textMoved && diff.Changed == 0 && v.SummaryWas == "" && len(v.RestsOnAdded) == 0 && len(v.RestsOnRemoved) == 0
	return v
}

// imagesDiff compares the record's image digests with the brief's own, by
// file name: changed bytes, newly referenced, no longer referenced (or gone).
func imagesDiff(was, now map[string]string) (changed, added, removed []string) {
	for name, digest := range now {
		if old, ok := was[name]; !ok {
			added = append(added, name)
		} else if old != digest {
			changed = append(changed, name)
		}
	}
	for name := range was {
		if _, ok := now[name]; !ok {
			removed = append(removed, name)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	sort.Strings(removed)
	return changed, added, removed
}

// imageStates is every image name the brief references now: true when its
// folder holds the file, false when it does not.
func imageStates(b briefs.Brief) map[string]bool {
	out := make(map[string]bool, len(b.Images))
	for _, img := range b.Images {
		out[img.Name] = img.Present
	}
	return out
}

// drawAbsentImages replaces each rendered image that the page cannot show —
// one the brief no longer references (so neither build copies it nor serve
// answers it) or one its folder does not hold — with a line naming it,
// instead of a broken image. The renderer writes every brief image as exactly
// <img class="md-img" src="brief-assets/<folder>/<name>" alt="...">, with the
// name drawn from [a-z0-9.-] and the alt text escaped, so the match is on the
// renderer's own output and nothing an author wrote.
func drawAbsentImages(html, folder string, referenced map[string]bool) string {
	re := regexp.MustCompile(`<img class="md-img" src="` + regexp.QuoteMeta(string(BriefAssetURLPrefix(folder))) + `([^"/]+)" alt="[^"]*">`)
	return re.ReplaceAllStringFunc(html, func(tag string) string {
		name := re.FindStringSubmatch(tag)[1]
		present, ok := referenced[name]
		switch {
		case ok && present:
			return tag
		case ok:
			return `<span class="brief-img-absent">image missing: ` + template.HTMLEscapeString(name) + `</span>`
		default:
			return `<span class="brief-img-absent">image removed: ` + template.HTMLEscapeString(name) + `</span>`
		}
	})
}

// idsDiff returns the ids in now and not in was, and the reverse, sorted.
func idsDiff(was, now []string) (added, removed []string) {
	in := func(list []string) map[string]bool {
		m := make(map[string]bool, len(list))
		for _, id := range list {
			m[id] = true
		}
		return m
	}
	w, n := in(was), in(now)
	for id := range n {
		if !w[id] {
			added = append(added, id)
		}
	}
	for id := range w {
		if !n[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// BriefApprovalView is the standing approval a locked or edited brief's
// header names: when, and on whose words.
type BriefApprovalView struct {
	// At is the record's RFC 3339 time, the <time> element's datetime; the
	// viewer rewrites the label into the reader's own time zone.
	At string
	// Long and Short are the date in UTC ("18 Sep 2026", "18 Sep"): what a
	// page with no script, and every golden, reads.
	Long   string
	Short  string
	Reason string
}

func briefApproval(r briefs.Review) *BriefApprovalView {
	if r.LockedAt == "" || (r.LockState != briefs.LockLocked && r.LockState != briefs.LockEdited) {
		return nil
	}
	v := &BriefApprovalView{At: r.LockedAt, Long: r.LockedAt, Short: r.LockedAt, Reason: r.LockReason}
	if t, err := time.Parse(time.RFC3339Nano, r.LockedAt); err == nil {
		t = t.UTC()
		v.Long, v.Short = t.Format("2 Jan 2006"), t.Format("2 Jan")
	}
	return v
}
