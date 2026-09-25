// summary_required.go implements the "summary-required" lint: every claim
// needs a short, single-line, plain-text summary. Missing, whitespace-only,
// multiline, and markdown-shaped values are ERROR on draft and locked claims
// alike. There is no exemption: mockup claims and project claims need one too.
//
// Shape failures share this finding name on purpose: NIT-8 named exactly
// three lints (summary-required, summary-oversize, body-oversize). A
// summary that is not a plain line is not a summary.
package lint

import (
	"strings"
	"unicode"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, summaryRequiredLint{})
}

type summaryRequiredLint struct{}

// Name returns this lint's rule name.
func (summaryRequiredLint) Name() string { return "summary-required" }

func (summaryRequiredLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if msg := summaryRequiredMessage(c.Summary); msg != "" {
			findings = append(findings, Finding{
				LintName: "summary-required",
				ClaimID:  c.ID,
				Severity: SeverityError,
				Message:  msg,
			})
		}
	}
	return findings
}

func summaryRequiredMessage(summary string) string {
	if model.SummaryIsMissing(summary) {
		return "summary is required: a short single-line plain-text description so claim list can name the card"
	}
	if strings.ContainsAny(summary, "\n\r") {
		return "summary must be a single line of plain text, with no newline"
	}
	if summaryLooksLikeMarkdown(summary) {
		return "summary must be plain text with no markdown (no emphasis, links, headings, lists, or code spans)"
	}
	return ""
}

// summaryLooksLikeMarkdown is a closed, conservative detector for the
// constructs FORMAT.md's markdown ceiling would treat as markup. Ordinary
// punctuation (commas, parentheses, hyphens) is allowed.
func summaryLooksLikeMarkdown(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "`") || strings.Contains(trimmed, "**") || strings.Contains(trimmed, "__") {
		return true
	}
	if strings.Contains(trimmed, "](") || strings.Contains(trimmed, "![") {
		return true
	}
	if strings.Contains(trimmed, "<") && strings.Contains(trimmed, ">") {
		return true
	}
	runes := []rune(trimmed)
	if runes[0] == '#' || runes[0] == '>' {
		return true
	}
	if (runes[0] == '-' || runes[0] == '*' || runes[0] == '+') && len(runes) > 1 && unicode.IsSpace(runes[1]) {
		return true
	}
	return false
}
