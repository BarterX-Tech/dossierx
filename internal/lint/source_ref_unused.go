// source_ref_unused.go implements the "source-ref-unused" lint: a sources[]
// entry whose ref no "[n]" marker in the claim's prose ever cites.
//
// It is the mirror of source-ref-undefined, and it is a WARNING where that one
// is an ERROR. The asymmetry is the point, so it is worth stating why rather
// than leaving a reader to infer it from the severity constant.
//
// An undefined marker is a FALSEHOOD: the claim asserts that a sentence is
// sourced and it is not. An uncited source is CLUTTER: the evidence exists, it
// is recorded, it is even checked (an uncited external source is still held to
// source-external-unanchored, an uncited internal one still to
// source-internal-drift) — it is simply not pointed at from anywhere in the
// prose. Nobody is misled by it. The likeliest causes are benign in both
// directions: a marker deleted during an edit and a source left behind, or a
// source added in preparation for prose not yet written.
//
// There is also a legitimate standing case for it, which is the second reason
// this is not an error. An author may record background evidence that informed
// the whole claim without any one sentence leaning on it — the "further
// reading" entry of a footnote list. The Supports field exists to let them say
// what it is for. A rule that made that arrangement impossible would push the
// author to invent a marker for a sentence that does not need one, which is a
// worse outcome than a warning nobody acts on.
//
// The warning still earns its place: the far more common case is the first
// one, and an orphaned entry is the visible residue of an edit that was not
// finished. Compare orphan.go, which draws the identical line for the same
// reason on the claim graph.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, sourceRefUnusedLint{})
}

type sourceRefUnusedLint struct{}

// Name returns this lint's rule name.
func (sourceRefUnusedLint) Name() string { return "source-ref-unused" }

// Check reports every sources[] entry whose ref appears in no "[n]" marker in
// the claim's prose.
//
// A ref is reported at most once per claim even when two entries share it:
// the duplicate itself is source-shape's finding, and repeating "nothing cites
// ref 2" for each half of a collision would describe one defect twice.
func (sourceRefUnusedLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if len(c.Sources) == 0 {
			continue
		}

		cited := make(map[int]bool)
		for _, ref := range citedSourceRefs(c.Body) {
			cited[ref] = true
		}

		reported := make(map[int]bool, len(c.Sources))
		for i, s := range c.Sources {
			if cited[s.Ref] || reported[s.Ref] {
				continue
			}
			reported[s.Ref] = true
			findings = append(findings, Finding{
				LintName: "source-ref-unused",
				ClaimID:  c.ID,
				Severity: SeverityWarning,
				Message: fmt.Sprintf(
					"sources[%d] declares ref %d but no [%d] marker in the body cites it; either cite it where the claim leans on it, or drop the entry",
					i, s.Ref, s.Ref),
			})
		}
	}
	return findings
}
