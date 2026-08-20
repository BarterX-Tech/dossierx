// source_external_unanchored.go implements the "source-external-unanchored"
// lint: an external source must carry BOTH a url and an accessed_on, and the
// accessed_on must be a real ISO calendar date (YYYY-MM-DD).
//
// THE ACCESS DATE IS WHAT MAKES AN EXTERNAL CITATION FALSIFIABLE RATHER THAN
// MERELY LOCATABLE, and that sentence is the whole rule.
//
// A url alone tells a reader where to look. It does not tell them what they
// are looking for, and it cannot, because the thing at the other end is
// outside this repository and free to change without anything local moving.
// Six months after a claim is written, the reader who follows a bare url and
// finds a page that does not say what the claim says it said has learned
// nothing they can act on: they cannot tell whether the author misread it,
// whether the vendor changed it, or whether the url was always wrong. All
// three are live possibilities and the citation distinguishes none of them, so
// the honest reader is stuck and the hurried one assumes the author was right.
//
// "This page said X on 2026-08-01" is a different kind of statement. It is
// checkable against an archive, it dates the disagreement when the page has
// since changed, and — the part that matters for a gate — it can be REFUTED.
// A claim whose evidence cannot be shown to be wrong is not evidence; it is an
// assertion wearing a link. See model.SourceKind's doc comment, which draws
// the external/internal split along exactly this line: the two kinds are
// falsifiable in different ways, and this is external's way.
//
// WHY THE DATE MUST PARSE, and not merely be non-empty. A date the engine
// cannot read is a date no later automation can compare, sort, or age out, and
// "recently" or "Aug 2026" in that field records a feeling rather than a fact.
// The format is fixed at YYYY-MM-DD with no alternatives accepted, because a
// field that admits several spellings is a field every consumer has to
// re-parse defensively forever. The check is strict about SHAPE and then about
// the CALENDAR: "2026-02-30" has the right shape and is not a day, and a
// citation dated to a day that did not happen was not read on that day.
//
// This rule says nothing about whether the url is reachable. Nothing here
// makes a network request: a lint that did would fail differently on a plane
// than in CI, would make the gate's verdict depend on someone else's uptime,
// and — worst — a 200 response proves only that SOMETHING is there today, not
// that it is what was cited. The date is the durable half of the anchor;
// reachability is not the engine's to assert.
package lint

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, sourceExternalUnanchoredLint{})
}

type sourceExternalUnanchoredLint struct{}

// Name returns this lint's rule name.
func (sourceExternalUnanchoredLint) Name() string { return "source-external-unanchored" }

// isoDateShape is the fixed accessed_on spelling: four digits, two, two.
//
// The shape is checked with a regexp BEFORE time.Parse rather than left to it,
// because Go's date layouts accept a one-digit month or day ("2026-8-1" parses
// against "2006-01-02"). That leniency is fine for reading input from
// elsewhere and wrong for a field this project defines the spelling of: two
// spellings of the same day is one more thing every later consumer has to
// normalize.
var isoDateShape = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Check reports every external source missing url, missing accessed_on, or
// carrying an accessed_on that is not a real ISO calendar date.
//
// Sources of any other kind are not this rule's business: an internal source
// carrying a url is source-shape's cross-kind finding, and a source with no
// legal kind at all has no anchoring regime to be judged against yet.
func (sourceExternalUnanchoredLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		for i, s := range c.Sources {
			if !s.IsExternal() {
				continue
			}
			where := fmt.Sprintf("sources[%d]", i)
			add := func(msg string) {
				findings = append(findings, Finding{
					LintName: "source-external-unanchored",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  where + ": " + msg,
				})
			}

			if strings.TrimSpace(s.URL) == "" {
				add("external source has no url; there is nowhere for a reader to go and check it")
			}

			accessed := strings.TrimSpace(s.AccessedOn)
			switch {
			case accessed == "":
				add("external source has no accessed_on; the date is what makes the citation falsifiable rather than merely locatable — without it, a page that no longer says what this claim says it said is an argument instead of a finding")
			case !isoDateShape.MatchString(accessed):
				add(fmt.Sprintf("accessed_on %q is not an ISO date; it must be exactly YYYY-MM-DD, so that every later reader and every tool reads the same day out of it", s.AccessedOn))
			default:
				if _, err := time.Parse("2006-01-02", accessed); err != nil {
					add(fmt.Sprintf("accessed_on %q has the right shape but is not a real calendar date; the source cannot have been read on a day that did not happen", s.AccessedOn))
				}
			}
		}
	}
	return findings
}
