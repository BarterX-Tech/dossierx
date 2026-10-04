package render

import (
	"errors"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
	"github.com/BarterX-Tech/dossierx/internal/visibility"
)

// BriefAssetDir is the directory, beside the viewer's index.html, a brief's
// images are served from: brief-assets/<folder>/<name>. The page names them by
// that RELATIVE path, so the one URL works in both places a viewer is read:
// `dossierx check` copies each referenced image there next to the static
// build (check.writeBriefAssets), and `dossierx serve` answers the same path
// from the brief's own folder (serve's brief-asset route). A folder and an
// image name are both drawn from [a-z0-9-] (internal/briefs), so the path needs
// no encoding at either end.
const BriefAssetDir = "brief-assets"

// BriefAssetURLPrefix is the URL prefix of every image in folder.
func BriefAssetURLPrefix(folder string) markdown.AssetPrefix {
	return markdown.AssetPrefix(path.Join(BriefAssetDir, folder) + "/")
}

// BriefAsset is one image the viewer references: where it is on disk, where
// it goes beside the viewer, and its size as discovery read it.
type BriefAsset struct {
	// Src is the image's path on disk, in its brief folder.
	Src string
	// Rel is its slash-separated path relative to the viewer's directory,
	// brief-assets/<folder>/<name> — exactly the src the page carries.
	Rel   string
	Bytes int64
	// Display is the image's path as every brief finding names a file:
	// relative to the config directory, "briefs/<folder>/<name>".
	Display string
}

// BriefAssets lists every image a brief in set references and its folder
// holds, once each, sorted by Rel. It is the list a static build copies and
// the bytes RenderBoundedWith charges against the viewer's budget, so the
// two can never disagree about which files the page needs. A reference to a
// file the folder does not hold is absent: the page shows it as a broken
// image, and check reports it (brief-shape).
func BriefAssets(cfg *config.Config, set *briefs.Set) []BriefAsset {
	if set.Empty() {
		return nil
	}
	root := cfg.BriefsDirPath()
	seen := map[string]bool{}
	var out []BriefAsset
	for _, b := range set.Briefs {
		for _, img := range b.Images {
			if !img.Present {
				continue
			}
			rel := path.Join(BriefAssetDir, b.Folder, img.Name)
			if seen[rel] {
				continue
			}
			seen[rel] = true
			out = append(out, BriefAsset{
				Src:     filepath.Join(root, b.Folder, img.Name),
				Rel:     rel,
				Bytes:   img.Bytes,
				Display: path.Join(set.DisplayDir, b.Folder, img.Name),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out
}

// ErrBriefImagesOverBound is the refusal of a bounded render in which brief
// images take part of the viewer's bound and the viewer does not fit. It is
// also conformance.ErrCapacityExceeded (errors.Is holds for both), so every
// caller that already branches on the capacity refusal keeps doing so; this
// one tells it the recovery is the images. The default brief caps allow more
// image bytes (60 briefs x 3 x 1 MiB) than the 64 MiB viewer bound holds, so
// a project inside every cap can reach it.
var ErrBriefImagesOverBound = errors.New("brief images over the viewer bound")

// briefImagesNamed is how many images the refusal names, largest first.
const briefImagesNamed = 5

type briefImagesBoundError struct {
	images []BriefAsset
	total  int64
	bound  int
	// alone is true when the images by themselves reach the bound, before a
	// byte of the page is counted.
	alone bool
}

func briefImagesOverBound(images []BriefAsset, total int64, bound int, alone bool) error {
	return &briefImagesBoundError{images: images, total: total, bound: bound, alone: alone}
}

func (e *briefImagesBoundError) Error() string {
	largest := append([]BriefAsset(nil), e.images...)
	sort.SliceStable(largest, func(i, j int) bool { return largest[i].Bytes > largest[j].Bytes })
	var names []string
	for i, a := range largest {
		if i == briefImagesNamed {
			names = append(names, fmt.Sprintf("and %d more", len(largest)-briefImagesNamed))
			break
		}
		names = append(names, fmt.Sprintf("%s %d bytes", a.Display, a.Bytes))
	}
	where := "the viewer does not fit"
	if e.alone {
		where = "the images alone reach it before the page is counted"
	}
	return fmt.Sprintf("%s: the static viewer is held to %d bytes, and its %d brief image(s) take %d of them, so %s (%s); shrink or remove brief images",
		conformance.ErrCapacityExceeded.Error(), e.bound, len(e.images), e.total, where, strings.Join(names, ", "))
}

func (e *briefImagesBoundError) Is(target error) bool {
	return target == ErrBriefImagesOverBound || target == conformance.ErrCapacityExceeded
}

// featuresFolder is the brief folder the Briefs tree leaves out: its briefs
// are features, listed under the sidebar's own Features entry and given the
// feature page (NIT-201, feature_page.go).
const featuresFolder = "features"

// briefsIndexID is the id of the "All briefs" page. Like Home's "_home" it
// carries an underscore, which slugify never emits, so no module can share it.
const briefsIndexID = "_briefs"

// BriefsView is the Briefs half of the viewer (NIT-197): the sidebar tree,
// one page per brief, and the "All briefs" index they share.
type BriefsView struct {
	// Folders are the tree's folders in name order, features/ excluded.
	Folders []BriefFolderView
	// Features are the pages under features/, ordered by file name without
	// ".md" (the brief id's order, so export sorts before export-to-csv): the
	// Features entry's rows and pages (NIT-201), never in the tree.
	Features []BriefPageView
	// Total is how many briefs the tree lists.
	Total int
	// IndexID is the "All briefs" page's section id.
	IndexID string
	// Index is the B6 "All briefs" page: folders, caps, and the inclusive
	// total. Empty when there is no tree to index.
	Index BriefsIndex
}

// Present reports whether the project holds any brief at all, which is what
// every brief element in the shell is conditional on: a project with none
// renders byte-identically to one rendered before briefs existed.
func (v BriefsView) Present() bool { return len(v.Folders) > 0 || len(v.Features) > 0 }

// BriefFolderView is one folder of the tree.
type BriefFolderView struct {
	Name  string
	Label string
	Count int
	Pages []BriefPageView
}

// BriefPageView is one brief's page.
type BriefPageView struct {
	// ID is the section id and the hash that opens the page:
	// brief-<folder>-<slug>, made unique on the page (briefAnchors).
	ID          string
	Path        string
	Folder      string
	FolderLabel string
	Title       string
	// NavLabel is the brief's row in the sidebar tree: its own title, the
	// words its page heading and the "All briefs" index show (NIT-249). Until
	// 0.7.23 it was the file name in sentence case, which turned
	// "macos-app-lifecycle" into "Macos app lifecycle" beside a page titled
	// "macOS app lifecycle". A long title wraps rather than truncating.
	NavLabel string
	Summary  string
	Status   string
	// BriefID is the brief's <folder>.<slug> id, which the comment rail and
	// the /api/briefs/{id}/comments routes address it by (NIT-198).
	BriefID string
	// LockState is the brief's state against its lock record (NIT-205):
	// draft, locked, edited or unrecorded.
	LockState string
	// Mark is the brief's one sidebar state mark and its label, from
	// briefMark: the one mark the Briefs tree, the Features list and Home's
	// Features tile read.
	Mark      string
	MarkLabel string
	// OpenThreads and Threads are the brief's unresolved and total comment
	// threads; OpenThreads is internal/briefs' OpenThreads, the number the
	// payload's open_threads carries.
	OpenThreads int
	Threads     int
	// CommentsPanel is the brief's threads baked into the page for a
	// static build's read-only rail; empty for a brief with none.
	CommentsPanel template.HTML
	// Pill is the header's pill; PillShort the phone's, which has room for
	// one word ("Edited" where the wide pill reads "Edited since approval").
	Pill      template.HTML
	PillShort template.HTML
	// Approval is the standing approval a locked or edited brief's meta line
	// names, with its reason; nil for a draft and an unrecorded brief.
	Approval *BriefApprovalView
	// Edit is the edited-since-approval page (NIT-199): nil unless the
	// brief is locked and its file moved since the approval.
	Edit *BriefEditView
	// Review is the review-pending page (NIT-200): nil unless a locked
	// brief's rests_on baselines no longer match the claims.
	Review *BriefReviewView
	// ReviewPending is Review != nil, also stamped on the section so the
	// Threads block can speak of confirming without reading the banners.
	ReviewPending bool
	// RepoPath is the brief's path for `git log -p` in the recover note:
	// from the repository root when the project sits in a git work tree
	// (RepoRelative), else relative to the project directory. Set only for
	// an edited brief with no approved text.
	RepoPath     string
	RepoRelative bool
	Body         template.HTML
	// Meta is the header's count line: words against the word cap, images
	// against the image cap.
	Meta string
	// MetaShort is the phone's meta line, which has room for the words
	// alone beside the pill and the "On this page" button.
	MetaShort string
	// RestsOn and CitedBy are relationship rows (components'
	// writeRelationshipRow); the counts head each list.
	RestsOn      template.HTML
	RestsOnCount int
	CitedBy      template.HTML
	CitedByCount int
	// Search is what the sidebar search matches a brief against: its title,
	// summary and folder, lowercased.
	Search string
	// Feature is set on a brief in features/ (NIT-201): its Made of list,
	// which takes the Rests on list's place. Nil on every other brief.
	Feature *FeatureDetail
}

// OpenThreadsLabel is the index row's thread count, "1 open thread".
func (p BriefPageView) OpenThreadsLabel() string { return openThreadsLabel(p.OpenThreads) }

// RestsOnLabel and CitedByLabel are the relationship lists' heads.
func (p BriefPageView) RestsOnLabel() string { return "Rests on · " + claimCount(p.RestsOnCount) }
func (p BriefPageView) CitedByLabel() string { return "Cited by · " + claimCount(p.CitedByCount) }

func claimCount(n int) string {
	if n == 1 {
		return "1 claim"
	}
	return strconv.Itoa(n) + " claims"
}

// renderedBrief is one brief's body, rendered once per render and shared by
// its page and the briefs payload, and the page id both name it by.
type renderedBrief struct {
	anchor string
	body   string
}

// renderBriefs renders every brief body in document mode with its images on
// brief-assets/<folder>/, and assigns each its page id.
func renderBriefs(set *briefs.Set, cat *catalog.Catalog, cfg *config.Config) map[string]renderedBrief {
	if set.Empty() {
		return nil
	}
	anchors := briefAnchors(set, cat, cfg)
	out := make(map[string]renderedBrief, len(set.Briefs))
	for _, b := range set.Briefs {
		out[b.ID] = renderedBrief{
			anchor: anchors[b.ID],
			body:   string(markdown.RenderDocument(b.Body, BriefAssetURLPrefix(b.Folder))),
		}
	}
	return out
}

// briefAnchors gives each brief its page id, brief-<folder>-<slug>. Two
// different briefs can spell the same one ("a-b/c" and "a/b-c"), and a
// module's section or facet id can too, since every one of them is drawn
// from [a-z0-9-]. A page id must name one element, so the second and later
// claimant take -2, -3 and so on, in brief-id order. Briefs are sorted by
// id, so the assignment is stable across renders of one tree.
func briefAnchors(set *briefs.Set, cat *catalog.Catalog, cfg *config.Config) map[string]string {
	taken := reservedSectionIDs(cat, cfg)
	out := make(map[string]string, len(set.Briefs))
	for _, b := range set.Briefs {
		base := "brief-" + b.Folder + "-" + b.Slug
		id := base
		for n := 2; taken[id]; n++ {
			id = base + "-" + strconv.Itoa(n)
		}
		taken[id] = true
		out[b.ID] = id
	}
	return out
}

// reservedSectionIDs is every section id the shell renders besides briefs:
// Home, the index, the Constitution's three, and each module's section and
// its facet tabs, spelled exactly as buildGroups spells them.
func reservedSectionIDs(cat *catalog.Catalog, cfg *config.Config) map[string]bool {
	taken := map[string]bool{
		"_home": true, briefsIndexID: true,
		"constitution": true, "constitution-file": true, "constitution-project-claims": true,
		ungroupedModuleName: true,
	}
	modules := map[string]bool{}
	if cfg != nil {
		for _, m := range cfg.Modules {
			modules[m] = true
		}
	}
	if cat != nil {
		for _, c := range cat.Claims {
			if c.Module != "" {
				modules[c.Module] = true
			}
		}
	}
	for m := range modules {
		taken[slugify(m)] = true
		for _, f := range visibility.ViewerTabs() {
			taken[slugify(m+"-"+f)] = true
		}
	}
	return taken
}

// buildBriefsView assembles the tree and the pages. rendered is
// renderBriefs' output for the same set. review is the briefs' lock and
// review state (NIT-205), read by every row's mark (briefMark); nil where a
// render has none, and the marks fall back to the frontmatter status.
// cfg places the project in its repository, for the path an edited brief's
// recover note names (nil: the path relative to the project directory).
func buildBriefsView(set *briefs.Set, rendered map[string]renderedBrief, cat *catalog.Catalog, cfg *config.Config, review *briefs.Evaluation) BriefsView {
	view := BriefsView{IndexID: briefsIndexID}
	if set.Empty() {
		return view
	}
	statuses := buildTargetStatusLookup(cat)
	citedBy := internalCitations(cat)
	targets := briefLinkTargets(set, rendered)
	var madeOf map[string]madeOfClaim
	byFolder := map[string]*BriefFolderView{}
	var names []string
	for _, b := range set.Briefs {
		page := briefPage(b, set.Caps, rendered[b.ID], statuses, citedBy[b.Path], targets, review)
		if page.Edit != nil && !page.Edit.Retained {
			dir := ""
			if cfg != nil {
				dir = cfg.Dir()
			}
			page.RepoPath, page.RepoRelative = repoRelativePath(dir, b.Path)
		}
		if b.Folder == featuresFolder {
			if madeOf == nil {
				madeOf = madeOfIndex(cat)
			}
			view.Features = append(view.Features, featurePage(page, b, madeOf, statuses, review))
			continue
		}
		f, ok := byFolder[b.Folder]
		if !ok {
			f = &BriefFolderView{Name: b.Folder, Label: components.DisplayCase(b.Folder)}
			byFolder[b.Folder] = f
			names = append(names, b.Folder)
		}
		f.Pages = append(f.Pages, page)
		f.Count++
		view.Total++
	}
	sort.Strings(names)
	for _, n := range names {
		view.Folders = append(view.Folders, *byFolder[n])
	}
	view.Index = buildBriefsIndex(set, view.Folders, len(view.Features))
	return view
}

func briefPage(b briefs.Brief, caps config.BriefCaps, r renderedBrief, statuses map[string]components.TargetStatus, citedBy []string, targets map[string]string, review *briefs.Evaluation) BriefPageView {
	folderLabel := components.DisplayCase(b.Folder)
	rv := briefReviewOf(review, b)
	mark, markLabel, _ := briefMark(rv)
	lockState := string(rv.LockState)
	links := func(body template.HTML) template.HTML {
		return template.HTML(linkCodeRefs(resolveBriefLinks(string(body), b.Path, targets), b.Path, targets, statuses))
	}
	edit := briefEditView(b, rv)
	if edit != nil && edit.Retained {
		edit.Changes, edit.Approved, edit.Current = links(edit.Changes), links(edit.Approved), links(edit.Current)
	}
	pending := briefReviewView(rv)
	if pending != nil {
		for i := range pending.Claims {
			if pending.Claims[i].Diff != "" {
				pending.Claims[i].Diff = links(pending.Claims[i].Diff)
			}
		}
	}
	var restsOnNotes map[string]string
	if pending != nil {
		restsOnNotes = pending.ChangedAt
	}
	return BriefPageView{
		ID:            r.anchor,
		Path:          b.Path,
		Folder:        b.Folder,
		FolderLabel:   folderLabel,
		Title:         b.Title,
		NavLabel:      b.Title,
		Summary:       b.Summary,
		Status:        string(b.Status),
		LockState:     lockState,
		Mark:          mark,
		MarkLabel:     markLabel,
		BriefID:       b.ID,
		OpenThreads:   b.OpenThreads(),
		Threads:       len(b.Comments),
		CommentsPanel: components.BriefCommentsPanelHTML(b.ID, b.Comments),
		Pill:          components.BriefLockPillHTML(reviewPillState(rv), string(b.Status), false),
		PillShort:     components.BriefLockPillHTML(reviewPillState(rv), string(b.Status), true),
		Approval:      briefApproval(rv),
		Edit:          edit,
		Review:        pending,
		ReviewPending: pending != nil,
		Body:          links(template.HTML(briefBodyOutline(withoutTitleHeading(r.body, b.Body)))),
		Meta:          fmt.Sprintf("%s of %s words · %d of %d images", groupDigits(b.Words), groupDigits(caps.Words), len(b.Images), caps.Images),
		MetaShort:     groupDigits(b.Words) + " words",
		RestsOn:       components.BriefRelationRowsChangedHTML(b.RestsOn, statuses, restsOnNotes),
		RestsOnCount:  len(b.RestsOn),
		CitedBy:       components.BriefRelationRowsHTML(citedBy, statuses),
		CitedByCount:  len(citedBy),
		Search:        strings.ToLower(strings.Join([]string{b.Title, sentenceCase(b.Slug), b.Summary, folderLabel}, " ")),
	}
}

// briefMark is a brief's one sidebar mark, its label, and the word Home's
// Features tile shows for it. The Briefs tree, the Features list, the
// "All briefs" index (NIT-203), Home's Briefs tile and the waiting-card
// halves all read it, from the brief's lock and review state (NIT-205's
// Review: the payload's lock_state, review_pending and open_threads). The
// marks outrank one another as the sidebar legend reads: edited since
// approval, then review pending, then an open thread (NIT-198: a locked
// brief with a thread the human has not resolved shows the thread), then
// the draft dot, then the padlock. A brief whose file says locked with no
// standing record (unrecorded) is not locked: it takes the draft mark, and
// check reports brief-unrecorded. The state is the lock state's word,
// raised to edited or review when those hold; a thread is a mark, not a
// state.
func briefMark(r briefs.Review) (mark, label, state string) {
	state = "draft"
	if r.LockState == briefs.LockLocked {
		state = "locked"
	}
	switch {
	case r.LockState == briefs.LockEdited:
		return "edited", "Edited since approval", "edited"
	case r.ReviewPending:
		// NIT-200: the amber review mark. A standing locked brief (and an
		// edited one that is also pending — edited already returned above)
		// whose rests_on baselines moved. ReviewPending is never true on a
		// draft; Evaluate is the only writer.
		return "review", "Review pending", "review"
	case r.OpenThreads > 0:
		return "thread", openThreadsLabel(r.OpenThreads), state
	case r.LockState == briefs.LockUnrecorded:
		return "draft", "Not locked: no approval on record", state
	case state == "locked":
		return "locked", "Locked", state
	}
	return "draft", "Draft", state
}

// briefReviewOf is b's Review from review, or, for a render with no
// evaluation (nil), one read from the frontmatter status and the brief's
// own threads, so the marks fall back to what the file says.
func briefReviewOf(review *briefs.Evaluation, b briefs.Brief) briefs.Review {
	if review != nil {
		return review.Review(b)
	}
	r := briefs.Review{LockState: briefs.LockDraft, OpenThreads: b.OpenThreads()}
	if b.Status == briefs.StatusLocked {
		r.LockState = briefs.LockLocked
	}
	return r
}

// openThreadsLabel is "1 open thread" or "N open threads".
func openThreadsLabel(n int) string {
	if n == 1 {
		return "1 open thread"
	}
	return strconv.Itoa(n) + " open threads"
}

// featurePage turns a features/ brief's page into a feature page (NIT-201):
// its sidebar row reads the brief's title (Paper B4 lists "Export to CSV",
// which no file name can spell), its meta line counts what it rests on, and
// its Made of list replaces the Rests on list.
func featurePage(page BriefPageView, b briefs.Brief, index map[string]madeOfClaim, statuses map[string]components.TargetStatus, review *briefs.Evaluation) BriefPageView {
	var changed map[string]string
	if page.Review != nil {
		changed = page.Review.ChangedAt
	}
	f := buildFeatureDetail(b.RestsOn, index, statuses, changed)
	modules := make([]string, 0, len(f.Groups))
	for _, g := range f.Groups {
		if !g.Unknown {
			modules = append(modules, g.Label)
		}
	}
	page.Feature = f
	_, _, f.State = briefMark(briefReviewOf(review, b))
	page.Meta = featureMeta(f, modules, page.Meta)
	page.MetaShort = featureMetaShort(f)
	page.RestsOn = ""
	page.RestsOnCount = 0
	return page
}

// withoutTitleHeading drops the level-1 heading a body opens with when that
// heading is the brief's title: the page header already shows the title, and
// the board draws the body starting at its first section. A body whose title
// is not its opening line (a paragraph first, or the title inside a quote)
// keeps every heading it has.
func withoutTitleHeading(rendered, body string) string {
	if _, ok := markdown.DocumentTitle(body); !ok {
		return rendered
	}
	trimmed := strings.TrimLeft(rendered, " \n")
	if !strings.HasPrefix(trimmed, "<h1>") {
		return rendered
	}
	end := strings.Index(trimmed, "</h1>")
	if end < 0 {
		return rendered
	}
	return strings.TrimLeft(trimmed[end+len("</h1>"):], "\n")
}

// briefHeadingLevel is where each document-mode heading level sits on the
// brief page. The page's title is its h2 (the level a module's or Home's title
// takes, under the sidebar's h1), so the body starts one below it: a "##"
// section is an h3, and a stray "#" after the title is an h3 too — it may not
// outrank the title it sits under. "###" and deeper follow one step down,
// with h6 as the floor.
var briefHeadingLevel = [7]byte{0, '3', '3', '4', '5', '6', '6'}

// briefBodyOutline moves the rendered body's headings under the page title
// (briefHeadingLevel). The renderer writes a heading as a bare <hN> and </hN>
// with no attribute, and escapes every "<" of author text, so the only "<h"
// followed by a digit in its output is a heading tag. The claim page and the
// payload's body_html keep document mode's own levels; this is the page's
// outline only.
func briefBodyOutline(body string) string {
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c == '<' && i+3 < len(body) {
			j := i + 1
			if body[j] == '/' {
				j++
			}
			if j+2 < len(body) && body[j] == 'h' && body[j+1] >= '1' && body[j+1] <= '6' && body[j+2] == '>' {
				b.WriteString(body[i:j])
				b.WriteByte('h')
				b.WriteByte(briefHeadingLevel[body[j+1]-'0'])
				b.WriteByte('>')
				i = j + 2
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

// internalCitations maps a brief path to the claims that cite it: every
// claim with an internal source whose path is the brief's, sorted by id,
// each once. A source path is relative to the config directory, the same
// anchor a brief's Path is.
func internalCitations(cat *catalog.Catalog) map[string][]string {
	out := map[string][]string{}
	if cat == nil {
		return out
	}
	for _, c := range cat.Claims {
		seen := map[string]bool{}
		for _, s := range c.Sources {
			if s.Kind != model.SourceKindInternal || s.Path == "" {
				continue
			}
			p := path.Clean(filepath.ToSlash(s.Path))
			if seen[p] {
				continue
			}
			seen[p] = true
			out[p] = append(out[p], c.ID)
		}
	}
	for p := range out {
		sort.Strings(out[p])
	}
	return out
}

// sentenceCase turns a [a-z0-9-] name into words with the first capitalized.
func sentenceCase(name string) string {
	s := strings.ReplaceAll(name, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// groupDigits writes n with a comma between each group of three digits, as
// the board spells a cap: "2,000".
func groupDigits(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// repoRelativePath is rel (relative to configDir, the project directory)
// from the root of the git work tree holding the project: the nearest
// ancestor of configDir with a .git entry (a directory, or the file a
// worktree has). With none found, or no configDir, it is rel itself and
// false.
// It reads the file system and runs no git: a static page stays a function
// of the tree it was built from.
func repoRelativePath(configDir, rel string) (string, bool) {
	if configDir == "" {
		return rel, false
	}
	dir := filepath.Clean(configDir)
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			r, err := filepath.Rel(dir, filepath.Join(configDir, filepath.FromSlash(rel)))
			if err != nil {
				return rel, false
			}
			return filepath.ToSlash(r), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return rel, false
		}
		dir = parent
	}
}
