// id_shape.go implements the "id-shape" lint: a claim's id must be exactly
// three dot-separated segments, module.facet.slug, where module is one of
// the project's configured modules, facet is one of the project's
// configured facets, those two segments agree with the claim's own Module
// and Facet fields, and slug is a non-empty kebab-case identifier.
package lint

import (
	"regexp"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, IDShapeLint{})
}

// IDShapeLint reports claims whose id does not follow the
// module.facet.slug grammar described in FORMAT.md.
type IDShapeLint struct{}

func (IDShapeLint) Name() string { return "id-shape" }

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (IDShapeLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	var modules, facets map[string]bool
	if cfg != nil {
		modules = toSet(cfg.Modules)
		facets = toSet(cfg.Facets)
	}

	var findings []Finding
	for _, c := range claims {
		segs := strings.Split(c.ID, ".")
		if len(segs) != 3 {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id must have exactly three dot-separated segments (module.facet.slug)",
			})
			continue
		}

		module, facet, slug := segs[0], segs[1], segs[2]

		if module == "" || facet == "" || slug == "" {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id segments must all be non-empty",
			})
			continue
		}

		if modules != nil && !modules[module] {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id module segment " + module + " is not in the project's configured modules",
			})
		}
		if facets != nil && !facets[facet] && facet != config.ReservedOverviewFacet {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id facet segment " + facet + " is not in the project's configured facets",
			})
		}
		if c.Module != "" && c.Module != module {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id module segment " + module + " does not match claim's module field " + c.Module,
			})
		}
		if c.Facet != "" && c.Facet != facet {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id facet segment " + facet + " does not match claim's facet field " + c.Facet,
			})
		}
		if !slugPattern.MatchString(slug) {
			findings = append(findings, Finding{
				LintName: "id-shape",
				ClaimID:  c.ID,
				Message:  "id slug segment " + slug + " must be kebab-case (lowercase alphanumerics separated by single hyphens)",
			})
		}
	}
	return findings
}

func toSet(ss []string) map[string]bool {
	set := make(map[string]bool, len(ss))
	for _, s := range ss {
		set[s] = true
	}
	return set
}
