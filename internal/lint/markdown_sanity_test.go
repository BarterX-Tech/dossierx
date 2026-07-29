package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestMarkdownSanity is the rule's table: one case per finding kind, plus the
// passing cases that keep the craft findings from becoming noise on ordinary
// claim prose.
func TestMarkdownSanity(t *testing.T) {
	cases := []struct {
		name       string
		claims     []model.Claim
		wantClaims []string
	}{
		{
			name: "passing: ordinary claim prose",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "A widget is the smallest unit; see `Render` and [the doc](https://example.com)."},
			},
			wantClaims: nil,
		},
		{
			name: "passing: the corpus's own vocabulary is not an emphasis run",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "Set governed_by, rests_on and review_pending on the claim."},
			},
			wantClaims: nil,
		},
		{
			name: "passing: a fenced example may contain anything",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "Example:\n\n```\n# heading\n*unclosed\n`span\n```\n"},
			},
			wantClaims: nil,
		},
		{
			name: "failing: unclosed fence",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "Example:\n\n```go\nfmt.Println(1)\n"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: unclosed code span",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "the ` character is special"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: unmatched emphasis run",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "this *never closes"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: dangling backslash",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "a line ending in a break marker\\"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: unresolvable list indentation",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "- first\n - second"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: malformed pipe table",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "| a | b |\n| --- |\n"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: reserved heading level",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "# Title\n"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: rejected link scheme",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "click [here](javascript:alert(1)) now"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: rejected image src",
			claims: []model.Claim{
				{ID: "a.contract.one", Body: "![flow](https://evil.example/p.png)"},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: image in a comment body",
			claims: []model.Claim{
				{
					ID: "a.contract.one",
					Comments: []model.Comment{
						{ID: "c1", Body: "look: ![shot](assets/x.png)"},
					},
				},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: image in a comment reply",
			claims: []model.Claim{
				{
					ID: "a.contract.one",
					Comments: []model.Comment{
						{ID: "c1", Body: "a question", Replies: []model.Reply{{ID: "r1", Body: "![shot](assets/x.png)"}}},
					},
				},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: a step carries the same defects a body would",
			claims: []model.Claim{
				{ID: "a.contract.one", Steps: []string{"run `go build"}},
			},
			wantClaims: []string{"a.contract.one"},
		},
		{
			name: "failing: a rejected scheme in a table cell",
			claims: []model.Claim{
				{
					ID:   "a.internals.fields",
					Rows: []model.Row{{"notes": "see [x](vbscript:x)"}},
				},
			},
			wantClaims: []string{"a.internals.fields"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MarkdownSanity{}.Check(tc.claims, nil)
			// Deduped: one malformed body legitimately trips more than one
			// finding kind (an unclosed fence's marker line falls through as
			// prose, where its backticks are then an unclosed code span — both
			// statements are true and the renderer emits both literally). The
			// table asserts WHICH claims are condemned; the severity and
			// message tables below assert what each finding says.
			assertStringSlicesEqual(t, dedupeStrings(findingClaimIDs(got)), tc.wantClaims)
		})
	}
}

// TestMarkdownSanity_SeveritySplit is the rule's most important test.
//
// lint.RunAll fills an unset Severity with SeverityError, and internal/lock.
// Lock refuses to lock when ANY error-severity finding exists anywhere in the
// corpus. So a craft finding that reached the default would mean one stray
// backtick in one draft froze every lock in the project — including the locks
// needed to go fix it. This asserts both halves of the gate-0 split, by
// message content, so that a future finding kind added to the wrong half fails
// here rather than in a consuming project.
func TestMarkdownSanity_SeveritySplit(t *testing.T) {
	craft := []struct {
		kind  string
		claim model.Claim
	}{
		{"unclosed code fence", model.Claim{ID: "a.contract.fence", Body: "```go\nx\n"}},
		{"unclosed code span", model.Claim{ID: "a.contract.span", Body: "the ` char"}},
		{"unmatched", model.Claim{ID: "a.contract.emph", Body: "this *never closes"}},
		{"dangling backslash", model.Claim{ID: "a.contract.slash", Body: "trailing\\"}},
		{"unresolvable list indentation", model.Claim{ID: "a.contract.indent", Body: "- a\n - b"}},
		{"malformed pipe table", model.Claim{ID: "a.contract.table", Body: "| a | b |\n| --- |\n"}},
		{"reserved heading level", model.Claim{ID: "a.contract.head", Body: "# Title\n"}},
	}
	for _, tc := range craft {
		t.Run("warning/"+tc.kind, func(t *testing.T) {
			assertOnlySeverity(t, MarkdownSanity{}.Check([]model.Claim{tc.claim}, nil), tc.kind, SeverityWarning)
		})
	}

	security := []struct {
		kind  string
		claim model.Claim
	}{
		{"rejected link scheme", model.Claim{ID: "a.contract.link", Body: "[x](javascript:alert(1))"}},
		{"rejected image src", model.Claim{ID: "a.contract.img", Body: "![x](//evil.example/p.png)"}},
		{"image in a comment body", model.Claim{
			ID:       "a.contract.cmt",
			Comments: []model.Comment{{ID: "c1", Body: "![x](assets/y.png)"}},
		}},
	}
	for _, tc := range security {
		t.Run("error/"+tc.kind, func(t *testing.T) {
			assertOnlySeverity(t, MarkdownSanity{}.Check([]model.Claim{tc.claim}, nil), tc.kind, SeverityError)
		})
	}
}

// TestMarkdownSanity_NoFindingRelisOnTheDefaultSeverity holds the rule to
// setting Severity on EVERY finding it builds, rather than letting RunAll's
// normalization decide. A finding that arrived here empty would be silently
// promoted to error, which is the exact failure mode the split exists to
// prevent, and it would be invisible in any test that only reads RunAll's
// output.
func TestMarkdownSanity_NoFindingRelisOnTheDefaultSeverity(t *testing.T) {
	claims := []model.Claim{
		{
			ID:       "a.contract.everything",
			Body:     "# Title\n\n```go\nx\n\nthe ` char and *an opener and [x](javascript:1) and ![y](//evil/p.png)\n\n- a\n - b\n\ntrailing\\",
			Steps:    []string{"run `go build"},
			Rows:     []model.Row{{"notes": "see [x](vbscript:x)"}},
			Comments: []model.Comment{{ID: "c1", Body: "![z](assets/z.png)"}},
		},
	}
	findings := MarkdownSanity{}.Check(claims, nil)
	if len(findings) == 0 {
		t.Fatal("expected findings from the kitchen-sink fixture, got none")
	}
	for _, f := range findings {
		if f.Severity != SeverityWarning && f.Severity != SeverityError {
			t.Errorf("finding %q has severity %q; markdown-sanity must set it explicitly on every finding", f.Message, f.Severity)
		}
		if f.LintName != "markdown-sanity" {
			t.Errorf("finding %q carries lint name %q", f.Message, f.LintName)
		}
	}
}

// TestMarkdownSanity_Deterministic guards the one place map iteration could
// leak into a report: Rows is a map, and lint output is compared byte-for-byte
// by the CLI's JSON envelope.
func TestMarkdownSanity_Deterministic(t *testing.T) {
	claims := []model.Claim{
		{
			ID: "a.internals.fields",
			Rows: []model.Row{{
				"alpha": "[x](javascript:1)",
				"beta":  "[y](data:text/html,x)",
				"gamma": "[z](vbscript:x)",
			}},
		},
	}
	first := messagesOf(MarkdownSanity{}.Check(claims, nil))
	for i := 0; i < 20; i++ {
		if got := messagesOf((MarkdownSanity{}).Check(claims, nil)); !equalStrings(got, first) {
			t.Fatalf("markdown-sanity output is not deterministic:\n%v\nvs\n%v", got, first)
		}
	}
}

// TestMarkdownSanity_CommentsAreImageOnly pins the surface decision: a
// reviewer's stray backtick is not the claim author's defect, so comment
// bodies get the image check and nothing else.
func TestMarkdownSanity_CommentsAreImageOnly(t *testing.T) {
	claims := []model.Claim{
		{
			ID: "a.contract.one",
			Comments: []model.Comment{
				{ID: "c1", Body: "the ` char, an *opener, a trailing backslash\\ and a # heading"},
			},
		},
	}
	if got := (MarkdownSanity{}).Check(claims, nil); len(got) != 0 {
		t.Fatalf("expected no findings on a comment body's craft defects, got %+v", got)
	}
}

func assertOnlySeverity(t *testing.T, findings []Finding, kindSubstring string, want Severity) {
	t.Helper()
	saw := false
	for _, f := range findings {
		if !strings.Contains(f.Message, kindSubstring) {
			continue
		}
		saw = true
		if f.Severity != want {
			t.Errorf("finding %q: got severity %q, want %q", f.Message, f.Severity, want)
		}
	}
	if !saw {
		t.Fatalf("expected a finding mentioning %q, got %+v", kindSubstring, findings)
	}
}

func messagesOf(findings []Finding) []string {
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = f.Message
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMarkdownSanity_CraftFindingsDoNotBlockALock is the end-to-end statement
// of why the split exists, asserted through the exact path internal/lock.Lock
// takes: RunAll over the whole corpus, then a count of every non-warning
// finding. A body full of craft defects must leave that count at zero, or the
// first stray backtick in any draft in a project freezes every lock in it —
// including the locks the author needs in order to go fix it.
func TestMarkdownSanity_CraftFindingsDoNotBlockALock(t *testing.T) {
	cfg := &config.Config{
		SchemaVersion: 1,
		Facets:        []string{"contract"},
		Modules:       []string{"widget"},
		ClaimsDir:     "claims",
	}
	claims := []model.Claim{
		{
			ID:     "widget.contract.anchor",
			Facet:  "contract",
			Module: "widget",
			Status: model.StatusDraft,
			Layout: model.LayoutCard,
			Body:   "An anchor claim with nothing wrong with it.",
			Governed: model.Governed{
				Type:   string(model.GovernedNone),
				Reason: "fixture claim, not backed by any real doctrine",
			},
		},
		{
			ID:     "widget.contract.crafty",
			Facet:  "contract",
			Module: "widget",
			Status: model.StatusDraft,
			Layout: model.LayoutCard,
			Body: "## a reserved heading\n\n" +
				"a ` that never closes, an *opener with no partner, and a line\n" +
				"that ends in a break marker with nothing to break to\\\n\n" +
				"- a\n - b\n",
			RestsOn: []string{"widget.contract.anchor"},
			Governed: model.Governed{
				Type:   string(model.GovernedNone),
				Reason: "fixture claim, not backed by any real doctrine",
			},
		},
	}

	var sawCraft bool
	var errors []Finding
	for _, f := range RunAll(claims, cfg) {
		if f.LintName == "markdown-sanity" {
			sawCraft = true
		}
		if f.Severity != SeverityWarning {
			errors = append(errors, f)
		}
	}
	if !sawCraft {
		t.Fatal("expected markdown-sanity to fire on the crafty claim")
	}
	if len(errors) != 0 {
		t.Fatalf("craft findings must never reach error severity through RunAll; got %+v", errors)
	}
}
