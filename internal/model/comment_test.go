package model

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenThreadPredicates_TruthTable(t *testing.T) {
	cases := []struct {
		name     string
		comments []Comment
		wantIDs  []string
		wantHas  bool
	}{
		{
			name:     "zero-value claim has no open threads",
			comments: nil,
			wantIDs:  nil,
			wantHas:  false,
		},
		{
			name:     "empty comment slice has no open threads",
			comments: []Comment{},
			wantIDs:  nil,
			wantHas:  false,
		},
		{
			name:     "single open thread",
			comments: []Comment{{ID: "c-000001", Status: CommentStatusOpen}},
			wantIDs:  []string{"c-000001"},
			wantHas:  true,
		},
		{
			name:     "single resolved thread is not open",
			comments: []Comment{{ID: "c-000001", Status: CommentStatusResolved}},
			wantIDs:  nil,
			wantHas:  false,
		},
		{
			name: "mixed open and resolved returns only open, in order",
			comments: []Comment{
				{ID: "c-aaaaaa", Status: CommentStatusResolved},
				{ID: "c-bbbbbb", Status: CommentStatusOpen},
				{ID: "c-cccccc", Status: CommentStatusResolved},
				{ID: "c-dddddd", Status: CommentStatusOpen},
			},
			wantIDs: []string{"c-bbbbbb", "c-dddddd"},
			wantHas: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Claim{ID: "widget.contract.a", Comments: tc.comments}
			gotIDs := c.OpenThreadIDs()
			if !reflect.DeepEqual(gotIDs, tc.wantIDs) {
				t.Errorf("OpenThreadIDs() = %#v, want %#v", gotIDs, tc.wantIDs)
			}
			if got := c.HasOpenThreads(); got != tc.wantHas {
				t.Errorf("HasOpenThreads() = %v, want %v", got, tc.wantHas)
			}
			// The two predicates must never disagree.
			if (len(gotIDs) > 0) != c.HasOpenThreads() {
				t.Errorf("OpenThreadIDs/HasOpenThreads disagree: ids=%v has=%v", gotIDs, c.HasOpenThreads())
			}
		})
	}
}

// TestComment_YAMLRoundTrip proves the plain struct tags round-trip a fully
// populated thread (with a reply and resolve/reopen metadata) through
// Marshal->Unmarshal without a custom marshaler, and that a fresh thread
// stays minimal (no empty replies/resolved_by keys) while edited:false is
// still written verbatim.
func TestComment_YAMLRoundTrip(t *testing.T) {
	orig := Comment{
		ID:      "c-8f3a2b",
		Status:  CommentStatusResolved,
		Author:  CommentRoleHuman,
		Created: "2026-07-24T10:12:00Z",
		Body:    "This row contradicts the API facet — which is right?",
		Edited:  false,
		Replies: []Reply{{
			ID:      "r-4c9e11",
			Author:  CommentRoleAgent,
			Created: "2026-07-24T10:40:00Z",
			Body:    "Fixed the rows; API facet was stale.",
			Edited:  true,
		}},
		ResolvedBy: CommentRoleHuman,
		ResolvedAt: "2026-07-24T11:02:00Z",
	}
	raw, err := yaml.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Comment
	if err := yaml.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, orig) {
		t.Fatalf("round-trip mismatch:\n got: %#v\nwant: %#v\nyaml:\n%s", got, orig, raw)
	}
}

func TestComment_FreshThreadOmitsLifecycleKeys(t *testing.T) {
	fresh := Comment{
		ID:      "c-000001",
		Status:  CommentStatusOpen,
		Author:  CommentRoleAgent,
		Created: "2026-07-24T10:12:00Z",
		Body:    "why does this rest on the loader?",
		Edited:  false,
	}
	raw, err := yaml.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(raw)
	for _, key := range []string{"replies:", "resolved_by:", "resolved_at:", "reopened_by:", "reopened_at:"} {
		if contains(out, key) {
			t.Errorf("fresh thread should omit %q, got:\n%s", key, out)
		}
	}
	// edited: false is written verbatim (not omitempty), matching FORMAT.md.
	if !contains(out, "edited: false") {
		t.Errorf("expected edited: false to be written verbatim, got:\n%s", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
