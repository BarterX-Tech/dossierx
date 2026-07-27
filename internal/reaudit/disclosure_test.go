// disclosure_test.go pins what a flag-sourced reaudit proposal has to TELL the
// human before they confirm it.
//
// The semantics are intended (a confirmed flag-sourced reaudit replaces the
// claim's whole body with --now-does; see ProposeFlagDiff). The disclosure was
// not there: the preview rendered a phrase-level diff and nothing else, so
// `claim reaudit --confirm --reason "confirmed with the human"` on a
// two-sentence locked claim silently deleted the second sentence and re-signed
// the truncated body into the ledger under words the human gave for a phrase
// swap they were shown.
package reaudit

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The reproduction, as a unit: a two-sentence locked body, a flag naming only
// the first sentence, and a preview that must say — before Apply is ever called
// — both what the body will BE and what is being lost.
func TestProposeFlagDiffDisclosesWholeBodyReplacement(t *testing.T) {
	claim := model.Claim{
		ID:            "widget.contract.overview",
		Status:        model.StatusLocked,
		ReviewPending: true,
		Body: "A widget is the smallest unit this fixture project documents.\n" +
			"It has a stable identity and a creation timestamp.",
	}
	flag := PendingFlag{
		ClaimSays: "the smallest unit this fixture project documents",
		NowDoes:   "the tiniest unit documented here",
		Reason:    "code changed",
	}

	diff, err := ProposeFlagDiff(claim, flag)
	if err != nil {
		t.Fatalf("ProposeFlagDiff: %v", err)
	}

	if diff.ResultingBody != flag.NowDoes {
		t.Fatalf("the preview must disclose the body Apply will write.\n got: %q\nwant: %q", diff.ResultingBody, flag.NowDoes)
	}
	if len(diff.SideEffects) == 0 {
		t.Fatalf("the preview must name the loss; SideEffects was empty (body=%q)", diff.Body)
	}
	joined := strings.Join(diff.SideEffects, "\n")
	if !strings.Contains(joined, "entire body") {
		t.Fatalf("the side effect must say the whole body is replaced, got %q", joined)
	}
	if !strings.Contains(joined, "will be removed") {
		t.Fatalf("the side effect must say content is removed, got %q", joined)
	}

	// And it must be the truth: Apply writes exactly what the preview promised.
	applied, err := Apply(claim, diff)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if applied.Body != diff.ResultingBody {
		t.Fatalf("preview and Apply disagree.\npreview: %q\napplied: %q", diff.ResultingBody, applied.Body)
	}
	if strings.Contains(applied.Body, "stable identity") {
		t.Fatalf("fixture precondition: the second sentence really is dropped, got %q", applied.Body)
	}
}

// The disclosure must not be noise. A body that is nothing BUT the flagged
// phrase loses nothing when it is replaced, and a side-effect line announcing a
// loss that does not happen is how disclosures become the thing people skip.
func TestProposeFlagDiffIsSilentWhenNothingIsLost(t *testing.T) {
	claim := model.Claim{
		ID:            "widget.contract.overview",
		Status:        model.StatusLocked,
		ReviewPending: true,
		Body:          "the smallest unit this fixture project documents",
	}
	flag := PendingFlag{
		ClaimSays: "the smallest unit this fixture project documents",
		NowDoes:   "the tiniest unit documented here",
		Reason:    "code changed",
	}

	diff, err := ProposeFlagDiff(claim, flag)
	if err != nil {
		t.Fatalf("ProposeFlagDiff: %v", err)
	}
	if len(diff.SideEffects) != 0 {
		t.Fatalf("nothing is lost here; expected no side effects, got %v", diff.SideEffects)
	}
	if diff.ResultingBody != flag.NowDoes {
		t.Fatalf("ResultingBody must still be disclosed, got %q", diff.ResultingBody)
	}
}
