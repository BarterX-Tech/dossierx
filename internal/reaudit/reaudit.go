// Package reaudit implements the "dossierx reaudit <id>" flow: proposing a
// diff for a locked+review_pending claim whose dependency changed
// underneath it, and — only on explicit human confirmation — applying it.
//
// ProposeDiff is deliberately stubbed here: producing the actual diff is
// an LLM call, which is out of scope for this repository. The seam is
// documented below so a later phase can drop in a real implementation
// without touching any caller (internal CLI code in cmd/dossierx only ever
// calls ProposeDiff and Apply, never anything LLM-specific directly).
package reaudit

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Diff is a proposed edit to a claim, rendered as git-diff-style markup:
// additions wrapped in `<mark style="background:#b7ebb0">…</mark>`,
// removals wrapped in `<mark style="background:#f7c2c2;text-decoration:line-through">…</mark>`.
// NoChange is set when the proposer concludes the claim still holds as-is;
// in that case Body carries just an audit note, not a content edit.
type Diff struct {
	ClaimID  string
	NoChange bool
	// Body is the proposed new claim body, inline-marked up per the
	// convention above. Present a claim's current body unmodified (no
	// <mark> at all) if NoChange is true.
	Body string
	// Note is a short human-readable explanation of why this diff (or
	// lack of one) was proposed; always shown alongside Body.
	Note string

	// ResultingBody is the EXACT text Apply will write into claim.Body if this
	// proposal is confirmed — Body with its markup collapsed, computed by the
	// same stripMarkup call Apply itself makes, so the two cannot disagree.
	//
	// It exists because the preview did not disclose the one thing a human has
	// to agree to. A flag-sourced proposal renders as a phrase-level diff (the
	// red claim-says span next to the green now-does span), and both skills
	// described it that way — but Apply REPLACES THE WHOLE BODY with now-does
	// (see ProposeFlagDiff for why that is the intended semantics). On a
	// two-sentence locked claim, `claim reaudit --confirm --reason "confirmed
	// with the human"` therefore deleted the second sentence and re-signed the
	// truncated body into the ledger with the human's own words attached to text
	// they were never shown. Showing the resulting body — not just the diff —
	// is what makes the confirmation informed.
	ResultingBody string

	// SideEffects names, in plain sentences, what confirming this proposal does
	// BEYOND the diff a reader can see in Body. It is empty when there is
	// nothing beyond it. Today there is exactly one entry it can carry (whole-
	// body replacement dropping content the diff never mentioned), but it is a
	// list rather than a bool so a later proposer can add its own without
	// changing this type or its callers.
	SideEffects []string
}

// Matches a full red (removal) <mark> span, content included, so it can be
// deleted entirely. Matches a full green (addition) <mark> span with its
// content captured, so it can be unwrapped (tags removed, text kept).
var (
	redMarkSpan   = regexp.MustCompile(`(?s)<mark style="background:#f7c2c2;text-decoration:line-through">.*?</mark>`)
	greenMarkSpan = regexp.MustCompile(`(?s)<mark style="background:#b7ebb0">(.*?)</mark>`)
	anyMarkTag    = regexp.MustCompile(`</?mark[^>]*>`)
)

// stripMarkup applies the docs-slice-review convention: delete red
// (removal) spans entirely, unwrap green (addition) spans keeping their
// text, then strip any remaining stray <mark> tag as a safety net. Green
// span content is html-unescaped on the way out, mirroring the
// html.EscapeString applied by ProposeFlagDiff when it built the span, so
// the plain text that survives into claim.Body matches exactly what the
// caller passed as --now-does (not an HTML-escaped copy of it).
func stripMarkup(s string) string {
	s = redMarkSpan.ReplaceAllString(s, "")
	s = greenMarkSpan.ReplaceAllStringFunc(s, func(m string) string {
		sub := greenMarkSpan.FindStringSubmatch(m)
		return html.UnescapeString(sub[1])
	})
	s = anyMarkTag.ReplaceAllString(s, "")
	return s
}

// ProposeDiff is the seam a real LLM-backed implementation replaces later.
// It never mutates claim or changedDep, never writes to disk, and never
// changes lock state — those only happen via a confirmed Apply, driven by
// the CLI's --confirm flag. This stub returns a placeholder Diff that
// flags NoChange so a naive caller never mistakes it for a real proposal.
//
// Per FORMAT.md, "dossierx reaudit <id>" is only ever valid against a
// claim that is currently locked AND flagged review_pending; ProposeDiff
// enforces that precondition itself (rather than leaving it to the CLI) so
// every caller gets it for free. The CLI maps this error to exit code 2.
func ProposeDiff(claim, changedDep model.Claim) (Diff, error) {
	if claim.ID == "" {
		return Diff{}, fmt.Errorf("reaudit: claim has no id")
	}
	if claim.Status != model.StatusLocked || !claim.ReviewPending {
		return Diff{}, fmt.Errorf("reaudit: claim %q is not locked+review_pending, refusing", claim.ID)
	}
	return Diff{
		ClaimID:  claim.ID,
		NoChange: true,
		Body:     claim.Body,
		// A no-change proposal writes the body back unchanged, so the resulting
		// body IS the current one. It is still filled in rather than left empty:
		// a caller that renders ResultingBody must never have to know which
		// proposer produced the Diff to know whether the field means anything.
		ResultingBody: stripMarkup(claim.Body),
		Note: fmt.Sprintf(
			"stub: ProposeDiff has no real LLM backend yet; dependency %q changed but no proposal was generated",
			changedDep.ID,
		),
	}, nil
}

// ProposeFlagDiff proposes a reaudit diff sourced from an agent's "dossierx
// flag" call (see PendingFlag) — the second reaudit trigger source
// alongside ProposeDiff's dependency-content-change path, and CLI-selected
// (see cmd/dossierx/main.go's newReauditCmd): a claim with a pending flag in
// FlagStore uses this function; every other review_pending claim keeps
// using ProposeDiff exactly as before.
//
// Unlike ProposeDiff's LLM-shaped stub — deciding whether a dependency's
// change actually invalidates a claim needs real reasoning this repo
// doesn't have — a flag already carries the exact assertion an agent is
// making (claim-says is wrong, now-does is right, reason is why), so this
// path produces a real, ready-to-review diff with no stub involved:
// claim-says becomes the red (removal) span, now-does becomes the green
// (addition) span, per the docs-slice-review markup convention this whole
// engine shares (see this file's Diff doc comment). Once confirmed,
// Apply's stripMarkup collapses that pair down to just now-does — i.e. a
// confirmed flag-sourced reaudit replaces the claim's entire Body with
// now-does. A flag is meant for "this claim's core assertion changed", not
// a surgical in-place phrase edit within a larger body, so replacing the
// whole body is the correct behavior here, not a limitation.
//
// It is, however, a thing the human confirming it has to be TOLD, and that is
// what Diff.ResultingBody and Diff.SideEffects are for. Body alone renders as a
// phrase-level diff — one red span, one green span — which reads as a surgical
// edit and says nothing about the rest of a multi-sentence body going with it.
// The semantics are intended; the silence was not, and it made `claim reaudit
// --confirm --reason "…"` sign a truncated body into the ledger under words the
// human gave for something else. Both fields are populated here so a caller can
// show the exact resulting text and the loss BEFORE Apply is ever called.
//
// Same precondition as ProposeDiff: claim must be locked AND
// review_pending — enforced here too so this function is just as safe to
// call directly (e.g. from a future non-CLI caller) without relying on the
// CLI to have already checked it.
func ProposeFlagDiff(claim model.Claim, flag PendingFlag) (Diff, error) {
	if claim.ID == "" {
		return Diff{}, fmt.Errorf("reaudit: claim has no id")
	}
	if claim.Status != model.StatusLocked || !claim.ReviewPending {
		return Diff{}, fmt.Errorf("reaudit: claim %q is not locked+review_pending, refusing", claim.ID)
	}
	// html.EscapeString guards against flag.ClaimSays/flag.NowDoes
	// containing a literal "<mark"/"</mark>" (or any other markup), which
	// would otherwise be indistinguishable from the span delimiters
	// themselves and corrupt both this rendered diff and, after --confirm,
	// stripMarkup's regex-based parse of it. stripMarkup unescapes the
	// green span back to plain text on the way out.
	body := fmt.Sprintf(
		`<mark style="background:#f7c2c2;text-decoration:line-through">%s</mark><mark style="background:#b7ebb0">%s</mark>`,
		html.EscapeString(flag.ClaimSays), html.EscapeString(flag.NowDoes),
	)
	return Diff{
		ClaimID:  claim.ID,
		NoChange: false,
		Body:     body,
		// ResultingBody is computed through stripMarkup — the SAME function Apply
		// calls — rather than by re-deriving "it will be NowDoes". Re-deriving it
		// would be a second implementation of the collapse rule, and a preview
		// that agrees with Apply only by inspection is exactly what shipped here
		// before: the preview showed a phrase diff, Apply replaced the body, and
		// nothing in the payload said so.
		ResultingBody: stripMarkup(body),
		SideEffects:   flagDiffSideEffects(claim.Body, flag.ClaimSays),
		Note:          fmt.Sprintf("flagged: %s", flag.Reason),
	}, nil
}

// flagDiffSideEffects returns the disclosure lines for a flag-sourced proposal:
// today, the one that says the confirmed reaudit replaces the claim's ENTIRE
// body with --now-does, and how much of the current body that drops.
//
// It fires only when there is something to lose — a body that is nothing but
// the flagged phrase loses nothing, and announcing a side effect that does not
// happen is how a disclosure becomes noise people skip. What "something to lose"
// means is deliberately literal: remove the flagged phrase from the current body
// and count the non-blank lines that remain. Counting LINES rather than
// characters is what makes the number checkable against the file a human is
// looking at.
func flagDiffSideEffects(currentBody, claimSays string) []string {
	remainder := currentBody
	if claimSays != "" {
		remainder = strings.Replace(remainder, claimSays, "", 1)
	}
	lost := 0
	for _, line := range strings.Split(remainder, "\n") {
		if strings.TrimSpace(line) != "" {
			lost++
		}
	}
	if lost == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"replaces the claim's entire body with --now-does (%d other line(s) of the current body will be removed)",
		lost)}
}

// Apply strips the diff's <mark> markup and returns claim with its Body
// replaced by the plain-text result. It does not touch Status,
// ReviewPending, or lock timestamps/hashes — internal/lock owns those and
// the CLI's confirmed-reaudit path is expected to call
// lock.ClearReviewPending separately after Apply succeeds.
//
// It always appends diff.Note (when non-blank) to claim.AuditNotes, so a
// confirmed reaudit — content change or a bare "still holds" confirmation
// — leaves a durable audit trail on the claim, per FORMAT.md.
func Apply(claim model.Claim, diff Diff) (model.Claim, error) {
	if diff.ClaimID != claim.ID {
		return claim, fmt.Errorf("reaudit: diff is for claim %q, not %q", diff.ClaimID, claim.ID)
	}

	claim.Body = stripMarkup(diff.Body)
	if note := strings.TrimSpace(diff.Note); note != "" {
		claim.AuditNotes = append(claim.AuditNotes, note)
	}
	return claim, nil
}
