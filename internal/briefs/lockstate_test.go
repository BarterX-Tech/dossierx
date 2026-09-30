package briefs

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func claimFor(id, body string) model.Claim {
	return model.Claim{ID: id, Module: "widget", Facet: "contract", Status: model.StatusDraft, Summary: "S " + id, Body: body}
}

// lockedSet is one brief resting on two claims, with the status given.
func lockedSet(t *testing.T, status string) *Set {
	t.Helper()
	body := "---\nsummary: A brief.\nstatus: " + status + "\nrests_on:\n  - widget.contract.a\n  - widget.contract.b\n---\n# Flow\n\nText.\n"
	return FromFiles(testConfig(t, t.TempDir(), ""), tree(map[string]File{"flow/overview.md": md(body)}))
}

// recordFor arms a store the way `brief lock` does: the brief's hash and
// approved text, and a baseline and receipt per rests_on claim.
func recordFor(t *testing.T, set *Set, claims []model.Claim) *lock.Store {
	t.Helper()
	store, err := lock.LoadStore(filepath.Join(t.TempDir(), "lock-store.json"))
	if err != nil {
		t.Fatal(err)
	}
	b := set.Briefs[0]
	hashes, receipts, unknown := Baselines(b, claims)
	if len(unknown) > 0 {
		t.Fatalf("fixture rests on unknown claims %v", unknown)
	}
	lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
		Path: b.Path, Hash: b.LockHash, At: "2026-09-30T00:00:00Z", Actor: "a", Reason: "approved",
		Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: b.RestsOn, Markdown: b.Body},
		Baselines: hashes, Receipts: receipts,
	})
	return store
}

// TestEvaluate_TheLockStatesAndTheirFindings is the lifecycle table: each row a
// state a brief can be in against its record, the lock state reported, whether
// it is review-pending, and exactly which findings it raises. It is the owner
// boundary for the four rules; check and the CLI carry them, and assert the
// carrying.
func TestEvaluate_TheLockStatesAndTheirFindings(t *testing.T) {
	claims := []model.Claim{claimFor("widget.contract.a", "a"), claimFor("widget.contract.b", "b")}
	moved := []model.Claim{claimFor("widget.contract.a", "a, rewritten"), claimFor("widget.contract.b", "b")}
	gone := []model.Claim{claimFor("widget.contract.a", "a")}

	for _, tc := range []struct {
		name      string
		status    string
		record    bool
		release   bool
		editBody  bool
		evalWith  []model.Claim
		wantState LockState
		pending   string
		want      []string // "rule path"
	}{
		{name: "draft, no record", status: "draft", evalWith: claims, wantState: LockDraft},
		{name: "draft with a record whose claims moved is never pending", status: "draft", record: true, evalWith: moved, wantState: LockDraft},
		{name: "locked and unchanged", status: "locked", record: true, evalWith: claims, wantState: LockLocked},
		{name: "locked, no record", status: "locked", evalWith: claims, wantState: LockUnrecorded, want: []string{"brief-unrecorded briefs/flow/overview.md"}},
		{name: "locked on a released record", status: "locked", record: true, release: true, evalWith: claims, wantState: LockUnrecorded, want: []string{"brief-unrecorded briefs/flow/overview.md"}},
		{name: "locked, body edited", status: "locked", record: true, editBody: true, evalWith: claims, wantState: LockEdited, want: []string{"brief-content-drift briefs/flow/overview.md"}},
		{name: "locked, a rests_on claim moved", status: "locked", record: true, evalWith: moved, wantState: LockLocked, pending: TriggerDependencyDrift, want: []string{"brief-dependency-drift briefs/flow/overview.md"}},
		{name: "locked, a rests_on claim is gone", status: "locked", record: true, evalWith: gone, wantState: LockLocked, pending: TriggerRestsOnMissing, want: []string{"brief-rests-on-missing briefs/flow/overview.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := lockedSet(t, tc.status)
			var store *lock.Store
			if tc.record {
				store = recordFor(t, set, claims)
				if tc.release {
					lock.ReleaseBriefApproval(store, set.Briefs[0].ID, lock.Approval{Actor: "a", Reason: "rework"})
				}
			} else {
				var err error
				if store, err = lock.LoadStore(filepath.Join(t.TempDir(), "s.json")); err != nil {
					t.Fatal(err)
				}
			}
			if tc.editBody {
				set.Briefs[0].Body += "More.\n"
				set.Briefs[0].LockHash = LockHash(set.Briefs[0].Summary, set.Briefs[0].RestsOn, set.Briefs[0].Body)
			}
			e := Evaluate(set, tc.evalWith, store)
			r := e.Review(set.Briefs[0])
			if r.LockState != tc.wantState {
				t.Fatalf("lock state = %q, want %q", r.LockState, tc.wantState)
			}
			if r.ReviewPending != (tc.pending != "") || r.ReviewPendingTrigger != tc.pending {
				t.Fatalf("review pending = %v %q, want %q", r.ReviewPending, r.ReviewPendingTrigger, tc.pending)
			}
			var got []string
			for _, f := range e.Integrity {
				got = append(got, f.Rule+" "+f.ClaimID)
			}
			// Only the lifecycle's own lint findings: Findings would add
			// brief-rests-on-unknown for the gone claim on a draft, which is
			// its own rule's business.
			for _, f := range e.Lint {
				got = append(got, f.LintName+" "+f.ClaimID)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEvaluate_AChangedClaimCarriesItsWordingThenAndNow pins what the viewer's
// per-claim redline (NIT-200) reads: the baseline wording from the brief's own
// receipt, the wording now, the two hashes; then, with the receipt gone and no
// other snapshot in the store, the "earlier wording not available" note and no
// invented wording.
func TestEvaluate_AChangedClaimCarriesItsWordingThenAndNow(t *testing.T) {
	claims := []model.Claim{claimFor("widget.contract.a", "a"), claimFor("widget.contract.b", "b")}
	set := lockedSet(t, "locked")
	store := recordFor(t, set, claims)
	moved := []model.Claim{claimFor("widget.contract.a", "a, rewritten"), claimFor("widget.contract.b", "b")}

	r := Evaluate(set, moved, store).Review(set.Briefs[0])
	if len(r.ChangedClaims) != 1 {
		t.Fatalf("changed claims = %+v", r.ChangedClaims)
	}
	c := r.ChangedClaims[0]
	if c.ID != "widget.contract.a" || c.Baseline == nil || c.Baseline.Body != "a" || c.Current == nil || c.Current.Body != "a, rewritten" ||
		c.BaselineHash == c.CurrentHash || c.BaselineNote != "" || c.ChangedAt != "" {
		t.Fatalf("changed claim = %+v", c)
	}

	rec, _ := store.BriefRecordFor(set.Briefs[0].ID)
	rec.Receipts = nil
	store.Briefs[set.Briefs[0].ID] = rec
	c = Evaluate(set, moved, store).Review(set.Briefs[0]).ChangedClaims[0]
	if c.Baseline != nil || c.BaselineNote != UnavailableWording {
		t.Fatalf("with nothing retained the wording must be reported unavailable, got %+v", c)
	}

	// Any other snapshot the store retains with that hash — here a claim's
	// dependency receipt — stands in for the brief's own.
	store.Receipts = map[string]map[string]lock.DependencyReceipt{
		"widget.contract.other": {"widget.contract.a": {Hash: lock.ContentHash(claims[0]), Content: claims[0]}},
	}
	c = Evaluate(set, moved, store).Review(set.Briefs[0]).ChangedClaims[0]
	if c.Baseline == nil || c.Baseline.Body != "a" || c.BaselineNote != "" {
		t.Fatalf("a snapshot retained elsewhere in the store must supply the wording, got %+v", c)
	}
}

// TestFindingsWith_AGoneBaselinedClaimIsOneFinding pins the hand-off between
// brief-rests-on-unknown and brief-rests-on-missing: the id a locked brief
// holds a baseline for is reported once, as missing; a draft brief resting on
// the same gone id is still unknown.
func TestFindingsWith_AGoneBaselinedClaimIsOneFinding(t *testing.T) {
	claims := []model.Claim{claimFor("widget.contract.a", "a"), claimFor("widget.contract.b", "b")}
	set := lockedSet(t, "locked")
	store := recordFor(t, set, claims)
	gone := []model.Claim{claimFor("widget.contract.a", "a")}

	got := rulesAndPaths(set.FindingsWith(gone, Evaluate(set, gone, store)))
	want := []string{"brief-rests-on-missing briefs/flow/overview.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("locked brief: %v, want %v", got, want)
	}
	draft := lockedSet(t, "draft")
	got = rulesAndPaths(draft.FindingsWith(gone, Evaluate(draft, gone, store)))
	want = []string{"brief-rests-on-unknown briefs/flow/overview.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("draft brief: %v, want %v", got, want)
	}
}

// TestLockHash_SignsWhatAReaderReadsAndNothingElse pins the hash's field set:
// summary, the rests_on set and the body move it; status, comments and the
// order of rests_on do not.
func TestLockHash_SignsWhatAReaderReadsAndNothingElse(t *testing.T) {
	base := LockHash("S.", []string{"a", "b"}, "Body.\n")
	for name, h := range map[string]string{
		"summary":  LockHash("S!", []string{"a", "b"}, "Body.\n"),
		"rests_on": LockHash("S.", []string{"a"}, "Body.\n"),
		"body":     LockHash("S.", []string{"a", "b"}, "Body!\n"),
	} {
		if h == base {
			t.Errorf("a change to %s must move the lock hash", name)
		}
	}
	if LockHash("S.", []string{"b", "a"}, "Body.\n") != base {
		t.Error("rests_on is a set: its order must not move the hash")
	}
	cfg := testConfig(t, t.TempDir(), "")
	draft := FromFiles(cfg, tree(map[string]File{"f/x.md": md("---\nsummary: S.\nstatus: draft\n---\nB.\n")})).Briefs[0]
	locked := FromFiles(cfg, tree(map[string]File{"f/x.md": md("---\nsummary: S.\nstatus: locked\ncomments:\n  - id: c-1\n    status: open\n    author: human\n    created: 2026-09-30T00:00:00Z\n    body: why?\n    edited: false\n---\nB.\n")})).Briefs[0]
	if draft.LockHash != locked.LockHash {
		t.Error("status and comments must not move the lock hash")
	}
	if locked.OpenThreads() != 1 {
		t.Errorf("open threads = %d, want 1", locked.OpenThreads())
	}
}
