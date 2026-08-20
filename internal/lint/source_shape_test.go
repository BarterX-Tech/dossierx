package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// wellFormedExternal and wellFormedInternal are the two shapes this rule must
// stay silent on. Every failing case below is one of them with exactly one
// field disturbed, so a test that goes red names the disturbance.
func wellFormedExternal() model.Source {
	return model.Source{
		Ref:        1,
		Kind:       model.SourceKindExternal,
		Title:      "Vendor rate limit reference",
		URL:        "https://example.com/docs/rate-limits",
		AccessedOn: "2026-08-01",
	}
}

func wellFormedInternal() model.Source {
	return model.Source{
		Ref:    1,
		Kind:   model.SourceKindInternal,
		Title:  "Extraction ledger",
		Path:   "sources/ledger.jsonl",
		SHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	}
}

func TestSourceShape(t *testing.T) {
	mutate := func(s model.Source, f func(*model.Source)) []model.Source {
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
			name:      "passing: well-formed external",
			sources:   []model.Source{wellFormedExternal()},
			wantCount: 0,
		},
		{
			name:      "passing: well-formed internal",
			sources:   []model.Source{wellFormedInternal()},
			wantCount: 0,
		},
		{
			name:      "passing: internal without record_id may point at any extension",
			sources:   mutate(wellFormedInternal(), func(s *model.Source) { s.Path = "sources/decision-log.md" }),
			wantCount: 0,
		},
		{
			name:      "passing: distinct refs on one claim",
			sources:   []model.Source{wellFormedExternal(), mutate(wellFormedExternal(), func(s *model.Source) { s.Ref = 2 })[0]},
			wantCount: 0,
		},
		{
			name:      "failing: ref zero",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Ref = 0 }),
			wantCount: 1,
			wantIn:    "not positive",
		},
		{
			name:      "failing: negative ref",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Ref = -3 }),
			wantCount: 1,
			wantIn:    "not positive",
		},
		{
			name:      "failing: two entries share a ref",
			sources:   []model.Source{wellFormedExternal(), wellFormedExternal()},
			wantCount: 1,
			wantIn:    "already used by sources[0]",
		},
		{
			name:      "failing: empty title",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Title = "" }),
			wantCount: 1,
			wantIn:    "title is required",
		},
		{
			name:      "failing: whitespace-only title",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Title = "   " }),
			wantCount: 1,
			wantIn:    "title is required",
		},
		{
			name:      "failing: missing kind",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Kind = "" }),
			wantCount: 1,
			wantIn:    "kind is required",
		},
		{
			name:      "failing: kind outside the closed enum",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Kind = "api-reference" }),
			wantCount: 1,
			wantIn:    `invalid kind "api-reference"`,
		},
		{
			name:      "failing: external carrying an internal anchor",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.SHA256 = "abc" }),
			wantCount: 1,
			wantIn:    "kind external must not set sha256",
		},
		{
			name:      "failing: external carrying a path",
			sources:   mutate(wellFormedExternal(), func(s *model.Source) { s.Path = "sources/x.md" }),
			wantCount: 1,
			wantIn:    "kind external must not set path",
		},
		{
			name:      "failing: internal carrying an external anchor",
			sources:   mutate(wellFormedInternal(), func(s *model.Source) { s.URL = "https://example.com/x" }),
			wantCount: 1,
			wantIn:    "kind internal must not set url",
		},
		{
			name:      "failing: internal carrying an accessed_on",
			sources:   mutate(wellFormedInternal(), func(s *model.Source) { s.AccessedOn = "2026-08-01" }),
			wantCount: 1,
			wantIn:    "kind internal must not set accessed_on",
		},
		{
			name:      "failing: internal with no path",
			sources:   mutate(wellFormedInternal(), func(s *model.Source) { s.Path = "" }),
			wantCount: 1,
			wantIn:    "kind internal requires path",
		},
		{
			name: "failing: record_id on a non-JSONL path",
			sources: mutate(wellFormedInternal(), func(s *model.Source) {
				s.Path = "sources/decision-log.md"
				s.RecordID = "REQ-17"
			}),
			wantCount: 1,
			wantIn:    "is not .jsonl",
		},
		{
			name: "passing: record_id on a JSONL path, extension case-insensitive",
			sources: mutate(wellFormedInternal(), func(s *model.Source) {
				s.Path = "sources/ledger.JSONL"
				s.RecordID = "REQ-17"
			}),
			wantCount: 0,
		},
		{
			name: "failing: two independent defects on one entry are two findings",
			sources: mutate(wellFormedExternal(), func(s *model.Source) {
				s.Ref = 0
				s.Title = ""
			}),
			wantCount: 2,
		},
	}

	l := sourceShapeLint{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := []model.Claim{{ID: "widget.internals.fields", Sources: tc.sources}}
			findings := l.Check(claims, nil)
			if len(findings) != tc.wantCount {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantCount, findings)
			}
			for _, f := range findings {
				if f.Severity != SeverityError {
					t.Errorf("Severity = %q, want error", f.Severity)
				}
				if f.LintName != "source-shape" {
					t.Errorf("LintName = %q", f.LintName)
				}
				if f.ClaimID != "widget.internals.fields" {
					t.Errorf("ClaimID = %q", f.ClaimID)
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

// TestSourceShapeEmptyInput pins the Lint contract's "must not panic on empty
// claims" clause for the whole family's most field-heavy rule.
func TestSourceShapeEmptyInput(t *testing.T) {
	if f := (sourceShapeLint{}).Check(nil, nil); len(f) != 0 {
		t.Fatalf("expected no findings over no claims, got %+v", f)
	}
	if f := (sourceShapeLint{}).Check([]model.Claim{{ID: "widget.internals.fields"}}, nil); len(f) != 0 {
		t.Fatalf("a claim with no sources must produce nothing, got %+v", f)
	}
}
