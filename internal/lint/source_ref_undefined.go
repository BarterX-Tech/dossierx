// source_ref_undefined.go implements the "source-ref-undefined" lint: a
// "[n]" citation marker in a claim's prose that no sources[] entry answers to.
//
// This is the dangling lint of the citation graph, and the harm is the same
// shape as a dangling edge's but lands on a different reader. A dangling
// rests_on breaks a tool; an unresolved marker breaks a person. The viewer
// renders "[3]" as a citation the reader is invited to follow to the footer,
// and the footer has no entry 3 — so the sentence carries the visual authority
// of a cited claim while resting on nothing at all. The reader who checks
// finds an empty link; the far more common reader, who does not check, simply
// believes the sentence was sourced.
//
// WHY THE PROSE SCAN IS GATED ON THE CLAIM DECLARING SOURCES. "[n]" is not a
// rare string in technical writing — "argv[0]", "buckets[3]", "the second
// element, rows[1]" are ordinary prose. citedSourceRefs already excludes
// fenced blocks and inline code spans, which is where most of those live, but
// not all of them: an author writing about array indexing in running text
// would trip this rule on every sentence. So the marker syntax is only in
// force for a claim that declares at least one source. A claim with no
// sources[] cites nothing, and nothing it writes can be a citation.
//
// The cost of that gate is stated plainly rather than hidden: a claim that
// declares one source and writes "array[0]" in prose DOES report here, and the
// author's remedy is to put the index in a code span, where it belonged
// anyway. That trade was taken because the alternative — never recognising a
// marker unless it happens to resolve — would make this rule incapable of
// reporting the one thing it exists to report.
//
// Severity is ERROR. A marker pointing at nothing is a claim asserting a
// provenance it does not have, and the consequence is a reader who acts on an
// unsourced sentence believing it was sourced.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, sourceRefUndefinedLint{})
}

type sourceRefUndefinedLint struct{}

// Name returns this lint's rule name.
func (sourceRefUndefinedLint) Name() string { return "source-ref-undefined" }

// Check reports every "[n]" marker in a sourced claim's prose whose n matches
// no sources[] entry's ref.
func (sourceRefUndefinedLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if len(c.Sources) == 0 {
			// Not a sourced claim: the marker syntax is not in force here, so
			// there is nothing to resolve rather than something unresolved.
			// See this file's doc comment.
			continue
		}

		declared := make(map[int]bool, len(c.Sources))
		for _, s := range c.Sources {
			declared[s.Ref] = true
		}

		for _, ref := range citedSourceRefs(c.Body) {
			if declared[ref] {
				continue
			}
			findings = append(findings, Finding{
				LintName: "source-ref-undefined",
				ClaimID:  c.ID,
				Severity: SeverityError,
				Message: fmt.Sprintf(
					"body cites [%d] but no sources[] entry has ref %d; the viewer renders that marker as a citation the reader can follow, and it leads nowhere",
					ref, ref),
			})
		}
	}
	return findings
}
