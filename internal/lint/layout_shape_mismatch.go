// Lint layout-shape-mismatch catches an explicit layout that would silently
// drop a claim's own data at render time. Per
// internal/render/components/*.html, only table.html renders .Rows and
// only steps.html renders .Steps; every other component ignores both
// fields entirely. So:
//
//   - layout: table with no Rows renders an empty "No rows." placeholder —
//     the wrong component was chosen (or the data was never filled in).
//   - layout: steps with no Steps is the same problem for steps.
//   - any non-table layout that does carry Rows silently drops them.
//   - any non-steps layout that does carry Steps silently drops them.
//
// A claim with Layout == "" has not gone through catalog.Build's shape
// inference yet (lint.RunAll is also called directly against raw claims,
// e.g. from internal/lock.Lock); by construction, inference always picks a
// layout consistent with the claim's own shape, so an unset Layout can
// never itself be a mismatch and is skipped here rather than guessed at.
//
// Separately (and regardless of Layout), a claim carrying none of body,
// rows, or steps has no renderable content in any component — card.html
// has nothing to show, table.html falls back to "No rows.", etc. That is
// always a shape problem, so it is checked unconditionally rather than
// under the Layout=="" guard above.
//
// rows is deliberately checked for presence (c.Rows != nil), not for
// having entries (len(c.Rows) > 0), when deciding whether a table claim
// "has rows" or a claim "has content" at all. YAML distinguishes an
// omitted "rows" key (decodes to nil) from an explicit empty array
// ("rows: []", decodes to a non-nil, zero-length slice) — and so does this
// lint: an explicit empty array is valid, intentional data (table.html
// renders it as an explicit "No rows." state), while an omitted key on a
// layout: table claim is very likely a forgotten field. Only the
// "silently dropped" check below cares about len(c.Rows) > 0, since an
// empty array — present or not — has nothing to drop.
package lint

import (
	"fmt"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, layoutShapeMismatchLint{})
}

type layoutShapeMismatchLint struct{}

func (layoutShapeMismatchLint) Name() string { return "layout-shape-mismatch" }

// allLayouts is the fixed, ordered list of every layout
// internal/render/components knows how to render (see that package's map
// lookup on Layout). It is the single source of truth for both validLayouts
// (the membership set this lint checks against) and the human-readable list
// in the invalid-layout message below, so the two can never drift out of
// sync — and so the message is deterministic (a ranged map would order it
// randomly). LayoutMockup belongs here for the same reason as every other
// entry — components.fileForLayout already renders it (mockup.html); its own
// additional constraints (raw_html only legal here, module allowlist, markup
// allowlist, review gate) are enforced separately by the raw-html-scope
// lint, not by this shape check.
var allLayouts = []model.Layout{
	model.LayoutCard,
	model.LayoutTable,
	model.LayoutList,
	model.LayoutSteps,
	model.LayoutTree,
	model.LayoutBanner,
	model.LayoutMockup,
}

// validLayouts is allLayouts as a membership set. A claim.Layout outside it
// can never be rendered — without this check that would only surface at
// render time (internal/render.Render returns an "unsupported layout" error)
// instead of at claim-load/lint time; this lint is what "dossierx
// lint"/"dossierx lock" need to catch it before render ever sees it.
var validLayouts = func() map[model.Layout]bool {
	m := make(map[model.Layout]bool, len(allLayouts))
	for _, l := range allLayouts {
		m[l] = true
	}
	return m
}()

// layoutsList renders allLayouts as a comma-separated string for use in the
// invalid-layout message, derived from the same slice as validLayouts so the
// message can never omit or misorder a layout.
func layoutsList() string {
	names := make([]string, len(allLayouts))
	for i, l := range allLayouts {
		names[i] = string(l)
	}
	return strings.Join(names, ", ")
}

func (layoutShapeMismatchLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		rowsPresent := c.Rows != nil
		hasRows := len(c.Rows) > 0
		hasSteps := len(c.Steps) > 0

		if c.Body == "" && !rowsPresent && !hasSteps {
			findings = append(findings, mismatchFinding(c.ID, "claim has no content: body, rows, and steps are all empty"))
		}

		if c.Layout == "" {
			continue
		}

		if !validLayouts[c.Layout] {
			findings = append(findings, mismatchFinding(c.ID, fmt.Sprintf("layout %q is not one of the allowed layouts (%s)", c.Layout, layoutsList())))
			continue
		}

		switch {
		case c.Layout == model.LayoutTable && !rowsPresent:
			findings = append(findings, mismatchFinding(c.ID, "layout: table but rows is missing; table.html will render an empty \"No rows.\" placeholder"))
		case c.Layout == model.LayoutSteps && !hasSteps:
			findings = append(findings, mismatchFinding(c.ID, "layout: steps but steps is empty; steps.html will render an empty \"No steps.\" placeholder"))
		}

		if c.Layout != model.LayoutTable && hasRows {
			findings = append(findings, mismatchFinding(c.ID, fmt.Sprintf("layout: %s has rows set, but only layout: table renders rows; the data will be silently dropped", c.Layout)))
		}
		if c.Layout != model.LayoutSteps && hasSteps {
			findings = append(findings, mismatchFinding(c.ID, fmt.Sprintf("layout: %s has steps set, but only layout: steps renders steps; the data will be silently dropped", c.Layout)))
		}
	}
	return findings
}

func mismatchFinding(claimID, msg string) Finding {
	return Finding{
		LintName: "layout-shape-mismatch",
		ClaimID:  claimID,
		Message:  msg,
		Severity: SeverityError,
	}
}
