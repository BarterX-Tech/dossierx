// Lint comments-unresolved flags a claim that carries one or more unresolved
// (status: open) comment threads. It is deliberately a WARNING, and — unlike
// rest-on-locked — it has NO Status branch: it fires on draft AND locked
// claims alike. An open thread is worth surfacing at every stage: on a draft
// under discussion, and on a locked claim where the same open thread is
// simultaneously internal/lock.Lock's refusal reason (a claim cannot lock
// while a thread is open) and the claim's review_pending trigger.
//
// This lint never FAILS "dossierx lint"/"dossierx check" — the hard refusal
// lives in internal/lock.Lock, and the buildorder completeness gate refuses a
// module with an open thread; this rule is only the visible, non-blocking
// surfacing of the same fact, exactly like the orphan warning.
//
// Banner-layout claims are skipped: a banner is a decorative divider that
// never renders an edges/comment panel and that the CLI and internal/comments
// refuse to attach a thread to, so it could never legitimately trip this rule.
//
// It reads model.Claim.OpenThreadIDs() from internal/model directly and imports
// NEITHER internal/lock NOR internal/comments: internal/lock imports
// internal/lint, so a lint that reached back into lock (or into comments, which
// itself imports lock) would create an import cycle. The predicate lives on
// model precisely so this rule stays cycle-free — see internal/model/comment.go.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, commentsUnresolvedLint{})
}

type commentsUnresolvedLint struct{}

func (commentsUnresolvedLint) Name() string { return "comments-unresolved" }

func (commentsUnresolvedLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, c := range claims {
		if c.Layout == model.LayoutBanner {
			continue
		}
		open := c.OpenThreadIDs()
		if len(open) == 0 {
			continue
		}
		findings = append(findings, Finding{
			LintName: "comments-unresolved",
			ClaimID:  c.ID,
			Message:  fmt.Sprintf("%d unresolved comment thread(s)", len(open)),
			Severity: SeverityWarning,
		})
	}
	return findings
}
