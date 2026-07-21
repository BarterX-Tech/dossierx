// This file holds small helpers shared by more than one of the lint files
// in this package (claim lookup by id, and extraction of claim-id-shaped
// tokens from free text). It intentionally implements no Lint itself.
package lint

import (
	"regexp"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// claimByID returns the claim with the given id, if any.
func claimByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}

// idShapedToken matches the on-disk id grammar (FORMAT.md): three
// dot-separated, kebab-case-ish segments. It is deliberately loose at the
// regex level (module/facet membership is checked separately against
// config) so it doesn't need to know anything project-specific itself.
var idShapedToken = regexp.MustCompile(`\b[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)

// codeFence matches a fenced ```...``` code block, across lines.
var codeFence = regexp.MustCompile("(?s)```.*?```")

// extractCandidateIDs finds every substring of text that is shaped like a
// claim id (module.facet.slug) AND whose module/facet segments are both
// declared in cfg — this ties the heuristic to the project's own
// vocabulary instead of matching arbitrary dotted text (version numbers,
// IP-shaped strings, etc.) that happens to contain two dots.
func extractCandidateIDs(text string, cfg *config.Config) []string {
	var out []string
	for _, tok := range idShapedToken.FindAllString(text, -1) {
		parts := strings.SplitN(tok, ".", 3)
		if len(parts) != 3 {
			continue
		}
		if contains(cfg.Modules, parts[0]) && contains(cfg.Facets, parts[1]) {
			out = append(out, tok)
		}
	}
	return out
}

// splitFencedAndProse separates a claim's Body into the text inside fenced
// code blocks and the remaining prose (fences stripped out), so lints can
// scan each half separately (code-orphan looks inside fences,
// body-edge-hint looks outside them).
func splitFencedAndProse(body string) (fenced, prose string) {
	var fencedParts []string
	for _, m := range codeFence.FindAllString(body, -1) {
		fencedParts = append(fencedParts, m)
	}
	fenced = strings.Join(fencedParts, "\n")
	prose = codeFence.ReplaceAllString(body, "")
	return fenced, prose
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func dedupeStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
