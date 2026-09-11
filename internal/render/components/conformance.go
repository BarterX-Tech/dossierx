package components

import (
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
func ConformanceHTML(result conformance.Result) template.HTML {
	var b strings.Builder
	b.WriteString(`<section class="claim-conformance" data-conformance-mode="`)
	b.WriteString(html.EscapeString(string(result.Mode)))
	b.WriteString(`" data-claim-id="`)
	b.WriteString(html.EscapeString(result.ClaimID))
	b.WriteString(`" data-implementation-ready="`)
	b.WriteString(fmt.Sprintf("%t", result.ImplementationReady))
	if result.Mode == model.EmbodimentModeNone {
		b.WriteString(`" data-conformance-state="declared_none`)
	}
	b.WriteString(`"><div class="claim-conformance-head"><strong>Implementation conformance</strong> <span class="pill `)
	if result.ImplementationReady {
		b.WriteString(`ps">ready`)
	} else {
		b.WriteString(`pw">not ready`)
	}
	b.WriteString(`</span></div>`)

	if result.Mode == model.EmbodimentModeNone {
		b.WriteString(`<p class="claim-conformance-scope">Ready here means this claim deliberately declares no software embodiment. Claim and release readiness remain separate.</p>`)
		writeConformanceLine(&b, "declaration", "none")
		writeConformanceLine(&b, "reason", result.Reason)
		b.WriteString(`</section>`)
		return template.HTML(b.String()) //nolint:gosec // all values escaped above
	}

	b.WriteString(`<p class="claim-conformance-scope">Ready here means every declared check matches. Claim and release readiness remain separate.</p>`)
	for _, check := range result.Checks {
		writeConformanceCheck(&b, check)
	}
	b.WriteString(`</section>`)
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
	b.WriteString(`"><div class="claim-conformance-check-head"><strong>`)
	b.WriteString(html.EscapeString(check.ID))
	b.WriteString(`</strong> <span class="pill `)
	b.WriteString(conformancePill(check.State))
	b.WriteString(`">`)
	b.WriteString(html.EscapeString(state))
	b.WriteString(`</span></div>`)

	writeConformanceLine(b, "shape", string(check.Shape))
	writeConformanceLine(b, "adapter", check.Adapter)
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
	b.WriteString(`</article>`)
}

func conformancePill(state conformance.State) string {
	switch state {
	case conformance.StateMatched:
		return "ps"
	case conformance.StateMismatch, conformance.StateUncheckable:
		return "pw"
	default:
		return "pv"
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

func writeConformanceMembers(b *strings.Builder, label string, values []string) {
	b.WriteString(`<p class="claim-conformance-line"><span>`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`:</span> `)
	for i, value := range values {
		if i > 0 {
			b.WriteString(`, `)
		}
		b.WriteString(`<code>`)
		b.WriteString(html.EscapeString(value))
		b.WriteString(`</code>`)
	}
	b.WriteString(`</p>`)
}
