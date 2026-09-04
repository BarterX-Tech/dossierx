package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestDocument_TrackMembershipIsResolved pins the projection of a claim's
// cross-cutting membership into build/catalog/catalog.json, and above all that the ROLE is
// written out resolved rather than raw. A membership authored without a role
// means "cites", and a consumer of the index must never have to know that
// rule to read the file correctly — the same contract Entry.Kind holds for
// the overview-facet rule.
func TestDocument_TrackMembershipIsResolved(t *testing.T) {
	cat, err := Build([]model.Claim{{
		ID: "payment.contract.authorize", Facet: "contract", Module: "payment",
		Status: model.StatusDraft,
		Tracks: []model.TrackRef{
			{ID: "checkout", Role: model.TrackRoleOwns},
			{ID: "refunds", Role: model.TrackRoleCites},
			{ID: "reporting"}, // no role: means cites
		},
	}}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	doc := cat.Document()
	if len(doc.Claims) != 1 {
		t.Fatalf("len(doc.Claims) = %d, want 1", len(doc.Claims))
	}
	got := doc.Claims[0].Tracks

	want := []TrackMembership{
		{ID: "checkout", Role: model.TrackRoleOwns},
		{ID: "refunds", Role: model.TrackRoleCites},
		{ID: "reporting", Role: model.TrackRoleCites},
	}
	if len(got) != len(want) {
		t.Fatalf("Tracks = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Tracks[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Declaration order is preserved: the author's ordering is information,
	// and the catalog is meant to be a faithful projection rather than a
	// re-opinionated one.
	raw, err := json.Marshal(doc.Claims[0])
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if !strings.Contains(string(raw), `"tracks":[{"id":"checkout","role":"owns"}`) {
		t.Errorf("serialized entry does not lead with the owned track in declaration order:\n%s", raw)
	}
}

// TestDocument_TrackMembershipOmittedWhenAbsent is the zero-cost-when-unused
// half. A project that declares no tracks must produce a build/catalog/catalog.json
// byte-identical to the one it produced before tracks existed — an empty
// `"tracks":[]` on every entry would be a diff in every consuming project for
// a feature none of them use.
func TestDocument_TrackMembershipOmittedWhenAbsent(t *testing.T) {
	cat, err := Build([]model.Claim{{
		ID: "payment.contract.authorize", Facet: "contract", Module: "payment",
		Status: model.StatusDraft,
	}}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	raw, err := json.Marshal(cat.Document().Claims[0])
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if strings.Contains(string(raw), "tracks") {
		t.Errorf("a claim with no track membership serialized a tracks key:\n%s", raw)
	}
}

// TestDocument_SourcesAreNotProjected pins a deliberate omission, not an
// oversight. build/catalog/catalog.json already leaves out body, rows and steps because
// they are render concerns rather than catalog structure, and a claim's
// evidence sits on that same side of the line: it is read by a human on the
// claim, not resolved by a consumer of the index. If sources ever do belong
// here, that is a decision to make deliberately — this test is what forces
// the conversation.
func TestDocument_SourcesAreNotProjected(t *testing.T) {
	cat, err := Build([]model.Claim{{
		ID: "payment.contract.authorize", Facet: "contract", Module: "payment",
		Status: model.StatusDraft,
		Sources: []model.Source{{
			Ref: 1, Kind: model.SourceKindExternal, Title: "A page",
			URL: "https://example.invalid/a", AccessedOn: "2026-01-01",
		}},
	}}, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	raw, err := json.Marshal(cat.Document().Claims[0])
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	if strings.Contains(string(raw), "example.invalid") || strings.Contains(string(raw), `"sources"`) {
		t.Errorf("sources leaked into the catalog projection:\n%s", raw)
	}
}
