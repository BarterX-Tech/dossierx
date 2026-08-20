package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestSourceExternalUnanchored(t *testing.T) {
	mutate := func(f func(*model.Source)) []model.Source {
		s := wellFormedExternal()
		f(&s)
		return []model.Source{s}
	}

	cases := []struct {
		name      string
		sources   []model.Source
		wantCount int
		wantIn    string
	}{
		{
			name:      "passing: url and a real ISO date",
			sources:   []model.Source{wellFormedExternal()},
			wantCount: 0,
		},
		{
			name:      "passing: an internal source is not this rule's business",
			sources:   []model.Source{wellFormedInternal()},
			wantCount: 0,
		},
		{
			name:      "passing: a source with no legal kind has no anchoring regime yet",
			sources:   mutate(func(s *model.Source) { s.Kind = "api-reference"; s.AccessedOn = "" }),
			wantCount: 0,
		},
		{
			name:      "failing: no url",
			sources:   mutate(func(s *model.Source) { s.URL = "" }),
			wantCount: 1,
			wantIn:    "has no url",
		},
		{
			name:      "failing: no accessed_on",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "" }),
			wantCount: 1,
			wantIn:    "falsifiable rather than merely locatable",
		},
		{
			name:      "failing: neither anchor half",
			sources:   mutate(func(s *model.Source) { s.URL = ""; s.AccessedOn = "" }),
			wantCount: 2,
		},
		{
			name:      "failing: a prose date is not a date",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "August 2026" }),
			wantCount: 1,
			wantIn:    "is not an ISO date",
		},
		{
			name:      "failing: single-digit month and day are refused, not normalized",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "2026-8-1" }),
			wantCount: 1,
			wantIn:    "is not an ISO date",
		},
		{
			name:      "failing: a timestamp is not a date",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "2026-08-01T09:00:00Z" }),
			wantCount: 1,
			wantIn:    "is not an ISO date",
		},
		{
			name:      "failing: right shape, impossible day",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "2026-02-30" }),
			wantCount: 1,
			wantIn:    "not a real calendar date",
		},
		{
			name:      "failing: right shape, impossible month",
			sources:   mutate(func(s *model.Source) { s.AccessedOn = "2026-13-01" }),
			wantCount: 1,
			wantIn:    "not a real calendar date",
		},
	}

	l := sourceExternalUnanchoredLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := l.Check([]model.Claim{{ID: "widget.internals.fields", Sources: tc.sources}}, nil)
			if len(findings) != tc.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantCount, findings)
			}
			for _, f := range findings {
				if f.Severity != SeverityError {
					t.Errorf("Severity = %q, want error", f.Severity)
				}
			}
			if tc.wantIn != "" {
				joined := ""
				for _, f := range findings {
					joined += f.Message + "\n"
				}
				if !strings.Contains(joined, tc.wantIn) {
					t.Errorf("no finding mentions %q; got:\n%s", tc.wantIn, joined)
				}
			}
		})
	}
}
