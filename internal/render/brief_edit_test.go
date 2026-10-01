package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
)

// arrow is the brief image, by its digest: arrowWas at the approval,
// arrowNow when a test changes its bytes.
const (
	arrowWas = "0000000000000000000000000000000000000000000000000000000000000001"
	arrowNow = "0000000000000000000000000000000000000000000000000000000000000002"
)

func briefImageDigest(rel, digest string) briefs.File {
	return briefs.File{Rel: rel, Regular: true, Size: 64, Digest: digest}
}

// editedBriefPage renders a project holding one brief at
// briefs/voice/money.md that was approved reading approved (beside
// arrow.png at arrowWas) and now reads current (beside arrow.png at
// arrowDigest), with the lock record `brief lock` writes — its approved
// markdown kept unless approvedText is false — and returns the brief's
// section.
func editedBriefPage(t *testing.T, approved, current string, approvedText bool) string {
	t.Helper()
	return editedBriefPageWith(t, approved, current, arrowWas, approvedText)
}

func editedBriefPageWith(t *testing.T, approved, current, arrowDigest string, approvedText bool) string {
	t.Helper()
	cat, cfg := briefViewFixture()
	was := briefs.FromFiles(cfg, []briefs.File{briefFile("voice/money.md", approved), briefImageDigest("voice/arrow.png", arrowWas)}).Briefs[0]
	set := briefs.FromFiles(cfg, []briefs.File{briefFile("voice/money.md", current), briefImageDigest("voice/arrow.png", arrowDigest)})
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	rec := lock.BriefRecord{Path: was.Path, Hash: was.LockHash, At: "2026-09-18T10:00:00Z", Reason: "voice approved", Images: was.ImageDigests(),
		Approved: lock.BriefApproved{Summary: was.Summary, RestsOn: was.RestsOn, Markdown: was.Body}}
	if !approvedText {
		// A record missing only its markdown: the summary and rests_on
		// are there, and the page still has nothing to diff the body
		// against.
		rec.Approved.Markdown = ""
	}
	lock.RecordBriefApproval(store, was.ID, rec)
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return sectionHTML(t, out, "brief-voice-money")
}

// view returns one of the edited page's three views.
func view(t *testing.T, section, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<article class="brief-body brief-view" data-view="` + name + `"( hidden)?>(.*?)</article>`).FindStringSubmatch(section)
	if m == nil {
		t.Fatalf("no %s view in:\n%s", name, section)
	}
	return m[2]
}

const moneyApproved = "---\nsummary: Plain, specific, never scolding.\nstatus: locked\n---\n# Talking about money\n\n## Principles\n\nSay the amount before the person.\n\nNever use the word debt.\n\n![An arrow](arrow.png)\n\n## Words we avoid\n\n- debt\n- owe\n"

const moneyCurrent = "---\nsummary: Plain, specific, never scolding.\nstatus: locked\n---\n# Talking about money\n\n## Principles\n\nSay the amount before the person.\n\nAvoid debt and owe in the interface.\n\n![An arrow](arrow.png)\n\n## Reminders\n\nA reminder names the expense.\n\n## Words we avoid\n\n- debt\n- owe\n"

// TestRender_BriefEditedSinceApproval is the B2 page contract (NIT-199) at
// its owner, the renderer: a locked brief whose file moved since its
// approval carries the EDITED SINCE APPROVAL pill (a word on a phone), the
// edited mark, the approval's date and reason in the meta line, the banner,
// and three views of the body — Changes (the approved passage struck above
// the current one, a section added whole as one added block), Approved (the
// approved text with no marks) and Current (no removed passage; each changed
// run ruled and captioned) — each with its headings under the page title and
// its image on the brief-assets path, Changes shown first.
func TestRender_BriefEditedSinceApproval(t *testing.T) {
	sec := editedBriefPage(t, moneyApproved, moneyCurrent, true)

	for _, want := range []string{
		`data-lock-state="edited"`,
		`<span class="brief-wide"><span class="pill brief-pill--edited brief-pill">`,
		`<span class="brief-pill__label">Edited since approval</span>`,
		`<span class="brief-narrow"><span class="pill brief-pill--edited brief-pill"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock"/></svg><span class="brief-pill__label">Edited</span>`,
		`Approved <time class="brief-date" datetime="2026-09-18T10:00:00Z" data-date-form="long">18 Sep 2026</time> · <span class="brief-meta__reason">“voice approved”</span> · `,
		`Approved <time class="brief-date" datetime="2026-09-18T10:00:00Z" data-date-form="short">18 Sep</time>`,
		`Edited after you approved it<span class="brief-wide"> · 3 passages changed</span>`,
		`on your yes the agent unlocks it, fixes it and locks it again, or restores the approved text from version control`,
		`<p class="brief-compare__note">The record keeps each image's fingerprint, not its bytes: every view draws the images as they are now.</p>`,
		`<code class="brief-banner__rule">brief-content-drift</code>`,
		`3 passages changed. Check fails until it is settled.`,
		`<div class="brief-compare" data-brief-view="changes">`,
		`aria-pressed="true">Changes</button>`,
		`<button type="button" class="brief-comment" aria-controls="commentsPanel" aria-expanded="false" disabled><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-message-circle"/></svg>Approve or restore in a thread</button>`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("edited page is missing %q", want)
		}
	}
	if strings.Contains(sec, `<article class="brief-body">`) {
		t.Error("an edited brief with its approved text shows the views, not the plain body")
	}

	changes, approved, current := view(t, sec, "changes"), view(t, sec, "approved"), view(t, sec, "current")
	if strings.Contains(sec, `data-view="changes" hidden`) || !strings.Contains(sec, `data-view="approved" hidden`) || !strings.Contains(sec, `data-view="current" hidden`) {
		t.Error("Changes is the view shown first, the other two hidden")
	}
	for name, v := range map[string]string{"changes": changes, "approved": approved, "current": current} {
		if !strings.Contains(v, `<h3>Principles</h3>`) || !strings.Contains(v, `<h3>Words we avoid</h3>`) {
			t.Errorf("%s: headings are not h3 under the page title:\n%s", name, v)
		}
		if strings.Contains(v, "<h2>") || strings.Contains(v, "Talking about money") {
			t.Errorf("%s: the title heading is the page header's, not the view's:\n%s", name, v)
		}
		if !strings.Contains(v, `<img class="md-img" src="brief-assets/voice/arrow.png" alt="An arrow">`) {
			t.Errorf("%s: the image is not on the brief-assets path:\n%s", name, v)
		}
		if !strings.Contains(v, "<li>owe</li>") {
			t.Errorf("%s: the list does not render as a list:\n%s", name, v)
		}
	}

	if !regexp.MustCompile(`<div class="claim-edit-passage claim-edit-passage--removed"><p>Never use the word debt\.</p></div><div class="claim-edit-passage claim-edit-passage--added"><p>Avoid debt and owe in the interface\.</p></div>`).MatchString(changes) {
		t.Errorf("Changes does not strike the approved passage above the current one:\n%s", changes)
	}
	if !strings.Contains(changes, `<div class="claim-edit-passage claim-edit-passage--added"><h3>Reminders</h3><p>A reminder names the expense.</p></div>`) {
		t.Errorf("Changes does not show the added section as one added block:\n%s", changes)
	}
	if strings.Contains(changes, `claim-edit-passage--removed"><p><img`) {
		t.Errorf("the unchanged image passage is redlined:\n%s", changes)
	}

	if strings.Contains(approved, "claim-edit") || !strings.Contains(approved, "<p>Never use the word debt.</p>") || strings.Contains(approved, "Reminders") {
		t.Errorf("Approved is not the approved text, unmarked:\n%s", approved)
	}

	if strings.Contains(current, "Never use") || strings.Contains(current, "claim-edit-passage--removed") {
		t.Errorf("Current shows a removed passage:\n%s", current)
	}
	for _, want := range []string{
		`<div class="claim-edit-passage claim-edit-passage--current brief-passage--current"><p>Avoid `,
		`<p class="brief-passage__caption">changed since approval</p>`,
		`<h3>Reminders</h3><p>A reminder names the expense.</p><p class="brief-passage__caption">added since approval</p>`,
	} {
		if !strings.Contains(current, want) {
			t.Errorf("Current is missing %q:\n%s", want, current)
		}
	}
}

// TestRender_BriefEditedWithNoApprovedText is the fallback: a standing
// record that carries no approved markdown (its summary and rests_on kept)
// cannot be diffed against, so the page says so, names the recovery the
// router gives (version control, or unlock → fix → lock on the human's yes;
// there is no brief recover command), and shows the file as it reads now with
// no switch — never a diff against an empty approval that would mark every
// passage added. With no project directory to place in a repository, the
// git path is the project's own and the note says where to run it.
func TestRender_BriefEditedWithNoApprovedText(t *testing.T) {
	sec := editedBriefPage(t, moneyApproved, moneyCurrent, false)
	for _, want := range []string{
		`class="brief-recover"`,
		`The approved wording was not kept`,
		`<code>git log -p -- briefs/voice/money.md</code>, run from the project directory, shows`,
		`on your yes the agent restores that text, or unlocks the brief, fixes it and locks it again`,
		`<article class="brief-body"><h3>Principles</h3>`,
		`Edited after you approved it<span class="brief-wide"></span>`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("recover page is missing %q:\n%s", want, sec)
		}
	}
	for _, not := range []string{"brief-compare", "brief-view-switch", "claim-edit-passage", "recover-approved-content"} {
		if strings.Contains(sec, not) {
			t.Errorf("recover page carries %q", not)
		}
	}
}

// TestRender_BriefEditedEscapesAuthorMarkup is the XSS lens: author markup
// in either wording reaches all three views as text, never as a tag or an
// attribute, and a word mark's aria-label is escaped.
func TestRender_BriefEditedEscapesAuthorMarkup(t *testing.T) {
	approved := "---\nsummary: s\nstatus: locked\n---\nSafe <script>alert(1)</script> and \"q\" text <b>x</b>.\n"
	current := "---\nsummary: s\nstatus: locked\n---\nSafe <img src=x onerror=alert(2)> and \"q\" text <b>x</b>.\n"
	sec := editedBriefPage(t, approved, current, true)
	for _, bad := range []string{"<script>", "<img src=x", "<b>x</b>", `onerror=alert(2)>`} {
		if strings.Contains(sec, bad) {
			t.Errorf("author markup %q reached the page as markup", bad)
		}
	}
	for _, v := range []string{"changes", "approved", "current"} {
		body := view(t, sec, v)
		if !strings.Contains(body, "&lt;") {
			t.Errorf("%s view does not carry the author's markup as escaped text:\n%s", v, body)
		}
	}
	if m := regexp.MustCompile(`aria-label="([^"]*)"`).FindAllStringSubmatch(view(t, sec, "changes"), -1); len(m) == 0 {
		t.Error("the redline names the moved words for a screen reader")
	} else {
		for _, a := range m {
			if strings.ContainsAny(a[1], "<>") {
				t.Errorf("aria-label is not escaped: %q", a[1])
			}
		}
	}
}

// TestRender_BriefLockStateInTheHeaderAndMark is the header and sidebar for
// the other lock states the record decides: a locked brief names its
// approval's date and reason and keeps the padlock; a brief whose file says
// locked with no approval on record is not drawn as locked (LOCK NOT
// RECORDED, the draft mark, and the meta line says why); and the mark
// follows the legend's priority when a brief carries more than one state.
func TestRender_BriefLockStateInTheHeaderAndMark(t *testing.T) {
	cat, cfg := briefViewFixture()
	doc := "---\nsummary: s\nstatus: locked\n---\nBody.\n"
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("a/locked.md", doc),
		briefFile("a/unrecorded.md", doc),
	})
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	b := set.Briefs[0]
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{Path: b.Path, Hash: b.LockHash, At: "2026-09-18T23:30:00Z", Reason: "ok <b>", Images: map[string]string{},
		Approved: lock.BriefApproved{Summary: b.Summary, Markdown: b.Body}})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	locked, unrecorded := sectionHTML(t, out, "brief-a-locked"), sectionHTML(t, out, "brief-a-unrecorded")
	for _, want := range []string{
		`data-lock-state="locked"`,
		`<span class="brief-pill__label">Locked</span>`,
		`Approved <time class="brief-date" datetime="2026-09-18T23:30:00Z" data-date-form="long">18 Sep 2026</time> · <span class="brief-meta__reason">“ok &lt;b&gt;”</span> · 1 of 2,000 words`,
		`<article class="brief-body"><p>Body.</p></article>`,
	} {
		if !strings.Contains(locked, want) {
			t.Errorf("locked page is missing %q:\n%s", want, locked)
		}
	}
	for _, not := range []string{"brief-banner", "brief-compare", "Approve or restore in a thread"} {
		if strings.Contains(locked, not) {
			t.Errorf("an unchanged locked brief carries %q", not)
		}
	}
	for _, want := range []string{
		`data-lock-state="unrecorded"`,
		`<span class="pill pv brief-pill--unrecorded brief-pill"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-lock-open"/></svg><span class="brief-pill__label">Lock not recorded</span>`,
		`<span class="brief-pill__label">Unrecorded</span>`,
		`Not locked: no approval on record · 1 of 2,000 words`,
	} {
		if !strings.Contains(unrecorded, want) {
			t.Errorf("unrecorded page is missing %q:\n%s", want, unrecorded)
		}
	}
	if strings.Contains(unrecorded, ">Locked</span>") || strings.Contains(unrecorded, "Approved <time") {
		t.Error("a brief with no approval on record is drawn as locked")
	}
	if !strings.Contains(out, `>Unrecorded</span><span class="brief-mark" data-mark="draft" role="img" aria-label="Not locked: no approval on record"`) {
		t.Error("the unrecorded brief's sidebar mark is not the draft mark with its reason")
	}
}

// TestRenderBriefDiff_Shapes is the diff's own contract on the shapes a brief
// edit takes: a heading-only change and a list-only change redline just that
// passage; appending a section leaves the last approved passage unmarked
// (textdiff keeps trailing blank lines on a passage, endPassage evens them);
// a retitled brief shows the title change; and the count takes a passage
// and its replacement once, so five reworded paragraphs in a row are five.
func TestRenderBriefDiff_Shapes(t *testing.T) {
	prefix := BriefAssetURLPrefix("voice")
	for _, tc := range []struct {
		name, before, after string
		changed             int
		changes             []string
		notChanges          []string
	}{
		{"heading only", "## Old name\n\nText.\n", "## New name\n\nText.\n", 1,
			[]string{`<div class="claim-edit-passage claim-edit-passage--removed"`, `<h3><span class="claim-edit-word claim-edit-word--removed">Old</span> name</h3>`, `<h3><span class="claim-edit-word claim-edit-word--added">New</span> name</h3>`, `</div><p>Text.</p>`}, nil},
		{"list only", "Intro.\n\n- one\n- two\n", "Intro.\n\n- one\n- three\n", 1,
			[]string{`<p>Intro.</p><div class="claim-edit-passage claim-edit-passage--removed"`, `<li><span class="claim-edit-word claim-edit-word--removed">two</span></li>`, `<li><span class="claim-edit-word claim-edit-word--added">three</span></li>`}, nil},
		{"appended section", "Intro.\n\n![Arrow](arrow.png)\n", "Intro.\n\n![Arrow](arrow.png)\n\n## Next\n\nMore.\n", 2,
			[]string{`<p><img class="md-img" src="brief-assets/voice/arrow.png" alt="Arrow"></p><div class="claim-edit-passage claim-edit-passage--added"><h3>Next</h3><p>More.</p></div>`},
			[]string{"--removed"}},
		{"retitled", "# Old title\n\nText.\n", "# New title\n\nText.\n", 1,
			[]string{`<h3><span class="claim-edit-word claim-edit-word--removed">Old</span> title</h3>`, `<h3><span class="claim-edit-word claim-edit-word--added">New</span> title</h3>`}, nil},
		{"two separate changes", "A.\n\nB.\n\nC.\n", "A2.\n\nB.\n\nC2.\n", 2, nil, nil},
		{"five reworded in a row", "A one.\n\nB one.\n\nC one.\n\nD one.\n\nE one.\n", "A two.\n\nB two.\n\nC two.\n\nD two.\n\nE two.\n", 5, nil, nil},
		{"identical", "A.\n\nB.\n", "A.\n\nB.\n", 0, nil, []string{"claim-edit-passage"}},
	} {
		d := RenderBriefDiff(tc.before, tc.after, prefix)
		if d.Changed != tc.changed {
			t.Errorf("%s: changed = %d, want %d", tc.name, d.Changed, tc.changed)
		}
		changes := string(d.ChangesHTML())
		for _, want := range tc.changes {
			if !strings.Contains(changes, want) {
				t.Errorf("%s: Changes is missing %q:\n%s", tc.name, want, changes)
			}
		}
		for _, not := range tc.notChanges {
			if strings.Contains(changes, not) {
				t.Errorf("%s: Changes carries %q:\n%s", tc.name, not, changes)
			}
		}
	}
}

// TestRender_BriefEditedOtherFields covers an edit the redline cannot
// show. A moved summary or rests_on is named under it. Images are compared
// by the digests the record signs, not inferred: an image whose bytes changed
// is named, the banner says so, and one the approved text referenced and the
// brief no longer does is listed as removed and drawn as a named placeholder
// rather than a broken image. An edit that changes only trailing whitespace
// (the lock signs every byte; the diff does not draw it) says exactly that,
// and never blames an image.
func TestRender_BriefEditedOtherFields(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	b := briefs.FromFiles(cfg, []briefs.File{briefFile("voice/money.md", "---\nsummary: Now.\nstatus: locked\nrests_on: [widget.contract.b]\n---\nBody.\n")}).Briefs[0]
	e := briefEditView(b, briefs.Review{LockState: briefs.LockEdited, Approved: &lock.BriefApproved{Summary: "Then.", RestsOn: []string{"widget.contract.a"}, Markdown: "Body.\n"}})
	if e == nil || !e.Retained || e.SummaryWas != "Then." || strings.Join(e.RestsOnAdded, ",") != "widget.contract.b" || strings.Join(e.RestsOnRemoved, ",") != "widget.contract.a" || e.WhitespaceOnly || e.Headline() != "the summary changed" {
		t.Fatalf("summary and rests_on edit = %+v", e)
	}
	if briefEditView(b, briefs.Review{LockState: briefs.LockLocked, Approved: &lock.BriefApproved{Markdown: "x"}}) != nil {
		t.Fatal("a brief that is not edited gets an edit view")
	}

	// An image's bytes changed; the text reads as approved.
	sec := editedBriefPageWith(t, moneyApproved, moneyApproved, arrowNow, true)
	for _, want := range []string{
		`Edited after you approved it<span class="brief-wide"> · an image changed</span>`,
		`<p class="brief-edit-other__head">No passage differs</p>`,
		`<p>Images, changed: <code>arrow.png</code></p>`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("image-only edit is missing %q:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "whitespace") {
		t.Error("an image edit is reported as whitespace")
	}

	// Only the trailing newline moved.
	sec = editedBriefPage(t, moneyApproved, strings.TrimSuffix(moneyApproved, "\n")+"\n\n\n", true)
	for _, want := range []string{
		`Edited after you approved it<span class="brief-wide"> · only whitespace changed</span>`,
		`Only whitespace differs:`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("whitespace-only edit is missing %q:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "image changed") || strings.Contains(sec, "Images, ") {
		t.Error("a whitespace-only edit blames an image")
	}

	// The approved text drew an image the brief no longer references.
	sec = editedBriefPage(t, moneyApproved, strings.Replace(moneyCurrent, "![An arrow](arrow.png)\n\n", "", 1), true)
	approved := view(t, sec, "approved")
	if !strings.Contains(sec, `<p>Images, removed: <code>arrow.png</code></p>`) ||
		!strings.Contains(approved, `<span class="brief-img-absent">image removed: arrow.png</span>`) || strings.Contains(approved, "<img") {
		t.Errorf("a removed image is not listed and named in place:\n%s", sec)
	}
}

// TestRepoRelativePath is the recover note's git path: from the nearest
// ancestor holding a .git entry (a worktree's .git is a file), else the
// project-relative path, flagged so the note says where to run it.
func TestRepoRelativePath(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "docs", "spec")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := repoRelativePath(project, "briefs/voice/money.md"); ok || got != "briefs/voice/money.md" {
		t.Fatalf("outside a work tree = %q, %v", got, ok)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := repoRelativePath(project, "briefs/voice/money.md"); !ok || got != "docs/spec/briefs/voice/money.md" {
		t.Fatalf("inside a work tree = %q, %v", got, ok)
	}
	if got, ok := repoRelativePath("", "briefs/x/y.md"); ok || got != "briefs/x/y.md" {
		t.Fatalf("no project directory = %q, %v", got, ok)
	}
}
