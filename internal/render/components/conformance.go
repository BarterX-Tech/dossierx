package components

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/conformance"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// ConformanceHTML renders one claim-level result with its independently
// addressable checks. Every project-supplied value is escaped; the returned
// template.HTML contains only engine-authored structure.
//
// Structure follows docs/design/screens/06a-how-this-was-checked-inline-disclosure.md:
// above a "line of authorship" a reviewer-facing sentence, an EXAMINED /
// COMPARED / FOUND label column and (for a set-shaped check) a marked
// coverage strip (R12.3 — tinting marks the compared members, order carries
// no meaning); below the line, behind a nested "How this was checked"
// disclosure that is closed by default and never auto-opens (R09.3), the
// engine's own nine-key envelope, verbatim, in lowercase mono (06a §4.8/§6).
func ConformanceHTML(result conformance.Result) template.HTML {
	var b strings.Builder
	claimID := html.EscapeString(result.ClaimID)
	b.WriteString(`<details class="claim-conformance" name="claim-footer-`)
	// The claim-footer name group is what gives this panel R09.2's "exactly
	// one expansion open at a time" for free, joining the relationships/
	// sources doors components.go already emits with the same
	// name="claim-footer-<id>" attribute (see that file's doc comment on
	// enqueueEdges/claim-footer, and VAULT G15). The disclosure-group
	// mechanism keys purely on the name string, not DOM position, so this
	// panel does not need to move to become a member.
	b.WriteString(claimID)
	b.WriteString(`" data-conformance-mode="`)
	b.WriteString(html.EscapeString(string(result.Mode)))
	b.WriteString(`" data-claim-id="`)
	b.WriteString(claimID)
	b.WriteString(`" data-implementation-ready="`)
	b.WriteString(fmt.Sprintf("%t", result.ImplementationReady))
	if result.Mode == model.EmbodimentModeNone {
		b.WriteString(`" data-conformance-state="declared_none`)
	}
	b.WriteString(`"`)
	if !result.ImplementationReady {
		b.WriteString(` open`)
	}
	b.WriteString(`><summary class="claim-conformance-head"><strong>Implementation checks</strong> <span class="pill `)
	if result.ImplementationReady {
		b.WriteString(`ps">Ready`)
	} else {
		b.WriteString(`pw">Not ready`)
	}
	b.WriteString(`</span></summary>`)

	if result.Mode == model.EmbodimentModeNone {
		// 07 §6's closing note is a rewrite of the scope sentence, not a
		// reuse of it: this is board 07's territory ("Claim — boundary, no
		// embodiment"), and 06a §8.4 requires the disclosure not render at
		// all here — there is no check article, so no nested "How this was
		// checked" for this branch.
		b.WriteString(`<p class="claim-conformance-scope">Ready here means this claim deliberately declares no software embodiment — not that something was checked and passed.</p>`)
		writeConformanceLine(&b, "declaration", "none")
		writeConformanceLine(&b, "reason", result.Reason)
		b.WriteString(`</details>`)
		return template.HTML(b.String()) //nolint:gosec // all values escaped above
	}

	b.WriteString(`<p class="claim-conformance-scope">Ready here means every declared check matches. Claim and release readiness remain separate.</p>`)
	b.WriteString(`<div class="claim-conformance-eyebrow"><span class="claim-conformance-eyebrow-label">Implementation checks</span><span class="claim-conformance-eyebrow-count">`)
	b.WriteString(fmt.Sprintf("%d", len(result.Checks)))
	b.WriteString(`</span></div>`)
	for _, check := range result.Checks {
		writeConformanceCheck(&b, check)
	}
	b.WriteString(`</details>`)
	return template.HTML(b.String()) //nolint:gosec // all values escaped above
}

func writeConformanceCheck(b *strings.Builder, check conformance.CheckResult) {
	state := string(check.State)
	b.WriteString(`<article class="claim-conformance-check" data-check-id="`)
	b.WriteString(html.EscapeString(check.ID))
	b.WriteString(`" data-conformance-state="`)
	b.WriteString(html.EscapeString(state))
	b.WriteString(`" data-shape="`)
	b.WriteString(html.EscapeString(string(check.Shape)))
	b.WriteString(`" data-implementation-ready="`)
	b.WriteString(fmt.Sprintf("%t", check.ImplementationReady))
	b.WriteString(`"><div class="claim-conformance-check-head"><span class="claim-conformance-check-title">`)
	b.WriteString(html.EscapeString(check.ID))
	b.WriteString(`</span><span class="claim-conformance-verdict"><span class="claim-conformance-verdict-dot" aria-hidden="true"></span>`)
	b.WriteString(html.EscapeString(conformanceStateLabel(check.State)))
	b.WriteString(`</span></div>`)

	// The reviewer-facing explanation sentence — serif, above the line
	// (06a §2 "the typography is the argument"). Matched carries no Reason
	// (internal/conformance's evaluateCheck never sets one on a match), so
	// nothing renders for it; every other state's Reason is one sentence.
	if check.Reason != "" {
		b.WriteString(`<p class="claim-conformance-explain">`)
		b.WriteString(html.EscapeString(check.Reason))
		b.WriteString(`</p>`)
	}

	writeConformanceReviewerRow(b, "EXAMINED", check.Target)
	writeConformanceReviewerRow(b, "COMPARED", check.Adapter)
	writeConformanceFoundRow(b, check)

	writeConformanceDisclosure(b, check)
	b.WriteString(`</article>`)
}

// writeConformanceReviewerRow emits one row of the EXAMINED / COMPARED
// label column (06a §4.4, §6). These two rows are reviewer-facing prose,
// always shown regardless of shape — unlike FOUND, neither is conditional.
func writeConformanceReviewerRow(b *strings.Builder, label, value string) {
	b.WriteString(`<div class="claim-conformance-row"><span class="claim-conformance-row-label">`)
	b.WriteString(label)
	b.WriteString(`</span><span class="claim-conformance-row-value">`)
	b.WriteString(html.EscapeString(value))
	b.WriteString(`</span></div>`)
}

// writeConformanceFoundRow emits the FOUND row: for a set-shaped check with
// an actual observation, a marked coverage strip over the declared members
// (R12.3 — tinting marks the compared members, set order carries no
// meaning; R09.8 — Expected + Observed merge into one marked set, not two
// lists); for a scalar-shaped check, the observed value directly (06a §8.6).
// A check with no observation yet (Owed) or an unusable observation
// (certain Uncheckable branches) has nothing to have "found", so the row is
// omitted rather than asserting a comparison that never ran.
func writeConformanceFoundRow(b *strings.Builder, check conformance.CheckResult) {
	if check.Observed == nil {
		return
	}
	switch check.Shape {
	case model.ExpectationShapeSet:
		expected, ok := check.Expected.([]string)
		if !ok || len(expected) == 0 {
			return
		}
		missing := make(map[string]bool, len(check.Missing))
		for _, m := range check.Missing {
			missing[m] = true
		}
		b.WriteString(`<div class="claim-conformance-row claim-conformance-row--found"><span class="claim-conformance-row-label">FOUND</span><div class="claim-conformance-row-value">`)
		b.WriteString(`<div class="claim-conformance-coverage" role="list">`)
		for _, member := range expected {
			if missing[member] {
				b.WriteString(`<span class="claim-conformance-chip claim-conformance-chip--missing" role="listitem">`)
			} else {
				b.WriteString(`<span class="claim-conformance-chip" role="listitem">`)
			}
			b.WriteString(html.EscapeString(member))
			b.WriteString(`</span>`)
		}
		b.WriteString(`</div>`)
		exercised := len(expected) - len(check.Missing)
		b.WriteString(`<div class="claim-conformance-legend"><span class="claim-conformance-legend-count">`)
		b.WriteString(fmt.Sprintf("%d of %d exercised", exercised, len(expected)))
		b.WriteString(`</span>`)
		if len(check.Missing) > 0 {
			b.WriteString(`<span class="claim-conformance-legend-swatch" aria-hidden="true"></span><span class="claim-conformance-legend-label">never run</span>`)
		}
		b.WriteString(`</div></div></div>`)
	case model.ExpectationShapeScalar:
		observed, ok := check.Observed.(string)
		if !ok {
			return
		}
		writeConformanceReviewerRow(b, "FOUND", observed)
	}
}

// writeConformanceDisclosure emits the nested "How this was checked"
// disclosure (R12.1). It is closed by default and does not inherit the
// panel's own auto-open (R09.3 — auto-open never cascades into the second
// level), regardless of the check's own readiness.
func writeConformanceDisclosure(b *strings.Builder, check conformance.CheckResult) {
	b.WriteString(`<details class="claim-conformance-disclosure"><summary class="claim-conformance-disclosure-trigger"><svg class="claim-conformance-chevron" viewBox="0 0 24 24" width="12" height="12" aria-hidden="true"><path d="m9 18 6-6-6-6" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/></svg><span>How this was checked</span></summary><div class="claim-conformance-machine">`)
	writeConformanceLine(b, "check", check.ID)
	writeConformanceLine(b, "adapter", check.Adapter)
	writeConformanceLine(b, "shape", string(check.Shape))
	writeConformanceLine(b, "target", check.Target)
	writeConformanceValue(b, "expected", check.Expected)
	if check.Observed != nil {
		writeConformanceValue(b, "observed", check.Observed)
	}
	if len(check.Missing) > 0 {
		writeConformanceMembers(b, "missing", check.Missing)
	}
	if len(check.Extra) > 0 {
		writeConformanceMembers(b, "extra", check.Extra)
	}
	if check.ObservationError != nil {
		writeConformanceLine(b, "observation error", check.ObservationError.Code)
		writeConformanceLine(b, "adapter message", check.ObservationError.Message)
	}
	if check.Reason != "" {
		writeConformanceLine(b, "detail", check.Reason)
	}
	writeConformanceCopyRow(b, check)
	b.WriteString(`</div></details>`)
}

// writeConformanceCopyRow emits the "Copy as JSON" control (06a §4.8, §6).
// The envelope it copies is the same nine-key set the machine layer above it
// renders, marshaled server-side so the button never has to re-derive it
// from the DOM; the click is wired by the delegated listener appended to
// conformanceStatusFetchGuardJS (conformance_view.go), injected at the same
// guarded shell.html point as the status-freshness guard.
func writeConformanceCopyRow(b *strings.Builder, check conformance.CheckResult) {
	envelope := conformanceCopyEnvelope(check)
	b.WriteString(`<button type="button" class="claim-conformance-copy" data-copy-json="`)
	b.WriteString(html.EscapeString(envelope))
	b.WriteString(`"><svg class="claim-conformance-copy-icon" viewBox="0 0 24 24" width="12" height="12" aria-hidden="true"><rect x="8" y="8" width="14" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="2"/><path d="M4 16V6a2 2 0 0 1 2-2h10" fill="none" stroke="currentColor" stroke-width="2"/></svg><span class="claim-conformance-copy-label">Copy as JSON</span><span class="claim-conformance-copy-caption">the same envelope dossierx check returns</span></button>`)
}

// conformanceCopyEnvelope builds the nine-key JSON object the machine layer
// displays, in the same fixed order, for the Copy control's data attribute.
// Building it with encoding/json (rather than string-joining) guarantees the
// attribute is valid JSON however a project-supplied id/target/reason value
// is spelled.
func conformanceCopyEnvelope(check conformance.CheckResult) string {
	type observationErrorJSON struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	}
	envelope := struct {
		Check            string                `json:"check"`
		Adapter          string                `json:"adapter"`
		Shape            string                `json:"shape"`
		Target           string                `json:"target"`
		Expected         any                   `json:"expected,omitempty"`
		Observed         any                   `json:"observed,omitempty"`
		Missing          []string              `json:"missing,omitempty"`
		Extra            []string              `json:"extra,omitempty"`
		ObservationError *observationErrorJSON `json:"observation_error,omitempty"`
		Detail           string                `json:"detail,omitempty"`
	}{
		Check: check.ID, Adapter: check.Adapter, Shape: string(check.Shape), Target: check.Target,
		Expected: check.Expected, Observed: check.Observed, Missing: check.Missing, Extra: check.Extra,
		Detail: check.Reason,
	}
	if check.ObservationError != nil {
		envelope.ObservationError = &observationErrorJSON{Code: check.ObservationError.Code, Message: check.ObservationError.Message}
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func conformanceStateLabel(state conformance.State) string {
	switch state {
	case conformance.StateMatched:
		return "Matched"
	case conformance.StateMismatch:
		return "Mismatch"
	case conformance.StateUncheckable:
		return "Uncheckable"
	case conformance.StateOwed:
		return "Owed"
	default:
		return string(state)
	}
}

func writeConformanceValue(b *strings.Builder, label string, value any) {
	switch value := value.(type) {
	case string:
		writeConformanceLine(b, label, value)
	case []string:
		writeConformanceMembers(b, label, value)
	}
}

func writeConformanceLine(b *strings.Builder, label, value string) {
	b.WriteString(`<p class="claim-conformance-line"><span>`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`:</span> <code>`)
	b.WriteString(html.EscapeString(value))
	b.WriteString(`</code></p>`)
}

// writeConformanceMembers renders a set-shaped value as its own flex child
// (claim-conformance-values) rather than as several sibling <code> elements
// directly inside the flex row .claim-conformance-line establishes for its
// 84px key column (06a §4.8): as direct flex items, thirteen small <code>
// elements each get squeezed towards their shrink-to-fit minimum by the row
// layout, and overflow-wrap:anywhere then breaks each one mid-word instead
// of the whole list wrapping normally. One wrapper flex-item lets the
// comma-joined list wrap as ordinary inline content inside it.
func writeConformanceMembers(b *strings.Builder, label string, values []string) {
	b.WriteString(`<p class="claim-conformance-line"><span>`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`:</span> <span class="claim-conformance-values">`)
	for i, value := range values {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteString(`<code>`)
		b.WriteString(html.EscapeString(value))
		b.WriteString(`</code>`)
	}
	b.WriteString(`</span></p>`)
}
