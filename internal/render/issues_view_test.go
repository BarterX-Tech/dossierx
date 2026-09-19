package render

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/readiness"
)

// TestIssuesModuleWeightCountsOnlyBlockedClaimsInScope pins 04 §2's per-module
// weight and §6's "blocks <N> of <M> claims here" numerator: a claim outside
// the given id set never counts, a claim inside it with no readiness entry
// at all never counts, and a claim with any one of the four fact kinds
// counts exactly once regardless of how many facts it carries.
func TestIssuesModuleWeightCountsOnlyBlockedClaimsInScope(t *testing.T) {
	assessments := map[string]readiness.Assessment{
		"a": {DependencyConditions: []readiness.DependencyCondition{{Kind: "dependency_unapproved"}}},
		"b": {ReviewCauses: []readiness.Cause{{Kind: "own_thread"}, {Kind: "own_flag"}}},
		"c": {}, // present in the map, nothing blocking
		"d": {DependencyConditions: []readiness.DependencyCondition{{Kind: "dependency_unapproved"}}},
	}
	claimIDs := map[string]bool{"a": true, "b": true, "c": true, "d": false}

	got := IssuesModuleWeight(assessments, claimIDs)
	if got != 2 {
		t.Fatalf("IssuesModuleWeight() = %d, want 2 (a and b only)", got)
	}
}

func TestIssuesModuleWeightUnknownClaimDoesNotCount(t *testing.T) {
	got := IssuesModuleWeight(map[string]readiness.Assessment{}, map[string]bool{"unknown": true})
	if got != 0 {
		t.Fatalf("IssuesModuleWeight() = %d, want 0", got)
	}
}

// TestIssuesApprovalRecordNoticeMatchesRuntimeCopy holds the Go constant and
// viewer-runtime.js's own file:// notice (04 §8 item 11 / §9 item 5) to the
// same sentence, so the two never drift into two hand-typed copies of one
// rule: an APPROVAL RECORD verdict never bakes into a static file, and its
// absence there is stated, not silently omitted.
func TestIssuesApprovalRecordNoticeMatchesRuntimeCopy(t *testing.T) {
	b, err := shellFS.ReadFile("viewer/template/viewer-runtime.js")
	if err != nil {
		t.Fatalf("read embedded viewer-runtime.js: %v", err)
	}
	if !strings.Contains(string(b), IssuesApprovalRecordUnavailableNotice) {
		t.Fatalf("viewer-runtime.js does not contain IssuesApprovalRecordUnavailableNotice %q", IssuesApprovalRecordUnavailableNotice)
	}
}
