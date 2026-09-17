// issues_view.go promotes the status strip's detail list into the Issues
// screen (docs/design/screens/04-issues-screen.md), while the strip itself
// stays the collapsed blocked banner on every reading view (02 §4.10/§4.19,
// 03 §4.5/§7.5). Grouping the promoted list by the module that owns the
// blocking claim (04 §2, §9 item 6) needs no server-rendered HTML today:
// viewer-runtime.js already computes it client-side, from in.cat.Readiness's
// own twin (the embedded graph payload, read back by offlineReadiness()) for
// a file:// export, and from GET /api/status's ledger_findings for a live
// serve — the same grouping, whether mounted or not (renderStatusStrip). The
// one piece of that arithmetic this package can and should own on the Go
// side is IssuesModuleWeight: 04 §6's "blocks <N> of <M> claims here" phrase
// and §2's per-module sort key are both "how many of this facet's claims are
// blocked", counted once here against catalog.Readiness rather than
// duplicated as untested template logic if a later lane renders any part of
// this screen server-side (04 §9 item 5's readiness/lint half is the
// candidate — the APPROVAL RECORD group is not, and never renders from a
// static build; see IssuesApprovalRecordUnavailableNotice).
package render

import "github.com/BarterX-Tech/dossierx/internal/readiness"

// IssuesModuleWeight counts, for one facet's claim id set, how many of those
// claims carry at least one dependency condition or review cause — 04 §2's
// per-module "weight" a group sorts by, and §6's numerator in
// "blocks <N> of <M> claims here". A claim id readiness has no entry for is
// simply not blocked and does not count. This mirrors, claim by claim, the
// same four-field check buildShellStaticData already runs project-wide for
// HasReadinessMaps (render.go) — this is the per-facet, per-claim form of
// that same test.
func IssuesModuleWeight(assessments map[string]readiness.Assessment, claimIDs map[string]bool) int {
	n := 0
	for id, in := range claimIDs {
		if !in {
			continue
		}
		assessment, ok := assessments[id]
		if !ok {
			continue
		}
		if len(assessment.DependencyConditions) > 0 || len(assessment.Conditions) > 0 ||
			len(assessment.ReviewCauses) > 0 || len(assessment.Causes) > 0 {
			n++
		}
	}
	return n
}

// IssuesApprovalRecordUnavailableNotice is the sentence 04 §8 item 11 / §9
// item 5 require when the Issues view has no live serve to source
// ledger_findings from: shell.html:265-271 already states the status strip
// itself exists only against a live serve, and a baked APPROVAL RECORD
// verdict would be exactly the "stale green strip nobody checked" that
// comment forbids. Its absence on a file:// export is stated, not silently
// omitted — viewer-runtime.js's renderStatusStrip renders this same copy
// into #statusStripBody when window.location.protocol is "file:". Kept as
// one exported constant so the two can be tested against each other instead
// of drifting as two hand-typed copies of one sentence.
const IssuesApprovalRecordUnavailableNotice = "Approval record findings need a live dossierx serve; open with dossierx serve to see them."
