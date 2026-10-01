// brief.go holds the two pieces of claim chrome a brief page (NIT-197)
// borrows: the status pill a claim card's head carries, and the relationship
// row a claim's footer lists its rests_on edges with. A brief is not a claim,
// so neither is reached through a layout partial; these are the same writers,
// exported for the one caller that is not a partial.
package components

import (
	"html/template"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// BriefStatusPillHTML is the status pill for a brief's frontmatter status,
// drawn by the claim pill's own rules (pillClass, StatusLabel,
// StatusIconHTML): a closed padlock and "Locked" for locked, an open one and
// "Draft" for draft. A brief carries no review_pending state yet (NIT-199
// and NIT-200 add the brief lock record and review), so the review form of
// the pill never appears here. Any other value renders no pill: the
// frontmatter parser refuses it, and a pill that guessed would be wrong.
func BriefStatusPillHTML(status string) template.HTML {
	st := model.Status(status)
	if st != model.StatusLocked && st != model.StatusDraft {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<span class="pill `)
	b.WriteString(pillClass(st, false))
	b.WriteString(` brief-pill">`)
	b.WriteString(string(StatusIconHTML(st, false)))
	b.WriteString(`<span class="brief-pill__label">`)
	b.WriteString(StatusLabel(st, false))
	b.WriteString(`</span></span>`)
	return template.HTML(b.String())
}

// BriefRelationRowsHTML is one <li> relationship row per claim id, written by
// the claim footer's own writeRelationshipRow: the lifecycle dot, the claim's
// label as a link to its card, its module · facet, and its lifecycle badge.
// A brief sits in no module and no facet, so no prefix is elided against one.
// An id statuses does not know (a rests_on entry that names no claim, which
// check reports as brief-rests-on-unknown) degrades to the bare link, exactly
// as it does in a claim footer.
func BriefRelationRowsHTML(ids []string, statuses map[string]TargetStatus) template.HTML {
	return BriefRelationRowsChangedHTML(ids, statuses, nil)
}

// BriefRelationRowsChangedHTML is BriefRelationRowsHTML with an optional
// "changed <date>" amber note per claim id (NIT-200): a rests_on claim
// whose baseline moved since the brief was approved. Unknown ids stay
// empty. The note is escaped.
func BriefRelationRowsChangedHTML(ids []string, statuses map[string]TargetStatus, changed map[string]string) template.HTML {
	var b strings.Builder
	for _, id := range ids {
		note := ""
		if changed != nil {
			note = changed[id]
		}
		writeRelationshipRowNoted(&b, "brief-relation", id, "", "", statuses, note)
	}
	return template.HTML(b.String())
}

// BriefLockPillHTML is a brief's pill once its lock record is read
// (NIT-199 / NIT-200): a locked or draft brief keeps BriefStatusPillHTML's
// pill; a locked brief whose file moved since its approval reads EDITED
// SINCE APPROVAL (Paper B2; "Edited" on a phone); a standing locked brief
// whose rests_on claim moved reads REVIEW PENDING ("Review" short) in the
// draft hue; and one whose file says locked with no approval on record
// reads LOCK NOT RECORDED ("Unrecorded" short). Edited outranks review.
func BriefLockPillHTML(lockState, status string, short bool) template.HTML {
	var cls, icon, label string
	switch lockState {
	case "edited":
		cls, icon, label = "brief-pill--edited", "#dx-icon-lock", "Edited since approval"
		if short {
			label = "Edited"
		}
	case "review":
		// Amber, the draft hue, matching the sidebar review mark and
		// brief-dependency-drift's Needs-you tone — not the claim card's
		// red review-pending pill.
		cls, icon, label = "pv brief-pill--review", "#dx-icon-lock", "Review pending"
		if short {
			label = "Review"
		}
	case "unrecorded":
		cls, icon, label = "pv brief-pill--unrecorded", "#dx-icon-lock-open", "Lock not recorded"
		if short {
			label = "Unrecorded"
		}
	default:
		return BriefStatusPillHTML(status)
	}
	return template.HTML(`<span class="pill ` + cls + ` brief-pill"><svg class="dx-icon" aria-hidden="true"><use href="` + icon + `"/></svg><span class="brief-pill__label">` + label + `</span></span>`)
}
