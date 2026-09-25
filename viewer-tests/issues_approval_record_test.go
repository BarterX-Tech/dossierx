package viewertests

// TestIssuesApprovalRecordSortsFirstWithRedBorder pins 04 §8 item 3 / R08.2:
// an APPROVAL RECORD (ledger/integrity) finding is a different KIND of
// finding than a lint or readiness one — it sorts before every module
// group regardless of weight, and it takes a red leading edge rather than
// the amber vocabulary the rest of the promoted list uses.
//
// ledger_findings only ever arrives from a live GET /api/status
// (shell.html:265-271), and no fixture in this corpus corrupts a lock
// ledger to produce one (04 §5's own residual-risk note: "ledger findings —
// have no viewer fixture at all"), so this test drives the same
// window.dossierxRenderStatusStrip(data) hook viewer-runtime.js exposes for
// exactly this purpose, with a synthetic ledger finding standing in for a
// live serve's response.

import (
	"testing"
)

func TestIssuesApprovalRecordSortsFirstWithRedBorder(t *testing.T) {
	p := newReadinessProject(t)
	// The served verdict must have landed before the synthetic one is painted,
	// or it arrives afterwards and replaces it.
	ctx := newLiveTabWithStatus(t, p)
	pollTrue(t, ctx, `!document.getElementById('statusStrip').hidden`)

	evalVoid(t, ctx, `window.dossierxRenderStatusStrip({
		ledger_findings: [{rule: 'lock-tamper', claim_id: 'widget.contract.root'}],
		readiness: {}
	})`)

	if !evalBool(t, ctx, `!!document.querySelector('#statusStripBody .status-group--approval-record')`) {
		t.Fatal("an APPROVAL RECORD group must render when ledger_findings is non-empty")
	}
	if evalString(t, ctx, `document.querySelector('#statusStripBody .status-group-head-name').textContent`) != "APPROVAL RECORD" {
		t.Fatalf("first group heading = %q, want literal APPROVAL RECORD (R08.2)",
			evalString(t, ctx, `document.querySelector('#statusStripBody .status-group-head-name').textContent`))
	}
	if !evalBool(t, ctx, `document.querySelector('#statusStripBody .status-group').classList.contains('status-group--approval-record')`) {
		t.Fatal("APPROVAL RECORD must be the FIRST group in the body, ahead of every module group, regardless of weight")
	}
	borderColor := evalString(t, ctx, `getComputedStyle(document.querySelector('.status-group--approval-record')).borderLeftColor`)
	warn := evalString(t, ctx, `getComputedStyle(document.documentElement).getPropertyValue('--warn').trim()`)
	warnRGB := evalString(t, ctx, `(function(){ var d = document.createElement('div'); d.style.color = getComputedStyle(document.documentElement).getPropertyValue('--warn'); document.body.appendChild(d); var c = getComputedStyle(d).color; d.remove(); return c; })()`)
	if borderColor != warnRGB {
		t.Fatalf("APPROVAL RECORD border-left-color = %q, want --warn (%q -> %q), never the amber lint colour", borderColor, warn, warnRGB)
	}
}
