package lint

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func intPtr(n int) *int { return &n }

func TestSummaryRequiredLint(t *testing.T) {
	t.Run("missing and whitespace", func(t *testing.T) {
		claims := []model.Claim{
			{ID: "widget.contract.a"},
			{ID: "widget.contract.b", Summary: "   "},
		}
		findings := summaryRequiredLint{}.Check(claims, nil)
		if len(findings) != 2 {
			t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
		}
		for _, f := range findings {
			if f.LintName != "summary-required" || f.Severity != SeverityError {
				t.Fatalf("unexpected finding: %+v", f)
			}
		}
	})
	t.Run("newline is not a summary", func(t *testing.T) {
		findings := summaryRequiredLint{}.Check([]model.Claim{{ID: "w.contract.a", Summary: "one\ntwo"}}, nil)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "single line") {
			t.Fatalf("got %+v", findings)
		}
	})
	t.Run("markdown is not a summary", func(t *testing.T) {
		for _, s := range []string{"**bold**", "`code`", "[x](https://example.invalid)", "# heading", "- list"} {
			findings := summaryRequiredLint{}.Check([]model.Claim{{ID: "w.contract.a", Summary: s}}, nil)
			if len(findings) != 1 {
				t.Fatalf("%q: got %+v", s, findings)
			}
		}
	})
	t.Run("plain punctuation is allowed", func(t *testing.T) {
		findings := summaryRequiredLint{}.Check([]model.Claim{{
			ID: "w.contract.a", Summary: "Retry three times (with backoff).",
		}}, nil)
		if len(findings) != 0 {
			t.Fatalf("got %+v", findings)
		}
	})
	t.Run("no exemption for mockup or orientation", func(t *testing.T) {
		findings := summaryRequiredLint{}.Check([]model.Claim{
			{ID: "w.contract.mock", Layout: model.LayoutMockup},
			{ID: "w.doctrine.hub", Facet: "doctrine", BuildRole: model.BuildRoleOrientation},
		}, nil)
		if len(findings) != 2 {
			t.Fatalf("got %d findings, want 2", len(findings))
		}
	})
}

func TestSummaryOversizeLint(t *testing.T) {
	cfg := &config.Config{MaxClaimSummaryChars: intPtr(5)}
	ok := model.Claim{ID: "w.contract.ok", Summary: "short"}
	over := model.Claim{ID: "w.contract.over", Summary: "toolong"}
	findings := summaryOversizeLint{}.Check([]model.Claim{ok, over, {ID: "w.contract.missing"}}, cfg)
	if len(findings) != 1 || findings[0].ClaimID != "w.contract.over" || findings[0].LintName != "summary-oversize" {
		t.Fatalf("got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "7") || !strings.Contains(findings[0].Message, "5") {
		t.Fatalf("message must name count and cap: %q", findings[0].Message)
	}
}

func TestSummaryOversizeLint_CountsRunesNotBytes(t *testing.T) {
	cfg := &config.Config{MaxClaimSummaryChars: intPtr(3)}
	// three runes, six bytes
	ok := model.Claim{ID: "w.contract.ok", Summary: "世界外"}
	if n := utf8.RuneCountInString(ok.Summary); n != 3 {
		t.Fatalf("fixture: %d runes", n)
	}
	if findings := (summaryOversizeLint{}).Check([]model.Claim{ok}, cfg); len(findings) != 0 {
		t.Fatalf("three runes at cap 3: %+v", findings)
	}
	over := model.Claim{ID: "w.contract.over", Summary: "世界外。"}
	if findings := (summaryOversizeLint{}).Check([]model.Claim{over}, cfg); len(findings) != 1 {
		t.Fatalf("four runes over cap 3: %+v", findings)
	}
}

func TestBodyOversizeLint(t *testing.T) {
	cfg := &config.Config{MaxClaimBodyChars: intPtr(10)}
	ok := model.Claim{ID: "w.contract.ok", Body: "ten chars!"}
	over := model.Claim{ID: "w.contract.over", Body: "eleven char"}
	findings := bodyOversizeLint{}.Check([]model.Claim{ok, over}, cfg)
	if len(findings) != 1 || findings[0].ClaimID != "w.contract.over" || findings[0].LintName != "body-oversize" {
		t.Fatalf("got %+v", findings)
	}
}

func TestBodyOversizeLint_SumsStepsAndRowsAndExemptsRawHTML(t *testing.T) {
	cfg := &config.Config{MaxClaimBodyChars: intPtr(8)}
	// body 3 + step 3 + cell 3 = 9
	over := model.Claim{
		ID:    "w.contract.over",
		Body:  "aaa",
		Steps: []string{"bbb"},
		Rows:  []model.Row{{"col": "ccc"}},
	}
	findings := bodyOversizeLint{}.Check([]model.Claim{over}, cfg)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "9") {
		t.Fatalf("got %+v", findings)
	}
	markup := model.Claim{
		ID:      "w.contract.html",
		Body:    "ok",
		RawHTML: strings.Repeat("x", 4000),
	}
	if findings := (bodyOversizeLint{}).Check([]model.Claim{markup}, cfg); len(findings) != 0 {
		t.Fatalf("raw_html must not count: %+v", findings)
	}
}

func TestBodyOversizeLint_NilConfigUsesDefault(t *testing.T) {
	ok := model.Claim{ID: "w.contract.ok", Body: strings.Repeat("a", config.DefaultMaxClaimBodyChars)}
	over := model.Claim{ID: "w.contract.over", Body: strings.Repeat("a", config.DefaultMaxClaimBodyChars+1)}
	if n := len((bodyOversizeLint{}).Check([]model.Claim{ok}, nil)); n != 0 {
		t.Fatalf("at default cap: %d findings", n)
	}
	if n := len((bodyOversizeLint{}).Check([]model.Claim{over}, nil)); n != 1 {
		t.Fatalf("over default cap: %d findings", n)
	}
}

func TestSummaryAndBodyCapLintsRegistered(t *testing.T) {
	for _, name := range []string{"summary-required", "summary-oversize", "body-oversize"} {
		if !lintRegistered(name) {
			t.Fatalf("%s is not registered", name)
		}
	}
}
