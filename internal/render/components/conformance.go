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
// snapshot is the observation pass's provenance hash
// (catalog.Catalog.ConformanceSnapshot, sourced from conformance.Report.Snapshot
// — see conformance_view.go's writeConformanceDisclosure doc comment and
// VAULT/learnings/inbox/L7.md item 3 for why this is a separate parameter
// rather than a field on conformance.Result: Result is also the exact shape
// catalog.Document embeds per claim into catalog.json and that
// catalogBudget.addConformance counts bytes against, and duplicating the
// same hash into every claim's JSON-serialized Result would grow both
// without either being updated for it. Passing it alongside instead keeps
// both untouched.
func ConformanceHTML(result conformance.Result, snapshot string) template.HTML {
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
	// RETRY FIX (verifier item 8): the summary's own label no longer echoes
	// the panel-body eyebrow's literal "Implementation checks" text two
	// lines below it (06a §4.3, §6 — the board draws that string exactly
	// once). The summary instead carries a count, following the sibling
	// claim-footer doors' own convention (components.go's
	// countSegment(links, "relationship") / countSegment(sources, "source")
	// for the closed chip label vs. their own panel-head eyebrow). For
	// declared_none the count is meaningless (there are no checks by
	// design), so the summary uses 07 §6's own footer-vocabulary word for
	// this state, "No checks declared", verbatim.
	b.WriteString(`><summary class="claim-conformance-head"><strong>`)
	if result.Mode == model.EmbodimentModeNone {
		b.WriteString(`No checks declared`)
	} else {
		b.WriteString(countSegment(len(result.Checks), "check"))
	}
	b.WriteString(`</strong> <span class="pill `)
	if result.ImplementationReady {
		b.WriteString(`ps">Ready`)
	} else {
		b.WriteString(`pw">Not ready`)
	}
	b.WriteString(`</span></summary>`)

	if result.Mode == model.EmbodimentModeNone {
		// RETRY FIX (verifier item 15, 07 §4.12): DECLARATION / REASON is a
		// two-row label column (96px, uppercase Inter 11/14 w600 tracking
		// 0.07em, --faint), `none` in mono 12/20 --muted and the reason in
		// serif 14/22 --muted max-width 640px — not a reuse of the machine
		// layer's writeConformanceLine (that shape is 84px/tracking .06em,
		// mono-only, and belongs to the compare-mode disclosure, not this
		// branch, which R09.8/06a §8.4 keep free of any nested disclosure).
		// The closing scope note moves BELOW the pair, serif 15/24 --muted
		// max-width 760px (07 §4.12's "Closing scope note" row) — 07 §6 is
		// itself a rewrite of this sentence, not a reuse of the compare-mode
		// scope sentence below.
		b.WriteString(`<div class="claim-conformance-declared">`)
		writeConformanceDeclaredRow(&b, "DECLARATION", "none", false)
		writeConformanceDeclaredRow(&b, "REASON", result.Reason, true)
		b.WriteString(`</div>`)
		b.WriteString(`<p class="claim-conformance-declared-note">Ready here means this claim deliberately declares no software embodiment — not that something was checked and passed.</p>`)
		b.WriteString(`</details>`)
		return template.HTML(b.String()) //nolint:gosec // all values escaped above
	}

	b.WriteString(`<p class="claim-conformance-scope">Ready here means every declared check matches. Claim and release readiness remain separate.</p>`)
	// RETRY FIX (verifier item 7): a hairline rule element between the
	// eyebrow label and its count, so the count right-ranges across the
	// panel width instead of sitting immediately after the label (06a §4.3
	// `3KE-0` gap 12px, `3KG-0` rule 1px `#EFE6E5`).
	b.WriteString(`<div class="claim-conformance-eyebrow"><span class="claim-conformance-eyebrow-label">Implementation checks</span><span class="claim-conformance-eyebrow-rule" aria-hidden="true"></span><span class="claim-conformance-eyebrow-count">`)
	b.WriteString(fmt.Sprintf("%d", len(result.Checks)))
	b.WriteString(`</span></div>`)
	for _, check := range result.Checks {
		writeConformanceCheck(&b, check, snapshot)
	}
	b.WriteString(`</details>`)
	return template.HTML(b.String()) //nolint:gosec // all values escaped above
}

// writeConformanceDeclaredRow emits one row of the declared_none DECLARATION
// / REASON label column (07 §4.12, verifier item 15). serif selects the
// REASON row's typography (serif 14/22 --muted, max-width 640px); the
// DECLARATION row stays mono 12/20 --muted, matching the family rule (07
// §4.12: "The DECLARATION value is mono and the REASON value is serif in
// the same two-row block ... rendering both in one family would erase the
// distinction").
func writeConformanceDeclaredRow(b *strings.Builder, label, value string, serif bool) {
	b.WriteString(`<div class="claim-conformance-declared-row"><span class="claim-conformance-declared-label">`)
	b.WriteString(label)
	b.WriteString(`</span><span class="claim-conformance-declared-value`)
	if serif {
		b.WriteString(` claim-conformance-declared-reason`)
	}
	b.WriteString(`">`)
	b.WriteString(html.EscapeString(value))
	b.WriteString(`</span></div>`)
}

func writeConformanceCheck(b *strings.Builder, check conformance.CheckResult, snapshot string) {
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

	// RETRY FIX (verifier item 6, R09.8, 06a §2): EXAMINED / COMPARED are
	// reviewer prose, not the demoted `Target` / `Adapter` identifiers —
	// both already appear verbatim in the machine layer below the line
	// ("target", "adapter"). EXAMINED's sentence is board-literal and
	// constant (`2UO-0`'s own worked example: "the verification suite for
	// this claim"); COMPARED varies by shape, matching the board's set-shape
	// example ("which of the thirteen declared steps the suite actually
	// exercises") generalised to any member count and, for a scalar check
	// with no set to compare, "the value this target reports".
	writeConformanceReviewerRow(b, "EXAMINED", "the verification suite for this claim")
	writeConformanceReviewerRow(b, "COMPARED", conformanceComparedSentence(check))
	writeConformanceFoundRow(b, check)

	writeConformanceDisclosure(b, check, snapshot)
	b.WriteString(`</article>`)
}

// conformanceComparedSentence builds the COMPARED row's reviewer-facing
// sentence (see writeConformanceCheck's doc comment above its call site).
func conformanceComparedSentence(check conformance.CheckResult) string {
	if check.Shape == model.ExpectationShapeSet {
		n := 0
		if expected, ok := check.Expected.([]string); ok {
			n = len(expected)
		}
		return fmt.Sprintf("which of the %d declared members the run actually exercised", n)
	}
	return "the value this target reports"
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
		// RETRY FIX (verifier item 3, 06a §4.5, R-H.3): the chip carries the
		// member's 1-based ORDINAL within `expected`, not the member id —
		// the board's own strip reads "1 2 3 … 13" (06a-desktop-light.png),
		// and only a short, fixed-length label lets the chip be a fixed
		// 32×29 / 24×28 box regardless of how long a project's step ids run.
		// The member id survives as the chip's accessible name.
		visible := expected
		overflow := 0
		if len(expected) > maxCoverageChips {
			// RETRY FIX (verifier item 4, 06a §9 Open decision 4, R-H.3):
			// the strip never wraps, so past the count R-H.3's own
			// arithmetic proves fits the narrowest (390px) tier — 13 chips
			// at 24px + 3px gaps = 348 of 358 — the run is capped and a
			// trailing "+N" marker in --color-faint stands in for the rest.
			// The full set stays readable in the machine layer's expected /
			// observed rows regardless of the cap. 13 is used at every
			// width, not just 390, because it is the one count proven safe
			// everywhere the strip renders; a wider tier having spare room
			// is not a reason to let the mobile tier overflow.
			visible = expected[:maxCoverageChips]
			overflow = len(expected) - maxCoverageChips
		}
		for i, member := range visible {
			class := `claim-conformance-chip`
			if missing[member] {
				class += ` claim-conformance-chip--missing`
			}
			b.WriteString(`<span class="`)
			b.WriteString(class)
			b.WriteString(`" role="listitem" aria-label="`)
			b.WriteString(html.EscapeString(member))
			b.WriteString(`" title="`)
			b.WriteString(html.EscapeString(member))
			b.WriteString(`">`)
			b.WriteString(fmt.Sprintf("%d", i+1))
			b.WriteString(`</span>`)
		}
		if overflow > 0 {
			b.WriteString(`<span class="claim-conformance-chip-overflow" aria-hidden="true">+`)
			b.WriteString(fmt.Sprintf("%d", overflow))
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
// level), regardless of the check's own readiness. snapshot is
// conformance.Result.Snapshot, threaded down from ConformanceHTML — the
// provenance hash for the observation pass that produced these values (06a
// §4.7 `2V2-0`, §6: "not an elapsed time and must not be rendered as one").
// It is per-check, not per-panel, because R09.2's one-open-at-a-time governs
// the checks expansion, not the nested disclosure inside each check (06a §8.5).
func writeConformanceDisclosure(b *strings.Builder, check conformance.CheckResult, snapshot string) {
	b.WriteString(`<details class="claim-conformance-disclosure"><summary class="claim-conformance-disclosure-trigger"><svg class="claim-conformance-chevron" viewBox="0 0 24 24" width="12" height="12" aria-hidden="true"><path d="m9 18 6-6-6-6" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"/></svg><span>How this was checked</span>`)
	if snapshot != "" {
		// RETRY FIX (verifier item 10, 06a §4.7 `2V1-0`/`2V2-0`, §6): a
		// flex-grow spacer, then the snapshot line, right-ranged, shown only
		// while the disclosure is open (conformanceCSS hides both by
		// default and reveals them under `.claim-conformance-disclosure[open]`).
		b.WriteString(`<span class="claim-conformance-disclosure-spacer" aria-hidden="true"></span><span class="claim-conformance-snapshot">snapshot `)
		b.WriteString(html.EscapeString(snapshot))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</summary><div class="claim-conformance-machine">`)
	writeConformanceLine(b, "check", check.ID)
	writeConformanceLine(b, "adapter", check.Adapter)
	writeConformanceLine(b, "shape", string(check.Shape))
	writeConformanceLine(b, "target", check.Target)
	writeConformanceValue(b, "expected", check.Expected)
	if check.Observed != nil {
		writeConformanceValue(b, "observed", check.Observed)
	}
	if len(check.Missing) > 0 {
		// RETRY FIX (verifier item 1, 06a §4.8 `2VR-0`): `missing` is the
		// blocked hue, weight 500 — the reader's one gap in the run.
		writeConformanceMembersModified(b, "missing", check.Missing, "blocked")
	}
	// RETRY FIX (verifier item 2, 06a §4.8 `2VU-0`, §9 Open decision 5): the
	// nine-key envelope is fixed-shape — `extra` with no members renders as
	// an em dash in --color-faint rather than being omitted, against the
	// pre-retry emitter behaviour this decision explicitly overrules.
	if len(check.Extra) > 0 {
		writeConformanceMembersModified(b, "extra", check.Extra, "")
	} else {
		writeConformanceLineModified(b, "extra", "—", "empty")
	}
	if check.ObservationError != nil {
		// RETRY FIX (verifier item 1, 06a §8.7): `observation error`'s
		// value is a failure code and takes the same blocked treatment as
		// `missing`; `adapter message` stays the plain machine-layer value.
		writeConformanceLineModified(b, "observation error", check.ObservationError.Code, "blocked")
		writeConformanceLine(b, "adapter message", check.ObservationError.Message)
	}
	if check.Reason != "" {
		writeConformanceLine(b, "detail", check.Reason)
	}
	writeConformanceCopyRow(b, check)
	b.WriteString(`</div></details>`)
}

// maxCoverageChips is the coverage strip's fixed cap (06a §9 Open decision
// 4, R-H.3) — see writeConformanceFoundRow's doc comment at its use site.
const maxCoverageChips = 13

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
	writeConformanceLineModified(b, label, value, "")
}

// writeConformanceLineModified renders one machine-layer key: value row,
// optionally carrying a state modifier class (RETRY FIX, verifier items 1-2;
// 06a §4.8, §8.7, §9 Open decision 5). "blocked" paints the value in --warn
// at weight 500 (the `missing` and `observation error` keys — `2VR-0`);
// "empty" paints an em-dash placeholder in --color-faint (the `extra` key
// with no members — `2VU-0`). conformanceCSS's
// `.claim-conformance-line--blocked > code, .claim-conformance-line--blocked
// .claim-conformance-values` descendant rule reaches writeConformanceMembers'
// wrapper span too, so the modifier only needs to sit on the outer `<p>`.
func writeConformanceLineModified(b *strings.Builder, label, value, modifier string) {
	b.WriteString(`<p class="claim-conformance-line`)
	if modifier != "" {
		b.WriteString(` claim-conformance-line--`)
		b.WriteString(modifier)
	}
	b.WriteString(`"><span>`)
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
	writeConformanceMembersModified(b, label, values, "")
}

// writeConformanceMembersModified is writeConformanceMembers with the same
// state-modifier hook as writeConformanceLineModified — see its doc comment.
func writeConformanceMembersModified(b *strings.Builder, label string, values []string, modifier string) {
	b.WriteString(`<p class="claim-conformance-line`)
	if modifier != "" {
		b.WriteString(` claim-conformance-line--`)
		b.WriteString(modifier)
	}
	b.WriteString(`"><span>`)
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
