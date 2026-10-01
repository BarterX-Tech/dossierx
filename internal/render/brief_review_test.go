package render

import (
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// pendingBriefPage renders a locked brief at briefs/widget/flow.md that
// rests on widget.contract.overview. The brief was approved against then;
// the catalog claim now reads now. dropReceipt drops the baseline wording
// so the page must say "earlier wording not available".
func pendingBriefPage(t *testing.T, then, now model.Claim, dropReceipt bool) string {
	t.Helper()
	cat := &catalog.Catalog{Claims: []model.Claim{now}}
	_, cfg := briefViewFixture()
	cat.Claims[0].Module, cat.Claims[0].Facet, cat.Claims[0].Layout = "widget", "contract", model.LayoutCard
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Widget flow\n\n## Why\n\nIt matters.\n"),
	})
	b := set.Briefs[0]
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	hashes, receipts, _ := briefs.Baselines(b, []model.Claim{then})
	if dropReceipt {
		receipts = nil
	}
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
		Path: b.Path, Hash: b.LockHash, At: "2026-09-18T10:00:00Z", Reason: "flow approved",
		Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
		Baselines: hashes, Receipts: receipts,
	})
	if now.Status == model.StatusLocked {
		lock.RecordApproval(store, now, lock.Approval{Reason: "claim reworded", Actor: "human"})
		if rec, ok := store.Record(now.ID); ok {
			rec.At = "2026-09-20T09:00:00Z"
			store.Ledger[now.ID] = rec
		}
	}
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	return sectionHTML(t, out, "brief-widget-flow")
}

func pendingClaim(body string, locked bool) model.Claim {
	st := model.StatusDraft
	if locked {
		st = model.StatusLocked
	}
	return model.Claim{
		ID: "widget.contract.overview", Module: "widget", Facet: "contract",
		Layout: model.LayoutCard, Status: st, Summary: "Overview", Body: body,
	}
}

// TestRender_BriefReviewPending is the B3 page contract (NIT-200) at its
// owner, the renderer: a locked brief whose rests_on claim moved carries
// the REVIEW PENDING pill, the amber review mark, one banner per changed
// claim with the claim redline, "changed <date>" on that Rests on row,
// and "Confirm in a thread" as the page-foot Comment button — the same
// .brief-comment NIT-198 already opens the rail with.
func TestRender_BriefReviewPending(t *testing.T) {
	then := pendingClaim("Say the amount before the person.", true)
	now := pendingClaim("Name the person after the amount.", true)
	sec := pendingBriefPage(t, then, now, false)

	for _, want := range []string{
		`data-lock-state="locked"`,
		`data-review-pending="true"`,
		`<span class="brief-wide"><span class="pill pv brief-pill--review brief-pill">`,
		`<span class="brief-pill__label">Review pending</span>`,
		`<span class="brief-pill__label">Review</span>`,
		`brief-banner--review`,
		`data-claim-id="widget.contract.overview"`,
		`href="#widget.contract.overview">widget.contract.overview</a> changed after approval`,
		`<code class="brief-banner__rule">brief-dependency-drift</code>`,
		`on your yes the agent runs <code>dossierx brief reaudit --confirm</code>`,
		`Claim locking is not blocked.`,
		`class="brief-review-diff"`,
		`claim-edit-passage--removed`,
		`claim-edit-passage--added`,
		`Say the amount before the person.`,
		`Name the person after the amount.`,
		`class="brief-relation-changed">changed 20 Sep</span>`,
		`<button type="button" class="brief-comment" aria-controls="commentsPanel" aria-expanded="false" disabled><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-message-circle"/></svg>Confirm in a thread</button>`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("review-pending page is missing %q", want)
		}
	}
	if strings.Contains(sec, "Approve or restore in a thread") {
		t.Error("a pending brief that is not edited must not use the edited thread label")
	}
}

// TestRender_BriefReviewPendingUnavailableWording is the fallback NIT-192
// named: when no snapshot holds the baseline, the banner still links the
// claim and says "earlier wording not available" instead of inventing a
// redline against empty text.
func TestRender_BriefReviewPendingUnavailableWording(t *testing.T) {
	then := pendingClaim("the wording the brief was approved against", false)
	now := pendingClaim("a claim under review.", false)
	sec := pendingBriefPage(t, then, now, true)
	for _, want := range []string{
		`earlier wording not available`,
		`href="#widget.contract.overview">widget.contract.overview</a> changed after approval`,
		`class="brief-relation-changed">changed</span>`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("unavailable wording page is missing %q", want)
		}
	}
	if strings.Contains(sec, "claim-edit-passage") {
		t.Error("no retained wording must not draw a redline")
	}
}

// TestRender_BriefReviewPendingEscapesAuthorMarkup pins the escaping
// boundary: claim wording reaches the banner through markdown.Render, so
// a <script> in the body is text, and a quote in a changed-at note (we
// format the date ourselves) cannot break out of the attribute. The claim
// id is engine-shaped; the wording is not.
func TestRender_BriefReviewPendingEscapesAuthorMarkup(t *testing.T) {
	then := pendingClaim(`Hello <script>alert(1)</script> & "x".`, true)
	now := pendingClaim(`Hello <img src=x onerror=alert(1)> & "y".`, true)
	sec := pendingBriefPage(t, then, now, false)
	for _, not := range []string{
		"<script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
	} {
		if strings.Contains(sec, not) {
			t.Errorf("author markup leaked: %q", not)
		}
	}
	if !strings.Contains(sec, "&lt;script&gt;alert(1)&lt;/script&gt;") && !strings.Contains(sec, "&lt;img") {
		t.Error("escaped author markup should still be readable as text")
	}
}

// TestRender_BriefReviewPendingAndEdited stacks B3's claim banners above
// B2's views when the agent already edited the brief to follow the claim,
// and captions the redline "Updated by the agent since approval".
func TestRender_BriefReviewPendingAndEdited(t *testing.T) {
	cat, cfg := briefViewFixture()
	then := cat.Claims[0]
	then.Body = "the wording the brief was approved against"
	was := briefs.FromFiles(cfg, []briefs.File{briefFile("widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Widget flow\n\nAs approved.\n")}).Briefs[0]
	set := briefs.FromFiles(cfg, []briefs.File{briefFile("widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Widget flow\n\nUpdated to follow the claim.\n")})
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	hashes, receipts, _ := briefs.Baselines(was, []model.Claim{then})
	lock.RecordBriefApproval(store, was.ID, lock.BriefRecord{
		Path: was.Path, Hash: was.LockHash, At: "2026-09-18T10:00:00Z", Reason: "flow approved",
		Approved:  lock.BriefApproved{Summary: was.Summary, RestsOn: was.RestsOn, Markdown: was.Body},
		Baselines: hashes, Receipts: receipts,
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	sec := sectionHTML(t, out, "brief-widget-flow")
	for _, want := range []string{
		`data-lock-state="edited"`,
		`data-review-pending="true"`,
		`brief-banner--review`,
		`Updated by the agent since approval`,
		`Edited after you approved it`,
		`Confirm in a thread`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("edited+pending page is missing %q", want)
		}
	}
}

// TestRender_BriefReviewPendingGoneClaim names a rests_on id that left
// the catalog: the banner says it is gone and carries brief-rests-on-missing.
func TestRender_BriefReviewPendingGoneClaim(t *testing.T) {
	then := pendingClaim("was here", false)
	cat := &catalog.Catalog{Claims: []model.Claim{{
		ID: "widget.contract.other", Module: "widget", Facet: "contract",
		Layout: model.LayoutCard, Status: model.StatusDraft, Body: "other",
	}}}
	_, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Widget flow\n"),
	})
	b := set.Briefs[0]
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	hashes, receipts, _ := briefs.Baselines(b, []model.Claim{then})
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
		Path: b.Path, Hash: b.LockHash, At: "2026-09-18T10:00:00Z", Reason: "flow approved",
		Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
		Baselines: hashes, Receipts: receipts,
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	sec := sectionHTML(t, out, "brief-widget-flow")
	for _, want := range []string{
		`is gone`,
		`brief-rests-on-missing`,
		`was here`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("gone-claim page is missing %q:\n%s", want, sec)
		}
	}
}

// TestRender_FeatureReviewPendingMadeOfChanged is the Made of row's
// "changed <date>" note: a features/ brief uses that list instead of
// Rests on, and the same ChangedAt map must still land on the drifted
// claim.
func TestRender_FeatureReviewPendingMadeOfChanged(t *testing.T) {
	then := pendingClaim("as approved", true)
	now := pendingClaim("rewritten for the feature", true)
	cat := &catalog.Catalog{Claims: []model.Claim{now}}
	cat.Claims[0].Module, cat.Claims[0].Facet, cat.Claims[0].Layout = "widget", "contract", model.LayoutCard
	_, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("features/export.md", "---\nsummary: Export the widget.\nstatus: locked\nrests_on: [widget.contract.overview]\n---\n# Export\n"),
	})
	b := set.Briefs[0]
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	hashes, receipts, _ := briefs.Baselines(b, []model.Claim{then})
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
		Path: b.Path, Hash: b.LockHash, At: "2026-09-18T10:00:00Z", Reason: "export approved",
		Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
		Baselines: hashes, Receipts: receipts,
	})
	lock.RecordApproval(store, now, lock.Approval{Reason: "claim reworded", Actor: "human"})
	if rec, ok := store.Record(now.ID); ok {
		rec.At = "2026-09-20T09:00:00Z"
		store.Ledger[now.ID] = rec
	}
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	sec := sectionHTML(t, out, "brief-features-export")
	for _, want := range []string{
		`brief-banner--review`,
		`class="brief-relation-changed">changed 20 Sep</span>`,
		`feature-made-of`,
		`Confirm in a thread`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("pending feature page is missing %q:\n%s", want, sec)
		}
	}
	if strings.Contains(sec, "Rests on ·") {
		t.Error("a feature must keep Made of in place of Rests on when pending")
	}
}

// TestRender_BriefReviewPendingTwoClaims stacks one banner per changed
// rests_on claim. A first-only renderer would still satisfy the single-claim
// page test.
func TestRender_BriefReviewPendingTwoClaims(t *testing.T) {
	thenA := pendingClaim("first as approved", true)
	thenB := thenA
	thenB.ID, thenB.Body = "widget.contract.other", "second as approved"
	nowA := pendingClaim("first rewritten", true)
	nowB := nowA
	nowB.ID, nowB.Body = "widget.contract.other", "second rewritten"
	cat := &catalog.Catalog{Claims: []model.Claim{nowA, nowB}}
	for i := range cat.Claims {
		cat.Claims[i].Module, cat.Claims[i].Facet, cat.Claims[i].Layout = "widget", "contract", model.LayoutCard
	}
	_, cfg := briefViewFixture()
	set := briefs.FromFiles(cfg, []briefs.File{
		briefFile("widget/flow.md", "---\nsummary: The widget flow.\nstatus: locked\nrests_on: [widget.contract.overview, widget.contract.other]\n---\n# Widget flow\n"),
	})
	b := set.Briefs[0]
	store, err := lock.LoadStore(t.TempDir() + "/lock-store.json")
	if err != nil {
		t.Fatal(err)
	}
	hashes, receipts, _ := briefs.Baselines(b, []model.Claim{thenA, thenB})
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
		Path: b.Path, Hash: b.LockHash, At: "2026-09-18T10:00:00Z", Reason: "flow approved",
		Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
		Baselines: hashes, Receipts: receipts,
	})
	out, err := renderBoundedAt(cat, cfg, Extras{Briefs: set, BriefReview: briefs.Evaluate(set, cat.Claims, store)}, time.Unix(1_700_000_000, 0).UTC(), 0)
	if err != nil {
		t.Fatal(err)
	}
	sec := sectionHTML(t, out, "brief-widget-flow")
	if n := strings.Count(sec, `class="brief-banner brief-banner--review"`); n != 2 {
		t.Fatalf("want 2 stacked banners, got %d\n%s", n, sec)
	}
	for _, want := range []string{
		`data-claim-id="widget.contract.overview"`,
		`data-claim-id="widget.contract.other"`,
		`first as approved`,
		`first rewritten`,
		`second as approved`,
		`second rewritten`,
	} {
		if !strings.Contains(sec, want) {
			t.Errorf("two-claim page is missing %q", want)
		}
	}
}
